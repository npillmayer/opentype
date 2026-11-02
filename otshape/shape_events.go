package otshape

import (
	"errors"
	"fmt"
	"io"
)

var (
	// ErrNilEventSource indicates that the event-based input source is nil.
	ErrNilEventSource = errors.New("otshape: nil input event source")
	// ErrEventIndexedFeatureRange indicates ShapeEvents forbids indexed
	// FeatureRange values (Start/End must both be 0).
	ErrEventIndexedFeatureRange = errors.New("otshape: ShapeEvents requires global-only FeatureRange values")
)

// ShapeEvents shapes src into sink according to params and bufOpts.
//
// Parameters:
//   - params selects font, segment metadata and global feature defaults.
//   - src emits rune and push/pop events incrementally until EOF.
//   - sink receives shaped glyph records in output order.
//   - bufOpts selects flush behavior and streaming thresholds.
//
// Returns nil on success, or an error for invalid inputs, source/sink failures,
// invalid event sequences, plan compilation failures, or pipeline failures.
//
// In ShapeEvents, params.Features is restricted to global defaults only:
// each FeatureRange must have Start==0 and End==0. Feature scoping is performed
// exclusively via InputEventPushFeatures/InputEventPopFeatures events. Scopes
// share normalization and lookup context within each buffered run. Settings at
// an initial base/mark group's first scalar apply to that entire group. Each
// carry uses one combined plan and mask budget, validated before shaping.
// A shaping error prevents emission of the current in-progress chunk. Previously
// emitted prefixes remain in sink; neither consumed input nor sink writes are
// rolled back. For atomic delivery, buffer output and commit only after success.
func (s *Shaper) ShapeEvents(params Params, src InputEventSource, sink GlyphSink, bufOpts BufferOptions) error {
	if params.Font == nil {
		return ErrNilFont
	}
	if src == nil {
		return ErrNilEventSource
	}
	if sink == nil {
		return ErrNilGlyphSink
	}
	if bufOpts.FlushBoundary == FlushExplicit {
		return ErrFlushExplicitUnsupported
	}
	if err := validateEventModeFeatures(params.Features); err != nil {
		return err
	}

	ctx := selectionContextFromParams(params)
	engine, err := selectShapingEngine(s.Engines, ctx)
	if err != nil {
		return err
	}
	compiler := newPlanCompiler(params, ctx, engine)

	rootFeatures := newFeatureSet(params.Features).asGlobalFeatureRanges()
	rootPlan, err := compiler.compile(rootFeatures)
	if err != nil {
		return err
	}
	cfg, err := resolveStreamingConfig(bufOpts)
	if err != nil {
		return err
	}
	ing := newStreamIngestor(cfg)
	st := ing.state()
	ws := newShapeWorkspace(cfg.highWatermark)
	ws.events = true
	ws.rootMark = planHasGposMark(rootPlan)
	stack := newPlanStack(rootFeatures)
	scopesByID := map[uint16]featureSet{stack.currentPlanID(): stack.currentFeatures()}

	for {
		if _, err := ing.fillEvents(src, stack, scopesByID); err != nil {
			return err
		}
		if len(st.rawRunes) == 0 {
			if st.eof {
				return stack.ensureClosed()
			}
			continue
		}

		run, err := shapeEventCarry(ws, st, compiler, rootPlan, scopesByID)
		if err != nil {
			return err
		}
		if run.Len() == 0 {
			if st.eof {
				ing.compact(len(st.rawRunes))
				return stack.ensureClosed()
			}
			// Match Shape: retain raw context until EOF is known, or fail at
			// the hard cap rather than silently discarding a provisional result.
			if len(st.rawRunes) >= st.cfg.maxBuffer {
				return ErrZeroOutputBufferFull
			}
			if _, err := ing.fillEventsLimit(src, stack, scopesByID, st.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}

		cut := findFlushCut(run, st)
		if !cut.ready {
			if len(st.rawRunes) == st.cfg.maxBuffer {
				return st.fullError()
			}
			if _, err := ing.fillEventsLimit(src, stack, scopesByID, st.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		assert(cut.glyphCut >= 0 && cut.glyphCut <= run.Len(), "flush decision glyph cut out of bounds")
		assert(cut.rawFlush >= 0 && cut.rawFlush <= len(st.rawRunes), "flush decision raw cut out of bounds")
		if cut.glyphCut == 0 {
			if _, err := ing.fillEventsLimit(src, stack, scopesByID, st.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		if err := writeRunBufferPrefixToSinkWithFont(run, sink, params.Font, bufOpts.FlushBoundary, cut.glyphCut); err != nil {
			return err
		}
		ing.compact(cut.rawFlush)
		if st.eof && len(st.rawRunes) == 0 {
			return stack.ensureClosed()
		}
	}
}

func validateEventModeFeatures(features []FeatureRange) error {
	for _, f := range features {
		if f.Feature == 0 {
			continue
		}
		if f.Start != 0 || f.End != 0 {
			return ErrEventIndexedFeatureRange
		}
	}
	return nil
}

func fillEventsUntilHighWatermark(
	src InputEventSource,
	st *streamingState,
	stack *planStack,
	scopesByID map[uint16]featureSet,
) (int, error) {
	return fillEventsUntilBufferLimit(src, st, stack, scopesByID, st.cfg.highWatermark)
}

func fillEventsUntilBufferLimit(
	src InputEventSource,
	st *streamingState,
	stack *planStack,
	scopesByID map[uint16]featureSet,
	limit int,
) (int, error) {
	assert(src != nil, "event source is nil")
	assert(st != nil, "streaming state is nil")
	assert(stack != nil, "plan stack is nil")
	assert(limit >= 0, "buffer fill limit must be >= 0")
	assert(limit <= st.cfg.maxBuffer, "buffer fill limit exceeds maxBuffer")
	st.assertInvariants()
	if st.eof {
		return 0, nil
	}
	read := 0
	for !st.eof && len(st.rawRunes) < limit {
		ev, err := src.ReadEvent()
		if err == io.EOF {
			st.eof = true
			break
		}
		if err != nil {
			return read, err
		}
		if err := ev.Validate(); err != nil {
			return read, err
		}
		switch ev.Kind {
		case InputEventRune:
			if err := st.checkInputRune(ev.Rune); err != nil {
				return read, err
			}
			st.rawRunes = append(st.rawRunes, ev.Rune)
			st.rawClusters = append(st.rawClusters, st.nextCluster)
			st.rawPlanIDs = append(st.rawPlanIDs, stack.currentPlanID())
			st.nextCluster++
			read++
		case InputEventPushFeatures:
			id, err := stack.push(ev.Push)
			if err != nil {
				return read, err
			}
			scopesByID[id] = stack.currentFeatures()
		case InputEventPopFeatures:
			if err := stack.pop(); err != nil {
				return read, err
			}
		default:
			return read, fmt.Errorf("otshape: unknown input event kind %d", ev.Kind)
		}
	}
	st.assertInvariants()
	return read, nil
}

// shapeEventCarry gives feature scopes one normalization/lookup context. A new
// engine instance is initialized for the combined plan, so span-local InitPlan
// state cannot leak from a later push into an earlier or restored scope.
func shapeEventCarry(ws *shapeWorkspace, st *streamingState, compiler planCompiler,
	rootPlan *plan, scopesByID map[uint16]featureSet) (*runBuffer, error) {
	st.assertInvariants()
	features, err := eventFeatureRanges(st, compiler.params.Features, rootPlan, scopesByID)
	if err != nil {
		return nil, err
	}
	engine := compiler.engine.New()
	if engine == nil {
		engine = compiler.engine
	}
	compiler.engine = engine
	pl, err := compiler.compile(features)
	if err != nil {
		return nil, err
	}
	runes, clusters := ws.copyRaw(st)
	runes, clusters, sources := ws.normalize(runes, clusters, compiler.params.Font, compiler.ctx, engine, pl)
	if ws.err != nil {
		return nil, ws.err
	}
	run := ws.mapMain(runes, clusters, sources, nil, compiler.params.Font)
	if err := shapeMappedRun(run, engine, pl); err != nil {
		return nil, err
	}
	return run, nil
}
