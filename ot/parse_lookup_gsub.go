package ot

import "fmt"

func parseConcreteGSubPayload(node *LookupNode, depth int) {
	if node == nil || node.GSub == nil || len(node.raw) < 4 {
		return
	}
	switch node.LookupType {
	case GSubLookupTypeSingle, GSubLookupTypeMultiple, GSubLookupTypeAlternate, GSubLookupTypeLigature:
		cov, err := parseCoverageAt(node.raw, 2)
		if err != nil {
			setLookupNodeError(node, err)
		} else {
			node.Coverage = cov
		}
	case GSubLookupTypeContext:
		if node.Format != 3 {
			cov, err := parseCoverageAt(node.raw, 2)
			if err != nil {
				setLookupNodeError(node, err)
			} else {
				node.Coverage = cov
			}
		}
	case GSubLookupTypeChainingContext:
		if node.Format != 3 {
			cov, err := parseCoverageAt(node.raw, 2)
			if err != nil {
				setLookupNodeError(node, err)
			} else {
				node.Coverage = cov
			}
		}
	case GSubLookupTypeReverseChaining:
		cov, err := parseCoverageAt(node.raw, 2)
		if err != nil {
			setLookupNodeError(node, err)
		} else {
			node.Coverage = cov
		}
	}
	switch node.LookupType {
	case GSubLookupTypeSingle:
		parseConcreteGSubType1(node)
	case GSubLookupTypeMultiple:
		parseConcreteGSubType2(node)
	case GSubLookupTypeAlternate:
		parseConcreteGSubType3(node)
	case GSubLookupTypeLigature:
		parseConcreteGSubType4(node)
	case GSubLookupTypeContext:
		parseConcreteGSubType5(node)
	case GSubLookupTypeChainingContext:
		parseConcreteGSubType6(node)
	case GSubLookupTypeReverseChaining:
		parseConcreteGSubType8(node)
	case GSubLookupTypeExtensionSubs:
		parseConcreteGSubType7(node, depth)
	}
}

