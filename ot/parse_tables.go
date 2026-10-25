package ot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Table-level decoding and shared layout parsing extracted from parse.go.
// Font orchestration, cross-table assembly, and parsing safeguards remain there.
// Lookup-specific parsers remain in the existing parse_lookup* files.

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
	base := newBaseTable(tag, b, offset, size)
	if len(b) < 8 {
		ec.addError(tag, "Header", fmt.Sprintf("BASE table too small: %d bytes (need at least 8)", len(b)), SeverityCritical, offset)
		return nil, errFontFormat("BASE table header too small")
	}

	base.Major = b.U16(0)
	base.Minor = b.U16(2)
	xOff := b.U16(4)
	yOff := b.U16(6)

	if base.Major != 1 {
		ec.addError(tag, "Version", fmt.Sprintf("unsupported BASE major version %d", base.Major), SeverityMajor, offset)
		base.err = fmt.Errorf("unsupported BASE major version %d", base.Major)
	}
	if base.Minor >= 1 {
		if len(b) < 12 {
			ec.addError(tag, "Header", "BASE v1.1+ header incomplete", SeverityMajor, offset)
			if base.err == nil {
				base.err = errFontFormat("BASE v1.1+ header incomplete")
			}
		} else {
			base.itemVarStoreOffset = b.U32(8)
		}
	}

	if xOff != 0 {
		axis, ok := viewBaseAxis(b, xOff)
		base.horizontal = axis
		if !ok {
			ec.addWarning(tag, fmt.Sprintf("horizontal axis offset %d cannot be parsed", xOff), offset+uint32(xOff))
			if base.err == nil && axis != nil {
				base.err = axis.err
			}
		}
	}
	if yOff != 0 {
		axis, ok := viewBaseAxis(b, yOff)
		base.vertical = axis
		if !ok {
			ec.addWarning(tag, fmt.Sprintf("vertical axis offset %d cannot be parsed", yOff), offset+uint32(yOff))
			if base.err == nil && axis != nil {
				base.err = axis.err
			}
		}
	}
	return base, nil
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
		subtable := link.jump()
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
	t.GlyphIndexMap, err = makeGlyphIndex(enc, tag, offset, ec)
	if err != nil {
		return nil, err
	}
	return t, nil
}

