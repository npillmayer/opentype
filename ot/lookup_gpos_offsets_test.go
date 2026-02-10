package ot

import "testing"

func TestMarkAttachmentOffsetAccessors(t *testing.T) {
	t.Run("mark_to_ligature", func(t *testing.T) {
		p := &GPosMarkToLigatureFmt1Payload{
			MarkClassCount: 2,
			MarkRecords: []GPosMarkAttachRecord{
				{anchorOffset: 12},
			},
			LigatureRecords: []GPosLigatureAttachRecord{
				{
					componentAnchorOffsets: [][]uint16{
						{3, 5},
						{7, 9},
					},
				},
			},
		}
		markOff, baseOff, ok := p.AnchorOffsets(0, 0, 1, 0)
		if !ok || markOff != 12 || baseOff != 7 {
			t.Fatalf("unexpected mark-to-ligature offsets: mark=%d base=%d ok=%v", markOff, baseOff, ok)
		}
		if _, _, ok := p.AnchorOffsets(0, 0, 2, 0); ok {
			t.Fatalf("expected invalid component index to fail")
		}
	})

	t.Run("mark_to_mark", func(t *testing.T) {
		p := &GPosMarkToMarkFmt1Payload{
			MarkClassCount: 2,
			Mark1Records: []GPosMarkAttachRecord{
				{anchorOffset: 14},
			},
			Mark2Records: []GPosBaseAttachRecord{
				{anchorOffsets: []uint16{4, 8}},
			},
		}
		markOff, baseOff, ok := p.AnchorOffsets(0, 0, 1)
		if !ok || markOff != 14 || baseOff != 8 {
			t.Fatalf("unexpected mark-to-mark offsets: mark=%d base=%d ok=%v", markOff, baseOff, ok)
		}
		if _, _, ok := p.AnchorOffsets(0, 0, 2); ok {
			t.Fatalf("expected invalid class index to fail")
		}
	})
}
