package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
)

// applyPositionPolicies applies the plan's position policies after GPOS.
// Currently the only policy cancels the advances of GDEF marks.
func (e *planExecutor) applyPositionPolicies(pl *plan, appliedGPOS bool) {
	if e == nil || e.run == nil || pl == nil {
		return
	}
	if !pl.Policy.ZeroGDEFMarks {
		return
	}
	e.run.EnsurePos()
	if appliedGPOS {
		e.zeroGDEFMarkAdvances(pl)
	}
}

// zeroGDEFMarkAdvances preserves the run's adjustment-only representation.
// Materialization adds the nominal horizontal metric, so cancel that metric
// rather than setting the adjustment to zero. Placement and attachment data
// are independent of advance zeroing. This policy does not infer marks from
// Unicode or attachments.
func (e *planExecutor) zeroGDEFMarkAdvances(pl *plan) {
	if pl.font == nil || pl.font.Layout.GDef == nil || pl.font.Layout.GDef.Header().GlyphClassDefOffset == 0 {
		return
	}
	for i, gid := range e.run.Glyphs {
		if ot.GlyphClassDefEnum(pl.font.Layout.GDef.GlyphClassDef.Lookup(gid)) != ot.MarkGlyph {
			continue
		}
		e.run.Pos[i].XAdvance = -int32(otquery.GlyphMetrics(pl.font, gid).Advance)
		e.run.Pos[i].YAdvance = 0 // materialization currently adds no vertical metric
	}
}
