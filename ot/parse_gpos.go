package ot

// parseGPos parses the GPOS (Glyph Positioning) table.
func parseGPos(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	var err error
	gpos := newGPosTable(tag, b, offset, size)
	err = parseLayoutHeader(&gpos.LayoutTable, b, err, tag, ec)
	err = parseLookupList(&gpos.LayoutTable, b, err, true, tag, ec) // true = GPOS
	err = parseFeatureList(&gpos.LayoutTable, b, err)
	err = parseScriptList(&gpos.LayoutTable, b, err)
	if err != nil {
		tracer().Errorf("error parsing GPOS table: %v", err)
		return gpos, err
	}
	mj, mn := gpos.header.Version()
	tracer().Debugf("GPOS table has version %d.%d", mj, mn)
	tracer().Debugf("GPOS table has %d lookup list entries", gpos.LookupList.length)
	return gpos, err
}

// parseValueRecord reads a ValueRecord from binary data based on the ValueFormat bitmask.
// Returns the parsed ValueRecord and the number of bytes consumed.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#value-record
func parseValueRecord(b binarySegm, offset int, format ValueFormat) (ValueRecord, int) {
	vr := ValueRecord{}
	pos := offset

	if format&ValueFormatXPlacement != 0 {
		vr.XPlacement = int16(b.U16(pos))
		pos += 2
	}
	if format&ValueFormatYPlacement != 0 {
		vr.YPlacement = int16(b.U16(pos))
		pos += 2
	}
	if format&ValueFormatXAdvance != 0 {
		vr.XAdvance = int16(b.U16(pos))
		pos += 2
	}
	if format&ValueFormatYAdvance != 0 {
		vr.YAdvance = int16(b.U16(pos))
		pos += 2
	}
	if format&ValueFormatXPlaDevice != 0 {
		vr.XPlaDevice = b.U16(pos)
		pos += 2
	}
	if format&ValueFormatYPlaDevice != 0 {
		vr.YPlaDevice = b.U16(pos)
		pos += 2
	}
	if format&ValueFormatXAdvDevice != 0 {
		vr.XAdvDevice = b.U16(pos)
		pos += 2
	}
	if format&ValueFormatYAdvDevice != 0 {
		vr.YAdvDevice = b.U16(pos)
		pos += 2
	}

	return vr, pos - offset
}

// valueRecordSize returns the size in bytes of a ValueRecord based on its format.
func valueRecordSize(format ValueFormat) int {
	size := 0
	if format&ValueFormatXPlacement != 0 {
		size += 2
	}
	if format&ValueFormatYPlacement != 0 {
		size += 2
	}
	if format&ValueFormatXAdvance != 0 {
		size += 2
	}
	if format&ValueFormatYAdvance != 0 {
		size += 2
	}
	if format&ValueFormatXPlaDevice != 0 {
		size += 2
	}
	if format&ValueFormatYPlaDevice != 0 {
		size += 2
	}
	if format&ValueFormatXAdvDevice != 0 {
		size += 2
	}
	if format&ValueFormatYAdvDevice != 0 {
		size += 2
	}
	return size
}

// parseAnchor parses an Anchor table from binary data.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#anchor-tables
func parseAnchor(b binarySegm) Anchor {
	if len(b) < 6 {
		tracer().Errorf("Anchor table too small")
		return Anchor{}
	}

	anchor := Anchor{
		Format:      AnchorFormat(b.U16(0)),
		XCoordinate: int16(b.U16(2)),
		YCoordinate: int16(b.U16(4)),
	}

	switch anchor.Format {
	case AnchorFormat2:
		if len(b) >= 8 {
			anchor.AnchorPoint = b.U16(6)
		}
	case AnchorFormat3:
		if len(b) >= 10 {
			anchor.XDeviceOffset = b.U16(6)
			anchor.YDeviceOffset = b.U16(8)
		}
	}

	return anchor
}

func parseGPosLookupSubtable(b binarySegm, lookupType LayoutTableLookupType) LookupSubtable {
	return parseGPosLookupSubtableWithDepth(b, lookupType, 0)
}

