package ot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Code comment often will cite passage from the
// OpenType specification version 1.8.4;
// see https://docs.microsoft.com/en-us/typography/opentype/spec/.

// ---------------------------------------------------------------------------

// Maximum reasonable counts for OpenType table structures.
// These limits prevent malicious fonts from claiming unreasonably large counts
// that could lead to excessive memory allocation or out-of-bounds reads.
const (
	MaxScriptCount    = 50    // Scripts: typically < 10
	MaxFeatureCount   = 500   // Features: typically < 200
	MaxLookupCount    = 1000  // Lookups: typically < 100
	MaxTagListCount   = 100   // Tag lists
	MaxGlyphCount     = 65536 // Maximum glyph index (uint16)
	MaxCoverageCount  = 65535 // Coverage tables
	MaxClassDefCount  = 65535 // Class definitions
	MaxRecordMapCount = 1000  // Generic tag record maps
)

// Maximum recursion/nesting depths to prevent stack overflow.
// These limits follow ttf-parser's approach of bounded recursion.
const (
	MaxExtensionDepth   = 16 // Maximum Extension lookup nesting
	MaxIndirectionDepth = 8  // Maximum varArray indirection levels
)

// ---------------------------------------------------------------------------

// Checked arithmetic operations to prevent integer overflow

// checkedMulInt checks for overflow in multiplication of two integers
func checkedMulInt(a, b int) (int, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a > 0 && b > 0 && a > math.MaxInt/b {
		return 0, fmt.Errorf("integer overflow: %d * %d", a, b)
	}
	if a < 0 && b < 0 && a < math.MaxInt/b {
		return 0, fmt.Errorf("integer overflow: %d * %d", a, b)
	}
	if (a < 0 && b > 0 && a < math.MinInt/b) || (a > 0 && b < 0 && b < math.MinInt/a) {
		return 0, fmt.Errorf("integer overflow: %d * %d", a, b)
	}
	return a * b, nil
}

// checkedAddInt checks for overflow in addition of two integers
func checkedAddInt(a, b int) (int, error) {
	if b > 0 && a > math.MaxInt-b {
		return 0, fmt.Errorf("integer overflow: %d + %d", a, b)
	}
	if b < 0 && a < math.MinInt-b {
		return 0, fmt.Errorf("integer overflow: %d + %d", a, b)
	}
	return a + b, nil
}

// checkedMulUint32 checks for overflow in multiplication of two uint32 values
func checkedMulUint32(a, b uint32) (uint32, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a > math.MaxUint32/b {
		return 0, fmt.Errorf("integer overflow: %d * %d", a, b)
	}
	return a * b, nil
}

// checkedAddUint32 checks for overflow in addition of two uint32 values
func checkedAddUint32(a, b uint32) (uint32, error) {
	if a > math.MaxUint32-b {
		return 0, fmt.Errorf("integer overflow: %d + %d", a, b)
	}
	return a + b, nil
}

// ---------------------------------------------------------------------------

// errFontFormat produces user level errors for font parsing.
// This is a compatibility helper that returns a simple error.
// In the future, this will be replaced with proper error collection.
func errFontFormat(message string) error {
	return fmt.Errorf("OpenType font format: %s", message)
}

// ---------------------------------------------------------------------------

// Parse parses an OpenType font from a byte slice.
// An ot.Font needs ongoing access to the fonts byte-data after the Parse function returns.
// Its elements are assumed immutable while the ot.Font remains in use.
func Parse(font []byte) (*Font, error) {
	// https://www.microsoft.com/typography/otspec/otff.htm: Offset Table is 12 bytes.
	r := bytes.NewReader(font)
	h := FontHeader{}
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		return nil, err
	}
	tracer().Debugf("header = %v, tag = %x|%s", h, h.FontType, Tag(h.FontType).String())

	// Create error collector for accumulating errors during parsing
	ec := &errorCollector{}

	if !(h.FontType == 0x4f54544f || // OTTO
		h.FontType == 0x00010000 || // TrueType
		h.FontType == 0x74727565) { // true
		ec.addError(T(""), "Header", fmt.Sprintf("font type not supported: %x", h.FontType), SeverityCritical, 0)
		return nil, errFontFormat(fmt.Sprintf("font type not supported: %x", h.FontType))
	}
	otf := &Font{Header: &h, tables: make(map[Tag]Table)}
	src := binarySegm(font)
	// "The Offset Table is followed immediately by the Table Record entries …
	// sorted in ascending order by tag", 16 bytes each.

	// Check for arithmetic overflow in table record size calculation
	tableRecordsSize, err := checkedMulInt(16, int(h.TableCount))
	if err != nil {
		ec.addError(T(""), "TableRecords", fmt.Sprintf("table count too large: %v", err), SeverityCritical, 12)
		return nil, errFontFormat(fmt.Sprintf("table count too large: %v", err))
	}

	buf, err := src.view(12, tableRecordsSize)
	if err != nil {
		ec.addError(T(""), "TableRecords", "table record entries", SeverityCritical, 12)
		return nil, errFontFormat("table record entries")
	}
	for b, prevTag := buf, Tag(0); len(b) > 0; b = b[16:] {
		tag := MakeTag(b)
		if tag < prevTag {
			ec.addError(T(""), "TableRecords", "table order", SeverityCritical, 12)
			return nil, errFontFormat("table order")
		}
		prevTag = tag
		off, size := u32(b[8:12]), u32(b[12:16])
		if off&3 != 0 { // ignore checksums, but "all tables must begin on four byte boundries".
			ec.addError(tag, "Offset", "invalid table offset", SeverityCritical, off)
			return nil, errFontFormat("invalid table offset")
		}

		// Validate table bounds before slicing to prevent panic
		tableEnd, err := checkedAddUint32(off, size)
		if err != nil {
			ec.addError(tag, "Size", fmt.Sprintf("size calculation overflow: %v", err), SeverityCritical, off)
			return nil, errFontFormat(fmt.Sprintf("table %s: size calculation overflow: %v", tag, err))
		}
		if off > uint32(len(src)) || tableEnd > uint32(len(src)) {
			ec.addError(tag, "Bounds", fmt.Sprintf("bounds [%d:%d] exceed font size %d", off, tableEnd, len(src)), SeverityCritical, off)
			return nil, errFontFormat(fmt.Sprintf("table %s: bounds [%d:%d] exceed font size %d",
				tag, off, tableEnd, len(src)))
		}

		otf.tables[tag], err = parseTable(tag, src[off:tableEnd], off, size, ec)
		if err != nil {
			return nil, err
		}
	}
	if err := extractLayoutInfo(otf, ec); err != nil {
		return nil, err
	}
	// Collect and centralize font information:
	// The number of glyphs in the font is restricted only by the value stated in the 'head' table. The order in which glyphs are placed in a font is arbitrary.
	// Note that a font must have at least two glyphs, and that glyph index 0 musthave an outline. See Glyph Mappings for details.
	//
	if hh := otf.tables[T("hhea")]; hh != nil {
		hhead := hh.Self().AsHHea()
		if mx := otf.tables[T("hmtx")]; mx != nil {
			hmtx := mx.Self().AsHMtx()
			hmtx.NumberOfHMetrics = hhead.NumberOfHMetrics
		}
	}
	if he := otf.Table(T("head")); he != nil {
		head := he.Self().AsHead()
		if lo := otf.Table(T("loca")); lo != nil {
			loca := lo.Self().AsLoca()
			if head.IndexToLocFormat == 1 {
				loca.inx2loc = longLocaVersion
			}
			if ma := otf.Table(T("maxp")); ma != nil {
				maxp := ma.Self().AsMaxP()
				loca.locCnt = maxp.NumGlyphs
			}
		}
	}

	// Transfer accumulated errors and warnings to the Font
	otf.parseErrors = ec.errors
	otf.parseWarnings = ec.warnings

	return otf, nil
}

// According to the OpenType spec, the following tables are
// required for the font to function correctly.
var RequiredTables = []string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post",
}

// These are the OpenType tables for advanced layout.
var LayoutTables = []string{
	"GSUB", "GPOS",
	//"GSUB", "GPOS", "GDEF", "BASE", "JSTF",
}

