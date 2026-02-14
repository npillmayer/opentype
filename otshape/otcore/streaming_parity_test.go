package otcore_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func TestStreamingParityCoreShaperMultiCycle(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	cp := otquery.CodePointForGlyph(font, 18)
	if cp == 0 {
		t.Skip("mini font does not expose a codepoint for glyph 18")
	}
	input := strings.Repeat(string(cp), 48)
	features := []otshape.FeatureRange{
		{Feature: ot.T("test"), On: true},
	}

	base := shapeCoreWithConfig(t, font, []rune(input), features, otshape.FlushOnRunBoundary, 0, 0, 0)
	stream := shapeCoreWithConfig(t, font, []rune(input), features, otshape.FlushOnRunBoundary, 5, 3, 32)
	if !reflect.DeepEqual(stream, base) {
		t.Fatalf("streaming output differs from baseline:\nstream=%#v\nbase=%#v", stream, base)
	}
}

func shapeCoreWithConfig(
	t *testing.T,
	font *ot.Font,
	runes []rune,
	features []otshape.FeatureRange,
	boundary otshape.FlushBoundary,
	high int,
	low int,
	max int,
) []otshape.GlyphRecord {
	t.Helper()
	sink := &glyphCollector{}
	req := otshape.ShapeRequest{
		Options: otshape.ShapeOptions{
			Params: otshape.Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    language.MustParseScript("Latn"),
				Language:  language.English,
				Features:  features,
			},
			FlushBoundary: boundary,
			HighWatermark: high,
			LowWatermark:  low,
			MaxBuffer:     max,
		},
		Source:  strings.NewReader(string(runes)),
		Sink:    sink,
		Shapers: []otshape.ShapingEngine{otcore.New()},
	}
	if err := otshape.Shape(req); err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	return sink.glyphs
}
