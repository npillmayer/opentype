package otshape

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestContextProgressionExecutesNonoverlappingInputs(t *testing.T) {
	for _, mode := range []string{"single", "expansion", "positioning"} {
		for _, isolated := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/isolated=%t", mode, isolated), func(t *testing.T) {
				phase := planGSUB
				file := "gsub_context1_lookupflag_f1.otf"
				if mode == "positioning" {
					file = "gpos_chaining3_boundary_f2.otf"
					phase = planGPOS
				}
				font := loadMiniOTFont(t, file)
				var graph *ot.LookupListGraph
				if mode == "positioning" {
					graph = font.Layout.GPos.LookupGraph()
				} else {
					graph = font.Layout.GSub.LookupGraph()
				}
				parent, child := graph.Lookup(0), graph.Lookup(1)
				pn, cn := parent.Subtable(0), child.Subtable(0)
				parent.Flag, child.Flag = 0, 0
				cov := ot.Coverage{GlyphRange: successCoverage(10)}
				records := []ot.SequenceLookupRecord{{LookupListIndex: 1}}
				if mode == "positioning" {
					parent.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeContextPos)
					*pn = ot.LookupNode{LookupType: parent.Type, Format: 3, GPos: &ot.GPosLookupPayload{ContextFmt3: &ot.GPosContextFmt3Payload{InputCoverages: []ot.Coverage{cov, cov}, Records: records}}}
					child.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeSingle)
					*cn = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov, GPos: &ot.GPosLookupPayload{SingleFmt1: &ot.GPosSingleFmt1Payload{ValueFormat: 4, Value: ot.ValueRecord{XAdvance: 1}}}}
				} else {
					parent.Type = ot.GSubLookupTypeContext
					*pn = ot.LookupNode{LookupType: parent.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov, cov}, Records: records}}}
					installSuccessLookup(graph, 1, ot.GSubLookupTypeSingle, 10, &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 10}})
					if mode == "expansion" {
						installSuccessLookup(graph, 1, ot.GSubLookupTypeMultiple, 10, &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12}}}})
					}
				}
				run := newRunBuffer(0)
				run.Glyphs = otlayout.GlyphBuffer{10, 10, 10, 10, 10}
				run.Pos = otlayout.NewPosBuffer(5)
				run.Clusters = []uint32{0, 1, 2, 3, 4}
				run.Syllables = []uint16{1, 1, 1, 1, 1}
				pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
				e := &planExecutor{}
				e.acquireBuffer(run)
				defer e.releaseBuffer()
				flags := lookupRunFlags(0)
				if isolated {
					flags = lookupPerSyllable
				}
				if err := e.applyLookups(pl, phase, []lookupOp{{LookupIndex: 0, FeatureTag: ot.T("test"), Flags: flags}}); err != nil {
					t.Fatal(err)
				}
				want := otlayout.GlyphBuffer{20, 10, 20, 10, 10}
				clusters := []uint32{0, 1, 2, 3, 4}
				if mode == "expansion" {
					want = otlayout.GlyphBuffer{11, 12, 10, 11, 12, 10, 10}
					clusters = []uint32{0, 0, 1, 2, 2, 3, 4}
				}
				if mode == "positioning" {
					want = otlayout.GlyphBuffer{10, 10, 10, 10, 10}
				}
				wantPos := otlayout.NewPosBuffer(len(want))
				if mode == "positioning" {
					wantPos[0].XAdvance = 1
					wantPos[2].XAdvance = 1
				}
				if !reflect.DeepEqual(run.Glyphs, want) || !reflect.DeepEqual(run.Clusters, clusters) || !reflect.DeepEqual(run.Pos, wantPos) {
					t.Fatalf("glyphs=%v clusters=%v positions=%v; want %v %v %v", run.Glyphs, run.Clusters, run.Pos, want, clusters, wantPos)
				}
			})
		}
	}
}
