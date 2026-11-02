package otshape

import (
	"errors"
	"github.com/npillmayer/opentype/otlayout"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestResolveStreamingConfigDefaults(t *testing.T) {
	cfg, err := resolveStreamingConfig(BufferOptions{})
	if err != nil {
		t.Fatalf("resolveStreamingConfig failed: %v", err)
	}
	if cfg.highWatermark != defaultHighWatermark || cfg.lowWatermark != defaultLowWatermark || cfg.maxBuffer != defaultMaxBuffer {
		t.Fatalf("unexpected defaults high=%d low=%d max=%d", cfg.highWatermark, cfg.lowWatermark, cfg.maxBuffer)
	}
}

func TestResolveStreamingConfigOverrides(t *testing.T) {
	cfg, err := resolveStreamingConfig(BufferOptions{
		HighWatermark: 300,
		LowWatermark:  150,
		MaxBuffer:     600,
	})
	if err != nil {
		t.Fatalf("resolveStreamingConfig failed: %v", err)
	}
	if cfg.highWatermark != 300 || cfg.lowWatermark != 150 || cfg.maxBuffer != MinBuffer {
		t.Fatalf("unexpected config high=%d low=%d max=%d", cfg.highWatermark, cfg.lowWatermark, cfg.maxBuffer)
	}
}

func TestResolveStreamingConfigInvalid(t *testing.T) {
	_, err := resolveStreamingConfig(BufferOptions{HighWatermark: 64, LowWatermark: 96})
	if err == nil {
		t.Fatalf("expected error for low>high")
	}
	_, err = resolveStreamingConfig(BufferOptions{HighWatermark: MinBuffer + 1, MaxBuffer: MinBuffer})
	if err == nil {
		t.Fatalf("expected error for max<high")
	}
	_, err = resolveStreamingConfig(BufferOptions{HighWatermark: -1})
	if err == nil {
		t.Fatalf("expected error for negative watermark")
	}
}

func TestFillUntilHighWatermarkReadsAndAssignsClusters(t *testing.T) {
	cfg := streamingConfig{highWatermark: 3, lowWatermark: 2, maxBuffer: 8}
	st := newStreamingState(cfg)
	src := strings.NewReader("abcdef")
	read, err := fillUntilHighWatermark(src, st)
	if err != nil {
		t.Fatalf("fill failed: %v", err)
	}
	if read != 3 {
		t.Fatalf("read count=%d, want 3", read)
	}
	if got := string(st.rawRunes); got != "abc" {
		t.Fatalf("raw runes=%q, want %q", got, "abc")
	}
	wantClusters := []uint32{0, 1, 2}
	for i, w := range wantClusters {
		if st.rawClusters[i] != w {
			t.Fatalf("cluster[%d]=%d, want %d", i, st.rawClusters[i], w)
		}
	}
	if st.nextCluster != 3 {
		t.Fatalf("nextCluster=%d, want 3", st.nextCluster)
	}
	if st.eof {
		t.Fatalf("unexpected eof=true")
	}
}

func TestFillUntilHighWatermarkSetsEOF(t *testing.T) {
	cfg := streamingConfig{highWatermark: 8, lowWatermark: 4, maxBuffer: 16}
	st := newStreamingState(cfg)
	src := strings.NewReader("ab")
	read, err := fillUntilHighWatermark(src, st)
	if err != nil {
		t.Fatalf("fill failed: %v", err)
	}
	if read != 2 {
		t.Fatalf("read count=%d, want 2", read)
	}
	if !st.eof {
		t.Fatalf("expected eof=true")
	}
	read, err = fillUntilHighWatermark(src, st)
	if err != nil {
		t.Fatalf("2nd fill failed: %v", err)
	}
	if read != 0 {
		t.Fatalf("2nd read count=%d, want 0", read)
	}
}

type errRuneSource struct {
	data string
	at   int
	read int
	err  error
}

func (s *errRuneSource) ReadRune() (r rune, size int, err error) {
	if s.read == s.at {
		return 0, 0, s.err
	}
	if s.read >= len(s.data) {
		return 0, 0, io.EOF
	}
	r = rune(s.data[s.read])
	s.read++
	return r, 1, nil
}

func TestFillUntilHighWatermarkReadError(t *testing.T) {
	cfg := streamingConfig{highWatermark: 8, lowWatermark: 4, maxBuffer: 16}
	st := newStreamingState(cfg)
	src := &errRuneSource{data: "abcd", at: 2, err: errors.New("boom")}
	read, err := fillUntilHighWatermark(src, st)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("fill error=%v, want boom", err)
	}
	if read != 2 {
		t.Fatalf("read count=%d, want 2", read)
	}
	if got := string(st.rawRunes); got != "ab" {
		t.Fatalf("raw runes=%q, want %q", got, "ab")
	}
}

