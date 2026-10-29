package otshape

import (
	"math"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/unicode/bidi"
)

// finishMarkAttachments resolves horizontal Latin/Hebrew attachments after GPOS
// and advance zeroing, before the postprocess hook sees completed positions.
// GPOS 4/5/6 attach only to an earlier glyph. Walking in logical order therefore
// resolves chains without recursion, including RTL. Prefix sums keep long mark
// sequences linear in the size of the run rather than quadratic.
func (e *planExecutor) finishMarkAttachments(pl *plan) error {
	if e == nil || e.run == nil || pl == nil || !pl.Policy.ApplyGPOS ||
		(pl.ScriptTag != ot.T("latn") && pl.ScriptTag != ot.T("hebr")) {
		return nil
	}
	run := e.run
	pending := false
	for i, pos := range run.Pos {
		if !pos.MarkAttachmentPending {
			continue
		}
		pending = true
		if pos.AttachTo < 0 || int(pos.AttachTo) >= i ||
			(pos.AttachKind != otlayout.AttachMarkToBase && pos.AttachKind != otlayout.AttachMarkToLigature && pos.AttachKind != otlayout.AttachMarkToMark) {
			return errShaper("mark attachment has invalid target or kind")
		}
	}
	if !pending {
		return nil
	}
	if len(run.Pos) != run.Len() || pl.font == nil {
		return errShaper("mark attachment requires aligned positions and font metrics")
	}
	// Positions store advance adjustments; include the same nominal metric as
	// materializeGlyphRecord. Y has no nominal metric in horizontal shaping.
	type pen struct{ x, y int64 }
	pens := make([]pen, run.Len()+1)
	for i, gid := range run.Glyphs {
		pens[i+1] = pen{
			x: pens[i].x + int64(otquery.GlyphMetrics(pl.font, gid).Advance) + int64(run.Pos[i].XAdvance),
			y: pens[i].y + int64(run.Pos[i].YAdvance),
		}
	}
	for i := range run.Pos {
		pos := &run.Pos[i]
		if !pos.MarkAttachmentPending {
			continue
		}
		j := int(pos.AttachTo)
		dx, dy := pens[j].x-pens[i].x, pens[j].y-pens[i].y
		if pl.Props.Direction == bidi.RightToLeft {
			// In visual RTL order the mark precedes its target. Keep storage in
			// logical order while using the advances from (target, mark].
			dx, dy = pens[i+1].x-pens[j+1].x, pens[i+1].y-pens[j+1].y
		}
		pos.XOffset = clampMarkPosition(int64(pos.XOffset) + int64(run.Pos[j].XOffset) + dx)
		// Target Y placement was captured when this attachment was applied.
		pos.YOffset = clampMarkPosition(int64(pos.YOffset) + dy)
		pos.MarkAttachmentPending = false // finalizing twice must not add again
	}
	return nil
}

func clampMarkPosition(v int64) int32 {
	return int32(max(int64(math.MinInt32), min(int64(math.MaxInt32), v)))
}
