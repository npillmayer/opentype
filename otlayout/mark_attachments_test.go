package otlayout

import (
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestMarkAttachmentYOffsetSaturation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		targetY      int32
		markY, baseY int16
		want         int32
	}{
		{"positive overflow", math.MaxInt32 - 5, 0, 10, math.MaxInt32},
		{"negative overflow", math.MinInt32 + 5, 0, -10, math.MinInt32},
		{"exact maximum", math.MaxInt32 - 10, 0, 10, math.MaxInt32},
		{"exact minimum", math.MinInt32 + 10, 0, -10, math.MinInt32},
		{"below maximum", math.MaxInt32 - 11, 0, 10, math.MaxInt32 - 1},
		{"above minimum", math.MinInt32 + 11, 0, -10, math.MinInt32 + 1},
		{"full positive anchor difference", 0, math.MinInt16, math.MaxInt16, 65535},
		{"full negative anchor difference", 0, math.MaxInt16, math.MinInt16, -65535},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var pos PosItem
			// HarfBuzz's MarkArray application saturates the anchor difference
			// plus the target's cross-stream placement at attachment time.
			setMarkAttachment(&pos, 0, AttachMarkToBase, 0, AnchorRef{},
				&ot.Anchor{YCoordinate: tc.markY}, &ot.Anchor{YCoordinate: tc.baseY}, tc.targetY)
			if pos.YOffset != tc.want {
				t.Fatalf("YOffset=%d, want %d", pos.YOffset, tc.want)
			}
		})
	}
}

func TestMarkAnchorPlacement(t *testing.T) {
	for _, kind := range []AttachKind{AttachMarkToBase, AttachMarkToLigature, AttachMarkToMark} {
		t.Run(map[AttachKind]string{AttachMarkToBase: "base", AttachMarkToLigature: "ligature", AttachMarkToMark: "mark"}[kind], func(t *testing.T) {
			file := "gpos4_simple_1.otf"
			if kind == AttachMarkToLigature {
				file = "gpos5_font1.otf"
			}
			font := loadTestFont(t, file)
			graph := font.Layout.GPos.LookupGraph()
			lookup := graph.Lookup(0)
			node := lookup.Subtable(0)
			lookup.Flag = 0
			if kind == AttachMarkToMark {
				// Reuse parsed anchor records (including their private offsets) in
				// an isolated type-6 lookup; do not alter the font's backing bytes.
				p := node.GPos.MarkToBaseFmt1
				lookup.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeMarkToMark)
				node.LookupType = lookup.Type
				node.GPos = &ot.GPosLookupPayload{MarkToMarkFmt1: &ot.GPosMarkToMarkFmt1Payload{
					Mark2Coverage: p.BaseCoverage, MarkClassCount: p.MarkClassCount,
					Mark1Records: p.MarkRecords, Mark2Records: p.BaseRecords,
				}}
			}
			st := NewBufferState(GlyphBuffer{18, 19}, NewPosBuffer(2))
			st.Index = 1
			st.Pos[0].XOffset, st.Pos[0].YOffset = 41, 13
			st.Pos[1].XOffset, st.Pos[1].YOffset = 99, -88
			st.Pos[1].XAdvance = 7
			env, err := newApplyEnv(font)
			if err != nil {
				t.Fatal(err)
			}
			feat := errorTestFeature{GPosFeatureType, []int{0}}
			_, applied, _, err := applyLookupConcrete(lookup, graph, feat, st, 0, font.Layout.GDef, env)
			if err != nil || !applied {
				t.Fatalf("applied=%t err=%v", applied, err)
			}
			// TTX: mark (200,230), base (100,150); the last ligature
			// component has (101,151). Target Y placement is captured now.
			wantX, wantY := int32(-100), int32(-67)
			if kind == AttachMarkToLigature {
				wantX, wantY = -99, -66
			}
			p := st.Pos[1]
			if p.XOffset != wantX || p.YOffset != wantY || p.XAdvance != 7 ||
				p.AttachKind != kind || p.AttachTo != 0 || !p.MarkAttachmentPending || p.AnchorRef.MarkAnchor == 0 || p.AnchorRef.BaseAnchor == 0 {
				t.Fatalf("attachment=%+v want offsets %d,%d kind=%d", p, wantX, wantY, kind)
			}
			applyValueRecord(&st.Pos[1], ot.ValueRecord{XPlacement: 3, YPlacement: 4}, ot.ValueFormatXPlacement|ot.ValueFormatYPlacement)
			if st.Pos[1].XOffset != wantX+3 || st.Pos[1].YOffset != wantY+4 {
				t.Fatal("later placement was lost")
			}
			before := append(PosBuffer(nil), st.Pos...)
			st.Index = 0 // start has no mark coverage: no mutation
			_, applied, _, err = applyLookupConcrete(lookup, graph, feat, st, 0, font.Layout.GDef, env)
			if err != nil || applied || !reflect.DeepEqual(st.Pos, before) {
				t.Fatalf("nonmatch changed positions: %v", err)
			}
			// A later malformed operation must retain the completed attachment.
			st.Index = 1
			node.GPos = nil
			_, applied, _, err = applyLookupConcrete(lookup, graph, feat, st, 0, font.Layout.GDef, env)
			if err == nil || applied || !reflect.DeepEqual(st.Pos, before) {
				t.Fatalf("error lost completed placement: %v", err)
			}
		})
	}
}

func TestMarkAttachmentReplacementAndEdit(t *testing.T) {
	pb := NewPosBuffer(3)
	mark := &ot.Anchor{XCoordinate: 20, YCoordinate: 30}
	base := &ot.Anchor{XCoordinate: 100, YCoordinate: 150}
	setMarkAttachment(&pb[2], 0, AttachMarkToBase, 2, AnchorRef{MarkAnchor: 12}, mark, base, 10)
	setMarkAttachment(&pb[2], 1, AttachMarkToMark, 3, AnchorRef{MarkAnchor: 24}, mark, base, 50)
	if pb[2].XOffset != 80 || pb[2].YOffset != 170 || pb[2].AttachTo != 1 || pb[2].AnchorRef.MarkAnchor != 24 {
		t.Fatalf("replacement accumulated old attachment: %+v", pb[2])
	}
	shifted := pb.ApplyEdit(&EditSpan{From: 0, To: 0, Len: 1})
	if shifted[3].AttachTo != 2 || !shifted[3].MarkAttachmentPending {
		t.Fatal("pending attachment did not follow target")
	}
	deleted := shifted.ApplyEdit(&EditSpan{From: 2, To: 3, Len: 0})
	if deleted[2].AttachKind != AttachNone || deleted[2].MarkAttachmentPending {
		t.Fatal("deleted target left pending attachment")
	}
}
