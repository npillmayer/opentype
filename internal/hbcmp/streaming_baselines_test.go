package hbcmp

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
)

var streamingCases = map[string]string{
	"stream_ligatures": "", "stream_kerning": "", "stream_accents": "", "stream_hebrew": "",
	"stream_range": "", "scope_nested": "", "scope_pair": "", "scope_accent": "",
	"progress_expand": "", "progress_delete": "", "progress_all_deleted": "", "progress_contract": "",
}

func loadStreamingBaselines(t *testing.T) []textBaseline {
	t.Helper()
	cases := loadTextBaselineFile(t, "testdata/text/streaming.json", streamingCases, map[string]bool{"progress_all_deleted": true})
	for _, c := range loadTextBaselines(t) {
		switch c.ID {
		case "latin_office_liga_off", "latin_avatar_kern_off", "latin_uncomposable_mark", "hebrew_vav_holam_mark_on":
			cases = append(cases, c)
		}
	}
	return cases
}

type streamProfile struct {
	name           string
	high, low, max int
}

func streamProfiles(n int) []streamProfile {
	return []streamProfile{
		{"whole", n + 1, 1, n + 2},
		{"tiny", 1, 1, 2},
		{"lookahead", 3, 1, 8},
		{"forced", 3, 3, 3},
		{"carry", 5, 3, 8},
	}
}

// Nil settings means pop. Offsets count source scalars, never event records.
type scopeChange struct {
	at       int
	settings []otshape.FeatureSetting
}

func scopePush(at int, tag string, enabled bool) scopeChange {
	return scopeChange{at, []otshape.FeatureSetting{{Tag: ot.T(tag), Enabled: enabled, Value: 1}}}
}

func scopeRecipe(id string) []scopeChange {
	switch id {
	case "stream_range":
		return []scopeChange{scopePush(8, "liga", true), {at: 11}}
	case "scope_nested":
		return []scopeChange{scopePush(0, "liga", false), scopePush(7, "liga", true), {at: 14}, {at: 21}, scopePush(28, "liga", false), {at: 28}}
	case "scope_pair":
		return []scopeChange{scopePush(1, "liga", false), {at: 2}}
	case "scope_accent":
		return []scopeChange{scopePush(3, "liga", false), {at: 10}}
	case "empty_events":
		return []scopeChange{scopePush(0, "test", false), scopePush(0, "test", true), {at: 0}, {at: 0}}
	}
	return nil
}

// Build and independently validate the scope schedule against the explicit HB
// ranges at every input scalar. The event stream never comes from shaped output.
func streamEvents(t *testing.T, c textBaseline, features []otshape.FeatureRange) ([]otshape.InputEvent, []otshape.FeatureRange) {
	t.Helper()
	var globals []otshape.FeatureRange
	base := make(map[ot.Tag]int)
	value := func(f otshape.FeatureRange) int {
		if !f.On {
			return 0
		}
		return f.Arg
	}
	for _, f := range features {
		if f.Start == 0 && f.End == 0 {
			globals = append(globals, f)
			base[f.Feature] = value(f)
		}
	}
	stack := []map[ot.Tag]int{base}
	changes, cursor := scopeRecipe(c.ID), 0
	runes := []rune(c.Text)
	var events []otshape.InputEvent
	for i := 0; i <= len(runes); i++ {
		for cursor < len(changes) && changes[cursor].at == i {
			change := changes[cursor]
			if change.settings == nil {
				if len(stack) == 1 {
					t.Fatal("scope recipe underflow")
				}
				stack = stack[:len(stack)-1]
				events = append(events, otshape.InputEvent{Kind: otshape.InputEventPopFeatures})
			} else {
				next := make(map[ot.Tag]int)
				for tag, v := range stack[len(stack)-1] {
					next[tag] = v
				}
				for _, s := range change.settings {
					if s.Enabled {
						next[s.Tag] = s.Value
					} else {
						next[s.Tag] = 0
					}
				}
				stack = append(stack, next)
				events = append(events, otshape.InputEvent{Kind: otshape.InputEventPushFeatures, Push: change.settings})
			}
			cursor++
		}
		if i == len(runes) {
			break
		}
		want := make(map[ot.Tag]int)
		for _, f := range features {
			if (f.Start == 0 && f.End == 0) || (i >= f.Start && i < f.End) {
				want[f.Feature] = value(f)
			}
		}
		got := stack[len(stack)-1]
		if len(want) != len(got) {
			t.Fatalf("%s scope at %d has extra/missing feature", c.ID, i)
		}
		for tag, v := range want {
			if actual, ok := got[tag]; !ok || actual != v {
				t.Fatalf("%s scope %s at %d: got %d, want %d", c.ID, tag, i, actual, v)
			}
		}
		events = append(events, otshape.InputEvent{Kind: otshape.InputEventRune, Rune: runes[i], Size: utf8.RuneLen(runes[i])})
	}
	if cursor != len(changes) || len(stack) != 1 {
		t.Fatal("invalid or unclosed scope recipe")
	}
	return events, globals
}