func parseGPosLookupSubtableWithDepth(b binarySegm, lookupType LayoutTableLookupType, depth int) LookupSubtable {
	// Validate minimum buffer size to prevent panics
	if len(b) < 4 {
		tracer().Errorf("GPOS lookup subtable buffer too small: %d bytes", len(b))
		return LookupSubtable{}
	}

	format := b.U16(0)
	tracer().Debugf("parsing GPOS sub-table type %s, format %d at depth %d", lookupType.GPosString(), format, depth)
	sub := LookupSubtable{LookupType: lookupType, Format: format}

	// Most GPOS types have coverage at offset 2 (except Extension format 1)
	if !(lookupType == 9 && format == 1) {
		covlink, err := parseLink16(b, 2, b, "Coverage")
		if err == nil {
			sub.Coverage = parseCoverage(covlink.Jump().Bytes())
		}
	}

	switch lookupType {
	case 1:
		return parseGPosLookupSubtableType1(b, sub)
	case 2:
		return parseGPosLookupSubtableType2(b, sub)
	case 3:
		return parseGPosLookupSubtableType3(b, sub)
	case 4:
		return parseGPosLookupSubtableType4(b, sub)
	case 5:
		return parseGPosLookupSubtableType5(b, sub)
	case 6:
		return parseGPosLookupSubtableType6(b, sub)
	case 7:
		return parseGPosLookupSubtableType7(b, sub)
	case 8:
		return parseGPosLookupSubtableType8(b, sub)
	case 9:
		return parseGPosLookupSubtableType9WithDepth(b, sub, depth)
	}
	tracer().Errorf("unknown GPOS lookup type: %d", lookupType)
	return LookupSubtable{}
}

// LookupType 1: Single Adjustment Positioning Subtable
// Adjusts the placement or advance of a single glyph.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-1-single-adjustment-positioning-subtable
func parseGPosLookupSubtableType1(b binarySegm, sub LookupSubtable) LookupSubtable {
	if len(b) < 6 {
		tracer().Errorf("GPOS type 1 buffer too small: %d bytes", len(b))
		return LookupSubtable{}
	}

	format := ValueFormat(b.U16(4))

	if sub.Format == 1 {
		// Format 1: Single positioning value applied to all covered glyphs
		vr, _ := parseValueRecord(b, 6, format)
		sub.Support = vr
	} else if sub.Format == 2 {
		// Format 2: Array of positioning values, one per covered glyph
		if len(b) < 8 {
			tracer().Errorf("GPOS type 1 format 2 buffer too small: %d bytes", len(b))
			return LookupSubtable{}
		}
		valueCount := b.U16(6)
		values := make([]ValueRecord, valueCount)
		offset := 8

		for i := uint16(0); i < valueCount; i++ {
			vr, size := parseValueRecord(b, offset, format)
			values[i] = vr
			offset += size
		}
		sub.Support = values
	}

	return sub
}

// LookupType 2: Pair Adjustment Positioning Subtable
// Adjusts the placement or advance of a pair of glyphs (kerning).
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-2-pair-adjustment-positioning-subtable
func parseGPosLookupSubtableType2(b binarySegm, sub LookupSubtable) LookupSubtable {
	if sub.Format == 1 {
		// Format 1: Adjustments for glyph pairs
		valueFormat1 := ValueFormat(b.U16(4))
		valueFormat2 := ValueFormat(b.U16(6))
		sub.Index = parseVarArray16(b, 8, 2, 2, "LookupSubtableGPos2-1")
		sub.Support = [2]ValueFormat{valueFormat1, valueFormat2}
	} else if sub.Format == 2 {
		// Format 2: Class pair adjustment
		valueFormat1 := ValueFormat(b.U16(4))
		valueFormat2 := ValueFormat(b.U16(6))

		// Parse class definitions
		classDef1Link, _ := parseLink16(b, 8, b, "ClassDef1")
		classDef2Link, _ := parseLink16(b, 10, b, "ClassDef2")

		classDef1, _ := parseClassDefinitions(classDef1Link.Jump().Bytes())
		classDef2, _ := parseClassDefinitions(classDef2Link.Jump().Bytes())

		class1Count := b.U16(12)
		class2Count := b.U16(14)

		// Store format information and class definitions
		sub.Support = struct {
			ValueFormat1 ValueFormat
			ValueFormat2 ValueFormat
			ClassDef1    ClassDefinitions
			ClassDef2    ClassDefinitions
			Class1Count  uint16
			Class2Count  uint16
		}{valueFormat1, valueFormat2, classDef1, classDef2, class1Count, class2Count}
	}

	return sub
}

