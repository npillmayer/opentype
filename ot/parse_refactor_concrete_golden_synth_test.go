package ot

import "testing"

func mustParseConcreteGoldenNode(t *testing.T, b []byte, lookupType LayoutTableLookupType) *LookupNode {
	t.Helper()
	node := parseConcreteLookupNode(b, lookupType)
	if node == nil {
		t.Fatalf("expected concrete lookup node for type=%d", lookupType)
	}
	if node.Error() != nil {
		t.Fatalf("unexpected parse error for type=%d: %v", lookupType, node.Error())
	}
	return node
}

func TestConcreteGoldenSyntheticGSUBMatrix(t *testing.T) {
	t.Run("GSUB1/1 SingleFmt1", func(t *testing.T) {
		b := make([]byte, 12)
		putU16(b, 0, 1)
		putU16(b, 2, 6)
		putU16(b, 4, 3)
		copy(b[6:], coverageFmt1(5))

		node := mustParseConcreteGoldenNode(t, b, GSubLookupTypeSingle)
		if node.LookupType != GSubLookupTypeSingle || node.Format != 1 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGSubPayloadSlots(node.GSubPayload()); slots != 1 {
			t.Fatalf("expected exactly one GSUB payload slot, got %d", slots)
		}
		p := node.GSubPayload().SingleFmt1
		if p == nil || p.DeltaGlyphID != 3 {
			t.Fatalf("unexpected GSUB1/1 payload: %+v", p)
		}
		if inx, ok := node.Coverage.Match(5); !ok || inx != 0 {
			t.Fatalf("expected coverage index 0 for glyph 5, got (%d,%v)", inx, ok)
		}
	})

	t.Run("GSUB5/2 ContextFmt2", func(t *testing.T) {
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

		node := mustParseConcreteGoldenNode(t, b, GSubLookupTypeContext)
		if node.LookupType != GSubLookupTypeContext || node.Format != 2 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGSubPayloadSlots(node.GSubPayload()); slots != 1 {
			t.Fatalf("expected exactly one GSUB payload slot, got %d", slots)
		}
		p := node.GSubPayload().ContextFmt2
		if p == nil || len(p.RuleSets) != 1 || len(p.RuleSets[0]) != 1 {
			t.Fatalf("unexpected GSUB5/2 payload shape")
		}
		if got := p.ClassDef.Lookup(20); got != 1 {
			t.Fatalf("unexpected class def lookup for glyph 20: %d", got)
		}
	})

	t.Run("GSUB6/3 ChainingContextFmt3", func(t *testing.T) {
		b := make([]byte, 46)
		putU16(b, 0, 3)
		putU16(b, 2, 1)
		putU16(b, 4, 22)
		putU16(b, 6, 2)
		putU16(b, 8, 28)
		putU16(b, 10, 34)
		putU16(b, 12, 1)
		putU16(b, 14, 40)
		putU16(b, 16, 1)
		putU16(b, 18, 1)
		putU16(b, 20, 13)
		copy(b[22:], coverageFmt1(201))
		copy(b[28:], coverageFmt1(202))
		copy(b[34:], coverageFmt1(203))
		copy(b[40:], coverageFmt1(204))

		node := mustParseConcreteGoldenNode(t, b, GSubLookupTypeChainingContext)
		if node.LookupType != GSubLookupTypeChainingContext || node.Format != 3 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGSubPayloadSlots(node.GSubPayload()); slots != 1 {
			t.Fatalf("expected exactly one GSUB payload slot, got %d", slots)
		}
		p := node.GSubPayload().ChainingContextFmt3
		if p == nil || len(p.BacktrackCoverages) != 1 || len(p.InputCoverages) != 2 || len(p.LookaheadCoverages) != 1 {
			t.Fatalf("unexpected GSUB6/3 payload shape")
		}
		if len(p.Records) != 1 || p.Records[0].LookupListIndex != 13 {
			t.Fatalf("unexpected GSUB6/3 lookup records: %+v", p.Records)
		}
	})

	t.Run("GSUB8/1 ReverseChainingFmt1", func(t *testing.T) {
		b := make([]byte, 36)
		putU16(b, 0, 1)
		putU16(b, 2, 18)
		putU16(b, 4, 1)
		putU16(b, 6, 24)
		putU16(b, 8, 1)
		putU16(b, 10, 30)
		putU16(b, 12, 2)
		putU16(b, 14, 301)
		putU16(b, 16, 302)
		copy(b[18:], coverageFmt1(200))
		copy(b[24:], coverageFmt1(201))
		copy(b[30:], coverageFmt1(202))

		node := mustParseConcreteGoldenNode(t, b, GSubLookupTypeReverseChaining)
		if node.LookupType != GSubLookupTypeReverseChaining || node.Format != 1 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGSubPayloadSlots(node.GSubPayload()); slots != 1 {
			t.Fatalf("expected exactly one GSUB payload slot, got %d", slots)
		}
		p := node.GSubPayload().ReverseChainingFmt1
		if p == nil || len(p.SubstituteGlyphIDs) != 2 {
			t.Fatalf("unexpected GSUB8/1 payload: %+v", p)
		}
		if p.SubstituteGlyphIDs[0] != 301 || p.SubstituteGlyphIDs[1] != 302 {
			t.Fatalf("unexpected substitute list: %v", p.SubstituteGlyphIDs)
		}
	})

	t.Run("GSUB7/1 ExtensionFmt1", func(t *testing.T) {
		b := make([]byte, 20)
		putU16(b, 0, 1)
		putU16(b, 2, 1)
		putU32(b, 4, 8)
		putU16(b, 8, 1)
		putU16(b, 10, 6)
		putU16(b, 12, 5)
		copy(b[14:], coverageFmt1(42))

		node := mustParseConcreteGoldenNode(t, b, GSubLookupTypeExtensionSubs)
		if node.LookupType != GSubLookupTypeExtensionSubs || node.Format != 1 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGSubPayloadSlots(node.GSubPayload()); slots != 1 {
			t.Fatalf("expected exactly one GSUB payload slot on extension node, got %d", slots)
		}
		p := node.GSubPayload().ExtensionFmt1
		if p == nil || p.ResolvedType != GSubLookupTypeSingle || p.Resolved == nil {
			t.Fatalf("unexpected GSUB7/1 extension payload: %+v", p)
		}
		if p.Resolved.GSubPayload() == nil || countGSubPayloadSlots(p.Resolved.GSubPayload()) != 1 {
			t.Fatalf("expected exactly one resolved GSUB payload slot")
		}
		if p.Resolved.GSubPayload().SingleFmt1 == nil || p.Resolved.GSubPayload().SingleFmt1.DeltaGlyphID != 5 {
			t.Fatalf("unexpected resolved GSUB payload")
		}
	})
}

