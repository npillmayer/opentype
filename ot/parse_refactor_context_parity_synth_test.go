package ot

import "testing"

func TestSyntheticConcreteGPosType7Formats(t *testing.T) {
	// Format 1
	b1 := make([]byte, 28)
	putU16(b1, 0, 1)
	putU16(b1, 2, 8)
	putU16(b1, 4, 1)
	putU16(b1, 6, 14)
	copy(b1[8:], coverageFmt1(10))
	putU16(b1, 14, 1)
	putU16(b1, 16, 4)
	putU16(b1, 18, 2)
	putU16(b1, 20, 1)
	putU16(b1, 22, 11)
	putU16(b1, 24, 1)
	putU16(b1, 26, 7)

	n1 := parseConcreteLookupNode(b1, MaskGPosLookupType(GPosLookupTypeContextPos))
	if n1 == nil || n1.Error() != nil {
		t.Fatalf("expected concrete GPOS7/1 node, err=%v", n1.Error())
	}
	p1 := n1.GPosPayload().ContextFmt1
	if p1 == nil {
		t.Fatalf("expected concrete GPOS7/1 payload")
	}
	if len(p1.RuleSets) != 1 || len(p1.RuleSets[0]) != 1 {
		t.Fatalf("unexpected GPOS7/1 rule-set shape: %#v", p1.RuleSets)
	}

	// Format 2
	b2 := make([]byte, 40)
	putU16(b2, 0, 2)
	putU16(b2, 2, 10)
	putU16(b2, 4, 16)
	putU16(b2, 6, 1)
	putU16(b2, 8, 26)
	copy(b2[10:], coverageFmt1(15))
	putU16(b2, 16, 1)
	putU16(b2, 18, 20)
	putU16(b2, 20, 2)
	putU16(b2, 22, 1)
	putU16(b2, 24, 2)
	putU16(b2, 26, 1)
	putU16(b2, 28, 4)
	putU16(b2, 30, 2)
	putU16(b2, 32, 1)
	putU16(b2, 34, 2)
	putU16(b2, 36, 0)
	putU16(b2, 38, 9)

	n2 := parseConcreteLookupNode(b2, MaskGPosLookupType(GPosLookupTypeContextPos))
	if n2 == nil || n2.Error() != nil {
		t.Fatalf("expected concrete GPOS7/2 node, err=%v", n2.Error())
	}
	p2 := n2.GPosPayload().ContextFmt2
	if p2 == nil {
		t.Fatalf("expected concrete GPOS7/2 payload")
	}
	if p2.ClassDef.format != 1 {
		t.Fatalf("expected GPOS7/2 class-def format 1, got %d", p2.ClassDef.format)
	}
	if len(p2.RuleSets) != 1 || len(p2.RuleSets[0]) != 1 {
		t.Fatalf("unexpected GPOS7/2 rule-set shape: %#v", p2.RuleSets)
	}

	// Format 3
	b3 := make([]byte, 26)
	putU16(b3, 0, 3)
	putU16(b3, 2, 2)
	putU16(b3, 4, 1)
	putU16(b3, 6, 14)
	putU16(b3, 8, 20)
	putU16(b3, 10, 0)
	putU16(b3, 12, 11)
	copy(b3[14:], coverageFmt1(30))
	copy(b3[20:], coverageFmt1(31))

	n3 := parseConcreteLookupNode(b3, MaskGPosLookupType(GPosLookupTypeContextPos))
	if n3 == nil || n3.Error() != nil {
		t.Fatalf("expected concrete GPOS7/3 node, err=%v", n3.Error())
	}
	p3 := n3.GPosPayload().ContextFmt3
	if p3 == nil {
		t.Fatalf("expected concrete GPOS7/3 payload")
	}
	if len(p3.InputCoverages) != 2 {
		t.Fatalf("expected 2 input coverages, got %d", len(p3.InputCoverages))
	}
	if len(p3.Records) != 1 {
		t.Fatalf("expected 1 lookup record, got %d", len(p3.Records))
	}
}

