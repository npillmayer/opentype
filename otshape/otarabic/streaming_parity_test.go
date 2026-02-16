package otarabic_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otarabic"
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

func TestStreamingParityArabicShaperMultiCycle(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	// Arabic-Indic digits are non-joining; this fixture isolates streaming
	// cycle behavior from joining-form context effects.
	input := []rune(strings.Repeat("\u0661\u0662\u0663\u0664\u0665", 12))

	base := shapeArabicWithConfig(t, font, input, otshape.FlushOnRunBoundary, 0, 0, 0)
	stream := shapeArabicWithConfig(t, font, input, otshape.FlushOnRunBoundary, 4, 2, 24)
	if !reflect.DeepEqual(stream, base) {
		t.Fatalf("streaming output differs from baseline:\nstream=%#v\nbase=%#v", stream, base)
	}
}

func shapeArabicWithConfig(
	t *testing.T,
	font *ot.Font,
	runes []rune,
	boundary otshape.FlushBoundary,
	high int,
	low int,
	max int,
) []otshape.GlyphRecord {
	t.Helper()
	source := strings.NewReader(string(runes))
	sink := &glyphCollector{}
	params := otshape.Params{
		Font:      font,
		Direction: bidi.RightToLeft,
		Script:    language.MustParseScript("Arab"),
		Language:  language.Arabic,
	}
	options := otshape.BufferOptions{
		FlushBoundary: boundary,
		HighWatermark: high,
		LowWatermark:  low,
		MaxBuffer:     max,
	}
	engines := []otshape.ShapingEngine{otarabic.New()}
	shaper := otshape.NewShaper(engines...)
	err := shaper.Shape(params, source, sink, options)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	return sink.glyphs
}

func loadMiniOTFont(t *testing.T, filename string) *ot.Font {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fonttools", filename)
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
