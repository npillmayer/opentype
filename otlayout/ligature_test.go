package otlayout

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestLigaturePreservesSkippedGlyphs(t *testing.T) {
	for _, extension := range []bool{false, true} {
		t.Run(fmt.Sprint(extension), func(t *testing.T) {
			font := loadTestFont(t, "gpos5_font1.otf")
			lookup := font.Layout.GSub.LookupGraph().Lookup(0)
			if extension {
				leaf := *lookup.Subtable(0)
				lookup.Type = ot.GSubLookupTypeExtensionSubs
				*lookup.Subtable(0) = ot.LookupNode{LookupType: lookup.Type, Format: 1, GSub: &ot.GSubLookupPayload{
					ExtensionFmt1: &ot.GSubExtensionFmt1Payload{ResolvedType: leaf.LookupType, Resolved: &leaf}}}
			}
			original := NewBufferState(GlyphBuffer{17, 30, 19, 19, 31, 17}, NewPosBuffer(6))
			original.Pos[2].XOffset, original.Pos[3].YOffset = 37, 41
			original.Pos[0].AttachTo = 3 // a target inside the replaced span survives
			original.Pos[5].AttachTo = 4 // a consumed target does not survive
			before := append(PosBuffer(nil), original.Pos...)
			st := original.CloneShared()
			st.Index = 1
			pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
			if err != nil || !applied || pos != 4 || !reflect.DeepEqual(st.Glyphs, GlyphBuffer{17, 18, 19, 19, 17}) {
				t.Fatalf("pos=%d applied=%t err=%v glyphs=%v", pos, applied, err, st.Glyphs)
			}
			if st.Pos[2].XOffset != 37 || st.Pos[3].YOffset != 41 || st.Pos[0].AttachTo != 3 || st.Pos[4].AttachTo != -1 {
				t.Fatalf("survivor positions/targets lost: %+v", st.Pos)
			}
			if !reflect.DeepEqual(original.Pos, before) || !reflect.DeepEqual(original.Glyphs, GlyphBuffer{17, 30, 19, 19, 31, 17}) {
				t.Fatal("ligature modified shared original")
			}
			for _, i := range []int{2, 3} {
				if st.Pos[i].ligature != st.Pos[1].ligature || st.Pos[i].ligatureComponent != 1 {
					t.Fatal("surviving mark lost first-component association")
				}
			}
			edits := st.AppliedEdits()
			if len(edits) != 1 || edits[0].From != 1 || edits[0].To != 5 || edits[0].Len != 3 {
				t.Fatalf("journal=%+v", edits)
			}
			for input, want := range []int{0, 1, 2, 3, 1, 4} {
				if got := edits[0].MapIndex(input); got != want {
					t.Fatalf("input %d -> %d, want %d", input, got, want)
				}
			}
			edits[0].From = 99
			if st.AppliedEdits()[0].From != 1 {
				t.Fatal("journal snapshot aliases owner")
			}
		})
	}
}

func TestLigatureContextTargetsAndPartialProgress(t *testing.T) {
	for _, fail := range []bool{false, true} {
		font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
		graph := font.Layout.GSub.LookupGraph()
		cov := func(g ot.GlyphIndex) ot.Coverage { return ot.Coverage{GlyphRange: testGlyphRange{glyph: g}} }
		// The context sees the mark, while the child ligature skips it. A later
		// record must still reach that mark; another targets the consumed B and
		// must reach the new ligature, not disappear as with ordinary deletion.
		records := []ot.SequenceLookupRecord{{LookupListIndex: 1}, {SequenceIndex: 1, LookupListIndex: 2}, {SequenceIndex: 2, LookupListIndex: 3}}
		if fail {
			records = append(records, ot.SequenceLookupRecord{LookupListIndex: 65535})
		}
		parent := graph.Lookup(0)
		parent.Type, parent.Flag = ot.GSubLookupTypeContext, 0
		*parent.Subtable(0) = ot.LookupNode{LookupType: parent.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{
			InputCoverages: []ot.Coverage{cov(10), cov(90), cov(11)}, Records: records}}}
		child := graph.Lookup(1)
		child.Type, child.Flag = ot.GSubLookupTypeLigature, ot.LOOKUP_FLAG_IGNORE_MARKS
		*child.Subtable(0) = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov(10), GSub: &ot.GSubLookupPayload{LigatureFmt1: &ot.GSubLigatureFmt1Payload{
			LigatureSets: [][]ot.GSubLigatureRule{{{Components: []ot.GlyphIndex{11}, Ligature: 12}}}}}}
		for i, g := range []ot.GlyphIndex{90, 12} {
			l := graph.Lookup(i + 2)
			l.Type, l.Flag = ot.GSubLookupTypeSingle, 0
			*l.Subtable(0) = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov(g), GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
		}
		st := NewBufferState(GlyphBuffer{10, 90, 11, 20}, nil)
		pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
		if (err != nil) != fail || !applied || !reflect.DeepEqual(st.Glyphs, GlyphBuffer{13, 91, 20}) || len(st.AppliedEdits()) != 3 {
			t.Fatalf("fail=%t pos=%d applied=%t error=%v glyphs=%v edits=%v", fail, pos, applied, err, st.Glyphs, st.AppliedEdits())
		}
		if !fail && pos != 2 {
			t.Fatalf("continuation=%d, want 2", pos)
		}
		if st.Pos[1].ligature != st.Pos[0].ligature || st.Pos[1].ligatureComponent != 1 {
			t.Fatal("later singles lost provenance")
		}
	}
}

func TestLigatureIdentityAndComponentClamp(t *testing.T) {
	font := loadTestFont(t, "gpos5_font1.otf")
	st := NewBufferState(GlyphBuffer{30, 19, 31, 30, 19, 31}, nil)
	for _, start := range []int{0, 2} {
		st.Index = start
		if _, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0); err != nil || !applied {
			t.Fatal(applied, err)
		}
	}
	if st.Pos[0].ligature == st.Pos[2].ligature {
		t.Fatal("distinct occurrences share an identity")
	}
	if st.Pos[0].OutputPosition() != st.Pos[2].OutputPosition() || st.Pos[0].ligature == nil {
		t.Fatal("emission must discard transient identity without changing shaping state")
	}
	for _, tc := range []struct {
		name      string
		matching  bool
		component int
		want      uint16
	}{
		{"first", true, 1, 0}, {"clamped", true, 99, 1}, {"different ligature", false, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clone := st.CloneShared()
			clone.EnsurePos()
			clone.Pos[3].ligatureComponent = tc.component
			if !tc.matching {
				clone.Pos[3].ligature = clone.Pos[0].ligature
			}
			clone.Index = 3
			_, applied, err := ApplyFeature(font, errorTestFeature{GPosFeatureType, []int{0}}, clone, 0)
			if err != nil || !applied || clone.Pos[3].AnchorRef.LigatureComp != tc.want {
				t.Fatalf("applied=%t err=%v attachment=%+v", applied, err, clone.Pos[3])
			}
		})
	}
}
