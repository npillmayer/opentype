package otlayout

import "github.com/npillmayer/opentype/ot"

// GSUB LookupType 1: Single Substitution Subtable
//
// Single substitution (SingleSubst) subtables tell a client to replace a single glyph
// with another glyph. The subtables can be either of two formats. Both formats require
// two distinct sets of glyph indices: one that defines input glyphs (specified in the
// Coverage table), and one that defines the output glyphs.

// GSUB LookupSubtable Type 1 Format 1 calculates the indices of the output glyphs, which
// are not explicitly defined in the subtable. To calculate an output glyph index,
// Format 1 adds a constant delta value to the input glyph index. For the substitutions to
// occur properly, the glyph indices in the input and output ranges must be in the same order.
// This format does not use the Coverage index that is returned from the Coverage table.
func gsubLookupType1Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, _, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage) // format 1 does not use the Coverage index
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d", buf.At(mpos), ok)
	} else {
		return pos, false, buf, nil
	}
	// support is deltaGlyphID: add to original glyph ID to get substitute glyph ID
	var delta int
	fromConcrete := false
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.SingleFmt1 != nil {
			delta = int(p.SingleFmt1.DeltaGlyphID)
			fromConcrete = true
		}
	}
	if !fromConcrete {
		switch v := lksub.Support.(type) {
		case int16:
			delta = int(v)
		case ot.GlyphIndex:
			delta = int(v)
		case int:
			delta = v
		default:
			tracer().Errorf("GSUB 1/1: unexpected delta type %T", lksub.Support)
			return pos, false, buf, nil
		}
	}
	newGlyph := int(buf.At(mpos)) + delta
	tracer().Debugf("OT lookup GSUB 1/1: subst %d for %d", newGlyph, buf.At(mpos))
	// TODO: check bounds against max glyph ID
	ctx.buf.Set(mpos, ot.GlyphIndex(newGlyph))
	return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
}

// GSUB LookupSubtable Type 1 Format 2 provides an array of output glyph indices
// (substituteGlyphIDs) explicitly matched to the input glyph indices specified in the
// Coverage table.
// The substituteGlyphIDs array must contain the same number of glyph indices as the
// Coverage table. To locate the corresponding output glyph index in the substituteGlyphIDs
// array, this format uses the Coverage index returned from the Coverage table.
func gsubLookupType1Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	} else {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.SingleFmt2 != nil {
			if inx >= 0 && inx < len(p.SingleFmt2.SubstituteGlyphIDs) {
				glyph := p.SingleFmt2.SubstituteGlyphIDs[inx]
				tracer().Debugf("OT lookup GSUB 1/2 (concrete): subst %d for %d", glyph, buf.At(mpos))
				ctx.buf.Set(mpos, glyph)
				return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
			}
			return pos, false, buf, nil
		}
	}
	if glyph := lookupGlyph(lksub.Index, inx, false); glyph != 0 {
		tracer().Debugf("OT lookup GSUB 1/2: subst %d for %d", glyph, buf.At(mpos))
		ctx.buf.Set(mpos, glyph)
		return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
	}
	return pos, false, buf, nil
}

// LookupType 2: Multiple Substitution Subtable
//
// A Multiple Substitution (MultipleSubst) subtable replaces a single glyph with more
// than one glyph, as when multiple glyphs replace a single ligature.

// GSUB LookupSubtable Type 2 Format 1 defines a count of offsets in the sequenceOffsets
// array (sequenceCount), and an array of offsets to Sequence tables that define the output
// glyph indices (sequenceOffsets). The Sequence table offsets are ordered by the Coverage
// index of the input glyphs.
// For each input glyph listed in the Coverage table, a Sequence table defines the output
// glyphs. Each Sequence table contains a count of the glyphs in the output glyph sequence
// (glyphCount) and an array of output glyph indices (substituteGlyphIDs).
func gsubLookupType2Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	} else {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.MultipleFmt1 != nil {
			if inx >= 0 && inx < len(p.MultipleFmt1.Sequences) {
				glyphs := p.MultipleFmt1.Sequences[inx]
				if len(glyphs) != 0 {
					tracer().Debugf("OT lookup GSUB 2/1 (concrete): subst %v for %d", glyphs, buf.At(mpos))
					edit := ctx.buf.ReplaceGlyphs(mpos, mpos+1, glyphs)
					return mpos + len(glyphs), true, ctx.buf.Glyphs, edit
				}
			}
			return pos, false, buf, nil
		}
	}
	if glyphs := lookupGlyphs(lksub.Index, inx, true); len(glyphs) != 0 {
		tracer().Debugf("OT lookup GSUB 2/1: subst %v for %d", glyphs, buf.At(mpos))
		edit := ctx.buf.ReplaceGlyphs(mpos, mpos+1, glyphs)
		return mpos + len(glyphs), true, ctx.buf.Glyphs, edit
	}
	return pos, false, buf, nil
}

