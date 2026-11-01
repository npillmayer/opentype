package otlayout

import (
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func deletionNode(glyph ot.GlyphIndex) ot.LookupNode {
	return ot.LookupNode{LookupType: ot.GSubLookupTypeMultiple, Format: 1,
		Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: glyph}},
		GSub:     &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{}}}}}
}

func TestGSUBDeletionJournalsAndPreservesSharedPositions(t *testing.T) {
	for _, extension := range []bool{false, true} {
		font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
		lookup := font.Layout.GSub.LookupGraph().Lookup(0)
		node := deletionNode(10)
		if extension {
			leaf := node
			node = ot.LookupNode{LookupType: ot.GSubLookupTypeExtensionSubs, Format: 1,
				GSub: &ot.GSubLookupPayload{ExtensionFmt1: &ot.GSubExtensionFmt1Payload{ResolvedType: leaf.LookupType, Resolved: &leaf}}}
		}
		lookup.Type, lookup.Flag = node.LookupType, 0
		*lookup.Subtable(0) = node
		original := NewBufferState(GlyphBuffer{10, 20}, NewPosBuffer(2))
		original.Pos[1].XAdvance = 123
		st := original.CloneShared()
		pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
		if err != nil || !applied || pos != 0 || st.Index != 0 || !reflect.DeepEqual(st.Glyphs, GlyphBuffer{20}) {
			t.Fatalf("extension=%v: pos=%d applied=%v err=%v glyphs=%v", extension, pos, applied, err, st.Glyphs)
		}
		if len(st.Pos) != 1 || st.Pos[0].XAdvance != 123 || !reflect.DeepEqual(st.AppliedEdits(), []EditSpan{{From: 0, To: 1, Len: 0, ResetsPositions: true}}) {
			t.Fatalf("positions=%v edits=%v", st.Pos, st.AppliedEdits())
		}
		if !reflect.DeepEqual(original.Glyphs, GlyphBuffer{10, 20}) || len(original.Pos) != 2 || original.Pos[1].XAdvance != 123 {
			t.Fatal("deletion changed shared original")
		}
	}
}

func TestContextDeletionRemapsTargetsAndContinuation(t *testing.T) {
	for _, failAfter := range []bool{false, true} {
		font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
		graph := font.Layout.GSub.LookupGraph()
		// The invalid lookup targeting deleted input must be skipped entirely.
		records := []ot.SequenceLookupRecord{{LookupListIndex: 1}, {LookupListIndex: 65535}, {SequenceIndex: 1, LookupListIndex: 2}}
		if failAfter {
			records = append(records, ot.SequenceLookupRecord{SequenceIndex: 1, LookupListIndex: 65535})
		}
		parent := graph.Lookup(0)
		parent.Type, parent.Flag = ot.GSubLookupTypeContext, 0
		*parent.Subtable(0) = ot.LookupNode{LookupType: parent.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{
			InputCoverages: []ot.Coverage{{GlyphRange: testGlyphRange{glyph: 10}}, {GlyphRange: testGlyphRange{glyph: 20}}}, Records: records}}}
		child := graph.Lookup(1)
		child.Type, child.Flag = ot.GSubLookupTypeMultiple, 0
		*child.Subtable(0) = deletionNode(10)
		child = graph.Lookup(2)
		child.Type, child.Flag = ot.GSubLookupTypeSingle, 0
		*child.Subtable(0) = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 20}}, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
		st := NewBufferState(GlyphBuffer{10, 20, 20}, nil)
		pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
		if (err != nil) != failAfter || !applied || !reflect.DeepEqual(st.Glyphs, GlyphBuffer{21, 20}) {
			t.Fatalf("failAfter=%v: applied=%v err=%v glyphs=%v", failAfter, applied, err, st.Glyphs)
		}
		if !failAfter && (pos != 1 || st.Index != 1) {
			t.Fatalf("continuation=%d index=%d, want end 1", pos, st.Index)
		}
		want := []EditSpan{{From: 0, To: 1, Len: 0, ResetsPositions: true}, {From: 0, To: 1, Len: 1}}
		if !reflect.DeepEqual(st.AppliedEdits(), want) {
			t.Fatalf("edits=%v want=%v", st.AppliedEdits(), want)
		}
	}
}
