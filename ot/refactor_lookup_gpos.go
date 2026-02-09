package ot

// GPosLookupPayload is a typed payload scaffold for GPOS lookup-subtable variants.
// Exactly one pointer field is expected to be non-nil for a concrete GPOS node.
type GPosLookupPayload struct {
	SingleFmt1          *GPosSingleFmt1Payload
	SingleFmt2          *GPosSingleFmt2Payload
	PairFmt1            *GPosPairFmt1Payload
	PairFmt2            *GPosPairFmt2Payload
	CursiveFmt1         *GPosCursiveFmt1Payload
	MarkToBaseFmt1      *GPosMarkToBaseFmt1Payload
	MarkToLigatureFmt1  *GPosMarkToLigatureFmt1Payload
	MarkToMarkFmt1      *GPosMarkToMarkFmt1Payload
	ContextFmt1         *GPosContextFmt1Payload
	ContextFmt2         *GPosContextFmt2Payload
	ContextFmt3         *GPosContextFmt3Payload
	ChainingContextFmt1 *GPosChainingContextFmt1Payload
	ChainingContextFmt2 *GPosChainingContextFmt2Payload
	ChainingContextFmt3 *GPosChainingContextFmt3Payload
	ExtensionFmt1       *GPosExtensionFmt1Payload
}

type GPosSingleFmt1Payload struct {
	ValueFormat ValueFormat
	Value       ValueRecord
}

type GPosSingleFmt2Payload struct {
	ValueFormat ValueFormat
	Values      []ValueRecord
}

type GPosPairFmt1Payload struct {
	ValueFormat1 ValueFormat
	ValueFormat2 ValueFormat
	PairSets     [][]PairValueRecord
}

type GPosClass2ValueRecord struct {
	Value1 ValueRecord
	Value2 ValueRecord
}

type GPosPairFmt2Payload struct {
	ValueFormat1 ValueFormat
	ValueFormat2 ValueFormat
	ClassDef1    ClassDefinitions
	ClassDef2    ClassDefinitions
	Class1Count  uint16
	Class2Count  uint16
	ClassRecords [][]GPosClass2ValueRecord
}

type GPosEntryExitAnchor struct {
	Entry *Anchor
	Exit  *Anchor
}

type GPosCursiveFmt1Payload struct {
	Entries []GPosEntryExitAnchor
}

type GPosMarkAttachRecord struct {
	Class  uint16
	Anchor *Anchor
}

type GPosBaseAttachRecord struct {
	Anchors []*Anchor
}

type GPosMarkToBaseFmt1Payload struct {
	BaseCoverage   Coverage
	MarkClassCount uint16
	MarkRecords    []GPosMarkAttachRecord
	BaseRecords    []GPosBaseAttachRecord
}

type GPosLigatureAttachRecord struct {
	ComponentAnchors [][]*Anchor
}

type GPosMarkToLigatureFmt1Payload struct {
	LigatureCoverage Coverage
	MarkClassCount   uint16
	MarkRecords      []GPosMarkAttachRecord
	LigatureRecords  []GPosLigatureAttachRecord
}

type GPosMarkToMarkFmt1Payload struct {
	Mark2Coverage  Coverage
	MarkClassCount uint16
	Mark1Records   []GPosMarkAttachRecord
	Mark2Records   []GPosBaseAttachRecord
}

type GPosSequenceRule struct {
	InputGlyphs []GlyphIndex
	Records     []SequenceLookupRecord
}

type GPosClassSequenceRule struct {
	InputClasses []uint16
	Records      []SequenceLookupRecord
}

type GPosContextFmt1Payload struct {
	RuleSets [][]GPosSequenceRule
}

type GPosContextFmt2Payload struct {
	ClassDef ClassDefinitions
	RuleSets [][]GPosClassSequenceRule
}

type GPosContextFmt3Payload struct {
	InputCoverages []Coverage
	Records        []SequenceLookupRecord
}

type GPosChainedSequenceRule struct {
	Backtrack []GlyphIndex
	Input     []GlyphIndex
	Lookahead []GlyphIndex
	Records   []SequenceLookupRecord
}

type GPosChainedClassRule struct {
	Backtrack []uint16
	Input     []uint16
	Lookahead []uint16
	Records   []SequenceLookupRecord
}

type GPosChainingContextFmt1Payload struct {
	RuleSets [][]GPosChainedSequenceRule
}

type GPosChainingContextFmt2Payload struct {
	BacktrackClassDef ClassDefinitions
	InputClassDef     ClassDefinitions
	LookaheadClassDef ClassDefinitions
	RuleSets          [][]GPosChainedClassRule
}

type GPosChainingContextFmt3Payload struct {
	BacktrackCoverages []Coverage
	InputCoverages     []Coverage
	LookaheadCoverages []Coverage
	Records            []SequenceLookupRecord
}

type GPosExtensionFmt1Payload struct {
	ResolvedType LayoutTableLookupType
	Resolved     *LookupNode
}
