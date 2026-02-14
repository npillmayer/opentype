package otshape

import "github.com/npillmayer/opentype/ot"

type streamIngestor struct {
	st *streamingState
}

type planCompiler struct {
	opts   ShapeOptions
	ctx    SelectionContext
	engine ShapingEngine
}

func newPlanCompiler(opts ShapeOptions, ctx SelectionContext, engine ShapingEngine) planCompiler {
	return planCompiler{
		opts:   opts,
		ctx:    ctx,
		engine: engine,
	}
}

func (pc planCompiler) compile(features []FeatureRange) (*plan, error) {
	return compileShapePlanWithFeatures(pc.opts, pc.ctx, pc.engine, features)
}

func (pc planCompiler) compileDefault() (*plan, error) {
	return pc.compile(pc.opts.Features)
}

func newStreamIngestor(cfg streamingConfig) *streamIngestor {
	return &streamIngestor{st: newStreamingState(cfg)}
}

func (in *streamIngestor) state() *streamingState {
	assert(in != nil, "stream ingestor is nil")
	return in.st
}

func (in *streamIngestor) fillRunes(src RuneSource) (int, error) {
	assert(in != nil, "stream ingestor is nil")
	return fillUntilHighWatermark(src, in.st)
}

func (in *streamIngestor) fillRunesLimit(src RuneSource, limit int) (int, error) {
	assert(in != nil, "stream ingestor is nil")
	return fillUntilBufferLimit(src, in.st, limit)
}

func (in *streamIngestor) fillEvents(
	src InputEventSource,
	stack *planStack,
	plansByID map[uint16]*plan,
	build func([]FeatureRange) (*plan, error),
) (int, error) {
	assert(in != nil, "stream ingestor is nil")
	return fillEventsUntilHighWatermark(src, in.st, stack, plansByID, build)
}

func (in *streamIngestor) fillEventsLimit(
	src InputEventSource,
	stack *planStack,
	plansByID map[uint16]*plan,
	build func([]FeatureRange) (*plan, error),
	limit int,
) (int, error) {
	assert(in != nil, "stream ingestor is nil")
	return fillEventsUntilBufferLimit(src, in.st, stack, plansByID, build, limit)
}

func (in *streamIngestor) compact(flushedCodepoints int) {
	assert(in != nil, "stream ingestor is nil")
	compactCarry(in.st, flushedCodepoints)
}

type shapeWorkspace struct {
	main        *runBuffer
	out         *runBuffer
	seg         *runBuffer
	runes       []rune
	clusters    []uint32
	planIDs     []uint16
	spanPlanIDs []uint16
	normRunesA  []rune
	normRunesB  []rune
	normClusA   []uint32
	normClusB   []uint32
}

func newShapeWorkspace(capHint int) *shapeWorkspace {
	if capHint < 16 {
		capHint = 16
	}
	return &shapeWorkspace{
		main: newRunBuffer(capHint),
		out:  newRunBuffer(capHint),
		seg:  newRunBuffer(capHint),
	}
}

func (ws *shapeWorkspace) copyRaw(st *streamingState) ([]rune, []uint32) {
	assert(ws != nil, "shape workspace is nil")
	assert(st != nil, "streaming state is nil")
	ws.runes = append(ws.runes[:0], st.rawRunes...)
	ws.clusters = append(ws.clusters[:0], st.rawClusters...)
	return ws.runes, ws.clusters
}

func (ws *shapeWorkspace) copyRawWithPlanIDs(st *streamingState) ([]rune, []uint32, []uint16) {
	assert(ws != nil, "shape workspace is nil")
	assert(st != nil, "streaming state is nil")
	ws.runes = append(ws.runes[:0], st.rawRunes...)
	ws.clusters = append(ws.clusters[:0], st.rawClusters...)
	ws.planIDs = append(ws.planIDs[:0], st.rawPlanIDs...)
	return ws.runes, ws.clusters, ws.planIDs
}

func (ws *shapeWorkspace) mapMain(runes []rune, clusters []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	assert(ws != nil, "shape workspace is nil")
	ws.main = mapRunesToRunBufferInto(ws.main, runes, clusters, planIDs, font)
	return ws.main
}

func (ws *shapeWorkspace) beginOut() *runBuffer {
	assert(ws != nil, "shape workspace is nil")
	ws.out.Reset()
	return ws.out
}

func (ws *shapeWorkspace) mapSegment(runes []rune, clusters []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	assert(ws != nil, "shape workspace is nil")
	ws.seg = mapRunesToRunBufferInto(ws.seg, runes, clusters, planIDs, font)
	return ws.seg
}

func (ws *shapeWorkspace) spanPlanIDsFor(pid uint16, n int) []uint16 {
	assert(ws != nil, "shape workspace is nil")
	if n <= 0 {
		ws.spanPlanIDs = ws.spanPlanIDs[:0]
		return ws.spanPlanIDs
	}
	if cap(ws.spanPlanIDs) < n {
		ws.spanPlanIDs = make([]uint16, n)
	}
	ws.spanPlanIDs = ws.spanPlanIDs[:n]
	for i := range ws.spanPlanIDs {
		ws.spanPlanIDs[i] = pid
	}
	return ws.spanPlanIDs
}

func (ws *shapeWorkspace) normalize(
	runes []rune,
	clusters []uint32,
	opts ShapeOptions,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
) ([]rune, []uint32) {
	assert(ws != nil, "shape workspace is nil")
	r, c, aR, aC, bR, bC := normalizeRuneStreamWithScratch(
		runes,
		clusters,
		opts,
		ctx,
		engine,
		pl,
		ws.normRunesA[:0],
		ws.normClusA[:0],
		ws.normRunesB[:0],
		ws.normClusB[:0],
	)
	ws.normRunesA = aR
	ws.normClusA = aC
	ws.normRunesB = bR
	ws.normClusB = bC
	return r, c
}
