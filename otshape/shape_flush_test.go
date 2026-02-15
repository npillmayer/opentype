package otshape

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

type collectSink struct {
	glyphs []GlyphRecord
}

func (s *collectSink) WriteGlyph(g GlyphRecord) error {
	s.glyphs = append(s.glyphs, g)
	return nil
}

func TestClusterSpans(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11, 12, 13, 14)
	run.Clusters = []uint32{0, 0, 1, 2, 2}
	spans := clusterSpans(run)
	want := []runSpan{
		{start: 0, end: 2},
		{start: 2, end: 3},
		{start: 3, end: 5},
	}
	if !reflect.DeepEqual(spans, want) {
		t.Fatalf("cluster spans = %#v, want %#v", spans, want)
	}

	run.Clusters = nil
	spans = clusterSpans(run)
	want = []runSpan{{start: 0, end: 5}}
	if !reflect.DeepEqual(spans, want) {
		t.Fatalf("cluster spans without cluster data = %#v, want %#v", spans, want)
	}
}

func TestWriteRunBufferToSinkFlushModesPreserveOutput(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11, 12, 13, 14)
	run.Clusters = []uint32{0, 0, 1, 2, 2}
	run.Masks = []uint32{1, 1, 2, 4, 4}
	run.UnsafeFlags = []uint16{0, 1, 0, 2, 0}
	run.Pos = otlayout.NewPosBuffer(5)
	run.Pos[0].XAdvance = 100
	run.Pos[1].XAdvance = 101
	run.Pos[2].XAdvance = 102
	run.Pos[3].XAdvance = 103
	run.Pos[4].XAdvance = 104

	runSink := &collectSink{}
	clusterSink := &collectSink{}
	if err := writeRunBufferToSink(run, runSink, FlushOnRunBoundary); err != nil {
		t.Fatalf("run-boundary write failed: %v", err)
	}
	if err := writeRunBufferToSink(run, clusterSink, FlushOnClusterBoundary); err != nil {
		t.Fatalf("cluster-boundary write failed: %v", err)
	}
	if !reflect.DeepEqual(runSink.glyphs, clusterSink.glyphs) {
		t.Fatalf("flush modes produced different output:\nrun=%#v\ncluster=%#v", runSink.glyphs, clusterSink.glyphs)
	}
}

func TestWriteRunBufferToSinkFlushExplicitUnsupported(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10)
	sink := &collectSink{}
	err := writeRunBufferToSink(run, sink, FlushExplicit)
	if !errors.Is(err, ErrFlushExplicitUnsupported) {
		t.Fatalf("flush explicit error = %v, want %v", err, ErrFlushExplicitUnsupported)
	}
}

func TestShapeFlushExplicitUnsupported(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	params := standardParams(font)
	params.Features = []FeatureRange{
		{Feature: ot.T("test"), On: true},
	}
	opts := singleBufOpts
	opts.FlushBoundary = FlushExplicit
	eventSource := strings.NewReader(string(rune(0x12)))
	eventSink := &collectSink{}
	shaper := NewShaper([]ShapingEngine{&hookProbeShaper{}}...)
	err := shaper.Shape(params, eventSource, eventSink, opts)
	if !errors.Is(err, ErrFlushExplicitUnsupported) {
		t.Fatalf("shape error = %v, want %v", err, ErrFlushExplicitUnsupported)
	}
}
