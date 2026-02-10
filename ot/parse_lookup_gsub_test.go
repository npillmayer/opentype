package ot

import (
	"encoding/binary"
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func putU16(b []byte, at int, v uint16) {
	binary.BigEndian.PutUint16(b[at:at+2], v)
}

func coverageFmt1(glyphs ...uint16) []byte {
	out := make([]byte, 4+len(glyphs)*2)
	putU16(out, 0, 1)
	putU16(out, 2, uint16(len(glyphs)))
	for i, g := range glyphs {
		putU16(out, 4+i*2, g)
	}
	return out
}

func classDefFmt1(start uint16, classes ...uint16) []byte {
	out := make([]byte, 6+len(classes)*2)
	putU16(out, 0, 1)
	putU16(out, 2, start)
	putU16(out, 4, uint16(len(classes)))
	for i, clz := range classes {
		putU16(out, 6+i*2, clz)
	}
	return out
}

func TestParseConcreteGSubType1Format1(t *testing.T) {
	// format=1, coverageOffset=6, deltaGlyphID=3, coverage=[5]
	b := make([]byte, 12)
	putU16(b, 0, 1)
	putU16(b, 2, 6)
	putU16(b, 4, 3)
	copy(b[6:], coverageFmt1(5))

	node := parseConcreteLookupNode(b, GSubLookupTypeSingle)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB1/1 node, err=%v", node.Error())
	}
	if node.GSubPayload() == nil || node.GSubPayload().SingleFmt1 == nil {
		t.Fatalf("expected GSUB1/1 payload scaffold")
	}
	if node.GSubPayload().SingleFmt1.DeltaGlyphID != 3 {
		t.Fatalf("expected delta 3, have %d", node.GSubPayload().SingleFmt1.DeltaGlyphID)
	}
	if inx, ok := node.Coverage.Match(5); !ok || inx != 0 {
		t.Fatalf("expected coverage to contain glyph 5 at index 0")
	}
}

func TestParseConcreteGSubType1Format2(t *testing.T) {
	// format=2, coverageOffset=10, glyphCount=2, subst=[10,11], coverage=[3,4]
	b := make([]byte, 18)
	putU16(b, 0, 2)
	putU16(b, 2, 10)
	putU16(b, 4, 2)
	putU16(b, 6, 10)
	putU16(b, 8, 11)
	copy(b[10:], coverageFmt1(3, 4))

	node := parseConcreteLookupNode(b, GSubLookupTypeSingle)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB1/2 node, err=%v", node.Error())
	}
	p := node.GSubPayload().SingleFmt2
	if p == nil || len(p.SubstituteGlyphIDs) != 2 {
		t.Fatalf("expected 2 substitute glyphs")
	}
	if p.SubstituteGlyphIDs[0] != 10 || p.SubstituteGlyphIDs[1] != 11 {
		t.Fatalf("unexpected substitutes: %v", p.SubstituteGlyphIDs)
	}
}

func TestParseConcreteGSubType2Format1(t *testing.T) {
	// format=1, coverageOffset=8, sequenceCount=1, sequenceOffset=14
	// coverage=[7], sequence=[20,21]
	b := make([]byte, 20)
	putU16(b, 0, 1)
	putU16(b, 2, 8)
	putU16(b, 4, 1)
	putU16(b, 6, 14)
	copy(b[8:], coverageFmt1(7))
	putU16(b, 14, 2)
	putU16(b, 16, 20)
	putU16(b, 18, 21)

	node := parseConcreteLookupNode(b, GSubLookupTypeMultiple)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB2/1 node, err=%v", node.Error())
	}
	p := node.GSubPayload().MultipleFmt1
	if p == nil || len(p.Sequences) != 1 || len(p.Sequences[0]) != 2 {
		t.Fatalf("expected one sequence with 2 glyphs")
	}
	if p.Sequences[0][0] != 20 || p.Sequences[0][1] != 21 {
		t.Fatalf("unexpected sequence glyphs: %v", p.Sequences[0])
	}
}

