package otlayout

import "github.com/npillmayer/opentype/ot"

// GPOS Lookup Type 1, Format 1: Single Adjustment (single value for all covered glyphs).
func gposLookupType1Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	// inx should be 1, i.e. next glyph?
	if inx != 1 {
		tracer().Errorf("GPOS 1|1 unexpected coverage index")
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 1|1 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.(struct {
		Format ot.ValueFormat
		Record ot.ValueRecord
	})
	if !ok {
		tracer().Errorf("GPOS 1|1 support type mismatch")
		return pos, false, buf, nil
	}
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos < 0 || mpos >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	applyValueRecord(&ctx.buf.Pos[mpos], sup.Record, sup.Format)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 1, Format 2: Single Adjustment (one value per covered glyph).
func gposLookupType1Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 1|2 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.(struct {
		Format  ot.ValueFormat
		Records []ot.ValueRecord
	})
	if !ok {
		tracer().Errorf("GPOS 1|2 support type mismatch")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(sup.Records) {
		return pos, false, buf, nil
	}
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos < 0 || mpos >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	applyValueRecord(&ctx.buf.Pos[mpos], sup.Records[inx], sup.Format)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 2, Format 1: Pair Adjustment (glyph pair).
func gposLookupType2Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if ctx.lookupList == nil {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 2|1 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.([2]ot.ValueFormat)
	if !ok {
		tracer().Errorf("GPOS 2|1 support type mismatch")
		return pos, false, buf, nil
	}
	if lksub.Index.Size() == 0 {
		return pos, false, buf, nil
	}
	next, ok := nextMatchable(ctx, buf, mpos+1)
	if !ok {
		return pos, false, buf, nil
	}
	pairSetLoc, err := lksub.Index.Get(inx, false)
	if err != nil || pairSetLoc.Size() < 2 {
		return pos, false, buf, nil
	}
	pairCount := int(pairSetLoc.U16(0))
	recSize1 := valueRecordSize(sup[0])
	recSize2 := valueRecordSize(sup[1])
	recSize := 2 + recSize1 + recSize2
	offset := 2
	for i := 0; i < pairCount; i++ {
		if offset+recSize > pairSetLoc.Size() {
			break
		}
		second := pairSetLoc.U16(offset)
		if ot.GlyphIndex(second) == buf.At(next) {
			v1, _ := parseValueRecord(pairSetLoc, offset+2, sup[0])
			v2, _ := parseValueRecord(pairSetLoc, offset+2+recSize1, sup[1])
			ctx.buf.EnsurePos()
			if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) || next >= len(ctx.buf.Pos) {
				return pos, false, buf, nil
			}
			applyValueRecordPair(&ctx.buf.Pos[mpos], &ctx.buf.Pos[next], v1, sup[0], v2, sup[1])
			return mpos + 1, true, buf, nil
		}
		offset += recSize
	}
	return pos, false, buf, nil
}

// GPOS Lookup Type 2, Format 2: Pair Adjustment (class-based).
func gposLookupType2Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, _, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 2|2 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.(struct {
		ValueFormat1 ot.ValueFormat
		ValueFormat2 ot.ValueFormat
		ClassDef1    ot.ClassDefinitions
		ClassDef2    ot.ClassDefinitions
		Class1Count  uint16
		Class2Count  uint16
		ClassRecords [][]struct {
			Value1 ot.ValueRecord
			Value2 ot.ValueRecord
		}
	})
	if !ok {
		tracer().Errorf("GPOS 2|2 support type mismatch")
		return pos, false, buf, nil
	}
	next, ok := nextMatchable(ctx, buf, mpos+1)
	if !ok {
		return pos, false, buf, nil
	}
	c1 := sup.ClassDef1.Lookup(buf.At(mpos))
	c2 := sup.ClassDef2.Lookup(buf.At(next))
	if c1 < 0 || c2 < 0 || c1 >= int(sup.Class1Count) || c2 >= int(sup.Class2Count) {
		return pos, false, buf, nil
	}
	if c1 >= len(sup.ClassRecords) || c2 >= len(sup.ClassRecords[c1]) {
		return pos, false, buf, nil
	}
	rec := sup.ClassRecords[c1][c2]
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) || next >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	applyValueRecordPair(&ctx.buf.Pos[mpos], &ctx.buf.Pos[next], rec.Value1, sup.ValueFormat1, rec.Value2, sup.ValueFormat2)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 4, Format 1: MarkToBase Attachment.
func gposLookupType4Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, markInx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 4|1 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.(struct {
		BaseCoverage   ot.Coverage
		MarkClassCount uint16
		MarkArray      ot.MarkArray
		BaseArray      []ot.BaseRecord
	})
	if !ok {
		tracer().Errorf("GPOS 4|1 support type mismatch")
		return pos, false, buf, nil
	}
	if markInx < 0 || markInx >= len(sup.MarkArray.MarkRecords) {
		return pos, false, buf, nil
	}
	// find base glyph before mark
	basePos := -1
	baseInx := -1
	for i := mpos - 1; i >= 0; {
		prev, ok := prevMatchable(ctx, buf, i)
		if !ok {
			break
		}
		if inx, ok := sup.BaseCoverage.Match(buf.At(prev)); ok {
			basePos = prev
			baseInx = inx
			break
		}
		i = prev - 1
	}
	if basePos < 0 || baseInx < 0 {
		return pos, false, buf, nil
	}
	if baseInx >= len(sup.BaseArray) {
		return pos, false, buf, nil
	}
	markRec := sup.MarkArray.MarkRecords[markInx]
	class := int(markRec.Class)
	if class < 0 || class >= int(sup.MarkClassCount) {
		return pos, false, buf, nil
	}
	baseRec := sup.BaseArray[baseInx]
	if class >= len(baseRec.BaseAnchors) {
		return pos, false, buf, nil
	}
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	ref := AnchorRef{
		MarkAnchor: markRec.MarkAnchor,
		BaseAnchor: baseRec.BaseAnchors[class],
	}
	setMarkAttachment(&ctx.buf.Pos[mpos], basePos, AttachMarkToBase, markRec.Class, ref)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 5, Format 1: MarkToLigature Attachment.