func parseConcreteGSubType1(node *LookupNode) {
	switch node.Format {
	case 1:
		if node.GSub.SingleFmt1 == nil {
			return
		}
		if len(node.raw) < 6 {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		node.GSub.SingleFmt1.DeltaGlyphID = int16(node.raw.U16(4))
	case 2:
		if node.GSub.SingleFmt2 == nil {
			return
		}
		array, err := parseArray16(node.raw, 4, "GSUB1", "SubstituteGlyphIDs")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.SingleFmt2.SubstituteGlyphIDs = make([]GlyphIndex, array.Len())
		for i := 0; i < array.Len(); i++ {
			node.GSub.SingleFmt2.SubstituteGlyphIDs[i] = GlyphIndex(array.Get(i).U16(0))
		}
	}
}

func parseConcreteGSubType2(node *LookupNode) {
	if node.Format != 1 || node.GSub.MultipleFmt1 == nil {
		return
	}
	sequenceOffsets, err := parseArray16(node.raw, 4, "GSUB2", "SequenceOffsets")
	if err != nil {
		setLookupNodeError(node, err)
		return
	}
	node.GSub.MultipleFmt1.Sequences = make([][]GlyphIndex, sequenceOffsets.Len())
	for i := 0; i < sequenceOffsets.Len(); i++ {
		off := sequenceOffsets.Get(i).U16(0)
		if off == 0 || int(off) >= len(node.raw) {
			setLookupNodeError(node, fmt.Errorf("GSUB2 sequence offset out of bounds: %d (size %d)", off, len(node.raw)))
			continue
		}
		glyphs, err := parseGlyphSlice16(node.raw[off:], 0, "GSUB2.Sequence")
		if err != nil {
			setLookupNodeError(node, err)
			continue
		}
		node.GSub.MultipleFmt1.Sequences[i] = glyphs
	}
}

func parseConcreteGSubType3(node *LookupNode) {
	if node.Format != 1 || node.GSub.AlternateFmt1 == nil {
		return
	}
	altSetOffsets, err := parseArray16(node.raw, 4, "GSUB3", "AlternateSetOffsets")
	if err != nil {
		setLookupNodeError(node, err)
		return
	}
	node.GSub.AlternateFmt1.Alternates = make([][]GlyphIndex, altSetOffsets.Len())
	for i := 0; i < altSetOffsets.Len(); i++ {
		off := altSetOffsets.Get(i).U16(0)
		if off == 0 || int(off) >= len(node.raw) {
			setLookupNodeError(node, fmt.Errorf("GSUB3 alternate-set offset out of bounds: %d (size %d)", off, len(node.raw)))
			continue
		}
		glyphs, err := parseGlyphSlice16(node.raw[off:], 0, "GSUB3.AlternateSet")
		if err != nil {
			setLookupNodeError(node, err)
			continue
		}
		node.GSub.AlternateFmt1.Alternates[i] = glyphs
	}
}

func parseConcreteGSubType4(node *LookupNode) {
	if node.Format != 1 || node.GSub.LigatureFmt1 == nil {
		return
	}
	ligSetOffsets, err := parseArray16(node.raw, 4, "GSUB4", "LigatureSetOffsets")
	if err != nil {
		setLookupNodeError(node, err)
		return
	}
	node.GSub.LigatureFmt1.LigatureSets = make([][]GSubLigatureRule, ligSetOffsets.Len())
	for i := 0; i < ligSetOffsets.Len(); i++ {
		setOffset := ligSetOffsets.Get(i).U16(0)
		if setOffset == 0 || int(setOffset) >= len(node.raw) {
			setLookupNodeError(node, fmt.Errorf("GSUB4 ligature-set offset out of bounds: %d (size %d)", setOffset, len(node.raw)))
			continue
		}
		ligSet := node.raw[setOffset:]
		ligOffsets, err := parseArray16(ligSet, 0, "GSUB4.LigatureSet", "LigatureOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			continue
		}
		rules := make([]GSubLigatureRule, 0, ligOffsets.Len())
		for j := 0; j < ligOffsets.Len(); j++ {
			ligOffset := ligOffsets.Get(j).U16(0)
			if ligOffset == 0 || int(ligOffset) >= len(ligSet) {
				setLookupNodeError(node, fmt.Errorf("GSUB4 ligature offset out of bounds: %d (size %d)", ligOffset, len(ligSet)))
				continue
			}
			rule, err := parseLigatureRule(ligSet[ligOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			rules = append(rules, rule)
		}
		node.GSub.LigatureFmt1.LigatureSets[i] = rules
	}
}

func parseConcreteGSubType5(node *LookupNode) {
	switch node.Format {
	case 1:
		if node.GSub.ContextFmt1 == nil {
			return
		}
		ruleSetOffsets, err := parseArray16(node.raw, 4, "GSUB5", "SequenceRuleSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.ContextFmt1.RuleSets = make([][]GSubSequenceRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GSUB5/1 rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubSequenceRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GSub.ContextFmt1.RuleSets[i] = rules
		}
	case 2:
		if node.GSub.ContextFmt2 == nil {
			return
		}
		classDef, err := parseContextClassDef(node.raw, 4)
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.ContextFmt2.ClassDef = classDef
		ruleSetOffsets, err := parseArray16(node.raw, 6, "GSUB5", "ClassSequenceRuleSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.ContextFmt2.RuleSets = make([][]GSubClassSequenceRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GSUB5/2 class-rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubClassSequenceRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GSub.ContextFmt2.RuleSets[i] = rules
		}
	case 3:
		if node.GSub.ContextFmt3 == nil {
			return
		}
		if len(node.raw) < 6 {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		glyphCount := int(node.raw.U16(2))
		seqLookupCount := int(node.raw.U16(4))
		need := 6 + glyphCount*2 + seqLookupCount*4
		if need > len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		node.GSub.ContextFmt3.InputCoverages = make([]Coverage, glyphCount)
		for i := 0; i < glyphCount; i++ {
			link, err := parseLink16(node.raw, 6+i*2, node.raw, "GSUB5/3.InputCoverage")
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GSub.ContextFmt3.InputCoverages[i] = parseCoverage(link.jump().Bytes())
		}
		records, err := parseSequenceLookupRecords(node.raw, 6+glyphCount*2, seqLookupCount)
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.ContextFmt3.Records = records
	}
}

func parseConcreteGSubType6(node *LookupNode) {
	switch node.Format {
	case 1:
		if node.GSub.ChainingContextFmt1 == nil {
			return
		}
		ruleSetOffsets, err := parseArray16(node.raw, 4, "GSUB6", "ChainSubRuleSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.ChainingContextFmt1.RuleSets = make([][]GSubChainedSequenceRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GSUB6/1 rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubChainedSequenceRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GSub.ChainingContextFmt1.RuleSets[i] = rules
		}
	case 2:
		if node.GSub.ChainingContextFmt2 == nil {
			return
		}
		backtrack, err1 := parseContextClassDef(node.raw, 4)
		input, err2 := parseContextClassDef(node.raw, 6)
		lookahead, err3 := parseContextClassDef(node.raw, 8)
		if err1 != nil || err2 != nil || err3 != nil {
			if err1 != nil {
				setLookupNodeError(node, err1)
			} else if err2 != nil {
				setLookupNodeError(node, err2)
			} else {
				setLookupNodeError(node, err3)
			}
			return
		}
		node.GSub.ChainingContextFmt2.BacktrackClassDef = backtrack
		node.GSub.ChainingContextFmt2.InputClassDef = input
		node.GSub.ChainingContextFmt2.LookaheadClassDef = lookahead
		ruleSetOffsets, err := parseArray16(node.raw, 10, "GSUB6", "ChainSubClassSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.ChainingContextFmt2.RuleSets = make([][]GSubChainedClassRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GSUB6/2 class-rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubChainedClassRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GSub.ChainingContextFmt2.RuleSets[i] = rules
		}
	case 3:
		if node.GSub.ChainingContextFmt3 == nil {
			return
		}
		backtrack, next, err := parseCoverageList(node.raw, 2, "GSUB6/3.Backtrack")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		input, next, err := parseCoverageList(node.raw, next, "GSUB6/3.Input")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		lookahead, next, err := parseCoverageList(node.raw, next, "GSUB6/3.Lookahead")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		if next+2 > len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		seqLookupCount := int(node.raw.U16(next))
		records, err := parseSequenceLookupRecords(node.raw, next+2, seqLookupCount)
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GSub.ChainingContextFmt3.BacktrackCoverages = backtrack
		node.GSub.ChainingContextFmt3.InputCoverages = input
		node.GSub.ChainingContextFmt3.LookaheadCoverages = lookahead
		node.GSub.ChainingContextFmt3.Records = records
	}
}

func parseConcreteGSubType7(node *LookupNode, depth int) {
	if node.Format != 1 || node.GSub.ExtensionFmt1 == nil {
		return
	}
	if depth >= MaxExtensionDepth {
		setLookupNodeError(node, fmt.Errorf("lookup subtable exceeds maximum extension depth %d", MaxExtensionDepth))
		return
	}
	if len(node.raw) < 8 {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	actualType := LayoutTableLookupType(node.raw.U16(2))
	if actualType == GSubLookupTypeExtensionSubs {
		setLookupNodeError(node, fmt.Errorf("GSUB extension subtable cannot recursively reference extension type"))
		return
	}
	link, err := parseLink32(node.raw, 4, node.raw, "GSUB7.Extension")
	if err != nil {
		setLookupNodeError(node, err)
		return
	}
	resolved := parseConcreteLookupNodeWithDepth(link.jump().Bytes(), actualType, depth+1)
	node.GSub.ExtensionFmt1.ResolvedType = actualType
	node.GSub.ExtensionFmt1.Resolved = resolved
	if resolved != nil {
		node.Coverage = resolved.Coverage
		if resolved.err != nil {
			setLookupNodeError(node, resolved.err)
		}
	}
}

func parseConcreteGSubType8(node *LookupNode) {
	if node.Format != 1 || node.GSub.ReverseChainingFmt1 == nil {
		return
	}
	backtrack, next, err := parseCoverageList(node.raw, 4, "GSUB8.Backtrack")
	if err != nil {
		setLookupNodeError(node, err)
		return
	}
	lookahead, next, err := parseCoverageList(node.raw, next, "GSUB8.Lookahead")
	if err != nil {
		setLookupNodeError(node, err)
		return
	}
	if next+2 > len(node.raw) {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	glyphCount := int(node.raw.U16(next))
	next += 2
	if next+glyphCount*2 > len(node.raw) {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	subst := make([]GlyphIndex, glyphCount)
	for i := 0; i < glyphCount; i++ {
		subst[i] = GlyphIndex(node.raw.U16(next + i*2))
	}
	node.GSub.ReverseChainingFmt1.BacktrackCoverages = backtrack
	node.GSub.ReverseChainingFmt1.LookaheadCoverages = lookahead
	node.GSub.ReverseChainingFmt1.SubstituteGlyphIDs = subst
}

func parseCoverageAt(b binarySegm, at int) (Coverage, error) {
	link, err := parseLink16(b, at, b, "Coverage")
	if err != nil {
		return Coverage{}, err
	}
	return parseCoverage(link.jump().Bytes()), nil
}

func parseGlyphSlice16(b binarySegm, countAt int, name string) ([]GlyphIndex, error) {
	array, err := parseArray16(b, countAt, name, "GlyphIDs")
	if err != nil {
		return nil, err
	}
	glyphs := make([]GlyphIndex, array.Len())
	for i := 0; i < array.Len(); i++ {
		glyphs[i] = GlyphIndex(array.Get(i).U16(0))
	}
	return glyphs, nil
}

func parseLigatureRule(b binarySegm) (GSubLigatureRule, error) {
	if len(b) < 4 {
		return GSubLigatureRule{}, errBufferBounds
	}
	ligGlyph := GlyphIndex(b.U16(0))
	componentCount := int(b.U16(2))
	if componentCount < 1 {
		return GSubLigatureRule{}, fmt.Errorf("GSUB4 ligature has illegal component count %d", componentCount)
	}
	need := 4 + (componentCount-1)*2
	if need > len(b) {
		return GSubLigatureRule{}, errBufferBounds
	}
	components := make([]GlyphIndex, componentCount-1)
	for i := 0; i < componentCount-1; i++ {
		components[i] = GlyphIndex(b.U16(4 + i*2))
	}
	return GSubLigatureRule{
		Components: components,
		Ligature:   ligGlyph,
	}, nil
}

func parseGSubSequenceRuleSet(b binarySegm) ([]GSubSequenceRule, error) {
	ruleOffsets, err := parseArray16(b, 0, "GSUB5.SequenceRuleSet", "SequenceRuleOffsets")
	if err != nil {
		return nil, err
	}
	rules := make([]GSubSequenceRule, 0, ruleOffsets.Len())
	for i := 0; i < ruleOffsets.Len(); i++ {
		off := ruleOffsets.Get(i).U16(0)
		if off == 0 {
			continue
		}
		if int(off) >= len(b) {
			return nil, errBufferBounds
		}
		rule, err := parseGSubSequenceRule(b[off:])
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseGSubClassSequenceRuleSet(b binarySegm) ([]GSubClassSequenceRule, error) {
	ruleOffsets, err := parseArray16(b, 0, "GSUB5.ClassSequenceRuleSet", "ClassSequenceRuleOffsets")
	if err != nil {
		return nil, err
	}
	rules := make([]GSubClassSequenceRule, 0, ruleOffsets.Len())
	for i := 0; i < ruleOffsets.Len(); i++ {
		off := ruleOffsets.Get(i).U16(0)
		if off == 0 {
			continue
		}
		if int(off) >= len(b) {
			return nil, errBufferBounds
		}
		rule, err := parseGSubClassSequenceRule(b[off:])
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseGSubSequenceRule(b binarySegm) (GSubSequenceRule, error) {
	if len(b) < 4 {
		return GSubSequenceRule{}, errBufferBounds
	}
	glyphCount := int(b.U16(0))
	seqLookupCount := int(b.U16(2))
	inputCount := glyphCount - 1
	if inputCount < 0 {
		return GSubSequenceRule{}, errBufferBounds
	}
	need := 4 + inputCount*2 + seqLookupCount*4
	if need > len(b) {
		return GSubSequenceRule{}, errBufferBounds
	}
	rule := GSubSequenceRule{
		InputGlyphs: make([]GlyphIndex, inputCount),
	}
	for i := 0; i < inputCount; i++ {
		rule.InputGlyphs[i] = GlyphIndex(b.U16(4 + i*2))
	}
	records, err := parseSequenceLookupRecords(b, 4+inputCount*2, seqLookupCount)
	if err != nil {
		return GSubSequenceRule{}, err
	}
	rule.Records = records
	return rule, nil
}

func parseGSubClassSequenceRule(b binarySegm) (GSubClassSequenceRule, error) {
	if len(b) < 4 {
		return GSubClassSequenceRule{}, errBufferBounds
	}
	glyphCount := int(b.U16(0))
	seqLookupCount := int(b.U16(2))
	inputCount := glyphCount - 1
	if inputCount < 0 {
		return GSubClassSequenceRule{}, errBufferBounds
	}
	need := 4 + inputCount*2 + seqLookupCount*4
	if need > len(b) {
		return GSubClassSequenceRule{}, errBufferBounds
	}
	rule := GSubClassSequenceRule{
		InputClasses: make([]uint16, inputCount),
	}
	for i := 0; i < inputCount; i++ {
		rule.InputClasses[i] = b.U16(4 + i*2)
	}
	records, err := parseSequenceLookupRecords(b, 4+inputCount*2, seqLookupCount)
	if err != nil {
		return GSubClassSequenceRule{}, err
	}
	rule.Records = records
	return rule, nil
}

func parseSequenceLookupRecords(b binarySegm, at, count int) ([]SequenceLookupRecord, error) {
	need := at + count*4
	if at < 0 || need > len(b) {
		return nil, errBufferBounds
	}
	records := make([]SequenceLookupRecord, count)
	for i := 0; i < count; i++ {
		off := at + i*4
		records[i] = SequenceLookupRecord{
			SequenceIndex:   b.U16(off),
			LookupListIndex: b.U16(off + 2),
		}
	}
	return records, nil
}

func parseGSubChainedSequenceRuleSet(b binarySegm) ([]GSubChainedSequenceRule, error) {
	ruleOffsets, err := parseArray16(b, 0, "GSUB6.ChainSubRuleSet", "ChainSubRuleOffsets")
	if err != nil {
		return nil, err
	}
	rules := make([]GSubChainedSequenceRule, 0, ruleOffsets.Len())
	for i := 0; i < ruleOffsets.Len(); i++ {
		off := ruleOffsets.Get(i).U16(0)
		if off == 0 {
			continue
		}
		if int(off) >= len(b) {
			return nil, errBufferBounds
		}
		rule, err := parseGSubChainedSequenceRule(b[off:])
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseGSubChainedClassRuleSet(b binarySegm) ([]GSubChainedClassRule, error) {
	ruleOffsets, err := parseArray16(b, 0, "GSUB6.ChainSubClassSet", "ChainSubClassRuleOffsets")
	if err != nil {
		return nil, err
	}
	rules := make([]GSubChainedClassRule, 0, ruleOffsets.Len())
	for i := 0; i < ruleOffsets.Len(); i++ {
		off := ruleOffsets.Get(i).U16(0)
		if off == 0 {
			continue
		}
		if int(off) >= len(b) {
			return nil, errBufferBounds
		}
		rule, err := parseGSubChainedClassRule(b[off:])
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseGSubChainedSequenceRule(b binarySegm) (GSubChainedSequenceRule, error) {
	offset := 0
	backtrack, next, err := parseGlyphList(b, offset)
	if err != nil {
		return GSubChainedSequenceRule{}, err
	}
	offset = next
	input, next, err := parseInputGlyphList(b, offset)
	if err != nil {
		return GSubChainedSequenceRule{}, err
	}
	offset = next
	lookahead, next, err := parseGlyphList(b, offset)
	if err != nil {
		return GSubChainedSequenceRule{}, err
	}
	offset = next
	if offset+2 > len(b) {
		return GSubChainedSequenceRule{}, errBufferBounds
	}
	seqLookupCount := int(b.U16(offset))
	records, err := parseSequenceLookupRecords(b, offset+2, seqLookupCount)
	if err != nil {
		return GSubChainedSequenceRule{}, err
	}
	return GSubChainedSequenceRule{
		Backtrack: backtrack,
		Input:     input,
		Lookahead: lookahead,
		Records:   records,
	}, nil
}

func parseGSubChainedClassRule(b binarySegm) (GSubChainedClassRule, error) {
	offset := 0
	backtrack, next, err := parseClassList(b, offset)
	if err != nil {
		return GSubChainedClassRule{}, err
	}
	offset = next
	input, next, err := parseInputClassList(b, offset)
	if err != nil {
		return GSubChainedClassRule{}, err
	}
	offset = next
	lookahead, next, err := parseClassList(b, offset)
	if err != nil {
		return GSubChainedClassRule{}, err
	}
	offset = next
	if offset+2 > len(b) {
		return GSubChainedClassRule{}, errBufferBounds
	}
	seqLookupCount := int(b.U16(offset))
	records, err := parseSequenceLookupRecords(b, offset+2, seqLookupCount)
	if err != nil {
		return GSubChainedClassRule{}, err
	}
	return GSubChainedClassRule{
		Backtrack: backtrack,
		Input:     input,
		Lookahead: lookahead,
		Records:   records,
	}, nil
}

func parseCoverageList(b binarySegm, at int, name string) ([]Coverage, int, error) {
	if at+2 > len(b) {
		return nil, 0, errBufferBounds
	}
	count := int(b.U16(at))
	if at+2+count*2 > len(b) {
		return nil, 0, errBufferBounds
	}
	coverages := make([]Coverage, count)
	for i := 0; i < count; i++ {
		link, err := parseLink16(b, at+2+i*2, b, name)
		if err != nil {
			return nil, 0, err
		}
		coverages[i] = parseCoverage(link.jump().Bytes())
	}
	return coverages, at + 2 + count*2, nil
}

func parseGlyphList(b binarySegm, at int) ([]GlyphIndex, int, error) {
	if at+2 > len(b) {
		return nil, 0, errBufferBounds
	}
	count := int(b.U16(at))
	off := at + 2
	if off+count*2 > len(b) {
		return nil, 0, errBufferBounds
	}
	out := make([]GlyphIndex, count)
	for i := 0; i < count; i++ {
		out[i] = GlyphIndex(b.U16(off + i*2))
	}
	return out, off + count*2, nil
}

func parseClassList(b binarySegm, at int) ([]uint16, int, error) {
	if at+2 > len(b) {
		return nil, 0, errBufferBounds
	}
	count := int(b.U16(at))
	off := at + 2
	if off+count*2 > len(b) {
		return nil, 0, errBufferBounds
	}
	out := make([]uint16, count)
	for i := 0; i < count; i++ {
		out[i] = b.U16(off + i*2)
	}
	return out, off + count*2, nil
}

func parseInputGlyphList(b binarySegm, at int) ([]GlyphIndex, int, error) {
	if at+2 > len(b) {
		return nil, 0, errBufferBounds
	}
	glyphCount := int(b.U16(at))
	inputCount := glyphCount - 1
	if inputCount < 0 {
		return nil, 0, errBufferBounds
	}
	off := at + 2
	if off+inputCount*2 > len(b) {
		return nil, 0, errBufferBounds
	}
	out := make([]GlyphIndex, inputCount)
	for i := 0; i < inputCount; i++ {
		out[i] = GlyphIndex(b.U16(off + i*2))
	}
	return out, off + inputCount*2, nil
}

func parseInputClassList(b binarySegm, at int) ([]uint16, int, error) {
	if at+2 > len(b) {
		return nil, 0, errBufferBounds
	}
	glyphCount := int(b.U16(at))
	inputCount := glyphCount - 1
	if inputCount < 0 {
		return nil, 0, errBufferBounds
	}
	off := at + 2
	if off+inputCount*2 > len(b) {
		return nil, 0, errBufferBounds
	}
	out := make([]uint16, inputCount)
	for i := 0; i < inputCount; i++ {
		out[i] = b.U16(off + i*2)
	}
	return out, off + inputCount*2, nil
}

func setLookupNodeError(node *LookupNode, err error) {
	if node != nil && err != nil && node.err == nil {
		node.err = err
	}
}
