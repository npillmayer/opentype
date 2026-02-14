package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"golang.org/x/text/unicode/norm"
)

// runContext is an internal adapter from runBuffer to exported RunContext.
type runContext struct {
	run *runBuffer
}

func newRunContext(run *runBuffer) runContext {
	return runContext{run: run}
}

func (rc runContext) Len() int {
	if rc.run == nil {
		return 0
	}
	return rc.run.Len()
}

func (rc runContext) Glyph(i int) ot.GlyphIndex {
	if rc.run == nil || i < 0 || i >= rc.run.Len() {
		return 0
	}
	return rc.run.Glyphs[i]
}

func (rc runContext) SetGlyph(i int, gid ot.GlyphIndex) {
	if rc.run == nil || i < 0 || i >= rc.run.Len() {
		return
	}
	rc.run.Glyphs[i] = gid
}

func (rc runContext) Codepoint(i int) rune {
	if rc.run == nil || i < 0 || i >= rc.run.Len() || len(rc.run.Codepoints) != rc.run.Len() {
		return 0
	}
	return rc.run.Codepoints[i]
}

func (rc runContext) SetCodepoint(i int, cp rune) {
	if rc.run == nil || i < 0 || i >= rc.run.Len() {
		return
	}
	rc.run.EnsureCodepoints()
	rc.run.Codepoints[i] = cp
}

func (rc runContext) Cluster(i int) uint32 {
	if rc.run == nil || i < 0 || i >= rc.run.Len() || len(rc.run.Clusters) != rc.run.Len() {
		return 0
	}
	return rc.run.Clusters[i]
}

func (rc runContext) SetCluster(i int, cluster uint32) {
	if rc.run == nil || i < 0 || i >= rc.run.Len() {
		return
	}
	rc.run.EnsureClusters()
	rc.run.Clusters[i] = cluster
}

func (rc runContext) MergeClusters(start, end int) {
	if rc.run == nil || len(rc.run.Clusters) != rc.run.Len() {
		return
	}
	n := rc.run.Len()
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start >= end {
		return
	}
	minCluster := rc.run.Clusters[start]
	for i := start + 1; i < end; i++ {
		if rc.run.Clusters[i] < minCluster {
			minCluster = rc.run.Clusters[i]
		}
	}
	for i := start; i < end; i++ {
		rc.run.Clusters[i] = minCluster
	}
}

func (rc runContext) Pos(i int) otlayout.PosItem {
	if rc.run == nil || i < 0 || i >= rc.run.Len() || len(rc.run.Pos) != rc.run.Len() {
		return otlayout.PosItem{AttachTo: -1}
	}
	return rc.run.Pos[i]
}

func (rc runContext) SetPos(i int, pos otlayout.PosItem) {
	if rc.run == nil || i < 0 || i >= rc.run.Len() {
		return
	}
	rc.run.EnsurePos()
	rc.run.Pos[i] = pos
}

func (rc runContext) Mask(i int) uint32 {
	if rc.run == nil || i < 0 || i >= rc.run.Len() || len(rc.run.Masks) != rc.run.Len() {
		return 0
	}
	return rc.run.Masks[i]
}

func (rc runContext) SetMask(i int, mask uint32) {
	if rc.run == nil || i < 0 || i >= rc.run.Len() {
		return
	}
	rc.run.EnsureMasks()
	rc.run.Masks[i] = mask
}

func (rc runContext) InsertGlyphs(index int, glyphs []ot.GlyphIndex) {
	if rc.run == nil {
		return
	}
	rc.run.InsertGlyphs(index, glyphs)
}

func (rc runContext) InsertGlyphCopies(index int, source int, count int) {
	if rc.run == nil {
		return
	}
	rc.run.InsertGlyphCopies(index, source, count)
}

