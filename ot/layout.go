package ot

/*
From https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2:

OpenType Layout consists of five tables: the Glyph Substitution table (GSUB),
the Glyph Positioning table (GPOS), the Baseline table (BASE),
the Justification table (JSTF), and the Glyph Definition table (GDEF).
These tables use some of the same data formats.
*/

import (
	"fmt"
	"iter"
)

// --- Layout tables ---------------------------------------------------------

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

// Header returns the layout table header for this GSUB table.
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

// --- Layout tables sections ------------------------------------------------

// layoutTableSectionName lists the sections of OT layout tables, i.e. GPOS and GSUB.
type layoutTableSectionName int

const (
	layoutScriptSection layoutTableSectionName = iota
	layoutFeatureSection
	layoutLookupSection
	layoutFeatureVariationsSection
)

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

// Layout table script record
type scriptRecord struct {
	Tag    Tag
	Offset uint16
}

// Layout table feature record
type featureRecord struct {
	Tag    Tag
	Offset uint16
}

// --- GDEF table ------------------------------------------------------------

// GDefTable, the Glyph Definition (GDEF) table, provides various glyph properties
// used in OpenType Layout processing.
//
// See also
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#class-definition-table
type GDefTable struct {
	tableBase
	header                 GDefHeader
	GlyphClassDef          ClassDefinitions
	AttachmentPointList    AttachmentPointList
	MarkAttachmentClassDef ClassDefinitions
	MarkGlyphSets          []GlyphRange
}

func newGDefTable(tag Tag, b binarySegm, offset, size uint32) *GDefTable {
	t := &GDefTable{}
	base := tableBase{
		data:   b,
		name:   tag,
		offset: offset,
		length: size,
	}
	t.tableBase = base
	t.self = t
	return t
}

// Header returns the Glyph Definition header for t.
func (t *GDefTable) Header() GDefHeader {
	return t.header
}

// GDefHeader contains general information for a Glyph Definition table (GDEF).
type GDefHeader struct {
	gDefHeader
}

// Version returns major and minor version numbers for this GDef table.
func (h GDefHeader) Version() (int, int) {
	return int(h.Major), int(h.Minor)
}

// gDefHeader starts with a version number. Three versions are defined:
// 1.0, 1.2 and 1.3.
type gDefHeader struct {
	gDefHeaderV1_0
	MarkGlyphSetsDefOffset uint16
	ItemVarStoreOffset     uint32
	headerSize             uint8 // header size in bytes
}

type gDefHeaderV1_0 struct {
	versionHeader
	GlyphClassDefOffset      uint16
	AttachListOffset         uint16
	LigCaretListOffset       uint16
	MarkAttachClassDefOffset uint16
}

// GDefTableSectionName lists the sections of a GDEF table.
//type GDefTableSectionName int

// GDefGlyphClassDefSection GDefTableSectionName = iota
// GDefAttachListSection
// GDefLigCaretListSection
// GDefMarkAttachClassSection
// GDefMarkGlyphSetsDefSection
// GDefItemVarStoreSection

// Sections of a GDEF table.
const (
	GDefGlyphClassDefSection    = "GlyphClassDef"
	GDefAttachListSection       = "AttachList"
	GDefLigCaretListSection     = "LigCaretList"
	GDefMarkAttachClassSection  = "MarkAttachClassDef"
	GDefMarkGlyphSetsDefSection = "MarkGlyphSetsDef"
	GDefItemVarStoreSection     = "ItemVarStore"
)

