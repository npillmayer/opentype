package otshape

import (
	"strings"
	"testing"

	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

type hookProbeSink struct {
	glyphs []GlyphRecord
}

func (s *hookProbeSink) WriteGlyph(g GlyphRecord) error {
	s.glyphs = append(s.glyphs, g)
	return nil
}

type hookProbeShaper struct {
	useCompose bool
	useReorder bool

	composeCalls int
	reorderCalls int
}

func (s *hookProbeShaper) Name() string { return "hook-probe" }

func (s *hookProbeShaper) Match(SelectionContext) ShaperConfidence {
	return ShaperConfidenceCertain
}

func (s *hookProbeShaper) New() ShapingEngine { return s }

func (s *hookProbeShaper) NormalizationPreference() NormalizationMode {
	return NormalizationComposed
}

func (s *hookProbeShaper) ApplyGPOS() bool {
	return true
}

func (s *hookProbeShaper) Compose(_ NormalizeContext, a, b rune) (rune, bool) {
	s.composeCalls++
	if !s.useCompose {
		return 0, false
	}
	if a == 0x12 && b == 0x13 {
		return 0x12, true
	}
	return 0, false
}

func (s *hookProbeShaper) ReorderMarks(run RunContext, start, end int) {
	s.reorderCalls++
	if !s.useReorder {
		return
	}
	if end-start >= 2 {
		run.Swap(start, start+1)
		run.MergeClusters(start, start+2)
	}
}

func TestShapeComposeHookCanCollapseRunePair(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	engine := &hookProbeShaper{useCompose: true}
	sink := &hookProbeSink{}

	err := Shape(ShapeRequest{
		Options: ShapeOptions{
			Params: Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    language.MustParseScript("Latn"),
				Language:  language.English,
			},
			FlushBoundary: FlushOnRunBoundary,
		},
		Source:  strings.NewReader(string([]rune{0x12, 0x13})),
		Sink:    sink,
		Shapers: []ShapingEngine{engine},
	})
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if engine.composeCalls == 0 {
		t.Fatalf("compose hook was not called")
	}
	if len(sink.glyphs) != 1 {
		t.Fatalf("glyph count = %d, want 1", len(sink.glyphs))
	}
	want := otquery.GlyphIndex(font, 0x12)
	if sink.glyphs[0].GID != want {
		t.Fatalf("composed glyph = %d, want %d", sink.glyphs[0].GID, want)
	}
}

func TestShapeReorderHookCanSwapRunItems(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	engine := &hookProbeShaper{useReorder: true}
	sink := &hookProbeSink{}

	err := Shape(ShapeRequest{
		Options: ShapeOptions{
			Params: Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    language.MustParseScript("Latn"),
				Language:  language.English,
			},
			FlushBoundary: FlushOnRunBoundary,
		},
		Source:  strings.NewReader(string([]rune{0x12, 0x13})),
		Sink:    sink,
		Shapers: []ShapingEngine{engine},
	})
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if engine.reorderCalls == 0 {
		t.Fatalf("reorder hook was not called")
	}
	if len(sink.glyphs) != 2 {
		t.Fatalf("glyph count = %d, want 2", len(sink.glyphs))
	}
	want0 := otquery.GlyphIndex(font, 0x13)
	want1 := otquery.GlyphIndex(font, 0x12)
	if sink.glyphs[0].GID != want0 || sink.glyphs[1].GID != want1 {
		t.Fatalf("reordered glyphs = [%d %d], want [%d %d]",
			sink.glyphs[0].GID, sink.glyphs[1].GID, want0, want1)
	}
	if sink.glyphs[0].Cluster != sink.glyphs[1].Cluster {
		t.Fatalf("cluster merge not applied: clusters = [%d %d]",
			sink.glyphs[0].Cluster, sink.glyphs[1].Cluster)
	}
}