func TestParseConcreteGSubType3And4Format1(t *testing.T) {
	// GSUB3/1: alternateSetCount=1, alternateSetOffset=14, alternates=[30,31]
	b3 := make([]byte, 20)
	putU16(b3, 0, 1)
	putU16(b3, 2, 8)
	putU16(b3, 4, 1)
	putU16(b3, 6, 14)
	copy(b3[8:], coverageFmt1(9))
	putU16(b3, 14, 2)
	putU16(b3, 16, 30)
	putU16(b3, 18, 31)
	n3 := parseConcreteLookupNode(b3, GSubLookupTypeAlternate)
	if n3 == nil || n3.Error() != nil {
		t.Fatalf("expected concrete GSUB3/1 node, err=%v", n3.Error())
	}
	if p := n3.GSubPayload().AlternateFmt1; p == nil || len(p.Alternates) != 1 || len(p.Alternates[0]) != 2 {
		t.Fatalf("expected one alternate set with 2 glyphs")
	}

	// GSUB4/1: ligSetCount=1, ligSetOffset=14, ligCount=1, ligOffset=4,
	// ligatureGlyph=50, compCount=2, components=[40]
	b4 := make([]byte, 24)
	putU16(b4, 0, 1)
	putU16(b4, 2, 8)
	putU16(b4, 4, 1)
	putU16(b4, 6, 14)
	copy(b4[8:], coverageFmt1(12))
	putU16(b4, 14, 1)
	putU16(b4, 16, 4)
	putU16(b4, 18, 50)
	putU16(b4, 20, 2)
	putU16(b4, 22, 40)
	n4 := parseConcreteLookupNode(b4, GSubLookupTypeLigature)
	if n4 == nil || n4.Error() != nil {
		t.Fatalf("expected concrete GSUB4/1 node, err=%v", n4.Error())
	}
	p4 := n4.GSubPayload().LigatureFmt1
	if p4 == nil || len(p4.LigatureSets) != 1 || len(p4.LigatureSets[0]) != 1 {
		t.Fatalf("expected one ligature rule")
	}
	r := p4.LigatureSets[0][0]
	if r.Ligature != 50 || len(r.Components) != 1 || r.Components[0] != 40 {
		t.Fatalf("unexpected ligature rule: %+v", r)
	}
}

func TestParseConcreteGSubType5Format1(t *testing.T) {
	// format=1, coverageOffset=8, seqRuleSetCount=1, seqRuleSetOffset=14
	// coverage=[10], rule: input=[11], record=(seq=1, lookup=7)
	b := make([]byte, 28)
	putU16(b, 0, 1)
	putU16(b, 2, 8)
	putU16(b, 4, 1)
	putU16(b, 6, 14)
	copy(b[8:], coverageFmt1(10))
	putU16(b, 14, 1)
	putU16(b, 16, 4)
	putU16(b, 18, 2)
	putU16(b, 20, 1)
	putU16(b, 22, 11)
	putU16(b, 24, 1)
	putU16(b, 26, 7)

	node := parseConcreteLookupNode(b, GSubLookupTypeContext)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB5/1 node, err=%v", node.Error())
	}
	p := node.GSubPayload().ContextFmt1
	if p == nil || len(p.RuleSets) != 1 || len(p.RuleSets[0]) != 1 {
		t.Fatalf("expected one rule set with one rule")
	}
	r := p.RuleSets[0][0]
	if len(r.InputGlyphs) != 1 || r.InputGlyphs[0] != 11 {
		t.Fatalf("unexpected rule input glyphs: %v", r.InputGlyphs)
	}
	if len(r.Records) != 1 || r.Records[0].SequenceIndex != 1 || r.Records[0].LookupListIndex != 7 {
		t.Fatalf("unexpected rule records: %v", r.Records)
	}
	if inx, ok := node.Coverage.Match(10); !ok || inx != 0 {
		t.Fatalf("expected coverage to contain glyph 10")
	}
}

