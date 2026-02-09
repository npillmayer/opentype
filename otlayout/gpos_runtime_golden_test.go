package otlayout

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func applyGPOSLookup(t *testing.T, otf *ot.Font, lookupIndex int, input []ot.GlyphIndex, pos int) (*BufferState, bool) {
	t.Helper()
	if otf.Layout.GPos == nil {
		t.Fatalf("font has no GPOS table")
	}
	lookup := otf.Layout.GPos.LookupList.Navigate(lookupIndex)
	var clookup *ot.LookupTable
	if graph := otf.Layout.GPos.LookupGraph(); graph != nil {
		clookup = graph.Lookup(lookupIndex)
	}
	feat := testFeature{tag: ot.T("test"), typ: GPosFeatureType}
	buf := append(GlyphBuffer(nil), input...)
	st := NewBufferState(buf, NewPosBuffer(len(buf)))
	st.Index = pos
	_, ok, _ := applyLookupConcrete(&lookup, clookup, otf.Layout.GPos.LookupGraph(), feat, st, 0, otf.Layout.GDef, otf.Layout.GPos.LookupList)
	return st, ok
}

func maxGlyphCount(otf *ot.Font) int {
	if otf == nil {
		return ot.MaxGlyphCount
	}
	if maxp := otf.Table(ot.T("maxp")); maxp != nil {
		if t := maxp.Self().AsMaxP(); t != nil && t.NumGlyphs > 0 {
			return t.NumGlyphs
		}
	}
	return ot.MaxGlyphCount
}

func firstCoveredGlyph(t *testing.T, otf *ot.Font, cov ot.Coverage) ot.GlyphIndex {
	t.Helper()
	limit := maxGlyphCount(otf)
	for gid := 0; gid < limit; gid++ {
		if _, ok := cov.Match(ot.GlyphIndex(gid)); ok {
			return ot.GlyphIndex(gid)
		}
	}
	t.Fatalf("could not find covered glyph in coverage")
	return 0
}

func firstUncoveredGlyph(t *testing.T, otf *ot.Font, cov ot.Coverage) ot.GlyphIndex {
	t.Helper()
	limit := maxGlyphCount(otf)
	for gid := 0; gid < limit; gid++ {
		if _, ok := cov.Match(ot.GlyphIndex(gid)); !ok {
			return ot.GlyphIndex(gid)
		}
	}
	t.Fatalf("could not find uncovered glyph for coverage")
	return 0
}

func valueDelta(vr ot.ValueRecord, format ot.ValueFormat) PosItem {
	var p PosItem
	if format&ot.ValueFormatXPlacement != 0 {
		p.XOffset = int32(vr.XPlacement)
	}
	if format&ot.ValueFormatYPlacement != 0 {
		p.YOffset = int32(vr.YPlacement)
	}
	if format&ot.ValueFormatXAdvance != 0 {
		p.XAdvance = int32(vr.XAdvance)
	}
	if format&ot.ValueFormatYAdvance != 0 {
		p.YAdvance = int32(vr.YAdvance)
	}
	return p
}

func assertPosDelta(t *testing.T, got PosItem, want PosItem) {
	t.Helper()
	if got.XAdvance != want.XAdvance || got.YAdvance != want.YAdvance || got.XOffset != want.XOffset || got.YOffset != want.YOffset {
		t.Fatalf("unexpected pos delta: got {xa=%d ya=%d xo=%d yo=%d}, want {xa=%d ya=%d xo=%d yo=%d}",
			got.XAdvance, got.YAdvance, got.XOffset, got.YOffset,
			want.XAdvance, want.YAdvance, want.XOffset, want.YOffset)
	}
}

