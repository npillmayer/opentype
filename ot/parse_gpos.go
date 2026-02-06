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

// parseMarkArray parses a MarkArray table from binary data.
func parseMarkArray(b binarySegm) MarkArray {
	if len(b) < 2 {
		return MarkArray{}
	}
	markCount := b.U16(0)
	records := make([]MarkRecord, 0, markCount)
	offset := 2
	for i := 0; i < int(markCount); i++ {
		if offset+4 > len(b) {
			break
		}
		rec := MarkRecord{
			Class:      b.U16(offset),
			MarkAnchor: b.U16(offset + 2),
		}
		records = append(records, rec)
		offset += 4
	}
	return MarkArray{MarkCount: markCount, MarkRecords: records}
}

// parseBaseArray parses a BaseArray table from binary data.
func parseBaseArray(b binarySegm, classCount int) []BaseRecord {
	if len(b) < 2 || classCount <= 0 {
		return nil
	}
	baseCount := int(b.U16(0))
	offset := 2
	recs := make([]BaseRecord, 0, baseCount)
	for i := 0; i < baseCount; i++ {
		if offset+classCount*2 > len(b) {
			break
		}
		anchors := make([]uint16, classCount)
		for c := 0; c < classCount; c++ {
			anchors[c] = b.U16(offset + c*2)
		}
		recs = append(recs, BaseRecord{BaseAnchors: anchors})
		offset += classCount * 2
	}
	return recs
}

