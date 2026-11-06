package otshape_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func TestShapeIntoBoundsAndParity(t *testing.T) {
	latin := otshape.Params{
		Font: loadStreamFont(t, "GentiumPlus-R.ttf"), Direction: bidi.LeftToRight,
		Script: language.MustParseScript("Latn"), Language: language.English,
	}
	noLiga := latin
	noLiga.Features = []otshape.FeatureRange{{Feature: ot.T("liga"), On: false, Start: 1, End: 4}}
	hebrew := otshape.Params{
		Font: loadStreamFont(t, "NotoSansHebrew-Regular.ttf"), Direction: bidi.RightToLeft,
		Script: language.MustParseScript("Hebr"), Language: language.Hebrew,
	}
	progress := latin
	progress.Font = loadStreamFont(t, "ShaperTestProgress.ttf")
	progress.Features = []otshape.FeatureRange{{Feature: ot.T("test"), On: true}}
	shaper := otshape.NewShaper(otcore.New(), othebrew.New())
	for _, tc := range []struct {
		name, text string
		params     otshape.Params
	}{
		{"empty", "", latin},
		{"ligature", "office", latin},
		{"feature range", "office", noLiga},
		{"fragment", "hyphen-", latin},
		{"composition", "e\u0301", latin},
		{"RTL marks", "שָׁלוֹם", hebrew},
		{"expansion", "b", progress},
		{"deletion", "ddd", progress},
		{"trailing deletion", "addd", progress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Adapter parity checks every record field; ExampleShaper_ShapeInto
			// uses the existing independent HarfBuzz "office" capture.
			var want streamBaselineSink
			if err := shaper.Shape(tc.params, strings.NewReader(tc.text), &want, otshape.BufferOptions{}); err != nil {
				t.Fatal(err)
			}
			for size := 0; size <= len(want)+1; size++ {
				t.Run(fmt.Sprint(size), func(t *testing.T) {
					marker := otshape.GlyphRecord{GID: 65535, Cluster: 1234567, Mask: 7654321, UnsafeFlags: 65535}
					backing := make([]otshape.GlyphRecord, size+3)
					for i := range backing {
						backing[i] = marker
					}
					// The slice deliberately has spare capacity. Neither that
					// capacity nor either guard may be used for output.
					dst := backing[1 : size+1]
					n, err := shaper.ShapeInto(tc.params, strings.NewReader(tc.text), dst)
					if size < len(want) {
						if !errors.Is(err, otshape.ErrGlyphBufferFull) || errors.Is(err, otshape.ErrBufferFull) {
							t.Fatalf("overflow error = %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					if n != min(size, len(want)) || !slices.Equal(dst[:n], want[:n]) {
						t.Fatalf("result n=%d glyphs=%v, want prefix of %v", n, dst, want)
					}
					if backing[0] != marker {
						t.Fatal("leading guard overwritten")
					}
					for _, g := range backing[1+n:] {
						if g != marker {
							t.Fatal("unused output or spare capacity overwritten")
						}
					}
					// Reusing the shaper with another destination must not
					// change the earlier result or retain its cluster numbering.
					saved := slices.Clone(backing)
					retry := make([]otshape.GlyphRecord, len(want))
					n, err = shaper.ShapeInto(tc.params, strings.NewReader(tc.text), retry)
					if err != nil || n != len(want) || !slices.Equal(retry, want) || !slices.Equal(backing, saved) {
						t.Fatalf("retry failed: n=%d err=%v", n, err)
					}
				})
			}
			n, err := shaper.ShapeInto(tc.params, strings.NewReader(tc.text), nil)
			if n != 0 || (len(want) == 0 && err != nil) || (len(want) > 0 && !errors.Is(err, otshape.ErrGlyphBufferFull)) {
				t.Fatalf("nil destination: n=%d err=%v", n, err)
			}
		})
	}
}

func TestShapeIntoErrors(t *testing.T) {
	params := otshape.Params{
		Font: loadStreamFont(t, "ShaperTestProgress.ttf"), Direction: bidi.LeftToRight,
		Script: language.MustParseScript("Latn"), Language: language.English,
	}
	shaper := otshape.NewShaper(otcore.New())
	failure := errors.New("reader failed")
	for _, tc := range []struct {
		name   string
		shaper *otshape.Shaper
		params otshape.Params
		src    otshape.RuneSource
		want   error
	}{
		{"nil font", shaper, otshape.Params{}, strings.NewReader("a"), otshape.ErrNilFont},
		{"nil source", shaper, params, nil, otshape.ErrNilRuneSource},
		{"no engine", otshape.NewShaper(), params, strings.NewReader("a"), otshape.ErrNoShaper},
		{"source", shaper, params, &failingStreamSource{strings.NewReader("a"), 0, fmt.Errorf("read: %w", failure)}, failure},
		{"shaping limit", shaper, params, strings.NewReader("a" + strings.Repeat("\u0301", otshape.MaxInputGroup)), otshape.ErrSupportedLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dst := []otshape.GlyphRecord{{Cluster: 999}}
			n, err := tc.shaper.ShapeInto(tc.params, tc.src, dst)
			if n != 0 || !errors.Is(err, tc.want) || dst[0] != (otshape.GlyphRecord{Cluster: 999}) {
				t.Fatalf("n=%d err=%v dst=%v", n, err, dst)
			}
		})
	}

	// A later source failure must report and preserve the already emitted
	// prefix, just as Shape does. This input crosses actual flush boundaries.
	input := strings.Repeat("a ", 2800)
	source := func() otshape.RuneSource {
		return &failingStreamSource{strings.NewReader(input), 4800, failure}
	}
	var want streamBaselineSink
	if err := shaper.Shape(params, source(), &want, otshape.BufferOptions{}); !errors.Is(err, failure) {
		t.Fatalf("baseline error: %v", err)
	}
	dst := make([]otshape.GlyphRecord, len(input))
	n, err := shaper.ShapeInto(params, source(), dst)
	if !errors.Is(err, failure) || n == 0 || n >= len(input) || n != len(want) || !slices.Equal(dst[:n], want) {
		t.Fatalf("partial result: n=%d want=%d err=%v", n, len(want), err)
	}
	for _, g := range dst[n:] {
		if g != (otshape.GlyphRecord{}) {
			t.Fatal("unused output overwritten on source failure")
		}
	}
}
