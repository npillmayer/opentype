package othebrew_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/othebrew"
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

func TestStreamingParityHebrewShaperMultiCycle(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	input := []rune(strings.Repeat("\u05D0\u05D1\u05D2\u05D3\u05D4", 12))

	base := shapeHebrewWithConfig(t, font, input, otshape.FlushOnRunBoundary, 0, 0, 0)
	stream := shapeHebrewWithConfig(t, font, input, otshape.FlushOnRunBoundary, 4, 2, 24)
	if !reflect.DeepEqual(stream, base) {
		t.Fatalf("streaming output differs from baseline:\nstream=%#v\nbase=%#v", stream, base)
	}
}

func shapeHebrewWithConfig(
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
		Script:    language.MustParseScript("Hebr"),
		Language:  language.Hebrew,
	}
	options := otshape.BufferOptions{
		FlushBoundary: boundary,
		HighWatermark: high,
		LowWatermark:  low,
		MaxBuffer:     max,
	}
	engines := []otshape.ShapingEngine{othebrew.New()}
	shaper := otshape.NewShaper(engines...)
	err := shaper.Shape(params, source, sink, options)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	return sink.glyphs
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
