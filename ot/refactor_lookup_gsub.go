package ot

// GSubLookupPayload is a typed payload scaffold for GSUB lookup-subtable variants.
// Exactly one pointer field is expected to be non-nil for a concrete GSUB node.
type GSubLookupPayload struct {
	SingleFmt1          *GSubSingleFmt1Payload
	SingleFmt2          *GSubSingleFmt2Payload
	MultipleFmt1        *GSubMultipleFmt1Payload
	AlternateFmt1       *GSubAlternateFmt1Payload
	LigatureFmt1        *GSubLigatureFmt1Payload
	ContextFmt1         *GSubContextFmt1Payload
	ContextFmt2         *GSubContextFmt2Payload
	ContextFmt3         *GSubContextFmt3Payload
	ChainingContextFmt1 *GSubChainingContextFmt1Payload
	ChainingContextFmt2 *GSubChainingContextFmt2Payload
	ChainingContextFmt3 *GSubChainingContextFmt3Payload
	ExtensionFmt1       *GSubExtensionFmt1Payload
	ReverseChainingFmt1 *GSubReverseChainingFmt1Payload
}

type GSubSequenceRule struct {
	InputGlyphs []GlyphIndex
	Records     []SequenceLookupRecord
}

type GSubClassSequenceRule struct {
	InputClasses []uint16
	Records      []SequenceLookupRecord
}

type GSubChainedSequenceRule struct {
	Backtrack []GlyphIndex
	Input     []GlyphIndex
	Lookahead []GlyphIndex
	Records   []SequenceLookupRecord
}

type GSubChainedClassRule struct {
	Backtrack []uint16
	Input     []uint16
	Lookahead []uint16
	Records   []SequenceLookupRecord
}

type GSubSingleFmt1Payload struct {
	DeltaGlyphID int16
}

type GSubSingleFmt2Payload struct {
	SubstituteGlyphIDs []GlyphIndex
}

type GSubMultipleFmt1Payload struct {
	Sequences [][]GlyphIndex
}

type GSubAlternateFmt1Payload struct {
	Alternates [][]GlyphIndex
}

type GSubLigatureRule struct {
	Components []GlyphIndex
	Ligature   GlyphIndex
}

type GSubLigatureFmt1Payload struct {
	LigatureSets [][]GSubLigatureRule
}

type GSubContextFmt1Payload struct {
	RuleSets [][]GSubSequenceRule
}

type GSubContextFmt2Payload struct {
	ClassDef ClassDefinitions
	RuleSets [][]GSubClassSequenceRule
}

type GSubContextFmt3Payload struct {
	InputCoverages []Coverage
	Records        []SequenceLookupRecord
}

type GSubChainingContextFmt1Payload struct {
	RuleSets [][]GSubChainedSequenceRule
}

type GSubChainingContextFmt2Payload struct {
	BacktrackClassDef ClassDefinitions
	InputClassDef     ClassDefinitions
	LookaheadClassDef ClassDefinitions
	RuleSets          [][]GSubChainedClassRule
}

type GSubChainingContextFmt3Payload struct {
	BacktrackCoverages []Coverage
	InputCoverages     []Coverage
	LookaheadCoverages []Coverage
	Records            []SequenceLookupRecord
}

type GSubExtensionFmt1Payload struct {
	ResolvedType LayoutTableLookupType
	Resolved     *LookupNode
}

type GSubReverseChainingFmt1Payload struct {
	BacktrackCoverages []Coverage
	LookaheadCoverages []Coverage
	SubstituteGlyphIDs []GlyphIndex
}
