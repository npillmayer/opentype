package otlayout

import "github.com/npillmayer/opentype/ot"

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