// Consistency check and shortcuts to essential tables, including layout tables.
func extractLayoutInfo(otf *Font, ec *errorCollector) error {
	for _, tag := range RequiredTables {
		h := otf.tables[T(tag)]
		if h == nil {
			ec.addError(T(tag), "Missing", "missing required table", SeverityCritical, 0)
			return errFontFormat("missing required table " + tag)
		}
	}
	otf.CMap = otf.tables[T("cmap")].Self().AsCMap()

	// Set NumGlyphs in CMap and GlyphIndexMap for glyph index validation
	if maxpTable := otf.Table(T("maxp")); maxpTable != nil {
		maxp := maxpTable.Self().AsMaxP()
		otf.CMap.NumGlyphs = maxp.NumGlyphs

		// Set numGlyphs in the concrete glyph index map types
		switch gim := otf.CMap.GlyphIndexMap.(type) {
		case format4GlyphIndex:
			gim.numGlyphs = maxp.NumGlyphs
			otf.CMap.GlyphIndexMap = gim
		case format12GlyphIndex:
			gim.numGlyphs = maxp.NumGlyphs
			otf.CMap.GlyphIndexMap = gim
		}
	}

	// We'll operate on OpenType fonts only, i.e. fonts containing GSUB and GPOS tables.
	for _, tag := range LayoutTables {
		h := otf.tables[T(tag)]
		if h == nil {
			ec.addError(T(tag), "Missing", "missing advanced layout table", SeverityCritical, 0)
			return errFontFormat("missing advanced layout table " + tag)
		}
	}
	// store shortcuts to layout tables
	otf.Layout.GSub = otf.tables[T("GSUB")].Self().AsGSub()
	otf.Layout.GPos = otf.tables[T("GPOS")].Self().AsGPos()
	if gdefTable := otf.tables[T("GDEF")]; gdefTable != nil {
		otf.Layout.GDef = gdefTable.Self().AsGDef()
	}
	//otf.Layout.Base = otf.tables[T("BASE")].Self().AsBase()
	//otf.Layout.Jstf = otf.tables[T("JSTF")].Self().AsJstf()

	// Collect layout requirements from parsed GSUB/GPOS lookup flags.
	otf.Layout.Requirements = LayoutRequirements{}
	if otf.Layout.GSub != nil {
		otf.Layout.Requirements.Merge(otf.Layout.GSub.Requirements)
	}
	if otf.Layout.GPos != nil {
		otf.Layout.Requirements.Merge(otf.Layout.GPos.Requirements)
	}

	// If GDEF is present, validate its version.
	if otf.Layout.GDef != nil {
		major, minor := otf.Layout.GDef.Header().Version()
		if major != 1 || minor > 3 {
			ec.addError(T("GDEF"), "Version", fmt.Sprintf("unsupported GDEF version %d.%d", major, minor), SeverityCritical, 0)
			return errFontFormat("unsupported GDEF version")
		}
	}

	// Enforce GDEF presence only when required by lookup flags.
	// TODO: apply the same requirement checks for JSTF lookups when JSTF parsing is enabled.
	req := otf.Layout.Requirements
	if req.NeedGlyphClassDef || req.NeedMarkAttachClassDef || req.NeedMarkGlyphSets {
		if otf.Layout.GDef == nil {
			ec.addError(T("GDEF"), "Missing", "missing required GDEF table", SeverityCritical, 0)
			return errFontFormat("missing required GDEF table")
		}
		if req.NeedGlyphClassDef && otf.Layout.GDef.Header().offsetFor(GDefGlyphClassDefSection) == 0 {
			ec.addError(T("GDEF"), "GlyphClassDef", "missing required GDEF GlyphClassDef", SeverityCritical, 0)
			return errFontFormat("missing required GDEF GlyphClassDef")
		}
		if req.NeedMarkAttachClassDef && otf.Layout.GDef.Header().offsetFor(GDefMarkAttachClassSection) == 0 {
			ec.addError(T("GDEF"), "MarkAttachClassDef", "missing required GDEF MarkAttachClassDef", SeverityCritical, 0)
			return errFontFormat("missing required GDEF MarkAttachClassDef")
		}
		if req.NeedMarkGlyphSets && otf.Layout.GDef.Header().offsetFor(GDefMarkGlyphSetsDefSection) == 0 {
			ec.addError(T("GDEF"), "MarkGlyphSetsDef", "missing required GDEF MarkGlyphSetsDef", SeverityCritical, 0)
			return errFontFormat("missing required GDEF MarkGlyphSetsDef")
		}
	}
	// GSUB/GPOS must have ScriptList, FeatureList, and LookupList
	gsub := otf.Layout.GSub
	if gsub.ScriptList.IsVoid() || gsub.FeatureList.Len() == 0 {
		ec.addError(T("GSUB"), "Structure", "GSUB table missing required lists", SeverityCritical, 0)
		return errFontFormat("GSUB table missing required lists")
	}

	// Perform cross-table consistency validation
	if err := validateCrossTableConsistency(otf, ec); err != nil {
		return err
	}

	return nil
}

// validateCrossTableConsistency performs cross-table validation to ensure
// internal consistency between related tables.
func validateCrossTableConsistency(otf *Font, ec *errorCollector) error {
	// Get maxp table for glyph count
	maxpTable := otf.Table(T("maxp"))
	if maxpTable == nil {
		ec.addError(T("maxp"), "Missing", "maxp table required for validation", SeverityCritical, 0)
		return errFontFormat("maxp table required for validation")
	}
	maxp := maxpTable.Self().AsMaxP()
	numGlyphs := maxp.NumGlyphs

	// Validate hhea.NumberOfHMetrics against hmtx table capacity
	hheaTable := otf.Table(T("hhea"))
	hmtxTable := otf.Table(T("hmtx"))
	if hheaTable != nil && hmtxTable != nil {
		hhea := hheaTable.Self().AsHHea()
		hmtx := hmtxTable.Self().AsHMtx()

		// NumberOfHMetrics must not exceed numGlyphs
		if hhea.NumberOfHMetrics > numGlyphs {
			ec.addError(T("hhea"), "NumberOfHMetrics",
				fmt.Sprintf("value %d exceeds maxp.NumGlyphs %d", hhea.NumberOfHMetrics, numGlyphs),
				SeverityMajor, 0)
			return errFontFormat(fmt.Sprintf("hhea.NumberOfHMetrics (%d) exceeds maxp.NumGlyphs (%d)",
				hhea.NumberOfHMetrics, numGlyphs))
		}

		// hmtx table size validation
		// hmtx contains NumberOfHMetrics longHorMetrics (4 bytes each) +
		// (numGlyphs - NumberOfHMetrics) leftSideBearings (2 bytes each)
		longMetricsSize, err := checkedMulInt(int(hhea.NumberOfHMetrics), 4)
		if err != nil {
			ec.addError(T("hmtx"), "Size", fmt.Sprintf("longMetrics size overflow: %v", err), SeverityCritical, 0)
			return errFontFormat(fmt.Sprintf("hmtx longMetrics size overflow: %v", err))
		}

		lsbCount := numGlyphs - hhea.NumberOfHMetrics
		lsbSize, err := checkedMulInt(lsbCount, 2)
		if err != nil {
			ec.addError(T("hmtx"), "Size", fmt.Sprintf("leftSideBearings size overflow: %v", err), SeverityCritical, 0)
			return errFontFormat(fmt.Sprintf("hmtx leftSideBearings size overflow: %v", err))
		}

		requiredSize, err := checkedAddInt(longMetricsSize, lsbSize)
		if err != nil {
			ec.addError(T("hmtx"), "Size", fmt.Sprintf("total size overflow: %v", err), SeverityCritical, 0)
			return errFontFormat(fmt.Sprintf("hmtx total size overflow: %v", err))
		}

		if int(hmtx.length) < requiredSize {
			ec.addError(T("hmtx"), "Size",
				fmt.Sprintf("table size %d insufficient for %d glyphs (need %d)", hmtx.length, numGlyphs, requiredSize),
				SeverityCritical, 0)
			return errFontFormat(fmt.Sprintf("hmtx table size (%d) insufficient for %d glyphs (need %d)",
				hmtx.length, numGlyphs, requiredSize))
		}
	}

	// Validate head.IndexToLocFormat consistency with loca table
	headTable := otf.Table(T("head"))
	locaTable := otf.Table(T("loca"))
	if headTable != nil && locaTable != nil {
		head := headTable.Self().AsHead()
		loca := locaTable.Self().AsLoca()

		// Calculate expected loca table size based on IndexToLocFormat
		if head.IndexToLocFormat == 0 {
			// Short format: (numGlyphs + 1) * 2 bytes
			expectedLocaSize, err := checkedMulInt(numGlyphs+1, 2)
			if err != nil {
				ec.addError(T("loca"), "Size", fmt.Sprintf("size calculation overflow: %v", err), SeverityCritical, 0)
				return errFontFormat(fmt.Sprintf("loca size calculation overflow: %v", err))
			}
			if int(loca.length) < expectedLocaSize {
				ec.addError(T("loca"), "Size", fmt.Sprintf("table size (%d) insufficient for %d glyphs in short format (need %d)", loca.length, numGlyphs, expectedLocaSize), SeverityCritical, 0)
				return errFontFormat(fmt.Sprintf("loca table size (%d) insufficient for %d glyphs in short format (need %d)",
					loca.length, numGlyphs, expectedLocaSize))
			}
		} else if head.IndexToLocFormat == 1 {
			// Long format: (numGlyphs + 1) * 4 bytes
			expectedLocaSize, err := checkedMulInt(numGlyphs+1, 4)
			if err != nil {
				ec.addError(T("loca"), "Size", fmt.Sprintf("size calculation overflow: %v", err), SeverityCritical, 0)
				return errFontFormat(fmt.Sprintf("loca size calculation overflow: %v", err))
			}
			if int(loca.length) < expectedLocaSize {
				ec.addError(T("loca"), "Size", fmt.Sprintf("table size (%d) insufficient for %d glyphs in long format (need %d)", loca.length, numGlyphs, expectedLocaSize), SeverityCritical, 0)
				return errFontFormat(fmt.Sprintf("loca table size (%d) insufficient for %d glyphs in long format (need %d)",
					loca.length, numGlyphs, expectedLocaSize))
			}
		} else {
			ec.addError(T("head"), "IndexToLocFormat", fmt.Sprintf("invalid value: %d (must be 0 or 1)", head.IndexToLocFormat), SeverityCritical, 0)
			return errFontFormat(fmt.Sprintf("invalid head.IndexToLocFormat: %d (must be 0 or 1)",
				head.IndexToLocFormat))
		}
	}

	// Validate that glyph indices in cmap don't exceed numGlyphs
	if otf.CMap != nil {
		// This is validated during cmap lookup, but we can add a spot check here
		// The actual validation happens in the Lookup methods
		tracer().Debugf("Cross-table validation: maxp.NumGlyphs = %d", numGlyphs)
	}

	return nil
}

