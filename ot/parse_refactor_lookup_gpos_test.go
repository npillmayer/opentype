package ot

import "testing"

func TestParseConcreteGPosType1Format1And2(t *testing.T) {
	// GPOS1/1: valueFormat=XAdvance, value=7, coverage=[10]
	b1 := make([]byte, 14)
	putU16(b1, 0, 1)
	putU16(b1, 2, 8)
	putU16(b1, 4, uint16(ValueFormatXAdvance))
	putU16(b1, 6, 7)
	copy(b1[8:], coverageFmt1(10))

	n1 := parseConcreteLookupNode(b1, MaskGPosLookupType(GPosLookupTypeSingle))
	if n1 == nil || n1.Error() != nil {
		t.Fatalf("expected concrete GPOS1/1 node, err=%v", n1.Error())
	}
	p1 := n1.GPosPayload().SingleFmt1
	if p1 == nil {
		t.Fatalf("expected GPOS1/1 payload")
	}
	if p1.ValueFormat != ValueFormatXAdvance || p1.Value.XAdvance != 7 {
		t.Fatalf("unexpected GPOS1/1 value payload: format=0x%x value=%+v", p1.ValueFormat, p1.Value)
	}
	if _, ok := n1.Coverage.Match(10); !ok {
		t.Fatalf("expected GPOS1/1 coverage to match glyph 10")
	}

	// GPOS1/2: valueFormat=XPlacement, values=[5,6], coverage=[20,21]
	b2 := make([]byte, 18)
	putU16(b2, 0, 2)
	putU16(b2, 2, 12)
	putU16(b2, 4, uint16(ValueFormatXPlacement))
	putU16(b2, 6, 2)
	putU16(b2, 8, 5)
	putU16(b2, 10, 6)
	copy(b2[12:], coverageFmt1(20, 21))

	n2 := parseConcreteLookupNode(b2, MaskGPosLookupType(GPosLookupTypeSingle))
	if n2 == nil || n2.Error() != nil {
		t.Fatalf("expected concrete GPOS1/2 node, err=%v", n2.Error())
	}
	p2 := n2.GPosPayload().SingleFmt2
	if p2 == nil || len(p2.Values) != 2 {
		t.Fatalf("expected GPOS1/2 payload with 2 value records")
	}
	if p2.Values[0].XPlacement != 5 || p2.Values[1].XPlacement != 6 {
		t.Fatalf("unexpected GPOS1/2 values: %+v", p2.Values)
	}
}

func TestParseConcreteGPosType2Format1And2(t *testing.T) {
	// GPOS2/1: one pair-set with one record (second=40, value1=7, value2=-2), coverage=[25]
	b1 := make([]byte, 26)
	putU16(b1, 0, 1)
	putU16(b1, 2, 20)
	putU16(b1, 4, uint16(ValueFormatXAdvance))
	putU16(b1, 6, uint16(ValueFormatXPlacement))
	putU16(b1, 8, 1)
	putU16(b1, 10, 12)
	putU16(b1, 12, 1)
	putU16(b1, 14, 40)
	putU16(b1, 16, 7)
	putU16(b1, 18, 0xfffe)
	copy(b1[20:], coverageFmt1(25))

	n1 := parseConcreteLookupNode(b1, MaskGPosLookupType(GPosLookupTypePair))
	if n1 == nil || n1.Error() != nil {
		t.Fatalf("expected concrete GPOS2/1 node, err=%v", n1.Error())
	}
	p1 := n1.GPosPayload().PairFmt1
	if p1 == nil || len(p1.PairSets) != 1 || len(p1.PairSets[0]) != 1 {
		t.Fatalf("expected one pair set with one record")
	}
	r := p1.PairSets[0][0]
	if r.SecondGlyph != 40 || r.Value1.XAdvance != 7 || r.Value2.XPlacement != -2 {
		t.Fatalf("unexpected pair record: %+v", r)
	}

	// GPOS2/2: class-based pair values, class1Count=1 class2Count=2, values=[11,12], coverage=[30]
	b2 := make([]byte, 44)
	putU16(b2, 0, 2)
	putU16(b2, 2, 38)
	putU16(b2, 4, uint16(ValueFormatXAdvance))
	putU16(b2, 6, 0)
	putU16(b2, 8, 20)
	putU16(b2, 10, 28)
	putU16(b2, 12, 1)
	putU16(b2, 14, 2)
	putU16(b2, 16, 11)
	putU16(b2, 18, 12)
	copy(b2[20:], classDefFmt1(30, 1))
	copy(b2[28:], classDefFmt1(40, 2, 3))
	copy(b2[38:], coverageFmt1(30))

	n2 := parseConcreteLookupNode(b2, MaskGPosLookupType(GPosLookupTypePair))
	if n2 == nil || n2.Error() != nil {
		t.Fatalf("expected concrete GPOS2/2 node, err=%v", n2.Error())
	}
	p2 := n2.GPosPayload().PairFmt2
	if p2 == nil || len(p2.ClassRecords) != 1 || len(p2.ClassRecords[0]) != 2 {
		t.Fatalf("expected 1x2 class records")
	}
	if p2.ClassRecords[0][0].Value1.XAdvance != 11 || p2.ClassRecords[0][1].Value1.XAdvance != 12 {
		t.Fatalf("unexpected class value records: %+v", p2.ClassRecords[0])
	}
	if p2.ClassDef1.Lookup(30) != 1 || p2.ClassDef2.Lookup(40) != 2 {
		t.Fatalf("unexpected classDef lookup values")
	}
}

