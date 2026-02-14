package otshape

import (
	"errors"
	"io"

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
	// ErrNilFont indicates that ShapeOptions.Font is nil.
	ErrNilFont = errors.New("otshape: nil font in shape options")
	// ErrNilRuneSource indicates that the shape input source is nil.
	ErrNilRuneSource = errors.New("otshape: nil rune source")
	// ErrNilGlyphSink indicates that the shape output sink is nil.
	ErrNilGlyphSink = errors.New("otshape: nil glyph sink")
	// ErrFlushExplicitUnsupported indicates that FlushExplicit is not yet implemented.
	ErrFlushExplicitUnsupported = errors.New("otshape: FlushExplicit is not supported yet")
	// ErrShapePipelineUnimplemented is reserved for incomplete pipeline phases.
	ErrShapePipelineUnimplemented = errors.New("otshape: streaming shape pipeline not implemented yet")
)

// ShapeRequest bundles all inputs required by [Shape].
type ShapeRequest struct {
	Options ShapeOptions    // Options configures script/language/font and streaming behavior.
	Source  RuneSource      // Source provides input runes.
	Sink    GlyphSink       // Sink receives shaped glyph records.
	Shapers []ShapingEngine // Shapers lists candidate engines used for selection.
}

// Shaper is the injectable top-level shaping orchestrator.
//
// It intentionally has no global registry; callers provide candidate shapers.
type Shaper struct {
	shapers []ShapingEngine
}

// NewShaper creates a shaper from explicit candidate engines.
//
// Nil entries in shapers are ignored. The returned value keeps the candidate
// list and selects the best matching engine per [Shape] call.
func NewShaper(shapers ...ShapingEngine) *Shaper {
	list := make([]ShapingEngine, 0, len(shapers))
	for _, sh := range shapers {
		if sh != nil {
			list = append(list, sh)
		}
	}
	return &Shaper{shapers: list}
}

// Shape shapes req.Source into req.Sink using req.Options and req.Shapers.
//
// Shape is a convenience wrapper around [NewShaper] followed by [Shaper.Shape].
// It returns the first source/sink/pipeline error encountered.
func Shape(req ShapeRequest) error {
	s := NewShaper(req.Shapers...)
	return s.Shape(req.Options, req.Source, req.Sink)
}

