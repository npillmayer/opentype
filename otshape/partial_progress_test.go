package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"reflect"
	"testing"
)

type partialProgressCoverage struct{}

func (partialProgressCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == 10 }
func (partialProgressCoverage) ByteSize() int                     { return 0 }

// Stage-5 contract test: a detached syllable buffer must not lose completed work.
func TestPartialProgressIsolatedSpanRetainsCompletedEdits(t *testing.T) {
	font := loadMiniOTFont(t, "gsub_context1_lookupflag_f1.otf")
	graph := font.Layout.GSub.LookupGraph()
	cov := ot.Coverage{GlyphRange: partialProgressCoverage{}}
	l := graph.Lookup(0)
	n := l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeContext, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}, {LookupListIndex: 65535}}}}}
	l = graph.Lookup(1)
	n = l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeMultiple, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12, 13}}}}}
	for _, isolated := range []bool{false, true} {
		t.Run(map[bool]string{false: "whole span", true: "isolated span"}[isolated], func(t *testing.T) {
			run := newRunBuffer(0)
			run.Glyphs = otlayout.GlyphBuffer{30, 10, 20}
			run.Clusters = []uint32{0, 1, 2}
			run.PlanIDs = []uint16{7, 7, 7}
			e := &planExecutor{run: run}
			pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
			st := otlayout.NewBufferState(run.Glyphs, nil)
			feat := planLookupFeature{tag: ot.T("test"), typ: otlayout.GSubFeatureType, lookupInx: 0}
			var err error
			if isolated {
				_, err = e.applyLookupIsolatedSpan(pl, lookupOp{}, feat, st, 0, 1, 2)
			} else {
				_, err = e.applyLookupSpan(pl, lookupOp{}, feat, st, 0, 0, st.Len(), 0)
			}
			if err == nil {
				t.Fatal("expected nested reference error")
			}
			want := otlayout.GlyphBuffer{30, 11, 12, 13, 20}
			if !reflect.DeepEqual(st.Glyphs, want) {
				t.Errorf("lost completed edit: got %v want %v", st.Glyphs, want)
			}
			if !reflect.DeepEqual(run.Glyphs, want) {
				t.Errorf("stale run slice: got %v want %v", run.Glyphs, want)
			}
			if len(run.Clusters) != len(want) || len(run.PlanIDs) != len(want) {
				t.Errorf("stale side arrays: clusters=%v plan IDs=%v", run.Clusters, run.PlanIDs)
			}
		})
	}
}
