package otlayout

import "github.com/npillmayer/opentype/ot"

// GPOS Lookup Type 1, Format 1: Single Adjustment (single value for all covered glyphs).
func gposLookupType1Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, _, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GPosSingleFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.SingleFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 1|1 missing concrete payload")
		return pos, false, buf, nil
	}
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos < 0 || mpos >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	applyValueRecord(&ctx.buf.Pos[mpos], payload.Value, payload.ValueFormat)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 1, Format 2: Single Adjustment (one value per covered glyph).
func gposLookupType1Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GPosSingleFmt2Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.SingleFmt2
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 1|2 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.Values) {
		return pos, false, buf, nil
	}
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos < 0 || mpos >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	applyValueRecord(&ctx.buf.Pos[mpos], payload.Values[inx], payload.ValueFormat)
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
	next, ok := nextMatchable(ctx, buf, mpos+1)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GPosPairFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.PairFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 2|1 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.PairSets) {
		return pos, false, buf, nil
	}
	for _, rec := range payload.PairSets[inx] {
		if ot.GlyphIndex(rec.SecondGlyph) == buf.At(next) {
			ctx.buf.EnsurePos()
			if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) || next >= len(ctx.buf.Pos) {
				return pos, false, buf, nil
			}
			applyValueRecordPair(&ctx.buf.Pos[mpos], &ctx.buf.Pos[next], rec.Value1, payload.ValueFormat1, rec.Value2, payload.ValueFormat2)
			return mpos + 1, true, buf, nil
		}
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
	next, ok := nextMatchable(ctx, buf, mpos+1)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GPosPairFmt2Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.PairFmt2
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 2|2 missing concrete payload")
		return pos, false, buf, nil
	}
	c1 := payload.ClassDef1.Lookup(buf.At(mpos))
	c2 := payload.ClassDef2.Lookup(buf.At(next))
	if c1 < 0 || c2 < 0 || c1 >= int(payload.Class1Count) || c2 >= int(payload.Class2Count) {
		return pos, false, buf, nil
	}
	if c1 >= len(payload.ClassRecords) || c2 >= len(payload.ClassRecords[c1]) {
		return pos, false, buf, nil
	}
	rec := payload.ClassRecords[c1][c2]
	ctx.buf.EnsurePos()
	if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) || next >= len(ctx.buf.Pos) {
		return pos, false, buf, nil
	}
	applyValueRecordPair(&ctx.buf.Pos[mpos], &ctx.buf.Pos[next], rec.Value1, payload.ValueFormat1, rec.Value2, payload.ValueFormat2)
	return mpos + 1, true, buf, nil
}

// GPOS Lookup Type 4, Format 1: MarkToBase Attachment.
func gposLookupType4Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, markInx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil && p.MarkToBaseFmt1 != nil {
			if markInx < 0 || markInx >= len(p.MarkToBaseFmt1.MarkRecords) {
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
				if inx, ok := p.MarkToBaseFmt1.BaseCoverage.Match(buf.At(prev)); ok {
					basePos = prev
					baseInx = inx
					break
				}
				i = prev - 1
			}
			if basePos < 0 || baseInx < 0 || baseInx >= len(p.MarkToBaseFmt1.BaseRecords) {
				return pos, false, buf, nil
			}
			markRec := p.MarkToBaseFmt1.MarkRecords[markInx]
			class := int(markRec.Class)
			if class < 0 || class >= int(p.MarkToBaseFmt1.MarkClassCount) {
				return pos, false, buf, nil
			}
			baseRec := p.MarkToBaseFmt1.BaseRecords[baseInx]
			if class >= len(baseRec.Anchors) || markRec.Anchor == nil || baseRec.Anchors[class] == nil {
				return pos, false, buf, nil
			}
			ctx.buf.EnsurePos()
			if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) {
				return pos, false, buf, nil
			}

			// Keep unresolved anchor reference behavior aligned with legacy path when possible.
			var markAnchor uint16
			var baseAnchor uint16
			if lksup, ok := lksub.Support.(struct {
				BaseCoverage   ot.Coverage
				MarkClassCount uint16
				MarkArray      ot.MarkArray
				BaseArray      []ot.BaseRecord
			}); ok {
				if markInx >= 0 && markInx < len(lksup.MarkArray.MarkRecords) {
					markAnchor = lksup.MarkArray.MarkRecords[markInx].MarkAnchor
				}
				if baseInx >= 0 && baseInx < len(lksup.BaseArray) {
					if class >= 0 && class < len(lksup.BaseArray[baseInx].BaseAnchors) {
						baseAnchor = lksup.BaseArray[baseInx].BaseAnchors[class]
					}
				}
			}
			ref := AnchorRef{
				MarkAnchor: markAnchor,
				BaseAnchor: baseAnchor,
			}
			setMarkAttachment(&ctx.buf.Pos[mpos], basePos, AttachMarkToBase, markRec.Class, ref)
			return mpos + 1, true, buf, nil
		}
	}
	tracer().Errorf("GPOS 4|1 missing concrete payload")
	return pos, false, buf, nil
}

