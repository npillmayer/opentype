package otshape

import (
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestDeletionPreservesSurvivorMetadata(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		font := loadMiniOTFont(t, "gsub_context1_lookupflag_f1.otf")
		graph := font.Layout.GSub.LookupGraph()
		installSuccessLookup(graph, 0, ot.GSubLookupTypeMultiple, 10, &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{}}}})
		installSuccessLookup(graph, 1, ot.GSubLookupTypeSingle, 20, &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}})
		run := newRunBuffer(0)
		run.Glyphs = otlayout.GlyphBuffer{10, 10, 20, 10, 20, 10}
		run.Codepoints = []rune{'d', 'd', 'a', 'd', 'a', 'd'}
		run.Clusters = []uint32{0, 1, 2, 3, 4, 5}
		run.SourceAnchors = []uint32{10, 11, 12, 13, 14, 15}
		run.Masks = []uint32{0, 0, 2, 0, 0, 0}
		run.PlanIDs = []uint16{0, 1, 2, 3, 4, 5}
		run.UnsafeFlags = []uint16{1, 2, 3, 4, 5, 6}
		run.Syllables = []uint16{1, 1, 1, 1, 2, 2}
		run.Joiners = []uint8{0, 0, 1, 0, 2, 0}
		run.Pos = otlayout.NewPosBuffer(6)
		run.Pos[2].XAdvance, run.Pos[4].XAdvance = 17, 29
		pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
		exec := &planExecutor{}
		exec.acquireBuffer(run)
		flags := lookupRunFlags(0)
		if isolated {
			flags = lookupPerSyllable
		}
		err := exec.applyLookups(pl, planGSUB, []lookupOp{{LookupIndex: 0, Flags: flags}, {LookupIndex: 1, Mask: 2, Flags: flags}})
		exec.releaseBuffer()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(run.Glyphs, otlayout.GlyphBuffer{21, 20}) || !reflect.DeepEqual(run.Masks, []uint32{2, 0}) || !reflect.DeepEqual(run.Clusters, []uint32{0, 4}) || !reflect.DeepEqual(run.SourceAnchors, []uint32{12, 14}) || !reflect.DeepEqual(run.PlanIDs, []uint16{2, 4}) || !reflect.DeepEqual(run.UnsafeFlags, []uint16{3, 5}) || !reflect.DeepEqual(run.Syllables, []uint16{1, 2}) || !reflect.DeepEqual(run.Joiners, []uint8{1, 2}) || !reflect.DeepEqual(run.Codepoints, []rune{'a', 'a'}) {
			t.Fatalf("isolated=%v: correspondence lost: %+v", isolated, run)
		}
		if len(run.Pos) != 2 || run.Pos[0].XAdvance != 17 || run.Pos[1].XAdvance != 29 {
			t.Fatalf("positions=%v", run.Pos)
		}
	}
}

func TestDeletionClusterMerging(t *testing.T) {
	for _, tc := range []struct {
		name     string
		clusters []uint32
		index    int
		want     []uint32
	}{
		{"leading_group", []uint32{0, 2, 2}, 0, []uint32{0, 0}},
		{"backward_group", []uint32{5, 5, 2, 8}, 2, []uint32{2, 2, 8}},
		{"surviving_group", []uint32{2, 5, 5}, 1, []uint32{2, 5}},
		{"trailing", []uint32{0, 2}, 1, []uint32{0}},
		{"all", []uint32{0}, 0, []uint32{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := newRunBuffer(0)
			run.Clusters = tc.clusters
			run.applyLayoutEdit(otlayout.EditSpan{From: tc.index, To: tc.index + 1, Len: 0, ResetsPositions: true})
			if !reflect.DeepEqual(run.Clusters, tc.want) {
				t.Fatalf("clusters=%v want=%v", run.Clusters, tc.want)
			}
		})
	}
}
