package otshape

import (
	"fmt"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"reflect"
	"strings"
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
			run.Pos = otlayout.NewPosBuffer(3)
			run.Pos[0].AttachTo = 2
			run.Pos[2].AttachTo = 0
			run.Pos[2].XAdvance = 123
			run.Clusters = []uint32{0, 1, 2}
			run.PlanIDs = []uint16{7, 7, 7}
			e := &planExecutor{run: run}
			pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
			st := otlayout.NewBufferState(run.Glyphs, run.Pos)
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
			if len(run.Pos) != 5 || run.Pos[0].AttachTo != 4 || run.Pos[4].AttachTo != 0 || run.Pos[4].XAdvance != 123 {
				t.Fatalf("position metadata lost: %v", run.Pos)
			}
			if !reflect.DeepEqual(run.Glyphs, want) {
				t.Errorf("stale run slice: got %v want %v", run.Glyphs, want)
			}
			if !reflect.DeepEqual(run.Clusters, []uint32{0, 1, 1, 1, 2}) || !reflect.DeepEqual(run.PlanIDs, []uint16{7, 7, 7, 7, 7}) {
				t.Errorf("stale side arrays: clusters=%v plan IDs=%v", run.Clusters, run.PlanIDs)
			}
		})
	}
}

type partialProgressSinkEngine struct{ hookProbeShaper }

func (s *partialProgressSinkEngine) New() ShapingEngine { return s }
func (s *partialProgressSinkEngine) CollectFeatures(p FeaturePlanner, _ SelectionContext) {
	p.EnableFeature(ot.T("test"))
}
func (s *partialProgressSinkEngine) OverrideFeatures(FeaturePlanner) {}

type partialProgressPairCoverage struct{}

func (partialProgressPairCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == 18 || g == 19 }
func (partialProgressPairCoverage) ByteSize() int                     { return 0 }

func TestPartialProgressPublicSinkBoundary(t *testing.T) {
	for _, events := range []bool{false, true} {
		for _, prefix := range []bool{false, true} {
			t.Run(fmt.Sprintf("events=%t/prefix=%t", events, prefix), func(t *testing.T) {
				font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
				l := font.Layout.GSub.LookupGraph().Lookup(0)
				n := l.Subtable(0)
				count := font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs
				l.Type, l.Flag = ot.GSubLookupTypeSingle, 0
				*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: partialProgressPairCoverage{}}, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: int16(count - 19)}}}
				input := string(rune(19))
				if prefix {
					input = strings.Repeat(string(rune(18)), 40) + input
				}
				sink := &collectSink{}
				shaper := NewShaper(&partialProgressSinkEngine{})
				params := standardParams(font)
				var err error
				if events {
					err = shaper.ShapeEvents(params, NewInputEventSource(strings.NewReader(input)), sink, singleBufOpts)
				} else {
					err = shaper.Shape(params, strings.NewReader(input), sink, singleBufOpts)
				}
				if err == nil || !strings.Contains(err.Error(), "glyph count") {
					t.Fatalf("expected lookup bounds error, got %v; output=%v count=%d", err, sink.glyphs, count)
				}
				if prefix && len(sink.glyphs) == 0 {
					t.Fatal("expected previously emitted prefix")
				}
				if !prefix && len(sink.glyphs) != 0 {
					t.Fatal("failing chunk was emitted")
				}
				for _, g := range sink.glyphs {
					if g.GID != ot.GlyphIndex(count-1) {
						t.Fatalf("unexpected prefix glyph: %v", g)
					}
				}
				// A failing run must not change future behavior for valid input.
				good := func() []GlyphRecord {
					out := &collectSink{}
					if err := shaper.Shape(params, strings.NewReader(string(rune(18))), out, singleBufOpts); err != nil {
						t.Fatal(err)
					}
					return out.glyphs
				}
				first := good()
				_ = shaper.Shape(params, strings.NewReader(string(rune(19))), &collectSink{}, singleBufOpts)
				if !reflect.DeepEqual(first, good()) {
					t.Fatal("failure changed subsequent shaping")
				}
			})
		}
	}
}

func TestLayoutMetadataEditsPreserveCorrespondence(t *testing.T) {
	rb := newRunBuffer(0)
	rb.Clusters = []uint32{9, 4, 8}
	rb.PlanIDs = []uint16{7, 7, 9}
	rb.Masks = []uint32{3, 5, 6}
	rb.Syllables = []uint16{1, 2, 3}
	rb.UnsafeFlags = []uint16{1, 2, 4}
	rb.Joiners = []uint8{0, 1, 2}
	rb.Codepoints = []rune{'a', 'b', 'c'}
	rb.applyLayoutEdit(otlayout.EditSpan{From: 0, To: 2, Len: 1})
	if !reflect.DeepEqual(rb.Clusters, []uint32{4, 8}) || !reflect.DeepEqual(rb.UnsafeFlags, []uint16{3, 4}) {
		t.Fatalf("merge failed: %+v", rb)
	}
	rb.applyLayoutEdit(otlayout.EditSpan{From: 0, To: 1, Len: 2})
	if !reflect.DeepEqual(rb.PlanIDs, []uint16{7, 7, 9}) || !reflect.DeepEqual(rb.Masks, []uint32{3, 3, 6}) || !reflect.DeepEqual(rb.Clusters, []uint32{4, 4, 8}) || !reflect.DeepEqual(rb.Codepoints, []rune{'a', 'a', 'c'}) {
		t.Fatalf("inheritance failed: %+v", rb)
	}
	if !reflect.DeepEqual(rb.Syllables, []uint16{1, 1, 3}) || !reflect.DeepEqual(rb.Joiners, []uint8{0, 0, 2}) || !reflect.DeepEqual(rb.UnsafeFlags, []uint16{3, 3, 4}) {
		t.Fatalf("annotation inheritance failed: %+v", rb)
	}
}

