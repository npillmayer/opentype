package otshape

import (
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"golang.org/x/text/unicode/bidi"
)

func TestMarkAttachmentChainAndLookupOrder(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		t.Run(map[bidi.Direction]string{bidi.LeftToRight: "LTR", bidi.RightToLeft: "RTL"}[dir], func(t *testing.T) {
			font := loadMiniOTFont(t, "gpos4_simple_1.otf")
			lookup := font.Layout.GPos.LookupGraph().Lookup(0)
			node := lookup.Subtable(0)
			p := node.GPos.MarkToBaseFmt1
			run := newRunBuffer(0)
			run.Glyphs = otlayout.GlyphBuffer{18, 19, 19}
			run.Pos = otlayout.NewPosBuffer(3)
			run.Pos[0].XOffset, run.Pos[0].YOffset = 7, 9
			pl := &plan{font: font, ScriptTag: ot.T("latn"), Props: segmentProps{Direction: dir},
				Policy: planPolicy{ApplyGPOS: true, ZeroGDEFMarks: true}, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
			e := &planExecutor{}
			e.acquireBuffer(run)
			defer e.releaseBuffer()
			op := []lookupOp{{LookupIndex: 0, FeatureTag: ot.T("test")}}
			if err := e.applyLookups(pl, planGPOS, op); err != nil {
				t.Fatal(err)
			}
			// The second mark now attaches to the first. Reuse the parsed
			// anchor matrix with a synthetic mark2 coverage, retaining offsets.
			lookup.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeMarkToMark)
			lookup.Flag = 0
			node.LookupType = lookup.Type
			node.GPos = &ot.GPosLookupPayload{MarkToMarkFmt1: &ot.GPosMarkToMarkFmt1Payload{
				Mark2Coverage: ot.Coverage{GlyphRange: successCoverage(19)}, MarkClassCount: p.MarkClassCount,
				Mark1Records: p.MarkRecords, Mark2Records: p.BaseRecords,
			}}
			if err := e.applyLookups(pl, planGPOS, op); err != nil {
				t.Fatal(err)
			}
			if run.Pos[2].AttachTo != 1 || run.Pos[2].AttachKind != otlayout.AttachMarkToMark {
				t.Fatalf("missing chain: %+v", run.Pos)
			}
			// Later positioning changes the base X and advance, and adds to the
			// first mark. Its later Y change must not move the already attached
			// second mark. The base's later Y change must not move either mark.
			run.Pos[0].XOffset, run.Pos[0].YOffset, run.Pos[0].XAdvance = 12, 29, 25
			run.Pos[1].XOffset += 3
			run.Pos[1].YOffset += 4
			e.applyPositionPolicies(pl, true)
			if err := e.finishMarkAttachments(pl); err != nil {
				t.Fatal(err)
			}
			wantX := []int32{-1610, -1710} // 12-100+3-1525; then another -100
			if dir == bidi.RightToLeft {
				wantX = []int32{-85, -185}
			} // marks precede base visually, with zero advances
			wantY := []int32{-67, -151} // 9-80+4; (9-80)-80 captured before +4
			for i := 1; i < 3; i++ {
				p := run.Pos[i]
				if p.XOffset != wantX[i-1] || p.YOffset != wantY[i-1] || p.MarkAttachmentPending || p.XAdvance != -1500 {
					t.Fatalf("mark[%d]=%+v want offsets %d,%d", i, p, wantX[i-1], wantY[i-1])
				}
			}
			before := append(otlayout.PosBuffer(nil), run.Pos...)
			if err := e.finishMarkAttachments(pl); err != nil || !reflect.DeepEqual(before, run.Pos) {
				t.Fatalf("second finish changed offsets: %v", err)
			}
			if !reflect.DeepEqual(run.Glyphs, otlayout.GlyphBuffer{18, 19, 19}) {
				t.Fatal("finalization changed logical order")
			}
		})
	}
}

func TestMarkAttachmentPenCompensation(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		font := loadMiniOTFont(t, "gpos4_simple_1.otf")
		run := newRunBuffer(0)
		run.Glyphs = otlayout.GlyphBuffer{18, 17, 19}
		run.Pos = otlayout.NewPosBuffer(3)
		// Final advances are (200,5), (100,7), (50,11). This deliberately
		// retains a mark advance to verify the distinct RTL interval (j,i].
		run.Pos[0].XAdvance, run.Pos[0].YAdvance, run.Pos[0].XOffset = -1300, 5, 3
		run.Pos[1].XAdvance, run.Pos[1].YAdvance = -1400, 7
		run.Pos[2] = otlayout.PosItem{XAdvance: -1450, YAdvance: 11, XOffset: 10, YOffset: 20,
			AttachTo: 0, AttachKind: otlayout.AttachMarkToBase, MarkAttachmentPending: true}
		e := &planExecutor{run: run}
		pl := &plan{font: font, ScriptTag: ot.T("hebr"), Policy: planPolicy{ApplyGPOS: true}, Props: segmentProps{Direction: dir}}
		if err := e.finishMarkAttachments(pl); err != nil {
			t.Fatal(err)
		}
		wantX, wantY := int32(-287), int32(8)
		if dir == bidi.RightToLeft {
			wantX, wantY = 163, 38
		}
		if p := run.Pos[2]; p.XOffset != wantX || p.YOffset != wantY {
			t.Fatalf("dir=%v pos=%+v want=%d,%d", dir, p, wantX, wantY)
		}
	}
}

func TestMarkAttachmentLongChainAndInvalidTarget(t *testing.T) {
	font := loadMiniOTFont(t, "gpos4_simple_1.otf")
	run := newRunBuffer(0)
	run.Glyphs = make(otlayout.GlyphBuffer, 513)
	run.Pos = otlayout.NewPosBuffer(513)
	for i := range run.Glyphs {
		run.Glyphs[i] = 18
		if i > 0 {
			run.Pos[i] = otlayout.PosItem{XAdvance: -1500, XOffset: 1, AttachTo: int32(i - 1), AttachKind: otlayout.AttachMarkToMark, MarkAttachmentPending: true}
		}
	}
	e := &planExecutor{run: run}
	pl := &plan{font: font, ScriptTag: ot.T("latn"), Policy: planPolicy{ApplyGPOS: true}}
	if err := e.finishMarkAttachments(pl); err != nil {
		t.Fatal(err)
	}
	if p := run.Pos[512]; p.XOffset != -988 || p.MarkAttachmentPending {
		t.Fatalf("long chain: %+v", p)
	}
	for _, target := range []int32{-1, 1, 512, 999} {
		run.Pos[1].MarkAttachmentPending, run.Pos[1].AttachTo = true, target
		before := append(otlayout.PosBuffer(nil), run.Pos...)
		if err := e.finishMarkAttachments(pl); err == nil || !reflect.DeepEqual(before, run.Pos) {
			t.Fatalf("invalid target %d: err=%v", target, err)
		}
	}
}