func TestParseConcreteGPosType3And4(t *testing.T) {
	// GPOS3/1: two entry/exit records, coverage=[99]
	b3 := make([]byte, 32)
	putU16(b3, 0, 1)
	putU16(b3, 2, 26)
	putU16(b3, 4, 2)
	putU16(b3, 6, 14)
	putU16(b3, 8, 0)
	putU16(b3, 10, 0)
	putU16(b3, 12, 20)
	putU16(b3, 14, 1)
	putU16(b3, 16, 10)
	putU16(b3, 18, 20)
	putU16(b3, 20, 1)
	putU16(b3, 22, 30)
	putU16(b3, 24, 40)
	copy(b3[26:], coverageFmt1(99))

	n3 := parseConcreteLookupNode(b3, MaskGPosLookupType(GPosLookupTypeCursive))
	if n3 == nil || n3.Error() != nil {
		t.Fatalf("expected concrete GPOS3/1 node, err=%v", n3.Error())
	}
	p3 := n3.GPosPayload().CursiveFmt1
	if p3 == nil || len(p3.Entries) != 2 {
		t.Fatalf("expected two entry/exit records")
	}
	if p3.Entries[0].Entry == nil || p3.Entries[0].Entry.XCoordinate != 10 {
		t.Fatalf("unexpected first entry anchor: %+v", p3.Entries[0].Entry)
	}
	if p3.Entries[1].Exit == nil || p3.Entries[1].Exit.YCoordinate != 40 {
		t.Fatalf("unexpected second exit anchor: %+v", p3.Entries[1].Exit)
	}

	// GPOS4/1: one mark class, one mark, one base; coverage mark=[50], base=[60]
	b4 := make([]byte, 46)
	putU16(b4, 0, 1)
	putU16(b4, 2, 12)
	putU16(b4, 4, 18)
	putU16(b4, 6, 1)
	putU16(b4, 8, 24)
	putU16(b4, 10, 36)
	copy(b4[12:], coverageFmt1(50))
	copy(b4[18:], coverageFmt1(60))
	putU16(b4, 24, 1)
	putU16(b4, 26, 0)
	putU16(b4, 28, 6)
	// mark anchor at 24+6 = 30
	putU16(b4, 30, 1)
	putU16(b4, 32, 1)
	putU16(b4, 34, 1)
	// base array at 36: one base record with one anchor offset (4)
	putU16(b4, 36, 1)
	putU16(b4, 38, 4)
	// base anchor at 36+4 = 40
	putU16(b4, 40, 1)
	putU16(b4, 42, 3)
	putU16(b4, 44, 4)

	n4 := parseConcreteLookupNode(b4, MaskGPosLookupType(GPosLookupTypeMarkToBase))
	if n4 == nil || n4.Error() != nil {
		t.Fatalf("expected concrete GPOS4/1 node, err=%v", n4.Error())
	}
	p4 := n4.GPosPayload().MarkToBaseFmt1
	if p4 == nil || len(p4.MarkRecords) != 1 || len(p4.BaseRecords) != 1 {
		t.Fatalf("expected one mark and one base record")
	}
	if p4.MarkRecords[0].Anchor == nil || p4.BaseRecords[0].Anchors[0] == nil {
		t.Fatalf("expected resolved anchors for mark/base records")
	}
}

func TestParseConcreteGPosType9ExtensionFormat1(t *testing.T) {
	// GPOS9/1 extension wrapping GPOS1/1 at offset 8.
	b := make([]byte, 22)
	putU16(b, 0, 1)
	putU16(b, 2, uint16(GPosLookupTypeSingle))
	putU32(b, 4, 8)
	putU16(b, 8, 1)
	putU16(b, 10, 8)
	putU16(b, 12, uint16(ValueFormatXAdvance))
	putU16(b, 14, 9)
	copy(b[16:], coverageFmt1(77))

	node := parseConcreteLookupNode(b, MaskGPosLookupType(GPosLookupTypeExtensionPos))
	if node == nil || node.Error() != nil {
		t.Fatalf("expected concrete GPOS9/1 node, err=%v", node.Error())
	}
	p := node.GPosPayload().ExtensionFmt1
	if p == nil {
		t.Fatalf("expected GPOS9 extension payload")
	}
	if p.ResolvedType != MaskGPosLookupType(GPosLookupTypeSingle) {
		t.Fatalf("unexpected resolved type: %d", p.ResolvedType)
	}
	if p.Resolved == nil || p.Resolved.GPosPayload() == nil || p.Resolved.GPosPayload().SingleFmt1 == nil {
		t.Fatalf("expected resolved GPOS1/1 payload")
	}
	if p.Resolved.GPosPayload().SingleFmt1.Value.XAdvance != 9 {
		t.Fatalf("expected resolved XAdvance=9, got %d", p.Resolved.GPosPayload().SingleFmt1.Value.XAdvance)
	}
	if _, ok := node.Coverage.Match(77); !ok {
		t.Fatalf("expected extension node coverage forwarded from resolved payload")
	}
}
