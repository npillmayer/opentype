package otshape

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestLigatureSurvivorMetadata(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		t.Run(fmt.Sprint(isolated), func(t *testing.T) {
			font := loadMiniOTFont(t, "gpos5_font1.otf")
			run := newRunBuffer(0)
			run.Glyphs = otlayout.GlyphBuffer{17, 30, 19, 31, 17}
			run.Pos = otlayout.NewPosBuffer(5)
			run.Pos[0].AttachTo = 2
			run.Pos[2].AttachTo = 4 // survivor's attachment points outside isolated span
			run.Pos[2].YOffset = 77
			run.Clusters = []uint32{0, 1, 1, 3, 4}
			// A previously reordered mark must not become the ligature's source.
			run.SourceAnchors = []uint32{0, 2, 1, 3, 4}
			run.Codepoints = []rune{'p', 'a', 'm', 'b', 'q'}
			run.Masks = []uint32{1, 2, 4, 8, 16}
			run.PlanIDs = []uint16{1, 2, 3, 4, 5}
			run.Syllables = []uint16{1, 2, 2, 2, 3}
			run.Joiners = []uint8{0, 0, joinerClassZWJ, 0, 0}
			run.UnsafeFlags = []uint16{0, 1, 2, 4, 0}
			e := &planExecutor{run: run}
			pl := &plan{font: font}
			st := otlayout.NewBufferState(run.Glyphs, run.Pos)
			feat := planLookupFeature{tag: ot.T("test"), typ: otlayout.GSubFeatureType, lookupInx: 0}
			var err error
			if isolated {
				_, err = e.applyLookupIsolatedSpan(pl, lookupOp{}, feat, st, 0, 1, 4)
			} else {
				_, err = e.applyLookupSpan(pl, lookupOp{}, feat, st, 0, 1, 4, 0)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(run.Glyphs, otlayout.GlyphBuffer{17, 18, 19, 17}) ||
				!reflect.DeepEqual(run.Clusters, []uint32{0, 1, 1, 4}) ||
				!reflect.DeepEqual(run.SourceAnchors, []uint32{0, 2, 1, 4}) ||
				!reflect.DeepEqual(run.Codepoints, []rune{'p', 'a', 'm', 'q'}) ||
				!reflect.DeepEqual(run.Masks, []uint32{1, 2, 4, 16}) ||
				!reflect.DeepEqual(run.PlanIDs, []uint16{1, 2, 3, 5}) ||
				!reflect.DeepEqual(run.Syllables, []uint16{1, 2, 2, 3}) ||
				!reflect.DeepEqual(run.Joiners, []uint8{0, 0, joinerClassZWJ, 0}) ||
				!reflect.DeepEqual(run.UnsafeFlags, []uint16{0, 7, 2, 0}) {
				t.Fatalf("metadata lost: %+v", run)
			}
			if run.Pos[0].AttachTo != 2 || run.Pos[2].AttachTo != 3 || run.Pos[2].YOffset != 77 {
				t.Fatalf("attachments=%+v", run.Pos)
			}
			// A fresh BufferState for GPOS must retain the GSUB association.
			st = otlayout.NewBufferState(run.Glyphs, run.Pos)
			feat.typ = otlayout.GPosFeatureType
			if _, err = e.applyLookupSpan(pl, lookupOp{}, feat, st, 0, 0, st.Len(), 0); err != nil {
				t.Fatal(err)
			}
			if run.Pos[2].AnchorRef.LigatureComp != 0 || run.Pos[2].YOffset != -80 {
				t.Fatalf("wrong component: %+v", run.Pos[2])
			}
		})
	}
}

func TestLigaturePartialProgressMetadata(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		t.Run(fmt.Sprint(isolated), func(t *testing.T) {
			font := loadMiniOTFont(t, "gsub_context1_lookupflag_f1.otf")
			graph := font.Layout.GSub.LookupGraph()
			font.Layout.GDef = loadMiniOTFont(t, "gpos5_font1.otf").Layout.GDef
			installSuccessLookup(graph, 1, ot.GSubLookupTypeLigature, 30, &ot.GSubLookupPayload{LigatureFmt1: &ot.GSubLigatureFmt1Payload{
				LigatureSets: [][]ot.GSubLigatureRule{{{Components: []ot.GlyphIndex{31}, Ligature: 18}}}}})
			graph.Lookup(1).Flag = ot.LOOKUP_FLAG_IGNORE_MARKS
			l := graph.Lookup(0)
			l.Type, l.Flag = ot.GSubLookupTypeContext, 0
			*l.Subtable(0) = ot.LookupNode{LookupType: l.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{
				InputCoverages: []ot.Coverage{{GlyphRange: successCoverage(30)}},
				Records:        []ot.SequenceLookupRecord{{LookupListIndex: 1}, {LookupListIndex: 65535}},
			}}}
			run := newRunBuffer(0)
			run.Glyphs = otlayout.GlyphBuffer{17, 30, 19, 31, 17}
			run.Clusters = []uint32{0, 1, 1, 3, 4}
			run.SourceAnchors = []uint32{0, 1, 2, 3, 4}
			run.Masks = []uint32{1, 2, 4, 8, 16}
			e := &planExecutor{run: run}
			st := otlayout.NewBufferState(run.Glyphs, nil)
			pl := &plan{font: font}
			feat := planLookupFeature{tag: ot.T("test"), typ: otlayout.GSubFeatureType, lookupInx: 0}
			var err error
			if isolated {
				_, err = e.applyLookupIsolatedSpan(pl, lookupOp{}, feat, st, 0, 1, 4)
			} else {
				_, err = e.applyLookupSpan(pl, lookupOp{}, feat, st, 0, 1, 4, 0)
			}
			if err == nil || !reflect.DeepEqual(run.Glyphs, otlayout.GlyphBuffer{17, 18, 19, 17}) ||
				!reflect.DeepEqual(run.Clusters, []uint32{0, 1, 1, 4}) ||
				!reflect.DeepEqual(run.SourceAnchors, []uint32{0, 1, 2, 4}) ||
				!reflect.DeepEqual(run.Masks, []uint32{1, 2, 4, 16}) || len(run.Pos) != 4 {
				t.Fatalf("partial edit lost: error=%v run=%+v", err, run)
			}
		})
	}
}
