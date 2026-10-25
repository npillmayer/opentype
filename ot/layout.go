package ot

//From https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2:
//
//OpenType Layout consists of five tables: the Glyph Substitution table (GSUB),
//the Glyph Positioning table (GPOS), the Baseline table (BASE),
//the Justification table (JSTF), and the Glyph Definition table (GDEF).
//These tables use some of the same data formats.
//

// --- Shared layout tables and headers --------------------------------------
//
// GSUB and GPOS share this table shell, header layout, and section addressing.
// Script/feature and lookup graph implementations live in their own files.
// versionHeader is also used by the separate BASE and GDEF representations.

// LayoutTable is a base type for layout tables.
// OpenType specifies two such tables–GPOS and GSUB–which share some of their
// structure.
type LayoutTable struct {
	scriptGraph  *ScriptList
	featureGraph *FeatureList
	lookupGraph  *LookupListGraph
	Requirements LayoutRequirements
	header       *LayoutHeader
}

// Header returns the shared layout header for this GSUB or GPOS table.
func (t *LayoutTable) Header() LayoutHeader {
	return *t.header
}

// ScriptGraph returns the concrete shared-script graph for this layout table.
// During transition this may be nil when not yet parsed/instantiated.
func (t *LayoutTable) ScriptGraph() *ScriptList {
	if t == nil {
		return nil
	}
	return t.scriptGraph
}

// FeatureGraph returns the concrete shared-feature graph for this layout table.
// During transition this may be nil when not yet parsed/instantiated.
func (t *LayoutTable) FeatureGraph() *FeatureList {
	if t == nil {
		return nil
	}
	return t.featureGraph
}

// LookupGraph returns the concrete lookup graph for this layout table.
// During transition this may be nil when not yet parsed/instantiated.
func (t *LayoutTable) LookupGraph() *LookupListGraph {
	if t == nil {
		return nil
	}
	return t.lookupGraph
}

// LayoutHeader represents header information common to the layout tables.
type LayoutHeader struct {
	versionHeader
	offsets layoutHeader11
}

// Version returns major and minor version numbers for this layout table.
func (h LayoutHeader) Version() (int, int) {
	return int(h.Major), int(h.Minor)
}

// offsetFor returns an offset for a layout table section within the layout table
// (GPOS or GSUB).
// A layout table contains four sections:
// ▪︎ Script Section,
// ▪︎ Feature Section,
// ▪︎ Lookup Section,
// ▪︎ Feature Variations Section.
// (see type LayoutTableSectionName)
func (h *LayoutHeader) offsetFor(which layoutTableSectionName) int {
	switch which {
	case layoutScriptSection:
		return int(h.offsets.ScriptListOffset)
	case layoutFeatureSection:
		return int(h.offsets.FeatureListOffset)
	case layoutLookupSection:
		return int(h.offsets.LookupListOffset)
	case layoutFeatureVariationsSection:
		return int(h.offsets.FeatureVariationsOffset)
	}
	tracer().Errorf("illegal section offset type into layout table: %d", which)
	return 0 // illegal call, nothing sensible to return
}

// versionHeader is the beginning of on-disk format of some format headers.
// See https://docs.microsoft.com/en-us/typography/opentype/spec/gdef#gdef-header
// See https://www.microsoft.com/typography/otspec/GPOS.htm
// See https://www.microsoft.com/typography/otspec/GSUB.htm
// Fields are public for reflection-access.
type versionHeader struct {
	Major uint16
	Minor uint16
}

// layoutHeader10 is the on-disk format of GPOS/GSUB version header when major=1 and minor=0.
// Fields are public for reflection-access.
type layoutHeader10 struct {
	ScriptListOffset  uint16 // offset to ScriptList table, from beginning of GPOS/GSUB table.
	FeatureListOffset uint16 // offset to FeatureList table, from beginning of GPOS/GSUB table.
	LookupListOffset  uint16 // offset to LookupList table, from beginning of GPOS/GSUB table.
}

// layoutHeader11 is the on-disk format of GPOS/GSUB version header when major=1 and minor=1.
// Fields are public for reflection-access.
type layoutHeader11 struct {
	layoutHeader10
	FeatureVariationsOffset uint32 // offset to FeatureVariations table, from beginning of GPOS/GSUB table (may be NULL).
}

// layoutTableSectionName lists the sections of OT layout tables, i.e. GPOS and GSUB.
type layoutTableSectionName int

const (
	layoutScriptSection layoutTableSectionName = iota
	layoutFeatureSection
	layoutLookupSection
	layoutFeatureVariationsSection
)

// --- Lookup vocabulary and requirements ------------------------------------
//
// Lookup flags retain their on-disk bit values. Lookup types use an internal
// family encoding: GSUB types occupy the low byte, while GPOS types occupy the
// high byte. The helpers below extract the numeric type or encode a GPOS type;
// this family encoding is not the on-disk LookupType representation.
// SequenceLookupRecord describes a nested application within matched input.
// LayoutRequirements aggregates the GDEF data needed to honor lookup flags.

