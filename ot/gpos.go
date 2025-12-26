package ot

import "strconv"

// GPosTable is a type representing an OpenType GPOS table
// (see https://docs.microsoft.com/en-us/typography/opentype/spec/gsub).
type GPosTable struct {
	tableBase
	LayoutTable
}

func newGPosTable(tag Tag, b binarySegm, offset, size uint32) *GPosTable {
	t := &GPosTable{}
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

var _ Table = &GPosTable{}

// GPOS Table
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#table-organization

// GPOS Lookup Type Enumeration
const (
	GPosLookupTypeSingle            LayoutTableLookupType = 1 // Adjust position of a single glyph
	GPosLookupTypePair              LayoutTableLookupType = 2 // Adjust position of a pair of glyphs
	GPosLookupTypeCursive           LayoutTableLookupType = 3 // Attach cursive glyphs
	GPosLookupTypeMarkToBase        LayoutTableLookupType = 4 // Attach a combining mark to a base glyph
	GPosLookupTypeMarkToLigature    LayoutTableLookupType = 5 // Attach a combining mark to a ligature
	GPosLookupTypeMarkToMark        LayoutTableLookupType = 6 // Attach a combining mark to another mark
	GPosLookupTypeContextPos        LayoutTableLookupType = 7 // Position one or more glyphs in context
	GPosLookupTypeChainedContextPos LayoutTableLookupType = 8 // Position one or more glyphs in chained context
	GPosLookupTypeExtensionPos      LayoutTableLookupType = 9 // Extension mechanism for other positionings
)

const gposLookupTypeNames = "Single|Pair|Cursive|MarkToBase|MarkToLigature|MarkToMark|ContextPos|Chained|Ext"

var gposLookupTypeInx = [...]int{0, 7, 12, 20, 31, 46, 57, 68, 76, 80}

// GPosString interprets a layout table lookup type as a GPOS table type.
func (lt LayoutTableLookupType) GPosString() string {
	lt -= 1
	if lt < GPosLookupTypeExtensionPos {
		return gposLookupTypeNames[gposLookupTypeInx[lt] : gposLookupTypeInx[lt+1]-1]
	}
	return strconv.Itoa(int(lt))
}

// ValueFormat is a bitmask that describes which fields are present in a ValueRecord.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#value-record
type ValueFormat uint16

const (
	ValueFormatXPlacement ValueFormat = 0x0001 // Includes horizontal adjustment for placement
	ValueFormatYPlacement ValueFormat = 0x0002 // Includes vertical adjustment for placement
	ValueFormatXAdvance   ValueFormat = 0x0004 // Includes horizontal adjustment for advance
	ValueFormatYAdvance   ValueFormat = 0x0008 // Includes vertical adjustment for advance
	ValueFormatXPlaDevice ValueFormat = 0x0010 // Includes Device table for horizontal placement
	ValueFormatYPlaDevice ValueFormat = 0x0020 // Includes Device table for vertical placement
	ValueFormatXAdvDevice ValueFormat = 0x0040 // Includes Device table for horizontal advance
	ValueFormatYAdvDevice ValueFormat = 0x0080 // Includes Device table for vertical advance
	// Bits 0x0F00 are reserved for future use
)

// ValueRecord represents a positioning adjustment for a glyph.
// The actual fields present depend on the ValueFormat bitmask.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#value-record
type ValueRecord struct {
	XPlacement int16  // Horizontal adjustment for placement, in design units
	YPlacement int16  // Vertical adjustment for placement, in design units
	XAdvance   int16  // Horizontal adjustment for advance, in design units
	YAdvance   int16  // Vertical adjustment for advance, in design units
	XPlaDevice uint16 // Offset to Device table for horizontal placement (may be NULL)
	YPlaDevice uint16 // Offset to Device table for vertical placement (may be NULL)
	XAdvDevice uint16 // Offset to Device table for horizontal advance (may be NULL)
	YAdvDevice uint16 // Offset to Device table for vertical advance (may be NULL)
}

// AnchorFormat represents the format of an Anchor table.
type AnchorFormat uint16

const (
	AnchorFormat1 AnchorFormat = 1 // Design units only
	AnchorFormat2 AnchorFormat = 2 // Design units plus contour point
	AnchorFormat3 AnchorFormat = 3 // Design units plus Device tables
)

// Anchor represents an attachment point on a glyph.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#anchor-tables
type Anchor struct {
	Format        AnchorFormat // Format identifier
	XCoordinate   int16        // Horizontal value, in design units
	YCoordinate   int16        // Vertical value, in design units
	AnchorPoint   uint16       // Index to glyph contour point (Format 2 only)
	XDeviceOffset uint16       // Offset to Device table for X coordinate (Format 3 only)
	YDeviceOffset uint16       // Offset to Device table for Y coordinate (Format 3 only)
}

// PairValueRecord represents a kerning pair with positioning adjustments.
// Used in GPOS Lookup Type 2 (Pair Adjustment).
type PairValueRecord struct {
	SecondGlyph uint16      // Glyph ID of second glyph in pair
	Value1      ValueRecord // Positioning for first glyph
	Value2      ValueRecord // Positioning for second glyph
}

// MarkRecord associates a mark glyph with a class and anchor point.
// Used in GPOS Lookup Types 4, 5, and 6 (Mark attachment).
type MarkRecord struct {
	Class      uint16 // Class value for this mark
	MarkAnchor uint16 // Offset to Anchor table for this mark
}

// MarkArray contains an array of MarkRecords.
type MarkArray struct {
	MarkCount   uint16
	MarkRecords []MarkRecord
}

// BaseRecord contains attachment points for a base glyph.
// Used in GPOS Lookup Type 4 (MarkToBase).
type BaseRecord struct {
	BaseAnchors []uint16 // Array of offsets to Anchor tables, one per mark class
}

// LigatureAttach contains attachment points for each component of a ligature.
// Used in GPOS Lookup Type 5 (MarkToLigature).
type LigatureAttach struct {
	ComponentCount   uint16
	ComponentAnchors [][]uint16 // 2D array: [component][mark class] -> Anchor offset
}
