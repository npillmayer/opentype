package otshape

import (
	"math/rand/v2"
	"unicode"

	"github.com/npillmayer/opentype/ot"
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
	main         *runBuffer
	out          *runBuffer
	seg          *runBuffer
	runes        []rune
	clusters     []uint32
	inputGroups  []uint32
	outputGroups []uint32
	planIDs      []uint16
	spanPlanIDs  []uint16
	normRunesA   []rune
	normRunesB   []rune
	normClusA    []uint32
	normClusB    []uint32
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

func (ws *shapeWorkspace) mapMain(runes []rune, clusters, sources []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	assert(ws != nil, "shape workspace is nil")
	ws.main = mapRunesToRunBufferInto(ws.main, runes, clusters, planIDs, font)
	copy(ws.main.SourceAnchors, sources)
	return ws.main
}

func (ws *shapeWorkspace) beginOut() *runBuffer {
	assert(ws != nil, "shape workspace is nil")
	ws.out.Reset()
	return ws.out
}

func (ws *shapeWorkspace) mapSegment(runes []rune, clusters, sources []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	assert(ws != nil, "shape workspace is nil")
	ws.seg = mapRunesToRunBufferInto(ws.seg, runes, clusters, planIDs, font)
	copy(ws.seg.SourceAnchors, sources)
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
	font *ot.Font,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
) ([]rune, []uint32, []uint32) {
	assert(ws != nil, "shape workspace is nil")
	// Group before normalization: composition can consume a mark, while
	// decomposition can produce multiple glyphs from one original scalar.
	ws.inputGroups = formInputClusters(ws.inputGroups[:0], runes, clusters)
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
	)
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

// formInputClusters implements the shared base/mark grouping needed by Latin
// and Hebrew at cluster level 0. A leading mark starts its own group; following
// marks (including variation selectors) and ZWJ continue the preceding group.
// ZWNJ remains a boundary. Script-specific and emoji sequence rules are separate.
func formInputClusters(dst []uint32, runes []rune, sources []uint32) []uint32 {
	for i, r := range runes {
		cluster := sources[i]
		if i > 0 && (unicode.Is(unicode.M, r) || r == '\u200D') {
			cluster = dst[i-1]
		}
		dst = append(dst, cluster)
	}
	return dst
}

// mergeClusterRange includes adjacent members of either boundary group. A
// substitution or reorder must not split an existing base/mark association.
// SourceAnchors and already-assigned feature masks are intentionally unchanged.
func mergeClusterRange(clusters []uint32, start, end int) {
	if start >= end {
		return
	}
	for start > 0 && clusters[start-1] == clusters[start] {
		start--
	}
	for end < len(clusters) && clusters[end] == clusters[end-1] {
		end++
	}
	minimum := clusters[start]
	for _, c := range clusters[start:end] {
		if c < minimum {
			minimum = c
		}
	}
	for i := start; i < end; i++ {
		clusters[i] = minimum
	}
}