// offsetFor returns an offset for a table section within the GDEF table.
// A GDEF table contains six sections:
// ▪︎ glyph class definitions,
// ▪︎ attachment list definitions,
// ▪︎ ligature carets lists,
// ▪︎ mark attachment class definitions,
// ▪︎ mark glyph sets definitions,
// ▪︎ item variant section.
// (see https://docs.microsoft.com/en-us/typography/opentype/spec/gdef#gdef-header)
func (h GDefHeader) offsetFor(which string) int {
	switch which {
	case GDefGlyphClassDefSection: // Candidate for a RangeTable
		return int(h.GlyphClassDefOffset)
	case GDefAttachListSection:
		return int(h.AttachListOffset)
	case GDefLigCaretListSection:
		return int(h.LigCaretListOffset)
	case GDefMarkAttachClassSection: // Candidate for a RangeTable
		return int(h.MarkAttachClassDefOffset)
	case GDefMarkGlyphSetsDefSection:
		return int(h.MarkGlyphSetsDefOffset)
	case GDefItemVarStoreSection:
		return int(h.ItemVarStoreOffset)
	}
	tracer().Errorf("illegal section offset type into GDEF table: %d", which)
	return 0 // illegal call, nothing sensible to return
}

// --- BASE table ------------------------------------------------------------

// BaseTable, the Baseline table (BASE), provides information used to align glyphs
// of different scripts and sizes in a line of text, whether the glyphs are in the
// same font or in different fonts.
// BaseTable, the Glyph Definition (BSE) table, provides various glyph properties
// used in OpenType Layout processing.
//
// See also
// https://docs.microsoft.com/en-us/typography/opentype/spec/base
type BaseTable struct {
	tableBase
	versionHeader
	horizontal         *BaseAxis
	vertical           *BaseAxis
	itemVarStoreOffset uint32
	err                error
}

func newBaseTable(tag Tag, b binarySegm, offset, size uint32) *BaseTable {
	t := &BaseTable{}
	base := tableBase{
		data:   b,
		name:   tag,
		offset: offset,
		length: size,
	}
	t.tableBase = base
	t.self = t
	return t
}

// BaseAxis is a projected view over one BASE axis table (horizontal or vertical).
// It keeps raw table bytes and record-list projections without copying records.
type BaseAxis struct {
	raw          binarySegm
	baselineTags baseTagListView
	scripts      baseTagOffset16View
	err          error
}

// BaseScript is a projected view over one BaseScript table.
type BaseScript struct {
	raw              binarySegm
	baseValuesOffset uint16
	defaultMinMax    uint16
	langSysMinMax    baseTagOffset16View
	err              error
}

// BaseValues is a projected view over a BaseValues table.
type BaseValues struct {
	raw                  binarySegm
	defaultBaselineIndex uint16
	coords               offset16View
	err                  error
}

// MinMax is a projected view over a MinMax table.
type MinMax struct {
	raw             binarySegm
	minCoordOffset  uint16
	maxCoordOffset  uint16
	featureMinMaxes baseTagOffset16View
	err             error
}

// FeatureMinMax is a projected view over a FeatureTableMinMaxValue table.
type FeatureMinMax struct {
	raw            binarySegm
	minCoordOffset uint16
	maxCoordOffset uint16
	err            error
}

// BaseCoord is a projected view over a BaseCoord table.
type BaseCoord struct {
	raw    binarySegm
	format uint16
	err    error
}

// Version returns major and minor BASE table version numbers.
func (b *BaseTable) Version() (uint16, uint16) {
	if b == nil {
		return 0, 0
	}
	return b.Major, b.Minor
}

// Horizontal returns the horizontal BASE axis table, if present.
func (b *BaseTable) Horizontal() *BaseAxis {
	if b == nil {
		return nil
	}
	return b.horizontal
}

// Vertical returns the vertical BASE axis table, if present.
func (b *BaseTable) Vertical() *BaseAxis {
	if b == nil {
		return nil
	}
	return b.vertical
}

// ItemVarStoreOffset returns the optional offset to ItemVariationStore (BASE v1.1+).
func (b *BaseTable) ItemVarStoreOffset() uint32 {
	if b == nil {
		return 0
	}
	return b.itemVarStoreOffset
}