func parseTable(t Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	switch t {
	case T("BASE"):
		return parseBase(t, b, offset, size, ec)
	case T("cmap"):
		return parseCMap(t, b, offset, size, ec)
	case T("head"):
		return parseHead(t, b, offset, size, ec)
	case T("glyf"):
		// We do not parse the glyf table (glyph outline data).
		// For shaping and layout, all necessary metrics are provided by hmtx (advance width, LSB).
		// The glyf table contains outline data for rendering, which is out of scope.
		return newTable(t, b, offset, size), nil
	case T("GDEF"):
		return parseGDef(t, b, offset, size, ec)
	case T("GPOS"):
		return parseGPos(t, b, offset, size, ec)
	case T("GSUB"):
		return parseGSub(t, b, offset, size, ec)
	case T("hhea"):
		return parseHHea(t, b, offset, size, ec)
	case T("hmtx"):
		return parseHMtx(t, b, offset, size, ec)
	case T("kern"):
		return parseKern(t, b, offset, size, ec)
	case T("loca"):
		return parseLoca(t, b, offset, size, ec)
	case T("maxp"):
		return parseMaxP(t, b, offset, size, ec)
	}
	tracer().Infof("font contains table (%s), will not be interpreted", t)
	// Record as minor warning - not parsed but not a problem
	ec.addWarning(t, fmt.Sprintf("table not interpreted"), offset)
	return newTable(t, b, offset, size), nil
}

// --- Head table ------------------------------------------------------------

func parseHead(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	if size < 54 {
		ec.addError(tag, "Size", fmt.Sprintf("head table too small: %d bytes (need 54)", size), SeverityCritical, offset)
		return nil, errFontFormat("size of head table")
	}
	t := newHeadTable(tag, b, offset, size)
	t.Flags, _ = b.u16(16)      // flags
	t.UnitsPerEm, _ = b.u16(18) // units per em
	// IndexToLocFormat is needed to interpret the loca table:
	// 0 for short offsets, 1 for long
	t.IndexToLocFormat, _ = b.u16(50)
	return t, nil
}

// --- BASE table ------------------------------------------------------------

// The Baseline table (BASE) provides information used to align glyphs of different
// scripts and sizes in a line of text, whether the glyphs are in the same font or
// in different fonts.
func parseBase(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	var err error
	base := newBaseTable(tag, b, offset, size)
	// The BASE table begins with offsets to Axis tables that describe layout data for
	// the horizontal and vertical layout directions of text. A font can provide layout
	// data for both text directions or for only one text direction.
	xaxis, errx := parseLink16(b, 4, b, "Axis")
	yaxis, erry := parseLink16(b, 6, b, "Axis")
	if errx != nil || erry != nil {
		ec.addError(tag, "Axis", "BASE table axis-tables", SeverityCritical, offset)
		return nil, errFontFormat("BASE table axis-tables")
	}
	err = parseBaseAxis(base, 0, xaxis, err)
	err = parseBaseAxis(base, 1, yaxis, err)
	if err != nil {
		tracer().Errorf("error parsing BASE table: %v", err)
		return base, err
	}
	return base, err
}

// An Axis table consists of offsets, measured from the beginning of the Axis table,
// to a BaseTagList and a BaseScriptList.
// link may be NULL.
func parseBaseAxis(base *BaseTable, hOrV int, link NavLink, err error) error {
	if err != nil {
		return err
	}
	base.axisTables[hOrV] = AxisTable{}
	if link.IsNull() {
		return nil
	}
	axisHeader := link.Jump()
	axisbase := axisHeader.Bytes()
	// The BaseTagList enumerates all baselines used to render the scripts in the
	// text layout direction. If no baseline data is available for a text direction,
	// the offset to the corresponding BaseTagList may be set to NULL.
	if basetags, err := parseLink16(axisbase, 0, axisbase, "BaseTagList"); err == nil {
		b := basetags.Jump()
		base.axisTables[hOrV].baselineTags = parseTagList(b.Bytes())
		tracer().Debugf("axis table %d has %d entries", hOrV,
			base.axisTables[hOrV].baselineTags.Count)
	}
	// For each script listed in the BaseScriptList table, a BaseScriptRecord must be
	// defined that identifies the script and references its layout data.
	// BaseScriptRecords are stored in the baseScriptRecords array, ordered
	// alphabetically by the baseScriptTag in each record.
	if basescripts, err := parseLink16(axisbase, 2, axisbase, "BaseScriptList"); err == nil {
		b := basescripts.Jump()
		base.axisTables[hOrV].baseScriptRecords = parseTagRecordMap16(b.Bytes(),
			0, b.Bytes(), "BaseScriptList", "BaseScript")

		tracer().Debugf("axis table %d has %d entries", hOrV,
			base.axisTables[hOrV].baselineTags.Count)
	}
	tracer().Infof("BASE axis %d has no/unreadable entires", hOrV)
	return nil
}

// --- CMap table ------------------------------------------------------------

