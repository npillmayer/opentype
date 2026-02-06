package otlayout

import "github.com/npillmayer/opentype/ot"

// valueRecordSize returns the size in bytes of a ValueRecord based on its format.
func valueRecordSize(format ot.ValueFormat) int {
	size := 0
	if format&ot.ValueFormatXPlacement != 0 {
		size += 2
	}
	if format&ot.ValueFormatYPlacement != 0 {
		size += 2
	}
	if format&ot.ValueFormatXAdvance != 0 {
		size += 2
	}
	if format&ot.ValueFormatYAdvance != 0 {
		size += 2
	}
	if format&ot.ValueFormatXPlaDevice != 0 {
		size += 2
	}
	if format&ot.ValueFormatYPlaDevice != 0 {
		size += 2
	}
	if format&ot.ValueFormatXAdvDevice != 0 {
		size += 2
	}
	if format&ot.ValueFormatYAdvDevice != 0 {
		size += 2
	}
	return size
}

// parseValueRecord reads a ValueRecord from a byte location based on format.
// Returns the parsed ValueRecord and the number of bytes consumed.
func parseValueRecord(loc ot.NavLocation, offset int, format ot.ValueFormat) (ot.ValueRecord, int) {
	vr := ot.ValueRecord{}
	size := 0
	if format&ot.ValueFormatXPlacement != 0 {
		vr.XPlacement = int16(loc.U16(offset + size))
		size += 2
	}
	if format&ot.ValueFormatYPlacement != 0 {
		vr.YPlacement = int16(loc.U16(offset + size))
		size += 2
	}
	if format&ot.ValueFormatXAdvance != 0 {
		vr.XAdvance = int16(loc.U16(offset + size))
		size += 2
	}
	if format&ot.ValueFormatYAdvance != 0 {
		vr.YAdvance = int16(loc.U16(offset + size))
		size += 2
	}
	if format&ot.ValueFormatXPlaDevice != 0 {
		vr.XPlaDevice = loc.U16(offset + size)
		size += 2
	}
	if format&ot.ValueFormatYPlaDevice != 0 {
		vr.YPlaDevice = loc.U16(offset + size)
		size += 2
	}
	if format&ot.ValueFormatXAdvDevice != 0 {
		vr.XAdvDevice = loc.U16(offset + size)
		size += 2
	}
	if format&ot.ValueFormatYAdvDevice != 0 {
		vr.YAdvDevice = loc.U16(offset + size)
		size += 2
	}
	return vr, size
}

// applyValueRecord applies a ValueRecord to a position item according to format.
// Device table offsets are currently ignored.
func applyValueRecord(pos *PosItem, vr ot.ValueRecord, format ot.ValueFormat) {
	if pos == nil {
		return
	}
	if format&ot.ValueFormatXPlacement != 0 {
		pos.XOffset += int32(vr.XPlacement)
	}
	if format&ot.ValueFormatYPlacement != 0 {
		pos.YOffset += int32(vr.YPlacement)
	}
	if format&ot.ValueFormatXAdvance != 0 {
		pos.XAdvance += int32(vr.XAdvance)
	}
	if format&ot.ValueFormatYAdvance != 0 {
		pos.YAdvance += int32(vr.YAdvance)
	}
}

// applyValueRecordPair applies two ValueRecords to a pair of position items.
func applyValueRecordPair(p1, p2 *PosItem, v1 ot.ValueRecord, f1 ot.ValueFormat, v2 ot.ValueRecord, f2 ot.ValueFormat) {
	applyValueRecord(p1, v1, f1)
	applyValueRecord(p2, v2, f2)
}

// setMarkAttachment records a mark attachment without resolving anchor coordinates.
func setMarkAttachment(pos *PosItem, baseIndex int, kind AttachKind, class uint16, ref AnchorRef) {
	if pos == nil {
		return
	}
	pos.AttachTo = int32(baseIndex)
	pos.AttachKind = kind
	pos.AttachClass = class
	pos.AnchorRef = ref
}

// setCursiveAttachment records a cursive attachment without resolving anchor coordinates.
func setCursiveAttachment(pos *PosItem, baseIndex int, ref AnchorRef) {
	if pos == nil {
		return
	}
	pos.AttachTo = int32(baseIndex)
	pos.AttachKind = AttachCursive
	pos.AnchorRef = ref
}

// parseMarkArray parses a MarkArray table into an ot.MarkArray.
func parseMarkArray(loc ot.NavLocation) ot.MarkArray {
	if loc.Size() < 2 {
		return ot.MarkArray{}
	}
	markCount := loc.U16(0)
	records := make([]ot.MarkRecord, 0, markCount)
	off := 2
	for i := 0; i < int(markCount); i++ {
		if off+4 > loc.Size() {
			break
		}
		rec := ot.MarkRecord{
			Class:      loc.U16(off),
			MarkAnchor: loc.U16(off + 2),
		}
		records = append(records, rec)
		off += 4
	}
	return ot.MarkArray{MarkCount: markCount, MarkRecords: records}
}

// parseBaseArray parses a BaseArray table into a slice of BaseRecords.
func parseBaseArray(loc ot.NavLocation, classCount int) []ot.BaseRecord {
	if loc.Size() < 2 || classCount <= 0 {
		return nil
	}
	baseCount := int(loc.U16(0))
	off := 2
	recs := make([]ot.BaseRecord, 0, baseCount)
	for i := 0; i < baseCount; i++ {
		if off+classCount*2 > loc.Size() {
			break
		}
		anchors := make([]uint16, classCount)
		for c := 0; c < classCount; c++ {
			anchors[c] = loc.U16(off + c*2)
		}
		recs = append(recs, ot.BaseRecord{BaseAnchors: anchors})
		off += classCount * 2
	}
	return recs
}

