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

type gposEntryExitOffsets struct {
	entry uint16
	exit  uint16
}

type GPosCursiveFmt1Payload struct {
	Entries          []GPosEntryExitAnchor
	entryExitOffsets []gposEntryExitOffsets
}

// EntryExitOffsets returns raw subtable-relative offsets for one cursive
// entry/exit record. These offsets are used for unresolved attachment metadata.
func (p *GPosCursiveFmt1Payload) EntryExitOffsets(i int) (entry uint16, exit uint16, ok bool) {
	if p == nil || i < 0 || i >= len(p.entryExitOffsets) {
		return 0, 0, false
	}
	off := p.entryExitOffsets[i]
	return off.entry, off.exit, true
}

type GPosMarkAttachRecord struct {
	Class        uint16
	Anchor       *Anchor
	anchorOffset uint16
}

type GPosBaseAttachRecord struct {
	Anchors       []*Anchor
	anchorOffsets []uint16
}

type GPosMarkToBaseFmt1Payload struct {
	BaseCoverage   Coverage
	MarkClassCount uint16
	MarkRecords    []GPosMarkAttachRecord
	BaseRecords    []GPosBaseAttachRecord
}

type GPosLigatureAttachRecord struct {
	ComponentAnchors       [][]*Anchor
	componentAnchorOffsets [][]uint16
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

// AnchorOffsets returns unresolved mark/base anchor offsets for one mark-to-base
// match tuple (mark record index, base record index, mark class).
func (p *GPosMarkToBaseFmt1Payload) AnchorOffsets(markInx, baseInx, class int) (markAnchor uint16, baseAnchor uint16, ok bool) {
	if p == nil || markInx < 0 || markInx >= len(p.MarkRecords) || baseInx < 0 || baseInx >= len(p.BaseRecords) {
		return 0, 0, false
	}
	if class < 0 || class >= int(p.MarkClassCount) || class >= len(p.BaseRecords[baseInx].anchorOffsets) {
		return 0, 0, false
	}
	return p.MarkRecords[markInx].anchorOffset, p.BaseRecords[baseInx].anchorOffsets[class], true
}

// AnchorOffsets returns unresolved mark/base anchor offsets for one
// mark-to-ligature match tuple (mark record index, ligature record index,
// component index, mark class).
func (p *GPosMarkToLigatureFmt1Payload) AnchorOffsets(markInx, ligInx, comp, class int) (markAnchor uint16, baseAnchor uint16, ok bool) {
	if p == nil || markInx < 0 || markInx >= len(p.MarkRecords) || ligInx < 0 || ligInx >= len(p.LigatureRecords) {
		return 0, 0, false
	}
	if class < 0 || class >= int(p.MarkClassCount) {
		return 0, 0, false
	}
	lig := p.LigatureRecords[ligInx]
	if comp < 0 || comp >= len(lig.componentAnchorOffsets) || class >= len(lig.componentAnchorOffsets[comp]) {
		return 0, 0, false
	}
	return p.MarkRecords[markInx].anchorOffset, lig.componentAnchorOffsets[comp][class], true
}

// AnchorOffsets returns unresolved mark/base anchor offsets for one mark-to-mark
// match tuple (mark1 record index, mark2 record index, mark class).
func (p *GPosMarkToMarkFmt1Payload) AnchorOffsets(mark1Inx, mark2Inx, class int) (markAnchor uint16, baseAnchor uint16, ok bool) {
	if p == nil || mark1Inx < 0 || mark1Inx >= len(p.Mark1Records) || mark2Inx < 0 || mark2Inx >= len(p.Mark2Records) {
		return 0, 0, false
	}
	if class < 0 || class >= int(p.MarkClassCount) || class >= len(p.Mark2Records[mark2Inx].anchorOffsets) {
		return 0, 0, false
	}
	return p.Mark1Records[mark1Inx].anchorOffset, p.Mark2Records[mark2Inx].anchorOffsets[class], true
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
