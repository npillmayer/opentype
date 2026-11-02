package otshape

import (
	"fmt"
	"io"
)

// ErrZeroOutputBufferFull means shaping produced no glyphs, but the raw input
// reached MaxBuffer before EOF was known. The carry cannot be discarded safely:
// additional context may change the result. Earlier sink writes are retained.
// This conservative limit also applies when input ends exactly at MaxBuffer,
// since discovering EOF would require another source read.
var ErrZeroOutputBufferFull = fmt.Errorf("%w: zero-output shaping cannot be finalized", ErrBufferFull)

const (
	defaultHighWatermark = 3500
	defaultLowWatermark  = 1024
	defaultMaxBuffer     = MinBuffer
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
		return streamingConfig{}, fmt.Errorf("%w: streaming watermarks must be >= 0", ErrBufferConfig)
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
		cfg.maxBuffer = max(MinBuffer, opts.MaxBuffer)
	} else if cfg.maxBuffer < cfg.highWatermark {
		cfg.maxBuffer = cfg.highWatermark
	}
	if !cfg.valid() {
		return streamingConfig{}, fmt.Errorf("%w: high=%d low=%d max=%d", ErrBufferConfig, cfg.highWatermark, cfg.lowWatermark, cfg.maxBuffer)
	}
	return cfg, nil
}

// streamingState holds in-flight source content not yet flushed to sink.
type streamingState struct {
	rawRunes    []rune
	rawClusters []uint32
	rawPlanIDs  []uint16
	nextCluster uint32
	groupStart  uint32
	groupCount  int
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
		if err := st.checkInputRune(r); err != nil {
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
	// Both halves must restart independently. Lookup observations and retained
	// future context certify raw boundaries; output provenance selects the matching
	// glyph prefix even after contraction, expansion, deletion, or reordering.
	trace := run.stream
	if trace == nil || trace.Disabled || len(trace.Sources) != run.Len() {
		return flushDecision{}
	}
	low := min(st.cfg.lowWatermark, len(st.rawRunes))
	fallback := flushDecision{}
	// Prefix ends and suffix starts can be checked in linear time even when
	// normalization reordered glyphs within a source group.
	suffixStart := make([]int, run.Len()+1)
	suffixStart[run.Len()] = len(st.rawRunes)
	for i := run.Len() - 1; i >= 0; i-- {
		suffixStart[i] = min(trace.Sources[i].Start, suffixStart[i+1])
	}
	glyph := 0
	for raw := 1; raw <= trace.KeepFrom && raw < len(trace.Unsafe); raw++ {
		if trace.Unsafe[raw] {
			continue
		}
		for glyph < run.Len() && trace.Sources[glyph].End <= raw {
			glyph++
		}
		if glyph == 0 {
			continue
		}
		if suffixStart[glyph] < raw || !isBreakSafeCut(run, glyph) {
			continue
		}
		if glyph < run.Len() && run.Clusters[glyph-1] == run.Clusters[glyph] {
			continue
		}
		cut := flushDecision{glyphCut: glyph, rawFlush: raw, ready: true}
		if raw >= low {
			return cut
		}
		fallback = cut
	}
	if len(st.rawRunes) == st.cfg.maxBuffer {
		return fallback
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