func TestParseConcreteGSubType5Format2(t *testing.T) {
	// format=2, coverageOffset=10, classDefOffset=16, classSeqRuleSetCount=1, classSeqRuleSetOffset=26
	// classDef fmt1: start=20, values=[1,2]
	// class rule: inputClasses=[2], record=(seq=0, lookup=9)
	b := make([]byte, 40)
	putU16(b, 0, 2)
	putU16(b, 2, 10)
	putU16(b, 4, 16)
	putU16(b, 6, 1)
	putU16(b, 8, 26)
	copy(b[10:], coverageFmt1(15))
	putU16(b, 16, 1)
	putU16(b, 18, 20)
	putU16(b, 20, 2)
	putU16(b, 22, 1)
	putU16(b, 24, 2)
	putU16(b, 26, 1)
	putU16(b, 28, 4)
	putU16(b, 30, 2)
	putU16(b, 32, 1)
	putU16(b, 34, 2)
	putU16(b, 36, 0)
	putU16(b, 38, 9)

	node := parseConcreteLookupNode(b, GSubLookupTypeContext)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB5/2 node, err=%v", node.Error())
	}
	p := node.GSubPayload().ContextFmt2
	if p == nil || len(p.RuleSets) != 1 || len(p.RuleSets[0]) != 1 {
		t.Fatalf("expected one class-rule set with one rule")
	}
	if clz := p.ClassDef.Lookup(20); clz != 1 {
		t.Fatalf("expected classDef.Lookup(20)=1, got %d", clz)
	}
	r := p.RuleSets[0][0]
	if len(r.InputClasses) != 1 || r.InputClasses[0] != 2 {
		t.Fatalf("unexpected class rule input classes: %v", r.InputClasses)
	}
	if len(r.Records) != 1 || r.Records[0].LookupListIndex != 9 {
		t.Fatalf("unexpected class rule records: %v", r.Records)
	}
}

func TestParseConcreteGSubType5Format3(t *testing.T) {
	// format=3, glyphCount=2, seqLookupCount=1
	// input coverages=[30],[31], record=(seq=0, lookup=11)
	b := make([]byte, 26)
	putU16(b, 0, 3)
	putU16(b, 2, 2)
	putU16(b, 4, 1)
	putU16(b, 6, 14)
	putU16(b, 8, 20)
	putU16(b, 10, 0)
	putU16(b, 12, 11)
	copy(b[14:], coverageFmt1(30))
	copy(b[20:], coverageFmt1(31))

	node := parseConcreteLookupNode(b, GSubLookupTypeContext)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB5/3 node, err=%v", node.Error())
	}
	p := node.GSubPayload().ContextFmt3
	if p == nil || len(p.InputCoverages) != 2 {
		t.Fatalf("expected two input coverages")
	}
	if _, ok := p.InputCoverages[0].Match(30); !ok {
		t.Fatalf("expected first input coverage to match glyph 30")
	}
	if _, ok := p.InputCoverages[1].Match(31); !ok {
		t.Fatalf("expected second input coverage to match glyph 31")
	}
	if len(p.Records) != 1 || p.Records[0].LookupListIndex != 11 {
		t.Fatalf("unexpected lookup records: %v", p.Records)
	}
}

func TestParseConcreteGSubType6Format1(t *testing.T) {
	// format=1, coverageOffset=8, chainSubRuleSetCount=1, chainSubRuleSetOffset=14
	// one chained rule: back=[101], input=[102], lookahead=[103], record=(1,9)
	b := make([]byte, 36)
	putU16(b, 0, 1)
	putU16(b, 2, 8)
	putU16(b, 4, 1)
	putU16(b, 6, 14)
	copy(b[8:], coverageFmt1(100))
	putU16(b, 14, 1)
	putU16(b, 16, 4)
	putU16(b, 18, 1)
	putU16(b, 20, 101)
	putU16(b, 22, 2)
	putU16(b, 24, 102)
	putU16(b, 26, 1)
	putU16(b, 28, 103)
	putU16(b, 30, 1)
	putU16(b, 32, 1)
	putU16(b, 34, 9)

	node := parseConcreteLookupNode(b, GSubLookupTypeChainingContext)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB6/1 node, err=%v", node.Error())
	}
	p := node.GSubPayload().ChainingContextFmt1
	if p == nil || len(p.RuleSets) != 1 || len(p.RuleSets[0]) != 1 {
		t.Fatalf("expected one chained rule set with one rule")
	}
	r := p.RuleSets[0][0]
	if len(r.Backtrack) != 1 || r.Backtrack[0] != 101 {
		t.Fatalf("unexpected backtrack: %v", r.Backtrack)
	}
	if len(r.Input) != 1 || r.Input[0] != 102 {
		t.Fatalf("unexpected input: %v", r.Input)
	}
	if len(r.Lookahead) != 1 || r.Lookahead[0] != 103 {
		t.Fatalf("unexpected lookahead: %v", r.Lookahead)
	}
	if len(r.Records) != 1 || r.Records[0].LookupListIndex != 9 {
		t.Fatalf("unexpected records: %v", r.Records)
	}
}