// parseLigatureArray parses a LigatureArray table into LigatureAttach records.
func parseLigatureArray(loc ot.NavLocation, classCount int) []ot.LigatureAttach {
	if loc.Size() < 2 || classCount <= 0 {
		return nil
	}
	ligCount := int(loc.U16(0))
	off := 2
	offsets := make([]uint16, 0, ligCount)
	for i := 0; i < ligCount; i++ {
		if off+2 > loc.Size() {
			break
		}
		offsets = append(offsets, loc.U16(off))
		off += 2
	}
	ligs := make([]ot.LigatureAttach, 0, len(offsets))
	for _, o := range offsets {
		if o == 0 || int(o)+2 > loc.Size() {
			continue
		}
		ligLoc := loc.Slice(int(o), loc.Size())
		compCount := int(ligLoc.U16(0))
		compAnchors := make([][]uint16, 0, compCount)
		compOff := 2
		for c := 0; c < compCount; c++ {
			if compOff+classCount*2 > ligLoc.Size() {
				break
			}
			anchors := make([]uint16, classCount)
			for k := 0; k < classCount; k++ {
				anchors[k] = ligLoc.U16(compOff + k*2)
			}
			compAnchors = append(compAnchors, anchors)
			compOff += classCount * 2
		}
		ligs = append(ligs, ot.LigatureAttach{
			ComponentCount:   uint16(compCount),
			ComponentAnchors: compAnchors,
		})
	}
	return ligs
}

// parseMark2Array parses a Mark2Array table into BaseRecords.
func parseMark2Array(loc ot.NavLocation, classCount int) []ot.BaseRecord {
	return parseBaseArray(loc, classCount)
}

// gposSupportOffsets extracts mark/base/ligature array offsets and class count from sub.Support.
// Returns false if the support structure is not compatible.
func gposSupportOffsets(sub *ot.LookupSubtable) (markArrayOffset uint16, baseArrayOffset uint16, classCount uint16, ok bool) {
	if sub == nil {
		return 0, 0, 0, false
	}
	switch v := sub.Support.(type) {
	case struct {
		BaseCoverageOffset uint16
		MarkClassCount     uint16
		MarkArrayOffset    uint16
	}:
		return v.MarkArrayOffset, v.BaseCoverageOffset, v.MarkClassCount, true
	case struct {
		LigatureCoverageOffset uint16
		MarkClassCount         uint16
		MarkArrayOffset        uint16
	}:
		return v.MarkArrayOffset, v.LigatureCoverageOffset, v.MarkClassCount, true
	case struct {
		Mark2CoverageOffset uint16
		MarkClassCount      uint16
		Mark1ArrayOffset    uint16
	}:
		return v.Mark1ArrayOffset, v.Mark2CoverageOffset, v.MarkClassCount, true
	default:
		return 0, 0, 0, false
	}
}

// parseMarkAttachmentTables parses mark array and the corresponding attachment array for a subtable.
// The caller must supply the raw subtable location and the expected attachment array offset.
func parseMarkAttachmentTables(sub *ot.LookupSubtable, loc ot.NavLocation, attachArrayOffset uint16) (
	ot.MarkArray, []ot.BaseRecord, bool,
) {
	markArrayOffset, _, classCount, ok := gposSupportOffsets(sub)
	if !ok || markArrayOffset == 0 || attachArrayOffset == 0 {
		return ot.MarkArray{}, nil, false
	}
	if int(markArrayOffset) >= loc.Size() || int(attachArrayOffset) >= loc.Size() {
		return ot.MarkArray{}, nil, false
	}
	markArray := parseMarkArray(loc.Slice(int(markArrayOffset), loc.Size()))
	attachments := parseBaseArray(loc.Slice(int(attachArrayOffset), loc.Size()), int(classCount))
	return markArray, attachments, true
}

// parseMarkToLigatureTables parses mark array and ligature attachment array.
func parseMarkToLigatureTables(sub *ot.LookupSubtable, loc ot.NavLocation, ligatureArrayOffset uint16) (
	ot.MarkArray, []ot.LigatureAttach, bool,
) {
	markArrayOffset, _, classCount, ok := gposSupportOffsets(sub)
	if !ok || markArrayOffset == 0 || ligatureArrayOffset == 0 {
		return ot.MarkArray{}, nil, false
	}
	if int(markArrayOffset) >= loc.Size() || int(ligatureArrayOffset) >= loc.Size() {
		return ot.MarkArray{}, nil, false
	}
	markArray := parseMarkArray(loc.Slice(int(markArrayOffset), loc.Size()))
	ligs := parseLigatureArray(loc.Slice(int(ligatureArrayOffset), loc.Size()), int(classCount))
	return markArray, ligs, true
}

// parseMarkToMarkTables parses mark array and mark2 attachment array.
func parseMarkToMarkTables(sub *ot.LookupSubtable, loc ot.NavLocation, mark2ArrayOffset uint16) (
	ot.MarkArray, []ot.BaseRecord, bool,
) {
	markArrayOffset, _, classCount, ok := gposSupportOffsets(sub)
	if !ok || markArrayOffset == 0 || mark2ArrayOffset == 0 {
		return ot.MarkArray{}, nil, false
	}
	if int(markArrayOffset) >= loc.Size() || int(mark2ArrayOffset) >= loc.Size() {
		return ot.MarkArray{}, nil, false
	}
	markArray := parseMarkArray(loc.Slice(int(markArrayOffset), loc.Size()))
	attachments := parseMark2Array(loc.Slice(int(mark2ArrayOffset), loc.Size()), int(classCount))
	return markArray, attachments, true
}
