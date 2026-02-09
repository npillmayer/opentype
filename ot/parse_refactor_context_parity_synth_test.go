package ot

import "testing"

func TestConcreteLegacyParitySyntheticGPosType7Formats(t *testing.T) {
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

	l1 := parseLookupSubtable(b1, MaskGPosLookupType(GPosLookupTypeContextPos))
	n1 := parseConcreteLookupNode(b1, MaskGPosLookupType(GPosLookupTypeContextPos))
	if n1 == nil || n1.Error() != nil {
		t.Fatalf("expected concrete GPOS7/1 node, err=%v", n1.Error())
	}
	if n1.GPosPayload().ContextFmt1 == nil {
		t.Fatalf("expected concrete GPOS7/1 context payload")
	}
	if l1.Index == nil {
		t.Fatalf("expected legacy GPOS7/1 index")
	}
	if got, want := len(n1.GPosPayload().ContextFmt1.RuleSets), l1.Index.Size(); got != want {
		t.Fatalf("GPOS7/1 ruleset count mismatch: legacy=%d concrete=%d", want, got)
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

	l2 := parseLookupSubtable(b2, MaskGPosLookupType(GPosLookupTypeContextPos))
	n2 := parseConcreteLookupNode(b2, MaskGPosLookupType(GPosLookupTypeContextPos))
	if n2 == nil || n2.Error() != nil {
		t.Fatalf("expected concrete GPOS7/2 node, err=%v", n2.Error())
	}
	if n2.GPosPayload().ContextFmt2 == nil {
		t.Fatalf("expected concrete GPOS7/2 context payload")
	}
	if l2.Index == nil {
		t.Fatalf("expected legacy GPOS7/2 index")
	}
	if got, want := len(n2.GPosPayload().ContextFmt2.RuleSets), l2.Index.Size(); got != want {
		t.Fatalf("GPOS7/2 ruleset count mismatch: legacy=%d concrete=%d", want, got)
	}
	legacyCtx2, ok := asSequenceContext(l2.Support)
	if !ok || len(legacyCtx2.ClassDefs) != 1 {
		t.Fatalf("expected legacy context info for GPOS7/2")
	}
	if n2.GPosPayload().ContextFmt2.ClassDef.format != legacyCtx2.ClassDefs[0].format {
		t.Fatalf("GPOS7/2 class-def format mismatch: legacy=%d concrete=%d",
			legacyCtx2.ClassDefs[0].format, n2.GPosPayload().ContextFmt2.ClassDef.format)
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

	l3 := parseLookupSubtable(b3, MaskGPosLookupType(GPosLookupTypeContextPos))
	n3 := parseConcreteLookupNode(b3, MaskGPosLookupType(GPosLookupTypeContextPos))
	if n3 == nil || n3.Error() != nil {
		t.Fatalf("expected concrete GPOS7/3 node, err=%v", n3.Error())
	}
	if n3.GPosPayload().ContextFmt3 == nil {
		t.Fatalf("expected concrete GPOS7/3 context payload")
	}
	legacyCtx3, ok := asSequenceContext(l3.Support)
	if !ok {
		t.Fatalf("expected legacy sequence context for GPOS7/3")
	}
	got := len(n3.GPosPayload().ContextFmt3.InputCoverages)
	want := len(legacyCtx3.InputCoverage)
	if want == 0 {
		// Legacy GPOS7/3 materialization may be sparse; concrete path must still decode.
		if got == 0 {
			t.Fatalf("expected non-empty concrete GPOS7/3 input coverages")
		}
	} else if got != want {
		t.Fatalf("GPOS7/3 input coverage count mismatch: legacy=%d concrete=%d", want, got)
	}
}

func TestConcreteLegacyParitySyntheticGPosType8Formats(t *testing.T) {
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

	l1 := parseLookupSubtable(b1, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	n1 := parseConcreteLookupNode(b1, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	if n1 == nil || n1.Error() != nil {
		t.Fatalf("expected concrete GPOS8/1 node, err=%v", n1.Error())
	}
	if n1.GPosPayload().ChainingContextFmt1 == nil {
		t.Fatalf("expected concrete GPOS8/1 chaining payload")
	}
	if l1.Index == nil {
		t.Fatalf("expected legacy GPOS8/1 index")
	}
	if got, want := len(n1.GPosPayload().ChainingContextFmt1.RuleSets), l1.Index.Size(); got != want {
		t.Fatalf("GPOS8/1 ruleset count mismatch: legacy=%d concrete=%d", want, got)
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

	l2 := parseLookupSubtable(b2, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	n2 := parseConcreteLookupNode(b2, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	if n2 == nil || n2.Error() != nil {
		t.Fatalf("expected concrete GPOS8/2 node, err=%v", n2.Error())
	}
	if n2.GPosPayload().ChainingContextFmt2 == nil {
		t.Fatalf("expected concrete GPOS8/2 chaining payload")
	}
	if l2.Index == nil {
		t.Fatalf("expected legacy GPOS8/2 index")
	}
	if got, want := len(n2.GPosPayload().ChainingContextFmt2.RuleSets), l2.Index.Size(); got != want {
		t.Fatalf("GPOS8/2 ruleset count mismatch: legacy=%d concrete=%d", want, got)
	}
	legacyCtx2, ok := asSequenceContext(l2.Support)
	if !ok || len(legacyCtx2.ClassDefs) != 3 {
		t.Fatalf("expected legacy class context for GPOS8/2")
	}
	if n2.GPosPayload().ChainingContextFmt2.BacktrackClassDef.format != legacyCtx2.ClassDefs[0].format ||
		n2.GPosPayload().ChainingContextFmt2.InputClassDef.format != legacyCtx2.ClassDefs[1].format ||
		n2.GPosPayload().ChainingContextFmt2.LookaheadClassDef.format != legacyCtx2.ClassDefs[2].format {
		t.Fatalf("GPOS8/2 class-def format mismatch")
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

	l3 := parseLookupSubtable(b3, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	n3 := parseConcreteLookupNode(b3, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
	if n3 == nil || n3.Error() != nil {
		t.Fatalf("expected concrete GPOS8/3 node, err=%v", n3.Error())
	}
	if n3.GPosPayload().ChainingContextFmt3 == nil {
		t.Fatalf("expected concrete GPOS8/3 chaining payload")
	}
	legacyCtx3, ok := asSequenceContext(l3.Support)
	if !ok {
		t.Fatalf("expected legacy sequence context for GPOS8/3")
	}
	p3 := n3.GPosPayload().ChainingContextFmt3
	if len(p3.BacktrackCoverages) != len(legacyCtx3.BacktrackCoverage) ||
		len(p3.InputCoverages) != len(legacyCtx3.InputCoverage) ||
		len(p3.LookaheadCoverages) != len(legacyCtx3.LookaheadCoverage) {
		t.Fatalf("GPOS8/3 coverage-count mismatch")
	}
	if !sequenceLookupRecordsEqual(p3.Records, l3.LookupRecords) {
		t.Fatalf("GPOS8/3 lookup-record mismatch with legacy")
	}
}