// LayoutTableLookupFlag is a flag type for layout tables (GPOS and GSUB).
type LayoutTableLookupFlag uint16

// Lookup flags of layout tables (GPOS and GSUB)
const ( // LookupFlag bit enumeration
	// Note that the RIGHT_TO_LEFT flag is used only for GPOS type 3 lookups and is ignored
	// otherwise. It is not used by client software in determining text direction.
	LOOKUP_FLAG_RIGHT_TO_LEFT             LayoutTableLookupFlag = 0x0001
	LOOKUP_FLAG_IGNORE_BASE_GLYPHS        LayoutTableLookupFlag = 0x0002 // If set, skips over base glyphs
	LOOKUP_FLAG_IGNORE_LIGATURES          LayoutTableLookupFlag = 0x0004 // If set, skips over ligatures
	LOOKUP_FLAG_IGNORE_MARKS              LayoutTableLookupFlag = 0x0008 // If set, skips over all combining marks
	LOOKUP_FLAG_USE_MARK_FILTERING_SET    LayoutTableLookupFlag = 0x0010 // If set, indicates that the lookup table structure is followed by a MarkFilteringSet field.
	LOOKUP_FLAG_reserved                  LayoutTableLookupFlag = 0x00E0 // For future use (Set to zero)
	LOOKUP_FLAG_MARK_ATTACHMENT_TYPE_MASK LayoutTableLookupFlag = 0xFF00 // If not zero, skips over all marks of attachment type different from specified.
)

// LayoutTableLookupType is a type identifier for layout lookup records (GPOS and GSUB).
// Enum values are different for GPOS and GSUB.
type LayoutTableLookupType uint16

func GSubLookupType(ltype LayoutTableLookupType) LayoutTableLookupType {
	return ltype & 0x00ff
}

func GPosLookupType(ltype LayoutTableLookupType) LayoutTableLookupType {
	return (ltype & 0xff00) >> 8
}

func MaskGPosLookupType(ltype LayoutTableLookupType) LayoutTableLookupType {
	return ltype << 8
}

func IsGPosLookupType(ltype LayoutTableLookupType) bool {
	return ltype&0xff00 > 0
}

// SequenceLookupRecord identifies a nested lookup to apply at a position
// within a matched input sequence.
type SequenceLookupRecord struct {
	SequenceIndex   uint16
	LookupListIndex uint16
}

// LayoutRequirements collects GDEF subtable requirements implied by lookup flags.
// Requirements are aggregated during the parse of GSUB/GPOS lookup lists.
type LayoutRequirements struct {
	NeedGlyphClassDef      bool
	NeedMarkAttachClassDef bool
	NeedMarkGlyphSets      bool
}

// AddFromLookupFlag updates requirements based on a lookup's flag bits.
func (r *LayoutRequirements) AddFromLookupFlag(flag LayoutTableLookupFlag) {
	if flag&(LOOKUP_FLAG_IGNORE_BASE_GLYPHS|LOOKUP_FLAG_IGNORE_LIGATURES|LOOKUP_FLAG_IGNORE_MARKS) != 0 {
		r.NeedGlyphClassDef = true
	}
	if flag&LOOKUP_FLAG_USE_MARK_FILTERING_SET != 0 {
		r.NeedMarkGlyphSets = true
	}
	if flag&LOOKUP_FLAG_MARK_ATTACHMENT_TYPE_MASK != 0 {
		r.NeedMarkAttachClassDef = true
	}
}

// Merge combines requirements from another layout table.
func (r *LayoutRequirements) Merge(other LayoutRequirements) {
	r.NeedGlyphClassDef = r.NeedGlyphClassDef || other.NeedGlyphClassDef
	r.NeedMarkAttachClassDef = r.NeedMarkAttachClassDef || other.NeedMarkAttachClassDef
	r.NeedMarkGlyphSets = r.NeedMarkGlyphSets || other.NeedMarkGlyphSets
}

// --- Coverage tables -------------------------------------------------------
//
// Coverage maps a glyph ID to its index in a covered set. That index selects
// corresponding lookup data; it is not a class number. GSUB, GPOS, and GDEF
// share this representation. A non-match is distinct from coverage index zero.
// Construction delegates the format-specific storage to GlyphRange.

// Coverage denotes an indexed set of glyphs.
// Each LookupSubtable (except an Extension LookupType subtable) in a lookup references
// a Coverage table (Coverage), which specifies all the glyphs affected by a
// substitution or positioning operation described in the subtable.
// The GSUB, GPOS, and GDEF tables rely on this notion of coverage. If a glyph does
// not appear in a Coverage table, the client can skip that subtable and move
// immediately to the next subtable.
type Coverage struct {
	coverageHeader
	GlyphRange GlyphRange
}

