package ot

import "testing"

func TestParseConcreteGSUBMalformedInputs(t *testing.T) {
	t.Run("GSUB5Format3Truncated", func(t *testing.T) {
		// format=3, glyphCount=2, seqLookupCount=1 but payload truncated
		b := make([]byte, 10)
		putU16(b, 0, 3)
		putU16(b, 2, 2)
		putU16(b, 4, 1)
		putU16(b, 6, 8)
		putU16(b, 8, 0)
		node := parseConcreteLookupNode(b, GSubLookupTypeContext)
		if node == nil || node.Error() == nil {
			t.Fatalf("expected parse error for truncated GSUB5/3")
		}
	})

	t.Run("GSUB6Format3Truncated", func(t *testing.T) {
		// format=3 with backtrack/input/lookahead counts but missing lookup records.
		b := make([]byte, 12)
		putU16(b, 0, 3)
		putU16(b, 2, 1) // backtrack count
		putU16(b, 4, 10)
		putU16(b, 6, 1) // input count
		putU16(b, 8, 10)
		putU16(b, 10, 0) // lookahead count; missing seqLookupCount and records
		node := parseConcreteLookupNode(b, GSubLookupTypeChainingContext)
		if node == nil || node.Error() == nil {
			t.Fatalf("expected parse error for truncated GSUB6/3")
		}
	})

	t.Run("GSUB7RecursiveExtension", func(t *testing.T) {
		// extension format1 recursively pointing to extension type
		b := make([]byte, 8)
		putU16(b, 0, 1)
		putU16(b, 2, uint16(GSubLookupTypeExtensionSubs))
		putU32(b, 4, 4)
		node := parseConcreteLookupNode(b, GSubLookupTypeExtensionSubs)
		if node == nil || node.Error() == nil {
			t.Fatalf("expected parse error for recursive GSUB extension")
		}
	})
}

func TestParseConcreteGPOSMalformedInputs(t *testing.T) {
	t.Run("GPOS2Format2ClassRecordsTruncated", func(t *testing.T) {
		// format=2 with valid coverage/classDef links but class1/2 counts too large for payload.
		b := make([]byte, 44)
		putU16(b, 0, 2)
		putU16(b, 2, 38) // coverage
		putU16(b, 4, uint16(ValueFormatXAdvance))
		putU16(b, 6, 0)
		putU16(b, 8, 20) // classDef1
		putU16(b, 10, 28)
		putU16(b, 12, 50) // class1Count
		putU16(b, 14, 50) // class2Count -> impossible for buffer size
		copy(b[20:], classDefFmt1(30, 1))
		copy(b[28:], classDefFmt1(40, 2, 3))
		copy(b[38:], coverageFmt1(30))
		node := parseConcreteLookupNode(b, MaskGPosLookupType(GPosLookupTypePair))
		if node == nil || node.Error() == nil {
			t.Fatalf("expected parse error for truncated GPOS2/2 class records")
		}
	})

	t.Run("GPOS4MarkArrayOffsetOutOfBounds", func(t *testing.T) {
		b := make([]byte, 20)
		putU16(b, 0, 1)
		putU16(b, 2, 12) // mark coverage
		putU16(b, 4, 12) // base coverage (reuse)
		putU16(b, 6, 1)  // markClassCount
		putU16(b, 8, 200)
		putU16(b, 10, 0)
		copy(b[12:], coverageFmt1(10))
		node := parseConcreteLookupNode(b, MaskGPosLookupType(GPosLookupTypeMarkToBase))
		if node == nil || node.Error() == nil {
			t.Fatalf("expected parse error for out-of-bounds mark array offset in GPOS4")
		}
	})

	t.Run("GPOS9RecursiveExtension", func(t *testing.T) {
		b := make([]byte, 8)
		putU16(b, 0, 1)
		putU16(b, 2, uint16(GPosLookupTypeExtensionPos))
		putU32(b, 4, 4)
		node := parseConcreteLookupNode(b, MaskGPosLookupType(GPosLookupTypeExtensionPos))
		if node == nil || node.Error() == nil {
			t.Fatalf("expected parse error for recursive GPOS extension")
		}
	})
}
