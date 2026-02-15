package otshape

import (
	"reflect"
	"strings"
	"testing"
)

type countingRuneSource struct {
	inner *strings.Reader
	calls int
}

func newCountingRuneSource(s string) *countingRuneSource {
	return &countingRuneSource{inner: strings.NewReader(s)}
}

func (s *countingRuneSource) ReadRune() (r rune, size int, err error) {
	s.calls++
	return s.inner.ReadRune()
}

func TestShapeStreamingHighWatermarkSmallProducesOutput(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	input := []rune{0x12, 0x13, 0x12, 0x13, 0x12, 0x13}
	source := newCountingRuneSource(string(input))
	sink := &collectSink{}
	engine := &hookProbeShaper{}
	shaper := NewShaper([]ShapingEngine{engine}...)

	err := shaper.Shape(params, source, sink, singleBufOpts)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if len(sink.glyphs) != len(input) {
		t.Fatalf("glyph count=%d, want %d", len(sink.glyphs), len(input))
	}
	if source.calls < len(input) {
		t.Fatalf("read calls=%d, want at least %d", source.calls, len(input))
	}
}

func TestShapeStreamingMatchesDefaultForSimpleInput(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	input := []rune{0x12, 0x13, 0x12, 0x13, 0x12, 0x13}
	baseEngine := &hookProbeShaper{}
	source := strings.NewReader(string(input))
	sink := &collectSink{}
	shaper := NewShaper([]ShapingEngine{baseEngine}...)

	baseErr := shaper.Shape(params, source, sink, singleBufOpts)
	if baseErr != nil {
		t.Fatalf("baseline shape failed: %v", baseErr)
	}

	streamSource := strings.NewReader(string(input))
	streamSink := &collectSink{}
	streamEngine := &hookProbeShaper{}
	shaper = NewShaper([]ShapingEngine{streamEngine}...)

	streamErr := shaper.Shape(params, streamSource, streamSink, singleBufOpts)
	if streamErr != nil {
		t.Fatalf("streaming shape failed: %v", streamErr)
	}
	if !reflect.DeepEqual(streamSink.glyphs, sink.glyphs) {
		t.Fatalf("streaming output differs from baseline:\nstream=%#v\nbase=%#v", streamSink.glyphs, sink.glyphs)
	}
}

func TestShapeStreamingEmptySourceNoOutput(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	source := strings.NewReader("")
	sink := &collectSink{}
	engine := &hookProbeShaper{}
	shaper := NewShaper([]ShapingEngine{engine}...)

	err := shaper.Shape(params, source, sink, singleBufOpts)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if len(sink.glyphs) != 0 {
		t.Fatalf("glyph count=%d, want 0", len(sink.glyphs))
	}
}

func TestShapeStreamingComposeHookMatchesSinglePassBaseline(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	input := []rune{0x12, 0x13, 0x12, 0x13, 0x12, 0x13}
	baselineOpts := BufferOptions{
		FlushBoundary: FlushOnRunBoundary,
		HighWatermark: 64,
		LowWatermark:  32,
		MaxBuffer:     64,
	}

	baselineSink := &collectSink{}
	baselineEngine := &hookProbeShaper{useCompose: true}
	shaper := NewShaper([]ShapingEngine{baselineEngine}...)
	baseErr := shaper.Shape(params, strings.NewReader(string(input)), baselineSink, baselineOpts)
	if baseErr != nil {
		t.Fatalf("single-pass baseline failed: %v", baseErr)
	}

	streamSink := &collectSink{}
	streamEngine := &hookProbeShaper{useCompose: true}
	shaper = NewShaper([]ShapingEngine{streamEngine}...)
	streamErr := shaper.Shape(params, strings.NewReader(string(input)), streamSink, singleBufOpts)
	if streamErr != nil {
		t.Fatalf("streaming shape failed: %v", streamErr)
	}
	if !reflect.DeepEqual(streamSink.glyphs, baselineSink.glyphs) {
		t.Fatalf("streaming compose output differs from baseline:\nstream=%#v\nbase=%#v", streamSink.glyphs, baselineSink.glyphs)
	}
}

func TestShapeStreamingReorderHookMatchesSinglePassBaseline(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	input := []rune{0x12, 0x13, 0x12, 0x13, 0x12, 0x13}
	baselineOpts := BufferOptions{
		FlushBoundary: FlushOnRunBoundary,
		HighWatermark: 64,
		LowWatermark:  32,
		MaxBuffer:     64,
	}

	baselineSink := &collectSink{}
	baselineEngine := &hookProbeShaper{useReorder: true}
	shaper := NewShaper([]ShapingEngine{baselineEngine}...)
	baseErr := shaper.Shape(params, strings.NewReader(string(input)), baselineSink, baselineOpts)
	if baseErr != nil {
		t.Fatalf("single-pass baseline failed: %v", baseErr)
	}

	streamSink := &collectSink{}
	streamEngine := &hookProbeShaper{useReorder: true}
	shaper = NewShaper([]ShapingEngine{streamEngine}...)
	streamErr := shaper.Shape(params, strings.NewReader(string(input)), streamSink, singleBufOpts)
	if streamErr != nil {
		t.Fatalf("streaming shape failed: %v", streamErr)
	}
	if !reflect.DeepEqual(streamSink.glyphs, baselineSink.glyphs) {
		t.Fatalf("streaming reorder output differs from baseline:\nstream=%#v\nbase=%#v", streamSink.glyphs, baselineSink.glyphs)
	}
}
