package otlayout

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func limitFixture(t *testing.T, kind LayoutTagType) (*ot.Font, *ot.LookupListGraph) {
	t.Helper()
	if kind == GPosFeatureType {
		font := loadTestFont(t, "gpos_chaining3_boundary_f2.otf")
		return font, font.Layout.GPos.LookupGraph()
	}
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	return font, font.Layout.GSub.LookupGraph()
}

func limitContext(graph *ot.LookupListGraph, kind LayoutTagType, index int, refs ...uint16) {
	l := graph.Lookup(index)
	node := l.Subtable(0)
	l.Flag = 0
	records := make([]ot.SequenceLookupRecord, len(refs))
	for i, ref := range refs {
		records[i].LookupListIndex = ref
	}
	cov := []ot.Coverage{{GlyphRange: testGlyphRange{glyph: 10}}}
	if kind == GPosFeatureType {
		l.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeContextPos)
		*node = ot.LookupNode{LookupType: l.Type, Format: 3, GPos: &ot.GPosLookupPayload{ContextFmt3: &ot.GPosContextFmt3Payload{InputCoverages: cov, Records: records}}}
	} else {
		l.Type = ot.GSubLookupTypeContext
		*node = ot.LookupNode{LookupType: l.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: cov, Records: records}}}
	}
}

func limitLeaf(graph *ot.LookupListGraph, kind LayoutTagType, index int) {
	l := graph.Lookup(index)
	node := l.Subtable(0)
	l.Flag = 0
	cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
	if kind == GPosFeatureType {
		l.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeSingle)
		*node = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov, GPos: &ot.GPosLookupPayload{SingleFmt1: &ot.GPosSingleFmt1Payload{ValueFormat: 4, Value: ot.ValueRecord{XAdvance: 1}}}}
	} else {
		l.Type = ot.GSubLookupTypeSingle
		*node = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{}}}
	}
}

func TestContextCyclesAreBounded(t *testing.T) {
	for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
		for _, cycle := range []string{"self", "mutual", "after application"} {
			t.Run(fmt.Sprintf("%d/%s", kind, cycle), func(t *testing.T) {
				font, graph := limitFixture(t, kind)
				switch cycle {
				case "self":
					limitContext(graph, kind, 0, 0)
				case "mutual":
					limitContext(graph, kind, 0, 1)
					limitContext(graph, kind, 1, 0)
				default:
					limitLeaf(graph, kind, 1)
					limitContext(graph, kind, 0, 1, 0)
				}
				st := NewBufferState(GlyphBuffer{10}, nil)
				for i := 0; i < 2; i++ {
					pos, applied, err := ApplyFeature(font, errorTestFeature{kind, []int{0}}, st, 0)
					if !errors.Is(err, ErrLookupLimit) || !strings.Contains(err.Error(), "nesting depth") || !strings.Contains(err.Error(), "sequence record") {
						t.Fatalf("missing wrapped depth failure: %v", err)
					}
					if applied != (cycle == "after application") || pos != st.Index || st.execution != nil {
						t.Fatalf("bad unwind: applied=%v state=%+v", applied, st)
					}
					if cycle == "after application" {
						if kind == GSubFeatureType && len(st.AppliedEdits()) == 0 {
							t.Fatal("completed edits lost")
						}
						if kind == GPosFeatureType && (len(st.Pos) != 1 || st.Pos[0].XAdvance == 0) {
							t.Fatal("completed positioning lost")
						}
					}
				}
				// A non-matching input still succeeds: no persistent disabling or budget.
				st.Glyphs[0] = 11
				if _, applied, err := ApplyFeature(font, errorTestFeature{kind, []int{0}}, st, 0); err != nil || applied {
					t.Fatalf("later call affected: %v", err)
				}
			})
		}
	}
}

