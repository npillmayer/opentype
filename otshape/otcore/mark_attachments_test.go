package otcore_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func TestMarkAttachmentPositionsPublicAPIs(t *testing.T) {
	for _, ligature := range []bool{false, true} {
		for _, events := range []bool{false, true} {
			for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
				t.Run(fmt.Sprintf("ligature=%t/events=%t/dir=%v", ligature, events, dir), func(t *testing.T) {
					file, kind, deltaX, deltaY := "gpos4_simple_1.otf", otlayout.AttachMarkToBase, int32(-100), int32(-80)
					if ligature {
						file, kind, deltaX, deltaY = "gpos5_font1.otf", otlayout.AttachMarkToLigature, -99, -79
					}
					font := loadMiniOTFont(t, file)
					params := otshape.Params{Font: font, Script: language.MustParseScript("Latn"), Language: language.English,
						Direction: dir, Features: []otshape.FeatureRange{{Feature: ot.T("test"), On: true}}}
					input := []rune{17, 18, 19, 17}
					sh := otshape.NewShaper(otcore.New())
					sink := &glyphCollector{}
					opts := otshape.BufferOptions{HighWatermark: 5, LowWatermark: 1, MaxBuffer: 6}
					var err error
					if events {
						err = sh.ShapeEvents(params, otshape.NewInputEventSource(strings.NewReader(string(input))), sink, opts)
					} else {
						err = sh.Shape(params, strings.NewReader(string(input)), sink, opts)
					}
					if err != nil {
						t.Fatal(err)
					}
					if len(sink.glyphs) != 4 {
						t.Fatalf("records=%+v", sink.glyphs)
					}
					// Anchor coordinates and 1500-unit nominal metrics are from the
					// checked-in TTX. RTL uses the zero mark advance, not the base
					// advance, and retains the same logical storage order.
					for i, g := range sink.glyphs {
						ax, dx, dy := int32(1500), int32(0), int32(0)
						if i == 2 {
							ax, dx, dy = 0, deltaX, deltaY
							if dir == bidi.LeftToRight {
								dx -= 1500
							}
							if g.Pos.AttachKind != kind || g.Pos.AttachTo != 1 {
								t.Fatalf("attachment=%+v", g.Pos)
							}
						}
						if g.GID != ot.GlyphIndex(input[i]) || g.Cluster != uint32(i) || g.Pos.XAdvance != ax || g.Pos.YAdvance != 0 ||
							g.Pos.XOffset != dx || g.Pos.YOffset != dy || g.Pos.MarkAttachmentPending {
							t.Fatalf("record[%d]=%+v want gid=%d cl=%d advance=%d,0 offset=%d,%d", i, g, input[i], i, ax, dx, dy)
						}
					}
				})
			}
		}
	}
}
