package ot

// legacyLookupSubtableFromConcrete projects a concrete lookup node onto the
// transitional legacy LookupSubtable model.
//
// This adapter is intentionally internal and transitional. It allows legacy
// API surfaces to be kept stable while concrete lookup parsing becomes the
// single source of truth.
func legacyLookupSubtableFromConcrete(node *LookupNode) LookupSubtable {
	if node == nil {
		return LookupSubtable{}
	}
	// Legacy model exposes extension subtables as the wrapped effective type.
	if p := node.GSubPayload(); p != nil && p.ExtensionFmt1 != nil && p.ExtensionFmt1.Resolved != nil {
		return legacyLookupSubtableFromConcrete(p.ExtensionFmt1.Resolved)
	}
	if p := node.GPosPayload(); p != nil && p.ExtensionFmt1 != nil && p.ExtensionFmt1.Resolved != nil {
		return legacyLookupSubtableFromConcrete(p.ExtensionFmt1.Resolved)
	}

	legacyType := node.LookupType
	if IsGPosLookupType(legacyType) {
		legacyType = GPosLookupType(legacyType)
	}
	sub := LookupSubtable{
		LookupType: legacyType,
		Format:     node.Format,
		Coverage:   node.Coverage,
	}
	if IsGPosLookupType(node.LookupType) {
		adaptLegacyGPosLookupSubtable(node, &sub)
	} else {
		adaptLegacyGSubLookupSubtable(node, &sub)
	}
	return sub
}

func adaptLegacyGSubLookupSubtable(node *LookupNode, sub *LookupSubtable) {
	if node == nil || sub == nil || node.GSubPayload() == nil {
		return
	}
	switch sub.LookupType {
	case GSubLookupTypeSingle:
		switch sub.Format {
		case 1:
			if p := node.GSubPayload().SingleFmt1; p != nil {
				sub.Support = p.DeltaGlyphID
			}
		case 2:
			sub.Index = parseVarArray16(node.raw, 4, 2, 1, "LookupSubtableGSub1")
		}
	case GSubLookupTypeMultiple, GSubLookupTypeAlternate, GSubLookupTypeLigature:
		indirections := 2
		if sub.LookupType == GSubLookupTypeMultiple || sub.LookupType == GSubLookupTypeAlternate {
			indirections = 1
		}
		sub.Index = parseVarArray16(node.raw, 4, 2, indirections, "LookupSubtableGSub2/3/4")
	case GSubLookupTypeContext:
		switch sub.Format {
		case 1:
			sub.Index = parseVarArray16(node.raw, 4, 2, 2, "LookupSubtableGSub5-1")
		case 2:
			sub.Index = parseVarArray16(node.raw, 6, 2, 2, "LookupSubtableGSub5-2")
			if p := node.GSubPayload().ContextFmt2; p != nil {
				sub.Support = &SequenceContext{
					ClassDefs: []ClassDefinitions{p.ClassDef},
				}
			}
		case 3:
			sub.Index = parseVarArray16(node.raw, 4, 4, 2, "LookupSubtableGSub5-3")
			if p := node.GSubPayload().ContextFmt3; p != nil {
				seqctx := SequenceContext{
					InputCoverage: copyCoverageSlice(p.InputCoverages),
				}
				sub.Support = seqctx // keep legacy value semantics for fmt3
			}
		}
	case GSubLookupTypeChainingContext:
		switch sub.Format {
		case 1:
			sub.Index = parseVarArray16(node.raw, 4, 2, 2, "LookupSubtableGSub6-1")
		case 2:
			sub.Index = parseVarArray16(node.raw, 10, 2, 2, "LookupSubtableGSub6-2")
			if p := node.GSubPayload().ChainingContextFmt2; p != nil {
				sub.Support = &SequenceContext{
					ClassDefs: []ClassDefinitions{
						p.BacktrackClassDef,
						p.InputClassDef,
						p.LookaheadClassDef,
					},
				}
			}
		case 3:
			if p := node.GSubPayload().ChainingContextFmt3; p != nil {
				sub.Support = &SequenceContext{
					BacktrackCoverage: copyCoverageSlice(p.BacktrackCoverages),
					InputCoverage:     copyCoverageSlice(p.InputCoverages),
					LookaheadCoverage: copyCoverageSlice(p.LookaheadCoverages),
				}
				sub.LookupRecords = copySequenceLookupRecords(p.Records)
			}
		}
	case GSubLookupTypeReverseChaining:
		if p := node.GSubPayload().ReverseChainingFmt1; p != nil {
			sub.Support = ReverseChainingSubst{
				BacktrackCoverage:  copyCoverageSlice(p.BacktrackCoverages),
				LookaheadCoverage:  copyCoverageSlice(p.LookaheadCoverages),
				SubstituteGlyphIDs: copyGlyphIndices(p.SubstituteGlyphIDs),
			}
		}
	}
}

