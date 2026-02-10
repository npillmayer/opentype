package ot

import "fmt"

func parseConcreteGPosPayload(node *LookupNode, depth int) {
	if node == nil || node.GPos == nil || len(node.raw) < 4 {
		return
	}
	gposType := GPosLookupType(node.LookupType)
	switch gposType {
	case GPosLookupTypeSingle, GPosLookupTypePair, GPosLookupTypeCursive, GPosLookupTypeMarkToBase, GPosLookupTypeMarkToLigature, GPosLookupTypeMarkToMark:
		cov, err := parseCoverageAt(node.raw, 2)
		if err != nil {
			setLookupNodeError(node, err)
		} else {
			node.Coverage = cov
		}
	case GPosLookupTypeContextPos, GPosLookupTypeChainedContextPos:
		if node.Format != 3 {
			cov, err := parseCoverageAt(node.raw, 2)
			if err != nil {
				setLookupNodeError(node, err)
			} else {
				node.Coverage = cov
			}
		}
	}
	switch gposType {
	case GPosLookupTypeSingle:
		parseConcreteGPosType1(node)
	case GPosLookupTypePair:
		parseConcreteGPosType2(node)
	case GPosLookupTypeCursive:
		parseConcreteGPosType3(node)
	case GPosLookupTypeMarkToBase:
		parseConcreteGPosType4(node)
	case GPosLookupTypeMarkToLigature:
		parseConcreteGPosType5(node)
	case GPosLookupTypeMarkToMark:
		parseConcreteGPosType6(node)
	case GPosLookupTypeContextPos:
		parseConcreteGPosType7(node)
	case GPosLookupTypeChainedContextPos:
		parseConcreteGPosType8(node)
	case GPosLookupTypeExtensionPos:
		parseConcreteGPosType9(node, depth)
	}
}