func gposLookupType5Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, markInx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 5|1 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.(struct {
		LigatureCoverage ot.Coverage
		MarkClassCount   uint16
		MarkArray        ot.MarkArray
		LigatureArray    []ot.LigatureAttach
	})
	if !ok {
		tracer().Errorf("GPOS 5|1 support type mismatch")
		return pos, false, buf, nil
	}
	if markInx < 0 || markInx >= len(sup.MarkArray.MarkRecords) {
		return pos, false, buf, nil
	}
	// find ligature glyph before mark
	ligPos := -1
	ligInx := -1
	for i := mpos - 1; i >= 0; {
		prev, ok := prevMatchable(ctx, buf, i)
		if !ok {
			break
		}
		if inx, ok := sup.LigatureCoverage.Match(buf.At(prev)); ok {
			ligPos = prev
			ligInx = inx
			break
		}
		i = prev - 1
	}
	if ligPos < 0 || ligInx < 0 {
		return pos, false, buf, nil
	}
	if ligInx >= len(sup.LigatureArray) {
		return pos, false, buf, nil
	}
	markRec := sup.MarkArray.MarkRecords[markInx]
	class := int(markRec.Class)
	if class < 0 || class >= int(sup.MarkClassCount) {
		return pos, false, buf, nil
	}
	lig := sup.LigatureArray[ligInx]
	if len(lig.ComponentAnchors) == 0 {
		return pos, false, buf, nil
	}
	// Select last component by default (TODO: refine with caret/anchor logic)
	compIndex := len(lig.ComponentAnchors) - 1
	if compIndex < 0 || compIndex >= len(lig.ComponentAnchors) {
		return pos, false, buf, nil
	}
	if class >= len(lig.ComponentAnchors[compIndex]) {
		return pos, false, buf, nil
	}
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	ref := AnchorRef{
		MarkAnchor:   markRec.MarkAnchor,
		BaseAnchor:   lig.ComponentAnchors[compIndex][class],
		LigatureComp: uint16(compIndex),
	}
	setMarkAttachment(&ctx.buf.Pos[mpos], ligPos, AttachMarkToLigature, markRec.Class, ref)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 6, Format 1: MarkToMark Attachment.
