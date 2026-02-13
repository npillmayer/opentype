package otshape

import "github.com/npillmayer/opentype/ot"

func (e *planExecutor) prepareGSUBAnnotations(pl *plan) {
	if e == nil || e.run == nil {
		return
	}
	e.ensureRunSyllables()
	e.ensureRunJoiners(pl)
}

func (e *planExecutor) ensureRunSyllables() {
	run := e.run
	n := run.Len()
	if n == 0 {
		return
	}
	if len(run.Syllables) == n && hasAnySyllable(run.Syllables) {
		return
	}
	run.EnsureSyllables()
	if len(run.Syllables) != n {
		run.Syllables = resizeUint16(run.Syllables, n)
	}
	if len(run.Clusters) == n {
		syll := uint16(1)
		prev := run.Clusters[0]
		run.Syllables[0] = syll
		for i := 1; i < n; i++ {
			if run.Clusters[i] != prev {
				if syll < ^uint16(0) {
					syll++
				}
				prev = run.Clusters[i]
			}
			run.Syllables[i] = syll
		}
		return
	}
	for i := range run.Syllables {
		s := uint16(i + 1)
		if i >= int(^uint16(0)) {
			s = ^uint16(0)
		}
		run.Syllables[i] = s
	}
}

func (e *planExecutor) ensureRunJoiners(pl *plan) {
	run := e.run
	n := run.Len()
	if n == 0 {
		return
	}
	if len(run.Joiners) == n && hasAnyJoiner(run.Joiners) {
		return
	}
	run.EnsureJoiners()
	if len(run.Joiners) != n {
		run.Joiners = resizeUint8(run.Joiners, n)
	}
	clear(run.Joiners)
	if pl == nil || len(pl.joinerGlyphClass) == 0 {
		return
	}
	for i, gid := range run.Glyphs {
		run.Joiners[i] = joinerClassForGlyph(pl.joinerGlyphClass, gid)
	}
}

func joinerClassForGlyph(classes map[ot.GlyphIndex]uint8, gid ot.GlyphIndex) uint8 {
	if len(classes) == 0 {
		return joinerClassNone
	}
	return classes[gid]
}

func hasAnySyllable(s []uint16) bool {
	for _, v := range s {
		if v != 0 {
			return true
		}
	}
	return false
}

func hasAnyJoiner(s []uint8) bool {
	for _, v := range s {
		if v != joinerClassNone {
			return true
		}
	}
	return false
}