// LookupType 3: Cursive Attachment Positioning Subtable
// Attaches cursive glyphs to each other.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-3-cursive-attachment-positioning-subtable
func parseGPosLookupSubtableType3(b binarySegm, sub LookupSubtable) LookupSubtable {
	// Format 1 only
	entryExitCount := b.U16(4)

	// Parse array of entry/exit anchor pairs
	sub.Index = parseVarArray16(b, 6, 4, 1, "LookupSubtableGPos3")
	sub.Support = entryExitCount

	return sub
}

// LookupType 4: MarkToBase Attachment Positioning Subtable
// Attaches combining marks to base glyphs.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-4-mark-to-base-attachment-positioning-subtable
func parseGPosLookupSubtableType4(b binarySegm, sub LookupSubtable) LookupSubtable {
	// Format 1 only
	markCoverageOffset := b.U16(2)
	baseCoverageOffset := b.U16(4)
	markClassCount := b.U16(6)
	markArrayOffset := b.U16(8)
	baseArrayOffset := b.U16(10)

	// Parse mark coverage
	if markCoverageOffset > 0 {
		markCovLink, _ := parseLink16(b, 2, b, "MarkCoverage")
		sub.Coverage = parseCoverage(markCovLink.Jump().Bytes())
	}

	// Parse base array as VarArray
	sub.Index = parseVarArray16(b, int(baseArrayOffset), 2, int(markClassCount)*2, "LookupSubtableGPos4")

	// Store additional data
	sub.Support = struct {
		BaseCoverageOffset uint16
		MarkClassCount     uint16
		MarkArrayOffset    uint16
	}{baseCoverageOffset, markClassCount, markArrayOffset}

	return sub
}

// LookupType 5: MarkToLigature Attachment Positioning Subtable
// Attaches combining marks to ligature glyphs.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-5-mark-to-ligature-attachment-positioning-subtable
func parseGPosLookupSubtableType5(b binarySegm, sub LookupSubtable) LookupSubtable {
	// Format 1 only
	markCoverageOffset := b.U16(2)
	ligatureCoverageOffset := b.U16(4)
	markClassCount := b.U16(6)
	markArrayOffset := b.U16(8)
	ligatureArrayOffset := b.U16(10)

	// Parse mark coverage
	if markCoverageOffset > 0 {
		markCovLink, _ := parseLink16(b, 2, b, "MarkCoverage")
		sub.Coverage = parseCoverage(markCovLink.Jump().Bytes())
	}

	// Parse ligature array as VarArray
	sub.Index = parseVarArray16(b, int(ligatureArrayOffset), 2, 2, "LookupSubtableGPos5")

	// Store additional data
	sub.Support = struct {
		LigatureCoverageOffset uint16
		MarkClassCount         uint16
		MarkArrayOffset        uint16
	}{ligatureCoverageOffset, markClassCount, markArrayOffset}

	return sub
}

// LookupType 6: MarkToMark Attachment Positioning Subtable
// Attaches combining marks to other marks.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-6-mark-to-mark-attachment-positioning-subtable
func parseGPosLookupSubtableType6(b binarySegm, sub LookupSubtable) LookupSubtable {
	// Format 1 only
	mark1CoverageOffset := b.U16(2)
	mark2CoverageOffset := b.U16(4)
	markClassCount := b.U16(6)
	mark1ArrayOffset := b.U16(8)
	mark2ArrayOffset := b.U16(10)

	// Parse mark1 coverage
	if mark1CoverageOffset > 0 {
		mark1CovLink, _ := parseLink16(b, 2, b, "Mark1Coverage")
		sub.Coverage = parseCoverage(mark1CovLink.Jump().Bytes())
	}

	// Parse mark2 array as VarArray
	sub.Index = parseVarArray16(b, int(mark2ArrayOffset), 2, int(markClassCount)*2, "LookupSubtableGPos6")

	// Store additional data
	sub.Support = struct {
		Mark2CoverageOffset uint16
		MarkClassCount      uint16
		Mark1ArrayOffset    uint16
	}{mark2CoverageOffset, markClassCount, mark1ArrayOffset}

	return sub
}