type streamSource struct {
	runes                  []rune
	events                 []otshape.InputEvent
	index, consumed, calls int
	eof                    bool
}

func (s *streamSource) ReadRune() (rune, int, error) {
	s.calls++
	if s.calls > len(s.runes)+2 {
		return 0, 0, fmt.Errorf("rune source read budget exceeded")
	}
	if s.index == len(s.runes) {
		s.eof = true
		return 0, 0, io.EOF
	}
	r := s.runes[s.index]
	s.index++
	s.consumed++
	return r, utf8.RuneLen(r), nil
}

func (s *streamSource) ReadEvent() (otshape.InputEvent, error) {
	s.calls++
	if s.calls > len(s.events)+2 {
		return otshape.InputEvent{}, fmt.Errorf("event source read budget exceeded")
	}
	if s.index == len(s.events) {
		s.eof = true
		return otshape.InputEvent{}, io.EOF
	}
	e := s.events[s.index]
	s.index++
	if e.Kind == otshape.InputEventRune {
		s.consumed++
	}
	return e, nil
}

type streamEmission struct {
	consumed int
	eof      bool
}
type streamSink struct {
	source    *streamSource
	limit     int
	glyphs    []hbShapedGlyph
	emissions []streamEmission
}

func (s *streamSink) WriteGlyph(g otshape.GlyphRecord) error {
	if len(s.glyphs) >= s.limit {
		return fmt.Errorf("glyph output budget exceeded")
	}
	s.glyphs = append(s.glyphs, glyphRecord2hbShapedGlyph(g))
	s.emissions = append(s.emissions, streamEmission{s.source.consumed, s.source.eof})
	return nil
}