// This table defines mapping of character codes to a default glyph index. Different
// subtables may be defined that each contain mappings for different character encoding
// schemes. The table header indicates the character encodings for which subtables are
// present.
//
// From the spec.: “Apart from a format 14 subtable, all other subtables are exclusive:
// applications should select and use one and ignore the others. […]
// If a font includes Unicode subtables for both 16-bit encoding (typically, format 4)
// and also 32-bit encoding (formats 10 or 12), then the characters supported by the
// subtable for 32-bit encoding should be a superset of the characters supported by
// the subtable for 16-bit encoding, and the 32-bit encoding should be used by
// applications. Fonts should not include 16-bit Unicode subtables using both format 4
// and format 6; format 4 should be used. Similarly, fonts should not include 32-bit
// Unicode subtables using both format 10 and format 12; format 12 should be used.
// If a font includes encoding records for Unicode subtables of the same format but
// with different platform IDs, an application may choose which to select, but should
// make this selection consistently each time the font is used.”
//
// From Apple: // https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6cmap.html
// “The use of the Macintosh platformID is currently discouraged. Subtables with a
//
//	Macintosh platformID are only required for backwards compatibility.”
//
// and:
// “The Unicode platform's platform-specific ID 6 was intended to mark a 'cmap' subtable
//
//	as one used by a last resort font. This is not required by any Apple platform.”
//
// All in all, we only support the following plaform/encoding/format combinations:
//
//	0 (Unicode)  3    4   Unicode BMB
//	0 (Unicode)  4    12  Unicode full
//	3 (Win)      1    4   Unicode BMP
//	3 (Win)      10   12  Unicode full
func parseCMap(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	n, _ := b.u16(2) // number of sub-tables
	tracer().Debugf("font cmap has %d sub-tables in %d|%d bytes", n, len(b), size)
	t := newCMapTable(tag, b, offset, size)
	const headerSize, entrySize = 4, 8

	// Check for overflow in cmap size calculation
	entriesSize, err := checkedMulUint32(entrySize, uint32(n))
	if err != nil {
		ec.addError(tag, "Header", fmt.Sprintf("entries size overflow: %v", err), SeverityCritical, offset)
		return nil, errFontFormat(fmt.Sprintf("cmap entries size overflow: %v", err))
	}
	requiredSize, err := checkedAddUint32(headerSize, entriesSize)
	if err != nil {
		ec.addError(tag, "Header", fmt.Sprintf("table size overflow: %v", err), SeverityCritical, offset)
		return nil, errFontFormat(fmt.Sprintf("cmap table size overflow: %v", err))
	}
	if size < requiredSize {
		ec.addError(tag, "Header", fmt.Sprintf("table size %d < required %d", size, requiredSize), SeverityCritical, offset)
		return nil, errFontFormat("size of cmap table")
	}
	var enc encodingRecord
	for i := 0; i < int(n); i++ {
		rec, _ := b.view(headerSize+entrySize*i, entrySize)
		pid, psid := u16(rec), u16(rec[2:])
		width := platformEncodingWidth(pid, psid)
		if width <= enc.width {
			continue
		}
		link, err := parseLink32(rec, 4, b, "cmap.Subtable")
		if err != nil {
			tracer().Infof("cmap sub-table cannot be parsed")
			ec.addWarning(tag, fmt.Sprintf("sub-table %d (platform=%d, encoding=%d) cannot be parsed", i, pid, psid), offset)
			continue
		}
		subtable := link.Jump()
		format := subtable.U16(0)
		tracer().Debugf("cmap table contains subtable with format %d", format)
		if supportedCmapFormat(format, pid, psid) {
			enc.width = width
			enc.format = format
			enc.link = link
		}
	}
	if enc.width == 0 {
		ec.addError(tag, "Format", "no supported cmap format found", SeverityMajor, offset)
		return nil, errFontFormat("no supported cmap format found")
	}
	t.GlyphIndexMap, err = makeGlyphIndex(b, enc, tag, offset, ec)
	if err != nil {
		return nil, err
	}
	return t, nil
}

type encodingRecord struct {
	platformId uint16
	encodingId uint16
	link       NavLink
	format     uint16
	size       int
	width      int // encoding width in bytes
}

// --- Kern table ------------------------------------------------------------

type kernSubTableHeader struct {
	directory [4]uint16 // information to support binary search on sub-table
	offset    uint16    // start position of this sub-table's kern pairs
	length    uint32    // size of the sub-table in bytes, without header
	coverage  uint16    // info about type of information contained in this sub-table
}

// TrueType and OpenType slightly differ on formats of kern tables:
// see https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6kern.html
// and https://docs.microsoft.com/en-us/typography/opentype/spec/kern

// parseKern parses the kern table. There is significant confusion with this table
// concerning format differences between OpenType, TrueType, and fonts in the wild.
// We currently only support kern table format 0, which should be supported on any
// platform. In the real world, fonts usually have just one kern sub-table, and
// older Windows versions cannot handle more than one.
func parseKern(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	if size <= 4 {
		return nil, nil
	}
	var N, suboffset, subheaderlen int
	if version := u32(b); version == 0x00010000 {
		tracer().Debugf("font has Apple TTF kern table format")
		n, _ := b.u32(4) // number of kerning tables is uint32
		N, suboffset, subheaderlen = int(n), 8, 16
	} else {
		tracer().Debugf("font has OTF (MS) kern table format")
		n, _ := b.u16(2) // number of kerning tables is uint16
		N, suboffset, subheaderlen = int(n), 4, 14
	}
	tracer().Debugf("kern table has %d sub-tables", N)
	t := newKernTable(tag, b, offset, size)
	for i := 0; i < N; i++ { // read in N sub-tables
		if suboffset+subheaderlen >= int(size) { // check for sub-table header size
			ec.addError(tag, "Format", fmt.Sprintf("sub-table %d header exceeds table size", i), SeverityCritical, offset+uint32(suboffset))
			return nil, errFontFormat("kern table format")
		}
		h := kernSubTableHeader{
			offset: uint16(suboffset + subheaderlen),
			// sub-tables are of varying size; size may be off ⇒ see below
			length:   uint32(u16(b[suboffset+2:]) - uint16(subheaderlen)),
			coverage: u16(b[suboffset+4:]),
		}
		if format := h.coverage >> 8; format != 0 {
			tracer().Infof("kern sub-table format %d not supported, ignoring sub-table", format)
			continue // we only support format 0 kerning tables; skip this one
		}
		h.directory = [4]uint16{
			u16(b[suboffset+subheaderlen-8:]),
			u16(b[suboffset+subheaderlen-6:]),
			u16(b[suboffset+subheaderlen-4:]),
			u16(b[suboffset+subheaderlen-2:]),
		}
		kerncnt := uint32(h.directory[0])
		tracer().Debugf("kern sub-table has %d entries", kerncnt)
		// For some fonts, size calculation of kern sub-tables is off; see
		// https://github.com/fonttools/fonttools/issues/314#issuecomment-118116527
		// Testable with the Calibri font.
		sz, err := checkedMulUint32(kerncnt, 6) // kern pair is of size 6
		if err != nil {
			ec.addError(tag, "Size", fmt.Sprintf("sub-table %d size overflow: %v", i, err), SeverityCritical, offset+uint32(suboffset))
			return nil, errFontFormat(fmt.Sprintf("kern sub-table size overflow: %v", err))
		}
		if sz != h.length {
			tracer().Infof("kern sub-table size should be 0x%x, but given as 0x%x; fixing",
				sz, h.length)
			// Record as warning - this is a known issue with some fonts (e.g., Calibri)
			ec.addWarning(tag, fmt.Sprintf("kern sub-table size mismatch: expected 0x%x, got 0x%x", sz, h.length), offset+uint32(suboffset))
		}
		if uint32(suboffset)+sz >= size {
			ec.addError(tag, "Bounds", fmt.Sprintf("sub-table %d exceeds table bounds", i), SeverityCritical, offset+uint32(suboffset))
			return nil, errFontFormat("kern sub-table size exceeds kern table bounds")
		}
		t.headers = append(t.headers, h)
		suboffset += int(subheaderlen + int(h.length))
	}
	tracer().Debugf("table kern has %d sub-table(s)", len(t.headers))
	return t, nil
}

// --- Loca table ------------------------------------------------------------

// Dependencies (taken from Apple Developer page about TrueType):
// The size of entries in the 'loca' table must be appropriate for the value of the
// indexToLocFormat field of the 'head' table. The number of entries must be the same
// as the numGlyphs field of the 'maxp' table.
// The 'loca' table is most intimately dependent upon the contents of the 'glyf' table
// and vice versa. Changes to the 'loca' table must not be made unless appropriate
// changes to the 'glyf' table are simultaneously made.
func parseLoca(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	return newLocaTable(tag, b, offset, size), nil
}

// --- MaxP table ------------------------------------------------------------

// This table establishes the memory requirements for this font. Fonts with CFF data
// must use Version 0.5 of this table, specifying only the numGlyphs field. Fonts
// with TrueType outlines must use Version 1.0 of this table, where all data is required.
func parseMaxP(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	if size <= 6 {
		return nil, nil
	}
	t := newMaxPTable(tag, b, offset, size)
	n, _ := b.u16(4)
	t.NumGlyphs = int(n)
	return t, nil
}

// --- HHea table ------------------------------------------------------------

// This table establishes the memory requirements for this font. Fonts with CFF data
// must use Version 0.5 of this table, specifying only the numGlyphs field. Fonts
// with TrueType outlines must use Version 1.0 of this table, where all data is required.
func parseHHea(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	if size == 0 {
		return nil, nil
	}
	tracer().Debugf("HHea table has size %d", size)
	if size < 36 {
		ec.addError(tag, "Size", fmt.Sprintf("hhea table too small: %d bytes (need 36)", size), SeverityCritical, offset)
		return nil, errFontFormat("hhea table incomplete")
	}
	t := newHHeaTable(tag, b, offset, size)
	n, _ := b.u16(34)
	t.NumberOfHMetrics = int(n)
	return t, nil
}

// --- HMtx table ------------------------------------------------------------

