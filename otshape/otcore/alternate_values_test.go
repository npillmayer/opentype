package otcore_test

import (
	"fmt"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
)

// Promoted from shaping_knownbugs. The paired TTX lists [20,21,22] for GID 18.
func TestSHAPE003FeatureValueSelectsAlternate(t *testing.T) {
	for _, api := range []string{"Shape", "ShapeEvents"} {
		for _, tc := range []struct {
			value int
			want  ot.GlyphIndex
		}{
			{0, 20}, {1, 20}, {2, 21}, {3, 22}, {4, 18}, {256, 18}, {257, 18}, {math.MaxInt, 18},
			{-1, 0}, {-2, 0}, {math.MinInt, 0},
		} {
			t.Run(fmt.Sprintf("%s/%d", api, tc.value), func(t *testing.T) {
				params, cp := alternateAPIInput(t)
				params.Features[0].Arg = tc.value
				sink := &glyphCollector{}
				shaper := otshape.NewShaper(otcore.New())
				src := strings.NewReader(strings.Repeat(cp, 4))
				var err error
				if api == "ShapeEvents" {
					err = shaper.ShapeEvents(params, otshape.NewInputEventSource(src), sink, otshape.BufferOptions{})
				} else {
					err = shaper.Shape(params, src, sink, otshape.BufferOptions{})
				}
				if err != nil {
					t.Fatal(err)
				}
				if tc.value >= 0 {
					assertAPIGlyphs(t, sink.glyphs, []ot.GlyphIndex{tc.want, tc.want, tc.want, tc.want})
				} else {
					// Fixed-seed selection and variation are checked in otlayout and
					// otshape. Public calls have independent, undisclosed seeds.
					if len(sink.glyphs) != 4 {
						t.Fatalf("got %v", sink.glyphs)
					}
					for i, g := range sink.glyphs {
						if g.GID < 20 || g.GID > 22 || g.Cluster != uint32(i) {
							t.Fatalf("invalid random alternate: %+v", g)
						}
					}
				}
			})
		}
	}
}

func TestSHAPE003OverlappingAlternateRanges(t *testing.T) {
	params, cp := alternateAPIInput(t)
	params.Features = []otshape.FeatureRange{
		{Feature: ot.T("test"), On: true, Arg: 2},
		{Feature: ot.T("test"), On: true, Arg: 3, Start: 1, End: 5},
		{Feature: ot.T("test"), On: true, Arg: 257, Start: 2, End: 4},
		{Feature: ot.T("test"), On: false, Start: 3, End: 4},
		{Feature: ot.T("test"), On: true, Arg: 0, Start: 4, End: 5},
	}
	for _, opts := range []otshape.BufferOptions{
		{HighWatermark: 16, LowWatermark: 4, MaxBuffer: 16},
		{HighWatermark: 2, LowWatermark: 1, MaxBuffer: 4},
	} {
		got := shapeAPIRecords(t, params, strings.Repeat(cp, 6), opts, otcore.New())
		assertAPIGlyphs(t, got, []ot.GlyphIndex{21, 22, 18, 18, 20, 21})
	}
}

type alternateEventSource struct{ events []otshape.InputEvent }

func (s *alternateEventSource) ReadEvent() (otshape.InputEvent, error) {
	if len(s.events) == 0 {
		return otshape.InputEvent{}, io.EOF
	}
	e := s.events[0]
	s.events = s.events[1:]
	return e, nil
}

func TestSHAPE003NestedAlternateScopes(t *testing.T) {
	params, cp := alternateAPIInput(t)
	params.Features[0].Arg = 2
	r := otshape.InputEvent{Kind: otshape.InputEventRune, Rune: []rune(cp)[0], Size: len(cp)}
	push := func(value int, on bool) otshape.InputEvent {
		return otshape.InputEvent{Kind: otshape.InputEventPushFeatures, Push: []otshape.FeatureSetting{{Tag: ot.T("test"), Value: value, Enabled: on}}}
	}
	pop := otshape.InputEvent{Kind: otshape.InputEventPopFeatures}
	for _, opts := range []otshape.BufferOptions{
		{HighWatermark: 16, LowWatermark: 4, MaxBuffer: 16},
		{HighWatermark: 2, LowWatermark: 1, MaxBuffer: 4},
	} {
		src := &alternateEventSource{events: []otshape.InputEvent{
			r, push(3, true), r, push(257, true), r, pop, r,
			push(0, false), r, pop, r, push(0, true), r, pop, r, pop, r,
		}}
		sink := &glyphCollector{}
		if err := otshape.NewShaper(otcore.New()).ShapeEvents(params, src, sink, opts); err != nil {
			t.Fatal(err)
		}
		assertAPIGlyphs(t, sink.glyphs, []ot.GlyphIndex{21, 22, 18, 22, 18, 22, 20, 22, 21})
	}
}

// SHAPE-018: the lookup must use the settings at the actual starting glyph.
func TestSHAPE018AlternateValueAtMatchedGlyph(t *testing.T) {
	params, cp := alternateAPIInput(t)
	params.Features = []otshape.FeatureRange{
		{Feature: ot.T("test"), On: true, Arg: 1},
		{Feature: ot.T("test"), On: true, Arg: 3, Start: 1, End: 2},
	}
	got := shapeAPIRecords(t, params, string(rune(17))+cp, otshape.BufferOptions{}, otcore.New())
	assertAPIGlyphs(t, got, []ot.GlyphIndex{17, 22})
}

func TestSHAPE018ScopesAfterUncoveredGlyph(t *testing.T) {
	for _, api := range []string{"Shape", "ShapeEvents"} {
		for _, opts := range []otshape.BufferOptions{
			{HighWatermark: 8, LowWatermark: 4, MaxBuffer: 8},
			{HighWatermark: 2, LowWatermark: 1, MaxBuffer: 4},
		} {
			t.Run(fmt.Sprintf("%s/high=%d", api, opts.HighWatermark), func(t *testing.T) {
				params, cp := alternateAPIInput(t)
				params.Features[0].Arg = 1
				sink := &glyphCollector{}
				shaper := otshape.NewShaper(otcore.New())
				var err error
				if api == "Shape" {
					params.Features = append(params.Features,
						otshape.FeatureRange{Feature: ot.T("test"), On: false, Start: 1, End: 2},
						otshape.FeatureRange{Feature: ot.T("test"), On: true, Arg: 3, Start: 3, End: 4})
					err = shaper.Shape(params, strings.NewReader(string(rune(17))+cp+string(rune(17))+cp), sink, opts)
				} else {
					uncovered := otshape.InputEvent{Kind: otshape.InputEventRune, Rune: 17, Size: 1}
					covered := otshape.InputEvent{Kind: otshape.InputEventRune, Rune: []rune(cp)[0], Size: len(cp)}
					pop := otshape.InputEvent{Kind: otshape.InputEventPopFeatures}
					src := &alternateEventSource{events: []otshape.InputEvent{
						uncovered, {Kind: otshape.InputEventPushFeatures, Push: []otshape.FeatureSetting{{Tag: ot.T("test"), Enabled: false}}}, covered, pop,
						uncovered, {Kind: otshape.InputEventPushFeatures, Push: []otshape.FeatureSetting{{Tag: ot.T("test"), Enabled: true, Value: 3}}}, covered, pop,
					}}
					err = shaper.ShapeEvents(params, src, sink, opts)
				}
				if err != nil {
					t.Fatal(err)
				}
				assertAPIGlyphs(t, sink.glyphs, []ot.GlyphIndex{17, 18, 17, 22})
			})
		}
	}
}
