package otshape

import (
	"fmt"

	"github.com/npillmayer/opentype/ot"
)

// InputEventKind classifies events consumed by the shaping input pipeline.
type InputEventKind uint8

const (
	// InputEventRune emits one text rune from the source stream.
	InputEventRune InputEventKind = iota
	// InputEventPushFeatures pushes a nested feature delta onto the active stack.
	InputEventPushFeatures
	// InputEventPopFeatures pops one nested feature frame.
	InputEventPopFeatures
)

// FeatureSetting is one feature assignment used by push events.
type FeatureSetting struct {
	Tag     ot.Tag // OpenType feature tag.
	Value   int    // Feature value; values <= 0 default to 1 when Enabled is true.
	Enabled bool   // Whether the feature should be enabled for the pushed frame.
}

// InputEvent is one item in the event-shaped input stream.
type InputEvent struct {
	Kind InputEventKind
	// Rune is used when Kind==InputEventRune.
	Rune rune
	// Size mirrors ReadRune's size return for compatibility adapters.
	Size int
	// Push carries feature settings when Kind==InputEventPushFeatures.
	Push []FeatureSetting
}

// Validate checks that event payload matches its kind.
func (ev InputEvent) Validate() error {
	switch ev.Kind {
	case InputEventRune:
		if len(ev.Push) != 0 {
			return fmt.Errorf("otshape: rune event must not carry push settings")
		}
	case InputEventPushFeatures:
		if len(ev.Push) == 0 {
			return fmt.Errorf("otshape: push-features event must carry at least one setting")
		}
	case InputEventPopFeatures:
		if ev.Rune != 0 || ev.Size != 0 || len(ev.Push) != 0 {
			return fmt.Errorf("otshape: pop-features event must not carry rune or push payload")
		}
	default:
		return fmt.Errorf("otshape: unknown input event kind %d", ev.Kind)
	}
	return nil
}

// InputEventSource is the advanced streaming input interface.
//
// It extends rune streaming with explicit feature push/pop events.
type InputEventSource interface {
	ReadEvent() (InputEvent, error)
}

type runeEventSourceAdapter struct {
	src RuneSource
}

// NewInputEventSource adapts a RuneSource to an InputEventSource.
//
// If src already implements InputEventSource, it is returned unchanged.
func NewInputEventSource(src RuneSource) InputEventSource {
	if src == nil {
		return nil
	}
	if evs, ok := src.(InputEventSource); ok {
		return evs
	}
	return runeEventSourceAdapter{src: src}
}

func (a runeEventSourceAdapter) ReadEvent() (InputEvent, error) {
	r, size, err := a.src.ReadRune()
	if err != nil {
		return InputEvent{}, err
	}
	return InputEvent{
		Kind: InputEventRune,
		Rune: r,
		Size: size,
	}, nil
}