// Dependencies (taken from Apple Developer page about TrueType):
// The value of the numOfLongHorMetrics field is found in the 'hhea' (Horizontal Header)
// table. Fonts that lack an 'hhea' table must not have an 'hmtx' table.
// Other tables may have information duplicating data contained in the 'hmtx' table.
// For example, glyph metrics can also be found in the 'hdmx' (Horizontal Device Metrics)
// table and 'bloc' (Bitmap Location) table. There is naturally no requirement that
// the ideal metrics of the 'hmtx' table be perfectly consistent with the device metrics
// found in other tables, but care should be taken that they are not significantly
// inconsistent.
func parseHMtx(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	if size == 0 {
		return nil, nil
	}
	t := newHMtxTable(tag, b, offset, size)
	return t, nil
}

// --- Names -----------------------------------------------------------------

func parseNames(b binarySegm) (nameNames, error) {
	if len(b) < 6 {
		return nameNames{}, errFontFormat("name section corrupt")
	}
	N, _ := b.u16(2)
	names := nameNames{}
	strOffset, _ := b.u16(4)

	// Validate string offset bounds
	if int(strOffset) > len(b) {
		return nameNames{}, errFontFormat(fmt.Sprintf("name table string offset %d exceeds table size %d", strOffset, len(b)))
	}
	names.strbuf = b[strOffset:]
	tracer().Debugf("name table has %d strings, starting at %d", N, strOffset)

	// Check for arithmetic overflow in name records size calculation
	nameRecsSize, err := checkedMulInt(12, int(N))
	if err != nil {
		return nameNames{}, errFontFormat(fmt.Sprintf("name table records size overflow: %v", err))
	}
	requiredSize, err := checkedAddInt(6, nameRecsSize)
	if err != nil {
		return nameNames{}, errFontFormat(fmt.Sprintf("name table size calculation overflow: %v", err))
	}
	if len(b) < requiredSize {
		return nameNames{}, errFontFormat("name section corrupt")
	}
	recs := b[6 : 6+nameRecsSize]
	names.nameRecs = viewArray(recs, 12)
	return names, nil
}

// --- GDEF table ------------------------------------------------------------

// The Glyph Definition (GDEF) table provides various glyph properties used in
// OpenType Layout processing.
func parseGDef(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	var err error
	gdef := newGDefTable(tag, b, offset, size)
	err = parseGDefHeader(gdef, b, err, tag, offset, ec)
	err = parseGlyphClassDefinitions(gdef, b, err)
	err = parseAttachmentPointList(gdef, b, err, tag, offset, ec)
	// We do not parse the Ligature Caret List Table (used for text editing/cursor positioning).
	// This is not needed for layout analysis and glyph metrics extraction.
	err = parseMarkAttachmentClassDef(gdef, b, err)
	err = parseMarkGlyphSets(gdef, b, err, tag, offset, ec)
	// We do not parse the Item Variation Store (GDEF v1.3, variable fonts only).
	// Variable font support may be added in the future.
	if err != nil {
		tracer().Errorf("error parsing GDEF table: %v", err)
		return gdef, err
	}
	mj, mn := gdef.Header().Version()
	tracer().Debugf("GDEF table has version %d.%d", mj, mn)
	return gdef, err
}

// The GDEF table begins with a header that starts with a version number. Three
// versions are defined. Version 1.0 contains an offset to a Glyph Class Definition
// table (GlyphClassDef), an offset to an Attachment List table (AttachList), an offset
// to a Ligature Caret List table (LigCaretList), and an offset to a Mark Attachment
// Class Definition table (MarkAttachClassDef). Version 1.2 also includes an offset to
// a Mark Glyph Sets Definition table (MarkGlyphSetsDef). Version 1.3 also includes an
// offset to an Item Variation Store table.
func parseGDefHeader(gdef *GDefTable, b binarySegm, err error, tag Tag, offset uint32, ec *errorCollector) error {
	if err != nil {
		return err
	}
	if len(b) < 12 {
		ec.addError(tag, "Header", fmt.Sprintf("GDEF header too small: %d bytes (need 12)", len(b)), SeverityCritical, offset)
		return errFontFormat("GDEF table header too small")
	}

	h := GDefHeader{}
	r := bytes.NewReader(b)
	if err = binary.Read(r, binary.BigEndian, &h.gDefHeaderV1_0); err != nil {
		return err
	}
	headerlen := 12

	// Validate version
	if h.Major != 1 || h.Minor > 3 {
		return fmt.Errorf("unsupported GDEF version %d.%d", h.Major, h.Minor)
	}

	if h.versionHeader.Minor >= 2 {
		if len(b) < headerlen+2 {
			ec.addError(tag, "Header", "GDEF v1.2+ header incomplete", SeverityCritical, offset)
			return errFontFormat("GDEF v1.2+ header incomplete")
		}
		h.MarkGlyphSetsDefOffset, _ = b.u16(headerlen)
		headerlen += 2
	}
	if h.versionHeader.Minor >= 3 {
		if len(b) < headerlen+4 {
			ec.addError(tag, "Header", "GDEF v1.3+ header incomplete", SeverityCritical, offset)
			return errFontFormat("GDEF v1.3+ header incomplete")
		}
		h.ItemVarStoreOffset, _ = b.u32(headerlen)
		headerlen += 4
	}

	// Validate all offsets point within table bounds
	tableSize := len(b)
	if h.GlyphClassDefOffset > 0 && int(h.GlyphClassDefOffset) >= tableSize {
		return fmt.Errorf("GDEF GlyphClassDef offset out of bounds: %d >= %d",
			h.GlyphClassDefOffset, tableSize)
	}
	if h.AttachListOffset > 0 && int(h.AttachListOffset) >= tableSize {
		return fmt.Errorf("GDEF AttachList offset out of bounds: %d >= %d",
			h.AttachListOffset, tableSize)
	}
	if h.LigCaretListOffset > 0 && int(h.LigCaretListOffset) >= tableSize {
		return fmt.Errorf("GDEF LigCaretList offset out of bounds: %d >= %d",
			h.LigCaretListOffset, tableSize)
	}
	if h.MarkAttachClassDefOffset > 0 && int(h.MarkAttachClassDefOffset) >= tableSize {
		return fmt.Errorf("GDEF MarkAttachClassDef offset out of bounds: %d >= %d",
			h.MarkAttachClassDefOffset, tableSize)
	}
	if h.Minor >= 2 && h.MarkGlyphSetsDefOffset > 0 && int(h.MarkGlyphSetsDefOffset) >= tableSize {
		return fmt.Errorf("GDEF MarkGlyphSetsDef offset out of bounds: %d >= %d",
			h.MarkGlyphSetsDefOffset, tableSize)
	}
	if h.Minor >= 3 && h.ItemVarStoreOffset > 0 && int(h.ItemVarStoreOffset) >= tableSize {
		return fmt.Errorf("GDEF ItemVarStore offset out of bounds: %d >= %d",
			h.ItemVarStoreOffset, tableSize)
	}

	gdef.header = h
	gdef.header.headerSize = uint8(headerlen)
	return err
}

// This table uses the same format as the Class Definition table (defined in the
// OpenType Layout Common Table Formats chapter).
func parseGlyphClassDefinitions(gdef *GDefTable, b binarySegm, err error) error {
	if err != nil {
		return err
	}
	offset := gdef.Header().offsetFor(GDefGlyphClassDefSection)
	if offset >= len(b) {
		return io.ErrUnexpectedEOF
	}
	b = b[offset:]
	cdef, err := parseClassDefinitions(b)
	if err != nil {
		return err
	}
	gdef.GlyphClassDef = cdef
	return nil
}

