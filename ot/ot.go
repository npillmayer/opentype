package ot

import (
	"fmt"

	"github.com/npillmayer/opentype"
)

// Font represents the internal structure of an OpenType font.
// It is used to navigate properties of a font for typesetting tasks.
//
// We only support OpenType fonts with advanced layout, i.e. fonts containing tables
// GSUB, GPOS, etc.
type Font struct {
	F             *opentype.ScalableFont
	Header        *FontHeader
	tables        map[Tag]Table
	CMap          *CMapTable    // CMAP table is mandatory
	HHea          *HHeaTable    // typed access to hhea
	HMtx          *HMtxTable    // typed access to hmtx
	OS2           *OS2Table     // typed access to OS/2
	parseErrors   []FontError   // Errors accumulated during parsing
	parseWarnings []FontWarning // Warnings accumulated during parsing
	parseOptions  []ParseOption // Options to guide the parsing process
	Layout        struct {      // OpenType core layout tables
		GSub *GSubTable // OpenType layout GSUB
		GPos *GPosTable // OpenType layout GPOS
		GDef *GDefTable // OpenType layout GDEF
		Base *BaseTable // OpenType layout BASE
		// TODO JSTF
		Requirements LayoutRequirements
	}
}

// ParseOptions guides and influences the parsing of the font.
type ParseOption int

const (
	IsTestfont        ParseOption = iota // relaxes a number of cross-checks that are normally enforced
	relaxConsistency                     // relax conistency between tables (e.g, GSUB + GDEF)
	relaxCompleteness                    // aceept missing tables
)

// FontHeader is a directory of the top-level tables in a font. If the font file
// contains only one font, the table directory will begin at byte 0 of the file.
// If the font file is an OpenType Font Collection file (see below), the beginning
// point of the table directory for each font is indicated in the TTCHeader.
//
// OpenType fonts that contain TrueType outlines should use the value of 0x00010000
// for the FontType. OpenType fonts containing CFF data (version 1 or 2) should
// use 0x4F54544F ('OTTO', when re-interpreted as a Tag).
// The Apple specification for TrueType fonts allows for 'true' and 'typ1',
// but these version tags should not be used for OpenType fonts.
type FontHeader struct {
	FontType   uint32
	TableCount uint16
}

// Table returns the font table for a given tag. If a table for a tag cannot
// be found in the font, nil is returned.
//
// Please note that the current implementation will not interpret every kind of
// font table, either because there is no need to do so (with regard to
// text shaping or rasterization), or because implementation is not yet finished.
// However, `Table` will return at least a generic table type for each table contained in
// the font, i.e. no table information will be dropped.
//
// For example to receive the `OS/2` and the `loca` table, clients may call
//
//	os2  := otf.Table(ot.T("OS/2"))
//	loca := otf.Table(ot.T("loca")).Self().AsLoca()
//
// Table tag names are case-sensitive, following the names in the OpenType specification,
// i.e., one of:
//
// avar BASE CBDT CBLC CFF CFF2 cmap COLR CPAL cvar cvt DSIG EBDT EBLC EBSC fpgm fvar
// gasp GDEF glyf GPOS GSUB gvar hdmx head hhea hmtx HVAR JSTF kern loca LTSH MATH
// maxp MERG meta MVAR name OS/2 PCLT post prep sbix STAT SVG VDMX vhea vmtx VORG VVAR
func (otf *Font) Table(tag Tag) Table {
	if t, ok := otf.tables[tag]; ok {
		return t
	}
	return nil
}

// TableTags returns a list of tags, one for each table contained in the font.
func (otf *Font) TableTags() []Tag {
	var tags = make([]Tag, 0, len(otf.tables))
	for tag := range otf.tables {
		tags = append(tags, tag)
	}
	return tags
}

// HorizontalHeader returns the parsed hhea table, if present.
func (otf *Font) HorizontalHeader() *HHeaTable {
	if otf == nil {
		return nil
	}
	return otf.HHea
}

// HorizontalMetrics returns the parsed hmtx table, if present.
func (otf *Font) HorizontalMetrics() *HMtxTable {
	if otf == nil {
		return nil
	}
	return otf.HMtx
}

// OS2Metrics returns the parsed OS/2 table, if present.
func (otf *Font) OS2Metrics() *OS2Table {
	if otf == nil {
		return nil
	}
	return otf.OS2
}

