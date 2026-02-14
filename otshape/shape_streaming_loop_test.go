package otshape

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
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
	input := []rune{0x12, 0x13, 0x12, 0x13, 0x12, 0x13}
	src := newCountingRuneSource(string(input))
	sink := &collectSink{}
	engine := &hookProbeShaper{}
	err := Shape(ShapeRequest{
		Options: ShapeOptions{
			Params: Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    language.MustParseScript("Latn"),
				Language:  language.English,
			},
			FlushBoundary: FlushOnRunBoundary,
			HighWatermark: 2,
			LowWatermark:  1,
			MaxBuffer:     8,
		},
		Source:  src,
		Sink:    sink,
		Shapers: []ShapingEngine{engine},
	})
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if len(sink.glyphs) != len(input) {
		t.Fatalf("glyph count=%d, want %d", len(sink.glyphs), len(input))
	}
	if src.calls < len(input) {
		t.Fatalf("read calls=%d, want at least %d", src.calls, len(input))
	}
}

func TestShapeStreamingMatchesDefaultForSimpleInput(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	input := []rune{0x12, 0x13, 0x12, 0x13, 0x12, 0x13}
	baseEngine := &hookProbeShaper{}

	baseSink := &collectSink{}
	baseErr := Shape(ShapeRequest{
		Options: ShapeOptions{
			Params: Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    language.MustParseScript("Latn"),
				Language:  language.English,
			},
			FlushBoundary: FlushOnRunBoundary,
		},
		Source:  strings.NewReader(string(input)),
		Sink:    baseSink,
		Shapers: []ShapingEngine{baseEngine},
	})
	if baseErr != nil {
		t.Fatalf("baseline shape failed: %v", baseErr)
	}

	streamSink := &collectSink{}
	streamEngine := &hookProbeShaper{}
	streamErr := Shape(ShapeRequest{
		Options: ShapeOptions{
			Params: Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    language.MustParseScript("Latn"),
				Language:  language.English,
			},
			FlushBoundary: FlushOnRunBoundary,
			HighWatermark: 2,
			LowWatermark:  1,
			MaxBuffer:     8,
		},
		Source:  strings.NewReader(string(input)),
		Sink:    streamSink,
		Shapers: []ShapingEngine{streamEngine},
	})
	if streamErr != nil {
		t.Fatalf("streaming shape failed: %v", streamErr)
	}
	if !reflect.DeepEqual(streamSink.glyphs, baseSink.glyphs) {
		t.Fatalf("streaming output differs from baseline:\nstream=%#v\nbase=%#v", streamSink.glyphs, baseSink.glyphs)
	}
}

func TestShapeStreamingEmptySourceNoOutput(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	sink := &collectSink{}
	engine := &hookProbeShaper{}
	err := Shape(ShapeRequest{
		Options: ShapeOptions{
			Params: Params{
				Font:      font,
				Direction: bidi.LeftToRight,
				Script:    language.MustParseScript("Latn"),
				Language:  language.English,
			},
			FlushBoundary: FlushOnRunBoundary,
			HighWatermark: 2,
			LowWatermark:  1,
			MaxBuffer:     8,
		},
		Source:  strings.NewReader(""),
		Sink:    sink,
		Shapers: []ShapingEngine{engine},
	})
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if len(sink.glyphs) != 0 {
		t.Fatalf("glyph count=%d, want 0", len(sink.glyphs))
	}
}