// GPOS Lookup Type 5, Format 1: MarkToLigature Attachment.
func gposLookupType5Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, markInx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil && p.MarkToLigatureFmt1 != nil {
			if markInx < 0 || markInx >= len(p.MarkToLigatureFmt1.MarkRecords) {
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
				if inx, ok := p.MarkToLigatureFmt1.LigatureCoverage.Match(buf.At(prev)); ok {
					ligPos = prev
					ligInx = inx
					break
				}
				i = prev - 1
			}
			if ligPos < 0 || ligInx < 0 || ligInx >= len(p.MarkToLigatureFmt1.LigatureRecords) {
				return pos, false, buf, nil
			}
			markRec := p.MarkToLigatureFmt1.MarkRecords[markInx]
			class := int(markRec.Class)
			if class < 0 || class >= int(p.MarkToLigatureFmt1.MarkClassCount) {
				return pos, false, buf, nil
			}
			lig := p.MarkToLigatureFmt1.LigatureRecords[ligInx]
			if len(lig.ComponentAnchors) == 0 {
				return pos, false, buf, nil
			}
			// Select last component by default (TODO: refine with caret/anchor logic)
			compIndex := len(lig.ComponentAnchors) - 1
			if compIndex < 0 || compIndex >= len(lig.ComponentAnchors) {
				return pos, false, buf, nil
			}
			if class >= len(lig.ComponentAnchors[compIndex]) || markRec.Anchor == nil || lig.ComponentAnchors[compIndex][class] == nil {
				return pos, false, buf, nil
			}
			ctx.buf.EnsurePos()
			if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) {
				return pos, false, buf, nil
			}

			// Keep unresolved anchor reference behavior aligned with legacy path when possible.
			var markAnchor uint16
			var baseAnchor uint16
			if lksup, ok := lksub.Support.(struct {
				LigatureCoverage ot.Coverage
				MarkClassCount   uint16
				MarkArray        ot.MarkArray
				LigatureArray    []ot.LigatureAttach
			}); ok {
				if markInx >= 0 && markInx < len(lksup.MarkArray.MarkRecords) {
					markAnchor = lksup.MarkArray.MarkRecords[markInx].MarkAnchor
				}
				if ligInx >= 0 && ligInx < len(lksup.LigatureArray) {
					la := lksup.LigatureArray[ligInx]
					if compIndex >= 0 && compIndex < len(la.ComponentAnchors) {
						if class >= 0 && class < len(la.ComponentAnchors[compIndex]) {
							baseAnchor = la.ComponentAnchors[compIndex][class]
						}
					}
				}
			}
			ref := AnchorRef{
				MarkAnchor:   markAnchor,
				BaseAnchor:   baseAnchor,
				LigatureComp: uint16(compIndex),
			}
			setMarkAttachment(&ctx.buf.Pos[mpos], ligPos, AttachMarkToLigature, markRec.Class, ref)
			return mpos + 1, true, buf, nil
		}
	}
	tracer().Errorf("GPOS 5|1 missing concrete payload")
	return pos, false, buf, nil
}