/*
AttachList:
Type      Name                            Description
---------+-------------------------------+-----------------------
Offset16  coverageOffset                  Offset to Coverage table - from beginning of AttachList table
uint16    glyphCount                      Number of glyphs with attachment points
Offset16  attachPointOffsets[glyphCount]  Array of offsets to AttachPoint tables-from beginning of

	AttachList table-in Coverage Index order
*/
func parseAttachmentPointList(gdef *GDefTable, b binarySegm, err error, tag Tag, tableOffset uint32, ec *errorCollector) error {
	if err != nil {
		return err
	}
	offset := gdef.Header().offsetFor(GDefAttachListSection)
	if offset >= len(b) {
		return io.ErrUnexpectedEOF
	}
	b = b[offset:]
	if len(b) < 4 {
		ec.addError(tag, "AttachList", "attachment point list header too small", SeverityCritical, tableOffset+uint32(offset))
		return errFontFormat("GDEF attachment point list header too small")
	}

	count, err := b.u16(2)
	if err != nil {
		ec.addError(tag, "AttachList", "corrupt attachment point list", SeverityCritical, tableOffset+uint32(offset))
		return errFontFormat("GDEF has corrupt attachment point list")
	}
	if count == 0 {
		return nil // no entries
	}

	// Validate count and buffer size (each offset is 2 bytes)
	requiredSize := 4 + int(count)*2
	if requiredSize > len(b) {
		return fmt.Errorf("GDEF attachment point list: count %d requires %d bytes, have %d",
			count, requiredSize, len(b))
	}

	covOffset := u16(b)
	if int(covOffset) >= len(b) {
		ec.addError(tag, "AttachList", "coverage offset out of bounds", SeverityCritical, tableOffset+uint32(offset))
		return errFontFormat("GDEF attachment point coverage offset out of bounds")
	}
	coverage := parseCoverage(b[covOffset:])
	if coverage.GlyphRange == nil {
		ec.addError(tag, "AttachList", "coverage table unreadable", SeverityCritical, tableOffset+uint32(offset)+uint32(covOffset))
		return errFontFormat("GDEF attachment point coverage table unreadable")
	}

	gdef.AttachmentPointList = AttachmentPointList{
		Count:              int(count),
		Coverage:           coverage.GlyphRange,
		attachPointOffsets: b[4:],
	}
	return nil
}

// A Mark Attachment Class Definition Table defines the class to which a mark glyph may
// belong. This table uses the same format as the Class Definition table.
func parseMarkAttachmentClassDef(gdef *GDefTable, b binarySegm, err error) error {
	if err != nil {
		return err
	}
	offset := gdef.Header().offsetFor(GDefMarkAttachClassSection)
	if offset >= len(b) {
		return io.ErrUnexpectedEOF
	}
	b = b[offset:]
	cdef, err := parseClassDefinitions(b)
	if err != nil {
		return err
	}
	gdef.MarkAttachmentClassDef = cdef
	return nil
}

// Mark glyph sets are defined in a MarkGlyphSets table, which contains offsets to
// individual sets each represented by a standard Coverage table.
func parseMarkGlyphSets(gdef *GDefTable, b binarySegm, err error, tag Tag, tableOffset uint32, ec *errorCollector) error {
	if err != nil {
		return err
	}
	offset := gdef.Header().offsetFor(GDefMarkGlyphSetsDefSection)
	if offset >= len(b) {
		return io.ErrUnexpectedEOF
	}
	b = b[offset:]
	if len(b) < 4 {
		ec.addError(tag, "MarkGlyphSets", "mark glyph sets header too small", SeverityCritical, tableOffset+uint32(offset))
		return errFontFormat("GDEF mark glyph sets header too small")
	}

	count, _ := b.u16(2)

	// Validate count and buffer size (each offset is 4 bytes)
	requiredSize := 4 + int(count)*4
	if requiredSize > len(b) {
		return fmt.Errorf("GDEF mark glyph sets: count %d requires %d bytes, have %d",
			count, requiredSize, len(b))
	}

	for i := 0; i < int(count); i++ {
		covOffset, _ := b.u32(4 + i*4)
		if int(covOffset) >= len(b) {
			return fmt.Errorf("GDEF mark glyph set %d: coverage offset %d out of bounds", i, covOffset)
		}
		coverage := parseCoverage(b[covOffset:])
		if coverage.GlyphRange == nil {
			ec.addError(tag, "MarkGlyphSets", fmt.Sprintf("mark glyph set %d coverage table unreadable", i), SeverityCritical, tableOffset+uint32(offset)+covOffset)
			return errFontFormat("GDEF mark glyph set coverage table unreadable")
		}
		gdef.MarkGlyphSets = append(gdef.MarkGlyphSets, coverage.GlyphRange)
	}
	return nil
}

// === Common Code for GPOS and GSUB =========================================

// parseLayoutHeader parses a layout table header, i.e. reads version information
// and header information (containing offsets).
// Supports header versions 1.0 and 1.1
func parseLayoutHeader(lytt *LayoutTable, b binarySegm, err error, tableTag Tag, ec *errorCollector) error {
	if err != nil {
		return err
	}
	if len(b) < 10 {
		ec.addError(tableTag, "Header", fmt.Sprintf("header too small: %d bytes", len(b)), SeverityCritical, 0)
		return errFontFormat("layout table header too small")
	}

	h := &LayoutHeader{}
	r := bytes.NewReader(b)
	if err = binary.Read(r, binary.BigEndian, &h.versionHeader); err != nil {
		return err
	}
	if h.Major != 1 || (h.Minor != 0 && h.Minor != 1) {
		ec.addError(tableTag, "Header", fmt.Sprintf("unsupported version %d.%d", h.Major, h.Minor), SeverityMajor, 0)
		return fmt.Errorf("unsupported layout version (major: %d, minor: %d)",
			h.Major, h.Minor)
	}

	switch h.Minor {
	case 0:
		if len(b) < 10 {
			ec.addError(tableTag, "Header", "v1.0 header incomplete", SeverityCritical, 0)
			return errFontFormat("layout v1.0 header incomplete")
		}
		if err = binary.Read(r, binary.BigEndian, &h.offsets.layoutHeader10); err != nil {
			return err
		}
	case 1:
		if len(b) < 14 {
			ec.addError(tableTag, "Header", "v1.1 header incomplete", SeverityCritical, 0)
			return errFontFormat("layout v1.1 header incomplete")
		}
		if err = binary.Read(r, binary.BigEndian, &h.offsets); err != nil {
			return err
		}
	}

	// Validate all offsets point within table bounds
	tableSize := len(b)
	if h.offsets.ScriptListOffset > 0 && int(h.offsets.ScriptListOffset) >= tableSize {
		return fmt.Errorf("layout ScriptList offset out of bounds: %d >= %d",
			h.offsets.ScriptListOffset, tableSize)
	}
	if h.offsets.FeatureListOffset > 0 && int(h.offsets.FeatureListOffset) >= tableSize {
		return fmt.Errorf("layout FeatureList offset out of bounds: %d >= %d",
			h.offsets.FeatureListOffset, tableSize)
	}
	if h.offsets.LookupListOffset > 0 && int(h.offsets.LookupListOffset) >= tableSize {
		return fmt.Errorf("layout LookupList offset out of bounds: %d >= %d",
			h.offsets.LookupListOffset, tableSize)
	}
	if h.Minor >= 1 && h.offsets.FeatureVariationsOffset > 0 &&
		int(h.offsets.FeatureVariationsOffset) >= tableSize {
		return fmt.Errorf("layout FeatureVariations offset out of bounds: %d >= %d",
			h.offsets.FeatureVariationsOffset, tableSize)
	}

	lytt.header = h
	return nil
}

// --- Script list -----------------------------------------------------------

// A ScriptList table consists of a count of the scripts represented by the glyphs in the
// font (ScriptCount) and an array of records (ScriptRecord), one for each script for which
// the font defines script-specific features (a script without script-specific features
// does not need a ScriptRecord). Each ScriptRecord consists of a ScriptTag that identifies
// a script, and an offset to a Script table. The ScriptRecord array is stored in
// alphabetic order of the script tags.
func parseScriptList(lytt *LayoutTable, b binarySegm, err error) error {
	if err != nil {
		return err
	}
	//lytt.ScriptList = tagRecordMap16{}
	link := link16{base: b, offset: uint16(lytt.header.offsetFor(layoutScriptSection))}
	scripts := link.Jump() // now we stand at the ScriptList table
	//scriptRecords := parseTagRecordMap16(scripts.Bytes(), 0, scripts.Bytes(), "ScriptList", "Script")
	//lytt.ScriptList = scriptRecords
	lytt.ScriptList = NavigatorFactory("ScriptList", scripts, scripts)
	return nil
}

// --- Feature list ----------------------------------------------------------

// The FeatureList table enumerates features in an array of records (FeatureRecord) and
// specifies the total number of features (FeatureCount). Every feature must have a
// FeatureRecord, which consists of a FeatureTag that identifies the feature and an offset
// to a Feature table (described next). The FeatureRecord array is arranged alphabetically
// by FeatureTag names.
func parseFeatureList(lytt *LayoutTable, b []byte, err error) error {
	if err != nil {
		return err
	}
	//lytt.FeatureList = array{}
	lytt.FeatureList = tagRecordMap16{}
	link := link16{base: b, offset: uint16(lytt.header.offsetFor(layoutFeatureSection))}
	features := link.Jump() // now we stand at the FeatureList table
	featureRecords := parseTagRecordMap16(features.Bytes(), 0, features.Bytes(), "FeatureList", "Feature")
	//featureRecords, err := parseArray(features.Bytes(), 0, 6, "FeatureList", "Feature")
	if err != nil {
		return err
	}
	lytt.FeatureList = featureRecords
	return nil
}

