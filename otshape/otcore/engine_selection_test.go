package otcore_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
)

// SHAPE-004: selection must reject nonmatching engines before any input/output.
// Shared fixture and assertion helpers live in shared_api_test.go.

type confidenceEngine struct {
	name    string
	score   otshape.ShaperConfidence
	created *[]string
}

func (e confidenceEngine) Name() string { return e.name }
func (e confidenceEngine) Match(otshape.SelectionContext) otshape.ShaperConfidence {
	return e.score
}
func (e confidenceEngine) New() otshape.ShapingEngine {
	if e.created != nil {
		*e.created = append(*e.created, e.name)
	}
	return e
}

func TestSHAPE004EngineSelection(t *testing.T) {
	params, cp := alternateAPIInput(t)
	for _, tc := range []struct {
		name     string
		engines  []otshape.ShapingEngine
		wantErr  error
		selected string
	}{
		{"positive_confidence_control", []otshape.ShapingEngine{confidenceEngine{name: "match", score: otshape.ShaperConfidenceLow}}, nil, "match"},
		{"single_none", []otshape.ShapingEngine{confidenceEngine{name: "none"}}, otshape.ErrNoMatchingShaper, ""},
		{"all_none", []otshape.ShapingEngine{confidenceEngine{name: "b"}, confidenceEngine{name: "a"}}, otshape.ErrNoMatchingShaper, ""},
		{"negative_and_nil", []otshape.ShapingEngine{nil, confidenceEngine{name: "negative", score: -1}}, otshape.ErrNoMatchingShaper, ""},
		{"nil_only", []otshape.ShapingEngine{nil}, otshape.ErrNoShaper, ""},
		{"no_candidates", nil, otshape.ErrNoShaper, ""},
		{"mixed", []otshape.ShapingEngine{confidenceEngine{name: "a-none"}, nil, confidenceEngine{name: "match", score: otshape.ShaperConfidenceLow}}, nil, "match"},
		{"highest_score", []otshape.ShapingEngine{confidenceEngine{name: "a-low", score: otshape.ShaperConfidenceLow}, confidenceEngine{name: "z-certain", score: otshape.ShaperConfidenceCertain}}, nil, "z-certain"},
		{"positive_tie", []otshape.ShapingEngine{confidenceEngine{name: "b", score: otshape.ShaperConfidenceLow}, confidenceEngine{name: "a", score: otshape.ShaperConfidenceLow}}, nil, "a"},
	} {
		for _, api := range []string{"Shape", "ShapeEvents"} {
			t.Run(tc.name+"/"+api, func(t *testing.T) {
				var created []string
				engines := append([]otshape.ShapingEngine(nil), tc.engines...)
				for i, candidate := range engines {
					if engine, ok := candidate.(confidenceEngine); ok {
						engine.created = &created
						engines[i] = engine
					}
				}
				src := strings.NewReader(cp)
				sink := &glyphCollector{}
				shaper := otshape.NewShaper(engines...)
				var err error
				if api == "ShapeEvents" {
					err = shaper.ShapeEvents(params, otshape.NewInputEventSource(src), sink, otshape.BufferOptions{})
				} else {
					err = shaper.Shape(params, src, sink, otshape.BufferOptions{})
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error=%v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if len(sink.glyphs) != 0 || src.Len() != len(cp) || len(created) != 0 {
						t.Fatalf("failed selection had effects: glyphs=%v bytes left=%d created=%v", sink.glyphs, src.Len(), created)
					}
				} else {
					assertAPIGlyphs(t, sink.glyphs, []ot.GlyphIndex{20})
					// Event carries may create another instance of the selected engine.
					if len(created) == 0 {
						t.Fatal("selected engine was not instantiated")
					}
					for _, name := range created {
						if name != tc.selected {
							t.Fatalf("created=%v, want only %q", created, tc.selected)
						}
					}
				}
			})
		}
	}
}