// Match returns the Coverage Index for a glyph, and true if present.
func (c Coverage) Match(g GlyphIndex) (int, bool) {
	if c.GlyphRange == nil {
		return 0, false
	}
	return c.GlyphRange.Match(g)
}

// Contains reports whether a glyph is present in the coverage.
func (c Coverage) Contains(g GlyphIndex) bool {
	_, ok := c.Match(g)
	return ok
}

type coverageHeader struct {
	CoverageFormat uint16
	Count          uint16
}

func buildGlyphRangeFromCoverage(chead coverageHeader, b binarySegm) GlyphRange {
	tracer().Debugf("coverage format = %d, count = %d", chead.CoverageFormat, chead.Count)
	if chead.CoverageFormat == 1 {
		return &glyphRangeArray{
			//is32:     false,                  // entries are uint16
			count:    int(chead.Count),       // number of entries
			data:     b[4:],                  // header of format 1 coverage table is 4 bytes long
			byteSize: int(4 + chead.Count*2), // header is 4, entries are 2 bytes
		}
	}
	return &glyphRangeRecords{
		count:    int(chead.Count),       // number of records
		data:     b[4:],                  // header of format 2 coverage table is 4 bytes long
		byteSize: int(4 + chead.Count*6), // header is 4, entries are 6 bytes
	}
}

// --- Class-definition tables -----------------------------------------------
//
// Class definitions map glyph IDs to class numbers, not coverage indices.
// Glyphs without an explicit assignment belong to default class zero. The
// meaning of nonzero classes depends on the consuming GSUB, GPOS, or GDEF table.
// The format variants below share one lookup interface; their record readers
// also return class zero on read failure, without exposing an error result.

// ClassDefinitions groups glyphs into classes, denoted as integer values.
//
// From the spec:
// For efficiency and ease of representation, a font developer can group glyph indices
// to form glyph classes. Class assignments vary in meaning from one lookup subtable
// to another. For example, in the GSUB and GPOS tables, classes are used to describe
// glyph contexts. GDEF tables also use the idea of glyph classes.
// (see https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#class-definition-table)
type ClassDefinitions struct {
	format  uint16 // format version 1 or 2
	records classDefVariant
}

func (cdef *ClassDefinitions) setRecords(recs array, startGlyphID GlyphIndex) {
	switch cdef.format {
	case 1:
		cdef.records = &classDefinitionsFormat1{
			count:      recs.length,
			start:      startGlyphID,
			valueArray: recs,
		}
	case 2:
		cdef.records = &classDefinitionsFormat2{
			count:       recs.length,
			classRanges: recs,
		}
	default:
		tracer().Errorf("Unsupported ClassDef format %d", cdef.format)
		cdef.records = &classDefinitionsFormat2{count: 0}
	}
}

func (cdef *ClassDefinitions) makeArray(b binarySegm, numEntries int, format uint16) array {
	var size, recsize int
	switch format {
	case 1:
		recsize = 2
		size = 6 + numEntries*recsize
		b = b[6:size]
	case 2:
		recsize = 6
		size = 4 + numEntries*recsize
		b = b[4:size]
	default:
		tracer().Errorf("illegal format %d of class definition table", format)
		return array{}
	}
	return array{recordSize: recsize, length: numEntries, loc: b}
}

// Lookup returns the class defined for a glyph, or 0 (= default class).
func (cdef *ClassDefinitions) Lookup(glyph GlyphIndex) int {
	return cdef.records.Lookup(glyph)
}

// Class returns the class defined for a glyph, or 0 (= default class).
func (cdef *ClassDefinitions) Class(glyph GlyphIndex) int {
	return cdef.Lookup(glyph)
}

type classDefVariant interface {
	Lookup(GlyphIndex) int
}

type classDefinitionsFormat1 struct {
	count      int        // number of entries
	start      GlyphIndex // glyph ID of the first entry in a format-1 table
	valueArray array      // array of Class Values — one per glyph ID
}

func (cdf *classDefinitionsFormat1) Lookup(glyph GlyphIndex) int {
	if glyph < cdf.start || glyph >= cdf.start+GlyphIndex(cdf.count) {
		return 0
	}
	item, err := cdf.valueArray.Get(int(glyph - cdf.start))
	if err != nil {
		return 0 // Default class when the record cannot be read.
	}
	clz := item.U16(0)
	return int(clz)
}

type classDefinitionsFormat2 struct {
	count       int   // number of records
	classRanges array // array of ClassRangeRecords — ordered by startGlyphID
}

func (cdf *classDefinitionsFormat2) Lookup(glyph GlyphIndex) int {
	for i := 0; i < cdf.count; i++ {
		rec, err := cdf.classRanges.Get(i)
		if err != nil {
			return 0 // Do not interpret a failed read as a class-range record.
		}
		if glyph < GlyphIndex(rec.U16(0)) {
			return 0
		}
		if glyph < GlyphIndex(rec.U16(2)) {
			return int(rec.U16(4))
		}
	}
	return 0
}
