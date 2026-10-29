package otshape

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

type markPolicyProbe struct {
	hookProbeShaper
	gpos bool
}

func (p *markPolicyProbe) ApplyGPOS() bool { return p.gpos }

func TestGDEFMarkAdvancePlanScope(t *testing.T) {
	font := loadMiniOTFont(t, "gpos4_simple_1.otf")
	for _, script := range []string{"Latn", "Hebr", "Arab", "Deva", "Zyyy"} {
		for _, gpos := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/gpos=%t", script, gpos), func(t *testing.T) {
				params := standardParams(font)
				params.Script = language.MustParseScript(script)
				ctx := SelectionContext{Script: params.Script, ScriptTag: ScriptTagForScript(params.Script), Language: params.Language, Direction: bidi.LeftToRight}
				pl, err := compileShapePlanWithFeatures(params, ctx, &markPolicyProbe{gpos: gpos}, nil)
				if err != nil {
					t.Fatal(err)
				}
				want := gpos && (script == "Latn" || script == "Hebr")
				if pl.Policy.ZeroGDEFMarks != want {
					t.Fatalf("ZeroGDEFMarks=%t want=%t", pl.Policy.ZeroGDEFMarks, want)
				}
			})
		}
	}
}

func TestGDEFMarkAdvanceCancellation(t *testing.T) {
	for _, classification := range []string{"GDEF", "absent-GDEF", "absent-class-table"} {
		for _, gpos := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/gpos=%t", classification, gpos), func(t *testing.T) {
				font := loadMiniOTFont(t, "gpos4_simple_1.otf")
				if c := font.Layout.GDef.GlyphClassDef.Lookup(19); c != 3 {
					t.Fatalf("fixture GID 19 class=%d, want mark", c)
				}
				if c := font.Layout.GDef.GlyphClassDef.Lookup(25); c != 0 {
					t.Fatalf("fixture GID 25 class=%d, want unclassified", c)
				}
				switch classification {
				case "absent-GDEF":
					font.Layout.GDef = nil
				case "absent-class-table":
					font.Layout.GDef = &ot.GDefTable{}
				}
				run := newRunBuffer(0)
				run.Glyphs = otlayout.GlyphBuffer{25, 19, 19, 18}
				run.Pos = otlayout.NewPosBuffer(4)
				// Neither a Unicode mark nor attachment metadata alone opts a glyph in.
				run.Codepoints = []rune{'\u0301', '\u0013', '\u0013', 'A'}
				run.Pos[0].AttachKind = otlayout.AttachMarkToBase
				run.Pos[0].AttachTo = 3
				run.Pos[0].XAdvance = 70
				run.Pos[0].YAdvance = 3
				run.Pos[1].XAdvance = 80
				run.Pos[1].YAdvance = 12
				run.Pos[1].XOffset = 24
				run.Pos[1].YOffset = -9
				run.Pos[1].AttachKind = otlayout.AttachMarkToBase
				run.Pos[1].AttachTo = 3
				run.Pos[1].AttachClass = 2
				run.Pos[1].AnchorRef.MarkAnchor = 42
				run.Pos[2].XAdvance = -40
				run.Pos[2].YAdvance = -8
				run.Pos[2].XOffset = -17
				run.Pos[2].YOffset = 21
				run.Pos[3].XAdvance = 100
				before := append(otlayout.PosBuffer(nil), run.Pos...)
				want := append(otlayout.PosBuffer(nil), before...)
				zero := classification == "GDEF" && gpos
				if zero {
					for _, i := range []int{1, 2} {
						want[i].XAdvance = -1500
						want[i].YAdvance = 0
					}
				}
				e := &planExecutor{}
				e.acquireBuffer(run)
				defer e.releaseBuffer()
				pl := &plan{font: font, Policy: planPolicy{ZeroGDEFMarks: true}}
				for repeat := 0; repeat < 2; repeat++ {
					e.applyPositionPolicies(pl, gpos)
					if !reflect.DeepEqual(run.Pos, want) {
						t.Fatalf("positions=%+v want=%+v", run.Pos, want)
					}
					for i := range run.Glyphs {
						record := materializeGlyphRecord(run, i, font, true, false, false, false)
						expected := before[i]
						expected.XAdvance += 1500
						if zero && (i == 1 || i == 2) {
							expected.XAdvance = 0
							expected.YAdvance = 0
						}
						if record.Pos != expected {
							t.Fatalf("output[%d]=%+v want=%+v", i, record.Pos, expected)
						}
					}
				}
			})
		}
	}
}