// Shape shapes src into sink according to opts.
//
// Parameters:
//   - opts selects font, segment metadata, feature ranges and streaming thresholds.
//   - src is read incrementally until EOF.
//   - sink receives shaped glyph records in output order.
//
// Returns nil on success, or an error for invalid inputs, source/sink failures,
// missing/invalid shaper selection, plan compilation failure, or pipeline failure.
func (s *Shaper) Shape(opts ShapeOptions, src RuneSource, sink GlyphSink) error {
	if opts.Font == nil {
		return ErrNilFont
	}
	if src == nil {
		return ErrNilRuneSource
	}
	if sink == nil {
		return ErrNilGlyphSink
	}
	if opts.FlushBoundary == FlushExplicit {
		return ErrFlushExplicitUnsupported
	}
	ctx := selectionContextFromOptions(opts)
	engine, err := selectShapingEngine(s.shapers, ctx)
	if err != nil {
		return err
	}

	plan, err := compileShapePlan(opts, ctx, engine)
	if err != nil {
		return err
	}
	cfg, err := resolveStreamingConfig(opts)
	if err != nil {
		return err
	}
	strState := newStreamingState(cfg)

	for {
		if _, err := fillUntilHighWatermark(src, strState); err != nil {
			return err
		}
		if len(strState.rawRunes) == 0 {
			if strState.eof {
				return nil
			}
			continue
		}

		runes := append([]rune(nil), strState.rawRunes...)
		clusters := append([]uint32(nil), strState.rawClusters...)
		runes, clusters = normalizeRuneStream(runes, clusters, opts, ctx, engine, plan)
		run := mapRunesToRunBuffer(runes, clusters, opts.Font)
		if run.Len() == 0 {
			compactCarry(strState, len(strState.rawRunes))
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
			if _, err := fillUntilBufferLimit(src, strState, strState.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		assert(cut.glyphCut >= 0 && cut.glyphCut <= run.Len(), "flush decision glyph cut out of bounds")
		assert(cut.rawFlush >= 0 && cut.rawFlush <= len(strState.rawRunes), "flush decision raw cut out of bounds")
		if cut.glyphCut == 0 {
			// No flushable prefix yet; attempt to read more.
			if _, err := fillUntilBufferLimit(src, strState, strState.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		if err := writeRunBufferPrefixToSink(run, sink, opts.FlushBoundary, cut.glyphCut); err != nil {
			return err
		}
		compactCarry(strState, cut.rawFlush)
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
		return writeRunBufferRange(run, sink, 0, end)
	case FlushOnClusterBoundary:
		for _, span := range clusterSpans(run) {
			if span.start >= end {
				break
			}
			if span.end > end {
				return errShaper("streaming prefix cut is not at a cluster boundary")
			}
			if err := writeRunBufferRange(run, sink, span.start, span.end); err != nil {
				return err
			}
		}
		return nil
	case FlushExplicit:
		return ErrFlushExplicitUnsupported
	default:
		return writeRunBufferRange(run, sink, 0, end)
	}
}

func selectionContextFromOptions(opts ShapeOptions) SelectionContext {
	scriptTag := ScriptTagForScript(opts.Script)
	langTag := LanguageTagForLanguage(opts.Language, language.Low)
	return SelectionContext{
		Direction: opts.Direction,
		Script:    opts.Script,
		Language:  opts.Language,
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

func compileShapePlan(opts ShapeOptions, ctx SelectionContext, engine ShapingEngine) (*plan, error) {
	return compileShapePlanWithFeatures(opts, ctx, engine, opts.Features)
}

func compileShapePlanWithFeatures(opts ShapeOptions, ctx SelectionContext, engine ShapingEngine, features []FeatureRange) (*plan, error) {
	policy := planPolicy{
		ApplyGPOS: true,
	}
	if ep, ok := engine.(ShapingEnginePolicy); ok {
		policy.ApplyGPOS = ep.ApplyGPOS()
	}
	req := planRequest{
		Font:      opts.Font,
		Props:     segmentProps{Direction: opts.Direction, Script: opts.Script, Language: opts.Language},
		ScriptTag: ctx.ScriptTag,
		LangTag:   ctx.LangTag,
		Selection: ctx,
		Engine:    engine,
		Policy:    policy,
	}
	req.UserFeatures = append(req.UserFeatures, features...)
	return compile(req)
}

func readRuneStream(src RuneSource) ([]rune, []uint32, error) {
	runes := make([]rune, 0, 32)
	clusters := make([]uint32, 0, 32)
	for cluster := uint32(0); ; cluster++ {
		r, _, err := src.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		runes = append(runes, r)
		clusters = append(clusters, cluster)
	}
	return runes, clusters, nil
}

func mapRunesToRunBuffer(runes []rune, clusters []uint32, font *ot.Font) *runBuffer {
	return mapRunesToRunBufferWithPlanIDs(runes, clusters, nil, font)
}

func mapRunesToRunBufferWithPlanIDs(runes []rune, clusters []uint32, planIDs []uint16, font *ot.Font) *runBuffer {
	run := newRunBuffer(32)
	for i, r := range runes {
		gid := otquery.GlyphIndex(font, r)
		run.Glyphs = append(run.Glyphs, gid)
		if len(clusters) == len(runes) {
			run.Clusters = append(run.Clusters, clusters[i])
		} else {
			run.Clusters = append(run.Clusters, uint32(i))
		}
		if len(planIDs) == len(runes) {
			run.PlanIDs = append(run.PlanIDs, planIDs[i])
		}
		run.Codepoints = append(run.Codepoints, r)
	}
	return run
}

func normalizeRuneStream(
	runes []rune,
	clusters []uint32,
	opts ShapeOptions,
	ctx SelectionContext,
	engine ShapingEngine,
	pl *plan,
) ([]rune, []uint32) {
	if len(runes) == 0 {
		return runes, clusters
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
		return runes, clusters
	}

	if mode == NormalizationDecomposed {
		runes, clusters = decomposeRuneStream(runes, clusters)
	}

	composeHook, hasComposeHook := engine.(ShapingEngineComposeHook)
	if !hasComposeHook && mode != NormalizationComposed {
		return runes, clusters
	}
	nctx := newNormalizeContext(opts.Font, ctx, planHasGposMark(pl))
	return composeRuneStream(runes, clusters, nctx, composeHook, hasComposeHook, mode == NormalizationComposed)
}

func decomposeRuneStream(runes []rune, clusters []uint32) ([]rune, []uint32) {
	outRunes := make([]rune, 0, len(runes))
	outClusters := make([]uint32, 0, len(clusters))
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
	if len(runes) <= 1 {
		return runes, clusters
	}
	outRunes := make([]rune, 0, len(runes))
	outClusters := make([]uint32, 0, len(clusters))
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
	switch boundary {
	case FlushOnRunBoundary:
		return writeRunBufferRange(run, sink, 0, run.Len())
	case FlushOnClusterBoundary:
		for _, span := range clusterSpans(run) {
			if err := writeRunBufferRange(run, sink, span.start, span.end); err != nil {
				return err
			}
		}
		return nil
	case FlushExplicit:
		return ErrFlushExplicitUnsupported
	default:
		return writeRunBufferRange(run, sink, 0, run.Len())
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
		record := GlyphRecord{GID: run.Glyphs[i]}
		if hasPos {
			record.Pos = run.Pos[i]
		}
		if hasClusters {
			record.Cluster = run.Clusters[i]
		}
		if hasMasks {
			record.Mask = run.Masks[i]
		}
		if hasUnsafe {
			record.UnsafeFlags = run.UnsafeFlags[i]
		}
		if err := sink.WriteGlyph(record); err != nil {
			return err
		}
	}
	return nil
}