// Error returns parser/validation errors attached to this BASE table view.
func (b *BaseTable) Error() error {
	if b == nil {
		return nil
	}
	return b.err
}

// BaselineTags returns the baseline tags in declaration order.
func (a *BaseAxis) BaselineTags() []Tag {
	if a == nil {
		return nil
	}
	return a.baselineTags.All()
}

// ScriptCount returns the number of script records in this axis.
func (a *BaseAxis) ScriptCount() int {
	if a == nil {
		return 0
	}
	return a.scripts.count
}

// Script returns a BaseScript by tag.
func (a *BaseAxis) Script(tag Tag) (*BaseScript, bool) {
	if a == nil {
		return nil, false
	}
	off, ok := a.scripts.LookupOffset(tag)
	if !ok || off == 0 {
		return nil, false
	}
	return viewBaseScript(a.raw, off)
}

// ScriptAt returns script record i as (tag, script, ok).
func (a *BaseAxis) ScriptAt(i int) (Tag, *BaseScript, bool) {
	if a == nil {
		return 0, nil, false
	}
	tag, off, ok := a.scripts.Record(i)
	if !ok || off == 0 {
		return 0, nil, false
	}
	s, ok := viewBaseScript(a.raw, off)
	return tag, s, ok
}

// RangeScripts iterates all script records in declaration order.
func (a *BaseAxis) RangeScripts() iter.Seq2[Tag, *BaseScript] {
	return func(yield func(Tag, *BaseScript) bool) {
		if a == nil {
			return
		}
		for i := 0; i < a.scripts.count; i++ {
			tag, script, ok := a.ScriptAt(i)
			if !ok {
				continue
			}
			if !yield(tag, script) {
				return
			}
		}
	}
}

// Error returns parser/validation errors attached to this axis view.
func (a *BaseAxis) Error() error {
	if a == nil {
		return nil
	}
	return a.err
}

// BaseValues returns the script-default BaseValues, if present.
func (s *BaseScript) BaseValues() (*BaseValues, bool) {
	if s == nil || s.baseValuesOffset == 0 {
		return nil, false
	}
	if int(s.baseValuesOffset) >= len(s.raw) {
		return nil, false
	}
	return viewBaseValues(s.raw[s.baseValuesOffset:])
}

// DefaultMinMax returns the default MinMax for this script, if present.
func (s *BaseScript) DefaultMinMax() (*MinMax, bool) {
	if s == nil || s.defaultMinMax == 0 {
		return nil, false
	}
	if int(s.defaultMinMax) >= len(s.raw) {
		return nil, false
	}
	return viewMinMax(s.raw[s.defaultMinMax:])
}

// LangSysMinMax returns language-specific MinMax by language-system tag.
func (s *BaseScript) LangSysMinMax(tag Tag) (*MinMax, bool) {
	if s == nil {
		return nil, false
	}
	off, ok := s.langSysMinMax.LookupOffset(tag)
	if !ok || off == 0 || int(off) >= len(s.raw) {
		return nil, false
	}
	return viewMinMax(s.raw[off:])
}

// LangSysCount returns the number of language-system MinMax records.
func (s *BaseScript) LangSysCount() int {
	if s == nil {
		return 0
	}
	return s.langSysMinMax.count
}

// LangSysMinMaxAt returns record i as (langTag, minmax, ok).
func (s *BaseScript) LangSysMinMaxAt(i int) (Tag, *MinMax, bool) {
	if s == nil {
		return 0, nil, false
	}
	tag, off, ok := s.langSysMinMax.Record(i)
	if !ok || off == 0 || int(off) >= len(s.raw) {
		return 0, nil, false
	}
	mm, ok := viewMinMax(s.raw[off:])
	return tag, mm, ok
}