func TestContextWorkBudgetAndDepthBoundary(t *testing.T) {
	for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			font, graph := limitFixture(t, kind)
			limitContext(graph, kind, 0, 1, 1, 1)
			limitLeaf(graph, kind, 1)
			env, err := newApplyEnv(font)
			if err != nil {
				t.Fatal(err)
			}
			// One parent entry plus three record/child pairs costs seven units.
			for _, work := range []int{6, 7} {
				st := NewBufferState(GlyphBuffer{10}, nil)
				exec := &lookupExecution{remaining: work}
				st.execution = exec
				_, applied, _, err := applyLookupConcrete(graph.Lookup(0), graph, errorTestFeature{kind, []int{0}}, st, 0, nil, env)
				if !applied || exec.depth != 0 {
					t.Fatalf("bad partial result/unwind: %v %+v", applied, exec)
				}
				if work == 6 && !errors.Is(err, ErrLookupLimit) {
					t.Fatalf("expected work limit: %v", err)
				}
				if work == 7 && err != nil {
					t.Fatal(err)
				}
			}
			limitContext(graph, kind, 0, 1)
			for _, depth := range []int{maxLookupDepth - 2, maxLookupDepth - 1} {
				st := NewBufferState(GlyphBuffer{10}, nil)
				exec := &lookupExecution{depth: depth, remaining: 100}
				st.execution = exec
				_, _, _, err := applyLookupConcrete(graph.Lookup(0), graph, errorTestFeature{kind, []int{0}}, st, 0, nil, env)
				if exec.depth != depth {
					t.Fatal("depth not restored")
				}
				if depth == maxLookupDepth-2 && err != nil {
					t.Fatal(err)
				}
				if depth == maxLookupDepth-1 && !errors.Is(err, ErrLookupLimit) {
					t.Fatalf("expected depth limit: %v", err)
				}
			}
		})
	}
}

func TestPublicContextWorkBudget(t *testing.T) {
	font, graph := limitFixture(t, GSubFeatureType)
	refs := make([]uint16, maxLookupWork/2)
	for i := range refs {
		refs[i] = 1
	}
	limitContext(graph, GSubFeatureType, 0, refs...)
	limitLeaf(graph, GSubFeatureType, 1)
	st := NewBufferState(GlyphBuffer{10}, nil)
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if !applied || !errors.Is(err, ErrLookupLimit) || !strings.Contains(err.Error(), "work budget") {
		t.Fatalf("expected broad acyclic budget exhaustion: %v %v", applied, err)
	}
	if len(st.AppliedEdits()) != maxLookupWork/2-1 {
		t.Fatalf("wrong completed work: %d", len(st.AppliedEdits()))
	}
}

func TestContextBudgetsArePerApplication(t *testing.T) {
	font, graph := limitFixture(t, GSubFeatureType)
	limitContext(graph, GSubFeatureType, 0, 0)
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			st := NewBufferState(GlyphBuffer{10}, nil)
			_, _, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
			if !errors.Is(err, ErrLookupLimit) || st.execution != nil {
				t.Fatalf("unbounded/shared state: %v", err)
			}
		})
	}
}

type limitProgressCoverage struct{}

func (limitProgressCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g > 0 }
func (limitProgressCoverage) ByteSize() int                     { return 0 }

func TestTerminatingContextRecursion(t *testing.T) {
	font, graph := limitFixture(t, GSubFeatureType)
	limitContext(graph, GSubFeatureType, 0, 1, 0)
	limitLeaf(graph, GSubFeatureType, 1)
	cov := ot.Coverage{GlyphRange: limitProgressCoverage{}}
	graph.Lookup(0).Subtable(0).GSub.ContextFmt3.InputCoverages = []ot.Coverage{cov}
	child := graph.Lookup(1).Subtable(0)
	child.Coverage = cov
	child.GSub.SingleFmt1.DeltaGlyphID = -1
	for _, input := range []ot.GlyphIndex{4, maxLookupDepth + 2} {
		st := NewBufferState(GlyphBuffer{input}, nil)
		_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
		if !applied {
			t.Fatal("completed substitutions lost")
		}
		if input == 4 {
			if err != nil || st.Glyphs[0] != 0 {
				t.Fatalf("valid bounded recursion rejected: %v %v", st.Glyphs, err)
			}
		} else if !errors.Is(err, ErrLookupLimit) || st.Glyphs[0] >= input {
			t.Fatalf("excessive terminating recursion not bounded: %v %v", st.Glyphs, err)
		}
	}
}
