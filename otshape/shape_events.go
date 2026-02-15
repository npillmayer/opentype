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

// ShapeEventsRequest bundles all inputs required by [ShapeEvents].
//
// Deprecated: use [ShapeEvents] or [Shaper.ShapeEvents] directly with
// explicit arguments (`Params`, `InputEventSource`, `GlyphSink`, `BufferOptions`).
type ShapeEventsRequest struct {
	Options BufferOptions // Options configures buffering/flush behavior.
	Source  InputEventSource
	Sink    GlyphSink
	Shapers []ShapingEngine
}

// ShapeEvents shapes src into sink using params.
//
// ShapeEvents is the event-stream counterpart of [Shape]. It consumes explicit
// rune/push/pop events from [InputEventSource], compiles nested plans on demand,
// and shapes buffered spans with explicit plan boundaries.
func ShapeEvents(params Params, src InputEventSource, sink GlyphSink, engines ...ShapingEngine) error {
	s := NewShaper(engines...)
	bufOpts := BufferOptions{
		FlushBoundary: FlushOnRunBoundary,
		HighWatermark: defaultHighWatermark,
		LowWatermark:  defaultLowWatermark,
		MaxBuffer:     defaultMaxBuffer,
	}
	return s.ShapeEvents(params, src, sink, bufOpts)
}

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
// exclusively via InputEventPushFeatures/InputEventPopFeatures events.
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
	ws := newShapeWorkspace(cfg.maxBuffer)
	stack := newPlanStack(rootFeatures, rootPlan)
	plansByID := map[uint16]*plan{
		stack.currentPlanID(): rootPlan,
	}
	build := func(features []FeatureRange) (*plan, error) {
		return compiler.compile(features)
	}

	for {
		if _, err := ing.fillEvents(src, stack, plansByID, build); err != nil {
			return err
		}
		if len(st.rawRunes) == 0 {
			if st.eof {
				return stack.ensureClosed()
			}
			continue
		}

		run, err := shapeEventCarry(ws, st, params, ctx, engine, plansByID)
		if err != nil {
			return err
		}
		if run.Len() == 0 {
			ing.compact(len(st.rawRunes))
			if st.eof {
				return stack.ensureClosed()
			}
			continue
		}

		cut := findFlushCut(run, st)
		if !cut.ready {
			if _, err := ing.fillEventsLimit(src, stack, plansByID, build, st.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		assert(cut.glyphCut >= 0 && cut.glyphCut <= run.Len(), "flush decision glyph cut out of bounds")
		assert(cut.rawFlush >= 0 && cut.rawFlush <= len(st.rawRunes), "flush decision raw cut out of bounds")
		if cut.glyphCut == 0 {
			if _, err := ing.fillEventsLimit(src, stack, plansByID, build, st.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		if err := writeRunBufferPrefixToSink(run, sink, bufOpts.FlushBoundary, cut.glyphCut); err != nil {
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
	plansByID map[uint16]*plan,
	build func([]FeatureRange) (*plan, error),
) (int, error) {
	return fillEventsUntilBufferLimit(src, st, stack, plansByID, build, st.cfg.highWatermark)
}

func fillEventsUntilBufferLimit(
	src InputEventSource,
	st *streamingState,
	stack *planStack,
	plansByID map[uint16]*plan,
	build func([]FeatureRange) (*plan, error),
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
			st.rawRunes = append(st.rawRunes, ev.Rune)
			st.rawClusters = append(st.rawClusters, st.nextCluster)
			st.rawPlanIDs = append(st.rawPlanIDs, stack.currentPlanID())
			st.nextCluster++
			read++
		case InputEventPushFeatures:
			id, err := stack.push(ev.Push, build)
			if err != nil {
				return read, err
			}
			plansByID[id] = stack.currentPlan()
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

func shapeEventCarry(
	ws *shapeWorkspace,
	st *streamingState,
	params Params,
	ctx SelectionContext,
	engine ShapingEngine,
	plansByID map[uint16]*plan,
) (*runBuffer, error) {
	assert(ws != nil, "shape workspace is nil")
	assert(st != nil, "streaming state is nil")
	st.assertInvariants()
	if len(st.rawRunes) == 0 {
		return ws.beginOut(), nil
	}
	if len(st.rawPlanIDs) != len(st.rawRunes) {
		return nil, errShaper("event carry plan-id alignment mismatch")
	}
	runes, clusters, planIDs := ws.copyRawWithPlanIDs(st)

	out := ws.beginOut()
	for start := 0; start < len(runes); {
		end := start + 1
		pid := planIDs[start]
		for end < len(runes) && planIDs[end] == pid {
			end++
		}
		pl := plansByID[pid]
		if pl == nil {
			return nil, errShaper("missing compiled plan for event span")
		}
		segRunes := runes[start:end]
		segClusters := clusters[start:end]
		segRunes, segClusters = ws.normalize(segRunes, segClusters, params.Font, ctx, engine, pl)
		if len(segRunes) == 0 {
			start = end
			continue
		}
		segPlanIDs := ws.spanPlanIDsFor(pid, len(segRunes))
		segRun := ws.mapSegment(segRunes, segClusters, segPlanIDs, params.Font)
		if err := shapeMappedRun(segRun, engine, pl); err != nil {
			return nil, err
		}
		out.AppendRun(segRun)
		start = end
	}
	return out, nil
}