// RangeLangSysMinMax iterates language-system specific MinMax records.
func (s *BaseScript) RangeLangSysMinMax() iter.Seq2[Tag, *MinMax] {
	return func(yield func(Tag, *MinMax) bool) {
		if s == nil {
			return
		}
		for i := 0; i < s.langSysMinMax.count; i++ {
			tag, mm, ok := s.LangSysMinMaxAt(i)
			if !ok {
				continue
			}
			if !yield(tag, mm) {
				return
			}
		}
	}
}

// Error returns parser/validation errors attached to this script view.
func (s *BaseScript) Error() error {
	if s == nil {
		return nil
	}
	return s.err
}

// DefaultBaselineIndex returns the default baseline index.
func (v *BaseValues) DefaultBaselineIndex() uint16 {
	if v == nil {
		return 0
	}
	return v.defaultBaselineIndex
}

// Len returns the number of BaseCoord offsets in this BaseValues table.
func (v *BaseValues) Len() int {
	if v == nil {
		return 0
	}
	return v.coords.count
}

// CoordAt returns BaseCoord #i from this BaseValues table.
func (v *BaseValues) CoordAt(i int) (*BaseCoord, bool) {
	if v == nil {
		return nil, false
	}
	off, ok := v.coords.At(i)
	if !ok || off == 0 || int(off) >= len(v.raw) {
		return nil, false
	}
	return viewBaseCoord(v.raw[off:])
}

// Error returns parser/validation errors attached to this BaseValues view.
func (v *BaseValues) Error() error {
	if v == nil {
		return nil
	}
	return v.err
}

// Min returns the minimum coordinate, if present.
func (m *MinMax) Min() (*BaseCoord, bool) {
	if m == nil || m.minCoordOffset == 0 || int(m.minCoordOffset) >= len(m.raw) {
		return nil, false
	}
	return viewBaseCoord(m.raw[m.minCoordOffset:])
}

// Max returns the maximum coordinate, if present.
func (m *MinMax) Max() (*BaseCoord, bool) {
	if m == nil || m.maxCoordOffset == 0 || int(m.maxCoordOffset) >= len(m.raw) {
		return nil, false
	}
	return viewBaseCoord(m.raw[m.maxCoordOffset:])
}

// Feature returns a feature-specific min/max override by feature tag.
func (m *MinMax) Feature(tag Tag) (*FeatureMinMax, bool) {
	if m == nil {
		return nil, false
	}
	off, ok := m.featureMinMaxes.LookupOffset(tag)
	if !ok || off == 0 || int(off) >= len(m.raw) {
		return nil, false
	}
	return viewFeatureMinMax(m.raw[off:])
}

// FeatureCount returns the number of feature-specific min/max records.
func (m *MinMax) FeatureCount() int {
	if m == nil {
		return 0
	}
	return m.featureMinMaxes.count
}

// FeatureAt returns feature record i as (featureTag, minmax, ok).
func (m *MinMax) FeatureAt(i int) (Tag, *FeatureMinMax, bool) {
	if m == nil {
		return 0, nil, false
	}
	tag, off, ok := m.featureMinMaxes.Record(i)
	if !ok || off == 0 || int(off) >= len(m.raw) {
		return 0, nil, false
	}
	fmm, ok := viewFeatureMinMax(m.raw[off:])
	return tag, fmm, ok
}

// RangeFeatures iterates feature-specific MinMax records.
func (m *MinMax) RangeFeatures() iter.Seq2[Tag, *FeatureMinMax] {
	return func(yield func(Tag, *FeatureMinMax) bool) {
		if m == nil {
			return
		}
		for i := 0; i < m.featureMinMaxes.count; i++ {
			tag, fmm, ok := m.FeatureAt(i)
			if !ok {
				continue
			}
			if !yield(tag, fmm) {
				return
			}
		}
	}
}

// Error returns parser/validation errors attached to this MinMax view.
func (m *MinMax) Error() error {
	if m == nil {
		return nil
	}
	return m.err
}