// LookupType 7: Contextual Positioning Subtable
// Positions one or more glyphs in context.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-7-contextual-positioning-subtable
func parseGPosLookupSubtableType7(b binarySegm, sub LookupSubtable) LookupSubtable {
	switch sub.Format {
	case 1:
		sub.Index = parseVarArray16(b, 4, 2, 2, "LookupSubtableGPos7-1")
	case 2:
		sub.Index = parseVarArray16(b, 6, 2, 2, "LookupSubtableGPos7-2")
	case 3:
		sub.Index = parseVarArray16(b, 4, 4, 2, "LookupSubtableGPos7-3")
	}
	var err error
	sub, err = parseSequenceContext(b, sub)
	if err != nil {
		tracer().Errorf(err.Error())
	}
	return sub
}

// LookupType 8: Chained Contexts Positioning Subtable
// Positions one or more glyphs in chained context with backtrack and lookahead.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-8-chained-contexts-positioning-subtable
func parseGPosLookupSubtableType8(b binarySegm, sub LookupSubtable) LookupSubtable {
	if len(b) < 6 {
		tracer().Errorf("GPOS type 8 buffer too small: %d bytes", len(b))
		return LookupSubtable{}
	}

	var err error
	sub, err = parseChainedSequenceContext(b, sub)
	if err != nil {
		tracer().Errorf("GPOS type 8 chained context error: %v", err)
		return LookupSubtable{}
	}

	switch sub.Format {
	case 1:
		sub.Index = parseVarArray16(b, 4, 2, 2, "LookupSubtableGPos8-1")
	case 2:
		if len(b) < 12 {
			tracer().Errorf("GPOS type 8 format 2 buffer too small: %d bytes", len(b))
			return LookupSubtable{}
		}
		sub.Index = parseVarArray16(b, 10, 2, 2, "LookupSubtableGPos8-2")
	case 3:
		// Safe type assertion to prevent panic
		seqctx, ok := sub.Support.(*SequenceContext)
		if !ok {
			tracer().Errorf("GPOS type 8 format 3: Support is not *SequenceContext")
			return LookupSubtable{}
		}

		offset := 2
		offset += 2 + len(seqctx.BacktrackCoverage)*2
		offset += 2 + len(seqctx.InputCoverage)*2
		offset += 2 + len(seqctx.LookaheadCoverage)*2

		if offset >= len(b) {
			tracer().Errorf("GPOS type 8 format 3: offset %d exceeds buffer size %d", offset, len(b))
			return LookupSubtable{}
		}

		sub.Index = parseVarArray16(b, offset, 2, 2, "LookupSubtableGPos8-3")
	}
	return sub
}

// LookupType 9: Extension Positioning
// Provides a mechanism for extending GPOS lookups to use 32-bit offsets.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-9-extension-positioning
func parseGPosLookupSubtableType9(b binarySegm, sub LookupSubtable) LookupSubtable {
	return parseGPosLookupSubtableType9WithDepth(b, sub, 0)
}

func parseGPosLookupSubtableType9WithDepth(b binarySegm, sub LookupSubtable, depth int) LookupSubtable {
	if b.Size() < 8 {
		tracer().Errorf("OpenType GPOS lookup subtable type %d corrupt", sub.LookupType)
		return LookupSubtable{}
	}

	actualType := LayoutTableLookupType(b.U16(2))
	if actualType == GPosLookupTypeExtensionPos {
		tracer().Errorf("OpenType GPOS extension subtable cannot recursively reference extension type")
		return LookupSubtable{}
	}

	tracer().Debugf("OpenType GPOS extension subtable is of type %s at depth %d", actualType.GPosString(), depth)
	link, _ := parseLink32(b, 4, b, "ext.LookupSubtable")
	loc := link.Jump()

	// Recurse with incremented depth
	return parseGPosLookupSubtableWithDepth(loc.Bytes(), actualType, depth+1)
}
