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
	RuleSets []ExpectedSubRuleSet
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
