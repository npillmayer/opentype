package ot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// Code comment often will cite passage from the
// OpenType specification version 1.8.4;
// see https://docs.microsoft.com/en-us/typography/opentype/spec/.

// --- Parsing limits --------------------------------------------------------

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
	MaxExtensionDepth = 16 // Maximum Extension lookup nesting
)

// --- Checked arithmetic ----------------------------------------------------

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

// --- Format errors ---------------------------------------------------------

// errFontFormat produces user level errors for font parsing.
// This is a compatibility helper that returns a simple error.
// In the future, this will be replaced with proper error collection.
func errFontFormat(message string) error {
	return fmt.Errorf("OpenType font format: %s", message)
}

// --- Font parsing orchestration --------------------------------------------

// Parse parses an OpenType font from a byte slice.
// An ot.Font needs ongoing access to the fonts byte-data after the Parse function returns.
// Its elements are assumed immutable while the ot.Font remains in use.
func Parse(font []byte, options ...ParseOption) (*Font, error) {
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
	src := binarySegm(font)
	otf := &Font{raw: src, Header: &h, tables: make(map[Tag]Table)}
	configureWithOptions(otf, options)
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
	if hh := otf.tables[T("hhea")]; hh != nil {
		hhead := hh.Self().AsHHea()
		if mx := otf.tables[T("hmtx")]; mx != nil {
			hmtx := mx.Self().AsHMtx()
			hmtx.NumberOfHMetrics = hhead.NumberOfHMetrics
		}
	}
	if err := extractLayoutInfo(otf, ec); err != nil {
		return nil, err
	}
	// Collect and centralize font information:
	// The number of glyphs in the font is restricted only by the value stated in the 'head' table. The order in which glyphs are placed in a font is arbitrary.
	// Note that a font must have at least two glyphs, and that glyph index 0 musthave an outline. See Glyph Mappings for details.
	//
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

func configureWithOptions(otf *Font, options []ParseOption) {
	for _, option := range options {
		switch option {
		case IsTestfont:
			otf.parseOptions = append(otf.parseOptions, relaxCompleteness)
			otf.parseOptions = append(otf.parseOptions, relaxConsistency)
		}
	}
}

// --- Font assembly and consistency -----------------------------------------

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
			if !slices.Contains(otf.parseOptions, relaxCompleteness) {
				return errFontFormat("missing required table " + tag)
			}
		}
	}
	if otf.tables[T("cmap")] == nil { // cmap is always required, even in test-fonts
		ec.addError(T("cmap"), "Missing", "missing required table", SeverityCritical, 0)
		return errFontFormat("missing required table cmap")
	}
	otf.CMap = otf.tables[T("cmap")].Self().AsCMap()
	if hheaTable := otf.Table(T("hhea")); hheaTable != nil {
		otf.HHea = hheaTable.Self().AsHHea()
	}
	if hmtxTable := otf.Table(T("hmtx")); hmtxTable != nil {
		otf.HMtx = hmtxTable.Self().AsHMtx()
	}
	if os2Table := otf.Table(T("OS/2")); os2Table != nil {
		otf.OS2 = os2Table.Self().AsOS2()
	}

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
			if !slices.Contains(otf.parseOptions, relaxCompleteness) {
				return errFontFormat("missing advanced layout table " + tag)
			}
		}
	}
	// store shortcuts to layout tables
	if gsubTable := otf.tables[T("GSUB")]; gsubTable != nil {
		otf.Layout.GSub = gsubTable.Self().AsGSub()
	}
	if gposTable := otf.tables[T("GPOS")]; gposTable != nil {
		otf.Layout.GPos = gposTable.Self().AsGPos()
	}
	if gdefTable := otf.tables[T("GDEF")]; gdefTable != nil {
		otf.Layout.GDef = gdefTable.Self().AsGDef()
	}
	if baseTable := otf.tables[T("BASE")]; baseTable != nil {
		otf.Layout.Base = baseTable.Self().AsBase()
	}
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
	doCheck := !slices.Contains(otf.parseOptions, relaxConsistency)
	if doCheck && (req.NeedGlyphClassDef || req.NeedMarkAttachClassDef || req.NeedMarkGlyphSets) {
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
	if gsub := otf.Layout.GSub; gsub != nil {
		sg := gsub.ScriptGraph()
		fg := gsub.FeatureGraph()
		if sg == nil || fg == nil || sg.Len() == 0 || fg.Len() == 0 {
			ec.addError(T("GSUB"), "Structure", "GSUB table missing required lists", SeverityCritical, 0)
			return errFontFormat("GSUB table missing required lists")
		}
	}
	if gpos := otf.Layout.GPos; gpos != nil {
		sg := gpos.ScriptGraph()
		fg := gpos.FeatureGraph()
		if sg == nil || fg == nil || sg.Len() == 0 || fg.Len() == 0 {
			ec.addError(T("GPOS"), "Structure", "GPOS table missing required lists", SeverityCritical, 0)
			return errFontFormat("GPOS table missing required lists")
		}
	}

	// Perform cross-table consistency validation
	if err := validateCrossTableConsistency(otf, ec); err != nil {
		if !slices.Contains(otf.parseOptions, relaxConsistency) {
			return err
		}
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
		if err := hmtx.parseAll(numGlyphs, hhea.NumberOfHMetrics); err != nil {
			ec.addError(T("hmtx"), "Decode",
				fmt.Sprintf("cannot decode hmtx records: %v", err),
				SeverityCritical, 0)
			return errFontFormat(fmt.Sprintf("cannot decode hmtx records: %v", err))
		}
	}

	// Validate head.IndexToLocFormat consistency with loca table
	headTable := otf.Table(T("head"))
	locaTable := otf.Table(T("loca"))
	if headTable != nil && locaTable != nil {
		head := headTable.Self().AsHead()
		loca := locaTable.Self().AsLoca()

		// Calculate expected loca table size based on IndexToLocFormat
		switch head.IndexToLocFormat {
		case 0: // Short format: (numGlyphs + 1) * 2 bytes
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
		case 1: // Long format: (numGlyphs + 1) * 4 bytes
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
		default:
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

// --- Table dispatch --------------------------------------------------------

func parseTable(t Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	switch t {
	case T("BASE"):
		return parseBase(t, b, offset, size, ec)
	case T("cmap"):
		return parseCMap(t, b, offset, size, ec)
	case T("head"):
		return parseHead(t, b, offset, size, ec)
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
	case T("loca"):
		return parseLoca(t, b, offset, size, ec)
	case T("maxp"):
		return parseMaxP(t, b, offset, size, ec)
	case T("OS/2"):
		return parseOS2(t, b, offset, size, ec)
	}
	tracer().Infof("font contains table (%s), will not be interpreted", t)
	// Record as minor warning - not parsed but not a problem
	ec.addWarning(t, "table not interpreted", offset)
	return newTable(t, b, offset, size), nil
}
