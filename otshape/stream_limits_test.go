package otshape

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestSHAPE015InputGroupLimit(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	for _, events := range []bool{false, true} {
		for _, prefix := range []string{"a", "\u0301", "\u200d"} {
			for _, count := range []int{MaxInputGroup, MaxInputGroup + 1} {
				t.Run(fmt.Sprintf("events=%t/first=%U/count=%d", events, []rune(prefix)[0], count), func(t *testing.T) {
					input := "x" + prefix + strings.Repeat("\u0301", count-1)
					// Leading marks/ZWJ extend x; an ordinary base begins a fresh group.
					start := uint32(1)
					if prefix != "a" {
						input = prefix + strings.Repeat("\u0301", count-1)
						start = 0
					}
					sink := &collectSink{}
					shaper := NewShaper(clusterPolicyEngine{NormalizationNone})
					opts := BufferOptions{HighWatermark: 8192, MaxBuffer: 8192}
					var err error
					if events {
						src := &sliceEventSource{}
						for i, r := range []rune(input) {
							if i == 2 {
								src.events = append(src.events, InputEvent{Kind: InputEventPushFeatures, Push: []FeatureSetting{{Tag: ot.T("test"), Enabled: false}}})
							}
							src.events = append(src.events, InputEvent{Kind: InputEventRune, Rune: r})
						}
						src.events = append(src.events, InputEvent{Kind: InputEventPopFeatures})
						err = shaper.ShapeEvents(standardParams(font), src, sink, opts)
					} else {
						err = shaper.Shape(standardParams(font), strings.NewReader(input), sink, opts)
					}
					if count == MaxInputGroup {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					var limit *LimitError
					if !errors.Is(err, ErrSupportedLimit) || !errors.As(err, &limit) || limit.Index != start || limit.Limit != MaxInputGroup || len(sink.glyphs) != 0 {
						t.Fatalf("err=%v limit=%+v output=%d", err, limit, len(sink.glyphs))
					}
				})
			}
		}
	}
}

func TestSHAPE015NormalizationEmergencyLimit(t *testing.T) {
	// U+0344 decomposes into two marks. Each initial group remains tiny.
	input := []rune(strings.Repeat("a\u0344", MaxIntermediateEntries/3+1))
	clusters := make([]uint32, len(input))
	for i := range clusters {
		clusters[i] = uint32(i)
	}
	ws := newShapeWorkspace(16)
	ws.normalize(input, clusters, nil, SelectionContext{}, clusterPolicyEngine{NormalizationDecomposed}, nil)
	var limit *LimitError
	if !errors.As(ws.err, &limit) || limit.Resource != "normalized rune buffer" {
		t.Fatalf("error=%v", ws.err)
	}
	if len(ws.normRunesA) > MaxIntermediateEntries || len(ws.normRunesB) > MaxIntermediateEntries {
		t.Fatal("normalized buffer exceeded cap")
	}
}

func TestSHAPE015HookExpansionEmergencyLimit(t *testing.T) {
	run := newRunBuffer(1)
	run.Glyphs = otlayout.GlyphBuffer{1}
	run.maxEntries = MaxIntermediateEntries
	run.InsertGlyphCopies(1, 0, MaxIntermediateEntries-1)
	if run.Len() != MaxIntermediateEntries || run.limitErr != nil {
		t.Fatalf("len=%d err=%v", run.Len(), run.limitErr)
	}
	run.InsertGlyphCopies(1, 0, 1)
	if run.Len() != MaxIntermediateEntries || !errors.Is(run.limitErr, ErrSupportedLimit) {
		t.Fatalf("len=%d err=%v", run.Len(), run.limitErr)
	}
}

func TestSHAPE015CustomEngineWholeRunFallback(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	for _, events := range []bool{false, true} {
		for _, size := range []int{MinBuffer - 1, MinBuffer, MinBuffer + 1} {
			input := strings.Repeat("a", size)
			sink := &collectSink{}
			shaper := NewShaper(&hookProbeShaper{})
			var err error
			if events {
				err = shaper.ShapeEvents(standardParams(font), NewInputEventSource(strings.NewReader(input)), sink, BufferOptions{})
			} else {
				err = shaper.Shape(standardParams(font), strings.NewReader(input), sink, BufferOptions{})
			}
			if size < MinBuffer {
				if err != nil || len(sink.glyphs) != size {
					t.Fatalf("size=%d err=%v output=%d", size, err, len(sink.glyphs))
				}
			} else if !errors.Is(err, ErrBufferFull) || len(sink.glyphs) != 0 {
				t.Fatalf("size=%d err=%v output=%d", size, err, len(sink.glyphs))
			}
		}
	}
}

func TestSHAPE015PublicGlyphEmergencyLimit(t *testing.T) {
	for _, events := range []bool{false, true} {
		font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
		l := font.Layout.GSub.LookupGraph().Lookup(0)
		l.Type = ot.GSubLookupTypeMultiple
		l.Flag = 0
		replacement := make([]ot.GlyphIndex, MaxIntermediateEntries+1)
		for i := range replacement {
			replacement[i] = 18
		}
		*l.Subtable(0) = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: outputPairCoverage{}}, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{replacement}}}}
		params := standardParams(font)
		params.Features = []FeatureRange{{Feature: ot.T("test"), On: true}}
		sink := &collectSink{}
		s := NewShaper(clusterPolicyEngine{NormalizationNone})
		var err error
		if events {
			err = s.ShapeEvents(params, NewInputEventSource(strings.NewReader(string(rune(18)))), sink, BufferOptions{})
		} else {
			err = s.Shape(params, strings.NewReader(string(rune(18))), sink, BufferOptions{})
		}
		if !errors.Is(err, ErrSupportedLimit) || !errors.Is(err, otlayout.ErrGlyphLimit) || len(sink.glyphs) != 0 {
			t.Fatalf("events=%t err=%v output=%d", events, err, len(sink.glyphs))
		}
	}
}

func TestSHAPE015FutureSharedLookupPolicyFallback(t *testing.T) {
	features := []otlayout.Feature{
		compiledFeature{tag: ot.T("salt"), typ: otlayout.GSubFeatureType, lookups: []int{1}},
		compiledFeature{tag: ot.T("ccmp"), typ: otlayout.GSubFeatureType, lookups: []int{1}},
	}
	// ccmp's manual ZWJ policy can alter an existing shared salt operation when
	// selected by a later event, even outside that event's scalar range.
	if !unstableEventLookupPolicy(features, planGSUB, true) {
		t.Fatal("shared joiner policy was certified")
	}
	if unstableEventLookupPolicy(features, planGSUB, false) {
		t.Fatal("joiner-free text unnecessarily rejected")
	}
	features[1] = compiledFeature{tag: ot.T("rand"), typ: otlayout.GSubFeatureType, lookups: []int{1}}
	if !unstableEventLookupPolicy(features, planGSUB, false) {
		t.Fatal("shared random policy was certified")
	}
	features[1] = compiledFeature{tag: ot.T("rand"), typ: otlayout.GSubFeatureType, lookups: []int{2}}
	if unstableEventLookupPolicy(features, planGSUB, false) {
		t.Fatal("unshared operations unnecessarily rejected")
	}
}
