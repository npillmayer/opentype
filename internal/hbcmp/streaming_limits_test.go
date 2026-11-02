package hbcmp

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/othebrew"
)

func TestSHAPE015HebrewFutureMarkEventUsesWholeRunFallback(t *testing.T) {
	var selected *textBaseline
	for _, c := range loadTextBaselines(t) {
		if c.ID == "hebrew_vav_holam_mark_off" {
			selected = &c
			break
		}
	}
	if selected == nil {
		t.Fatal("missing Hebrew mark-off fixture")
	}
	font, err := ot.Parse(selected.fontData)
	if err != nil {
		t.Fatal(err)
	}
	params, err := selected.Fixture.params(font)
	if err != nil {
		t.Fatal(err)
	}
	// The root has mark disabled. An as-yet unread enabling event can change
	// presentation-form normalization of already buffered vav/holam groups.
	input := strings.Repeat("וֹ", 3000)
	source := &streamSource{runes: []rune(input)}
	for i, r := range source.runes {
		if i == 5000 {
			source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventPushFeatures, Push: []otshape.FeatureSetting{{Tag: ot.T("mark"), Enabled: true, Value: 1}}})
		}
		source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventRune, Rune: r, Size: 2})
	}
	source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventPopFeatures})
	sink := &streamSink{source: source, limit: len(source.runes) + 1}
	err = otshape.NewShaper(othebrew.New()).ShapeEvents(params, source, sink, otshape.BufferOptions{})
	if !errors.Is(err, otshape.ErrBufferFull) || source.consumed != otshape.MinBuffer || source.eof || len(sink.glyphs) != 0 {
		t.Fatalf("err=%v consumed=%d eof=%t output=%d", err, source.consumed, source.eof, len(sink.glyphs))
	}
}