// Min returns the minimum coordinate override, if present.
func (f *FeatureMinMax) Min() (*BaseCoord, bool) {
	if f == nil || f.minCoordOffset == 0 || int(f.minCoordOffset) >= len(f.raw) {
		return nil, false
	}
	return viewBaseCoord(f.raw[f.minCoordOffset:])
}

// Max returns the maximum coordinate override, if present.
func (f *FeatureMinMax) Max() (*BaseCoord, bool) {
	if f == nil || f.maxCoordOffset == 0 || int(f.maxCoordOffset) >= len(f.raw) {
		return nil, false
	}
	return viewBaseCoord(f.raw[f.maxCoordOffset:])
}

// Error returns parser/validation errors attached to this feature override.
func (f *FeatureMinMax) Error() error {
	if f == nil {
		return nil
	}
	return f.err
}

// Format returns BaseCoord format number (1, 2, or 3).
func (c *BaseCoord) Format() uint16 {
	if c == nil {
		return 0
	}
	return c.format
}

// Coordinate returns the design-units baseline coordinate.
func (c *BaseCoord) Coordinate() int16 {
	if c == nil || len(c.raw) < 4 {
		return 0
	}
	return int16(c.raw.U16(2))
}

// ReferenceGlyph returns the reference glyph for BaseCoord format 2.
func (c *BaseCoord) ReferenceGlyph() (GlyphIndex, bool) {
	if c == nil || c.format != 2 || len(c.raw) < 8 {
		return 0, false
	}
	return GlyphIndex(c.raw.U16(4)), true
}

// BaseCoordPoint returns the contour point index for BaseCoord format 2.
func (c *BaseCoord) BaseCoordPoint() (uint16, bool) {
	if c == nil || c.format != 2 || len(c.raw) < 8 {
		return 0, false
	}
	return c.raw.U16(6), true
}

// DeviceOrVarIdxOffset returns device/variation offset for BaseCoord format 3.
func (c *BaseCoord) DeviceOrVarIdxOffset() (uint16, bool) {
	if c == nil || c.format != 3 || len(c.raw) < 6 {
		return 0, false
	}
	return c.raw.U16(4), true
}

// Error returns parser/validation errors attached to this BaseCoord view.
func (c *BaseCoord) Error() error {
	if c == nil {
		return nil
	}
	return c.err
}

type baseTagListView struct {
	raw   binarySegm
	count int
}

func parseBaseTagListView(raw binarySegm, offset int) (baseTagListView, error) {
	if offset < 0 || offset+2 > len(raw) {
		return baseTagListView{}, fmt.Errorf("BASE BaseTagList offset out of bounds")
	}
	count := int(raw.U16(offset))
	size := count * 4
	if size < 0 || offset+2+size > len(raw) {
		return baseTagListView{}, fmt.Errorf("BASE BaseTagList records out of bounds")
	}
	return baseTagListView{raw: raw[offset+2 : offset+2+size], count: count}, nil
}

func (v baseTagListView) All() []Tag {
	if v.count == 0 {
		return nil
	}
	tags := make([]Tag, v.count)
	for i := 0; i < v.count; i++ {
		start := i * 4
		tags[i] = Tag(u32(v.raw[start : start+4]))
	}
	return tags
}

type baseTagOffset16View struct {
	raw   binarySegm
	count int
}

func parseTagOffset16View(raw binarySegm, countOffset int) (baseTagOffset16View, error) {
	if countOffset < 0 || countOffset+2 > len(raw) {
		return baseTagOffset16View{}, fmt.Errorf("BASE tag-offset view count out of bounds")
	}
	count := int(raw.U16(countOffset))
	size := count * 6
	start := countOffset + 2
	if size < 0 || start+size > len(raw) {
		return baseTagOffset16View{}, fmt.Errorf("BASE tag-offset records out of bounds")
	}
	return baseTagOffset16View{raw: raw[start : start+size], count: count}, nil
}

