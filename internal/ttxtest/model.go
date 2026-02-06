package ttxtest

// ExpectedGSUB is a normalized model of a GSUB table as derived from TTX.
// It intentionally only covers the subset of fields needed for tests.
type ExpectedGSUB struct {
	Lookups []ExpectedLookup
}

// ExpectedLookup represents a GSUB lookup with its subtables.
type ExpectedLookup struct {
	Index     int
	Type      int
	Flag      uint16
	Subtables []ExpectedSubtable
}

// ExpectedSubtable holds type-specific GSUB subtable expectations.
// For now, only GSUB-3 format 1 is supported.
type ExpectedSubtable struct {
	Type   int
	Format int

	// Coverage and alternates are keyed by glyph name as present in TTX.
	Coverage   []string
	Alternates map[string][]string

	// SingleSubst maps input glyph name to output glyph name.
	SingleSubst map[string]string

	// Ligatures maps first-component glyph name to ligatures.
	Ligatures map[string][]ExpectedLigature

	// ContextSubst holds GSUB-5 format 1 expectations.
	ContextSubst *ExpectedContextSubst
}

// ExpectedLigature describes a GSUB-4 ligature definition.
type ExpectedLigature struct {
	Components []string
	Glyph      string
}

// ExpectedContextSubst describes GSUB-5 format 1 (simple glyph contexts).
type ExpectedContextSubst struct {
	RuleSets      []ExpectedSubRuleSet
	ClassRuleSets []ExpectedClassRuleSet
	ClassDefs     map[string]int
}

// ExpectedSubRuleSet holds the sequence rules for a coverage entry.
type ExpectedSubRuleSet struct {
	Rules []ExpectedSequenceRule
}

// ExpectedSequenceRule defines remaining input glyphs and lookup records.
type ExpectedSequenceRule struct {
	Input         []string
	LookupRecords []ExpectedSequenceLookupRecord
}

// ExpectedSequenceLookupRecord mirrors SequenceLookupRecord in GSUB/GPOS.
type ExpectedSequenceLookupRecord struct {
	SequenceIndex   int
	LookupListIndex int
}

// ExpectedClassRuleSet holds class-based rules for format 2.
type ExpectedClassRuleSet struct {
	Rules []ExpectedClassSequenceRule
}

// ExpectedClassSequenceRule defines remaining input classes and lookup records.
type ExpectedClassSequenceRule struct {
	Classes       []int
	LookupRecords []ExpectedSequenceLookupRecord
}

// ExpectedGPOS is a normalized model of a GPOS table as derived from TTX.
// It intentionally only covers the subset of fields needed for tests.
type ExpectedGPOS struct {
	Lookups []ExpectedGPosLookup
}

// ExpectedGPosLookup represents a GPOS lookup with its subtables.
type ExpectedGPosLookup struct {
	Index     int
	Type      int
	Flag      uint16
	Subtables []ExpectedGPosSubtable
}

// ExpectedGPosSubtable holds type-specific GPOS subtable expectations.
type ExpectedGPosSubtable struct {
	Type   int
	Format int

	Coverage []string

	// MarkBasePos (type 4)
	MarkCoverage   []string
	BaseCoverage   []string
	MarkClassCount int
	MarkAnchors    []ExpectedMarkRecord
	BaseAnchors    [][]ExpectedAnchor
	// MarkLigPos (type 5)
	LigatureCoverage []string
	LigatureAnchors  [][][]ExpectedAnchor

	// SinglePos (type 1)
	ValueFormat uint16
	Value       ExpectedValueRecord
	Values      []ExpectedValueRecord

	// PairPos (type 2, format 1)
	ValueFormat1 uint16
	ValueFormat2 uint16
	PairValues   map[string][]ExpectedPairValueRecord

	// ChainContextPos (type 8, format 3)
	BacktrackCoverage [][]string
	InputCoverage     [][]string
	LookAheadCoverage [][]string
	PosLookupRecords  []ExpectedSequenceLookupRecord
}

// ExpectedValueRecord mirrors the GPOS ValueRecord fields.
type ExpectedValueRecord struct {
	XPlacement int
	YPlacement int
	XAdvance   int
	YAdvance   int
	XPlaDevice int
	YPlaDevice int
	XAdvDevice int
	YAdvDevice int

	HasXPlacement bool
	HasYPlacement bool
	HasXAdvance   bool
	HasYAdvance   bool
	HasXPlaDevice bool
	HasYPlaDevice bool
	HasXAdvDevice bool
	HasYAdvDevice bool
}

// ExpectedAnchor mirrors an Anchor table.
type ExpectedAnchor struct {
	Format      int
	XCoordinate int
	YCoordinate int
	AnchorPoint int
	XDevice     int
	YDevice     int
	HasPoint    bool
	HasXDevice  bool
	HasYDevice  bool
}

// ExpectedMarkRecord represents a MarkRecord entry.
type ExpectedMarkRecord struct {
	Class  int
	Anchor ExpectedAnchor
}

// ExpectedPairValueRecord represents a pair adjustment entry in PairPos.
type ExpectedPairValueRecord struct {
	SecondGlyph string
	Value1      ExpectedValueRecord
	Value2      ExpectedValueRecord
}
