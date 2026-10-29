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

func TestGDEFMarkAdvancesPublicAPIs(t *testing.T) {
	for _, script := range []string{"Latn", "Hebr", "Arab"} {
		for _, events := range []bool{false, true} {
			for _, attached := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/events=%t/attached=%t", script, events, attached), func(t *testing.T) {
					font := loadMiniOTFont(t, "gpos4_simple_1.otf")
					input := []rune{17, 17, 19, 17}
					if attached {
						input[1] = 18
					}
					direction := bidi.LeftToRight
					if script != "Latn" {
						direction = bidi.RightToLeft
					}
					params := otshape.Params{Font: font, Script: language.MustParseScript(script), Language: language.English, Direction: direction, Features: []otshape.FeatureRange{{Feature: ot.T("test"), On: true}}}
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
					for i, g := range sink.glyphs {
						want := int32(1500)
						if i == 2 && script != "Arab" {
							want = 0
						}
						if g.GID != ot.GlyphIndex(input[i]) || g.Pos.XAdvance != want || g.Pos.YAdvance != 0 {
							t.Fatalf("record[%d]=%+v want GID %d advances %d/0", i, g, input[i], want)
						}
					}
					// The Latin fixture activates its attachment lookup. Other scripts here
					// verify policy selection and final metrics, not script-specific layout.
					if script == "Latn" {
						want := otlayout.AttachNone
						to := int32(-1)
						if attached {
							want = otlayout.AttachMarkToBase
							to = 1
						}
						if p := sink.glyphs[2].Pos; p.AttachKind != want || p.AttachTo != to {
							t.Fatalf("attachment=%+v want kind=%d to=%d", p, want, to)
						}
					}
				})
			}
		}
	}
}
