package otshape

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type eventSourceProbe struct {
	ev InputEvent
}

func (p eventSourceProbe) ReadRune() (r rune, size int, err error) {
	return 0, 0, io.EOF
}

func (p eventSourceProbe) ReadEvent() (InputEvent, error) {
	return p.ev, nil
}

func TestInputEventValidate(t *testing.T) {
	tests := []struct {
		name string
		ev   InputEvent
		ok   bool
	}{
		{
			name: "rune",
			ev:   InputEvent{Kind: InputEventRune, Rune: 'a', Size: 1},
			ok:   true,
		},
		{
			name: "push",
			ev: InputEvent{
				Kind: InputEventPushFeatures,
				Push: []FeatureSetting{{Tag: ot.T("smcp"), Enabled: true}},
			},
			ok: true,
		},
		{
			name: "pop",
			ev:   InputEvent{Kind: InputEventPopFeatures},
			ok:   true,
		},
		{
			name: "push-empty-invalid",
			ev:   InputEvent{Kind: InputEventPushFeatures},
			ok:   false,
		},
		{
			name: "pop-with-payload-invalid",
			ev:   InputEvent{Kind: InputEventPopFeatures, Rune: 'x'},
			ok:   false,
		},
		{
			name: "unknown-kind-invalid",
			ev:   InputEvent{Kind: InputEventKind(99)},
			ok:   false,
		},
	}
	for _, tc := range tests {
		err := tc.ev.Validate()
		if tc.ok && err != nil {
			t.Fatalf("%s: unexpected validation error: %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%s: expected validation error", tc.name)
		}
	}
}

func TestNewInputEventSourceAdapter(t *testing.T) {
	src := strings.NewReader("ab")
	evs := NewInputEventSource(src)
	ev1, err := evs.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent #1 failed: %v", err)
	}
	if ev1.Kind != InputEventRune || ev1.Rune != 'a' {
		t.Fatalf("unexpected event #1: %+v", ev1)
	}
	ev2, err := evs.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent #2 failed: %v", err)
	}
	if ev2.Kind != InputEventRune || ev2.Rune != 'b' {
		t.Fatalf("unexpected event #2: %+v", ev2)
	}
	_, err = evs.ReadEvent()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestNewInputEventSourcePassThrough(t *testing.T) {
	want := InputEvent{Kind: InputEventPopFeatures}
	src := eventSourceProbe{ev: want}
	evs := NewInputEventSource(src)
	got, err := evs.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event mismatch: got %+v, want %+v", got, want)
	}
}