func gposLookupType6Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, markInx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 6|1 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.(struct {
		Mark2Coverage  ot.Coverage
		MarkClassCount uint16
		Mark1Array     ot.MarkArray
		Mark2Array     []ot.BaseRecord
	})
	if !ok {
		tracer().Errorf("GPOS 6|1 support type mismatch")
		return pos, false, buf, nil
	}
	if markInx < 0 || markInx >= len(sup.Mark1Array.MarkRecords) {
		return pos, false, buf, nil
	}
	// find mark2 glyph before mark1
	mark2Pos := -1
	mark2Inx := -1
	for i := mpos - 1; i >= 0; {
		prev, ok := prevMatchable(ctx, buf, i)
		if !ok {
			break
		}
		if inx, ok := sup.Mark2Coverage.Match(buf.At(prev)); ok {
			mark2Pos = prev
			mark2Inx = inx
			break
		}
		i = prev - 1
	}
	if mark2Pos < 0 || mark2Inx < 0 {
		return pos, false, buf, nil
	}
	if mark2Inx >= len(sup.Mark2Array) {
		return pos, false, buf, nil
	}
	markRec := sup.Mark1Array.MarkRecords[markInx]
	class := int(markRec.Class)
	if class < 0 || class >= int(sup.MarkClassCount) {
		return pos, false, buf, nil
	}
	mark2Rec := sup.Mark2Array[mark2Inx]
	if class >= len(mark2Rec.BaseAnchors) {
		return pos, false, buf, nil
	}
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	ref := AnchorRef{
		MarkAnchor: markRec.MarkAnchor,
		BaseAnchor: mark2Rec.BaseAnchors[class],
	}
	setMarkAttachment(&ctx.buf.Pos[mpos], mark2Pos, AttachMarkToMark, markRec.Class, ref)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 7, Format 1: Contextual Positioning (glyph-based).
