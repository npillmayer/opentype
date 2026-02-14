package otshape

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

type sliceEventSource struct {
	events []InputEvent
	index  int
}

func (s *sliceEventSource) ReadEvent() (InputEvent, error) {
	if s.index >= len(s.events) {
		return InputEvent{}, io.EOF
	}
	ev := s.events[s.index]
	s.index++
	return ev, nil
}

func TestShapeEventsRuneOnlyParity(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	input := []rune{0x12, 0x13, 0x12, 0x13}
	opts := ShapeOptions{
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
	}

	baseline := &collectSink{}
	if err := Shape(ShapeRequest{
		Options: opts,
		Source:  strings.NewReader(string(input)),
		Sink:    baseline,
		Shapers: []ShapingEngine{&hookProbeShaper{}},
	}); err != nil {
		t.Fatalf("baseline Shape failed: %v", err)
	}

	eventSink := &collectSink{}
	if err := ShapeEvents(ShapeEventsRequest{
		Options: opts,
		Source:  NewInputEventSource(strings.NewReader(string(input))),
		Sink:    eventSink,
		Shapers: []ShapingEngine{&hookProbeShaper{}},
	}); err != nil {
		t.Fatalf("ShapeEvents failed: %v", err)
	}

	if !reflect.DeepEqual(eventSink.glyphs, baseline.glyphs) {
		t.Fatalf("event output differs from baseline:\nevent=%#v\nbase=%#v", eventSink.glyphs, baseline.glyphs)
	}
}

func TestShapeEventsPopUnderflowIsError(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	opts := ShapeOptions{
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
	}
	src := &sliceEventSource{
		events: []InputEvent{
			{Kind: InputEventPopFeatures},
		},
	}
	err := ShapeEvents(ShapeEventsRequest{
		Options: opts,
		Source:  src,
		Sink:    &collectSink{},
		Shapers: []ShapingEngine{&hookProbeShaper{}},
	})
	if !errors.Is(err, errPlanStackUnderflow) {
		t.Fatalf("ShapeEvents error=%v, want %v", err, errPlanStackUnderflow)
	}
}

func TestShapeEventsUnclosedStackAtEOFIsError(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	opts := ShapeOptions{
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
	}
	src := &sliceEventSource{
		events: []InputEvent{
			{
				Kind: InputEventPushFeatures,
				Push: []FeatureSetting{{Tag: ot.T("liga"), Enabled: false}},
			},
			{Kind: InputEventRune, Rune: 0x12, Size: 1},
		},
	}
	err := ShapeEvents(ShapeEventsRequest{
		Options: opts,
		Source:  src,
		Sink:    &collectSink{},
		Shapers: []ShapingEngine{&hookProbeShaper{}},
	})
	if !errors.Is(err, errPlanStackUnclosed) {
		t.Fatalf("ShapeEvents error=%v, want %v", err, errPlanStackUnclosed)
	}
}

func TestShapeEventsRejectsIndexedFeatureRangeInOptions(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	opts := ShapeOptions{
		Params: Params{
			Font:      font,
			Direction: bidi.LeftToRight,
			Script:    language.MustParseScript("Latn"),
			Language:  language.English,
			Features: []FeatureRange{
				{Feature: ot.T("liga"), On: true, Start: 1, End: 3},
			},
		},
		FlushBoundary: FlushOnRunBoundary,
		HighWatermark: 2,
		LowWatermark:  1,
		MaxBuffer:     8,
	}
	err := ShapeEvents(ShapeEventsRequest{
		Options: opts,
		Source:  NewInputEventSource(strings.NewReader("ab")),
		Sink:    &collectSink{},
		Shapers: []ShapingEngine{&hookProbeShaper{}},
	})
	if !errors.Is(err, ErrEventIndexedFeatureRange) {
		t.Fatalf("ShapeEvents error=%v, want %v", err, ErrEventIndexedFeatureRange)
	}
}

func TestFillEventsStoresPlanIDsInCarryBuffer(t *testing.T) {
	cfg := streamingConfig{
		highWatermark: 8,
		lowWatermark:  4,
		maxBuffer:     8,
	}
	st := newStreamingState(cfg)
	stack := newPlanStack(nil, &plan{})
	plans := map[uint16]*plan{
		stack.currentPlanID(): stack.currentPlan(),
	}
	src := &sliceEventSource{
		events: []InputEvent{
			{
				Kind: InputEventPushFeatures,
				Push: []FeatureSetting{{Tag: ot.T("liga"), Enabled: false}},
			},
			{Kind: InputEventRune, Rune: 'A', Size: 1},
			{Kind: InputEventPopFeatures},
			{Kind: InputEventRune, Rune: 'B', Size: 1},
		},
	}
	build := func([]FeatureRange) (*plan, error) { return &plan{}, nil }
	if _, err := fillEventsUntilHighWatermark(src, st, stack, plans, build); err != nil {
		t.Fatalf("fillEventsUntilHighWatermark failed: %v", err)
	}
	want := []uint16{2, 1}
	if !reflect.DeepEqual(st.rawPlanIDs, want) {
		t.Fatalf("raw plan ids = %v, want %v", st.rawPlanIDs, want)
	}
	if len(st.rawRunes) != 2 || len(st.rawClusters) != 2 {
		t.Fatalf("raw buffers not aligned: runes=%d clusters=%d", len(st.rawRunes), len(st.rawClusters))
	}
}
