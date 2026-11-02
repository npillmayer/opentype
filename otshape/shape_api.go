package otshape

import (
	"errors"

	"github.com/npillmayer/opentype/ot"
	"golang.org/x/text/language"
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
//   - sink receives shaped glyph records in logical shaping order, also for RTL.
//   - bufOpts selects flush behavior and streaming thresholds.
//
// Returns nil on success, or an error for invalid inputs, source/sink failures,
// missing/invalid shaper selection, plan compilation failure, or pipeline failure.
// A shaping error prevents emission of the current in-progress chunk. Previously
// emitted prefixes remain in sink; neither consumed input nor sink writes are
// rolled back. For atomic delivery, buffer output and commit only after success.
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
		runes, clusters, sources := ws.normalize(runes, clusters, params.Font, ctx, engine, plan)
		run := ws.mapMain(runes, clusters, sources, nil, params.Font)
		if err := shapeMappedRun(run, engine, plan); err != nil {
			return err
		}
		if run.Len() == 0 {
			if strState.eof {
				ing.compact(len(strState.rawRunes))
				return nil
			}
			// Zero output is not proof that more input cannot affect shaping.
			if len(strState.rawRunes) >= strState.cfg.maxBuffer {
				return ErrZeroOutputBufferFull
			}
			if _, err := ing.fillRunesLimit(src, strState.cfg.maxBuffer); err != nil {
				return err
			}
			continue
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
	// Engines with a normalization hook already reordered before composition.
	// Keep the legacy post-normalization timing for all other engines.
	if _, early := engine.(ShapingEngineNormalizationReorderHook); !early {
		if hook, ok := engine.(ShapingEngineReorderHook); ok {
			hook.ReorderMarks(rc, 0, rc.Len())
		}
	}
	if hook, ok := engine.(ShapingEnginePreGSUBHook); ok {
		hook.PrepareGSUB(rc)
	}

	exec := &planExecutor{}
	exec.acquireBuffer(run)
	defer exec.releaseBuffer()

	// Prepare this fresh run once, before the engine customizes its masks.
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
		if score <= ShaperConfidenceNone {
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
	// This late advance policy is deliberately limited to the agreed scripts
	// and GDEF classification. Unicode fallback and other scripts are separate.
	policy.ZeroGDEFMarks = policy.ApplyGPOS && (ctx.ScriptTag == ot.T("latn") || ctx.ScriptTag == ot.T("hebr"))
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