func TestGPOSSingleAndPairRuntimeGolden(t *testing.T) {
	otf := loadTestFont(t, "gpos_chaining3_boundary_f2.otf")
	graph := otf.Layout.GPos.LookupGraph()
	if graph == nil {
		t.Fatalf("font has no concrete GPOS lookup graph")
	}

	t.Run("singlepos_fmt1_covered", func(t *testing.T) {
		node := graph.Lookup(0).Subtable(0)
		p := node.GPosPayload().SingleFmt1
		if p == nil {
			t.Fatalf("expected SingleFmt1 payload")
		}
		gid := firstCoveredGlyph(t, otf, node.Coverage)
		st, applied := applyGPOSLookup(t, otf, 0, []ot.GlyphIndex{gid}, 0)
		if !applied {
			t.Fatalf("expected lookup to apply")
		}
		assertPosDelta(t, st.Pos[0], valueDelta(p.Value, p.ValueFormat))
		if got := st.Pos[0].AttachTo; got != -1 {
			t.Fatalf("expected AttachTo=-1, got %d", got)
		}
	})

	t.Run("singlepos_fmt1_not_covered", func(t *testing.T) {
		node := graph.Lookup(0).Subtable(0)
		gid := firstUncoveredGlyph(t, otf, node.Coverage)
		st, applied := applyGPOSLookup(t, otf, 0, []ot.GlyphIndex{gid}, 0)
		if applied {
			t.Fatalf("expected lookup to not apply")
		}
		assertPosDelta(t, st.Pos[0], PosItem{})
	})

	t.Run("pairpos_fmt1_match", func(t *testing.T) {
		node := graph.Lookup(1).Subtable(0)
		p := node.GPosPayload().PairFmt1
		if p == nil {
			t.Fatalf("expected PairFmt1 payload")
		}
		first := firstCoveredGlyph(t, otf, node.Coverage)
		row, ok := node.Coverage.Match(first)
		if !ok || row < 0 || row >= len(p.PairSets) || len(p.PairSets[row]) == 0 {
			t.Fatalf("invalid pair-set row for first glyph")
		}
		rec := p.PairSets[row][0]
		second := ot.GlyphIndex(rec.SecondGlyph)
		st, applied := applyGPOSLookup(t, otf, 1, []ot.GlyphIndex{first, second}, 0)
		if !applied {
			t.Fatalf("expected lookup to apply")
		}
		assertPosDelta(t, st.Pos[0], valueDelta(rec.Value1, p.ValueFormat1))
		assertPosDelta(t, st.Pos[1], valueDelta(rec.Value2, p.ValueFormat2))
	})

	t.Run("pairpos_fmt1_mismatch", func(t *testing.T) {
		node := graph.Lookup(1).Subtable(0)
		p := node.GPosPayload().PairFmt1
		if p == nil {
			t.Fatalf("expected PairFmt1 payload")
		}
		first := firstCoveredGlyph(t, otf, node.Coverage)
		row, ok := node.Coverage.Match(first)
		if !ok || row < 0 || row >= len(p.PairSets) || len(p.PairSets[row]) == 0 {
			t.Fatalf("invalid pair-set row for first glyph")
		}
		secondSet := make(map[ot.GlyphIndex]struct{}, len(p.PairSets[row]))
		for _, rec := range p.PairSets[row] {
			secondSet[ot.GlyphIndex(rec.SecondGlyph)] = struct{}{}
		}
		limit := maxGlyphCount(otf)
		var second ot.GlyphIndex
		found := false
		for gid := 0; gid < limit; gid++ {
			candidate := ot.GlyphIndex(gid)
			if _, ok := secondSet[candidate]; !ok {
				second = candidate
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("could not find mismatch second glyph")
		}
		st, applied := applyGPOSLookup(t, otf, 1, []ot.GlyphIndex{first, second}, 0)
		if applied {
			t.Fatalf("expected lookup to not apply")
		}
		assertPosDelta(t, st.Pos[0], PosItem{})
	})

	t.Run("chainingpos_fmt3_forwards_to_single", func(t *testing.T) {
		node := graph.Lookup(4).Subtable(0)
		p := node.GPosPayload().ChainingContextFmt3
		if p == nil {
			t.Fatalf("expected ChainingContextFmt3 payload")
		}
		if len(p.BacktrackCoverages) == 0 || len(p.InputCoverages) == 0 || len(p.LookaheadCoverages) == 0 || len(p.Records) == 0 {
			t.Fatalf("expected non-empty chaining context payload")
		}
		seq := make([]ot.GlyphIndex, 0, len(p.BacktrackCoverages)+len(p.InputCoverages)+len(p.LookaheadCoverages))
		for _, cov := range p.BacktrackCoverages {
			seq = append(seq, firstCoveredGlyph(t, otf, cov))
		}
		for _, cov := range p.InputCoverages {
			seq = append(seq, firstCoveredGlyph(t, otf, cov))
		}
		for _, cov := range p.LookaheadCoverages {
			seq = append(seq, firstCoveredGlyph(t, otf, cov))
		}
		inputPos := len(p.BacktrackCoverages)
		targetLookup := graph.Lookup(int(p.Records[0].LookupListIndex))
		targetNode := targetLookup.Subtable(0)
		target := targetNode.GPosPayload().SingleFmt1
		if target == nil {
			t.Fatalf("expected chained lookup to resolve to SingleFmt1 payload")
		}
		st, applied := applyGPOSLookup(t, otf, 4, seq, inputPos)
		if !applied {
			t.Fatalf("expected lookup to apply")
		}
		assertPosDelta(t, st.Pos[inputPos], valueDelta(target.Value, target.ValueFormat))
	})
}

func TestGPOSMarkAttachmentRuntimeGolden(t *testing.T) {
	t.Run("mark_to_base_fmt1", func(t *testing.T) {
		otf := loadTestFont(t, "gpos4_simple_1.otf")
		graph := otf.Layout.GPos.LookupGraph()
		if graph == nil {
			t.Fatalf("font has no concrete GPOS lookup graph")
		}
		node := graph.Lookup(0).Subtable(0)
		p := node.GPosPayload().MarkToBaseFmt1
		if p == nil {
			t.Fatalf("expected MarkToBaseFmt1 payload")
		}
		base := firstCoveredGlyph(t, otf, p.BaseCoverage)
		mark := firstCoveredGlyph(t, otf, node.Coverage)
		markInx, ok := node.Coverage.Match(mark)
		if !ok || markInx < 0 || markInx >= len(p.MarkRecords) {
			t.Fatalf("invalid mark index")
		}
		baseInx, ok := p.BaseCoverage.Match(base)
		if !ok || baseInx < 0 || baseInx >= len(p.BaseRecords) {
			t.Fatalf("invalid base index")
		}
		class := int(p.MarkRecords[markInx].Class)
		wantMarkOff, wantBaseOff, ok := p.AnchorOffsets(markInx, baseInx, class)
		if !ok {
			t.Fatalf("invalid mark-to-base anchor offsets")
		}
		st, applied := applyGPOSLookup(t, otf, 0, []ot.GlyphIndex{base, mark}, 1)
		if !applied {
			t.Fatalf("expected lookup to apply")
		}
		markPos := st.Pos[1]
		if markPos.AttachKind != AttachMarkToBase {
			t.Fatalf("expected AttachKind=%d, got %d", AttachMarkToBase, markPos.AttachKind)
		}
		if markPos.AttachTo != 0 {
			t.Fatalf("expected AttachTo=0, got %d", markPos.AttachTo)
		}
		if want := p.MarkRecords[markInx].Class; markPos.AttachClass != want {
			t.Fatalf("expected AttachClass=%d, got %d", want, markPos.AttachClass)
		}
		if markPos.AnchorRef.MarkAnchor != wantMarkOff || markPos.AnchorRef.BaseAnchor != wantBaseOff {
			t.Fatalf("unexpected AnchorRef offsets: got mark=%d base=%d, want mark=%d base=%d",
				markPos.AnchorRef.MarkAnchor, markPos.AnchorRef.BaseAnchor, wantMarkOff, wantBaseOff)
		}
	})

	t.Run("mark_to_base_requires_prior_base", func(t *testing.T) {
		otf := loadTestFont(t, "gpos4_simple_1.otf")
		graph := otf.Layout.GPos.LookupGraph()
		node := graph.Lookup(0).Subtable(0)
		mark := firstCoveredGlyph(t, otf, node.Coverage)
		_, applied := applyGPOSLookup(t, otf, 0, []ot.GlyphIndex{mark}, 0)
		if applied {
			t.Fatalf("expected lookup to not apply")
		}
	})

	t.Run("mark_to_ligature_fmt1", func(t *testing.T) {
		otf := loadTestFont(t, "gpos5_font1.otf")
		graph := otf.Layout.GPos.LookupGraph()
		if graph == nil {
			t.Fatalf("font has no concrete GPOS lookup graph")
		}
		node := graph.Lookup(0).Subtable(0)
		p := node.GPosPayload().MarkToLigatureFmt1
		if p == nil {
			t.Fatalf("expected MarkToLigatureFmt1 payload")
		}
		lig := firstCoveredGlyph(t, otf, p.LigatureCoverage)
		mark := firstCoveredGlyph(t, otf, node.Coverage)
		markInx, ok := node.Coverage.Match(mark)
		if !ok || markInx < 0 || markInx >= len(p.MarkRecords) {
			t.Fatalf("invalid mark index")
		}
		ligInx, ok := p.LigatureCoverage.Match(lig)
		if !ok || ligInx < 0 || ligInx >= len(p.LigatureRecords) || len(p.LigatureRecords[ligInx].ComponentAnchors) == 0 {
			t.Fatalf("invalid ligature index")
		}
		comp := len(p.LigatureRecords[ligInx].ComponentAnchors) - 1
		class := int(p.MarkRecords[markInx].Class)
		wantMarkOff, wantBaseOff, ok := p.AnchorOffsets(markInx, ligInx, comp, class)
		if !ok {
			t.Fatalf("invalid mark-to-ligature anchor offsets")
		}
		st, applied := applyGPOSLookup(t, otf, 0, []ot.GlyphIndex{lig, mark}, 1)
		if !applied {
			t.Fatalf("expected lookup to apply")
		}
		markPos := st.Pos[1]
		if markPos.AttachKind != AttachMarkToLigature {
			t.Fatalf("expected AttachKind=%d, got %d", AttachMarkToLigature, markPos.AttachKind)
		}
		if markPos.AttachTo != 0 {
			t.Fatalf("expected AttachTo=0, got %d", markPos.AttachTo)
		}
		if want := p.MarkRecords[markInx].Class; markPos.AttachClass != want {
			t.Fatalf("expected AttachClass=%d, got %d", want, markPos.AttachClass)
		}
		if markPos.AnchorRef.LigatureComp != uint16(comp) {
			t.Fatalf("expected LigatureComp=%d, got %d", comp, markPos.AnchorRef.LigatureComp)
		}
		if markPos.AnchorRef.MarkAnchor != wantMarkOff || markPos.AnchorRef.BaseAnchor != wantBaseOff {
			t.Fatalf("unexpected AnchorRef offsets: got mark=%d base=%d, want mark=%d base=%d",
				markPos.AnchorRef.MarkAnchor, markPos.AnchorRef.BaseAnchor, wantMarkOff, wantBaseOff)
		}
	})

	t.Run("mark_to_ligature_requires_prior_ligature", func(t *testing.T) {
		otf := loadTestFont(t, "gpos5_font1.otf")
		graph := otf.Layout.GPos.LookupGraph()
		node := graph.Lookup(0).Subtable(0)
		mark := firstCoveredGlyph(t, otf, node.Coverage)
		_, applied := applyGPOSLookup(t, otf, 0, []ot.GlyphIndex{mark}, 0)
		if applied {
			t.Fatalf("expected lookup to not apply")
		}
	})
}
