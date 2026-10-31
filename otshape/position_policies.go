package otshape

import (
	"unicode"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/unicode/bidi"
)

func (e *planExecutor) applyPositionPolicies(pl *plan, appliedGPOS bool) {
	if e == nil || e.run == nil || pl == nil {
		return
	}
	if !pl.Policy.ZeroMarks && !pl.Policy.ZeroGDEFMarks && !pl.Policy.FallbackMarkPos {
		return
	}
	e.run.EnsurePos()
	if pl.Policy.FallbackMarkPos && !appliedGPOS {
		e.applyFallbackMarkPosition(pl)
	}
	if pl.Policy.ZeroMarks {
		adjustOffsets := !appliedGPOS && pl.Props.Direction == bidi.LeftToRight
		e.zeroMarkAdvances(pl, adjustOffsets)
	}
	if pl.Policy.ZeroGDEFMarks && appliedGPOS {
		e.zeroGDEFMarkAdvances(pl)
	}
}

// zeroGDEFMarkAdvances preserves the run's adjustment-only representation.
// Materialization adds the nominal horizontal metric, so cancel that metric
// rather than setting the adjustment to zero. Placement and attachment data
// are independent of advance zeroing. The older fallback policy below remains
// separate: this policy does not infer marks from Unicode or attachments.
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

func (e *planExecutor) zeroMarkAdvances(pl *plan, adjustOffsets bool) {
	if e == nil || e.run == nil {
		return
	}
	for i := range e.run.Pos {
		if !e.isMarkGlyph(pl, i) {
			continue
		}
		if adjustOffsets {
			e.run.Pos[i].XOffset -= e.run.Pos[i].XAdvance
			e.run.Pos[i].YOffset -= e.run.Pos[i].YAdvance
		}
		e.run.Pos[i].XAdvance = 0
		e.run.Pos[i].YAdvance = 0
	}
}

func (e *planExecutor) applyFallbackMarkPosition(pl *plan) {
	if e == nil || e.run == nil {
		return
	}
	for i := range e.run.Pos {
		if !e.isMarkByClass(pl, i) {
			continue
		}
		if e.run.Pos[i].AttachTo >= 0 {
			continue
		}
		base := e.prevBaseGlyph(pl, i)
		if base < 0 {
			continue
		}
		e.run.Pos[i].AttachKind = otlayout.AttachMarkToBase
		e.run.Pos[i].AttachTo = int32(base)
	}
}

func (e *planExecutor) prevBaseGlyph(pl *plan, from int) int {
	for i := from - 1; i >= 0; i-- {
		if !e.isMarkByClass(pl, i) {
			return i
		}
	}
	return -1
}

func (e *planExecutor) isMarkGlyph(pl *plan, inx int) bool {
	if e == nil || e.run == nil || inx < 0 || inx >= e.run.Len() {
		return false
	}
	if len(e.run.Pos) == e.run.Len() {
		switch e.run.Pos[inx].AttachKind {
		case otlayout.AttachMarkToBase, otlayout.AttachMarkToLigature, otlayout.AttachMarkToMark:
			return true
		}
	}
	return e.isMarkByClass(pl, inx)
}

func (e *planExecutor) isMarkByClass(pl *plan, inx int) bool {
	if e == nil || e.run == nil || inx < 0 || inx >= e.run.Len() {
		return false
	}
	gid := e.run.Glyphs[inx]
	if pl != nil && pl.font != nil && pl.font.Layout.GDef != nil {
		if ot.GlyphClassDefEnum(pl.font.Layout.GDef.GlyphClassDef.Lookup(gid)) == ot.MarkGlyph {
			return true
		}
	}
	if pl != nil && pl.font != nil {
		if cp := otquery.CodePointForGlyph(pl.font, gid); cp != 0 && unicode.Is(unicode.M, cp) {
			return true
		}
	}
	return false
}