// GPOS Lookup Type 6, Format 1: MarkToMark Attachment.
func gposLookupType6Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, markInx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil && p.MarkToMarkFmt1 != nil {
			if markInx < 0 || markInx >= len(p.MarkToMarkFmt1.Mark1Records) {
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
				if inx, ok := p.MarkToMarkFmt1.Mark2Coverage.Match(buf.At(prev)); ok {
					mark2Pos = prev
					mark2Inx = inx
					break
				}
				i = prev - 1
			}
			if mark2Pos < 0 || mark2Inx < 0 || mark2Inx >= len(p.MarkToMarkFmt1.Mark2Records) {
				return pos, false, buf, nil
			}
			markRec := p.MarkToMarkFmt1.Mark1Records[markInx]
			class := int(markRec.Class)
			if class < 0 || class >= int(p.MarkToMarkFmt1.MarkClassCount) {
				return pos, false, buf, nil
			}
			mark2Rec := p.MarkToMarkFmt1.Mark2Records[mark2Inx]
			if class >= len(mark2Rec.Anchors) || markRec.Anchor == nil || mark2Rec.Anchors[class] == nil {
				return pos, false, buf, nil
			}
			ctx.buf.EnsurePos()
			if ctx.buf.Pos == nil || mpos >= len(ctx.buf.Pos) {
				return pos, false, buf, nil
			}

			// Keep unresolved anchor reference behavior aligned with legacy path when possible.
			var markAnchor uint16
			var baseAnchor uint16
			if lksup, ok := lksub.Support.(struct {
				Mark2Coverage  ot.Coverage
				MarkClassCount uint16
				Mark1Array     ot.MarkArray
				Mark2Array     []ot.BaseRecord
			}); ok {
				if markInx >= 0 && markInx < len(lksup.Mark1Array.MarkRecords) {
					markAnchor = lksup.Mark1Array.MarkRecords[markInx].MarkAnchor
				}
				if mark2Inx >= 0 && mark2Inx < len(lksup.Mark2Array) {
					if class >= 0 && class < len(lksup.Mark2Array[mark2Inx].BaseAnchors) {
						baseAnchor = lksup.Mark2Array[mark2Inx].BaseAnchors[class]
					}
				}
			}
			ref := AnchorRef{
				MarkAnchor: markAnchor,
				BaseAnchor: baseAnchor,
			}
			setMarkAttachment(&ctx.buf.Pos[mpos], mark2Pos, AttachMarkToMark, markRec.Class, ref)
			return mpos + 1, true, buf, nil
		}
	}
	tracer().Errorf("GPOS 6|1 missing concrete payload")
	return pos, false, buf, nil
}

