//go:build shaping_knownbugs

package otcore_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
)

// These tests assert intended behavior and currently fail. See
// doc/SHAPING_FINDINGS.md for evidence and promotion into the default suite.
// Shared fixture and assertion helpers live in shared_api_test.go.

type confidenceEngine struct {
	name  string
	score otshape.ShaperConfidence
}

func (e confidenceEngine) Name() string { return e.name }
func (e confidenceEngine) Match(otshape.SelectionContext) otshape.ShaperConfidence {
	return e.score
}
func (e confidenceEngine) New() otshape.ShapingEngine { return e }

func TestKnownBugSHAPE004RejectNonmatchingEngines(t *testing.T) {
	params, cp := alternateAPIInput(t)
	t.Run("positive_confidence_control", func(t *testing.T) {
		got := shapeAPIRecords(t, params, cp, otshape.BufferOptions{},
			confidenceEngine{name: "match", score: otshape.ShaperConfidenceLow})
		assertAPIGlyphs(t, got, []ot.GlyphIndex{20})
	})
	for _, tc := range []struct {
		name    string
		engines []otshape.ShapingEngine
	}{
		{"single_none", []otshape.ShapingEngine{confidenceEngine{name: "none"}}},
		{"all_none", []otshape.ShapingEngine{confidenceEngine{name: "b"}, confidenceEngine{name: "a"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.NewReader(cp)
			sink := &glyphCollector{}
			err := otshape.NewShaper(tc.engines...).Shape(params, src, sink, otshape.BufferOptions{})
			if !errors.Is(err, otshape.ErrNoMatchingShaper) {
				t.Errorf("Shape error = %v, want ErrNoMatchingShaper", err)
			}
			if len(sink.glyphs) != 0 {
				t.Errorf("nonmatching engines emitted %d glyphs, want none", len(sink.glyphs))
			}
			if src.Len() != len(cp) {
				t.Errorf("engine selection consumed input: %d bytes remain, want %d", src.Len(), len(cp))
			}
		})
	}
}
