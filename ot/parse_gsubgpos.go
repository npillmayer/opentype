package ot

// parseGSub parses the GSUB (Glyph Substitution) table.
func parseGSub(tag Tag, b binarySegm, offset, size uint32, ec *errorCollector) (Table, error) {
	var err error
	gsub := newGSubTable(tag, b, offset, size)
	err = parseLayoutHeader(&gsub.LayoutTable, b, err, tag, ec)
	err = parseLookupList(&gsub.LayoutTable, b, err, false, tag, ec) // false = GSUB
	err = parseFeatureList(&gsub.LayoutTable, b, err)
	err = parseScriptList(&gsub.LayoutTable, b, err)
	if err != nil {
		tracer().Errorf("error parsing GSUB table: %v", err)
		return gsub, err
	}
	mj, mn := gsub.header.Version()
	tracer().Debugf("GSUB table has version %d.%d", mj, mn)
	if graph := gsub.LookupGraph(); graph != nil {
		tracer().Debugf("GSUB table has %d lookup list entries", graph.Len())
	}
	return gsub, err
}

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
	if graph := gpos.LookupGraph(); graph != nil {
		tracer().Debugf("GPOS table has %d lookup list entries", graph.Len())
	}
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
	for range baseCount {
		if offset+classCount*2 > len(b) {
			break
		}
		anchors := make([]uint16, classCount)
		for c := range classCount {
			anchors[c] = b.U16(offset + c*2)
		}
		recs = append(recs, BaseRecord{BaseAnchors: anchors})
		offset += classCount * 2
	}
	return recs
}