// b+offset has to be positioned at the start of the feature index list block, e.g.,
// the second uint16 of a LangSys table:
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#language-system-table
//
// uint16  requiredFeatureIndex               Index of a feature required for this language system
// uint16  featureIndexCount                  Number of feature index values for this language system
// uint16  featureIndices[featureIndexCount]  Array of indices into the FeatureList, in arbitrary order
func parseLangSys(b binarySegm, offset int, target string) (langSys, error) {
	lsys := langSys{}
	if len(b) < offset+4 {
		return lsys, errBufferBounds
	}
	tracer().Debugf("parsing LangSys (%s)", target)
	b = b[offset:]
	lsys.mandatory, _ = b.u16(0)
	features, err := parseArray16(b, 2, "LangSys", target)
	if err != nil {
		return lsys, err
	}
	lsys.featureIndices = features
	tracer().Debugf("LangSys points to %d features", features.length)
	return lsys, nil
}

// --- Layout table lookup list ----------------------------------------------

// parseLookupList parses the LookupList.
// See https://www.microsoft.com/typography/otspec/chapter2.htm#lulTbl
func parseLookupList(lytt *LayoutTable, b binarySegm, err error, isGPos bool, tableTag Tag, ec *errorCollector) error {
	if err != nil {
		return err
	}
	lloffset := lytt.header.offsetFor(layoutLookupSection)
	if lloffset >= len(b) {
		return io.ErrUnexpectedEOF
	}
	b = b[lloffset:]

	// Validate lookup count before parsing
	if len(b) < 2 {
		ec.addError(tableTag, "LookupList", "header too small", SeverityCritical, 0)
		return errFontFormat("lookup list header too small")
	}
	count, _ := b.u16(0)
	if int(count) > MaxLookupCount {
		ec.addError(tableTag, "LookupList", fmt.Sprintf("count %d exceeds maximum %d", count, MaxLookupCount), SeverityCritical, 0)
		return fmt.Errorf("lookup list count %d exceeds maximum %d", count, MaxLookupCount)
	}

	const layoutListName = "LookupList"
	ll := LookupList{name: layoutListName, base: b, isGPos: isGPos}
	ll.array, ll.err = parseArray16(b, 0, "Lookup", "LookupSubtables")
	if ll.err != nil {
		return ll.err
	}
	lytt.LookupList = ll

	// Collect GDEF requirements from lookup flags during the first parse pass.
	for i := 0; i < ll.array.Len(); i++ {
		off := int(ll.array.Get(i).U16(0))
		if off == 0 {
			continue
		}
		if off+4 > len(b) {
			ec.addError(tableTag, "LookupList", fmt.Sprintf("lookup offset %d out of bounds (size %d)", off, len(b)), SeverityCritical, 0)
			return errFontFormat("lookup offset out of bounds")
		}
		flag := LayoutTableLookupFlag(b.U16(off + 2))
		lytt.Requirements.AddFromLookupFlag(flag)
	}

	return nil
}

func parseLookupSubtable(b binarySegm, lookupType LayoutTableLookupType) LookupSubtable {
	return parseLookupSubtableWithDepth(b, lookupType, 0)
}

func parseLookupSubtableWithDepth(b binarySegm, lookupType LayoutTableLookupType, depth int) LookupSubtable {
	tracer().Debugf("parse lookup subtable b = %v", asU16Slice(b[:20]))
	if len(b) < 4 {
		return LookupSubtable{}
	}
	if depth > MaxExtensionDepth {
		tracer().Errorf("lookup subtable exceeds maximum extension depth %d", MaxExtensionDepth)
		return LookupSubtable{}
	}
	if IsGPosLookupType(lookupType) {
		return parseGPosLookupSubtableWithDepth(b, GPosLookupType(lookupType), depth)
	}
	return parseGSubLookupSubtableWithDepth(b, GSubLookupType(lookupType), depth)
}

// --- parse class def table -------------------------------------------------

// The ClassDef table can have either of two formats: one that assigns a range of
// consecutive glyph indices to different classes, or one that puts groups of consecutive
// glyph indices into the same class.
func parseClassDefinitions(b binarySegm) (ClassDefinitions, error) {
	tracer().Debugf("HELLO, parsing a ClassDef")
	if len(b) < 4 {
		return ClassDefinitions{}, errFontFormat("ClassDef table too small")
	}

	cdef := ClassDefinitions{}
	r := bytes.NewReader(b)
	if err := binary.Read(r, binary.BigEndian, &cdef.format); err != nil {
		return cdef, err
	}

	var n, g uint16
	if cdef.format == 1 {
		tracer().Debugf("parsing a ClassDef of format 1")
		if len(b) < 6 {
			return cdef, errFontFormat("ClassDef format 1 header incomplete")
		}
		n, _ = b.u16(4) // number of glyph IDs in table
		g, _ = b.u16(2) // start glyph ID

		// Validate array bounds: each entry is 2 bytes (uint16 class value)
		if len(b) < 6+int(n)*2 {
			return cdef, fmt.Errorf("ClassDef format 1 array extends beyond bounds: need %d bytes, have %d",
				6+int(n)*2, len(b))
		}
	} else if cdef.format == 2 {
		tracer().Debugf("parsing a ClassDef of format 2")
		if len(b) < 4 {
			return cdef, errFontFormat("ClassDef format 2 header incomplete")
		}
		n, _ = b.u16(2) // number of glyph ID ranges in table
		// Validate array bounds: each range record is 6 bytes (start, end, class)
		if len(b) < 4+int(n)*6 {
			return cdef, fmt.Errorf("ClassDef format 2 array extends beyond bounds: need %d bytes, have %d",
				4+int(n)*6, len(b))
		}
	} else {
		return cdef, errFontFormat(fmt.Sprintf("unknown ClassDef format %d", cdef.format))
	}
	records := cdef.makeArray(b, int(n), cdef.format)
	cdef.setRecords(records, GlyphIndex(g))
	return cdef, nil
}

// --- parse coverage table-module -------------------------------------------

// Read a coverage table-module, which comes in two formats (1 and 2).
// A Coverage table defines a unique index value, the Coverage Index, for each
// covered glyph.
func parseCoverage(b binarySegm) Coverage {
	tracer().Debugf("parsing Coverage")
	h := coverageHeader{}
	h.CoverageFormat = b.U16(0)
	h.Count = b.U16(2)
	tracer().Debugf("coverage header format %d has count = %d ", h.CoverageFormat, h.Count)

	// Validate based on format
	if h.CoverageFormat == 1 {
		// Format 1: array of glyph IDs (2 bytes each)
		requiredSize := 4 + int(h.Count)*2
		if len(b) < requiredSize {
			tracer().Errorf("coverage format 1 extends beyond bounds: need %d, have %d",
				requiredSize, len(b))
			return Coverage{}
		}
	} else if h.CoverageFormat == 2 {
		// Format 2: array of range records (6 bytes each: start, end, startCoverageIndex)
		requiredSize := 4 + int(h.Count)*6
		if len(b) < requiredSize {
			tracer().Errorf("coverage format 2 extends beyond bounds: need %d, have %d",
				requiredSize, len(b))
			return Coverage{}
		}
	} else {
		tracer().Errorf("unknown coverage format %d", h.CoverageFormat)
		return Coverage{}
	}

	return Coverage{
		coverageHeader: h,
		GlyphRange:     buildGlyphRangeFromCoverage(h, b),
	}
}

// --- Sequence context ------------------------------------------------------

// The contextual lookup types support specifying input glyph sequences that can be
// acted upon, as well as a list of actions to be taken on any glyph within the sequence.
// Actions are specified as references to separate nested lookups (an index into the
// LookupList). The actions are specified for each glyph position, but the entire sequence
// must be matched, and so the actions are specified in a context-sensitive manner.

// Three subtable formats are defined, which describe the input sequences in different ways.
func parseSequenceContext(b binarySegm, sub LookupSubtable) (LookupSubtable, error) {
	if len(b) <= 2 {
		return sub, errFontFormat("corrupt sequence context")
	}
	//format := b.U16(0)
	switch sub.Format {
	case 1:
		return parseSequenceContextFormat1(b, sub)
	case 2:
		return parseSequenceContextFormat2(b, sub)
	case 3:
		return parseSequenceContextFormat3(b, sub)
	}
	return sub, errFontFormat(fmt.Sprintf("unknown sequence context format %d", sub.Format))
}

