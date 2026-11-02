package otshape

import (
	"math/rand/v2"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

type streamIngestor struct {
	st *streamingState
}

type planCompiler struct {
	params     Params
	ctx        SelectionContext
	engine     ShapingEngine
	randomSeed uint64
}

func newPlanCompiler(params Params, ctx SelectionContext, engine ShapingEngine) planCompiler {
	return planCompiler{
		params:     params,
		ctx:        ctx,
		engine:     engine,
		randomSeed: rand.Uint64(),
	}
}

func (pc planCompiler) compile(features []FeatureRange) (*plan, error) {
	pl, err := compileShapePlanWithFeatures(pc.params, pc.ctx, pc.engine, features)
	if pl != nil {
		pl.randomSeed = pc.randomSeed
	}
	return pl, err
}

func (pc planCompiler) compileDefault() (*plan, error) {
	return pc.compile(pc.params.Features)
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
	scopesByID map[uint16]featureSet,
) (int, error) {
	assert(in != nil, "stream ingestor is nil")
	return fillEventsUntilHighWatermark(src, in.st, stack, scopesByID)
}

func (in *streamIngestor) fillEventsLimit(
	src InputEventSource,
	stack *planStack,
	scopesByID map[uint16]featureSet,
	limit int,
) (int, error) {
	assert(in != nil, "stream ingestor is nil")
	return fillEventsUntilBufferLimit(src, in.st, stack, scopesByID, limit)
}

func (in *streamIngestor) compact(flushedCodepoints int) {
	assert(in != nil, "stream ingestor is nil")
	compactCarry(in.st, flushedCodepoints)
}

type shapeWorkspace struct {
	trace            *otlayout.StreamTrace
	err              error
	events, rootMark bool
	rawBase          uint32
	main             *runBuffer
	runes            []rune
	clusters         []uint32
	inputGroups      []uint32
	outputGroups     []uint32
	normRunesA       []rune
	normRunesB       []rune
	normClusA        []uint32
	normClusB        []uint32
}

func newShapeWorkspace(capHint int) *shapeWorkspace {
	if capHint < 16 {
		capHint = 16
	}
	return &shapeWorkspace{
		main: newRunBuffer(min(capHint, MaxIntermediateEntries)),
	}
}

func (ws *shapeWorkspace) copyRaw(st *streamingState) ([]rune, []uint32) {
	assert(ws != nil, "shape workspace is nil")
	assert(st != nil, "streaming state is nil")
	ws.runes = append(ws.runes[:0], st.rawRunes...)
	ws.clusters = append(ws.clusters[:0], st.rawClusters...)
	return ws.runes, ws.clusters
}

func (ws *shapeWorkspace) mapMain(runes []rune, clusters, sources []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	assert(ws != nil, "shape workspace is nil")
	ws.main = mapRunesToRunBufferInto(ws.main, runes, clusters, planIDs, font)
	copy(ws.main.SourceAnchors, sources)
	ws.main.stream, ws.main.streamEvents = ws.trace, ws.events
	ws.main.maxEntries = MaxIntermediateEntries
	if ws.trace != nil {
		ws.trace.Sources = make([]otlayout.SourceSpan, len(sources))
		for i, source := range sources {
			start := int(ws.inputGroups[int(source-ws.rawBase)] - ws.rawBase)
			end := start + 1
			for end < len(ws.inputGroups) && ws.inputGroups[end] == ws.inputGroups[start] {
				end++
			}
			ws.trace.Sources[i] = otlayout.SourceSpan{Start: start, End: end}
		}
	}
	return ws.main
}

func (ws *shapeWorkspace) normalize(
	runes []rune,
	clusters []uint32,
	font *ot.Font,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
) ([]rune, []uint32, []uint32) {
	assert(ws != nil, "shape workspace is nil")
	// Group before normalization: composition can consume a mark, while
	// decomposition can produce multiple glyphs from one original scalar.
	ws.inputGroups = formInputClusters(ws.inputGroups[:0], runes, clusters)
	ws.trace = otlayout.NewStreamTrace(len(runes))
	ws.trace.Disabled = !localStreamingEngine(engine, pl, ws.events, ws.rootMark)
	ws.err = nil
	if len(clusters) > 0 {
		ws.rawBase = clusters[0]
	}
	for i := 0; i < len(runes); {
		end := i + 1
		for end < len(runes) && ws.inputGroups[end] == ws.inputGroups[i] {
			end++
		}
		ws.trace.Join(i, end)
		if end == len(runes) {
			ws.trace.KeepFrom = i
		}
		i = end
	}
	work := &normalizationWork{trace: ws.trace, base: ws.rawBase}
	r, c, aR, aC, bR, bC := normalizeRuneStreamWithScratch(
		runes,
		clusters,
		font,
		ctx,
		engine,
		pl,
		ws.normRunesA[:0],
		ws.normClusA[:0],
		ws.normRunesB[:0],
		ws.normClusB[:0],
		work,
	)
	ws.err = work.err
	ws.normRunesA = aR
	ws.normClusA = aC
	ws.normRunesB = bR
	ws.normClusB = bC
	ws.outputGroups = resizeUint32(ws.outputGroups, len(c))
	for i, source := range c {
		// Raw carry and event spans contain consecutive original scalar indexes.
		ws.outputGroups[i] = ws.inputGroups[int(source-clusters[0])]
	}
	return r, ws.outputGroups, c
}
