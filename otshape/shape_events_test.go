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

var singleBufOpts = BufferOptions{
	FlushBoundary: FlushOnRunBoundary,
	HighWatermark: 2,
	LowWatermark:  1,
	MaxBuffer:     8,
}

func (s *sliceEventSource) ReadEvent() (InputEvent, error) {
	if s.index >= len(s.events) {
		return InputEvent{}, io.EOF
	}
	ev := s.events[s.index]
	s.index++
	return ev, nil
}

func standardParams(font *ot.Font) Params {
	return Params{
		Font:      font,
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
	}
}

func TestShapeEventsRuneOnlyParity(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	input := []rune{0x12, 0x13, 0x12, 0x13}
	source := strings.NewReader(string(input))
	baseline := &collectSink{}
	shaper := NewShaper([]ShapingEngine{&hookProbeShaper{}}...)
	err := shaper.Shape(params, source, baseline, singleBufOpts)
	if err != nil {
		t.Fatalf("baseline Shape failed: %v", err)
	}

	evsource := NewInputEventSource(strings.NewReader(string(input)))
	eventSink := &collectSink{}
	shaper = NewShaper([]ShapingEngine{&hookProbeShaper{}}...)
	err = shaper.ShapeEvents(params, evsource, eventSink, singleBufOpts)
	if err != nil {
		t.Fatalf("baseline Shape failed: %v", err)
	}

	if !reflect.DeepEqual(eventSink.glyphs, baseline.glyphs) {
		t.Fatalf("event output differs from baseline:\nevent=%#v\nbase=%#v", eventSink.glyphs, baseline.glyphs)
	}
}

func TestShapeEventsPopUnderflowIsError(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	evsource := &sliceEventSource{
		events: []InputEvent{
			{Kind: InputEventPopFeatures},
		},
	}
	eventSink := &collectSink{}
	shaper := NewShaper([]ShapingEngine{&hookProbeShaper{}}...)
	err := shaper.ShapeEvents(params, evsource, eventSink, singleBufOpts)
	if !errors.Is(err, errPlanStackUnderflow) {
		t.Fatalf("ShapeEvents error=%v, want %v", err, errPlanStackUnderflow)
	}
}

func TestShapeEventsUnclosedStackAtEOFIsError(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	evsource := &sliceEventSource{
		events: []InputEvent{
			{
				Kind: InputEventPushFeatures,
				Push: []FeatureSetting{{Tag: ot.T("liga"), Enabled: false}},
			},
			{Kind: InputEventRune, Rune: 0x12, Size: 1},
		},
	}
	eventSink := &collectSink{}
	shaper := NewShaper([]ShapingEngine{&hookProbeShaper{}}...)
	err := shaper.ShapeEvents(params, evsource, eventSink, singleBufOpts)
	if !errors.Is(err, errPlanStackUnclosed) {
		t.Fatalf("ShapeEvents error=%v, want %v", err, errPlanStackUnclosed)
	}
}

func TestShapeEventsRejectsIndexedFeatureRangeInOptions(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	params.Features = []FeatureRange{
		{Feature: ot.T("liga"), On: true, Start: 1, End: 3},
	}
	evsource := NewInputEventSource(strings.NewReader("ab"))
	eventSink := &collectSink{}
	shaper := NewShaper([]ShapingEngine{&hookProbeShaper{}}...)
	err := shaper.ShapeEvents(params, evsource, eventSink, singleBufOpts)
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