// parseLigatureArray parses a LigatureArray table from binary data.
func parseLigatureArray(b binarySegm, classCount int) []LigatureAttach {
	if len(b) < 2 || classCount <= 0 {
		return nil
	}
	ligCount := int(b.U16(0))
	offset := 2
	offs := make([]uint16, 0, ligCount)
	for i := 0; i < ligCount; i++ {
		if offset+2 > len(b) {
			break
		}
		offs = append(offs, b.U16(offset))
		offset += 2
	}
	out := make([]LigatureAttach, 0, len(offs))
	for _, o := range offs {
		if o == 0 || int(o)+2 > len(b) {
			continue
		}
		lig := b[o:]
		compCount := int(binarySegm(lig).U16(0))
		compAnchors := make([][]uint16, 0, compCount)
		compOff := 2
		for c := 0; c < compCount; c++ {
			if compOff+classCount*2 > len(lig) {
				break
			}
			anchors := make([]uint16, classCount)
			for k := 0; k < classCount; k++ {
				anchors[k] = binarySegm(lig).U16(compOff + k*2)
			}
			compAnchors = append(compAnchors, anchors)
			compOff += classCount * 2
		}
		out = append(out, LigatureAttach{
			ComponentCount:   uint16(compCount),
			ComponentAnchors: compAnchors,
		})
	}
	return out
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
	if !(lookupType == 9 && format == 1) && !((lookupType == 7 || lookupType == 8) && format == 3) {
		// GPOS type 7/8 format 3 has coverage arrays, not a single coverage offset at byte 2.
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
		sub.Support = struct {
			Format ValueFormat
			Record ValueRecord
		}{format, vr}
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
		sub.Support = struct {
			Format  ValueFormat
			Records []ValueRecord
		}{format, values}
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

		// Parse class-based value records
		recSize1 := valueRecordSize(valueFormat1)
		recSize2 := valueRecordSize(valueFormat2)
		offset := 16
		need := offset + int(class1Count)*int(class2Count)*(recSize1+recSize2)
		records := make([][]struct {
			Value1 ValueRecord
			Value2 ValueRecord
		}, class1Count)
		if need <= len(b) {
			for i := 0; i < int(class1Count); i++ {
				row := make([]struct {
					Value1 ValueRecord
					Value2 ValueRecord
				}, class2Count)
				for j := 0; j < int(class2Count); j++ {
					v1, s1 := parseValueRecord(b, offset, valueFormat1)
					offset += s1
					v2, s2 := parseValueRecord(b, offset, valueFormat2)
					offset += s2
					row[j] = struct {
						Value1 ValueRecord
						Value2 ValueRecord
					}{v1, v2}
				}
				records[i] = row
			}
		} else {
			tracer().Errorf("GPOS 2/2 class records extend beyond buffer")
		}

		// Store format information and class definitions
		sub.Support = struct {
			ValueFormat1 ValueFormat
			ValueFormat2 ValueFormat
			ClassDef1    ClassDefinitions
			ClassDef2    ClassDefinitions
			Class1Count  uint16
			Class2Count  uint16
			ClassRecords [][]struct {
				Value1 ValueRecord
				Value2 ValueRecord
			}
		}{valueFormat1, valueFormat2, classDef1, classDef2, class1Count, class2Count, records}
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
	sub.Support = struct {
		EntryExitCount uint16
	}{entryExitCount}

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

	var baseCoverage Coverage
	if baseCoverageOffset > 0 {
		baseCovLink, _ := parseLink16(b, 4, b, "BaseCoverage")
		baseCoverage = parseCoverage(baseCovLink.Jump().Bytes())
	}
	var markArray MarkArray
	if markArrayOffset > 0 && int(markArrayOffset) < len(b) {
		markArray = parseMarkArray(b[markArrayOffset:])
	}
	var baseArray []BaseRecord
	if baseArrayOffset > 0 && int(baseArrayOffset) < len(b) {
		baseArray = parseBaseArray(b[baseArrayOffset:], int(markClassCount))
	}

	// Store additional data
	sub.Support = struct {
		BaseCoverage   Coverage
		MarkClassCount uint16
		MarkArray      MarkArray
		BaseArray      []BaseRecord
	}{baseCoverage, markClassCount, markArray, baseArray}

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

	var ligatureCoverage Coverage
	if ligatureCoverageOffset > 0 {
		ligCovLink, _ := parseLink16(b, 4, b, "LigatureCoverage")
		ligatureCoverage = parseCoverage(ligCovLink.Jump().Bytes())
	}
	var markArray MarkArray
	if markArrayOffset > 0 && int(markArrayOffset) < len(b) {
		markArray = parseMarkArray(b[markArrayOffset:])
	}
	var ligatureArray []LigatureAttach
	if ligatureArrayOffset > 0 && int(ligatureArrayOffset) < len(b) {
		ligatureArray = parseLigatureArray(b[ligatureArrayOffset:], int(markClassCount))
	}

	// Store additional data
	sub.Support = struct {
		LigatureCoverage Coverage
		MarkClassCount   uint16
		MarkArray        MarkArray
		LigatureArray    []LigatureAttach
	}{ligatureCoverage, markClassCount, markArray, ligatureArray}

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

	var mark2Coverage Coverage
	if mark2CoverageOffset > 0 {
		mark2CovLink, _ := parseLink16(b, 4, b, "Mark2Coverage")
		mark2Coverage = parseCoverage(mark2CovLink.Jump().Bytes())
	}
	var mark1Array MarkArray
	if mark1ArrayOffset > 0 && int(mark1ArrayOffset) < len(b) {
		mark1Array = parseMarkArray(b[mark1ArrayOffset:])
	}
	var mark2Array []BaseRecord
	if mark2ArrayOffset > 0 && int(mark2ArrayOffset) < len(b) {
		mark2Array = parseBaseArray(b[mark2ArrayOffset:], int(markClassCount))
	}

	// Store additional data
	sub.Support = struct {
		Mark2Coverage  Coverage
		MarkClassCount uint16
		Mark1Array     MarkArray
		Mark2Array     []BaseRecord
	}{mark2Coverage, markClassCount, mark1Array, mark2Array}

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

		if offset+2 > len(b) {
			tracer().Errorf("GPOS type 8 format 3: missing seqLookupCount at %d", offset)
			return LookupSubtable{}
		}

		seqLookupCount := int(b.U16(offset))
		offset += 2
		need := offset + seqLookupCount*4
		if need > len(b) {
			tracer().Errorf("GPOS type 8 format 3: lookup records extend beyond buffer")
			return LookupSubtable{}
		}
		sub.LookupRecords = make([]SequenceLookupRecord, seqLookupCount)
		for i := 0; i < seqLookupCount; i++ {
			base := offset + i*4
			sub.LookupRecords[i] = SequenceLookupRecord{
				SequenceIndex:   b.U16(base),
				LookupListIndex: b.U16(base + 2),
			}
		}
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