func parseConcreteGPosType1(node *LookupNode) {
	switch node.Format {
	case 1:
		if node.GPos.SingleFmt1 == nil {
			return
		}
		if len(node.raw) < 6 {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		valueFormat := ValueFormat(node.raw.U16(4))
		size := valueRecordSize(valueFormat)
		if 6+size > len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		vr, _ := parseValueRecord(node.raw, 6, valueFormat)
		node.GPos.SingleFmt1.ValueFormat = valueFormat
		node.GPos.SingleFmt1.Value = vr
	case 2:
		if node.GPos.SingleFmt2 == nil {
			return
		}
		if len(node.raw) < 8 {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		valueFormat := ValueFormat(node.raw.U16(4))
		valueCount := int(node.raw.U16(6))
		size := valueRecordSize(valueFormat)
		if 8+valueCount*size > len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		values := make([]ValueRecord, valueCount)
		offset := 8
		for i := range valueCount {
			vr, n := parseValueRecord(node.raw, offset, valueFormat)
			values[i] = vr
			offset += n
		}
		node.GPos.SingleFmt2.ValueFormat = valueFormat
		node.GPos.SingleFmt2.Values = values
	}
}

func parseConcreteGPosType2(node *LookupNode) {
	switch node.Format {
	case 1:
		if node.GPos.PairFmt1 == nil {
			return
		}
		if len(node.raw) < 8 {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		valueFormat1 := ValueFormat(node.raw.U16(4))
		valueFormat2 := ValueFormat(node.raw.U16(6))
		pairSetOffsets, err := parseArray16(node.raw, 8, "GPOS2", "PairSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		pairSets := make([][]PairValueRecord, pairSetOffsets.Len())
		for i := 0; i < pairSetOffsets.Len(); i++ {
			off := pairSetOffsets.Get(i).U16(0)
			if off == 0 || int(off) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GPOS2/1 pair-set offset out of bounds: %d (size %d)", off, len(node.raw)))
				continue
			}
			records, err := parseGPosPairSet(node.raw[off:], valueFormat1, valueFormat2)
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			pairSets[i] = records
		}
		node.GPos.PairFmt1.ValueFormat1 = valueFormat1
		node.GPos.PairFmt1.ValueFormat2 = valueFormat2
		node.GPos.PairFmt1.PairSets = pairSets
	case 2:
		if node.GPos.PairFmt2 == nil {
			return
		}
		if len(node.raw) < 16 {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		valueFormat1 := ValueFormat(node.raw.U16(4))
		valueFormat2 := ValueFormat(node.raw.U16(6))
		classDef1, err1 := parseContextClassDef(node.raw, 8)
		classDef2, err2 := parseContextClassDef(node.raw, 10)
		if err1 != nil || err2 != nil {
			if err1 != nil {
				setLookupNodeError(node, err1)
			} else {
				setLookupNodeError(node, err2)
			}
			return
		}
		class1Count := int(node.raw.U16(12))
		class2Count := int(node.raw.U16(14))
		recSize1 := valueRecordSize(valueFormat1)
		recSize2 := valueRecordSize(valueFormat2)
		need := 16 + class1Count*class2Count*(recSize1+recSize2)
		if need > len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
			return
		}
		records := make([][]GPosClass2ValueRecord, class1Count)
		offset := 16
		for i := range class1Count {
			row := make([]GPosClass2ValueRecord, class2Count)
			for j := range class2Count {
				v1, n1 := parseValueRecord(node.raw, offset, valueFormat1)
				offset += n1
				v2, n2 := parseValueRecord(node.raw, offset, valueFormat2)
				offset += n2
				row[j] = GPosClass2ValueRecord{
					Value1: v1,
					Value2: v2,
				}
			}
			records[i] = row
		}
		node.GPos.PairFmt2.ValueFormat1 = valueFormat1
		node.GPos.PairFmt2.ValueFormat2 = valueFormat2
		node.GPos.PairFmt2.ClassDef1 = classDef1
		node.GPos.PairFmt2.ClassDef2 = classDef2
		node.GPos.PairFmt2.Class1Count = uint16(class1Count)
		node.GPos.PairFmt2.Class2Count = uint16(class2Count)
		node.GPos.PairFmt2.ClassRecords = records
	}
}

func parseConcreteGPosType3(node *LookupNode) {
	if node.Format != 1 || node.GPos.CursiveFmt1 == nil {
		return
	}
	if len(node.raw) < 6 {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	entryExitCount := int(node.raw.U16(4))
	need := 6 + entryExitCount*4
	if need > len(node.raw) {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	entries := make([]GPosEntryExitAnchor, entryExitCount)
	offsets := make([]gposEntryExitOffsets, entryExitCount)
	for i := range entryExitCount {
		base := 6 + i*4
		entryOff := node.raw.U16(base)
		exitOff := node.raw.U16(base + 2)
		offsets[i] = gposEntryExitOffsets{entry: entryOff, exit: exitOff}
		if entryOff != 0 {
			if int(entryOff) >= len(node.raw) {
				setLookupNodeError(node, errBufferBounds)
			} else {
				a := parseAnchor(node.raw[entryOff:])
				entries[i].Entry = &a
			}
		}
		if exitOff != 0 {
			if int(exitOff) >= len(node.raw) {
				setLookupNodeError(node, errBufferBounds)
			} else {
				a := parseAnchor(node.raw[exitOff:])
				entries[i].Exit = &a
			}
		}
	}
	node.GPos.CursiveFmt1.Entries = entries
	node.GPos.CursiveFmt1.entryExitOffsets = offsets
}

func parseConcreteGPosType4(node *LookupNode) {
	if node.Format != 1 || node.GPos.MarkToBaseFmt1 == nil {
		return
	}
	if len(node.raw) < 12 {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	baseCoverage, err := parseCoverageAt(node.raw, 4)
	if err != nil {
		setLookupNodeError(node, err)
	}
	markClassCount := int(node.raw.U16(6))
	markArrayOffset := node.raw.U16(8)
	baseArrayOffset := node.raw.U16(10)

	var marks []GPosMarkAttachRecord
	if markArrayOffset != 0 {
		if int(markArrayOffset) >= len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
		} else {
			records, err := parseGPosMarkAttachRecords(node.raw[markArrayOffset:])
			if err != nil {
				setLookupNodeError(node, err)
			}
			marks = records
		}
	}
	var bases []GPosBaseAttachRecord
	if baseArrayOffset != 0 {
		if int(baseArrayOffset) >= len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
		} else {
			records, err := parseGPosBaseAttachRecords(node.raw[baseArrayOffset:], markClassCount)
			if err != nil {
				setLookupNodeError(node, err)
			}
			bases = records
		}
	}

	node.GPos.MarkToBaseFmt1.BaseCoverage = baseCoverage
	node.GPos.MarkToBaseFmt1.MarkClassCount = uint16(markClassCount)
	node.GPos.MarkToBaseFmt1.MarkRecords = marks
	node.GPos.MarkToBaseFmt1.BaseRecords = bases
}

func parseConcreteGPosType5(node *LookupNode) {
	if node.Format != 1 || node.GPos.MarkToLigatureFmt1 == nil {
		return
	}
	if len(node.raw) < 12 {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	ligCoverage, err := parseCoverageAt(node.raw, 4)
	if err != nil {
		setLookupNodeError(node, err)
	}
	markClassCount := int(node.raw.U16(6))
	markArrayOffset := node.raw.U16(8)
	ligatureArrayOffset := node.raw.U16(10)

	var marks []GPosMarkAttachRecord
	if markArrayOffset != 0 {
		if int(markArrayOffset) >= len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
		} else {
			records, err := parseGPosMarkAttachRecords(node.raw[markArrayOffset:])
			if err != nil {
				setLookupNodeError(node, err)
			}
			marks = records
		}
	}
	var ligatures []GPosLigatureAttachRecord
	if ligatureArrayOffset != 0 {
		if int(ligatureArrayOffset) >= len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
		} else {
			records, err := parseGPosLigatureAttachRecords(node.raw[ligatureArrayOffset:], markClassCount)
			if err != nil {
				setLookupNodeError(node, err)
			}
			ligatures = records
		}
	}

	node.GPos.MarkToLigatureFmt1.LigatureCoverage = ligCoverage
	node.GPos.MarkToLigatureFmt1.MarkClassCount = uint16(markClassCount)
	node.GPos.MarkToLigatureFmt1.MarkRecords = marks
	node.GPos.MarkToLigatureFmt1.LigatureRecords = ligatures
}

func parseConcreteGPosType6(node *LookupNode) {
	if node.Format != 1 || node.GPos.MarkToMarkFmt1 == nil {
		return
	}
	if len(node.raw) < 12 {
		setLookupNodeError(node, errBufferBounds)
		return
	}
	mark2Coverage, err := parseCoverageAt(node.raw, 4)
	if err != nil {
		setLookupNodeError(node, err)
	}
	markClassCount := int(node.raw.U16(6))
	mark1ArrayOffset := node.raw.U16(8)
	mark2ArrayOffset := node.raw.U16(10)

	var mark1 []GPosMarkAttachRecord
	if mark1ArrayOffset != 0 {
		if int(mark1ArrayOffset) >= len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
		} else {
			records, err := parseGPosMarkAttachRecords(node.raw[mark1ArrayOffset:])
			if err != nil {
				setLookupNodeError(node, err)
			}
			mark1 = records
		}
	}
	var mark2 []GPosBaseAttachRecord
	if mark2ArrayOffset != 0 {
		if int(mark2ArrayOffset) >= len(node.raw) {
			setLookupNodeError(node, errBufferBounds)
		} else {
			records, err := parseGPosBaseAttachRecords(node.raw[mark2ArrayOffset:], markClassCount)
			if err != nil {
				setLookupNodeError(node, err)
			}
			mark2 = records
		}
	}

	node.GPos.MarkToMarkFmt1.Mark2Coverage = mark2Coverage
	node.GPos.MarkToMarkFmt1.MarkClassCount = uint16(markClassCount)
	node.GPos.MarkToMarkFmt1.Mark1Records = mark1
	node.GPos.MarkToMarkFmt1.Mark2Records = mark2
}

func parseConcreteGPosType7(node *LookupNode) {
	switch node.Format {
	case 1:
		if node.GPos.ContextFmt1 == nil {
			return
		}
		ruleSetOffsets, err := parseArray16(node.raw, 4, "GPOS7", "SequenceRuleSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GPos.ContextFmt1.RuleSets = make([][]GPosSequenceRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GPOS7/1 rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubSequenceRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GPos.ContextFmt1.RuleSets[i] = toGPosSequenceRules(rules)
		}
	case 2:
		if node.GPos.ContextFmt2 == nil {
			return
		}
		classDef, err := parseContextClassDef(node.raw, 4)
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GPos.ContextFmt2.ClassDef = classDef
		ruleSetOffsets, err := parseArray16(node.raw, 6, "GPOS7", "ClassSequenceRuleSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GPos.ContextFmt2.RuleSets = make([][]GPosClassSequenceRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GPOS7/2 class-rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubClassSequenceRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GPos.ContextFmt2.RuleSets[i] = toGPosClassSequenceRules(rules)
		}
	case 3:
		if node.GPos.ContextFmt3 == nil {
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
		node.GPos.ContextFmt3.InputCoverages = make([]Coverage, glyphCount)
		for i := range glyphCount {
			link, err := parseLink16(node.raw, 6+i*2, node.raw, "GPOS7/3.InputCoverage")
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GPos.ContextFmt3.InputCoverages[i] = parseCoverage(link.jump().Bytes())
		}
		records, err := parseSequenceLookupRecords(node.raw, 6+glyphCount*2, seqLookupCount)
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GPos.ContextFmt3.Records = records
	}
}

func parseConcreteGPosType8(node *LookupNode) {
	switch node.Format {
	case 1:
		if node.GPos.ChainingContextFmt1 == nil {
			return
		}
		ruleSetOffsets, err := parseArray16(node.raw, 4, "GPOS8", "ChainSubRuleSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GPos.ChainingContextFmt1.RuleSets = make([][]GPosChainedSequenceRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GPOS8/1 rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubChainedSequenceRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GPos.ChainingContextFmt1.RuleSets[i] = toGPosChainedSequenceRules(rules)
		}
	case 2:
		if node.GPos.ChainingContextFmt2 == nil {
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
		node.GPos.ChainingContextFmt2.BacktrackClassDef = backtrack
		node.GPos.ChainingContextFmt2.InputClassDef = input
		node.GPos.ChainingContextFmt2.LookaheadClassDef = lookahead
		ruleSetOffsets, err := parseArray16(node.raw, 10, "GPOS8", "ChainSubClassSetOffsets")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		node.GPos.ChainingContextFmt2.RuleSets = make([][]GPosChainedClassRule, ruleSetOffsets.Len())
		for i := 0; i < ruleSetOffsets.Len(); i++ {
			ruleSetOffset := ruleSetOffsets.Get(i).U16(0)
			if ruleSetOffset == 0 {
				continue
			}
			if int(ruleSetOffset) >= len(node.raw) {
				setLookupNodeError(node, fmt.Errorf("GPOS8/2 class-rule-set offset out of bounds: %d (size %d)", ruleSetOffset, len(node.raw)))
				continue
			}
			rules, err := parseGSubChainedClassRuleSet(node.raw[ruleSetOffset:])
			if err != nil {
				setLookupNodeError(node, err)
				continue
			}
			node.GPos.ChainingContextFmt2.RuleSets[i] = toGPosChainedClassRules(rules)
		}
	case 3:
		if node.GPos.ChainingContextFmt3 == nil {
			return
		}
		backtrack, next, err := parseCoverageList(node.raw, 2, "GPOS8/3.Backtrack")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		input, next, err := parseCoverageList(node.raw, next, "GPOS8/3.Input")
		if err != nil {
			setLookupNodeError(node, err)
			return
		}
		lookahead, next, err := parseCoverageList(node.raw, next, "GPOS8/3.Lookahead")
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
		node.GPos.ChainingContextFmt3.BacktrackCoverages = backtrack
		node.GPos.ChainingContextFmt3.InputCoverages = input
		node.GPos.ChainingContextFmt3.LookaheadCoverages = lookahead
		node.GPos.ChainingContextFmt3.Records = records
	}
}

func parseConcreteGPosType9(node *LookupNode, depth int) {
	if node.Format != 1 || node.GPos.ExtensionFmt1 == nil {
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
	if actualType == GPosLookupTypeExtensionPos {
		setLookupNodeError(node, fmt.Errorf("GPOS extension subtable cannot recursively reference extension type"))
		return
	}
	link, err := parseLink32(node.raw, 4, node.raw, "GPOS9.Extension")
	if err != nil {
		setLookupNodeError(node, err)
		return
	}
	resolvedType := MaskGPosLookupType(actualType)
	resolved := parseConcreteLookupNodeWithDepth(link.jump().Bytes(), resolvedType, depth+1)
	node.GPos.ExtensionFmt1.ResolvedType = resolvedType
	node.GPos.ExtensionFmt1.Resolved = resolved
	if resolved != nil {
		node.Coverage = resolved.Coverage
		if resolved.err != nil {
			setLookupNodeError(node, resolved.err)
		}
	}
}

func parseGPosPairSet(b binarySegm, format1, format2 ValueFormat) ([]PairValueRecord, error) {
	if len(b) < 2 {
		return nil, errBufferBounds
	}
	pairValueCount := int(b.U16(0))
	recordSize := 2 + valueRecordSize(format1) + valueRecordSize(format2)
	if 2+pairValueCount*recordSize > len(b) {
		return nil, errBufferBounds
	}
	records := make([]PairValueRecord, pairValueCount)
	offset := 2
	for i := range pairValueCount {
		second := b.U16(offset)
		offset += 2
		v1, n1 := parseValueRecord(b, offset, format1)
		offset += n1
		v2, n2 := parseValueRecord(b, offset, format2)
		offset += n2
		records[i] = PairValueRecord{
			SecondGlyph: second,
			Value1:      v1,
			Value2:      v2,
		}
	}
	return records, nil
}

func parseGPosMarkAttachRecords(markArray binarySegm) ([]GPosMarkAttachRecord, error) {
	if len(markArray) < 2 {
		return nil, errBufferBounds
	}
	markArrayRec := parseMarkArray(markArray)
	records := make([]GPosMarkAttachRecord, len(markArrayRec.MarkRecords))
	for i, rec := range markArrayRec.MarkRecords {
		out := GPosMarkAttachRecord{Class: rec.Class}
		out.anchorOffset = rec.MarkAnchor
		if rec.MarkAnchor != 0 {
			if int(rec.MarkAnchor) >= len(markArray) {
				return nil, errBufferBounds
			}
			anchor := parseAnchor(markArray[rec.MarkAnchor:])
			out.Anchor = &anchor
		}
		records[i] = out
	}
	return records, nil
}

func parseGPosBaseAttachRecords(baseArray binarySegm, classCount int) ([]GPosBaseAttachRecord, error) {
	if classCount < 0 || len(baseArray) < 2 {
		return nil, errBufferBounds
	}
	baseRecords := parseBaseArray(baseArray, classCount)
	records := make([]GPosBaseAttachRecord, len(baseRecords))
	for i, rec := range baseRecords {
		out := GPosBaseAttachRecord{
			Anchors:       make([]*Anchor, len(rec.BaseAnchors)),
			anchorOffsets: make([]uint16, len(rec.BaseAnchors)),
		}
		copy(out.anchorOffsets, rec.BaseAnchors)
		for j, off := range rec.BaseAnchors {
			if off == 0 {
				continue
			}
			if int(off) >= len(baseArray) {
				return nil, errBufferBounds
			}
			anchor := parseAnchor(baseArray[off:])
			out.Anchors[j] = &anchor
		}
		records[i] = out
	}
	return records, nil
}

func parseGPosLigatureAttachRecords(ligArray binarySegm, classCount int) ([]GPosLigatureAttachRecord, error) {
	if classCount < 0 || len(ligArray) < 2 {
		return nil, errBufferBounds
	}
	ligCount := int(ligArray.U16(0))
	if 2+ligCount*2 > len(ligArray) {
		return nil, errBufferBounds
	}
	records := make([]GPosLigatureAttachRecord, 0, ligCount)
	for i := range ligCount {
		off := ligArray.U16(2 + i*2)
		if off == 0 {
			continue
		}
		if int(off) >= len(ligArray) {
			return nil, errBufferBounds
		}
		ligAttach := ligArray[off:]
		if len(ligAttach) < 2 {
			return nil, errBufferBounds
		}
		componentCount := int(ligAttach.U16(0))
		need := 2 + componentCount*classCount*2
		if need > len(ligAttach) {
			return nil, errBufferBounds
		}
		record := GPosLigatureAttachRecord{
			ComponentAnchors:       make([][]*Anchor, componentCount),
			componentAnchorOffsets: make([][]uint16, componentCount),
		}
		offset := 2
		for c := range componentCount {
			componentAnchors := make([]*Anchor, classCount)
			componentAnchorOffsets := make([]uint16, classCount)
			for clz := range classCount {
				aoff := ligAttach.U16(offset)
				offset += 2
				componentAnchorOffsets[clz] = aoff
				if aoff == 0 {
					continue
				}
				if int(aoff) >= len(ligAttach) {
					return nil, errBufferBounds
				}
				anchor := parseAnchor(ligAttach[aoff:])
				componentAnchors[clz] = &anchor
			}
			record.ComponentAnchors[c] = componentAnchors
			record.componentAnchorOffsets[c] = componentAnchorOffsets
		}
		records = append(records, record)
	}
	return records, nil
}

func toGPosSequenceRules(in []GSubSequenceRule) []GPosSequenceRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]GPosSequenceRule, len(in))
	for i, r := range in {
		input := make([]GlyphIndex, len(r.InputGlyphs))
		copy(input, r.InputGlyphs)
		records := make([]SequenceLookupRecord, len(r.Records))
		copy(records, r.Records)
		out[i] = GPosSequenceRule{
			InputGlyphs: input,
			Records:     records,
		}
	}
	return out
}

func toGPosClassSequenceRules(in []GSubClassSequenceRule) []GPosClassSequenceRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]GPosClassSequenceRule, len(in))
	for i, r := range in {
		input := make([]uint16, len(r.InputClasses))
		copy(input, r.InputClasses)
		records := make([]SequenceLookupRecord, len(r.Records))
		copy(records, r.Records)
		out[i] = GPosClassSequenceRule{
			InputClasses: input,
			Records:      records,
		}
	}
	return out
}

