package otshape

import (
	"fmt"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"reflect"
	"strings"
	"testing"
)

type invalidOutputCoverage struct{}

func (invalidOutputCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == 11 }
func (invalidOutputCoverage) ByteSize() int                     { return 0 }

type outputPairCoverage struct{}

func (outputPairCoverage) Match(g ot.GlyphIndex) (int, bool) { return int(g) - 18, g == 18 || g == 19 }
func (outputPairCoverage) ByteSize() int                     { return 0 }

func TestInvalidMultipleOutputRetainsEarlierProgress(t *testing.T) {
	font := loadMiniOTFont(t, "gsub_context1_lookupflag_f1.otf")
	graph := font.Layout.GSub.LookupGraph()
	cov := ot.Coverage{GlyphRange: partialProgressCoverage{}}
	l := graph.Lookup(0)
	n := l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeContext, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}, {LookupListIndex: 2}}}}}
	l = graph.Lookup(1)
	n = l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeMultiple, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12, 13}}}}}
	count := font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs
	l = graph.Lookup(2)
	n = l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeMultiple, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: invalidOutputCoverage{}}, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{12, ot.GlyphIndex(count)}}}}}
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
			if err == nil || !strings.Contains(err.Error(), "glyph count") {
				t.Fatalf("expected output bounds error: %v", err)
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
func TestInvalidMultipleOutputPublicSinkBoundary(t *testing.T) {
	for _, events := range []bool{false, true} {
		for _, prefix := range []bool{false, true} {
			t.Run(fmt.Sprintf("events=%t/prefix=%t", events, prefix), func(t *testing.T) {
				font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
				l := font.Layout.GSub.LookupGraph().Lookup(0)
				n := l.Subtable(0)
				count := font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs
				l.Type, l.Flag = ot.GSubLookupTypeMultiple, 0
				*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: outputPairCoverage{}}, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{ot.GlyphIndex(count - 1)}, {12, ot.GlyphIndex(count)}}}}}
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