func TestParseConcreteGSubType6Format2(t *testing.T) {
	// format=2, coverageOffset=14, classDefOffsets=20/28/36, classRuleSetCount=1, classRuleSetOffset=44
	// one class rule: back=[5], input=[6], lookahead=[7], record=(0,12)
	b := make([]byte, 66)
	putU16(b, 0, 2)
	putU16(b, 2, 14)
	putU16(b, 4, 20)
	putU16(b, 6, 28)
	putU16(b, 8, 36)
	putU16(b, 10, 1)
	putU16(b, 12, 44)
	copy(b[14:], coverageFmt1(55))
	copy(b[20:], classDefFmt1(30, 5))
	copy(b[28:], classDefFmt1(40, 6))
	copy(b[36:], classDefFmt1(50, 7))
	putU16(b, 44, 1)
	putU16(b, 46, 4)
	putU16(b, 48, 1)
	putU16(b, 50, 5)
	putU16(b, 52, 2)
	putU16(b, 54, 6)
	putU16(b, 56, 1)
	putU16(b, 58, 7)
	putU16(b, 60, 1)
	putU16(b, 62, 0)
	putU16(b, 64, 12)

	node := parseConcreteLookupNode(b, GSubLookupTypeChainingContext)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB6/2 node, err=%v", node.Error())
	}
	p := node.GSubPayload().ChainingContextFmt2
	if p == nil || len(p.RuleSets) != 1 || len(p.RuleSets[0]) != 1 {
		t.Fatalf("expected one chained class rule set with one rule")
	}
	if p.BacktrackClassDef.Lookup(30) != 5 || p.InputClassDef.Lookup(40) != 6 || p.LookaheadClassDef.Lookup(50) != 7 {
		t.Fatalf("unexpected class-def lookups")
	}
	r := p.RuleSets[0][0]
	if len(r.Backtrack) != 1 || r.Backtrack[0] != 5 {
		t.Fatalf("unexpected backtrack classes: %v", r.Backtrack)
	}
	if len(r.Input) != 1 || r.Input[0] != 6 {
		t.Fatalf("unexpected input classes: %v", r.Input)
	}
	if len(r.Lookahead) != 1 || r.Lookahead[0] != 7 {
		t.Fatalf("unexpected lookahead classes: %v", r.Lookahead)
	}
	if len(r.Records) != 1 || r.Records[0].LookupListIndex != 12 {
		t.Fatalf("unexpected class records: %v", r.Records)
	}
}

func TestParseConcreteGSubType6Format3AndType8(t *testing.T) {
	// GSUB6/3: back=[cov201], input=[cov202,cov203], lookahead=[cov204], record=(1,13)
	b6 := make([]byte, 46)
	putU16(b6, 0, 3)
	putU16(b6, 2, 1)
	putU16(b6, 4, 22)
	putU16(b6, 6, 2)
	putU16(b6, 8, 28)
	putU16(b6, 10, 34)
	putU16(b6, 12, 1)
	putU16(b6, 14, 40)
	putU16(b6, 16, 1)
	putU16(b6, 18, 1)
	putU16(b6, 20, 13)
	copy(b6[22:], coverageFmt1(201))
	copy(b6[28:], coverageFmt1(202))
	copy(b6[34:], coverageFmt1(203))
	copy(b6[40:], coverageFmt1(204))

	n6 := parseConcreteLookupNode(b6, GSubLookupTypeChainingContext)
	if n6 == nil || n6.Error() != nil {
		t.Fatalf("expected concrete GSUB6/3 node, err=%v", n6.Error())
	}
	p6 := n6.GSubPayload().ChainingContextFmt3
	if p6 == nil || len(p6.BacktrackCoverages) != 1 || len(p6.InputCoverages) != 2 || len(p6.LookaheadCoverages) != 1 {
		t.Fatalf("unexpected GSUB6/3 coverage payload sizes")
	}
	if len(p6.Records) != 1 || p6.Records[0].LookupListIndex != 13 {
		t.Fatalf("unexpected GSUB6/3 records: %v", p6.Records)
	}

	// GSUB8/1: input coverage=[200], backtrack=[201], lookahead=[202], substitute=[301,302]
	b8 := make([]byte, 36)
	putU16(b8, 0, 1)
	putU16(b8, 2, 18)
	putU16(b8, 4, 1)
	putU16(b8, 6, 24)
	putU16(b8, 8, 1)
	putU16(b8, 10, 30)
	putU16(b8, 12, 2)
	putU16(b8, 14, 301)
	putU16(b8, 16, 302)
	copy(b8[18:], coverageFmt1(200))
	copy(b8[24:], coverageFmt1(201))
	copy(b8[30:], coverageFmt1(202))

	n8 := parseConcreteLookupNode(b8, GSubLookupTypeReverseChaining)
	if n8 == nil || n8.Error() != nil {
		t.Fatalf("expected concrete GSUB8/1 node, err=%v", n8.Error())
	}
	p8 := n8.GSubPayload().ReverseChainingFmt1
	if p8 == nil || len(p8.BacktrackCoverages) != 1 || len(p8.LookaheadCoverages) != 1 || len(p8.SubstituteGlyphIDs) != 2 {
		t.Fatalf("unexpected GSUB8/1 payload sizes")
	}
	if _, ok := n8.Coverage.Match(200); !ok {
		t.Fatalf("expected GSUB8/1 input coverage to match glyph 200")
	}
	if p8.SubstituteGlyphIDs[0] != 301 || p8.SubstituteGlyphIDs[1] != 302 {
		t.Fatalf("unexpected GSUB8/1 substitute IDs: %v", p8.SubstituteGlyphIDs)
	}
}

