package hbcmp

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
)

func TestSHAPE020EventBaselines(t *testing.T) {
	for _, id := range []string{"latin_lookup_order_liga_first", "latin_lookup_order_calt_first", "latin_position_order_kern_first", "latin_position_order_mark_first", "latin_shared_alternate_salt", "latin_shared_alternate_ss01", "latin_shared_alternate_equal"} {
		t.Run(id, func(t *testing.T) {
			c := textBaselineByID(t, id)
			n := len(c.Fixture.Input)
			runStreamCase(t, c, streamProfile{"whole", n + 1, 1, n + 2}, "events", otshape.FlushOnRunBoundary)
		})
	}
}

func TestSHAPE020PublicAlternateConflict(t *testing.T) {
	c := textBaselineByID(t, "latin_shared_alternate_equal")
	font, err := ot.Parse(c.fontData)
	if err != nil {
		t.Fatal(err)
	}
	f := c.Fixture
	f.Context.Features = nil
	params, err := f.params(font)
	if err != nil {
		t.Fatal(err)
	}
	params.Features = []otshape.FeatureRange{{Feature: ot.T("salt"), On: true, Arg: 1}, {Feature: ot.T("ss01"), On: true, Arg: 2}}
	for _, entry := range []string{"runes", "events"} {
		t.Run(entry, func(t *testing.T) {
			sink := &glyphCollector{}
			shaper := otshape.NewShaper(otcore.New())
			opts := otshape.BufferOptions{HighWatermark: 2, LowWatermark: 1, MaxBuffer: 3, FlushBoundary: otshape.FlushOnRunBoundary}
			var shapeErr error
			if entry == "events" {
				source := &streamSource{runes: []rune(c.Text)}
				source.events, _ = streamEvents(t, c, params.Features)
				shapeErr = shaper.ShapeEvents(params, source, sink, opts)
			} else {
				shapeErr = shaper.Shape(params, strings.NewReader(c.Text), sink, opts)
			}
			// This is the explicitly agreed API contract, not HB's packed-mask
			// result for incompatible values. No failing chunk may be emitted.
			if !errors.Is(shapeErr, otshape.ErrConflictingAlternates) || len(sink.glyphs) != 0 {
				t.Fatalf("error=%v output=%+v", shapeErr, sink.glyphs)
			}
		})
	}
}