func (s *partialProgressSinkEngine) InitPlan(PlanContext) {}

func TestPartialProgressPositionOnlyErrorPublishesBuffer(t *testing.T) {
	font := loadMiniOTFont(t, "gpos_chaining3_boundary_f2.otf")
	graph := font.Layout.GPos.LookupGraph()
	cov := ot.Coverage{GlyphRange: partialProgressCoverage{}}
	parent, child := graph.Lookup(0), graph.Lookup(1)
	pn, cn := parent.Subtable(0), child.Subtable(0)
	parent.Type, parent.Flag = ot.MaskGPosLookupType(ot.GPosLookupTypeContextPos), 0
	child.Type, child.Flag = ot.MaskGPosLookupType(ot.GPosLookupTypeSingle), 0
	*pn = ot.LookupNode{LookupType: parent.Type, Format: 3, GPos: &ot.GPosLookupPayload{ContextFmt3: &ot.GPosContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}, {LookupListIndex: 65535}}}}}
	*cn = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov, GPos: &ot.GPosLookupPayload{SingleFmt1: &ot.GPosSingleFmt1Payload{ValueFormat: 4, Value: ot.ValueRecord{XAdvance: 7}}}}
	run := newRunBuffer(0)
	run.Glyphs = otlayout.GlyphBuffer{10}
	e := &planExecutor{}
	e.acquireBuffer(run)
	defer e.releaseBuffer()
	pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
	err := e.applyLookups(pl, planGPOS, []lookupOp{{LookupIndex: 0, FeatureTag: ot.T("test")}})
	if err == nil || len(run.Pos) != 1 || run.Pos[0].XAdvance != 7 {
		t.Fatalf("position-only partial result lost: %v %v", err, run.Pos)
	}
}

func TestOneToOneAttachmentParity(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprintf("replace=%t", replace), func(t *testing.T) {
			font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
			lookup := font.Layout.GSub.LookupGraph().Lookup(0)
			node := lookup.Subtable(0)
			lookup.Flag = 0
			cov := ot.Coverage{GlyphRange: partialProgressCoverage{}}
			if replace {
				lookup.Type = ot.GSubLookupTypeMultiple
				*node = ot.LookupNode{LookupType: lookup.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11}}}}}
			} else {
				lookup.Type = ot.GSubLookupTypeSingle
				*node = ot.LookupNode{LookupType: lookup.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
			}
			var whole otlayout.PosBuffer
			for _, isolated := range []bool{false, true} {
				run := newRunBuffer(0)
				run.Glyphs = otlayout.GlyphBuffer{30, 10, 20}
				run.Pos = otlayout.NewPosBuffer(3)
				// Both outside-span glyphs attach to the replaced middle glyph.
				for _, i := range []int{0, 2} {
					run.Pos[i].AttachTo = 1
					run.Pos[i].AttachKind = otlayout.AttachMarkToBase
					run.Pos[i].AttachClass = 2
					run.Pos[i].AnchorRef = otlayout.AnchorRef{MarkAnchor: 4, BaseAnchor: 8}
				}
				run.Pos[1].XAdvance = 17
				st := otlayout.NewBufferState(run.Glyphs, run.Pos)
				e := &planExecutor{run: run}
				pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
				feat := planLookupFeature{tag: ot.T("test"), typ: otlayout.GSubFeatureType, lookupInx: 0}
				var err error
				if isolated {
					_, err = e.applyLookupIsolatedSpan(pl, lookupOp{}, feat, st, 0, 1, 2)
				} else {
					_, err = e.applyLookupSpan(pl, lookupOp{}, feat, st, 0, 0, st.Len(), 0)
				}
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(st.Glyphs, otlayout.GlyphBuffer{30, 11, 20}) {
					t.Fatalf("glyphs %v", st.Glyphs)
				}
				wantTarget := int32(1)
				wantAdvance := int32(17)
				if replace {
					wantTarget = -1
					wantAdvance = 0
				}
				if st.Pos[0].AttachTo != wantTarget || st.Pos[2].AttachTo != wantTarget || st.Pos[1].XAdvance != wantAdvance {
					t.Errorf("isolated=%t: wrong attachment policy: %+v", isolated, st.Pos)
				}
				if !isolated {
					whole = append(otlayout.PosBuffer(nil), st.Pos...)
				} else if !reflect.DeepEqual(st.Pos, whole) {
					t.Errorf("isolated positions %+v differ from whole %+v", st.Pos, whole)
				}
			}
		})
	}
}