func putU32(b []byte, at int, v uint32) {
	binary.BigEndian.PutUint32(b[at:at+4], v)
}

func TestParseConcreteGSubType7ExtensionFormat1(t *testing.T) {
	// GSUB7/1 extension with resolved GSUB1/1 at offset 8.
	b := make([]byte, 20)
	putU16(b, 0, 1) // extension format
	putU16(b, 2, 1) // resolved lookup type = GSUB single
	putU32(b, 4, 8) // offset32 to wrapped subtable
	// wrapped GSUB1/1
	putU16(b, 8, 1)  // format
	putU16(b, 10, 6) // coverage offset from wrapped start
	putU16(b, 12, 5) // delta
	copy(b[14:], coverageFmt1(42))

	node := parseConcreteLookupNode(b, GSubLookupTypeExtensionSubs)
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GSUB7/1 node, err=%v", node.Error())
	}
	p := node.GSubPayload().ExtensionFmt1
	if p == nil {
		t.Fatalf("expected GSUB7 extension payload")
	}
	if p.ResolvedType != GSubLookupTypeSingle {
		t.Fatalf("expected resolved type=1, got %d", p.ResolvedType)
	}
	if p.Resolved == nil || p.Resolved.GSubPayload() == nil || p.Resolved.GSubPayload().SingleFmt1 == nil {
		t.Fatalf("expected resolved GSUB1/1 payload")
	}
	if p.Resolved.GSubPayload().SingleFmt1.DeltaGlyphID != 5 {
		t.Fatalf("expected resolved delta=5, got %d", p.Resolved.GSubPayload().SingleFmt1.DeltaGlyphID)
	}
	if _, ok := node.Coverage.Match(42); !ok {
		t.Fatalf("expected extension node coverage forwarded from resolved payload")
	}
}

func TestParseGSubType8(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	fonts := []string{"Calibri", "GentiumPlus-R"}
	found := false
	for _, name := range fonts {
		otf := parseFont(t, name)
		table := getTable(otf, "GSUB", t)
		gsub := table.Self().AsGSub()
		if gsub == nil {
			t.Fatalf("cannot convert GSUB table for %s", name)
		}

		graph := gsub.LookupGraph()
		if graph == nil {
			continue
		}
		for i := 0; i < graph.Len(); i++ {
			lookup := graph.Lookup(i)
			if lookup == nil || lookup.Type != GSubLookupTypeReverseChaining {
				continue
			}
			found = true
			for j := 0; j < int(lookup.SubTableCount); j++ {
				node := lookup.Subtable(j)
				if node == nil {
					t.Fatalf("%s: lookup[%d] subtable[%d] missing", name, i, j)
				}
				if node.LookupType != GSubLookupTypeReverseChaining {
					t.Errorf("%s: lookup[%d] subtable[%d] type = %d", name, i, j, node.LookupType)
				}
				payload := node.GSubPayload()
				if payload == nil || payload.ReverseChainingFmt1 == nil {
					t.Fatalf("%s: lookup[%d] subtable[%d] missing reverse chaining payload", name, i, j)
				}
				if len(payload.ReverseChainingFmt1.SubstituteGlyphIDs) == 0 {
					t.Fatalf("%s: lookup[%d] subtable[%d] reverse chaining payload has no substitutes", name, i, j)
				}
			}
		}
	}
	if !found {
		t.Skip("no GSUB type 8 lookups found in testdata fonts")
	}
}
