package otshape

import (
	"errors"
	"fmt"
	"io"

	"github.com/npillmayer/opentype/otlayout"
)

var (
	// ErrNilEventSource indicates that the event-based input source is nil.
	ErrNilEventSource = errors.New("otshape: nil input event source")
	// ErrEventIndexedFeatureRange indicates event mode only supports global feature defaults.
	ErrEventIndexedFeatureRange = errors.New("otshape: event input mode does not support indexed FeatureRange values")
)

// ShapeEventsRequest bundles all inputs required by [ShapeEvents].
type ShapeEventsRequest struct {
	Options ShapeOptions // Options configures script/language/font and streaming behavior.
	Source  InputEventSource
	Sink    GlyphSink
	Shapers []ShapingEngine
}

// ShapeEvents shapes req.Source into req.Sink using req.Options and req.Shapers.
//
// ShapeEvents is the event-stream counterpart of [Shape]. It consumes explicit
// rune/push/pop events from [InputEventSource], compiles nested plans on demand,
// and shapes buffered spans with explicit plan boundaries.
func ShapeEvents(req ShapeEventsRequest) error {
	s := NewShaper(req.Shapers...)
	return s.ShapeEvents(req.Options, req.Source, req.Sink)
}

// ShapeEvents shapes an [InputEventSource] into sink according to opts.
//
// Parameters:
//   - opts selects font, segment metadata, global feature defaults and streaming thresholds.
//   - src emits rune and push/pop events incrementally until EOF.
//   - sink receives shaped glyph records in output order.
//
// Returns nil on success, or an error for invalid inputs, source/sink failures,
// invalid event sequences, plan compilation failures, or pipeline failures.
func (s *Shaper) ShapeEvents(opts ShapeOptions, src InputEventSource, sink GlyphSink) error {
	if opts.Font == nil {
		return ErrNilFont
	}
	if src == nil {
		return ErrNilEventSource
	}
	if sink == nil {
		return ErrNilGlyphSink
	}
	if opts.FlushBoundary == FlushExplicit {
		return ErrFlushExplicitUnsupported
	}
	if err := validateEventModeFeatures(opts.Features); err != nil {
		return err
	}

	ctx := selectionContextFromOptions(opts)
	engine, err := selectShapingEngine(s.shapers, ctx)
	if err != nil {
		return err
	}

	rootFeatures := newFeatureSet(opts.Features).asGlobalFeatureRanges()
	rootPlan, err := compileShapePlanWithFeatures(opts, ctx, engine, rootFeatures)
	if err != nil {
		return err
	}
	cfg, err := resolveStreamingConfig(opts)
	if err != nil {
		return err
	}
	st := newStreamingState(cfg)
	stack := newPlanStack(rootFeatures, rootPlan)
	plansByID := map[uint16]*plan{
		stack.currentPlanID(): rootPlan,
	}
	build := func(features []FeatureRange) (*plan, error) {
		return compileShapePlanWithFeatures(opts, ctx, engine, features)
	}

	for {
		if _, err := fillEventsUntilHighWatermark(src, st, stack, plansByID, build); err != nil {
			return err
		}
		if len(st.rawRunes) == 0 {
			if st.eof {
				return stack.ensureClosed()
			}
			continue
		}

		run, err := shapeEventCarry(st, opts, ctx, engine, plansByID)
		if err != nil {
			return err
		}
		if run.Len() == 0 {
			compactCarry(st, len(st.rawRunes))
			if st.eof {
				return stack.ensureClosed()
			}
			continue
		}

		cut := findFlushCut(run, st)
		if !cut.ready {
			if _, err := fillEventsUntilBufferLimit(src, st, stack, plansByID, build, st.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		assert(cut.glyphCut >= 0 && cut.glyphCut <= run.Len(), "flush decision glyph cut out of bounds")
		assert(cut.rawFlush >= 0 && cut.rawFlush <= len(st.rawRunes), "flush decision raw cut out of bounds")
		if cut.glyphCut == 0 {
			if _, err := fillEventsUntilBufferLimit(src, st, stack, plansByID, build, st.cfg.maxBuffer); err != nil {
				return err
			}
			continue
		}
		if err := writeRunBufferPrefixToSink(run, sink, opts.FlushBoundary, cut.glyphCut); err != nil {
			return err
		}
		compactCarry(st, cut.rawFlush)
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
	st *streamingState,
	opts ShapeOptions,
	ctx SelectionContext,
	engine ShapingEngine,
	plansByID map[uint16]*plan,
) (*runBuffer, error) {
	assert(st != nil, "streaming state is nil")
	st.assertInvariants()
	if len(st.rawRunes) == 0 {
		return newRunBuffer(0), nil
	}
	if len(st.rawPlanIDs) != len(st.rawRunes) {
		return nil, errShaper("event carry plan-id alignment mismatch")
	}
	runes := append([]rune(nil), st.rawRunes...)
	clusters := append([]uint32(nil), st.rawClusters...)
	planIDs := append([]uint16(nil), st.rawPlanIDs...)

	out := newRunBuffer(len(runes))
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
		segRunes := append([]rune(nil), runes[start:end]...)
		segClusters := append([]uint32(nil), clusters[start:end]...)
		segRunes, segClusters = normalizeRuneStream(segRunes, segClusters, opts, ctx, engine, pl)
		if len(segRunes) == 0 {
			start = end
			continue
		}
		segPlanIDs := make([]uint16, len(segRunes))
		for i := range segPlanIDs {
			segPlanIDs[i] = pid
		}
		segRun := mapRunesToRunBufferWithPlanIDs(segRunes, segClusters, segPlanIDs, opts.Font)
		if err := shapeMappedRun(segRun, engine, pl); err != nil {
			return nil, err
		}
		appendRunBuffer(out, segRun)
		start = end
	}
	return out, nil
}

func appendRunBuffer(dst *runBuffer, src *runBuffer) {
	if dst == nil || src == nil {
		return
	}
	srcLen := src.Len()
	if srcLen == 0 {
		return
	}
	dstLen := dst.Len()
	dst.Glyphs = append(dst.Glyphs, src.Glyphs...)

	appendRunesAligned(&dst.Codepoints, dstLen, src.Codepoints, srcLen)
	appendUint32Aligned(&dst.Clusters, dstLen, src.Clusters, srcLen)
	appendUint16Aligned(&dst.PlanIDs, dstLen, src.PlanIDs, srcLen)
	appendUint32Aligned(&dst.Masks, dstLen, src.Masks, srcLen)
	appendUint16Aligned(&dst.UnsafeFlags, dstLen, src.UnsafeFlags, srcLen)
	appendUint16Aligned(&dst.Syllables, dstLen, src.Syllables, srcLen)
	appendUint8Aligned(&dst.Joiners, dstLen, src.Joiners, srcLen)
	appendPosAligned(&dst.Pos, dstLen, src.Pos, srcLen)
}

func appendRunesAligned(dst *[]rune, dstLen int, src []rune, srcLen int) {
	hasSrc := len(src) == srcLen
	if *dst == nil && !hasSrc {
		return
	}
	if *dst == nil {
		*dst = make([]rune, dstLen)
	}
	if hasSrc {
		*dst = append(*dst, src...)
		return
	}
	*dst = append(*dst, make([]rune, srcLen)...)
}

func appendUint32Aligned(dst *[]uint32, dstLen int, src []uint32, srcLen int) {
	hasSrc := len(src) == srcLen
	if *dst == nil && !hasSrc {
		return
	}
	if *dst == nil {
		*dst = make([]uint32, dstLen)
	}
	if hasSrc {
		*dst = append(*dst, src...)
		return
	}
	*dst = append(*dst, make([]uint32, srcLen)...)
}

func appendUint16Aligned(dst *[]uint16, dstLen int, src []uint16, srcLen int) {
	hasSrc := len(src) == srcLen
	if *dst == nil && !hasSrc {
		return
	}
	if *dst == nil {
		*dst = make([]uint16, dstLen)
	}
	if hasSrc {
		*dst = append(*dst, src...)
		return
	}
	*dst = append(*dst, make([]uint16, srcLen)...)
}

func appendUint8Aligned(dst *[]uint8, dstLen int, src []uint8, srcLen int) {
	hasSrc := len(src) == srcLen
	if *dst == nil && !hasSrc {
		return
	}
	if *dst == nil {
		*dst = make([]uint8, dstLen)
	}
	if hasSrc {
		*dst = append(*dst, src...)
		return
	}
	*dst = append(*dst, make([]uint8, srcLen)...)
}

func appendPosAligned(dst *otlayout.PosBuffer, dstLen int, src otlayout.PosBuffer, srcLen int) {
	hasSrc := len(src) == srcLen
	if *dst == nil && !hasSrc {
		return
	}
	if *dst == nil {
		*dst = defaultPosBuffer(dstLen)
	}
	if hasSrc {
		*dst = append(*dst, src...)
		return
	}
	*dst = append(*dst, defaultPosBuffer(srcLen)...)
}

func defaultPosBuffer(n int) otlayout.PosBuffer {
	if n <= 0 {
		return nil
	}
	out := otlayout.NewPosBuffer(n)
	for i := range out {
		out[i].AttachTo = -1
	}
	return out
}