func adaptLegacyGPosLookupSubtable(node *LookupNode, sub *LookupSubtable) {
	if node == nil || sub == nil || node.GPosPayload() == nil {
		return
	}
	switch sub.LookupType {
	case GPosLookupTypeSingle:
		switch sub.Format {
		case 1:
			if p := node.GPosPayload().SingleFmt1; p != nil {
				sub.Support = struct {
					Format ValueFormat
					Record ValueRecord
				}{p.ValueFormat, p.Value}
			}
		case 2:
			if p := node.GPosPayload().SingleFmt2; p != nil {
				records := make([]ValueRecord, len(p.Values))
				copy(records, p.Values)
				sub.Support = struct {
					Format  ValueFormat
					Records []ValueRecord
				}{p.ValueFormat, records}
			}
		}
	case GPosLookupTypePair:
		switch sub.Format {
		case 1:
			sub.Index = parseVarArray16(node.raw, 8, 2, 2, "LookupSubtableGPos2-1")
			if p := node.GPosPayload().PairFmt1; p != nil {
				sub.Support = [2]ValueFormat{p.ValueFormat1, p.ValueFormat2}
			}
		case 2:
			if p := node.GPosPayload().PairFmt2; p != nil {
				classRecords := make([][]struct {
					Value1 ValueRecord
					Value2 ValueRecord
				}, len(p.ClassRecords))
				for i := range p.ClassRecords {
					row := make([]struct {
						Value1 ValueRecord
						Value2 ValueRecord
					}, len(p.ClassRecords[i]))
					for j := range p.ClassRecords[i] {
						row[j] = struct {
							Value1 ValueRecord
							Value2 ValueRecord
						}{
							Value1: p.ClassRecords[i][j].Value1,
							Value2: p.ClassRecords[i][j].Value2,
						}
					}
					classRecords[i] = row
				}
				sub.Support = struct {
					ValueFormat1 ValueFormat
					ValueFormat2 ValueFormat
					ClassDef1    ClassDefinitions
					ClassDef2    ClassDefinitions
					Class1Count  uint16
					Class2Count  uint16
					ClassRecords [][]struct {
						Value1 ValueRecord
						Value2 ValueRecord
					}
				}{
					ValueFormat1: p.ValueFormat1,
					ValueFormat2: p.ValueFormat2,
					ClassDef1:    p.ClassDef1,
					ClassDef2:    p.ClassDef2,
					Class1Count:  p.Class1Count,
					Class2Count:  p.Class2Count,
					ClassRecords: classRecords,
				}
			}
		}
	case GPosLookupTypeCursive:
		sub.Index = parseVarArray16(node.raw, 6, 4, 1, "LookupSubtableGPos3")
		if p := node.GPosPayload().CursiveFmt1; p != nil {
			sub.Support = struct {
				EntryExitCount uint16
			}{
				EntryExitCount: uint16(len(p.Entries)),
			}
		}
	case GPosLookupTypeMarkToBase:
		if len(node.raw) >= 12 {
			markClassCount := node.raw.U16(6)
			markArrayOffset := node.raw.U16(8)
			baseArrayOffset := node.raw.U16(10)
			var baseCoverage Coverage
			if cov, err := parseCoverageAt(node.raw, 4); err == nil {
				baseCoverage = cov
			}
			var markArray MarkArray
			if markArrayOffset > 0 && int(markArrayOffset) < len(node.raw) {
				markArray = parseMarkArray(node.raw[markArrayOffset:])
			}
			var baseArray []BaseRecord
			if baseArrayOffset > 0 && int(baseArrayOffset) < len(node.raw) {
				baseArray = parseBaseArray(node.raw[baseArrayOffset:], int(markClassCount))
			}
			sub.Support = struct {
				BaseCoverage   Coverage
				MarkClassCount uint16
				MarkArray      MarkArray
				BaseArray      []BaseRecord
			}{baseCoverage, markClassCount, markArray, baseArray}
		}
	case GPosLookupTypeMarkToLigature:
		if len(node.raw) >= 12 {
			markClassCount := node.raw.U16(6)
			markArrayOffset := node.raw.U16(8)
			ligatureArrayOffset := node.raw.U16(10)
			var ligatureCoverage Coverage
			if cov, err := parseCoverageAt(node.raw, 4); err == nil {
				ligatureCoverage = cov
			}
			var markArray MarkArray
			if markArrayOffset > 0 && int(markArrayOffset) < len(node.raw) {
				markArray = parseMarkArray(node.raw[markArrayOffset:])
			}
			var ligatureArray []LigatureAttach
			if ligatureArrayOffset > 0 && int(ligatureArrayOffset) < len(node.raw) {
				ligatureArray = parseLigatureArray(node.raw[ligatureArrayOffset:], int(markClassCount))
			}
			sub.Support = struct {
				LigatureCoverage Coverage
				MarkClassCount   uint16
				MarkArray        MarkArray
				LigatureArray    []LigatureAttach
			}{ligatureCoverage, markClassCount, markArray, ligatureArray}
		}
	case GPosLookupTypeMarkToMark:
		if len(node.raw) >= 12 {
			markClassCount := node.raw.U16(6)
			mark1ArrayOffset := node.raw.U16(8)
			mark2ArrayOffset := node.raw.U16(10)
			var mark2Coverage Coverage
			if cov, err := parseCoverageAt(node.raw, 4); err == nil {
				mark2Coverage = cov
			}
			var mark1Array MarkArray
			if mark1ArrayOffset > 0 && int(mark1ArrayOffset) < len(node.raw) {
				mark1Array = parseMarkArray(node.raw[mark1ArrayOffset:])
			}
			var mark2Array []BaseRecord
			if mark2ArrayOffset > 0 && int(mark2ArrayOffset) < len(node.raw) {
				mark2Array = parseBaseArray(node.raw[mark2ArrayOffset:], int(markClassCount))
			}
			sub.Support = struct {
				Mark2Coverage  Coverage
				MarkClassCount uint16
				Mark1Array     MarkArray
				Mark2Array     []BaseRecord
			}{mark2Coverage, markClassCount, mark1Array, mark2Array}
		}
	case GPosLookupTypeContextPos:
		switch sub.Format {
		case 1:
			sub.Index = parseVarArray16(node.raw, 4, 2, 2, "LookupSubtableGPos7-1")
		case 2:
			sub.Index = parseVarArray16(node.raw, 6, 2, 2, "LookupSubtableGPos7-2")
			if p := node.GPosPayload().ContextFmt2; p != nil {
				sub.Support = &SequenceContext{
					ClassDefs: []ClassDefinitions{p.ClassDef},
				}
			}
		case 3:
			sub.Index = parseVarArray16(node.raw, 4, 4, 2, "LookupSubtableGPos7-3")
			if p := node.GPosPayload().ContextFmt3; p != nil {
				seqctx := SequenceContext{
					InputCoverage: copyCoverageSlice(p.InputCoverages),
				}
				sub.Support = seqctx // keep legacy value semantics for fmt3
			}
		}
	case GPosLookupTypeChainedContextPos:
		switch sub.Format {
		case 1:
			sub.Index = parseVarArray16(node.raw, 4, 2, 2, "LookupSubtableGPos8-1")
		case 2:
			sub.Index = parseVarArray16(node.raw, 10, 2, 2, "LookupSubtableGPos8-2")
			if p := node.GPosPayload().ChainingContextFmt2; p != nil {
				sub.Support = &SequenceContext{
					ClassDefs: []ClassDefinitions{
						p.BacktrackClassDef,
						p.InputClassDef,
						p.LookaheadClassDef,
					},
				}
			}
		case 3:
			if p := node.GPosPayload().ChainingContextFmt3; p != nil {
				sub.Support = &SequenceContext{
					BacktrackCoverage: copyCoverageSlice(p.BacktrackCoverages),
					InputCoverage:     copyCoverageSlice(p.InputCoverages),
					LookaheadCoverage: copyCoverageSlice(p.LookaheadCoverages),
				}
				sub.LookupRecords = copySequenceLookupRecords(p.Records)
			}
		}
	}
}

func copySequenceLookupRecords(in []SequenceLookupRecord) []SequenceLookupRecord {
	if len(in) == 0 {
		return nil
	}
	out := make([]SequenceLookupRecord, len(in))
	copy(out, in)
	return out
}

func copyCoverageSlice(in []Coverage) []Coverage {
	if len(in) == 0 {
		return nil
	}
	out := make([]Coverage, len(in))
	copy(out, in)
	return out
}

func copyGlyphIndices(in []GlyphIndex) []GlyphIndex {
	if len(in) == 0 {
		return nil
	}
	out := make([]GlyphIndex, len(in))
	copy(out, in)
	return out
}