// LookupType 3: Alternate Substitution Subtable
//
// An Alternate Substitution (AlternateSubst) subtable identifies any number of aesthetic
// alternatives from which a user can choose a glyph variant to replace the input glyph.
// For example, if a font contains four variants of the ampersand symbol, the 'cmap' table
// will specify the index of one of the four glyphs as the default glyph index, and an
// AlternateSubst subtable will list the indices of the other three glyphs as alternatives.
// A text-processing client would then have the option of replacing the default glyph with
// any of the three alternatives.

// GSUB LookupSubtable Type 3 Format 1: For each glyph, an AlternateSet subtable contains a
// count of the alternative glyphs (glyphCount) and an array of their glyph indices
// (alternateGlyphIDs). Parameter `alt` selects an alternative glyph from this array.
// Having `alt` set to -1 will selected the last alternative glyph from the array.
func gsubLookupType3Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos, alt int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	} else {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.AlternateFmt1 != nil {
			if inx >= 0 && inx < len(p.AlternateFmt1.Alternates) {
				glyphs := p.AlternateFmt1.Alternates[inx]
				if len(glyphs) == 0 {
					return pos, false, buf, nil
				}
				if alt < 0 {
					alt = len(glyphs) - 1
				}
				if alt < len(glyphs) {
					tracer().Debugf("OT lookup GSUB 3/1 (concrete): subst %v for %d", glyphs[alt], buf.At(mpos))
					ctx.buf.Set(mpos, glyphs[alt])
					return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
				}
			}
			return pos, false, buf, nil
		}
	}
	if glyphs := lookupGlyphs(lksub.Index, inx, true); len(glyphs) != 0 {
		if alt < 0 {
			alt = len(glyphs) - 1
		}
		if alt < len(glyphs) {
			tracer().Debugf("OT lookup GSUB 3/1: subst %v for %d", glyphs[alt], buf.At(mpos))
			ctx.buf.Set(mpos, glyphs[alt])
			return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
		}
	}
	return pos, false, buf, nil
}

// LookupType 4: Ligature Substitution Subtable
//
// A Ligature Substitution (LigatureSubst) subtable identifies ligature substitutions where
// a single glyph replaces multiple glyphs. One LigatureSubst subtable can specify any number
// of ligature substitutions.

