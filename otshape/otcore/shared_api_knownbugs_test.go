//go:build shaping_knownbugs

package otcore_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// These tests assert intended behavior and currently fail. See
// doc/SHAPING_FINDINGS.md for evidence and promotion into the default suite.
// They use only exported APIs, the existing mini-font loader, and glyphCollector.

func alternateAPIInput(t *testing.T) (otshape.Params, string) {
	t.Helper()
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	cp := otquery.CodePointForGlyph(font, 18)
	if cp == 0 {
		t.Fatal("fixture precondition: GID 18 has no input codepoint")
	}
	if gid, err := otquery.GlyphIndex(font, cp); err != nil || gid != 18 {
		t.Fatalf("fixture precondition: cmap(%U) = %d, %v; want GID 18", cp, gid, err)
	}
	return otshape.Params{
		Font:      font,
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
		Features:  []otshape.FeatureRange{{Feature: ot.T("test"), On: true}},
	}, string(cp)
}

func shapeAPIRecords(t *testing.T, params otshape.Params, input string, opts otshape.BufferOptions, engines ...otshape.ShapingEngine) []otshape.GlyphRecord {
	t.Helper()
	sink := &glyphCollector{}
	if err := otshape.NewShaper(engines...).Shape(params, strings.NewReader(input), sink, opts); err != nil {
		t.Fatalf("Shape: %v", err)
	}
	return sink.glyphs
}

// All cases here are one-to-one transformations. Check original codepoint
// clusters as well as IDs so compaction cannot silently lose correspondence.
func assertAPIGlyphs(t *testing.T, got []otshape.GlyphRecord, want []ot.GlyphIndex) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("glyph count = %d, want %d; records=%+v", len(got), len(want), got)
	}
	for i, gid := range want {
		if got[i].GID != gid || got[i].Cluster != uint32(i) {
			t.Errorf("glyph[%d] = GID %d cluster %d, want GID %d cluster %d; record=%+v",
				i, got[i].GID, got[i].Cluster, gid, i, got[i])
		}
	}
}

type maskClearingEngine struct {
	otcore.Shaper
	calls *int
}

func (e maskClearingEngine) New() otshape.ShapingEngine {
	return maskClearingEngine{calls: e.calls}
}

func (e maskClearingEngine) SetupMasks(run otshape.RunContext) {
	*e.calls += 1
	if run.Len() > 0 {
		run.SetMask(0, 0)
	}
}

func TestKnownBugSHAPE001SetupMasksPreserved(t *testing.T) {
	params, cp := alternateAPIInput(t)
	input := strings.Repeat(cp, 2)
	opts := otshape.BufferOptions{HighWatermark: 8, LowWatermark: 4, MaxBuffer: 8}

	// Control: the fixture's enabled lookup substitutes both glyphs.
	control := shapeAPIRecords(t, params, input, opts, otcore.New())
	assertAPIGlyphs(t, control, []ot.GlyphIndex{20, 20})

	calls := 0
	got := shapeAPIRecords(t, params, input, opts, maskClearingEngine{calls: &calls})
	if calls == 0 {
		t.Fatal("SetupMasks was not invoked")
	}
	// The hook disables the first glyph only; the second remains enabled.
	assertAPIGlyphs(t, got, []ot.GlyphIndex{18, 20})
	if got[0].Mask != 0 {
		t.Errorf("first glyph mask = %#x, want the hook's zero mask", got[0].Mask)
	}
}

func TestKnownBugSHAPE002FeatureRangeUsesInputCoordinates(t *testing.T) {
	params, cp := alternateAPIInput(t)
	params.Features[0].Start, params.Features[0].End = 2, 3
	input := strings.Repeat(cp, 4)
	for _, tc := range []struct {
		name string
		opts otshape.BufferOptions
	}{
		{"whole_input", otshape.BufferOptions{HighWatermark: 8, LowWatermark: 4, MaxBuffer: 8}},
		{"streaming", otshape.BufferOptions{HighWatermark: 2, LowWatermark: 1, MaxBuffer: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := shapeAPIRecords(t, params, input, tc.opts, otcore.New())
			// A codepoint range refers to the original input in both configurations.
			assertAPIGlyphs(t, got, []ot.GlyphIndex{18, 18, 20, 18})
		})
	}
}

func TestKnownBugSHAPE003FeatureValueSelectsAlternate(t *testing.T) {
	params, cp := alternateAPIInput(t)
	// The paired TTX lists g20, g21, g22 as g18's alternatives.
	// Intended public convention: 0 while On means default (first), and positive
	// values select the one-based alternative. Lower-level ApplyFeature uses a
	// separate zero-based alt parameter; the public pipeline must translate it.
	for _, tc := range []struct {
		arg  int
		want ot.GlyphIndex
	}{
		{0, 20}, {1, 20}, {2, 21}, {3, 22},
	} {
		t.Run(fmt.Sprintf("arg_%d", tc.arg), func(t *testing.T) {
			p := params
			p.Features = []otshape.FeatureRange{{Feature: ot.T("test"), On: true, Arg: tc.arg}}
			got := shapeAPIRecords(t, p, cp, otshape.BufferOptions{}, otcore.New())
			assertAPIGlyphs(t, got, []ot.GlyphIndex{tc.want})
		})
	}
}

type confidenceEngine struct {
	name  string
	score otshape.ShaperConfidence
}

func (e confidenceEngine) Name() string { return e.name }
func (e confidenceEngine) Match(otshape.SelectionContext) otshape.ShaperConfidence {
	return e.score
}
func (e confidenceEngine) New() otshape.ShapingEngine { return e }

func TestKnownBugSHAPE004RejectNonmatchingEngines(t *testing.T) {
	params, cp := alternateAPIInput(t)
	t.Run("positive_confidence_control", func(t *testing.T) {
		got := shapeAPIRecords(t, params, cp, otshape.BufferOptions{},
			confidenceEngine{name: "match", score: otshape.ShaperConfidenceLow})
		assertAPIGlyphs(t, got, []ot.GlyphIndex{20})
	})
	for _, tc := range []struct {
		name    string
		engines []otshape.ShapingEngine
	}{
		{"single_none", []otshape.ShapingEngine{confidenceEngine{name: "none"}}},
		{"all_none", []otshape.ShapingEngine{confidenceEngine{name: "b"}, confidenceEngine{name: "a"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.NewReader(cp)
			sink := &glyphCollector{}
			err := otshape.NewShaper(tc.engines...).Shape(params, src, sink, otshape.BufferOptions{})
			if !errors.Is(err, otshape.ErrNoMatchingShaper) {
				t.Errorf("Shape error = %v, want ErrNoMatchingShaper", err)
			}
			if len(sink.glyphs) != 0 {
				t.Errorf("nonmatching engines emitted %d glyphs, want none", len(sink.glyphs))
			}
			if src.Len() != len(cp) {
				t.Errorf("engine selection consumed input: %d bytes remain, want %d", src.Len(), len(cp))
			}
		})
	}
}
