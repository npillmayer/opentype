package otlayout

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestMissingConcretePayloadDoesNotApply(t *testing.T) {
	sub := ot.LookupNode{
		LookupType: ot.GSubLookupTypeSingle,
		Format:     1,
		Coverage: ot.Coverage{
			GlyphRange: testGlyphRange{glyph: 10},
		},
	}

	ctx := applyCtx{
		clookup: &ot.LookupTable{},
		buf:     &BufferState{Glyphs: GlyphBuffer{10}},
		pos:     0,
	}
	_, ok, _, _, _ := dispatchGSubLookup(&ctx, &sub)
	if ok {
		t.Fatalf("expected missing concrete payload to skip lookup")
	}
}

func TestConcreteGSUBDeterministic(t *testing.T) {
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
			firstOut, firstApplied := applyGSUBLookup(t, otf, tc.lookup, tc.input, tc.pos, tc.alt)
			secondOut, secondApplied := applyGSUBLookup(t, otf, tc.lookup, tc.input, tc.pos, tc.alt)
			if secondApplied != firstApplied {
				t.Fatalf("applied mismatch: run-2=%v run-1=%v", secondApplied, firstApplied)
			}
			if len(secondOut) != len(firstOut) {
				t.Fatalf("glyph length mismatch: run-2=%d run-1=%d", len(secondOut), len(firstOut))
			}
			for i := range secondOut {
				if secondOut[i] != firstOut[i] {
					t.Fatalf("glyph[%d] mismatch: run-2=%d run-1=%d", i, secondOut[i], firstOut[i])
				}
			}
		})
	}
}

func TestConcreteGPOSDeterministic(t *testing.T) {
	type gposRun struct {
		applied bool
		glyphs  []ot.GlyphIndex
		pos     []PosItem
	}
	runGPOS := func(otf *ot.Font, lookup int, input []ot.GlyphIndex, pos int) gposRun {
		st, applied := applyGPOSLookup(t, otf, lookup, input, pos)
		out := gposRun{
			applied: applied,
			glyphs:  append([]ot.GlyphIndex(nil), st.Glyphs...),
			pos:     append([]PosItem(nil), st.Pos...),
		}
		return out
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
				run1 := runGPOS(otf, tc.lookup, tc.input, tc.pos)
				run2 := runGPOS(otf, tc.lookup, tc.input, tc.pos)
				assertGPOSRunEqual(t, run1, run2)
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
		run1Base := runGPOS(baseFont, 0, []ot.GlyphIndex{baseGlyph, baseMark}, 1)
		run2Base := runGPOS(baseFont, 0, []ot.GlyphIndex{baseGlyph, baseMark}, 1)
		assertGPOSRunEqual(t, run1Base, run2Base)

		ligFont := loadTestFont(t, "gpos5_font1.otf")
		ligGraph := ligFont.Layout.GPos.LookupGraph()
		ligNode := ligGraph.Lookup(0).Subtable(0)
		ligPayload := ligNode.GPosPayload().MarkToLigatureFmt1
		if ligPayload == nil {
			t.Fatalf("expected MarkToLigatureFmt1 payload")
		}
		ligGlyph := firstCoveredGlyph(t, ligFont, ligPayload.LigatureCoverage)
		ligMark := firstCoveredGlyph(t, ligFont, ligNode.Coverage)
		run1Lig := runGPOS(ligFont, 0, []ot.GlyphIndex{ligGlyph, ligMark}, 1)
		run2Lig := runGPOS(ligFont, 0, []ot.GlyphIndex{ligGlyph, ligMark}, 1)
		assertGPOSRunEqual(t, run1Lig, run2Lig)
	})
}

func assertGPOSRunEqual(t *testing.T, a, b struct {
	applied bool
	glyphs  []ot.GlyphIndex
	pos     []PosItem
}) {
	t.Helper()
	if a.applied != b.applied {
		t.Fatalf("applied mismatch: run-1=%v run-2=%v", a.applied, b.applied)
	}
	if len(a.glyphs) != len(b.glyphs) {
		t.Fatalf("glyph length mismatch: run-1=%d run-2=%d", len(a.glyphs), len(b.glyphs))
	}
	for i := range a.glyphs {
		if a.glyphs[i] != b.glyphs[i] {
			t.Fatalf("glyph[%d] mismatch: run-1=%d run-2=%d", i, a.glyphs[i], b.glyphs[i])
		}
	}
	if len(a.pos) != len(b.pos) {
		t.Fatalf("pos length mismatch: run-1=%d run-2=%d", len(a.pos), len(b.pos))
	}
	for i := range a.pos {
		if a.pos[i] != b.pos[i] {
			t.Fatalf("pos[%d] mismatch: run-1=%+v run-2=%+v", i, a.pos[i], b.pos[i])
		}
	}
}
