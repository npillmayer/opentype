package otshape

import (
	"fmt"
	"io"
)

const (
	defaultHighWatermark = 3500
	defaultLowWatermark  = 1024
	defaultMaxBuffer     = 4096
)

const (
	unsafeFlagToBreak  uint16 = 1 << 0
	unsafeFlagToConcat uint16 = 1 << 1
	unsafeCutMask             = unsafeFlagToBreak | unsafeFlagToConcat
)

// streamingConfig controls buffering thresholds for incremental shaping.
type streamingConfig struct {
	highWatermark int
	lowWatermark  int
	maxBuffer     int
}

func (cfg streamingConfig) valid() bool {
	return cfg.highWatermark > 0 && cfg.lowWatermark >= 0 && cfg.lowWatermark <= cfg.highWatermark && cfg.maxBuffer >= cfg.highWatermark
}

func resolveStreamingConfig(opts BufferOptions) (streamingConfig, error) {
	if opts.HighWatermark < 0 || opts.LowWatermark < 0 || opts.MaxBuffer < 0 {
		return streamingConfig{}, fmt.Errorf("otshape: streaming watermarks must be >= 0")
	}
	cfg := streamingConfig{
		highWatermark: defaultHighWatermark,
		lowWatermark:  defaultLowWatermark,
		maxBuffer:     defaultMaxBuffer,
	}
	if opts.HighWatermark > 0 {
		cfg.highWatermark = opts.HighWatermark
	}
	if opts.LowWatermark > 0 {
		cfg.lowWatermark = opts.LowWatermark
	} else if cfg.lowWatermark > cfg.highWatermark {
		cfg.lowWatermark = cfg.highWatermark
	}
	if opts.MaxBuffer > 0 {
		cfg.maxBuffer = opts.MaxBuffer
	} else if cfg.maxBuffer < cfg.highWatermark {
		cfg.maxBuffer = cfg.highWatermark
	}
	if !cfg.valid() {
		return streamingConfig{}, fmt.Errorf("otshape: invalid streaming watermark config high=%d low=%d max=%d", cfg.highWatermark, cfg.lowWatermark, cfg.maxBuffer)
	}
	return cfg, nil
}

// streamingState holds in-flight source content not yet flushed to sink.
type streamingState struct {
	rawRunes    []rune
	rawClusters []uint32
	rawPlanIDs  []uint16
	nextCluster uint32
	eof         bool
	cfg         streamingConfig
}

func newStreamingState(cfg streamingConfig) *streamingState {
	assert(cfg.valid(), "invalid streaming config")
	capHint := cfg.highWatermark
	if capHint < 16 {
		capHint = 16
	}
	return &streamingState{
		rawRunes:    make([]rune, 0, capHint),
		rawClusters: make([]uint32, 0, capHint),
		cfg:         cfg,
	}
}

func (st *streamingState) assertInvariants() {
	assert(st != nil, "streaming state is nil")
	assert(st.cfg.valid(), "invalid streaming config in state")
	assert(len(st.rawRunes) == len(st.rawClusters), "raw rune/cluster arrays out of alignment")
	if len(st.rawPlanIDs) != 0 {
		assert(len(st.rawPlanIDs) == len(st.rawRunes), "raw rune/plan arrays out of alignment")
	}
	assert(len(st.rawRunes) <= st.cfg.maxBuffer, "raw buffer exceeds maxBuffer")
	for i := 1; i < len(st.rawClusters); i++ {
		assert(st.rawClusters[i] > st.rawClusters[i-1], "raw clusters are not strictly monotonic")
	}
	if n := len(st.rawClusters); n > 0 {
		assert(st.nextCluster > st.rawClusters[n-1], "nextCluster must be greater than the tail cluster")
	}
}

// fillUntilHighWatermark reads runes until high-watermark or EOF is reached.
func fillUntilHighWatermark(src RuneSource, st *streamingState) (int, error) {
	return fillUntilBufferLimit(src, st, st.cfg.highWatermark)
}

func fillUntilBufferLimit(src RuneSource, st *streamingState, limit int) (int, error) {
	assert(src != nil, "rune source is nil")
	assert(st != nil, "streaming state is nil")
	assert(limit >= 0, "buffer fill limit must be >= 0")
	assert(limit <= st.cfg.maxBuffer, "buffer fill limit exceeds maxBuffer")
	st.assertInvariants()
	if st.eof {
		return 0, nil
	}
	read := 0
	for !st.eof && len(st.rawRunes) < limit {
		r, _, err := src.ReadRune()
		if err == io.EOF {
			st.eof = true
			break
		}
		if err != nil {
			return read, err
		}
		st.rawRunes = append(st.rawRunes, r)
		st.rawClusters = append(st.rawClusters, st.nextCluster)
		st.nextCluster++
		read++
	}
	st.assertInvariants()
	return read, nil
}

