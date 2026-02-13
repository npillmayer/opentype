package otcore_test

import (
	"os"
	"path/filepath"
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
	req := otshape.ShapeRequest{
		Options: otshape.ShapeOptions{
			Params: otshape.Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    latin,
				Language:  language.English,
				Features: []otshape.FeatureRange{
					{Feature: ot.T("test"), On: true},
				},
			},
			FlushBoundary: otshape.FlushOnRunBoundary,
		},
		Source:  src,
		Sink:    sink,
		Shapers: []otshape.ShapingEngine{otcore.New()},
	}
	if err := otshape.Shape(req); err != nil {
		t.Fatalf("shape failed: %v", err)
	}

	if len(sink.glyphs) != 1 {
		t.Fatalf("shaped glyph count = %d, want 1", len(sink.glyphs))
	}
	if got := sink.glyphs[0].GID; got != 20 {
		t.Fatalf("shaped glyph id = %d, want 20", got)
	}
}

func loadMiniOTFont(t *testing.T, filename string) *ot.Font {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fonttools-tests", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mini font %s: %v", path, err)
	}
	otf, err := ot.Parse(data, ot.IsTestfont)
	if err != nil {
		t.Fatalf("parse mini font %s: %v", path, err)
	}
	return otf
}