func (v baseTagOffset16View) Record(i int) (Tag, uint16, bool) {
	if i < 0 || i >= v.count {
		return 0, 0, false
	}
	start := i * 6
	tag := Tag(u32(v.raw[start : start+4]))
	off := u16(v.raw[start+4 : start+6])
	return tag, off, true
}

func (v baseTagOffset16View) LookupOffset(tag Tag) (uint16, bool) {
	lo, hi := 0, v.count-1
	for lo <= hi {
		mid := (lo + hi) / 2
		rtag, off, _ := v.Record(mid)
		switch {
		case rtag == tag:
			return off, true
		case rtag < tag:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}
	return 0, false
}

type offset16View struct {
	raw   binarySegm
	count int
}

func parseOffset16View(raw binarySegm, countOffset int) (offset16View, error) {
	if countOffset < 0 || countOffset+2 > len(raw) {
		return offset16View{}, fmt.Errorf("BASE offset view count out of bounds")
	}
	count := int(raw.U16(countOffset))
	size := count * 2
	start := countOffset + 2
	if size < 0 || start+size > len(raw) {
		return offset16View{}, fmt.Errorf("BASE offset records out of bounds")
	}
	return offset16View{raw: raw[start : start+size], count: count}, nil
}

func (v offset16View) At(i int) (uint16, bool) {
	if i < 0 || i >= v.count {
		return 0, false
	}
	start := i * 2
	return u16(v.raw[start : start+2]), true
}

func viewBaseAxis(base binarySegm, offset uint16) (*BaseAxis, bool) {
	if offset == 0 || int(offset) >= len(base) {
		return nil, false
	}
	raw := base[offset:]
	if len(raw) < 4 {
		return &BaseAxis{raw: raw, err: fmt.Errorf("BASE axis table too small")}, false
	}
	tagListOffset := int(raw.U16(0))
	scriptListOffset := int(raw.U16(2))
	axis := &BaseAxis{raw: raw}
	var err error
	if tagListOffset > 0 {
		axis.baselineTags, err = parseBaseTagListView(raw, tagListOffset)
		if err != nil {
			axis.err = err
		}
	}
	axis.scripts, err = parseTagOffset16View(raw, scriptListOffset)
	if err != nil {
		if axis.err == nil {
			axis.err = err
		}
		return axis, false
	}
	return axis, true
}

func viewBaseScript(axisRaw binarySegm, offset uint16) (*BaseScript, bool) {
	if offset == 0 || int(offset) >= len(axisRaw) {
		return nil, false
	}
	raw := axisRaw[offset:]
	if len(raw) < 6 {
		return &BaseScript{raw: raw, err: fmt.Errorf("BASE BaseScript table too small")}, false
	}
	langs, err := parseTagOffset16View(raw, 4)
	s := &BaseScript{
		raw:              raw,
		baseValuesOffset: raw.U16(0),
		defaultMinMax:    raw.U16(2),
		langSysMinMax:    langs,
		err:              err,
	}
	return s, err == nil
}

func viewBaseValues(raw binarySegm) (*BaseValues, bool) {
	if len(raw) < 4 {
		return &BaseValues{raw: raw, err: fmt.Errorf("BASE BaseValues table too small")}, false
	}
	coords, err := parseOffset16View(raw, 2)
	v := &BaseValues{
		raw:                  raw,
		defaultBaselineIndex: raw.U16(0),
		coords:               coords,
		err:                  err,
	}
	return v, err == nil
}

func viewMinMax(raw binarySegm) (*MinMax, bool) {
	if len(raw) < 6 {
		return &MinMax{raw: raw, err: fmt.Errorf("BASE MinMax table too small")}, false
	}
	features, err := parseTagOffset16View(raw, 4)
	m := &MinMax{
		raw:             raw,
		minCoordOffset:  raw.U16(0),
		maxCoordOffset:  raw.U16(2),
		featureMinMaxes: features,
		err:             err,
	}
	return m, err == nil
}

func viewFeatureMinMax(raw binarySegm) (*FeatureMinMax, bool) {
	if len(raw) < 4 {
		return &FeatureMinMax{raw: raw, err: fmt.Errorf("BASE FeatureMinMax table too small")}, false
	}
	f := &FeatureMinMax{
		raw:            raw,
		minCoordOffset: raw.U16(0),
		maxCoordOffset: raw.U16(2),
	}
	return f, true
}

func viewBaseCoord(raw binarySegm) (*BaseCoord, bool) {
	if len(raw) < 4 {
		return &BaseCoord{raw: raw, err: fmt.Errorf("BASE BaseCoord table too small")}, false
	}
	format := raw.U16(0)
	c := &BaseCoord{raw: raw, format: format}
	switch format {
	case 1:
		return c, true
	case 2:
		if len(raw) < 8 {
			c.err = fmt.Errorf("BASE BaseCoord format-2 table too small")
			return c, false
		}
		return c, true
	case 3:
		if len(raw) < 6 {
			c.err = fmt.Errorf("BASE BaseCoord format-3 table too small")
			return c, false
		}
		return c, true
	default:
		c.err = fmt.Errorf("unsupported BASE BaseCoord format %d", format)
		return c, false
	}
}

// --- Coverage table module -------------------------------------------------

// Covarage denotes an indexed set of glyphs.
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

// --- Class definition tables -----------------------------------------------

// GlyphClassDefEnum lists the glyph classes for ClassDefinitions
// ('GlyphClassDef'-table).
type GlyphClassDefEnum uint16

const (
	BaseGlyph      GlyphClassDefEnum = iota //single character, spacing glyph
	LigatureGlyph                           //multiple character, spacing glyph
	MarkGlyph                               //non-spacing combining glyph
	ComponentGlyph                          //part of single character, spacing glyph
)

// ClassDefinitions groups glyphs into classes, denoted as integer values.
//
// From the spec:
// For efficiency and ease of representation, a font developer can group glyph indices
// to form glyph classes. Class assignments vary in meaning from one lookup subtable
// to another. For example, in the GSUB and GPOS tables, classes are used to describe
// glyph contexts. GDEF tables also use the idea of glyph classes.
// (see https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#class-definition-table)
type ClassDefinitions struct {
	format  uint16          // format version 1 or 2
	records classDefVariant // either format 1 or 2
}

func (cdef *ClassDefinitions) setRecords(recs array, startGlyphID GlyphIndex) {
	if cdef.format == 1 {
		cdef.records = &classDefinitionsFormat1{
			count:      recs.length,
			start:      startGlyphID,
			valueArray: recs,
		}
	} else if cdef.format == 2 {
		cdef.records = &classDefinitionsFormat2{
			count:       recs.length,
			classRanges: recs,
		}
	}
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
	clz := cdf.valueArray.Get(int(glyph - cdf.start)).U16(0)
	return int(clz)
}

type classDefinitionsFormat2 struct {
	count       int   // number of records
	classRanges array // array of ClassRangeRecords — ordered by startGlyphID
}

func (cdf *classDefinitionsFormat2) Lookup(glyph GlyphIndex) int {
	//trace().Debugf("lookup up glyph %d in class def format 2", glyph)
	for i := 0; i < cdf.count; i++ {
		rec := cdf.classRanges.Get(i)
		if glyph < GlyphIndex(rec.U16(0)) {
			return 0
		}
		if glyph < GlyphIndex(rec.U16(2)) {
			return int(rec.U16(4))
		}
	}
	return 0
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

// --- Attachment point list -------------------------------------------------

// An AttachmentPointList consists of a count of the attachment points on a single
// glyph (PointCount) and an array of contour indices of those points (PointIndex),
// listed in increasing numerical order.
type AttachmentPointList struct {
	Coverage           GlyphRange
	Count              int
	attachPointOffsets binarySegm
}

// --- Lookup type helpers ---------------------------------------------------

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