func (rc runContext) Swap(i, j int) {
	if rc.run == nil || i < 0 || j < 0 || i >= rc.run.Len() || j >= rc.run.Len() || i == j {
		return
	}
	rc.run.Glyphs[i], rc.run.Glyphs[j] = rc.run.Glyphs[j], rc.run.Glyphs[i]
	if len(rc.run.Pos) == rc.run.Len() {
		rc.run.Pos[i], rc.run.Pos[j] = rc.run.Pos[j], rc.run.Pos[i]
	}
	if len(rc.run.Codepoints) == rc.run.Len() {
		rc.run.Codepoints[i], rc.run.Codepoints[j] = rc.run.Codepoints[j], rc.run.Codepoints[i]
	}
	if len(rc.run.Clusters) == rc.run.Len() {
		rc.run.Clusters[i], rc.run.Clusters[j] = rc.run.Clusters[j], rc.run.Clusters[i]
	}
	if len(rc.run.Masks) == rc.run.Len() {
		rc.run.Masks[i], rc.run.Masks[j] = rc.run.Masks[j], rc.run.Masks[i]
	}
	if len(rc.run.PlanIDs) == rc.run.Len() {
		rc.run.PlanIDs[i], rc.run.PlanIDs[j] = rc.run.PlanIDs[j], rc.run.PlanIDs[i]
	}
	if len(rc.run.UnsafeFlags) == rc.run.Len() {
		rc.run.UnsafeFlags[i], rc.run.UnsafeFlags[j] = rc.run.UnsafeFlags[j], rc.run.UnsafeFlags[i]
	}
	if len(rc.run.Syllables) == rc.run.Len() {
		rc.run.Syllables[i], rc.run.Syllables[j] = rc.run.Syllables[j], rc.run.Syllables[i]
	}
	if len(rc.run.Joiners) == rc.run.Len() {
		rc.run.Joiners[i], rc.run.Joiners[j] = rc.run.Joiners[j], rc.run.Joiners[i]
	}
}

// pauseContext is the internal adapter for pause callbacks.
type pauseContext struct {
	font *ot.Font
	run  runContext
}

func newPauseContext(font *ot.Font, run *runBuffer) pauseContext {
	return pauseContext{
		font: font,
		run:  newRunContext(run),
	}
}

func (pc pauseContext) Font() *ot.Font  { return pc.font }
func (pc pauseContext) Run() RunContext { return pc.run }

// planContext is an internal adapter implementing exported PlanContext.
type planContext struct {
	font      *ot.Font
	selection SelectionContext
	mask1     map[ot.Tag]uint32
	fallback  map[ot.Tag]bool
}

func newPlanContext(font *ot.Font, selection SelectionContext) planContext {
	return planContext{
		font:      font,
		selection: selection,
		mask1:     map[ot.Tag]uint32{},
		fallback:  map[ot.Tag]bool{},
	}
}

func (pc planContext) Font() *ot.Font {
	return pc.font
}

func (pc planContext) Selection() SelectionContext {
	return pc.selection
}

func (pc planContext) FeatureMask1(tag ot.Tag) uint32 {
	return pc.mask1[tag]
}

func (pc planContext) FeatureNeedsFallback(tag ot.Tag) bool {
	return pc.fallback[tag]
}

type normalizeContext struct {
	font        *ot.Font
	selection   SelectionContext
	hasGposMark bool
}

func newNormalizeContext(font *ot.Font, selection SelectionContext, hasGposMark bool) normalizeContext {
	return normalizeContext{
		font:        font,
		selection:   selection,
		hasGposMark: hasGposMark,
	}
}

func (nc normalizeContext) Font() *ot.Font {
	return nc.font
}

func (nc normalizeContext) Selection() SelectionContext {
	return nc.selection
}

func (nc normalizeContext) ComposeUnicode(a, b rune) (rune, bool) {
	s := norm.NFC.String(string([]rune{a, b}))
	r := []rune(s)
	if len(r) == 1 {
		return r[0], true
	}
	return 0, false
}

func (nc normalizeContext) HasGposMark() bool {
	return nc.hasGposMark
}