func TestCompactCarryDropsPrefix(t *testing.T) {
	cfg := streamingConfig{highWatermark: 8, lowWatermark: 4, maxBuffer: 16}
	st := newStreamingState(cfg)
	st.rawRunes = []rune{'a', 'b', 'c', 'd', 'e'}
	st.rawClusters = []uint32{10, 11, 12, 13, 14}
	st.nextCluster = 20
	compactCarry(st, 2)
	if got := string(st.rawRunes); got != "cde" {
		t.Fatalf("raw runes=%q, want %q", got, "cde")
	}
	wantClusters := []uint32{12, 13, 14}
	for i, w := range wantClusters {
		if st.rawClusters[i] != w {
			t.Fatalf("cluster[%d]=%d, want %d", i, st.rawClusters[i], w)
		}
	}
	if st.nextCluster != 20 {
		t.Fatalf("nextCluster=%d, want 20", st.nextCluster)
	}
}

func TestFindFlushCutRespectsLowWatermark(t *testing.T) {
	cfg := streamingConfig{highWatermark: 4, lowWatermark: 2, maxBuffer: 8}
	st := newStreamingState(cfg)
	st.rawRunes = []rune{'a', 'b', 'c', 'd'}
	st.rawClusters = []uint32{10, 11, 12, 13}
	st.nextCluster = 14

	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 100, 101, 102, 103)
	run.Clusters = []uint32{10, 11, 12, 13}
	run.stream = unitStreamTrace(4)

	cut := findFlushCut(run, st)
	want := flushDecision{glyphCut: 2, rawFlush: 2, ready: true}
	if !reflect.DeepEqual(cut, want) {
		t.Fatalf("flush cut=%#v, want %#v", cut, want)
	}
}

func TestFindFlushCutRejectsUncertifiedBoundaryAtMaxBuffer(t *testing.T) {
	cfg := streamingConfig{highWatermark: 4, lowWatermark: 3, maxBuffer: 4}
	st := newStreamingState(cfg)
	st.rawRunes = []rune{'a', 'b', 'c', 'd'}
	st.rawClusters = []uint32{0, 1, 2, 3}
	st.nextCluster = 4

	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11)
	run.Clusters = []uint32{0, 2}

	cut := findFlushCut(run, st)
	want := flushDecision{}
	if !reflect.DeepEqual(cut, want) {
		t.Fatalf("flush cut=%#v, want %#v", cut, want)
	}
}

func TestFindFlushCutEOFFlushesAll(t *testing.T) {
	cfg := streamingConfig{highWatermark: 4, lowWatermark: 2, maxBuffer: 8}
	st := newStreamingState(cfg)
	st.rawRunes = []rune{'a', 'b', 'c'}
	st.rawClusters = []uint32{2, 3, 4}
	st.nextCluster = 5
	st.eof = true

	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 20, 21)
	run.Clusters = []uint32{2, 4}

	cut := findFlushCut(run, st)
	want := flushDecision{glyphCut: 2, rawFlush: 3, ready: true}
	if !reflect.DeepEqual(cut, want) {
		t.Fatalf("flush cut=%#v, want %#v", cut, want)
	}
}