// compactCarry drops the first flushed codepoints from the raw carry buffer.
func compactCarry(st *streamingState, flushedCodepoints int) {
	assert(st != nil, "streaming state is nil")
	st.assertInvariants()
	assert(flushedCodepoints >= 0, "flushed codepoint count must be >= 0")
	assert(flushedCodepoints <= len(st.rawRunes), "flushed codepoint count exceeds carry length")
	if flushedCodepoints == 0 {
		return
	}
	st.rawRunes = append(st.rawRunes[:0], st.rawRunes[flushedCodepoints:]...)
	st.rawClusters = append(st.rawClusters[:0], st.rawClusters[flushedCodepoints:]...)
	if len(st.rawPlanIDs) != 0 {
		st.rawPlanIDs = append(st.rawPlanIDs[:0], st.rawPlanIDs[flushedCodepoints:]...)
	}
	st.assertInvariants()
}

type flushDecision struct {
	glyphCut int
	rawFlush int
	ready    bool
}

func findFlushCut(run *runBuffer, st *streamingState) flushDecision {
	assert(run != nil, "run buffer is nil")
	assert(st != nil, "streaming state is nil")
	st.assertInvariants()
	if run.Len() == 0 || len(st.rawRunes) == 0 {
		return flushDecision{}
	}
	if st.eof {
		return flushDecision{
			glyphCut: run.Len(),
			rawFlush: len(st.rawRunes),
			ready:    true,
		}
	}
	low := st.cfg.lowWatermark
	if low > len(st.rawRunes) {
		low = len(st.rawRunes)
	}
	if run.Len() != len(st.rawRunes) {
		// Length-changing transformations (compose/ligature expansion/deletion) do
		// not preserve enough provenance for safe partial raw compaction.
		// Defer partial flushes and rely on EOF/maxBuffer full flush instead.
		if len(st.rawRunes) >= st.cfg.maxBuffer {
			return flushDecision{
				glyphCut: run.Len(),
				rawFlush: len(st.rawRunes),
				ready:    true,
			}
		}
		return flushDecision{}
	}
	rawStart := st.rawClusters[0]
	for _, span := range clusterSpans(run) {
		if !isBreakSafeCut(run, span.end) {
			continue
		}
		rawFlush, ok := rawFlushForGlyphPrefix(run, span.end, rawStart, len(st.rawRunes))
		if !ok {
			continue
		}
		if rawFlush >= low {
			return flushDecision{
				glyphCut: span.end,
				rawFlush: rawFlush,
				ready:    true,
			}
		}
	}
	if len(st.rawRunes) >= st.cfg.maxBuffer {
		return flushDecision{
			glyphCut: run.Len(),
			rawFlush: len(st.rawRunes),
			ready:    true,
		}
	}
	return flushDecision{}
}

func isBreakSafeCut(run *runBuffer, cut int) bool {
	assert(run != nil, "run buffer is nil")
	n := run.Len()
	if cut <= 0 || cut >= n {
		return true
	}
	if len(run.UnsafeFlags) != n {
		return true
	}
	left := run.UnsafeFlags[cut-1] & unsafeCutMask
	right := run.UnsafeFlags[cut] & unsafeCutMask
	// A boundary is unsafe only when both sides carry unsafe flags, i.e. the cut
	// would slice through an unsafe span instead of landing at its edge.
	return left == 0 || right == 0
}

func rawFlushForGlyphPrefix(run *runBuffer, glyphCut int, rawStart uint32, rawLen int) (int, bool) {
	assert(run != nil, "run buffer is nil")
	if glyphCut < 0 || glyphCut > run.Len() || rawLen < 0 {
		return 0, false
	}
	if glyphCut == 0 {
		return 0, true
	}
	if len(run.Clusters) != run.Len() {
		return 0, false
	}
	next := rawStart
	for i := 0; i < glyphCut; i++ {
		cl := run.Clusters[i]
		if cl != next {
			// Partial raw compaction is only safe for strict 1:1 cluster coverage.
			// Any repeat/decrease/gap can hide source runes consumed by collapsed
			// or merged clusters, which would duplicate output on the next cycle.
			// In that case we defer progress until EOF or maxBuffer force-flush.
			return 0, false
		}
		next++
		if next-rawStart > uint32(rawLen) {
			return 0, false
		}
	}
	rawFlush := int(next - rawStart)
	if rawFlush < 0 || rawFlush > rawLen {
		return 0, false
	}
	if rawFlush == 0 {
		return 0, false
	}
	return rawFlush, true
}
