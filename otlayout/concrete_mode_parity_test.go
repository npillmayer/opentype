package otlayout

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func runWithLookupMode[T any](mode LookupExecutionMode, fn func() T) T {
	prev := SetLookupExecutionMode(mode)
	defer SetLookupExecutionMode(prev)
	return fn()
}

func TestConcreteOnlyDisablesLegacyFallback(t *testing.T) {
	sub := ot.LookupSubtable{
		LookupType: ot.GSubLookupTypeSingle,
		Format:     1,
		Coverage: ot.Coverage{
			GlyphRange: testGlyphRange{glyph: 10},
		},
		Support: ot.GlyphIndex(2),
	}

	concreteFirstApplied := runWithLookupMode(ConcreteFirst, func() bool {
		ctx := applyCtx{
			lookup: &ot.Lookup{},
			buf:    &BufferState{Glyphs: GlyphBuffer{10}},
			pos:    0,
		}
		_, ok, _, _, _ := dispatchGSubLookup(&ctx, &sub)
		return ok
	})
	if !concreteFirstApplied {
		t.Fatalf("expected legacy fallback to apply in ConcreteFirst mode")
	}

	concreteOnlyApplied := runWithLookupMode(ConcreteOnly, func() bool {
		ctx := applyCtx{
			lookup: &ot.Lookup{},
			buf:    &BufferState{Glyphs: GlyphBuffer{10}},
			pos:    0,
		}
		_, ok, _, _, _ := dispatchGSubLookup(&ctx, &sub)
		return ok
	})
	if concreteOnlyApplied {
		t.Fatalf("expected legacy fallback to be disabled in ConcreteOnly mode")
	}
}

func TestConcreteOnlyGSUBParity(t *testing.T) {
	type gsubCase struct {
		name   string
		font   string
		lookup int
		input  []ot.GlyphIndex
		pos    int
		alt    int
	}
	cases := []gsubCase{
		{
			name:   "alternate_fmt1",
			font:   "gsub3_1_simple_f1.otf",
			lookup: 0,
			input:  []ot.GlyphIndex{18},
			pos:    0,
			alt:    1,
		},
		{
			name:   "context_fmt1_ignoremarks",
			font:   "gsub_context1_lookupflag_f1.otf",
			lookup: 4,
			input:  []ot.GlyphIndex{20, 21, 22},
			pos:    0,
			alt:    0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			otf := loadTestFont(t, tc.font)
			concreteFirst := runWithLookupMode(ConcreteFirst, func() struct {
				out     []ot.GlyphIndex
				applied bool
			} {
				out, applied := applyGSUBLookup(t, otf, tc.lookup, tc.input, tc.pos, tc.alt)
				return struct {
					out     []ot.GlyphIndex
					applied bool
				}{
					out:     append([]ot.GlyphIndex(nil), out...),
					applied: applied,
				}
			})
			concreteOnly := runWithLookupMode(ConcreteOnly, func() struct {
				out     []ot.GlyphIndex
				applied bool
			} {
				out, applied := applyGSUBLookup(t, otf, tc.lookup, tc.input, tc.pos, tc.alt)
				return struct {
					out     []ot.GlyphIndex
					applied bool
				}{
					out:     append([]ot.GlyphIndex(nil), out...),
					applied: applied,
				}
			})

			if concreteOnly.applied != concreteFirst.applied {
				t.Fatalf("applied mismatch: concrete-only=%v concrete-first=%v", concreteOnly.applied, concreteFirst.applied)
			}
			if len(concreteOnly.out) != len(concreteFirst.out) {
				t.Fatalf("glyph length mismatch: concrete-only=%d concrete-first=%d", len(concreteOnly.out), len(concreteFirst.out))
			}
			for i := range concreteOnly.out {
				if concreteOnly.out[i] != concreteFirst.out[i] {
					t.Fatalf("glyph[%d] mismatch: concrete-only=%d concrete-first=%d", i, concreteOnly.out[i], concreteFirst.out[i])
				}
			}
		})
	}
}

