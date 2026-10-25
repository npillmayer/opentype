package otlayout

import (
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type partialProgressLaterCoverage struct{}

func (partialProgressLaterCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == 12 || g == 20 }
func (partialProgressLaterCoverage) ByteSize() int                     { return 0 }

// Stage-5 low-level partial-progress contract tests.
func TestPartialProgressExpansionEditsPositionsOnce(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	lookup := font.Layout.GSub.LookupGraph().Lookup(0)
	node := lookup.Subtable(0)
	lookup.Type, lookup.Flag = ot.GSubLookupTypeMultiple, 0
	*node = ot.LookupNode{LookupType: lookup.Type, Format: 1,
		Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}},
		GSub:     &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12, 13}}}}}
	st := NewBufferState(GlyphBuffer{10, 20}, NewPosBuffer(2))
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0, 65535}}, st, 0)
	if err == nil || !applied {
		t.Fatalf("expected partial success and error: %v %v", applied, err)
	}
	if !reflect.DeepEqual(st.Glyphs, GlyphBuffer{11, 12, 13, 20}) {
		t.Fatalf("lost completed expansion: %v", st.Glyphs)
	}
	if len(st.Pos) != len(st.Glyphs) {
		t.Fatalf("position edit applied twice: glyphs=%d positions=%d", len(st.Glyphs), len(st.Pos))
	}
}

func TestPartialProgressNestedEditRemapsOuterInput(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	graph := font.Layout.GSub.LookupGraph()
	cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
	installContext := func(index int, covs []ot.Coverage, records []ot.SequenceLookupRecord) {
		l := graph.Lookup(index)
		n := l.Subtable(0)
		l.Type, l.Flag = ot.GSubLookupTypeContext, 0
		*n = ot.LookupNode{LookupType: l.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: covs, Records: records}}}
	}
	installContext(0, []ot.Coverage{cov, {GlyphRange: testGlyphRange{glyph: 20}}},
		[]ot.SequenceLookupRecord{{LookupListIndex: 1}, {SequenceIndex: 1, LookupListIndex: 3}, {LookupListIndex: 65535}})
	installContext(1, []ot.Coverage{cov}, []ot.SequenceLookupRecord{{LookupListIndex: 2}})
	l := graph.Lookup(2)
	n := l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeMultiple, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12, 13}}}}}
	l = graph.Lookup(3)
	n = l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeSingle, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: partialProgressLaterCoverage{}}, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
	st := NewBufferState(GlyphBuffer{10, 20}, nil)
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || !applied {
		t.Fatalf("expected partial success: %v %v", applied, err)
	}
	if want := []EditSpan{{From: 0, To: 1, Len: 3}, {From: 3, To: 4, Len: 1}}; !reflect.DeepEqual(st.edits, want) {
		t.Fatalf("nested journal: got %v want %v", st.edits, want)
	}
	if want := (GlyphBuffer{11, 12, 13, 21}); !reflect.DeepEqual(st.Glyphs, want) {
		t.Fatalf("outer input mapping lost nested edit: got %v want %v", st.Glyphs, want)
	}
}

// A contraction must keep the untouched suffix positioning and shared original.
func TestPartialProgressContractionPreservesPositionSuffix(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	l := font.Layout.GSub.LookupGraph().Lookup(0)
	n := l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeLigature, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}},
		GSub: &ot.GSubLookupPayload{LigatureFmt1: &ot.GSubLigatureFmt1Payload{LigatureSets: [][]ot.GSubLigatureRule{{{Components: []ot.GlyphIndex{11}, Ligature: 12}}}}}}
	original := NewBufferState(GlyphBuffer{10, 11, 20}, NewPosBuffer(3))
	original.Pos[2].XAdvance = 123
	st := original.CloneShared()
	pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0, 65535}}, st, 0)
	if err == nil || !applied || pos != st.Index {
		t.Fatalf("invalid partial result %d %v %v", pos, applied, err)
	}
	if !reflect.DeepEqual(st.Glyphs, GlyphBuffer{12, 20}) || len(st.Pos) != 2 || st.Pos[1].XAdvance != 123 {
		t.Fatalf("bad contraction: %v %v", st.Glyphs, st.Pos)
	}
	if !reflect.DeepEqual(original.Glyphs, GlyphBuffer{10, 11, 20}) || len(original.Pos) != 3 || original.Pos[2].XAdvance != 123 {
		t.Fatal("shared original mutated")
	}
	if !reflect.DeepEqual(st.edits, []EditSpan{{From: 0, To: 2, Len: 1}}) {
		t.Fatalf("bad journal: %v", st.edits)
	}
}