type encodingRecord struct {
	platformId uint16
	encodingId uint16
	link       navLink
	format     uint16
	size       int
	width      int // encoding width in bytes
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
	// CFF OpenType fonts legitimately use maxp version 0.5 with exactly 6 bytes.
	if size < 6 {
		ec.addError(tag, "Size", fmt.Sprintf("maxp table too small: %d bytes (need at least 6)", size), SeverityCritical, offset)
		return nil, errFontFormat("maxp table incomplete")
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
	a, _ := b.u16(4)
	d, _ := b.u16(6)
	lg, _ := b.u16(8)
	aw, _ := b.u16(10)
	minLSB, _ := b.u16(12)
	minRSB, _ := b.u16(14)
	xmax, _ := b.u16(16)
	slopeRise, _ := b.u16(18)
	slopeRun, _ := b.u16(20)
	caretOff, _ := b.u16(22)
	n, _ := b.u16(34)
	t.Ascender = int16(a)
	t.Descender = int16(d)
	t.LineGap = int16(lg)
	t.AdvanceWidthMax = aw
	t.MinLeftSideBearing = int16(minLSB)
	t.MinRightSideBearing = int16(minRSB)
	t.XMaxExtent = int16(xmax)
	t.CaretSlopeRise = int16(slopeRise)
	t.CaretSlopeRun = int16(slopeRun)
	t.CaretOffset = int16(caretOff)
	t.NumberOfHMetrics = int(n)
	return t, nil
}

// --- OS/2 table ------------------------------------------------------------

// parseOS2 parses the OS/2 table subset required for metrics fallback.
// The parser is intentionally tolerant: if optional fields are truncated, zero values
// are kept and warnings are recorded.
func parseOS2(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	if size == 0 {
		return nil, nil
	}
	t := newOS2Table(tag, b, offset, size)
	if size < 2 {
		ec.addWarning(tag, "OS/2 table too small to decode version", offset)
		return t, nil
	}
	v, _ := b.u16(0)
	t.Version = v
	if size >= 4 {
		xavg, _ := b.u16(2)
		t.XAvgCharWidth = int16(xavg)
	}
	// OpenType OS/2 v0 and above include sTypoAscender..usWinDescent at offsets 68..76.
	if size >= 78 {
		typoAsc, _ := b.u16(68)
		typoDesc, _ := b.u16(70)
		typoGap, _ := b.u16(72)
		winAsc, _ := b.u16(74)
		winDesc, _ := b.u16(76)
		t.TypoAscender = int16(typoAsc)
		t.TypoDescender = int16(typoDesc)
		t.TypoLineGap = int16(typoGap)
		t.WinAscent = winAsc
		t.WinDescent = winDesc
	} else {
		ec.addWarning(tag, "OS/2 table truncated before typo/win metrics fields", offset)
	}
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
	h.Major, _ = b.u16(0)
	h.Minor, _ = b.u16(2)
	h.GlyphClassDefOffset, _ = b.u16(4)
	h.AttachListOffset, _ = b.u16(6)
	h.LigCaretListOffset, _ = b.u16(8)
	h.MarkAttachClassDefOffset, _ = b.u16(10)
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
	if offset == 0 {
		return nil
	}
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
	if offset == 0 {
		return nil
	}
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
	if offset == 0 {
		return nil
	}
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
	if offset == 0 {
		return nil
	}
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

// --- Shared GSUB/GPOS header -----------------------------------------------

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
	link := link16{base: b, offset: uint16(lytt.header.offsetFor(layoutScriptSection))}
	scripts := link.jump() // now we stand at the ScriptList table
	scriptRecords, perr := parseArray(scripts.Bytes(), 0, 6, "ScriptList", "Script")
	if perr != nil {
		lytt.scriptGraph = &ScriptList{
			featureGraph: lytt.featureGraph,
			raw:          binarySegm(scripts.Bytes()),
			err:          perr,
		}
		return nil
	}
	lytt.scriptGraph = parseConcreteScriptList(scripts.Bytes(), scriptRecords, lytt.featureGraph)
	return nil
}

func parseConcreteScriptList(scripts binarySegm, scriptRecords array, featureGraph *FeatureList) *ScriptList {
	sl := &ScriptList{
		scriptOrder:  make([]Tag, 0, scriptRecords.Len()),
		offsetByTag:  make(map[Tag]uint16, scriptRecords.Len()),
		scriptByTag:  make(map[Tag]*Script, 8),
		featureGraph: featureGraph,
		raw:          scripts,
	}
	for i := 0; i < scriptRecords.Len(); i++ {
		record, readErr := scriptRecords.Get(i)
		if readErr != nil {
			if sl.err == nil {
				sl.err = fmt.Errorf("parseConcreteScriptList: %w", readErr)
			}
			sl.scriptOrder = append(sl.scriptOrder, 0)
			continue
		}
		if record.Size() < 6 {
			if sl.err == nil {
				sl.err = errBufferBounds
			}
			sl.scriptOrder = append(sl.scriptOrder, 0)
			continue
		}
		tag := MakeTag(record.Bytes()[:4])
		offset := record.U16(4)
		sl.scriptOrder = append(sl.scriptOrder, tag)
		sl.offsetByTag[tag] = offset
		if offset == 0 || int(offset) >= len(scripts) {
			perr := fmt.Errorf("script record %s has invalid offset %d (size %d)", tag, offset, len(scripts))
			if sl.err == nil {
				sl.err = perr
			}
			continue
		}
		if verr := validateConcreteScript(scripts[offset:], featureGraph, tag == DFLT); verr != nil && sl.err == nil {
			sl.err = verr
		}
	}
	return sl
}

func parseConcreteScript(b binarySegm, featureGraph *FeatureList) *Script {
	s := &Script{
		raw:              b,
		featureGraph:     featureGraph,
		langOffsetsByTag: make(map[Tag]uint16),
		langByTag:        make(map[Tag]*LangSys, 8),
	}
	if len(b) < 4 {
		s.err = errBufferBounds
		return s
	}
	s.defaultLangSysOffset, _ = b.u16(0)
	if s.defaultLangSysOffset > 0 && int(s.defaultLangSysOffset) >= len(b) {
		s.err = fmt.Errorf("default langsys offset out of bounds: %d (size %d)", s.defaultLangSysOffset, len(b))
	}
	langRecords, err := parseArray(b, 2, 6, "Script", "LangSys")
	if err != nil {
		if s.err == nil {
			s.err = err
		}
		return s
	}
	s.langOrder = make([]Tag, 0, langRecords.Len())
	for i := 0; i < langRecords.Len(); i++ {
		record, readErr := langRecords.Get(i)
		if readErr != nil {
			if s.err == nil {
				s.err = fmt.Errorf("parseConcreteScript: %w", readErr)
			}
			continue
		}
		if record.Size() < 6 {
			if s.err == nil {
				s.err = errBufferBounds
			}
			continue
		}
		tag := MakeTag(record.Bytes()[:4])
		offset := record.U16(4)
		s.langOrder = append(s.langOrder, tag)
		s.langOffsetsByTag[tag] = offset
		if offset == 0 || int(offset) >= len(b) {
			perr := fmt.Errorf("langsys record %s has invalid offset %d (size %d)", tag, offset, len(b))
			if s.err == nil {
				s.err = perr
			}
			continue
		}
	}
	return s
}

func parseConcreteLangSys(b binarySegm, featureGraph *FeatureList) *LangSys {
	ls := &LangSys{featureGraph: featureGraph}
	if len(b) < 6 {
		ls.err = errBufferBounds
		return ls
	}
	ls.lookupOrderOffset, _ = b.u16(0)
	ls.requiredFeatureIndex, _ = b.u16(2)
	featureIndices, err := parseArray16(b, 4, "LangSys", "Feature-Index")
	if err != nil {
		ls.err = err
		return ls
	}
	if featureIndices.Len() == 0 {
		return ls
	}
	ls.featureIndices = make([]uint16, featureIndices.Len())
	for i := 0; i < featureIndices.Len(); i++ {
		item, readErr := featureIndices.Get(i)
		if readErr != nil {
			if ls.err == nil {
				ls.err = fmt.Errorf("parseConcreteLangSys: %w", readErr)
			}
			ls.featureIndices = ls.featureIndices[:i]
			return ls
		}
		index := item.U16(0)
		ls.featureIndices[i] = index
		if featureGraph != nil && int(index) >= featureGraph.Len() && ls.err == nil {
			ls.err = fmt.Errorf("feature index %d out of bounds (size %d)", index, featureGraph.Len())
		}
	}
	return ls
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
	link := link16{base: b, offset: uint16(lytt.header.offsetFor(layoutFeatureSection))}
	features := link.jump() // now we stand at the FeatureList table
	featureRecords, perr := parseArray(features.Bytes(), 0, 6, "FeatureList", "Feature")
	if perr != nil {
		lytt.featureGraph = &FeatureList{
			raw: binarySegm(features.Bytes()),
			err: perr,
		}
		return nil
	}
	lytt.featureGraph = parseConcreteFeatureList(features.Bytes(), featureRecords)
	return nil
}

func parseConcreteFeatureList(features binarySegm, featureRecords array) *FeatureList {
	fl := &FeatureList{
		featureOrder:          make([]Tag, 0, featureRecords.Len()),
		featureOffsetsByIndex: make([]uint16, 0, featureRecords.Len()),
		featuresByIndex:       make(map[int]*Feature, 8),
		indicesByTag:          make(map[Tag][]int, featureRecords.Len()),
		raw:                   features,
	}
	for i := 0; i < featureRecords.Len(); i++ {
		record, readErr := featureRecords.Get(i)
		if readErr != nil {
			if fl.err == nil {
				fl.err = fmt.Errorf("parseConcreteFeatureList: %w", readErr)
			}
			fl.featureOrder = append(fl.featureOrder, 0)
			fl.featureOffsetsByIndex = append(fl.featureOffsetsByIndex, 0)
			continue
		}
		if record.Size() < 6 {
			if fl.err == nil {
				fl.err = errBufferBounds
			}
			fl.featureOrder = append(fl.featureOrder, 0)
			fl.featureOffsetsByIndex = append(fl.featureOffsetsByIndex, 0)
			continue
		}
		tag := MakeTag(record.Bytes()[:4])
		offset := record.U16(4)
		fl.featureOrder = append(fl.featureOrder, tag)
		fl.featureOffsetsByIndex = append(fl.featureOffsetsByIndex, offset)
		fl.indicesByTag[tag] = append(fl.indicesByTag[tag], i)

		if offset == 0 || int(offset) >= len(features) {
			perr := fmt.Errorf("feature record %s has invalid offset %d (size %d)", tag, offset, len(features))
			if fl.err == nil {
				fl.err = perr
			}
			continue
		}
		if verr := validateConcreteFeature(features[offset:]); verr != nil && fl.err == nil {
			fl.err = verr
		}
	}
	return fl
}

func validateConcreteFeature(b binarySegm) error {
	if len(b) < 4 {
		return errBufferBounds
	}
	_, err := parseArray16(b, 2, "Feature", "Feature-Lookups")
	return err
}

func validateConcreteScript(b binarySegm, featureGraph *FeatureList, requireDefault bool) error {
	if len(b) < 4 {
		return errBufferBounds
	}
	defaultLangSysOffset, _ := b.u16(0)
	if requireDefault && defaultLangSysOffset == 0 {
		return fmt.Errorf("DFLT script requires defaultLangSysOffset")
	}
	if defaultLangSysOffset > 0 {
		if int(defaultLangSysOffset) >= len(b) {
			return fmt.Errorf("default langsys offset out of bounds: %d (size %d)", defaultLangSysOffset, len(b))
		}
		if err := validateConcreteLangSys(b[defaultLangSysOffset:], featureGraph); err != nil {
			return err
		}
	}
	langRecords, err := parseArray(b, 2, 6, "Script", "LangSys")
	if err != nil {
		return err
	}
	for i := 0; i < langRecords.Len(); i++ {
		record, readErr := langRecords.Get(i)
		if readErr != nil {
			return fmt.Errorf("validateConcreteScript: %w", readErr)
		}
		if record.Size() < 6 {
			return errBufferBounds
		}
		offset := record.U16(4)
		if offset == 0 || int(offset) >= len(b) {
			tag := MakeTag(record.Bytes()[:4])
			return fmt.Errorf("langsys record %s has invalid offset %d (size %d)", tag, offset, len(b))
		}
		if err := validateConcreteLangSys(b[offset:], featureGraph); err != nil {
			return err
		}
	}
	return nil
}

func validateConcreteLangSys(b binarySegm, featureGraph *FeatureList) error {
	if len(b) < 6 {
		return errBufferBounds
	}
	featureIndices, err := parseArray16(b, 4, "LangSys", "Feature-Index")
	if err != nil {
		return err
	}
	if featureGraph == nil {
		return nil
	}
	featureCount := featureGraph.Len()
	for i := 0; i < featureIndices.Len(); i++ {
		item, readErr := featureIndices.Get(i)
		if readErr != nil {
			return fmt.Errorf("validateConcreteLangSys: %w", readErr)
		}
		index := item.U16(0)
		if int(index) >= featureCount {
			return fmt.Errorf("feature index %d out of bounds (size %d)", index, featureCount)
		}
	}
	return nil
}

func parseConcreteFeature(b binarySegm) *Feature {
	f := &Feature{raw: b}
	if len(b) < 4 {
		f.err = errBufferBounds
		return f
	}
	f.featureParamsOffset, _ = b.u16(0)
	lookupArray, err := parseArray16(b, 2, "Feature", "Feature-Lookups")
	if err != nil {
		f.err = err
		return f
	}
	if lookupArray.Len() == 0 {
		return f
	}
	f.lookupListIndices = make([]uint16, lookupArray.Len())
	for i := 0; i < lookupArray.Len(); i++ {
		item, readErr := lookupArray.Get(i)
		if readErr != nil {
			if f.err == nil {
				f.err = fmt.Errorf("parseConcreteFeature: %w", readErr)
			}
			f.lookupListIndices = f.lookupListIndices[:i]
			return f
		}
		f.lookupListIndices[i] = item.U16(0)
	}
	return f
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

	lookupOffsets, perr := parseArray16(b, 0, "Lookup", "LookupSubtables")
	if perr != nil {
		return perr
	}
	lytt.lookupGraph = parseConcreteLookupListGraph(b, isGPos)

	// Collect GDEF requirements from lookup flags during the first parse pass.
	for i := 0; i < lookupOffsets.Len(); i++ {
		item, readErr := lookupOffsets.Get(i)
		if readErr != nil {
			return fmt.Errorf("parseLookupList: %w", readErr)
		}
		off := int(item.U16(0))
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
	switch cdef.format {
	case 1:
		tracer().Debugf("parsing a ClassDef of format 1")
		if len(b) < 6 {
			return cdef, errFontFormat("ClassDef format 1 header incomplete")
		}
		n, _ = b.u16(4)      // number of glyph IDs in table
		g, _ = b.u16(2)      // start glyph ID
		const preludeLen = 6 // prelude length in ClassDef format 1
		const entrySz = 2    // size of each entry in bytes

		// Validate array bounds: each entry is 2 bytes (uint16 class value)
		if len(b) < preludeLen+int(n)*entrySz {
			return cdef, fmt.Errorf("ClassDef format 1 array extends beyond bounds: need %d bytes, have %d",
				preludeLen+int(n)*entrySz, len(b))
		}
	case 2:
		tracer().Debugf("parsing a ClassDef of format 2")
		const preludeLen = 4 // prelude length in ClassDef format 1
		if len(b) < preludeLen {
			return cdef, errFontFormat("ClassDef format 2 header incomplete")
		}
		n, _ = b.u16(2)     // number of glyph ID ranges in table
		const recordLen = 6 // size of each range record in bytes (start, end, class)
		// Validate array bounds
		if len(b) < preludeLen+int(n)*recordLen {
			return cdef, fmt.Errorf("ClassDef format 2 array extends beyond bounds: need %d bytes, have %d",
				preludeLen+int(n)*recordLen, len(b))
		}
	default:
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
	switch h.CoverageFormat {
	case 1: // Format 1: array of glyph IDs (2 bytes each)
		requiredSize := 4 + int(h.Count)*2
		if len(b) < requiredSize {
			tracer().Errorf("coverage format 1 extends beyond bounds: need %d, have %d",
				requiredSize, len(b))
			return Coverage{}
		}
	case 2: // Format 2: array of range records (6 bytes each: start, end, startCoverageIndex)
		requiredSize := 4 + int(h.Count)*6
		if len(b) < requiredSize {
			tracer().Errorf("coverage format 2 extends beyond bounds: need %d, have %d",
				requiredSize, len(b))
			return Coverage{}
		}
	default: // there are only two formats
		tracer().Errorf("unknown coverage format %d", h.CoverageFormat)
		return Coverage{}
	}

	return Coverage{
		coverageHeader: h,
		GlyphRange:     buildGlyphRangeFromCoverage(h, b),
	}
}

func parseContextClassDef(b binarySegm, at int) (ClassDefinitions, error) {
	link, err := parseLink16(b, at, b, "ClassDef")
	if err != nil {
		return ClassDefinitions{}, err
	}
	cdef, err := parseClassDefinitions(link.jump().Bytes())
	if err != nil {
		return ClassDefinitions{}, err
	}
	return cdef, nil
}

// func parseChainedSeqContextCoverages(b binarySegm, at int, err error) ([]Coverage, error) {
// 	if err != nil {
// 		return []Coverage{}, err
// 	}
// 	count := int(b.U16(at))
// 	coverages := make([]Coverage, count)
// 	tracer().Debugf("chained seq context with %d coverages", count)
// 	for i := 0; i < count; i++ {
// 		link, err := parseLink16(b, at+2+i*2, b, "ChainedSequenceContext Coverage")
// 		if err != nil {
// 			tracer().Errorf("error parsing coverages' offset")
// 			return []Coverage{}, err
// 		}
// 		coverages[i] = parseCoverage(link.jump().Bytes())
// 	}
// 	return coverages, nil
// }
