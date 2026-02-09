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
	mpos, _, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage) // format 1 does not use the Coverage index
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubSingleFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.SingleFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 1|1 missing concrete payload")
		return pos, false, buf, nil
	}
	delta := int(payload.DeltaGlyphID)
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
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubSingleFmt2Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.SingleFmt2
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 1|2 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.SubstituteGlyphIDs) {
		return pos, false, buf, nil
	}
	glyph := payload.SubstituteGlyphIDs[inx]
	tracer().Debugf("OT lookup GSUB 1/2 (concrete): subst %d for %d", glyph, buf.At(mpos))
	ctx.buf.Set(mpos, glyph)
	return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
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
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubMultipleFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.MultipleFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 2|1 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.Sequences) {
		return pos, false, buf, nil
	}
	glyphs := payload.Sequences[inx]
	if len(glyphs) == 0 {
		return pos, false, buf, nil
	}
	tracer().Debugf("OT lookup GSUB 2/1 (concrete): subst %v for %d", glyphs, buf.At(mpos))
	edit := ctx.buf.ReplaceGlyphs(mpos, mpos+1, glyphs)
	return mpos + len(glyphs), true, ctx.buf.Glyphs, edit
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
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubAlternateFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.AlternateFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 3|1 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.Alternates) {
		return pos, false, buf, nil
	}
	glyphs := payload.Alternates[inx]
	if len(glyphs) == 0 {
		return pos, false, buf, nil
	}
	if alt < 0 {
		alt = len(glyphs) - 1
	}
	if alt >= len(glyphs) {
		return pos, false, buf, nil
	}
	tracer().Debugf("OT lookup GSUB 3/1 (concrete): subst %v for %d", glyphs[alt], buf.At(mpos))
	ctx.buf.Set(mpos, glyphs[alt])
	return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
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
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubLigatureFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.LigatureFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 4|1 missing concrete payload")
		return pos, false, buf, nil
	}
	if inx < 0 || inx >= len(payload.LigatureSets) {
		return pos, false, buf, nil
	}
	for _, rule := range payload.LigatureSets[inx] {
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
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubContextFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.ContextFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 5|1 missing concrete payload")
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
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType5Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, _, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubContextFmt2Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.ContextFmt2
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 5|2 missing concrete payload")
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
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType5Fmt3(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	var payload *ot.GSubContextFmt3Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.ContextFmt3
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 5|3 missing concrete payload")
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

func gsubLookupType6Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubChainingContextFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.ChainingContextFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 6|1 missing concrete payload")
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
			last := mpos
			if len(inputPos) > 0 {
				last = inputPos[len(inputPos)-1]
			}
			if _, ok := matchGlyphSequenceForward(ctx, buf, last+1, rule.Lookahead); !ok {
				continue
			}
		}
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

func gsubLookupType6Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if !ok {
		return pos, false, buf, nil
	}
	var payload *ot.GSubChainingContextFmt2Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.ChainingContextFmt2
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 6|2 missing concrete payload")
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
			last := mpos
			if len(inputPos) > 0 {
				last = inputPos[len(inputPos)-1]
			}
			if _, ok := matchClassSequenceForward(ctx, buf, last+1, payload.LookaheadClassDef, rule.Lookahead); !ok {
				continue
			}
		}
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
	var payload *ot.GSubChainingContextFmt3Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.ChainingContextFmt3
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 6|3 missing concrete payload")
		return pos, false, buf, nil
	}
	if len(payload.InputCoverages) == 0 {
		return pos, false, buf, nil
	}
	inputFn := func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
		return matchCoverageSequenceForward(ctx, buf, pos, payload.InputCoverages)
	}
	var backtrackFn matchSeqFn
	if len(payload.BacktrackCoverages) > 0 {
		backtrackFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceBackward(ctx, buf, pos, payload.BacktrackCoverages)
		}
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

// GSUB LookupType 8: Reverse Chaining Single Substitution Subtable
func gsubLookupType8Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	var payload *ot.GSubReverseChainingFmt1Payload
	if ctx.subnode != nil {
		if p := ctx.subnode.GSubPayload(); p != nil {
			payload = p.ReverseChainingFmt1
		}
	}
	if payload == nil {
		tracer().Errorf("GSUB 8|1 missing concrete payload")
		return pos, false, buf, nil
	}
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
		if len(payload.BacktrackCoverages) > 0 {
			if _, ok := matchCoverageSequenceBackward(ctx, buf, mpos, payload.BacktrackCoverages); !ok {
				tracer().Debugf("GSUB 8|1 backtrack did not match at pos %d", mpos)
				i = mpos - 1
				continue
			}
		}
		if len(payload.LookaheadCoverages) > 0 {
			if _, ok := matchCoverageSequenceForward(ctx, buf, mpos+1, payload.LookaheadCoverages); !ok {
				tracer().Debugf("GSUB 8|1 lookahead did not match at pos %d", mpos)
				i = mpos - 1
				continue
			}
		}
		if inx < 0 || inx >= len(payload.SubstituteGlyphIDs) {
			tracer().Debugf("GSUB 8|1 substitute index %d out of range", inx)
			i = mpos - 1
			continue
		}
		subst := payload.SubstituteGlyphIDs[inx]
		tracer().Debugf("GSUB 8|1 subst %d for %d at pos %d", subst, buf.At(mpos), mpos)
		ctx.buf.Set(mpos, subst)
		return mpos + 1, true, ctx.buf.Glyphs, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
	}
	return pos, false, buf, nil
}