func TestPartialProgressContextPreservesSharedOwnership(t *testing.T) {
	for _, gpos := range []bool{false, true} {
		t.Run(map[bool]string{false: "GSUB", true: "GPOS"}[gpos], func(t *testing.T) {
			file := "gsub_context1_lookupflag_f1.otf"
			kind := GSubFeatureType
			if gpos {
				file = "gpos_chaining3_boundary_f2.otf"
				kind = GPosFeatureType
			}
			font := loadTestFont(t, file)
			var graph *ot.LookupListGraph
			if gpos {
				graph = font.Layout.GPos.LookupGraph()
			} else {
				graph = font.Layout.GSub.LookupGraph()
			}
			parent, child := graph.Lookup(0), graph.Lookup(1)
			pn, cn := parent.Subtable(0), child.Subtable(0)
			parent.Flag, child.Flag = 0, 0
			cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
			records := []ot.SequenceLookupRecord{{LookupListIndex: 1}, {LookupListIndex: 65535}}
			if gpos {
				parent.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeContextPos)
				child.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeSingle)
				*pn = ot.LookupNode{LookupType: parent.Type, Format: 3, GPos: &ot.GPosLookupPayload{ContextFmt3: &ot.GPosContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: records}}}
				*cn = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov, GPos: &ot.GPosLookupPayload{SingleFmt1: &ot.GPosSingleFmt1Payload{ValueFormat: 4, Value: ot.ValueRecord{XAdvance: 5}}}}
			} else {
				parent.Type = ot.GSubLookupTypeContext
				child.Type = ot.GSubLookupTypeSingle
				*pn = ot.LookupNode{LookupType: parent.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: records}}}
				*cn = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
			}
			original := NewBufferState(GlyphBuffer{10}, NewPosBuffer(1))
			original.Pos[0].XAdvance = 7
			st := original.CloneShared()
			pos, applied, err := ApplyFeature(font, errorTestFeature{kind, []int{0}}, st, 0)
			if err == nil || !applied || pos != st.Index {
				t.Fatalf("invalid partial result: %v %v", applied, err)
			}
			if original.Glyphs[0] != 10 || original.Pos[0].XAdvance != 7 {
				t.Fatal("context mutated shared original")
			}
			if gpos {
				if st.Pos[0].XAdvance != 12 {
					t.Fatalf("lost positioning: %v", st.Pos)
				}
			} else {
				if st.Glyphs[0] != 11 || st.Pos[0].XAdvance != 7 {
					t.Fatalf("single substitution reset positions: %v %v", st.Glyphs, st.Pos)
				}
			}
		})
	}
}

func TestEditJournalRetainsZeroNetLengthChanges(t *testing.T) {
	st := NewBufferState(GlyphBuffer{10, 20, 30}, NewPosBuffer(3))
	st.Pos[2].XAdvance = 123
	st.ReplaceGlyphs(0, 1, []ot.GlyphIndex{11, 12})
	st.DeleteGlyphs(2, 3)
	want := []EditSpan{{From: 0, To: 1, Len: 2}, {From: 2, To: 3, Len: 0}}
	if !reflect.DeepEqual(st.edits, want) {
		t.Fatalf("got edits %v want %v", st.edits, want)
	}
	if !reflect.DeepEqual(st.Glyphs, GlyphBuffer{11, 12, 30}) || len(st.Pos) != 3 || st.Pos[2].XAdvance != 123 {
		t.Fatalf("bad zero-net edit: %v %v", st.Glyphs, st.Pos)
	}
	clone := st.CloneShared()
	clone.Set(0, 14)
	if len(st.edits) != 2 || len(clone.edits) != 1 {
		t.Fatal("clone shares journal history")
	}
}