func gposLookupType7Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Index.Size() == 0 {
		return pos, false, buf, nil
	}
	ruleSetLoc, err := lksub.Index.Get(inx, false)
	if err != nil || ruleSetLoc.Size() < 2 {
		return pos, false, buf, nil
	}
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "SequenceRuleSet")
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 2 {
			continue
		}
		glyphCount := int(ruleLoc.U16(0))
		seqLookupCount := int(ruleLoc.U16(2))
		if glyphCount <= 0 {
			continue
		}
		inputCount := glyphCount - 1
		inputBytes := inputCount * 2
		recBytes := seqLookupCount * 4
		minSize := 4 + inputBytes + recBytes
		if ruleLoc.Size() < minSize {
			continue
		}
		inputGlyphs := make([]ot.GlyphIndex, inputCount)
		for j := range inputCount {
			inputGlyphs[j] = ot.GlyphIndex(ruleLoc.U16(4 + j*2))
		}
		restPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, inputGlyphs)
		if !ok {
			continue
		}
		matchPositions := make([]int, 0, glyphCount)
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, restPos...)
		records := make([]ot.SequenceLookupRecord, seqLookupCount)
		recStart := 4 + inputBytes
		for r := range seqLookupCount {
			off := recStart + r*4
			records[r] = ot.SequenceLookupRecord{
				SequenceIndex:   ruleLoc.U16(off),
				LookupListIndex: ruleLoc.U16(off + 2),
			}
		}
		if ctx.lookupList == nil {
			return pos, false, buf, nil
		}
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		if applied {
			return mpos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

// GPOS Lookup Type 7, Format 2: Contextual Positioning (class-based).
func gposLookupType7Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, _, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 7|2 missing support data")
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok || len(seqctx.ClassDefs) == 0 {
		tracer().Errorf("GPOS 7|2 support type mismatch")
		return pos, false, buf, nil
	}
	firstClass := seqctx.ClassDefs[0].Lookup(buf.At(mpos))
	ruleSetLoc, err := lksub.Index.Get(int(firstClass), false)
	if err != nil || ruleSetLoc.Size() < 2 {
		return pos, false, buf, nil
	}
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "SequenceRuleSet")
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 2 {
			continue
		}
		glyphCount := int(ruleLoc.U16(0))
		seqLookupCount := int(ruleLoc.U16(2))
		if glyphCount <= 0 {
			continue
		}
		inputCount := glyphCount - 1
		inputBytes := inputCount * 2
		recBytes := seqLookupCount * 4
		minSize := 4 + inputBytes + recBytes
		if ruleLoc.Size() < minSize {
			continue
		}
		classes := make([]uint16, inputCount)
		for j := range inputCount {
			classes[j] = ruleLoc.U16(4 + j*2)
		}
		restPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, seqctx.ClassDefs[0], classes)
		if !ok {
			continue
		}
		matchPositions := make([]int, 0, glyphCount)
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, restPos...)
		records := make([]ot.SequenceLookupRecord, seqLookupCount)
		recStart := 4 + inputBytes
		for r := range seqLookupCount {
			off := recStart + r*4
			records[r] = ot.SequenceLookupRecord{
				SequenceIndex:   ruleLoc.U16(off),
				LookupListIndex: ruleLoc.U16(off + 2),
			}
		}
		if ctx.lookupList == nil {
			return pos, false, buf, nil
		}
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		if applied {
			return mpos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

// GPOS Lookup Type 7, Format 3: Contextual Positioning (coverage-based).
func gposLookupType7Fmt3(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	if lksub.Support == nil {
		tracer().Errorf("GPOS 7|3 missing support data")
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok || len(seqctx.InputCoverage) == 0 {
		tracer().Errorf("GPOS 7|3 support type mismatch")
		return pos, false, buf, nil
	}
	inputPos, ok := matchCoverageSequenceForward(ctx, buf, pos, seqctx.InputCoverage)
	if !ok {
		return pos, false, buf, nil
	}
	if len(lksub.LookupRecords) == 0 {
		return pos, false, buf, nil
	}
	if ctx.lookupList == nil {
		return pos, false, buf, nil
	}
	out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, inputPos, lksub.LookupRecords, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
	ctx.buf.Pos = outPosBuf
	if applied {
		return pos, true, out, nil
	}
	return pos, false, buf, nil
}

// GPOS Lookup Type 8, Format 1: Chained Contextual Positioning (glyph-based).
func gposLookupType8Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	rules, err := parseChainedSequenceRules(lksub, inx)
	if err != nil || len(rules) == 0 {
		return pos, false, buf, nil
	}
	for _, rule := range rules {
		inputPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, rule.Input)
		if !ok {
			continue
		}
		matchPositions := make([]int, 0, 1+len(inputPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, inputPos...)
		if len(rule.Backtrack) > 0 {
			if _, ok := matchGlyphSequenceBackward(ctx, buf, mpos, rule.Backtrack); !ok {
				continue
			}
		}
		if len(rule.Lookahead) > 0 {
			last := matchPositions[len(matchPositions)-1]
			if _, ok := matchGlyphSequenceForward(ctx, buf, last+1, rule.Lookahead); !ok {
				continue
			}
		}
		if ctx.lookupList == nil {
			return pos, false, buf, nil
		}
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		if applied {
			return mpos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

// GPOS Lookup Type 8, Format 2: Chained Contextual Positioning (class-based).
func gposLookupType8Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok || len(seqctx.ClassDefs) < 3 {
		return pos, false, buf, nil
	}
	rules, err := parseChainedClassSequenceRules(lksub, inx)
	if err != nil || len(rules) == 0 {
		return pos, false, buf, nil
	}
	for _, rule := range rules {
		inputPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, seqctx.ClassDefs[1], rule.Input)
		if !ok {
			continue
		}
		matchPositions := make([]int, 0, 1+len(inputPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, inputPos...)
		if len(rule.Backtrack) > 0 {
			if _, ok := matchClassSequenceBackward(ctx, buf, mpos, seqctx.ClassDefs[0], rule.Backtrack); !ok {
				continue
			}
		}
		if len(rule.Lookahead) > 0 {
			last := matchPositions[len(matchPositions)-1]
			if _, ok := matchClassSequenceForward(ctx, buf, last+1, seqctx.ClassDefs[2], rule.Lookahead); !ok {
				continue
			}
		}
		if ctx.lookupList == nil {
			return pos, false, buf, nil
		}
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		if applied {
			return mpos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

// GPOS Lookup Type 8, Format 3: Chained Contextual Positioning (coverage-based).
func gposLookupType8Fmt3(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok || len(seqctx.InputCoverage) == 0 {
		return pos, false, buf, nil
	}
	var backtrackFn matchSeqFn
	if len(seqctx.BacktrackCoverage) > 0 {
		backtrackFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceBackward(ctx, buf, pos, seqctx.BacktrackCoverage)
		}
	}
	inputFn := func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
		return matchCoverageSequenceForward(ctx, buf, pos, seqctx.InputCoverage)
	}
	var lookaheadFn matchSeqFn
	if len(seqctx.LookaheadCoverage) > 0 {
		lookaheadFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceForward(ctx, buf, pos+1, seqctx.LookaheadCoverage)
		}
	}
	inputPos, ok := matchChainedForward(ctx, buf, pos, backtrackFn, inputFn, lookaheadFn)
	if !ok {
		return pos, false, buf, nil
	}
	if len(lksub.LookupRecords) == 0 {
		return pos, false, buf, nil
	}
	if ctx.lookupList == nil {
		return pos, false, buf, nil
	}
	out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, inputPos, lksub.LookupRecords, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
	ctx.buf.Pos = outPosBuf
	if applied {
		return pos, true, out, nil
	}
	return pos, false, buf, nil
}

// GPOS Lookup Type 3, Format 1: Cursive Attachment.
func gposLookupType3Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("GPOS 3|1 missing support data")
		return pos, false, buf, nil
	}
	sup, ok := lksub.Support.(struct {
		EntryExitCount uint16
	})
	if !ok || sup.EntryExitCount == 0 {
		tracer().Errorf("GPOS 3|1 support type mismatch")
		return pos, false, buf, nil
	}
	if lksub.Index.Size() == 0 {
		return pos, false, buf, nil
	}
	entryExitLoc, err := lksub.Index.Get(inx, false)
	if err != nil || entryExitLoc.Size() < 4 {
		return pos, false, buf, nil
	}
	entryAnchor := entryExitLoc.U16(0)
	exitAnchor := entryExitLoc.U16(2)
	// find adjacent glyph for cursive attachment (next glyph by default)
	next, ok := nextMatchable(ctx, buf, mpos+1)
	if !ok {
		return pos, false, buf, nil
	}
	// determine attachment direction: if current has exit, attach next entry to current
	if exitAnchor != 0 {
		ctx.buf.EnsurePos()
		if ctx.buf.Pos == nil || next >= len(ctx.buf.Pos) {
			return pos, false, buf, nil
		}
		ref := AnchorRef{
			CursiveExit:  exitAnchor,
			CursiveEntry: entryAnchor,
		}
		setCursiveAttachment(&ctx.buf.Pos[next], mpos, ref)
		return mpos + 1, true, buf, nil
	}
	// else if current has entry, try to attach to previous glyph with exit
	prev, ok := prevMatchable(ctx, buf, mpos-1)
	if ok {
		prevInx, ok := lksub.Coverage.Match(buf.At(prev))
		if ok {
			prevLoc, err := lksub.Index.Get(prevInx, false)
			if err == nil && prevLoc.Size() >= 4 {
				prevExit := prevLoc.U16(2)
				if prevExit != 0 {
					ctx.buf.EnsurePos()
					if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) {
						return pos, false, buf, nil
					}
					ref := AnchorRef{
						CursiveExit:  prevExit,
						CursiveEntry: entryAnchor,
					}
					setCursiveAttachment(&ctx.buf.Pos[mpos], prev, ref)
					return mpos + 1, true, buf, nil
				}
			}
		}
	}
	return pos, false, buf, nil
}