// GSUB LookupSubtable Type 4 Format 1 receives a sequence of glyphs and outputs a
// single glyph replacing the sequence. The Coverage table specifies only the index of the
// first glyph component of each ligature set.
//
// As this is a multi-lookup algorithm, calling gsubLookupType4Fmt1 will return a
// NavLocation which is a LigatureSet, i.e. a list of records of unequal lengths.
func gsubLookupType4Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	} else {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.LigatureFmt1 != nil {
			if inx < 0 || inx >= len(p.LigatureFmt1.LigatureSets) {
				return pos, false, buf, nil
			}
			for _, rule := range p.LigatureFmt1.LigatureSets[inx] {
				match := true
				cur := mpos
				for _, g := range rule.Components {
					next, ok := nextMatchable(ctx, buf, cur+1)
					if !ok || g != buf.At(next) {
						match = false
						break
					}
					cur = next
				}
				if match {
					edit := ctx.buf.ReplaceGlyphs(mpos, cur+1, []ot.GlyphIndex{rule.Ligature})
					tracer().Debugf("OT lookup GSUB 4/1 (concrete): subst %d for %d", rule.Ligature, buf.At(mpos))
					return mpos + 1, true, ctx.buf.Glyphs, edit
				}
			}
			return pos, false, buf, nil
		}
	}
	ligatureSet, err := lksub.Index.Get(inx, false)
	if err != nil || ligatureSet.Size() < 2 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 4|1 ligature set size = %d", ligatureSet.Size())
	ligCount := ligatureSet.U16(0)
	if ligatureSet.Size() < int(2+ligCount*2) { // must have room for count and u16offset[count]
		return pos, false, buf, nil
	}
	for i := range int(ligCount) { // iterate over every ligature record in a ligature table
		ligpos := int(ligatureSet.U16(2 + i*2)) // jump to start of ligature record
		if ligatureSet.Size() < ligpos+6 {
			return pos, false, buf, nil
		}
		// Ligature table (glyph components for one ligature):
		// uint16 |  ligatureGlyph                       |  glyph ID of ligature to substitute
		// uint16 |  componentCount                      |  Number of components in the ligature
		// uint16 |  componentGlyphIDs[componentCount-1] |  Array of component glyph IDs
		componentCount := int(ligatureSet.U16(ligpos + 2))
		if componentCount == 0 || componentCount > 10 { // 10 is arbitrary, just to be careful
			continue
		}
		componentGlyphs := ligatureSet.Slice(ligpos+4, ligpos+4+(componentCount-1)*2).Glyphs()
		tracer().Debugf("%d component glyphs of ligature: %d %v", componentCount, buf.At(mpos), componentGlyphs)
		// now we know that buf[mpos] has matched the first glyph of the component pattern and
		// we will have to match following glyphs to the remaining componentGlyphs
		match := true
		cur := mpos
		for _, g := range componentGlyphs {
			next, ok := nextMatchable(ctx, buf, cur+1)
			if !ok || g != buf.At(next) {
				match = false
				break
			}
			cur = next
		}
		if match {
			ligatureGlyph := ot.GlyphIndex(ligatureSet.U16(ligpos))
			edit := ctx.buf.ReplaceGlyphs(mpos, cur+1, []ot.GlyphIndex{ligatureGlyph})
			tracer().Debugf("after application of ligature, glyph = %d", ctx.buf.At(mpos))
			return mpos + 1, true, ctx.buf.Glyphs, edit
		}
	}
	return pos, false, buf, nil
}

