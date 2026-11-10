package otshape

import "github.com/npillmayer/opentype/ot"

// PositionedGlyph is a compact rendering view of one shaped glyph occurrence.
// The font and desired size are supplied separately by the caller.
//
// Advances and offsets are signed font design units, not absolute coordinates.
// Advances move the pen; offsets move the glyph relative to the pen. XAdvance
// already includes the nominal glyph advance and shaping adjustments. Use
// otquery.FontScale to convert these values to a desired em size.
//
// PositionedGlyph omits feature masks, break-safety flags, and attachment
// diagnostics. Retain [GlyphRecord] when that detailed information is needed.
type PositionedGlyph struct {
	GID      ot.GlyphIndex // Glyph ID in the font used for shaping.
	Cluster  uint32        // Original zero-based Unicode scalar index, as in GlyphRecord.Cluster.
	XAdvance int32
	YAdvance int32
	XOffset  int32
	YOffset  int32
}

// PositionedGlyph returns a compact value copy of g's glyph ID, source cluster,
// and final advances and offsets. It uses g.Cluster, not g.Pos.Cluster.
// It does not scale, round, reorder, or finalize positioning. Use it on emitted
// shaping output. Converting a sequence preserves its existing order; to render
// an RTL run, apply [VisualOrder] to the detailed records before converting them.
func (g GlyphRecord) PositionedGlyph() PositionedGlyph {
	return PositionedGlyph{
		GID: g.GID, Cluster: g.Cluster,
		XAdvance: g.Pos.XAdvance, YAdvance: g.Pos.YAdvance,
		XOffset: g.Pos.XOffset, YOffset: g.Pos.YOffset,
	}
}