// Errors returns all errors encountered during font parsing.
// These errors represent issues that were found but did not prevent parsing from completing.
// Clients can inspect these errors to determine if the font is suitable for their use case.
func (otf *Font) Errors() []FontError {
	if otf.parseErrors == nil {
		return []FontError{}
	}
	return otf.parseErrors
}

// Warnings returns all warnings encountered during font parsing.
// Warnings indicate potential issues that are generally safe to ignore.
func (otf *Font) Warnings() []FontWarning {
	if otf.parseWarnings == nil {
		return []FontWarning{}
	}
	return otf.parseWarnings
}

// CriticalErrors returns all errors with critical severity.
// Critical errors indicate severe problems that may make the font unreliable.
func (otf *Font) CriticalErrors() []FontError {
	critical := make([]FontError, 0)
	for _, err := range otf.parseErrors {
		if err.Severity == SeverityCritical {
			critical = append(critical, err)
		}
	}
	return critical
}

// HasCriticalErrors returns true if any critical errors were encountered during parsing.
// Fonts with critical errors may be unreliable or unusable.
func (otf *Font) HasCriticalErrors() bool {
	for _, err := range otf.parseErrors {
		if err.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// GlyphIndex is a glyph index in a font.
type GlyphIndex uint16

// --- Tag -------------------------------------------------------------------

// Tag is defined by the spec as:
// Array of four uint8s (length = 32 bits) used to identify a table, design-variation axis,
// script, language system, feature, or baseline
type Tag uint32

// MakeTag creates a Tag from 4 bytes, e.g.,
// If b is shorter or longer, it will be silently extended or cut as appropriate
//
//	MakeTag([]byte("cmap"))
func MakeTag(b []byte) Tag {
	if b == nil {
		b = []byte{0, 0, 0, 0}
	} else if len(b) > 4 {
		b = b[:4]
	} else if len(b) < 4 {
		b = append([]byte{0, 0, 0, 0}[:4-len(b)], b...)
	}
	return Tag(u32(b))
}

// T returns a Tag from a (4-letter) string.
// If t is shorter or longer, it will be silently extended or cut as appropriate
func T(t string) Tag {
	t = (t + "    ")[:4]
	return Tag(u32([]byte(t)))
}

func (t Tag) String() string {
	bytes := []byte{
		byte(t >> 24 & 0xff),
		byte(t >> 16 & 0xff),
		byte(t >> 8 & 0xff),
		byte(t & 0xff),
	}
	return string(bytes)
}

// --- Table -----------------------------------------------------------------

// Table represents one of the various OpenType font tables
//
// Required Tables, according to the OpenType specification:
// 'cmap' (Character to glyph mapping), 'head' (Font header), 'hhea' (Horizontal header),
// 'hmtx' (Horizontal metrics), 'maxp' (Maximum profile), 'name' (Naming table),
// 'OS/2' (OS/2 and Windows specific metrics), 'post' (PostScript information).
//
// Advanced Typographic Tables: 'BASE' (Baseline data), 'GDEF' (Glyph definition data),
// 'GPOS' (Glyph positioning data), 'GSUB' (Glyph substitution data),
// 'JSTF' (Justification data), 'MATH' (Math layout data).
//
// For TrueType outline fonts: 'cvt ' (Control Value Table, optional),
// 'fpgm' (Font program, optional), 'glyf' (Glyph data), 'loca' (Index to location),
// 'prep' (CVT Program, optional), 'gasp' (Grid-fitting/Scan-conversion, optional).
//
// For OpenType fonts based on CFF outlines: 'CFF ' (Compact Font Format 1.0),
// 'CFF2' (Compact Font Format 2.0), 'VORG' (Vertical Origin, optional).
//
// Currently not used/supported:
// SVG font table, bitmap glyph tables, color font tables, font variations.
type Table interface {
	Extent() (uint32, uint32) // offset and byte size within the font's binary data
	Binary() []byte           // the bytes of this table; should be treatet as read-only by clients
	Fields() Navigator        // start for navigation calls
	Self() TableSelf          // reference to itself
}

func newTable(tag Tag, b binarySegm, offset, size uint32) *genericTable {
	t := &genericTable{tableBase{
		data:   b,
		name:   tag,
		offset: offset,
		length: size,
	},
	}
	t.self = t
	return t
}

type genericTable struct {
	tableBase
}

// tableBase is a common parent for all kinds of OpenType tables.
type tableBase struct {
	data   binarySegm // a table is a slice of font data
	name   Tag        // 4-byte name as an integer
	offset uint32     // from offset
	length uint32     // to offset + length
	self   any
}

// Offset returns offset and byte size of this table within the OpenType font.
func (tb *tableBase) Extent() (uint32, uint32) {
	return tb.offset, tb.length
}

// Binary returns the bytes of this table. Should be treatet as read-only by
// clients, as it is a view into the original data.
func (tb *tableBase) Binary() []byte {
	return tb.data
}

// func (tb *tableBase) bytes() fontBinSegm {
// 	return tb.data
// }

func (tb *tableBase) Self() TableSelf {
	return TableSelf{tableBase: tb}
}

func (tb *tableBase) Fields() Navigator {
	tableTag := tb.name.String()
	return NavigatorFactory(tableTag, tb.data, tb.data)
}

// TableSelf is a reference to a table. Its primary use is for converting
// a generic table to a concrete table flavour, and for reproducing the
// name tag of a table.
type TableSelf struct {
	tableBase *tableBase
}

// NameTag returns the 4-letter name of a table.
func (tself TableSelf) NameTag() Tag {
	return tself.tableBase.name
}

func safeSelf(tself TableSelf) any {
	if tself.tableBase == nil || tself.tableBase.self == nil {
		return TableSelf{}
	}
	return tself.tableBase.self
}

// AsCMap returns this table as a cmap table, or nil.
func (tself TableSelf) AsCMap() *CMapTable {
	if k, ok := safeSelf(tself).(*CMapTable); ok {
		return k
	}
	return nil
}

// AsGPos returns this table as a GPOS table, or nil.
func (tself TableSelf) AsGPos() *GPosTable {
	if g, ok := safeSelf(tself).(*GPosTable); ok {
		return g
	}
	return nil
}

// AsGSub returns this table as a GSUB table, or nil.
func (tself TableSelf) AsGSub() *GSubTable {
	if g, ok := safeSelf(tself).(*GSubTable); ok {
		return g
	}
	return nil
}

// AsGDef returns this table as a GDEF table, or nil.
func (tself TableSelf) AsGDef() *GDefTable {
	if g, ok := safeSelf(tself).(*GDefTable); ok {
		return g
	}
	return nil
}

// AsBase returns this table as a BASE table, or nil.
func (tself TableSelf) AsBase() *BaseTable {
	if k, ok := safeSelf(tself).(*BaseTable); ok {
		return k
	}
	return nil
}

// AsLoca returns this table as a kern table, or nil.
func (tself TableSelf) AsLoca() *LocaTable {
	if k, ok := safeSelf(tself).(*LocaTable); ok {
		return k
	}
	return nil
}

// AsMaxP returns this table as a kern table, or nil.
func (tself TableSelf) AsMaxP() *MaxPTable {
	if k, ok := safeSelf(tself).(*MaxPTable); ok {
		return k
	}
	return nil
}

// AsHead returns this table as a head table, or nil.
func (tself TableSelf) AsHead() *HeadTable {
	if k, ok := safeSelf(tself).(*HeadTable); ok {
		return k
	}
	return nil
}

// AsHHea returns this table as a hhea table, or nil.
func (tself TableSelf) AsHHea() *HHeaTable {
	if k, ok := safeSelf(tself).(*HHeaTable); ok {
		return k
	}
	return nil
}

// AsOS2 returns this table as an OS/2 table, or nil.
func (tself TableSelf) AsOS2() *OS2Table {
	if k, ok := safeSelf(tself).(*OS2Table); ok {
		return k
	}
	return nil
}

// AsHMtx returns this table as a hmtx table, or nil.
func (tself TableSelf) AsHMtx() *HMtxTable {
	if k, ok := safeSelf(tself).(*HMtxTable); ok {
		return k
	}
	return nil
}

// --- Concrete table implementations ----------------------------------------

// HeadTable gives global information about the font.
// Only a small subset of fields are made public by HeadTable, as they are
// needed for consistency-checks. To read any of the other fields of table 'head' use:
//
//	head   := otf.Table(T("head"))
//	fields := head.Fields().Get(n)     // get nth field value
//	fields := head.Fields().All()      // get a slice with all field values
//
// See also type `Navigator`.
type HeadTable struct {
	tableBase
	Flags            uint16 // see https://docs.microsoft.com/en-us/typography/opentype/spec/head
	UnitsPerEm       uint16 // values 16 … 16384 are valid
	IndexToLocFormat uint16 // needed to interpret loca table
}

func newHeadTable(tag Tag, b binarySegm, offset, size uint32) *HeadTable {
	t := &HeadTable{}
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

// LocaTable stores the offsets to the locations of the glyphs in the font,
// relative to the beginning of the glyph data table.
// By definition, index zero points to the “missing character”, which is the character
// that appears if a character is not found in the font. The missing character is
// commonly represented by a blank box or a space.
type LocaTable struct {
	tableBase
	inx2loc func(t *LocaTable, gid GlyphIndex) uint32 // returns glyph location for glyph gid
	locCnt  int                                       // number of locations
}

// IndexToLocation offsets, indexed by glyph IDs, which provide the location of each
// glyph data block within the 'glyf' table.
func (t *LocaTable) IndexToLocation(gid GlyphIndex) uint32 {
	return t.inx2loc(t, gid)
}

func newLocaTable(tag Tag, b binarySegm, offset, size uint32) *LocaTable {
	t := &LocaTable{}
	base := tableBase{
		data:   b,
		name:   tag,
		offset: offset,
		length: size,
	}
	t.tableBase = base
	t.inx2loc = shortLocaVersion // may get changed by font consistency check
	t.locCnt = 0                 // has to be set during consistency check
	t.self = t
	return t
}

func shortLocaVersion(t *LocaTable, gid GlyphIndex) uint32 {
	// in case of error link to 'missing character' at location 0
	if gid >= GlyphIndex(t.locCnt) {
		return 0
	}
	loc, err := t.data.u16(int(gid) * 2)
	if err != nil {
		return 0
	}
	return uint32(loc) * 2
}

func longLocaVersion(t *LocaTable, gid GlyphIndex) uint32 {
	// in case of error link to 'missing character' at location 0
	if gid >= GlyphIndex(t.locCnt) {
		return 0
	}
	loc, err := t.data.u32(int(gid) * 4)
	if err != nil {
		return 0
	}
	return loc
}

// MaxPTable establishes the memory requirements for this font.
// The 'maxp' table contains a count for the number of glyphs in the font.
// Whenever this value changes, other tables which depend on it should also be updated.
type MaxPTable struct {
	tableBase
	NumGlyphs int
}

func newMaxPTable(tag Tag, b binarySegm, offset, size uint32) *MaxPTable {
	t := &MaxPTable{}
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

// HHeaTable contains information for horizontal layout.
type HHeaTable struct {
	tableBase
	Ascender            int16
	Descender           int16
	LineGap             int16
	AdvanceWidthMax     uint16
	MinLeftSideBearing  int16
	MinRightSideBearing int16
	XMaxExtent          int16
	CaretSlopeRise      int16
	CaretSlopeRun       int16
	CaretOffset         int16
	NumberOfHMetrics    int
}

func newHHeaTable(tag Tag, b binarySegm, offset, size uint32) *HHeaTable {
	t := &HHeaTable{}
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

// OS2Table contains a small, concrete subset of metrics from table 'OS/2'
// required for layout fallback decisions.
type OS2Table struct {
	tableBase
	Version       uint16
	XAvgCharWidth int16
	TypoAscender  int16
	TypoDescender int16
	TypoLineGap   int16
	WinAscent     uint16
	WinDescent    uint16
}

func newOS2Table(tag Tag, b binarySegm, offset, size uint32) *OS2Table {
	t := &OS2Table{}
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

// HMtxTable contains metric information for the horizontal layout each of the glyphs in
// the font. Each element in the contained hMetrics-array has two parts: the advance width
// and left side bearing. The value NumberOfHMetrics is taken from the `hhea` table. In
// a monospaced font, only one entry is required but that entry may not be omitted.
// Optionally, an array of left side bearings follows.
// The corresponding glyphs are assumed to have the same
// advance width as that found in the last entry in the hMetrics array. Since there
// must be a left side bearing and an advance width associated with each glyph in the font,
// the number of entries in this array is derived from the total number of glyphs in the
// font minus the value `HHea.NumberOfHMetrics`, which is copied into the
// HMtxTable for easier access.
type HMtxTable struct {
	tableBase
	NumberOfHMetrics int
	numGlyphs        int
	longMetrics      []HMetricRecord
	leftSideBearings []int16
}

// HMetricRecord is one long horizontal metric record from table hmtx.
type HMetricRecord struct {
	AdvanceWidth    uint16
	LeftSideBearing int16
}

func newHMtxTable(tag Tag, b binarySegm, offset, size uint32) *HMtxTable {
	t := &HMtxTable{}
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

func (t *HMtxTable) parseAll(numGlyphs, numberOfHMetrics int) error {
	if t == nil {
		return nil
	}
	if numGlyphs < 0 {
		return fmt.Errorf("invalid glyph count %d", numGlyphs)
	}
	if numberOfHMetrics < 0 || numberOfHMetrics > numGlyphs {
		return fmt.Errorf("invalid numberOfHMetrics %d (numGlyphs=%d)", numberOfHMetrics, numGlyphs)
	}
	required := numberOfHMetrics*4 + (numGlyphs-numberOfHMetrics)*2
	if required > len(t.data) {
		return fmt.Errorf("hmtx table too small: need %d bytes, have %d", required, len(t.data))
	}
	longMetrics := make([]HMetricRecord, numberOfHMetrics)
	for i := 0; i < numberOfHMetrics; i++ {
		aw, err := t.data.u16(i * 4)
		if err != nil {
			return fmt.Errorf("cannot parse hmtx long metric %d: %w", i, err)
		}
		lsb, err := t.data.u16(i*4 + 2)
		if err != nil {
			return fmt.Errorf("cannot parse hmtx long metric lsb %d: %w", i, err)
		}
		longMetrics[i] = HMetricRecord{
			AdvanceWidth:    aw,
			LeftSideBearing: int16(lsb),
		}
	}
	lsbCount := numGlyphs - numberOfHMetrics
	leftSideBearings := make([]int16, lsbCount)
	base := numberOfHMetrics * 4
	for i := 0; i < lsbCount; i++ {
		lsb, err := t.data.u16(base + i*2)
		if err != nil {
			return fmt.Errorf("cannot parse hmtx lsb %d: %w", i, err)
		}
		leftSideBearings[i] = int16(lsb)
	}
	t.NumberOfHMetrics = numberOfHMetrics
	t.numGlyphs = numGlyphs
	t.longMetrics = longMetrics
	t.leftSideBearings = leftSideBearings
	return nil
}

// LongMetrics returns a copy of all long horizontal metrics records.
func (t *HMtxTable) LongMetrics() []HMetricRecord {
	if t == nil || len(t.longMetrics) == 0 {
		return nil
	}
	metrics := make([]HMetricRecord, len(t.longMetrics))
	copy(metrics, t.longMetrics)
	return metrics
}

// LeftSideBearings returns a copy of trailing LSB records.
func (t *HMtxTable) LeftSideBearings() []int16 {
	if t == nil || len(t.leftSideBearings) == 0 {
		return nil
	}
	lsbs := make([]int16, len(t.leftSideBearings))
	copy(lsbs, t.leftSideBearings)
	return lsbs
}

// GlyphCount returns the glyph count used when decoding this hmtx table.
func (t *HMtxTable) GlyphCount() int {
	if t == nil {
		return 0
	}
	return t.numGlyphs
}

// HMetrics returns the advance width and left side bearing for a glyph.
func (t *HMtxTable) HMetrics(g GlyphIndex) (uint16, int16, bool) {
	if t == nil || t.numGlyphs == 0 || int(g) < 0 || int(g) >= t.numGlyphs {
		return 0, 0, false
	}
	if int(g) < len(t.longMetrics) {
		m := t.longMetrics[int(g)]
		return m.AdvanceWidth, m.LeftSideBearing, true
	}
	if len(t.longMetrics) == 0 {
		return 0, 0, false
	}
	i := int(g) - len(t.longMetrics)
	if i < 0 || i >= len(t.leftSideBearings) {
		return 0, 0, false
	}
	return t.longMetrics[len(t.longMetrics)-1].AdvanceWidth, t.leftSideBearings[i], true
}

// hMetrics returns the advance width and left side bearing of a glyph.
// TODO: call from font or from HMtx ?
func (t *HMtxTable) hMetrics(g GlyphIndex) (uint16, int16) {
	a, l, _ := t.HMetrics(g)
	return a, l
}