// SequenceContextFormat1: simple glyph contexts
// Type 	Name 	Description
// uint16 	format 	Format identifier: format = 1
// Offset16 	coverageOffset 	Offset to Coverage table, from beginning of SequenceContextFormat1 table
// uint16 	seqRuleSetCount 	Number of SequenceRuleSet tables
// Offset16 	seqRuleSetOffsets[seqRuleSetCount] 	Array of offsets to SequenceRuleSet tables, from beginning of SequenceContextFormat1 table (offsets may be NULL)
func parseSequenceContextFormat1(b binarySegm, sub LookupSubtable) (LookupSubtable, error) {
	if len(b) <= 6 {
		return sub, errFontFormat("corrupt sequence context")
	}
	// nothing to to for format 1
	//
	// seqctx := SequenceContext{}
	// link, err := parseLink16(b, 2, b, "SequenceContext Coverage")
	// if err != nil {
	// 	return sequenceContext{}, errFontFormat("corrupt sequence context")
	// }
	// cov := link.Jump()
	// seqctx.coverage[0] = parseCoverage(cov.Bytes())
	// seqctx.rules = parseVarArrary16(b, 4, 2, "SequenceContext")
	// return seqctx, nil
	return sub, nil
}

// SequenceContextFormat2 table:
// Type      Name                   Description
// uint16    format                 Format identifier: format = 2
// Offset16  coverageOffset         Offset to Coverage table, from beginning of SequenceContextFormat2 table
// Offset16  classDefOffset         Offset to ClassDef table, from beginning of SequenceContextFormat2 table
// uint16    classSeqRuleSetCount   Number of ClassSequenceRuleSet tables
// Offset16  classSeqRuleSetOffsets[classSeqRuleSetCount]    Array of offsets to ClassSequenceRuleSet tables, from beginning of SequenceContextFormat2 table (may be NULL)
func parseSequenceContextFormat2(b binarySegm, sub LookupSubtable) (LookupSubtable, error) {
	if len(b) <= 8 {
		return sub, errFontFormat("corrupt sequence context")
	}
	seqctx := &SequenceContext{}
	sub.Support = seqctx
	seqctx.ClassDefs = make([]ClassDefinitions, 1)
	var err error
	seqctx.ClassDefs[0], err = parseContextClassDef(b, 4)
	sub.Support = seqctx
	return sub, err
}

// The SequenceContextFormat3 table specifies exactly one input sequence pattern. It has an
// array of offsets to coverage tables. These correspond, in order, to the positions in the
// input sequence pattern.
//
// SequenceContextFormat3 table:
// Type 	Name 	Description
// uint16 	format 	Format identifier: format = 3
// uint16 	glyphCount 	Number of glyphs in the input sequence
// uint16 	seqLookupCount 	Number of SequenceLookupRecords
// Offset16 	coverageOffsets[glyphCount] 	Array of offsets to Coverage tables, from beginning of SequenceContextFormat3 subtable
// SequenceLookupRecord 	seqLookupRecords[seqLookupCount] 	Array of SequenceLookupRecords
func parseSequenceContextFormat3(b binarySegm, sub LookupSubtable) (LookupSubtable, error) {
	if len(b) <= 8 {
		return sub, errFontFormat("corrupt sequence context")
	}
	glyphCount := int(b.U16(2))
	seqctx := SequenceContext{}
	sub.Support = seqctx
	seqctx.InputCoverage = make([]Coverage, glyphCount)
	for i := 0; i < glyphCount; i++ {
		link, err := parseLink16(b, 6+i*2, b, "SequenceContext Coverage")
		if err != nil {
			return sub, errFontFormat("corrupt sequence context")
		}
		cov := link.Jump()
		seqctx.InputCoverage[i] = parseCoverage(cov.Bytes())
	}
	return sub, nil
}

func parseChainedSequenceContext(b binarySegm, sub LookupSubtable) (LookupSubtable, error) {
	if len(b) <= 2 {
		return sub, errFontFormat("corrupt chained sequence context")
	}
	switch sub.Format {
	case 1:
		//parseSequenceContextFormat1(sub.Format, b, sub)
		// nothing to to for format 1
		return sub, nil
	case 2:
		return parseChainedSequenceContextFormat2(b, sub)
	case 3:
		return parseChainedSequenceContextFormat3(b, sub)
	}
	return sub, errFontFormat(fmt.Sprintf("unknown chained sequence context format %d", sub.Format))
}

func parseChainedSequenceContextFormat2(b binarySegm, sub LookupSubtable) (LookupSubtable, error) {
	backtrack, err1 := parseContextClassDef(b, 4)
	input, err2 := parseContextClassDef(b, 6)
	lookahead, err3 := parseContextClassDef(b, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return LookupSubtable{}, errFontFormat("corrupt chained sequence context (format 2)")
	}
	sub.Support = &SequenceContext{
		ClassDefs: []ClassDefinitions{backtrack, input, lookahead},
	}
	return sub, nil
}

func parseChainedSequenceContextFormat3(b binarySegm, sub LookupSubtable) (LookupSubtable, error) {
	tracer().Debugf("chained sequence context format 3 ........................")
	tracer().Debugf("b = %v", b[:26].Glyphs())
	offset := 2
	backtrack, err1 := parseChainedSeqContextCoverages(b, offset, nil)
	offset += 2 + len(backtrack)*2
	input, err2 := parseChainedSeqContextCoverages(b, offset, err1)
	offset += 2 + len(input)*2
	lookahead, err3 := parseChainedSeqContextCoverages(b, offset, err2)
	if err1 != nil || err2 != nil || err3 != nil {
		return LookupSubtable{}, errFontFormat("corrupt chained sequence context (format 3)")
	}
	sub.Support = &SequenceContext{
		BacktrackCoverage: backtrack,
		InputCoverage:     input,
		LookaheadCoverage: lookahead,
	}
	return sub, nil
}

func parseContextClassDef(b binarySegm, at int) (ClassDefinitions, error) {
	link, err := parseLink16(b, at, b, "ClassDef")
	if err != nil {
		return ClassDefinitions{}, err
	}
	cdef, err := parseClassDefinitions(link.Jump().Bytes())
	if err != nil {
		return ClassDefinitions{}, err
	}
	return cdef, nil
}

func parseChainedSeqContextCoverages(b binarySegm, at int, err error) ([]Coverage, error) {
	if err != nil {
		return []Coverage{}, err
	}
	count := int(b.U16(at))
	coverages := make([]Coverage, count)
	tracer().Debugf("chained seq context with %d coverages", count)
	for i := 0; i < count; i++ {
		link, err := parseLink16(b, at+2+i*2, b, "ChainedSequenceContext Coverage")
		if err != nil {
			tracer().Errorf("error parsing coverages' offset")
			return []Coverage{}, err
		}
		coverages[i] = parseCoverage(link.Jump().Bytes())
	}
	return coverages, nil
}

// TODO Argument should be NavLocation, return value should be []SeqLookupRecord
//
// SequenceRule table:
// Type     Name                          Description
// uint16   glyphCount                    Number of glyphs to be matched
// uint16   seqLookupCount                Number of SequenceLookupRecords
// uint16   inputSequence[glyphCount-1]   Sequence of classes to be matched to the input glyph sequence, beginning with the second glyph position
// SequenceLookupRecord seqLookupRecords[seqLookupCount]   Array of SequenceLookupRecords
func (lksub LookupSubtable) SequenceRule(b binarySegm) sequenceRule {
	seqrule := sequenceRule{}
	seqrule.glyphCount = b.U16(0)
	seqrule.inputSequence = array{
		recordSize: 2, // sizeof(uint16)
		length:     int(seqrule.glyphCount) - 1,
	}

	// Check for overflow in input sequence size calculation
	inputSeqSize, err := checkedMulInt(seqrule.inputSequence.length, 2)
	if err != nil {
		tracer().Errorf("SequenceRule input sequence size overflow: %v", err)
		return sequenceRule{}
	}
	inputSeqEnd, err := checkedAddInt(4, inputSeqSize)
	if err != nil || inputSeqEnd > len(b) {
		tracer().Errorf("SequenceRule input sequence bounds check failed")
		return sequenceRule{}
	}
	seqrule.inputSequence.loc = b[4:inputSeqEnd]

	// SequenceLookupRecord:
	// Type     Name             Description
	// uint16   sequenceIndex    Index (zero-based) into the input glyph sequence
	// uint16   lookupListIndex  Index (zero-based) into the LookupList
	cnt := b.U16(2)
	seqrule.lookupRecords = array{
		recordSize: 4, // 2* sizeof(uint16)
		length:     int(cnt),
		loc:        b[inputSeqEnd:],
	}
	return seqrule
}