func toGPosChainedSequenceRules(in []GSubChainedSequenceRule) []GPosChainedSequenceRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]GPosChainedSequenceRule, len(in))
	for i, r := range in {
		backtrack := make([]GlyphIndex, len(r.Backtrack))
		copy(backtrack, r.Backtrack)
		input := make([]GlyphIndex, len(r.Input))
		copy(input, r.Input)
		lookahead := make([]GlyphIndex, len(r.Lookahead))
		copy(lookahead, r.Lookahead)
		records := make([]SequenceLookupRecord, len(r.Records))
		copy(records, r.Records)
		out[i] = GPosChainedSequenceRule{
			Backtrack: backtrack,
			Input:     input,
			Lookahead: lookahead,
			Records:   records,
		}
	}
	return out
}

func toGPosChainedClassRules(in []GSubChainedClassRule) []GPosChainedClassRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]GPosChainedClassRule, len(in))
	for i, r := range in {
		backtrack := make([]uint16, len(r.Backtrack))
		copy(backtrack, r.Backtrack)
		input := make([]uint16, len(r.Input))
		copy(input, r.Input)
		lookahead := make([]uint16, len(r.Lookahead))
		copy(lookahead, r.Lookahead)
		records := make([]SequenceLookupRecord, len(r.Records))
		copy(records, r.Records)
		out[i] = GPosChainedClassRule{
			Backtrack: backtrack,
			Input:     input,
			Lookahead: lookahead,
			Records:   records,
		}
	}
	return out
}