// LookupType 5: Contextual Substitution
//
// GSUB type 5 format 1 subtables (and GPOS type 7 format 1 subtables) define input sequences in terms of
// specific glyph IDs. Several sequences may be specified, but each is specified using glyph IDs.
//
// The first glyphs for the sequences are specified in a Coverage table. The remaining glyphs in each
// sequence are defined in SequenceRule tables—one for each sequence. If multiple sequences start with
// the same glyph, that glyph ID must be listed once in the Coverage table, and the corresponding sequence
// rules are aggregated using a SequenceRuleSet table—one for each initial glyph specified in the
// Coverage table.
//
// When evaluating a SequenceContextFormat1 subtable for a given position in a glyph sequence, the client
// searches for the current glyph in the Coverage table. If found, the corresponding SequenceRuleSet
// table is retrieved, and the SequenceRule tables for that set are examined to see if the current glyph
// sequence matches any of the sequence rules. The first matching rule subtable is used.
func gsubLookupType5Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	} else {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.ContextFmt1 != nil {
			if inx < 0 || inx >= len(p.ContextFmt1.RuleSets) {
				return pos, false, buf, nil
			}
			rules := p.ContextFmt1.RuleSets[inx]
			tracer().Debugf("GSUB 5|1 concrete rule set has %d rules", len(rules))
			for i, rule := range rules {
				restPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, rule.InputGlyphs)
				if !ok {
					continue
				}
				tracer().Debugf("GSUB 5|1 concrete rule #%d matched at positions %v", i, append([]int{mpos}, restPos...))
				matchPositions := make([]int, 0, 1+len(restPos))
				matchPositions = append(matchPositions, mpos)
				matchPositions = append(matchPositions, restPos...)
				if len(rule.Records) == 0 || ctx.lookupList == nil {
					continue
				}
				out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
				ctx.buf.Pos = outPosBuf
				if applied {
					return pos, true, out, nil
				}
			}
			return pos, false, buf, nil
		}
	}
	ruleSetLoc, err := lksub.Index.Get(inx, false)
	if err != nil || ruleSetLoc.Size() < 2 { // extra coverage glyphs or extra sequence rule sets are ignored
		return pos, false, buf, nil
	}
	// SequenceRuleSet table – all contexts beginning with the same glyph:
	// uint16   | seqRuleCount                 | Number of SequenceRule tables
	// Offset16 | seqRuleOffsets[seqRuleCount] | Array of offsets to SequenceRule tables, from
	//                                           beginning of the SequenceRuleSet table
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "SequenceRuleSet")
	tracer().Debugf("GSUB 5|1 rule set has %d rules", ruleSet.Size())
	for i := range ruleSet.Size() {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 4 {
			continue
		}
		// SequenceRule table:
		// uint16 | glyphCount                  | Number of glyphs in the input glyph sequence
		// uint16 | seqLookupCount              | Number of SequenceLookupRecords
		// uint16 | inputSequence[glyphCount-1] | Array of input glyph IDs—starting with the second glyph
		// SequenceLookupRecord | seqLookupRecords[seqLookupCount] | Array of Sequence lookup records
		glyphCount := int(ruleLoc.U16(0))
		seqLookupCount := int(ruleLoc.U16(2))
		if glyphCount < 1 {
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
		tracer().Debugf("GSUB 5|1 rule #%d input glyphs = %v", i, inputGlyphs)
		restPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, inputGlyphs)
		if !ok {
			continue
		}
		tracer().Debugf("GSUB 5|1 rule #%d matched at positions %v", i, append([]int{mpos}, restPos...))
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
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		tracer().Debugf("GSUB 5|1 rule #%d applied = %v", i, applied)
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType5Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.ContextFmt2 != nil {
			firstClass := p.ContextFmt2.ClassDef.Lookup(buf.At(mpos))
			if firstClass < 0 || firstClass >= len(p.ContextFmt2.RuleSets) {
				return pos, false, buf, nil
			}
			rules := p.ContextFmt2.RuleSets[firstClass]
			tracer().Debugf("GSUB 5|2 concrete rule set has %d rules", len(rules))
			for i, rule := range rules {
				restPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, p.ContextFmt2.ClassDef, rule.InputClasses)
				if !ok {
					continue
				}
				tracer().Debugf("GSUB 5|2 concrete rule #%d matched at positions %v", i, append([]int{mpos}, restPos...))
				matchPositions := make([]int, 0, 1+len(restPos))
				matchPositions = append(matchPositions, mpos)
				matchPositions = append(matchPositions, restPos...)
				if len(rule.Records) == 0 || ctx.lookupList == nil {
					continue
				}
				out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
				ctx.buf.Pos = outPosBuf
				if applied {
					return pos, true, out, nil
				}
			}
			return pos, false, buf, nil
		}
	}
	if lksub.Support == nil {
		tracer().Errorf("expected SequenceContext|ClassDefs in field 'Support', is nil")
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok {
		tracer().Errorf("expected SequenceContext|ClassDefs in field 'Support', type error")
		return pos, false, buf, nil
	}
	if len(seqctx.ClassDefs) == 0 {
		tracer().Errorf("SequenceContext has no ClassDefs for GSUB 5|2")
		return pos, false, buf, nil
	}
	firstClass := seqctx.ClassDefs[0].Lookup(buf.At(mpos))
	tracer().Debugf("GSUB 5|2 first glyph class = %d", firstClass)
	ruleSetLoc, err := lksub.Index.Get(int(firstClass), false)
	if err != nil || ruleSetLoc.Size() < 2 {
		return pos, false, buf, nil
	}
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "SequenceRuleSet")
	tracer().Debugf("GSUB 5|2 rule set has %d rules", ruleSet.Size())
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 4 {
			continue
		}
		glyphCount := int(ruleLoc.U16(0))
		seqLookupCount := int(ruleLoc.U16(2))
		if glyphCount < 1 {
			continue
		}
		classCount := glyphCount - 1
		classBytes := classCount * 2
		recBytes := seqLookupCount * 4
		minSize := 4 + classBytes + recBytes
		if ruleLoc.Size() < minSize {
			continue
		}
		classes := make([]uint16, classCount)
		for j := range classCount {
			classes[j] = ruleLoc.U16(4 + j*2)
		}
		tracer().Debugf("GSUB 5|2 rule #%d classes = %v", i, classes)
		restPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, seqctx.ClassDefs[0], classes)
		if !ok {
			continue
		}
		tracer().Debugf("GSUB 5|2 rule #%d matched at positions %v", i, append([]int{mpos}, restPos...))
		matchPositions := make([]int, 0, glyphCount)
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, restPos...)
		records := make([]ot.SequenceLookupRecord, seqLookupCount)
		recStart := 4 + classBytes
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
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		tracer().Debugf("GSUB 5|2 rule #%d applied = %v", i, applied)
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType5Fmt3(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.ContextFmt3 != nil {
			if len(p.ContextFmt3.InputCoverages) == 0 {
				return pos, false, buf, nil
			}
			inputPos, ok := matchCoverageSequenceForward(ctx, buf, pos, p.ContextFmt3.InputCoverages)
			if !ok || len(p.ContextFmt3.Records) == 0 || ctx.lookupList == nil {
				return pos, false, buf, nil
			}
			out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, inputPos, p.ContextFmt3.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
			ctx.buf.Pos = outPosBuf
			if applied {
				return pos, true, out, nil
			}
			return pos, false, buf, nil
		}
	}
	if lksub.Support == nil {
		tracer().Errorf("expected SequenceContext in field 'Support', is nil")
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok {
		tracer().Errorf("expected SequenceContext in field 'Support', type error")
		return pos, false, buf, nil
	}
	if len(seqctx.InputCoverage) == 0 {
		tracer().Errorf("SequenceContext has no InputCoverage for GSUB 5|3")
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
	out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, inputPos, lksub.LookupRecords, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
	ctx.buf.Pos = outPosBuf
	if applied {
		return pos, true, out, nil
	}
	return pos, false, buf, nil
}

func gsubLookupType6Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	rules, err := parseChainedSequenceRules(lksub, ctx.subnode, inx)
	if err != nil || len(rules) == 0 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|1 rule set for coverage %d: %d rules", inx, len(rules))
	for _, rule := range rules {
		tracer().Debugf("GSUB 6|1 rule: backtrack=%d input=%d lookahead=%d records=%d",
			len(rule.Backtrack), len(rule.Input), len(rule.Lookahead), len(rule.Records))
		inputPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, rule.Input)
		if !ok {
			tracer().Debugf("GSUB 6|1 input sequence did not match at pos %d", mpos+1)
			continue
		}
		matchPositions := make([]int, 0, 1+len(inputPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, inputPos...)
		if len(rule.Backtrack) > 0 {
			if _, ok := matchGlyphSequenceBackward(ctx, buf, mpos, rule.Backtrack); !ok {
				tracer().Debugf("GSUB 6|1 backtrack did not match at pos %d", mpos)
				continue
			}
		}
		if len(rule.Lookahead) > 0 {
			last := mpos
			if len(inputPos) > 0 {
				last = inputPos[len(inputPos)-1]
			}
			if _, ok := matchGlyphSequenceForward(ctx, buf, last+1, rule.Lookahead); !ok {
				tracer().Debugf("GSUB 6|1 lookahead did not match at pos %d", last+1)
				continue
			}
		}
		if len(rule.Records) == 0 || ctx.lookupList == nil {
			continue
		}
		tracer().Debugf("GSUB 6|1 matched at positions %v", matchPositions)
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType6Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	var classDefs []ot.ClassDefinitions
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.ChainingContextFmt2 != nil {
			classDefs = []ot.ClassDefinitions{
				p.ChainingContextFmt2.BacktrackClassDef,
				p.ChainingContextFmt2.InputClassDef,
				p.ChainingContextFmt2.LookaheadClassDef,
			}
		}
	}
	if len(classDefs) < 3 {
		seqctx, ok := lksub.Support.(*ot.SequenceContext)
		if !ok || seqctx == nil || len(seqctx.ClassDefs) < 3 {
			tracer().Debugf("GSUB 6|2 missing class definitions")
			return pos, false, buf, nil
		}
		classDefs = seqctx.ClassDefs
	}
	rules, err := parseChainedClassSequenceRules(lksub, ctx.subnode, inx)
	if err != nil || len(rules) == 0 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|2 rule set for coverage %d: %d rules", inx, len(rules))
	for _, rule := range rules {
		tracer().Debugf("GSUB 6|2 rule: backtrack=%d input=%d lookahead=%d records=%d",
			len(rule.Backtrack), len(rule.Input), len(rule.Lookahead), len(rule.Records))
		inputPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, classDefs[1], rule.Input)
		if !ok {
			tracer().Debugf("GSUB 6|2 input classes did not match at pos %d", mpos+1)
			continue
		}
		matchPositions := make([]int, 0, 1+len(inputPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, inputPos...)
		if len(rule.Backtrack) > 0 {
			if _, ok := matchClassSequenceBackward(ctx, buf, mpos, classDefs[0], rule.Backtrack); !ok {
				tracer().Debugf("GSUB 6|2 backtrack did not match at pos %d", mpos)
				continue
			}
		}
		if len(rule.Lookahead) > 0 {
			last := mpos
			if len(inputPos) > 0 {
				last = inputPos[len(inputPos)-1]
			}
			if _, ok := matchClassSequenceForward(ctx, buf, last+1, classDefs[2], rule.Lookahead); !ok {
				tracer().Debugf("GSUB 6|2 lookahead did not match at pos %d", last+1)
				continue
			}
		}
		if len(rule.Records) == 0 || ctx.lookupList == nil {
			continue
		}
		tracer().Debugf("GSUB 6|2 matched at positions %v", matchPositions)
		out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, matchPositions, rule.Records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
		ctx.buf.Pos = outPosBuf
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

// Chained Sequence Context Format 3: coverage-based glyph contexts
// GSUB type 6 format 3 subtables and GPOS type 6 format 3 subtables define input sequences patterns, as
// well as chained backtrack and lookahead sequence patterns, in terms of sets of glyph defined using
// Coverage tables.
// The ChainedSequenceContextFormat3 table specifies exactly one input sequence pattern. It has three
// arrays of offsets to coverage tables: one for the input sequence pattern, one for the backtrack
// sequence pattern, and one for the lookahead sequence pattern. For each array, the offsets correspond,
// in order, to the positions in the sequence pattern.
func gsubLookupType6Fmt3(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	records := lksub.LookupRecords
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.ChainingContextFmt3 != nil {
			seqctx = &ot.SequenceContext{
				BacktrackCoverage: p.ChainingContextFmt3.BacktrackCoverages,
				InputCoverage:     p.ChainingContextFmt3.InputCoverages,
				LookaheadCoverage: p.ChainingContextFmt3.LookaheadCoverages,
			}
			records = p.ChainingContextFmt3.Records
			ok = true
		}
	}
	if !ok || len(seqctx.InputCoverage) == 0 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|3 coverages: backtrack=%d input=%d lookahead=%d records=%d",
		len(seqctx.BacktrackCoverage), len(seqctx.InputCoverage), len(seqctx.LookaheadCoverage), len(records))
	tracer().Debugf("GSUB 6|3 pos=%d glyph=%d", pos, buf.At(pos))
	if len(seqctx.InputCoverage) > 0 {
		tracer().Debugf("GSUB 6|3 input[0] contains glyph %d = %v", buf.At(pos), seqctx.InputCoverage[0].Contains(buf.At(pos)))
		if pos+1 < buf.Len() {
			tracer().Debugf("GSUB 6|3 input[0] contains glyph %d (pos+1) = %v", buf.At(pos+1), seqctx.InputCoverage[0].Contains(buf.At(pos+1)))
		}
		tracer().Debugf("GSUB 6|3 input[0] contains glyph 76 = %v", seqctx.InputCoverage[0].Contains(76))
		tracer().Debugf("GSUB 6|3 input[0] contains glyph 2195 = %v", seqctx.InputCoverage[0].Contains(2195))
		tracer().Debugf("GSUB 6|3 input[0] contains glyph 18944 = %v", seqctx.InputCoverage[0].Contains(18944))
	}
	inputFn := func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
		if len(seqctx.InputCoverage) > 0 {
			if _, ok := seqctx.InputCoverage[0].Match(buf.At(pos)); !ok {
				tracer().Debugf("GSUB 6|3 first input coverage did not match glyph %d at pos %d", buf.At(pos), pos)
			}
		}
		return matchCoverageSequenceForward(ctx, buf, pos, seqctx.InputCoverage)
	}
	var backtrackFn matchSeqFn
	if len(seqctx.BacktrackCoverage) > 0 {
		backtrackFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceBackward(ctx, buf, pos, seqctx.BacktrackCoverage)
		}
	}
	var lookaheadFn matchSeqFn
	if len(seqctx.LookaheadCoverage) > 0 {
		lookaheadFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceForward(ctx, buf, pos+1, seqctx.LookaheadCoverage)
		}
	}
	inputPos, ok := matchChainedForward(ctx, buf, pos, backtrackFn, inputFn, lookaheadFn)
	if !ok {
		tracer().Debugf("GSUB 6|3 no match at pos %d", pos)
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|3 matched at positions %v", inputPos)
	if len(records) == 0 {
		tracer().Debugf("GSUB 6|3 has no lookup records")
		return pos, false, buf, nil
	}
	if ctx.lookupList == nil {
		tracer().Debugf("GSUB 6|3 missing lookup list")
		return pos, false, buf, nil
	}
	out, outPosBuf, applied := applySequenceLookupRecords(buf, ctx.buf.Pos, inputPos, records, ctx.lookupList, ctx.lookupGraph, ctx.feat, ctx.alt, ctx.gdef)
	ctx.buf.Pos = outPosBuf
	tracer().Debugf("GSUB 6|3 applied = %v", applied)
	if applied {
		return pos, true, out, nil
	}
	return pos, false, buf, nil
}

