package otcore_test

import (
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

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

// SHAPE-001: engine mask decisions must survive entry into the executor.
func TestSHAPE001SetupMasksPreserved(t *testing.T) {
	for _, api := range []string{"Shape", "ShapeEvents"} {
		t.Run(api, func(t *testing.T) {
			params, cp := alternateAPIInput(t)
			input := strings.Repeat(cp, 2)
			opts := otshape.BufferOptions{HighWatermark: 8, LowWatermark: 4, MaxBuffer: 8}
			shape := func(engine otshape.ShapingEngine) []otshape.GlyphRecord {
				t.Helper()
				shaper := otshape.NewShaper(engine)
				src := strings.NewReader(input)
				sink := &glyphCollector{}
				var err error
				if api == "ShapeEvents" {
					err = shaper.ShapeEvents(params, otshape.NewInputEventSource(src), sink, opts)
				} else {
					err = shaper.Shape(params, src, sink, opts)
				}
				if err != nil {
					t.Fatalf("%s: %v", api, err)
				}
				return sink.glyphs
			}

			// Control: the fixture's enabled lookup substitutes both glyphs.
			control := shape(otcore.New())
			assertAPIGlyphs(t, control, []ot.GlyphIndex{20, 20})

			calls := 0
			got := shape(maskClearingEngine{calls: &calls})
			if calls == 0 {
				t.Fatal("SetupMasks was not invoked")
			}
			// The hook disables the first glyph only; the second remains enabled.
			assertAPIGlyphs(t, got, []ot.GlyphIndex{18, 20})
			if got[0].Mask != 0 {
				t.Errorf("first glyph mask = %#x, want the hook's zero mask", got[0].Mask)
			}
			if got[1].Mask == 0 || got[1].Mask != control[1].Mask {
				t.Errorf("second glyph mask = %#x, want unchanged enabled mask %#x", got[1].Mask, control[1].Mask)
			}
		})
	}
}

func TestSHAPE002FeatureRangeUsesInputCoordinates(t *testing.T) {
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

func TestSHAPE011FeatureRangesSelectInitialClusters(t *testing.T) {
	for _, tc := range []struct {
		name        string
		start, end  int
		first, last ot.GlyphIndex
	}{
		{"start_inside", 1, 3, 18, 20},
		{"end_inside", 0, 1, 20, 18},
		{"mark_only", 1, 2, 18, 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params, cp := alternateAPIInput(t)
			params.Features[0].Start, params.Features[0].End = tc.start, tc.end
			input := cp + "\u0301" + cp
			mark, err := otquery.GlyphIndex(params.Font, '\u0301')
			if err != nil {
				t.Fatal(err)
			}
			shaper := otshape.NewShaper(otcore.New())
			sink := &glyphCollector{}
			src := strings.NewReader(input)
			opts := otshape.BufferOptions{HighWatermark: 8, LowWatermark: 4, MaxBuffer: 8}
			err = shaper.Shape(params, src, sink, opts)
			if err != nil {
				t.Fatal(err)
			}
			wantIDs := []ot.GlyphIndex{tc.first, mark, tc.last}
			wantClusters := []uint32{0, 0, 2}
			if len(sink.glyphs) != 3 {
				t.Fatalf("glyphs=%+v", sink.glyphs)
			}
			for i, g := range sink.glyphs {
				if g.GID != wantIDs[i] || g.Cluster != wantClusters[i] {
					t.Errorf("glyph[%d]=%+v, want %d/%d", i, g, wantIDs[i], wantClusters[i])
				}
			}
			if sink.glyphs[0].Mask != sink.glyphs[1].Mask {
				t.Fatal("base and mark did not share range selection")
			}
		})
	}
}