func TestFindFlushCutSkipsUnsafeInteriorBoundary(t *testing.T) {
	cfg := streamingConfig{highWatermark: 4, lowWatermark: 2, maxBuffer: 8}
	st := newStreamingState(cfg)
	st.rawRunes = []rune{'a', 'b', 'c', 'd'}
	st.rawClusters = []uint32{0, 1, 2, 3}
	st.nextCluster = 4

	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 30, 31, 32, 33)
	run.Clusters = []uint32{0, 1, 2, 3}
	run.UnsafeFlags = []uint16{0, unsafeFlagToBreak, unsafeFlagToBreak, 0}
	run.stream = unitStreamTrace(4)

	cut := findFlushCut(run, st)
	want := flushDecision{glyphCut: 3, rawFlush: 3, ready: true}
	if !reflect.DeepEqual(cut, want) {
		t.Fatalf("flush cut=%#v, want %#v", cut, want)
	}
}

func TestFindFlushCutUnsafeEverywhereNeverForcesProgress(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 40, 41, 42, 43)
	run.Clusters = []uint32{0, 2, 4, 6}
	run.UnsafeFlags = []uint16{
		unsafeFlagToBreak,
		unsafeFlagToBreak,
		unsafeFlagToBreak,
		unsafeFlagToBreak,
	}

	stNoForce := newStreamingState(streamingConfig{highWatermark: 4, lowWatermark: 2, maxBuffer: 8})
	stNoForce.rawRunes = []rune{'a', 'b', 'c', 'd'}
	stNoForce.rawClusters = []uint32{0, 1, 2, 3}
	stNoForce.nextCluster = 4

	cut := findFlushCut(run, stNoForce)
	if cut.ready {
		t.Fatalf("expected no ready cut without forced progress, got %#v", cut)
	}

	stForce := newStreamingState(streamingConfig{highWatermark: 4, lowWatermark: 2, maxBuffer: 4})
	stForce.rawRunes = []rune{'a', 'b', 'c', 'd'}
	stForce.rawClusters = []uint32{0, 1, 2, 3}
	stForce.nextCluster = 4

	cut = findFlushCut(run, stForce)
	want := flushDecision{}
	if !reflect.DeepEqual(cut, want) {
		t.Fatalf("flush cut=%#v, want %#v", cut, want)
	}
}

func TestWriteRunBufferPrefixToSinkClusterBoundaryRejectsMisalignedCut(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11, 12)
	run.Clusters = []uint32{0, 0, 1}

	sink := &collectSink{}
	err := writeRunBufferPrefixToSink(run, sink, FlushOnClusterBoundary, 1)
	if err == nil {
		t.Fatalf("expected error for non-cluster-aligned cut")
	}
}

func unitStreamTrace(n int) *otlayout.StreamTrace {
	t := otlayout.NewStreamTrace(n)
	for i := 0; i < n; i++ {
		t.Sources = append(t.Sources, otlayout.SourceSpan{Start: i, End: i + 1})
	}
	t.KeepFrom = n - 1
	return t
}

func TestFindFlushCutMapsExpansionAndContraction(t *testing.T) {
	st := newStreamingState(streamingConfig{4, 2, 4})
	st.rawRunes = []rune("abcd")
	st.rawClusters = []uint32{50, 51, 52, 53}
	st.nextCluster = 54
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 1, 2, 3, 4)
	run.Clusters = []uint32{50, 50, 52, 53}
	run.stream = unitStreamTrace(4)
	run.stream.Sources = []otlayout.SourceSpan{{Start: 0, End: 2}, {Start: 0, End: 2}, {Start: 2, End: 3}, {Start: 3, End: 4}}
	run.stream.Join(0, 2)
	if got := findFlushCut(run, st); got != (flushDecision{2, 2, true}) {
		t.Fatalf("cut=%+v", got)
	}
	// A contraction is equally valid: two raw runes represented by one glyph.
	run.Glyphs = run.Glyphs[1:]
	run.Clusters = run.Clusters[1:]
	run.stream.Sources = run.stream.Sources[1:]
	if got := findFlushCut(run, st); got != (flushDecision{1, 2, true}) {
		t.Fatalf("cut=%+v", got)
	}
}
