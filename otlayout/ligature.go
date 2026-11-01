package otlayout

import "github.com/npillmayer/opentype/ot"

// Each occurrence gets its own immutable identity. Pointer identity avoids
// collisions when independently shaped spans or reconstructed BufferStates meet.
type ligatureOrigin struct{ marker byte } // nonzero size guarantees distinct live identities

func (p PosItem) componentCount() int {
	if p.ligatureComponents > 0 {
		return p.ligatureComponents
	}
	return 1
}

// ligate consumes exactly the matched positions. Skipped glyphs retain their
// position records and individual source correspondence in the edit journal.
func (ctx *applyCtx) ligate(positions []int, glyph ot.GlyphIndex) *EditSpan {
	b := ctx.buf
	from, to := positions[0], positions[len(positions)-1]+1
	b.EnsurePos()
	first := b.Pos[from]
	allMarks := glyphClass(ctx.gdef, b.Glyphs[from]) == ot.MarkGlyph
	baseAndMarks := glyphClass(ctx.gdef, b.Glyphs[from]) == ot.BaseGlyph
	total := first.componentCount()
	for _, index := range positions[1:] {
		if glyphClass(ctx.gdef, b.Glyphs[index]) != ot.MarkGlyph {
			allMarks, baseAndMarks = false, false
		}
		total += b.Pos[index].componentCount()
	}
	// A mark ligature retains its attachment to an earlier base ligature.
	// A base+marks composition does not create new base components.
	origin := first.ligature
	component := first.ligatureComponent
	isLigature := !allMarks && !baseAndMarks
	if isLigature {
		origin, component = &ligatureOrigin{}, 0
	}
	var survivors []int
	last := first
	componentsSoFar := first.componentCount()
	for i := 1; i < len(positions); i++ {
		for j := positions[i-1] + 1; j < positions[i]; j++ {
			survivors = append(survivors, j-from)
			if isLigature {
				comp := b.Pos[j].ligatureComponent
				if comp == 0 {
					comp = last.componentCount()
				}
				b.Pos[j].ligature = origin
				b.Pos[j].ligatureComponent = componentsSoFar - last.componentCount() + min(comp, last.componentCount())
			}
		}
		last = b.Pos[positions[i]]
		componentsSoFar += last.componentCount()
	}
	// A nested ligature's marks can follow the final consumed component.
	// Reassign those marks as well as the ones inside the matched span.
	if !allMarks && last.ligature != nil {
		for j := to; j < len(b.Pos); j++ {
			p := &b.Pos[j]
			if p.ligature != last.ligature || p.ligatureComponent == 0 {
				break
			}
			p.ligature = origin
			p.ligatureComponent = componentsSoFar - last.componentCount() + min(p.ligatureComponent, last.componentCount())
		}
	}
	repl := make([]ot.GlyphIndex, 1, len(survivors)+1)
	repl[0] = glyph
	for _, offset := range survivors {
		repl = append(repl, b.Glyphs[from+offset])
	}
	edit := &EditSpan{From: from, To: to, Len: len(repl), ResetsPositions: true}
	if len(survivors) != 0 {
		edit.survivors = &ligatureSurvivors{offsets: survivors}
	}
	b.ensureUniqueGlyphs()
	b.Glyphs = b.Glyphs.Replace(from, to, repl)
	b.Pos = b.Pos.ApplyEdit(edit)
	b.Pos[from].ligature, b.Pos[from].ligatureComponent = origin, component
	b.Pos[from].ligatureComponents = first.ligatureComponents
	if isLigature {
		b.Pos[from].ligatureComponents = total
	}
	b.recordEdit(*edit)
	return edit
}