func runStreamCase(t *testing.T, c textBaseline, profile streamProfile, entry string, boundary otshape.FlushBoundary) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("shaping panic: %v", p)
		}
	}()
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
	params.Features, err = parseTextFeatures(c.Fixture.Context.Features, len(c.Fixture.Input))
	if err != nil {
		t.Fatal(err)
	}
	source := &streamSource{runes: []rune(c.Text)}
	if entry == "events" {
		source.events, params.Features = streamEvents(t, c, params.Features)
	}
	want := textLogicalOutput(c.Fixture)
	sink := &streamSink{source: source, limit: 4 * (len(source.runes) + len(want) + 1)}
	opts := otshape.BufferOptions{HighWatermark: profile.high, LowWatermark: profile.low, MaxBuffer: profile.max, FlushBoundary: boundary}
	shaper := otshape.NewShaper(otcore.New(), othebrew.New())
	if entry == "events" {
		err = shaper.ShapeEvents(params, source, sink, opts)
	} else {
		err = shaper.Shape(params, source, sink, opts)
	}
	if err != nil || !source.eof || source.consumed != len(source.runes) {
		t.Fatalf("error=%v consumed=%d/%d calls=%d EOF=%t opts=%+v", err, source.consumed, len(source.runes), source.calls, source.eof, opts)
	}
	// Observe emission relative to ingestion, without claiming exact internal
	// cuts. In the whole profile every glyph must be emitted after EOF.
	if profile.name == "whole" && len(sink.emissions) > 0 && !sink.emissions[0].eof {
		t.Fatal("whole-input run emitted before EOF")
	}
	if profile.name != "whole" && len(source.runes) > profile.max && len(want) > 0 && (len(sink.emissions) == 0 || sink.emissions[0].consumed == len(source.runes)) {
		t.Fatal("small-buffer case did not exercise output before all input was consumed")
	}
	t.Logf("input=%d calls=%d output=%d emissions=%v", source.consumed, source.calls, len(sink.glyphs), sink.emissions)
	if mismatch := comparePositionedGlyphs(sink.glyphs, want); mismatch != nil {
		t.Fatalf("%s opts=%+v: %v\ngot=%+v\nwant=%+v", c.ID, opts, mismatch, sink.glyphs, want)
	}
}

// Exact failing configurations are assigned after discovery. Never turn an
// unexpected pass/fail into a runtime skip or an assertion of buggy output.
func streamingFinding(id, profile, entry string) string {
	switch id {
	case "latin_uncomposable_mark", "hebrew_vav_holam_mark_on":
		if profile == "tiny" {
			return "015" // Attachment context is lost; the full oracle remains unchanged.
		}
		return ""
	case "progress_delete", "progress_all_deleted":
		return "017"
	case "scope_pair":
		if entry == "events" {
			return "016"
		}
	}
	if profile == "whole" {
		return ""
	}
	switch id {
	case "stream_ligatures":
		return "015"
	case "stream_accents":
		if profile != "forced" {
			return "015"
		}
	case "stream_kerning", "stream_hebrew", "stream_range", "scope_nested", "scope_pair", "scope_accent", "progress_contract":
		// SHAPE-018 fixes scope masks; tiny/forced cuts still lose context.
		if profile == "tiny" || profile == "forced" {
			return "015"
		}
	}
	return ""
}

func runStreamingMatrix(t *testing.T, finding string) {
	t.Helper()
	for _, c := range loadStreamingBaselines(t) {
		for _, p := range streamProfiles(len(c.Fixture.Input)) {
			for _, entry := range []string{"runes", "events"} {
				if streamingFinding(c.ID, p.name, entry) != finding {
					continue
				}
				for _, mode := range []struct {
					name     string
					boundary otshape.FlushBoundary
				}{{"run", otshape.FlushOnRunBoundary}, {"cluster", otshape.FlushOnClusterBoundary}} {
					t.Run(strings.Join([]string{c.ID, p.name, entry, mode.name}, "/"), func(t *testing.T) { runStreamCase(t, c, p, entry, mode.boundary) })
				}
			}
		}
	}
}

func TestHarfBuzzStreamingBaselines(t *testing.T) { runStreamingMatrix(t, "") }

func TestStreamingEmptyInput(t *testing.T) {
	cases := loadStreamingBaselines(t)
	i := slices.IndexFunc(cases, func(c textBaseline) bool { return c.ID == "progress_all_deleted" })
	c := cases[i]
	// Empty input has a direct API identity expectation, not a fabricated HB
	// capture. Use balanced nested empty scopes to exercise event-only EOF.
	c.ID = "empty_events"
	c.Text = ""
	c.Fixture.Input = nil
	c.Fixture.Output = []hbShapedGlyph{}
	for _, entry := range []string{"runes", "events"} {
		for _, p := range streamProfiles(0) {
			t.Run(entry+"/"+p.name, func(t *testing.T) { runStreamCase(t, c, p, entry, otshape.FlushOnRunBoundary) })
		}
	}
}