// GSUB LookupType 8: Reverse Chaining Single Substitution Subtable
func gsubLookupType8Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	var rc *ot.ReverseChainingSubst
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil && p.ReverseChainingFmt1 != nil {
			rc = &ot.ReverseChainingSubst{
				BacktrackCoverage:  p.ReverseChainingFmt1.BacktrackCoverages,
				LookaheadCoverage:  p.ReverseChainingFmt1.LookaheadCoverages,
				SubstituteGlyphIDs: p.ReverseChainingFmt1.SubstituteGlyphIDs,
			}
		}
	}
	if rc == nil {
		var ok bool
		rc, ok = lksub.Support.(*ot.ReverseChainingSubst)
		if !ok || rc == nil {
			tracer().Debugf("GSUB 8|1 missing ReverseChainingSubst support")
			return pos, false, buf, nil
		}
	}
	tracer().Debugf("GSUB 8|1 pos=%d backtrack=%d lookahead=%d subst=%d",
		pos, len(rc.BacktrackCoverage), len(rc.LookaheadCoverage), len(rc.SubstituteGlyphIDs))
	minPos := max(0, pos)
	// if minPos < 0 {
	// 	minPos = 0
	// }
	for i := buf.Len() - 1; i >= minPos; {
		mpos, ok := prevMatchable(ctx, buf, i)
		if !ok || mpos < minPos {
			break
		}
		tracer().Debugf("GSUB 8|1 candidate pos=%d glyph=%d", mpos, buf.At(mpos))
		inx, ok := lksub.Coverage.Match(buf.At(mpos))
		if !ok {
			tracer().Debugf("GSUB 8|1 coverage did not match at pos %d", mpos)
			i = mpos - 1
			continue
		}
		if len(rc.BacktrackCoverage) > 0 {
			if _, ok := matchCoverageSequenceBackward(ctx, buf, mpos, rc.BacktrackCoverage); !ok {
				tracer().Debugf("GSUB 8|1 backtrack did not match at pos %d", mpos)
				i = mpos - 1
				continue
			}
		}
		if len(rc.LookaheadCoverage) > 0 {
			if _, ok := matchCoverageSequenceForward(ctx, buf, mpos+1, rc.LookaheadCoverage); !ok {
				tracer().Debugf("GSUB 8|1 lookahead did not match at pos %d", mpos)
				i = mpos - 1
				continue
			}
		}
		if inx < 0 || inx >= len(rc.SubstituteGlyphIDs) {
			tracer().Debugf("GSUB 8|1 substitute index %d out of range", inx)
			i = mpos - 1
			continue
		}
		subst := rc.SubstituteGlyphIDs[inx]
		tracer().Debugf("GSUB 8|1 subst %d for %d at pos %d", subst, buf.At(mpos), mpos)
		ctx.buf.Set(mpos, subst)
		return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
	}
	return pos, false, buf, nil
}
