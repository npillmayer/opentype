package hbcmp

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
)

func deletionParams(t *testing.T) otshape.Params {
	t.Helper()
	for _, c := range loadStreamingBaselines(t) {
		if c.ID != "progress_all_deleted" {
			continue
		}
		font, err := ot.Parse(c.fontData)
		if err != nil {
			t.Fatal(err)
		}
		params, err := c.Fixture.params(font)
		if err != nil {
			t.Fatal(err)
		}
		return params
	}
	t.Fatal("missing deletion fixture")
	return otshape.Params{}
}

func shapeDeletionInput(t *testing.T, params otshape.Params, input, entry string, high, max int, boundary otshape.FlushBoundary) (*streamSource, *streamSink, error) {
	t.Helper()
	source := &streamSource{runes: []rune(input)}
	for _, r := range source.runes {
		source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventRune, Rune: r, Size: 1})
	}
	sink := &streamSink{source: source, limit: len(source.runes) + 1}
	opts := otshape.BufferOptions{HighWatermark: high, LowWatermark: 1, MaxBuffer: max, FlushBoundary: boundary}
	shaper := otshape.NewShaper(otcore.New())
	var err error
	if entry == "events" {
		err = shaper.ShapeEvents(params, source, sink, opts)
	} else {
		err = shaper.Shape(params, source, sink, opts)
	}
	return source, sink, err
}

func TestSHAPE017ZeroOutputLimit(t *testing.T) {
	params := deletionParams(t)
	for _, tc := range []struct {
		name, input   string
		max, consumed int
		eof, limit    bool
		want          []hbShapedGlyph
	}{
		{"EOF", "ddd", 4, 3, true, false, nil},
		{"capacity", strings.Repeat("d", otshape.MinBuffer+1), otshape.MinBuffer, otshape.MinBuffer, false, true, nil},
		{"exact_capacity", strings.Repeat("d", otshape.MinBuffer), otshape.MinBuffer, otshape.MinBuffer, false, true, nil},
		{"earlier_output", "  " + strings.Repeat("d", otshape.MinBuffer), otshape.MinBuffer, otshape.MinBuffer + 1, false, true, []hbShapedGlyph{{G: 7, Cl: 0, AX: 600}}},
	} {
		for _, entry := range []string{"runes", "events"} {
			for _, boundary := range []otshape.FlushBoundary{otshape.FlushOnRunBoundary, otshape.FlushOnClusterBoundary} {
				t.Run(tc.name+"/"+entry+"/"+map[otshape.FlushBoundary]string{otshape.FlushOnRunBoundary: "run", otshape.FlushOnClusterBoundary: "cluster"}[boundary], func(t *testing.T) {
					source, sink, err := shapeDeletionInput(t, params, tc.input, entry, 1, tc.max, boundary)
					sentinel := otshape.ErrZeroOutputBufferFull
					if tc.name == "earlier_output" {
						sentinel = otshape.ErrBufferFull
					}
					if errors.Is(err, sentinel) != tc.limit || (!tc.limit && err != nil) {
						t.Fatalf("error=%v limit=%v", err, tc.limit)
					}
					if source.consumed != tc.consumed || source.eof != tc.eof || source.calls > tc.consumed+1 {
						t.Fatalf("source=%+v", source)
					}
					if mismatch := comparePositionedGlyphs(sink.glyphs, tc.want); mismatch != nil {
						t.Fatalf("output=%v: %v", sink.glyphs, mismatch)
					}
				})
			}
		}
	}
}

type deletionCoverage ot.GlyphIndex

func (c deletionCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == ot.GlyphIndex(c) }
func (deletionCoverage) ByteSize() int                       { return 0 }

func TestSHAPE017EmptyCarryRetainsLookaheadContext(t *testing.T) {
	params := deletionParams(t)
	graph := params.Font.Layout.GSub.LookupGraph()
	// Controlled rules: d -> a before a; otherwise the next lookup deletes d.
	// Shaping "d" alone is empty, but shaping "da" must produce "aa" and kern.
	// These explicit rules establish the expectation, not the shaper's output.
	lookup := graph.Lookup(0)
	lookup.Type, lookup.Flag = ot.GSubLookupTypeReverseChaining, 0
	*lookup.Subtable(0) = ot.LookupNode{LookupType: lookup.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: deletionCoverage(3)}, GSub: &ot.GSubLookupPayload{ReverseChainingFmt1: &ot.GSubReverseChainingFmt1Payload{LookaheadCoverages: []ot.Coverage{{GlyphRange: deletionCoverage(1)}}, SubstituteGlyphIDs: []ot.GlyphIndex{1}}}}
	lookup = graph.Lookup(1)
	lookup.Type, lookup.Flag = ot.GSubLookupTypeMultiple, 0
	*lookup.Subtable(0) = ot.LookupNode{LookupType: lookup.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: deletionCoverage(3)}, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{}}}}}
	for _, entry := range []string{"runes", "events"} {
		t.Run(entry, func(t *testing.T) {
			source, sink, err := shapeDeletionInput(t, params, "da", entry, 1, 3, otshape.FlushOnRunBoundary)
			want := []hbShapedGlyph{{G: 1, Cl: 0, AX: 580}, {G: 1, Cl: 1, AX: 600}}
			if mismatch := comparePositionedGlyphs(sink.glyphs, want); err != nil || mismatch != nil || !source.eof {
				t.Fatalf("err=%v comparison=%v output=%v EOF=%v", err, mismatch, sink.glyphs, source.eof)
			}
			for _, emission := range sink.emissions {
				if !emission.eof {
					t.Fatalf("provisional result emitted: %+v", emission)
				}
			}
		})
	}
}

func TestSHAPE017DeletedEventScopeStillRequiresPop(t *testing.T) {
	for _, closed := range []bool{false, true} {
		params := deletionParams(t)
		source := &streamSource{runes: []rune("d"), events: []otshape.InputEvent{{Kind: otshape.InputEventPushFeatures, Push: []otshape.FeatureSetting{{Tag: ot.T("test"), Enabled: true, Value: 1}}}, {Kind: otshape.InputEventRune, Rune: 'd', Size: 1}}}
		if closed {
			source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventPopFeatures})
		}
		sink := &glyphCollector{}
		err := otshape.NewShaper(otcore.New()).ShapeEvents(params, source, sink, otshape.BufferOptions{HighWatermark: 2, LowWatermark: 1, MaxBuffer: 3})
		if (err != nil) == closed || len(sink.glyphs) != 0 || !source.eof {
			t.Fatalf("closed=%v err=%v output=%v EOF=%v", closed, err, sink.glyphs, source.eof)
		}
		if !closed && !strings.Contains(err.Error(), "feature-plan stack not closed") {
			t.Fatalf("wrong EOF error: %v", err)
		}
	}
}