func TestSyntheticConcreteGPosType8Formats(t *testing.T) {
	// Format 1
	b1 := make([]byte, 36)
	putU16(b1, 0, 1)
	putU16(b1, 2, 8)
	putU16(b1, 4, 1)
	putU16(b1, 6, 14)
	copy(b1[8:], coverageFmt1(100))
	putU16(b1, 14, 1)
	putU16(b1, 16, 4)
	putU16(b1, 18, 1)
	putU16(b1, 20, 101)
	putU16(b1, 22, 2)
	putU16(b1, 24, 102)
	putU16(b1, 26, 1)
	putU16(b1, 28, 103)
	putU16(b1, 30, 1)
	putU16(b1, 32, 1)
	putU16(b1, 34, 9)

	n1 := parseConcreteLookupNode(b1, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	if n1 == nil || n1.Error() != nil {
		t.Fatalf("expected concrete GPOS8/1 node, err=%v", n1.Error())
	}
	p1 := n1.GPosPayload().ChainingContextFmt1
	if p1 == nil {
		t.Fatalf("expected concrete GPOS8/1 payload")
	}
	if len(p1.RuleSets) != 1 || len(p1.RuleSets[0]) != 1 {
		t.Fatalf("unexpected GPOS8/1 rule-set shape: %#v", p1.RuleSets)
	}

	// Format 2
	b2 := make([]byte, 66)
	putU16(b2, 0, 2)
	putU16(b2, 2, 14)
	putU16(b2, 4, 20)
	putU16(b2, 6, 28)
	putU16(b2, 8, 36)
	putU16(b2, 10, 1)
	putU16(b2, 12, 44)
	copy(b2[14:], coverageFmt1(55))
	copy(b2[20:], classDefFmt1(30, 5))
	copy(b2[28:], classDefFmt1(40, 6))
	copy(b2[36:], classDefFmt1(50, 7))
	putU16(b2, 44, 1)
	putU16(b2, 46, 4)
	putU16(b2, 48, 1)
	putU16(b2, 50, 5)
	putU16(b2, 52, 2)
	putU16(b2, 54, 6)
	putU16(b2, 56, 1)
	putU16(b2, 58, 7)
	putU16(b2, 60, 1)
	putU16(b2, 62, 0)
	putU16(b2, 64, 12)

	n2 := parseConcreteLookupNode(b2, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	if n2 == nil || n2.Error() != nil {
		t.Fatalf("expected concrete GPOS8/2 node, err=%v", n2.Error())
	}
	p2 := n2.GPosPayload().ChainingContextFmt2
	if p2 == nil {
		t.Fatalf("expected concrete GPOS8/2 payload")
	}
	if p2.BacktrackClassDef.format != 1 || p2.InputClassDef.format != 1 || p2.LookaheadClassDef.format != 1 {
		t.Fatalf("unexpected GPOS8/2 class-def formats: %d/%d/%d",
			p2.BacktrackClassDef.format, p2.InputClassDef.format, p2.LookaheadClassDef.format)
	}
	if len(p2.RuleSets) != 1 || len(p2.RuleSets[0]) != 1 {
		t.Fatalf("unexpected GPOS8/2 rule-set shape: %#v", p2.RuleSets)
	}

	// Format 3
	b3 := make([]byte, 46)
	putU16(b3, 0, 3)
	putU16(b3, 2, 1)
	putU16(b3, 4, 22)
	putU16(b3, 6, 2)
	putU16(b3, 8, 28)
	putU16(b3, 10, 34)
	putU16(b3, 12, 1)
	putU16(b3, 14, 40)
	putU16(b3, 16, 1)
	putU16(b3, 18, 1)
	putU16(b3, 20, 13)
	copy(b3[22:], coverageFmt1(201))
	copy(b3[28:], coverageFmt1(202))
	copy(b3[34:], coverageFmt1(203))
	copy(b3[40:], coverageFmt1(204))

	n3 := parseConcreteLookupNode(b3, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	if n3 == nil || n3.Error() != nil {
		t.Fatalf("expected concrete GPOS8/3 node, err=%v", n3.Error())
	}
	p3 := n3.GPosPayload().ChainingContextFmt3
	if p3 == nil {
		t.Fatalf("expected concrete GPOS8/3 payload")
	}
	if len(p3.BacktrackCoverages) != 1 || len(p3.InputCoverages) != 2 || len(p3.LookaheadCoverages) != 1 {
		t.Fatalf("unexpected GPOS8/3 coverage counts: backtrack=%d input=%d lookahead=%d",
			len(p3.BacktrackCoverages), len(p3.InputCoverages), len(p3.LookaheadCoverages))
	}
	if len(p3.Records) != 1 {
		t.Fatalf("expected 1 lookup record, got %d", len(p3.Records))
	}
}