func TestConcreteOnlyGPOSParity(t *testing.T) {
	type gposRun struct {
		applied bool
		glyphs  []ot.GlyphIndex
		pos     []PosItem
	}
	runGPOS := func(mode LookupExecutionMode, otf *ot.Font, lookup int, input []ot.GlyphIndex, pos int) gposRun {
		return runWithLookupMode(mode, func() gposRun {
			st, applied := applyGPOSLookup(t, otf, lookup, input, pos)
			out := gposRun{
				applied: applied,
				glyphs:  append([]ot.GlyphIndex(nil), st.Glyphs...),
				pos:     append([]PosItem(nil), st.Pos...),
			}
			return out
		})
	}

	t.Run("single_pair_chaining", func(t *testing.T) {
		otf := loadTestFont(t, "gpos_chaining3_boundary_f2.otf")
		graph := otf.Layout.GPos.LookupGraph()
		if graph == nil {
			t.Fatalf("font has no concrete GPOS lookup graph")
		}

		singleNode := graph.Lookup(0).Subtable(0)
		singleGlyph := firstCoveredGlyph(t, otf, singleNode.Coverage)
		pairNode := graph.Lookup(1).Subtable(0)
		pairPayload := pairNode.GPosPayload().PairFmt1
		if pairPayload == nil {
			t.Fatalf("expected PairFmt1 payload")
		}
		firstPair := firstCoveredGlyph(t, otf, pairNode.Coverage)
		row, ok := pairNode.Coverage.Match(firstPair)
		if !ok || row < 0 || row >= len(pairPayload.PairSets) || len(pairPayload.PairSets[row]) == 0 {
			t.Fatalf("invalid pair payload row")
		}
		secondPair := ot.GlyphIndex(pairPayload.PairSets[row][0].SecondGlyph)

		chainingNode := graph.Lookup(4).Subtable(0)
		chainingPayload := chainingNode.GPosPayload().ChainingContextFmt3
		if chainingPayload == nil {
			t.Fatalf("expected ChainingContextFmt3 payload")
		}
		chainingSeq := make([]ot.GlyphIndex, 0, len(chainingPayload.BacktrackCoverages)+len(chainingPayload.InputCoverages)+len(chainingPayload.LookaheadCoverages))
		for _, cov := range chainingPayload.BacktrackCoverages {
			chainingSeq = append(chainingSeq, firstCoveredGlyph(t, otf, cov))
		}
		for _, cov := range chainingPayload.InputCoverages {
			chainingSeq = append(chainingSeq, firstCoveredGlyph(t, otf, cov))
		}
		for _, cov := range chainingPayload.LookaheadCoverages {
			chainingSeq = append(chainingSeq, firstCoveredGlyph(t, otf, cov))
		}
		chainingPos := len(chainingPayload.BacktrackCoverages)

		cases := []struct {
			name   string
			lookup int
			input  []ot.GlyphIndex
			pos    int
		}{
			{name: "single_fmt1", lookup: 0, input: []ot.GlyphIndex{singleGlyph}, pos: 0},
			{name: "pair_fmt1", lookup: 1, input: []ot.GlyphIndex{firstPair, secondPair}, pos: 0},
			{name: "chaining_fmt3", lookup: 4, input: chainingSeq, pos: chainingPos},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				first := runGPOS(ConcreteFirst, otf, tc.lookup, tc.input, tc.pos)
				only := runGPOS(ConcreteOnly, otf, tc.lookup, tc.input, tc.pos)
				assertGPOSRunEqual(t, first, only)
			})
		}
	})

	t.Run("mark_attachments", func(t *testing.T) {
		baseFont := loadTestFont(t, "gpos4_simple_1.otf")
		baseGraph := baseFont.Layout.GPos.LookupGraph()
		baseNode := baseGraph.Lookup(0).Subtable(0)
		basePayload := baseNode.GPosPayload().MarkToBaseFmt1
		if basePayload == nil {
			t.Fatalf("expected MarkToBaseFmt1 payload")
		}
		baseGlyph := firstCoveredGlyph(t, baseFont, basePayload.BaseCoverage)
		baseMark := firstCoveredGlyph(t, baseFont, baseNode.Coverage)
		firstBase := runGPOS(ConcreteFirst, baseFont, 0, []ot.GlyphIndex{baseGlyph, baseMark}, 1)
		onlyBase := runGPOS(ConcreteOnly, baseFont, 0, []ot.GlyphIndex{baseGlyph, baseMark}, 1)
		assertGPOSRunEqual(t, firstBase, onlyBase)

		ligFont := loadTestFont(t, "gpos5_font1.otf")
		ligGraph := ligFont.Layout.GPos.LookupGraph()
		ligNode := ligGraph.Lookup(0).Subtable(0)
		ligPayload := ligNode.GPosPayload().MarkToLigatureFmt1
		if ligPayload == nil {
			t.Fatalf("expected MarkToLigatureFmt1 payload")
		}
		ligGlyph := firstCoveredGlyph(t, ligFont, ligPayload.LigatureCoverage)
		ligMark := firstCoveredGlyph(t, ligFont, ligNode.Coverage)
		firstLig := runGPOS(ConcreteFirst, ligFont, 0, []ot.GlyphIndex{ligGlyph, ligMark}, 1)
		onlyLig := runGPOS(ConcreteOnly, ligFont, 0, []ot.GlyphIndex{ligGlyph, ligMark}, 1)
		assertGPOSRunEqual(t, firstLig, onlyLig)
	})
}

func assertGPOSRunEqual(t *testing.T, a, b struct {
	applied bool
	glyphs  []ot.GlyphIndex
	pos     []PosItem
}) {
	t.Helper()
	if a.applied != b.applied {
		t.Fatalf("applied mismatch: concrete-first=%v concrete-only=%v", a.applied, b.applied)
	}
	if len(a.glyphs) != len(b.glyphs) {
		t.Fatalf("glyph length mismatch: concrete-first=%d concrete-only=%d", len(a.glyphs), len(b.glyphs))
	}
	for i := range a.glyphs {
		if a.glyphs[i] != b.glyphs[i] {
			t.Fatalf("glyph[%d] mismatch: concrete-first=%d concrete-only=%d", i, a.glyphs[i], b.glyphs[i])
		}
	}
	if len(a.pos) != len(b.pos) {
		t.Fatalf("pos length mismatch: concrete-first=%d concrete-only=%d", len(a.pos), len(b.pos))
	}
	for i := range a.pos {
		if a.pos[i] != b.pos[i] {
			t.Fatalf("pos[%d] mismatch: concrete-first=%+v concrete-only=%+v", i, a.pos[i], b.pos[i])
		}
	}
}