func TestConcreteGoldenSyntheticGPOSMatrix(t *testing.T) {
	t.Run("GPOS1/2 SingleFmt2", func(t *testing.T) {
		b := make([]byte, 18)
		putU16(b, 0, 2)
		putU16(b, 2, 12)
		putU16(b, 4, uint16(ValueFormatXPlacement))
		putU16(b, 6, 2)
		putU16(b, 8, 5)
		putU16(b, 10, 6)
		copy(b[12:], coverageFmt1(20, 21))

		node := mustParseConcreteGoldenNode(t, b, MaskGPosLookupType(GPosLookupTypeSingle))
		if node.LookupType != MaskGPosLookupType(GPosLookupTypeSingle) || node.Format != 2 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGPosPayloadSlots(node.GPosPayload()); slots != 1 {
			t.Fatalf("expected exactly one GPOS payload slot, got %d", slots)
		}
		p := node.GPosPayload().SingleFmt2
		if p == nil || len(p.Values) != 2 || p.Values[0].XPlacement != 5 || p.Values[1].XPlacement != 6 {
			t.Fatalf("unexpected GPOS1/2 payload: %+v", p)
		}
	})

	t.Run("GPOS2/2 PairFmt2", func(t *testing.T) {
		b := make([]byte, 44)
		putU16(b, 0, 2)
		putU16(b, 2, 38)
		putU16(b, 4, uint16(ValueFormatXAdvance))
		putU16(b, 6, 0)
		putU16(b, 8, 20)
		putU16(b, 10, 28)
		putU16(b, 12, 1)
		putU16(b, 14, 2)
		putU16(b, 16, 11)
		putU16(b, 18, 12)
		copy(b[20:], classDefFmt1(30, 1))
		copy(b[28:], classDefFmt1(40, 2, 3))
		copy(b[38:], coverageFmt1(30))

		node := mustParseConcreteGoldenNode(t, b, MaskGPosLookupType(GPosLookupTypePair))
		if node.LookupType != MaskGPosLookupType(GPosLookupTypePair) || node.Format != 2 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGPosPayloadSlots(node.GPosPayload()); slots != 1 {
			t.Fatalf("expected exactly one GPOS payload slot, got %d", slots)
		}
		p := node.GPosPayload().PairFmt2
		if p == nil || len(p.ClassRecords) != 1 || len(p.ClassRecords[0]) != 2 {
			t.Fatalf("unexpected GPOS2/2 payload shape")
		}
		if p.ClassRecords[0][0].Value1.XAdvance != 11 || p.ClassRecords[0][1].Value1.XAdvance != 12 {
			t.Fatalf("unexpected class pair values")
		}
	})

	t.Run("GPOS3/1 CursiveFmt1", func(t *testing.T) {
		b := make([]byte, 32)
		putU16(b, 0, 1)
		putU16(b, 2, 26)
		putU16(b, 4, 2)
		putU16(b, 6, 14)
		putU16(b, 8, 0)
		putU16(b, 10, 0)
		putU16(b, 12, 20)
		putU16(b, 14, 1)
		putU16(b, 16, 10)
		putU16(b, 18, 20)
		putU16(b, 20, 1)
		putU16(b, 22, 30)
		putU16(b, 24, 40)
		copy(b[26:], coverageFmt1(99))

		node := mustParseConcreteGoldenNode(t, b, MaskGPosLookupType(GPosLookupTypeCursive))
		if node.LookupType != MaskGPosLookupType(GPosLookupTypeCursive) || node.Format != 1 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGPosPayloadSlots(node.GPosPayload()); slots != 1 {
			t.Fatalf("expected exactly one GPOS payload slot, got %d", slots)
		}
		p := node.GPosPayload().CursiveFmt1
		if p == nil || len(p.Entries) != 2 {
			t.Fatalf("unexpected GPOS3/1 payload")
		}
		if p.Entries[0].Entry == nil || p.Entries[1].Exit == nil {
			t.Fatalf("expected concrete entry/exit anchors")
		}
	})

	t.Run("GPOS4/1 MarkToBaseFmt1", func(t *testing.T) {
		b := make([]byte, 46)
		putU16(b, 0, 1)
		putU16(b, 2, 12)
		putU16(b, 4, 18)
		putU16(b, 6, 1)
		putU16(b, 8, 24)
		putU16(b, 10, 36)
		copy(b[12:], coverageFmt1(50))
		copy(b[18:], coverageFmt1(60))
		putU16(b, 24, 1)
		putU16(b, 26, 0)
		putU16(b, 28, 6)
		putU16(b, 30, 1)
		putU16(b, 32, 1)
		putU16(b, 34, 1)
		putU16(b, 36, 1)
		putU16(b, 38, 4)
		putU16(b, 40, 1)
		putU16(b, 42, 3)
		putU16(b, 44, 4)

		node := mustParseConcreteGoldenNode(t, b, MaskGPosLookupType(GPosLookupTypeMarkToBase))
		if node.LookupType != MaskGPosLookupType(GPosLookupTypeMarkToBase) || node.Format != 1 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGPosPayloadSlots(node.GPosPayload()); slots != 1 {
			t.Fatalf("expected exactly one GPOS payload slot, got %d", slots)
		}
		p := node.GPosPayload().MarkToBaseFmt1
		if p == nil || len(p.MarkRecords) != 1 || len(p.BaseRecords) != 1 {
			t.Fatalf("unexpected GPOS4/1 payload")
		}
		if p.MarkRecords[0].Anchor == nil || p.BaseRecords[0].Anchors[0] == nil {
			t.Fatalf("expected concrete mark/base anchors")
		}
	})

	t.Run("GPOS7/3 ContextFmt3", func(t *testing.T) {
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

		node := mustParseConcreteGoldenNode(t, b, MaskGPosLookupType(GPosLookupTypeContextPos))
		if node.LookupType != MaskGPosLookupType(GPosLookupTypeContextPos) || node.Format != 3 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGPosPayloadSlots(node.GPosPayload()); slots != 1 {
			t.Fatalf("expected exactly one GPOS payload slot, got %d", slots)
		}
		p := node.GPosPayload().ContextFmt3
		if p == nil || len(p.InputCoverages) != 2 || len(p.Records) != 1 {
			t.Fatalf("unexpected GPOS7/3 payload")
		}
	})

	t.Run("GPOS8/2 ChainingContextFmt2", func(t *testing.T) {
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

		node := mustParseConcreteGoldenNode(t, b, MaskGPosLookupType(GPosLookupTypeChainedContextPos))
		if node.LookupType != MaskGPosLookupType(GPosLookupTypeChainedContextPos) || node.Format != 2 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGPosPayloadSlots(node.GPosPayload()); slots != 1 {
			t.Fatalf("expected exactly one GPOS payload slot, got %d", slots)
		}
		p := node.GPosPayload().ChainingContextFmt2
		if p == nil || len(p.RuleSets) != 1 || len(p.RuleSets[0]) != 1 {
			t.Fatalf("unexpected GPOS8/2 payload")
		}
		if p.BacktrackClassDef.Lookup(30) != 5 || p.InputClassDef.Lookup(40) != 6 || p.LookaheadClassDef.Lookup(50) != 7 {
			t.Fatalf("unexpected class def lookups in GPOS8/2 payload")
		}
	})

	t.Run("GPOS9/1 ExtensionFmt1", func(t *testing.T) {
		b := make([]byte, 22)
		putU16(b, 0, 1)
		putU16(b, 2, uint16(GPosLookupTypeSingle))
		putU32(b, 4, 8)
		putU16(b, 8, 1)
		putU16(b, 10, 8)
		putU16(b, 12, uint16(ValueFormatXAdvance))
		putU16(b, 14, 9)
		copy(b[16:], coverageFmt1(77))

		node := mustParseConcreteGoldenNode(t, b, MaskGPosLookupType(GPosLookupTypeExtensionPos))
		if node.LookupType != MaskGPosLookupType(GPosLookupTypeExtensionPos) || node.Format != 1 {
			t.Fatalf("unexpected node header: type=%d format=%d", node.LookupType, node.Format)
		}
		if slots := countGPosPayloadSlots(node.GPosPayload()); slots != 1 {
			t.Fatalf("expected exactly one GPOS payload slot on extension node, got %d", slots)
		}
		p := node.GPosPayload().ExtensionFmt1
		if p == nil || p.ResolvedType != MaskGPosLookupType(GPosLookupTypeSingle) || p.Resolved == nil {
			t.Fatalf("unexpected GPOS9/1 extension payload")
		}
		if p.Resolved.GPosPayload() == nil || countGPosPayloadSlots(p.Resolved.GPosPayload()) != 1 {
			t.Fatalf("expected exactly one resolved GPOS payload slot")
		}
		if p.Resolved.GPosPayload().SingleFmt1 == nil || p.Resolved.GPosPayload().SingleFmt1.Value.XAdvance != 9 {
			t.Fatalf("unexpected resolved GPOS payload")
		}
	})
}
