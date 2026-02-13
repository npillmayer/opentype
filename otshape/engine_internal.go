package otshape

import "github.com/npillmayer/opentype/ot"

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

func (rc runContext) Cluster(i int) uint32 {
	if rc.run == nil || i < 0 || i >= rc.run.Len() || len(rc.run.Clusters) != rc.run.Len() {
		return 0
	}
	return rc.run.Clusters[i]
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
	selection SelectionContext
	mask1     map[ot.Tag]uint32
	fallback  map[ot.Tag]bool
}

func newPlanContext(selection SelectionContext) planContext {
	return planContext{
		selection: selection,
		mask1:     map[ot.Tag]uint32{},
		fallback:  map[ot.Tag]bool{},
	}
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
