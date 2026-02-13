package otshape

import (
	"errors"
	"io"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/language"
)

var (
	ErrNoShaper                   = errors.New("otshape: no shaping engine supplied")
	ErrNoMatchingShaper           = errors.New("otshape: no supplied shaping engine matches selection context")
	ErrNilFont                    = errors.New("otshape: nil font in shape options")
	ErrNilRuneSource              = errors.New("otshape: nil rune source")
	ErrNilGlyphSink               = errors.New("otshape: nil glyph sink")
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
	ctx := selectionContextFromOptions(opts)
	engine, err := selectShapingEngine(s.shapers, ctx)
	if err != nil {
		return err
	}

	run, err := readRunesToRunBuffer(src, opts.Font)
	if err != nil {
		return err
	}
	if run.Len() == 0 {
		return nil
	}

	pl, err := compileShapePlan(opts, ctx, engine)
	if err != nil {
		return err
	}

	rc := newRunContext(run)
	if hook, ok := engine.(ShapingEnginePreprocessHook); ok {
		hook.PreprocessRun(rc)
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
	return writeRunBufferToSink(run, sink)
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
		Policy:    policy,
	}
	req.UserFeatures = append(req.UserFeatures, opts.Features...)
	return compile(req)
}

func readRunesToRunBuffer(src RuneSource, font *ot.Font) (*runBuffer, error) {
	run := newRunBuffer(32)
	for cluster := uint32(0); ; cluster++ {
		r, _, err := src.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		gid := otquery.GlyphIndex(font, r)
		run.Glyphs = append(run.Glyphs, gid)
		run.Clusters = append(run.Clusters, cluster)
	}
	return run, nil
}

func writeRunBufferToSink(run *runBuffer, sink GlyphSink) error {
	n := run.Len()
	if n == 0 {
		return nil
	}
	hasPos := len(run.Pos) == n
	hasClusters := len(run.Clusters) == n
	hasMasks := len(run.Masks) == n
	hasUnsafe := len(run.UnsafeFlags) == n
	for i := 0; i < n; i++ {
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