// GPOS Lookup Type 7, Format 1: Contextual Positioning (glyph-based).
func gposLookupType7Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GPosContextFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.ContextFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 7|1 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.RuleSets) {
		return pos, false, buf, nil
	}
	rules := payload.RuleSets[inx]
	for _, rule := range rules {
		restPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, rule.InputGlyphs)
		if !ok {
			continue
		}
		matchPositions := make([]int, 0, 1+len(restPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, restPos...)
		if len(rule.Records) == 0 || ctx.lookupList == nil {
			continue
		}
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
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
	var payload *ot.GPosContextFmt2Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.ContextFmt2
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 7|2 missing concrete payload")
		return pos, false, buf, nil
	}
	firstClass := payload.ClassDef.Lookup(buf.At(mpos))
	if firstClass < 0 || firstClass >= len(payload.RuleSets) {
		return pos, false, buf, nil
	}
	rules := payload.RuleSets[firstClass]
	for _, rule := range rules {
		restPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, payload.ClassDef, rule.InputClasses)
		if !ok {
			continue
		}
		matchPositions := make([]int, 0, 1+len(restPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, restPos...)
		if len(rule.Records) == 0 || ctx.lookupList == nil {
			continue
		}
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
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
	var payload *ot.GPosContextFmt3Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.ContextFmt3
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 7|3 missing concrete payload")
		return pos, false, buf, nil
	}
	if len(payload.InputCoverages) == 0 {
		return pos, false, buf, nil
	}
	inputPos, ok := matchCoverageSequenceForward(ctx, buf, pos, payload.InputCoverages)
	if !ok {
		return pos, false, buf, nil
	}
	if len(payload.Records) == 0 || ctx.lookupList == nil {
		return pos, false, buf, nil
	}
	out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, inputPos, payload.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
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
	var payload *ot.GPosChainingContextFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.ChainingContextFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 8|1 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.RuleSets) {
		return pos, false, buf, nil
	}
	rules := payload.RuleSets[inx]
	if len(rules) == 0 {
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
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
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
	var payload *ot.GPosChainingContextFmt2Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.ChainingContextFmt2
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 8|2 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.RuleSets) {
		return pos, false, buf, nil
	}
	rules := payload.RuleSets[inx]
	if len(rules) == 0 {
		return pos, false, buf, nil
	}
	for _, rule := range rules {
		inputPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, payload.InputClassDef, rule.Input)
		if !ok {
			continue
		}
		matchPositions := make([]int, 0, 1+len(inputPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, inputPos...)
		if len(rule.Backtrack) > 0 {
			if _, ok := matchClassSequenceBackward(ctx, buf, mpos, payload.BacktrackClassDef, rule.Backtrack); !ok {
				continue
			}
		}
		if len(rule.Lookahead) > 0 {
			last := matchPositions[len(matchPositions)-1]
			if _, ok := matchClassSequenceForward(ctx, buf, last+1, payload.LookaheadClassDef, rule.Lookahead); !ok {
				continue
			}
		}
		if ctx.lookupList == nil {
			return pos, false, buf, nil
		}
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
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
	var payload *ot.GPosChainingContextFmt3Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil {
			payload = p.ChainingContextFmt3
		}
	}
	if payload == nil {
		tracer().Errorf("GPOS 8|3 missing concrete payload")
		return pos, false, buf, nil
	}
	if len(payload.InputCoverages) == 0 {
		return pos, false, buf, nil
	}
	var backtrackFn matchSeqFn
	if len(payload.BacktrackCoverages) > 0 {
		backtrackFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceBackward(ctx, buf, pos, payload.BacktrackCoverages)
		}
	}
	inputFn := func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
		return matchCoverageSequenceForward(ctx, buf, pos, payload.InputCoverages)
	}
	var lookaheadFn matchSeqFn
	if len(payload.LookaheadCoverages) > 0 {
		lookaheadFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceForward(ctx, buf, pos+1, payload.LookaheadCoverages)
		}
	}
	inputPos, ok := matchChainedForward(ctx, buf, pos, backtrackFn, inputFn, lookaheadFn)
	if !ok {
		return pos, false, buf, nil
	}
	if len(payload.Records) == 0 {
		return pos, false, buf, nil
	}
	if ctx.lookupList == nil {
		return pos, false, buf, nil
	}
	out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, inputPos, payload.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
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
	if ctx.subnode != nil {
		if p := ctx.subnode.GPosPayload(); p != nil && p.CursiveFmt1 != nil {
			if inx < 0 || inx >= len(p.CursiveFmt1.Entries) {
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
			hasEntry := p.CursiveFmt1.Entries[inx].Entry != nil
			hasExit := p.CursiveFmt1.Entries[inx].Exit != nil

			next, ok := nextMatchable(ctx, buf, mpos+1)
			if !ok {
				return pos, false, buf, nil
			}
			if hasExit {
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
			if hasEntry {
				prev, ok := prevMatchable(ctx, buf, mpos-1)
				if ok {
					prevInx, ok := lksub.Coverage.Match(buf.At(prev))
					if ok && prevInx >= 0 && prevInx < len(p.CursiveFmt1.Entries) && p.CursiveFmt1.Entries[prevInx].Exit != nil {
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
			}
			return pos, false, buf, nil
		}
	}
	tracer().Errorf("GPOS 3|1 missing concrete payload")
	return pos, false, buf, nil
}
