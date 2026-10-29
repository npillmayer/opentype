package otlayout

import (
	"math"

	"github.com/npillmayer/opentype/ot"
)

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

// setMarkAttachment installs design-unit anchor placement for horizontal shaping.
// Attachment replaces earlier placement of this mark; later value records add
// to it. The target's cross-stream placement is captured now (also for mark
// chains). Its horizontal placement and the final pen difference are resolved
// after all GPOS lookups and advance policies have run.
// Anchor-point hinting and device/variation adjustments are not implemented.
func setMarkAttachment(pos *PosItem, baseIndex int, kind AttachKind, class uint16, ref AnchorRef, mark, base *ot.Anchor, baseYOffset int32) {
	if pos == nil {
		return
	}
	pos.AttachTo = int32(baseIndex)
	pos.AttachKind = kind
	pos.AttachClass = class
	pos.AnchorRef = ref
	pos.XOffset = int32(base.XCoordinate) - int32(mark.XCoordinate)
	// Saturate before storing: final pen compensation cannot repair a wrapped
	// target cross-stream placement.
	yOffset := int64(base.YCoordinate) - int64(mark.YCoordinate) + int64(baseYOffset)
	pos.YOffset = int32(max(int64(math.MinInt32), min(int64(math.MaxInt32), yOffset)))
	pos.MarkAttachmentPending = true
}

// setCursiveAttachment records a cursive attachment without resolving anchor coordinates.
func setCursiveAttachment(pos *PosItem, baseIndex int, ref AnchorRef) {
	if pos == nil {
		return
	}
	pos.AttachTo = int32(baseIndex)
	pos.AttachKind = AttachCursive
	pos.AnchorRef = ref
	pos.MarkAttachmentPending = false
}
