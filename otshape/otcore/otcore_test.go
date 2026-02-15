package otcore_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

type glyphCollector struct {
	glyphs []otshape.GlyphRecord
}

func (c *glyphCollector) WriteGlyph(g otshape.GlyphRecord) error {
	c.glyphs = append(c.glyphs, g)
	return nil
}

func TestShapeAppliesGSUBFromCoreShaper(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "opentype.shaper")
	defer teardown()
	//
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	latin := language.MustParseScript("Latn")

	cp := otquery.CodePointForGlyph(font, 18)
	if cp == 0 {
		t.Skip("mini font does not expose a codepoint for glyph 18")
	}

	src := strings.NewReader(string(cp))
	sink := &glyphCollector{}
	params := otshape.Params{
		Font:      font,
		Direction: bidi.LeftToRight,
		Script:    latin,
		Language:  language.English,
		Features: []otshape.FeatureRange{
			{Feature: ot.T("test"), On: true},
		},
	}
	options := otshape.BufferOptions{
		FlushBoundary: otshape.FlushOnRunBoundary,
	}
	engines := []otshape.ShapingEngine{otcore.New()}
	shaper := otshape.NewShaper(engines...)
	err := shaper.Shape(params, src, sink, options)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}

	if len(sink.glyphs) != 1 {
		t.Fatalf("shaped glyph count = %d, want 1", len(sink.glyphs))
	}
	if got := sink.glyphs[0].GID; got != 20 {
		t.Fatalf("shaped glyph id = %d, want 20", got)
	}
}

func TestShapeFlushModesProduceSameOutput(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	input := []rune{0x12, 0x13}
	features := []otshape.FeatureRange{
		{Feature: ot.T("test"), On: true},
	}
	runOut := shapeRunesWithBoundary(t, font, input, features, otshape.FlushOnRunBoundary)
	clusterOut := shapeRunesWithBoundary(t, font, input, features, otshape.FlushOnClusterBoundary)
	if !reflect.DeepEqual(runOut, clusterOut) {
		t.Fatalf("flush modes produced different output:\nrun=%#v\ncluster=%#v", runOut, clusterOut)
	}
}
