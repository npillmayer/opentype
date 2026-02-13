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
	ErrNoShaper                   = errors.New("otshape: no shaping engine supplied")
	ErrNoMatchingShaper           = errors.New("otshape: no supplied shaping engine matches selection context")
	ErrNilFont                    = errors.New("otshape: nil font in shape options")
	ErrNilRuneSource              = errors.New("otshape: nil rune source")
	ErrNilGlyphSink               = errors.New("otshape: nil glyph sink")
	ErrFlushExplicitUnsupported   = errors.New("otshape: FlushExplicit is not supported yet")
	ErrShapePipelineUnimplemented = errors.New("otshape: streaming shape pipeline not implemented yet")
)

// ShapeRequest bundles all inputs needed for a streaming shape operation.
type ShapeRequest struct {
	Options ShapeOptions
	Source  RuneSource
	Sink    GlyphSink
	Shapers []ShapingEngine
}

// Shaper is the injectable top-level shaping orchestrator.
//
// It intentionally has no global registry; callers provide candidate shapers.
type Shaper struct {
	shapers []ShapingEngine
}

// NewShaper creates a shaping orchestrator from explicitly injected engines.
func NewShaper(shapers ...ShapingEngine) *Shaper {
	list := make([]ShapingEngine, 0, len(shapers))
	for _, sh := range shapers {
		if sh != nil {
			list = append(list, sh)
		}
	}
	return &Shaper{shapers: list}
}

// Shape is a convenience wrapper around NewShaper(...).Shape(...).
func Shape(req ShapeRequest) error {
	s := NewShaper(req.Shapers...)
	return s.Shape(req.Options, req.Source, req.Sink)
}

// Shape shapes a rune stream into a glyph stream for one request.
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

	pl, err := compileShapePlan(opts, ctx, engine)
	if err != nil {
		return err
	}

	runes, clusters, err := readRuneStream(src)
	if err != nil {
		return err
	}
	if len(runes) == 0 {
		return nil
	}
	runes, clusters = normalizeRuneStream(runes, clusters, opts, ctx, engine, pl)
	run := mapRunesToRunBuffer(runes, clusters, opts.Font)
	if run.Len() == 0 {
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
	return writeRunBufferToSink(run, sink, opts.FlushBoundary)
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
	req.UserFeatures = append(req.UserFeatures, opts.Features...)
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
	run := newRunBuffer(32)
	for i, r := range runes {
		gid := otquery.GlyphIndex(font, r)
		run.Glyphs = append(run.Glyphs, gid)
		if len(clusters) == len(runes) {
			run.Clusters = append(run.Clusters, clusters[i])
		} else {
			run.Clusters = append(run.Clusters, uint32(i))
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
