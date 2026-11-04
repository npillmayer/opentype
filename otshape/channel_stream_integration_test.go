package otshape_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

type streamBaselineSink []otshape.GlyphRecord

func (s *streamBaselineSink) WriteGlyph(g otshape.GlyphRecord) error {
	*s = append(*s, g)
	return nil
}

func loadStreamFont(t *testing.T, name string) *ot.Font {
	t.Helper()
	data, err := os.ReadFile("../testdata/fonts/" + name)
	if err != nil {
		t.Fatal(err)
	}
	font, err := ot.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return font
}

func TestStreamingShaperSuccessiveRuns(t *testing.T) {
	latin := otshape.Params{
		Font: loadGentium(), Direction: bidi.LeftToRight,
		Script: language.MustParseScript("Latn"), Language: language.English,
	}
	noLiga := latin
	noLiga.Features = []otshape.FeatureRange{{Feature: ot.T("liga"), Start: 1, End: 4, On: false}}
	hebrew := otshape.Params{
		Font: loadStreamFont(t, "NotoSansHebrew-Regular.ttf"), Direction: bidi.RightToLeft,
		Script: language.MustParseScript("Hebr"), Language: language.Hebrew,
	}
	progress := latin
	progress.Font = loadStreamFont(t, "ShaperTestProgress.ttf")
	progress.Features = []otshape.FeatureRange{{Feature: ot.T("test"), On: true}}
	runs := []struct {
		text   string
		params otshape.Params
		opts   otshape.BufferOptions
	}{
		{"office", latin, otshape.BufferOptions{}},
		{"", latin, otshape.BufferOptions{}},
		{"office", noLiga, otshape.BufferOptions{FlushBoundary: otshape.FlushOnClusterBoundary}},
		{"e\u0301", latin, otshape.BufferOptions{}},
		{"ddd", progress, otshape.BufferOptions{}}, // All scalars are deleted.
		{"b", progress, otshape.BufferOptions{}},   // One scalar expands to two glyphs.
		{"שָׁלוֹם", hebrew, otshape.BufferOptions{}},
		{strings.Repeat("office ", 800), latin, otshape.BufferOptions{}},
		{"office", latin, otshape.BufferOptions{}},
	}
	// This is adapter parity, not a new shaping oracle. The example below uses
	// independently captured glyph IDs, positions, and clusters for "office".
	base := uint32(40)
	var want []otshape.GlyphRecord
	for i, run := range runs {
		if i == len(runs)-1 {
			base = 3 // An explicit reset can override automatic continuation.
		}
		var sink streamBaselineSink
		shaper := otshape.NewShaper(otcore.New(), othebrew.New())
		if err := shaper.Shape(run.params, strings.NewReader(run.text), &sink, run.opts); err != nil {
			t.Fatalf("baseline run %d: %v", i, err)
		}
		for _, glyph := range sink {
			glyph.Cluster += base
			want = append(want, glyph)
		}
		base += uint32(utf8.RuneCountInString(run.text))
	}

	stream, err := otshape.NewStreamingShaper(2, otcore.New(), othebrew.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.SetInitialCluster(40); err != nil {
		t.Fatal(err)
	}
	// Full font shaping under the race detector is much slower than channel
	// operations. This deadline bounds a stuck test, not shaping performance.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		for i, run := range runs {
			if i == len(runs)-1 {
				if err := stream.SetInitialCluster(3); err != nil {
					done <- err
					return
				}
			}
			if err := stream.Shape(ctx, run.params, strings.NewReader(run.text), run.opts); err != nil {
				done <- err
				return
			}
		}
		done <- stream.Finish()
	}()
	var got []otshape.GlyphRecord
	for g := range stream.Glyphs() {
		got = append(got, g)
		// Exercise concurrent status reads during both shaping and completion.
		if err := stream.Err(); err != nil {
			t.Errorf("stream failed: %v", err)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if stream.Err() != nil {
		t.Fatal(stream.Err())
	}
	if !reflect.DeepEqual(got, want) {
		if len(got) != len(want) {
			t.Fatalf("glyph count = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("glyph %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	}
}

type failingStreamSource struct {
	reader *strings.Reader
	left   int
	err    error
}

func (s *failingStreamSource) ReadRune() (rune, int, error) {
	if s.left == 0 {
		return 0, 0, s.err
	}
	s.left--
	return s.reader.ReadRune()
}

func TestStreamingShaperFailurePreservesCurrentRunPrefix(t *testing.T) {
	params := otshape.Params{
		Font: loadGentium(), Direction: bidi.LeftToRight,
		Script: language.MustParseScript("Latn"), Language: language.English,
	}
	input := strings.Repeat("office ", 800)
	failure := errors.New("interrupted input")
	var baseline streamBaselineSink
	if err := otshape.NewShaper(otcore.New()).Shape(params, strings.NewReader(input), &baseline, otshape.BufferOptions{}); err != nil {
		t.Fatal(err)
	}
	stream, err := otshape.NewStreamingShaper(2, otcore.New())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- stream.Shape(ctx, params, &failingStreamSource{strings.NewReader(input), 4800, failure}, otshape.BufferOptions{})
	}()
	var got []otshape.GlyphRecord
	for g := range stream.Glyphs() {
		got = append(got, g)
	}
	if err := <-done; !errors.Is(err, failure) || !errors.Is(stream.Err(), failure) {
		t.Fatalf("terminal result = %v, %v", err, stream.Err())
	}
	if len(got) == 0 || len(got) >= len(baseline) {
		t.Fatalf("expected a partial prefix, got %d of %d glyphs", len(got), len(baseline))
	}
	if !reflect.DeepEqual(got, []otshape.GlyphRecord(baseline[:len(got)])) {
		t.Fatal("emitted prefix differs from ordinary shaping")
	}
}
