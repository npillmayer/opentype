package otshape

import (
	"errors"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

var (
	// ErrNoShaper indicates that no candidate shaping engine was supplied.
	ErrNoShaper = errors.New("otshape: no shaping engine supplied")
	// ErrNoMatchingShaper indicates that none of the supplied engines matched the segment context.
	ErrNoMatchingShaper = errors.New("otshape: no supplied shaping engine matches selection context")
	// ErrNilFont indicates that Params.Font is nil.
	ErrNilFont = errors.New("otshape: nil font in shape options")
	// ErrNilRuneSource indicates that the shape input source is nil.
	ErrNilRuneSource = errors.New("otshape: nil rune source")
	// ErrNilGlyphSink indicates that the shape output sink is nil.
	ErrNilGlyphSink = errors.New("otshape: nil glyph sink")
	// ErrFlushExplicitUnsupported indicates that FlushExplicit is not yet implemented.
	ErrFlushExplicitUnsupported = errors.New("otshape: FlushExplicit is not supported yet")
)

// Shaper is the injectable top-level shaping orchestrator.
//
// It intentionally has no global registry; callers provide candidate shapers.
type Shaper struct {
	Engines []ShapingEngine
}

// NewShaper creates a shaper from explicit candidate engines.
//
// Nil entries in shapers are ignored. The returned value keeps the candidate
// list and selects the best matching engine per [Shape] call.
func NewShaper(engines ...ShapingEngine) *Shaper {
	list := make([]ShapingEngine, 0, len(engines))
	for _, sh := range engines {
		if sh != nil {
			list = append(list, sh)
		}
	}
	return &Shaper{Engines: list}
}

// Shape shapes src into sink according to params and bufOpts.
//
// Parameters:
//   - params selects font, segment metadata and feature ranges.
//   - src is read incrementally until EOF.
//   - sink receives shaped glyph records in output order.
//   - bufOpts selects flush behavior and streaming thresholds.
//
// Returns nil on success, or an error for invalid inputs, source/sink failures,
// missing/invalid shaper selection, plan compilation failure, or pipeline failure.
func (s *Shaper) Shape(params Params, src RuneSource, sink GlyphSink, bufOpts BufferOptions) error {
	if params.Font == nil {
		return ErrNilFont
	}
	if src == nil {
		return ErrNilRuneSource
	}
	if sink == nil {
		return ErrNilGlyphSink
	}
	if bufOpts.FlushBoundary == FlushExplicit {
		return ErrFlushExplicitUnsupported
	}
	ctx := selectionContextFromParams(params)
	engine, err := selectShapingEngine(s.Engines, ctx)
	if err != nil {
		return err
	}
	compiler := newPlanCompiler(params, ctx, engine)

	plan, err := compiler.compileDefault()
	if err != nil {
		return err
	}
	cfg, err := resolveStreamingConfig(bufOpts)
	if err != nil {
		return err
	}
	ing := newStreamIngestor(cfg)
	strState := ing.state()
	ws := newShapeWorkspace(cfg.maxBuffer)

	for {
		if _, err := ing.fillRunes(src); err != nil {
			return err
		}
		if len(strState.rawRunes) == 0 {
			if strState.eof {
				return nil
			}
			continue
		}

		runes, clusters := ws.copyRaw(strState)
		runes, clusters = ws.normalize(runes, clusters, params.Font, ctx, engine, plan)
		run := ws.mapMain(runes, clusters, nil, params.Font)
		if run.Len() == 0 {
			ing.compact(len(strState.rawRunes))
			if strState.eof {
				return nil
			}
			continue
		}

		if err := shapeMappedRun(run, engine, plan); err != nil {
			return err
		}
		cut := findFlushCut(run, strState)
		if !cut.ready {
			if _, err := ing.fillRunesLimit(src, strState.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		assert(cut.glyphCut >= 0 && cut.glyphCut <= run.Len(), "flush decision glyph cut out of bounds")
		assert(cut.rawFlush >= 0 && cut.rawFlush <= len(strState.rawRunes), "flush decision raw cut out of bounds")
		if cut.glyphCut == 0 {
			// No flushable prefix yet; attempt to read more.
			if _, err := ing.fillRunesLimit(src, strState.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		if err := writeRunBufferPrefixToSinkWithFont(run, sink, params.Font, bufOpts.FlushBoundary, cut.glyphCut); err != nil {
			return err
		}
		ing.compact(cut.rawFlush)
		if strState.eof {
			if len(strState.rawRunes) == 0 {
				return nil
			}
		}
	}
}

func shapeMappedRun(run *runBuffer, engine ShapingEngine, pl *plan) error {
	if run == nil || run.Len() == 0 {
		return nil
	}
	rc := newRunContext(run)
	if hook, ok := engine.(ShapingEnginePreprocessHook); ok {
		hook.PreprocessRun(rc)
	}
	if hook, ok := engine.(ShapingEngineReorderHook); ok {
		hook.ReorderMarks(rc, 0, rc.Len())
	}
	if hook, ok := engine.(ShapingEnginePreGSUBHook); ok {
		hook.PrepareGSUB(rc)
	}

	exec := &planExecutor{}
	exec.acquireBuffer(run)
	defer exec.releaseBuffer()

	exec.ensureRunMasks(pl)
	if hook, ok := engine.(ShapingEngineMaskHook); ok {
		hook.SetupMasks(rc)
	}
	if err := exec.apply(pl); err != nil {
		return err
	}
	if hook, ok := engine.(ShapingEnginePostprocessHook); ok {
		hook.PostprocessRun(rc)
	}
	return nil
}

func writeRunBufferPrefixToSink(run *runBuffer, sink GlyphSink, boundary FlushBoundary, end int) error {
	return writeRunBufferPrefixToSinkWithFont(run, sink, nil, boundary, end)
}

func writeRunBufferPrefixToSinkWithFont(run *runBuffer, sink GlyphSink, font *ot.Font, boundary FlushBoundary, end int) error {
	if run == nil {
		return nil
	}
	n := run.Len()
	if end < 0 {
		end = 0
	}
	if end > n {
		end = n
	}
	if end == 0 {
		return nil
	}
	switch boundary {
	case FlushOnRunBoundary:
		return writeRunBufferRangeWithFont(run, sink, font, 0, end)
	case FlushOnClusterBoundary:
		for _, span := range clusterSpans(run) {
			if span.start >= end {
				break
			}
			if span.end > end {
				return errShaper("streaming prefix cut is not at a cluster boundary")
			}
			if err := writeRunBufferRangeWithFont(run, sink, font, span.start, span.end); err != nil {
				return err
			}
		}
		return nil
	case FlushExplicit:
		return ErrFlushExplicitUnsupported
	default:
		return writeRunBufferRangeWithFont(run, sink, font, 0, end)
	}
}

func selectionContextFromParams(params Params) SelectionContext {
	scriptTag := ScriptTagForScript(params.Script)
	langTag := LanguageTagForLanguage(params.Language, language.Low)
	return SelectionContext{
		Direction: params.Direction,
		Script:    params.Script,
		Language:  params.Language,
		ScriptTag: scriptTag,
		LangTag:   langTag,
	}
}

func selectShapingEngine(candidates []ShapingEngine, ctx SelectionContext) (ShapingEngine, error) {
	if len(candidates) == 0 {
		return nil, ErrNoShaper
	}
	var (
		best      ShapingEngine
		bestScore = ShaperConfidenceNone
	)
	for _, sh := range candidates {
		if sh == nil {
			continue
		}
		score := sh.Match(ctx)
		if score < 0 {
			continue
		}
		if best == nil || score > bestScore || (score == bestScore && sh.Name() < best.Name()) {
			best = sh
			bestScore = score
		}
	}
	if best == nil {
		return nil, ErrNoMatchingShaper
	}
	inst := best.New()
	if inst == nil {
		inst = best
	}
	return inst, nil
}

func compileShapePlanWithFeatures(params Params, ctx SelectionContext, engine ShapingEngine, features []FeatureRange) (*plan, error) {
	policy := planPolicy{
		ApplyGPOS: true,
	}
	if ep, ok := engine.(ShapingEnginePolicy); ok {
		policy.ApplyGPOS = ep.ApplyGPOS()
	}
	req := planRequest{
		Font:      params.Font,
		Props:     segmentProps{Direction: params.Direction, Script: params.Script, Language: params.Language},
		ScriptTag: ctx.ScriptTag,
		LangTag:   ctx.LangTag,
		Selection: ctx,
		Engine:    engine,
		Policy:    policy,
	}
	req.UserFeatures = append(req.UserFeatures, features...)
	return compile(req)
}

func mapRunesToRunBuffer(runes []rune, clusters []uint32, font *ot.Font) *runBuffer {
	return mapRunesToRunBufferWithPlanIDs(runes, clusters, nil, font)
}

func mapRunesToRunBufferWithPlanIDs(runes []rune, clusters []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	return mapRunesToRunBufferInto(newRunBuffer(len(runes)), runes, clusters, planIDs, font)
}

func mapRunesToRunBufferInto(run *runBuffer, runes []rune, clusters []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	if run == nil {
		run = newRunBuffer(len(runes))
	}
	withPlanIDs := len(planIDs) == len(runes)
	run.PrepareForMappedRun(withPlanIDs, len(runes))
	for i, r := range runes {
		gid := otquery.GlyphIndex(font, r)
		cluster := uint32(i)
		if len(clusters) == len(runes) {
			cluster = clusters[i]
		}
		planID := uint16(0)
		if withPlanIDs {
			planID = planIDs[i]
		}
		run.AppendMappedGlyph(gid, r, cluster, planID, withPlanIDs)
	}
	return run
}

func normalizeRuneStream(
	runes []rune,
	clusters []uint32,
	font *ot.Font,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
) ([]rune, []uint32) {
	runes, clusters, _, _, _, _ = normalizeRuneStreamWithScratch(
		runes,
		clusters,
		font,
		ctx,
		engine,
		pl,
		nil,
		nil,
		nil,
		nil,
	)
	return runes, clusters
}

func normalizeRuneStreamWithScratch(
	runes []rune,
	clusters []uint32,
	font *ot.Font,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
	tmpARunes []rune,
	tmpAClusters []uint32,
	tmpBRunes []rune,
	tmpBClusters []uint32,
) ([]rune, []uint32, []rune, []uint32, []rune, []uint32) {
	if len(runes) == 0 {
		return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
	}
	mode := NormalizationAuto
	if ep, ok := engine.(ShapingEnginePolicy); ok {
		mode = ep.NormalizationPreference()
	}
	if mode == NormalizationAuto {
		if prefersDecomposed(ctx.ScriptTag, ctx.LangTag) {
			mode = NormalizationDecomposed
		} else {
			mode = NormalizationComposed
		}
	}
	if mode == NormalizationNone {
		return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
	}

	if mode == NormalizationDecomposed {
		runes, clusters = decomposeRuneStreamInto(tmpARunes, tmpAClusters, runes, clusters)
		tmpARunes, tmpAClusters = runes, clusters
	}

	composeHook, hasComposeHook := engine.(ShapingEngineComposeHook)
	if !hasComposeHook && mode != NormalizationComposed {
		return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
	}
	nctx := newNormalizeContext(font, ctx, planHasGposMark(pl))
	runes, clusters = composeRuneStreamInto(
		tmpBRunes,
		tmpBClusters,
		runes,
		clusters,
		nctx,
		composeHook,
		hasComposeHook,
		mode == NormalizationComposed,
	)
	tmpBRunes, tmpBClusters = runes, clusters
	return runes, clusters, tmpARunes, tmpAClusters, tmpBRunes, tmpBClusters
}

func decomposeRuneStream(runes []rune, clusters []uint32) ([]rune, []uint32) {
	return decomposeRuneStreamInto(nil, nil, runes, clusters)
}

func decomposeRuneStreamInto(
	outRunes []rune,
	outClusters []uint32,
	runes []rune,
	clusters []uint32,
) ([]rune, []uint32) {
	outRunes = outRunes[:0]
	outClusters = outClusters[:0]
	for i, r := range runes {
		var cluster uint32
		if len(clusters) == len(runes) {
			cluster = clusters[i]
		} else {
			cluster = uint32(i)
		}
		s := norm.NFD.String(string(r))
		for _, dr := range s {
			outRunes = append(outRunes, dr)
			outClusters = append(outClusters, cluster)
		}
	}
	return outRunes, outClusters
}

func composeRuneStream(
	runes []rune,
	clusters []uint32,
	nctx normalizeContext,
	hook ShapingEngineComposeHook,
	hasHook bool,
	allowUnicode bool,
) ([]rune, []uint32) {
	return composeRuneStreamInto(nil, nil, runes, clusters, nctx, hook, hasHook, allowUnicode)
}

func composeRuneStreamInto(
	outRunes []rune,
	outClusters []uint32,
	runes []rune,
	clusters []uint32,
	nctx normalizeContext,
	hook ShapingEngineComposeHook,
	hasHook bool,
	allowUnicode bool,
) ([]rune, []uint32) {
	if len(runes) <= 1 {
		return runes, clusters
	}
	outRunes = outRunes[:0]
	outClusters = outClusters[:0]
	for i, r := range runes {
		cluster := uint32(i)
		if len(clusters) == len(runes) {
			cluster = clusters[i]
		}
		if len(outRunes) == 0 {
			outRunes = append(outRunes, r)
			outClusters = append(outClusters, cluster)
			continue
		}
		last := len(outRunes) - 1
		a := outRunes[last]
		if hasHook {
			if composed, ok := hook.Compose(nctx, a, r); ok {
				outRunes[last] = composed
				if cluster < outClusters[last] {
					outClusters[last] = cluster
				}
				continue
			}
		}
		if allowUnicode {
			if composed, ok := nctx.ComposeUnicode(a, r); ok {
				outRunes[last] = composed
				if cluster < outClusters[last] {
					outClusters[last] = cluster
				}
				continue
			}
		}
		outRunes = append(outRunes, r)
		outClusters = append(outClusters, cluster)
	}
	return outRunes, outClusters
}

func planHasGposMark(pl *plan) bool {
	if pl == nil {
		return false
	}
	if ms, ok := pl.maskForFeature(ot.T("mark")); ok && ms.Mask != 0 {
		return true
	}
	if ms, ok := pl.maskForFeature(ot.T("mkmk")); ok && ms.Mask != 0 {
		return true
	}
	return false
}

func writeRunBufferToSink(run *runBuffer, sink GlyphSink, boundary FlushBoundary) error {
	return writeRunBufferToSinkWithFont(run, sink, nil, boundary)
}

func writeRunBufferToSinkWithFont(run *runBuffer, sink GlyphSink, font *ot.Font, boundary FlushBoundary) error {
	switch boundary {
	case FlushOnRunBoundary:
		return writeRunBufferRangeWithFont(run, sink, font, 0, run.Len())
	case FlushOnClusterBoundary:
		for _, span := range clusterSpans(run) {
			if err := writeRunBufferRangeWithFont(run, sink, font, span.start, span.end); err != nil {
				return err
			}
		}
		return nil
	case FlushExplicit:
		return ErrFlushExplicitUnsupported
	default:
		return writeRunBufferRangeWithFont(run, sink, font, 0, run.Len())
	}
}

type runSpan struct {
	start int
	end   int
}

func clusterSpans(run *runBuffer) []runSpan {
	n := run.Len()
	if n == 0 {
		return nil
	}
	if len(run.Clusters) != n {
		return []runSpan{{start: 0, end: n}}
	}
	spans := make([]runSpan, 0, n)
	start := 0
	current := run.Clusters[0]
	for i := 1; i < n; i++ {
		if run.Clusters[i] == current {
			continue
		}
		spans = append(spans, runSpan{start: start, end: i})
		start = i
		current = run.Clusters[i]
	}
	spans = append(spans, runSpan{start: start, end: n})
	return spans
}

func writeRunBufferRange(run *runBuffer, sink GlyphSink, start int, end int) error {
	return writeRunBufferRangeWithFont(run, sink, nil, start, end)
}

func writeRunBufferRangeWithFont(run *runBuffer, sink GlyphSink, font *ot.Font, start int, end int) error {
	n := run.Len()
	if n == 0 || start >= end {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	hasPos := len(run.Pos) == n
	hasClusters := len(run.Clusters) == n
	hasMasks := len(run.Masks) == n
	hasUnsafe := len(run.UnsafeFlags) == n
	for i := start; i < end; i++ {
		record := materializeGlyphRecord(run, i, font, hasPos, hasClusters, hasMasks, hasUnsafe)
		if err := sink.WriteGlyph(record); err != nil {
			return err
		}
	}
	return nil
}

func materializeGlyphRecord(
	run *runBuffer,
	inx int,
	font *ot.Font,
	hasPos bool,
	hasClusters bool,
	hasMasks bool,
	hasUnsafe bool,
) GlyphRecord {
	record := GlyphRecord{GID: run.Glyphs[inx]}
	if hasPos {
		record.Pos = run.Pos[inx]
	}
	if font != nil {
		record.Pos.XAdvance += int32(otquery.GlyphMetrics(font, record.GID).Advance)
	}
	if hasClusters {
		record.Cluster = run.Clusters[inx]
	}
	if hasMasks {
		record.Mask = run.Masks[inx]
	}
	if hasUnsafe {
		record.UnsafeFlags = run.UnsafeFlags[inx]
	}
	return record
}
