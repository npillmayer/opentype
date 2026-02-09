package ot

import (
	"fmt"
	"sync"
)

func parseConcreteLookupListGraph(lookupList binarySegm, isGPos bool) *LookupListGraph {
	lookupArray, err := parseArray16(lookupList, 0, "LookupList", "Lookup")
	lg := &LookupListGraph{
		isGPos: isGPos,
		raw:    lookupList,
		err:    err,
	}
	if err != nil {
		return lg
	}
	lg.lookupOffsets = make([]uint16, lookupArray.Len())
	lg.lookupTables = make([]*LookupTable, lookupArray.Len())
	lg.lookupOnce = make([]sync.Once, lookupArray.Len())
	for i := 0; i < lookupArray.Len(); i++ {
		off := lookupArray.Get(i).U16(0)
		lg.lookupOffsets[i] = off
		if off == 0 || int(off) >= len(lookupList) {
			if lg.err == nil {
				lg.err = fmt.Errorf("lookup record %d has invalid offset %d (size %d)", i, off, len(lookupList))
			}
			continue
		}
		if verr := validateConcreteLookupTable(lookupList[off:]); verr != nil && lg.err == nil {
			lg.err = verr
		}
	}
	return lg
}

func validateConcreteLookupTable(b binarySegm) error {
	if len(b) < 6 {
		return errBufferBounds
	}
	_, err := parseArray16(b, 4, "Lookup", "Lookup-Subtables")
	return err
}

func parseConcreteLookupTable(b binarySegm, isGPos bool) *LookupTable {
	lt := &LookupTable{raw: b}
	if len(b) < 6 {
		lt.err = errBufferBounds
		return lt
	}
	lookupType := LayoutTableLookupType(b.U16(0))
	if isGPos {
		lt.Type = MaskGPosLookupType(lookupType)
	} else {
		lt.Type = lookupType
	}
	lt.Flag = LayoutTableLookupFlag(b.U16(2))
	lt.SubTableCount = b.U16(4)
	subtables, err := parseArray16(b, 4, "Lookup", "Lookup-Subtables")
	if err != nil {
		lt.err = err
		return lt
	}
	lt.subtableOffsets = make([]uint16, subtables.Len())
	lt.subtables = make([]*LookupNode, subtables.Len())
	lt.subtableOnce = make([]sync.Once, subtables.Len())
	for i := 0; i < subtables.Len(); i++ {
		off := subtables.Get(i).U16(0)
		lt.subtableOffsets[i] = off
		if off == 0 || int(off) >= len(b) {
			if lt.err == nil {
				lt.err = fmt.Errorf("lookup subtable record %d has invalid offset %d (size %d)", i, off, len(b))
			}
		}
	}
	if len(b) >= 4+subtables.Size()+2 {
		lt.markFilteringSet = b.U16(4 + subtables.Size())
	}
	return lt
}

func parseConcreteLookupNode(b binarySegm, lookupType LayoutTableLookupType) *LookupNode {
	return parseConcreteLookupNodeWithDepth(b, lookupType, 0)
}

func parseConcreteLookupNodeWithDepth(b binarySegm, lookupType LayoutTableLookupType, depth int) *LookupNode {
	node := &LookupNode{
		LookupType: lookupType,
		raw:        b,
	}
	if len(b) < 4 {
		node.err = errBufferBounds
		return node
	}
	node.Format = b.U16(0)
	if IsGPosLookupType(lookupType) {
		gposType := GPosLookupType(lookupType)
		node.GPos = parseConcreteGPosPayloadScaffold(gposType, node.Format)
		parseConcreteGPosPayload(node, depth)
	} else {
		node.GSub = parseConcreteGSubPayloadScaffold(lookupType, node.Format)
		parseConcreteGSubPayload(node, depth)
	}
	return node
}

func parseConcreteGSubPayloadScaffold(lookupType LayoutTableLookupType, format uint16) *GSubLookupPayload {
	payload := &GSubLookupPayload{}
	switch lookupType {
	case GSubLookupTypeSingle:
		if format == 1 {
			payload.SingleFmt1 = &GSubSingleFmt1Payload{}
		} else if format == 2 {
			payload.SingleFmt2 = &GSubSingleFmt2Payload{}
		}
	case GSubLookupTypeMultiple:
		if format == 1 {
			payload.MultipleFmt1 = &GSubMultipleFmt1Payload{}
		}
	case GSubLookupTypeAlternate:
		if format == 1 {
			payload.AlternateFmt1 = &GSubAlternateFmt1Payload{}
		}
	case GSubLookupTypeLigature:
		if format == 1 {
			payload.LigatureFmt1 = &GSubLigatureFmt1Payload{}
		}
	case GSubLookupTypeContext:
		switch format {
		case 1:
			payload.ContextFmt1 = &GSubContextFmt1Payload{}
		case 2:
			payload.ContextFmt2 = &GSubContextFmt2Payload{}
		case 3:
			payload.ContextFmt3 = &GSubContextFmt3Payload{}
		}
	case GSubLookupTypeChainingContext:
		switch format {
		case 1:
			payload.ChainingContextFmt1 = &GSubChainingContextFmt1Payload{}
		case 2:
			payload.ChainingContextFmt2 = &GSubChainingContextFmt2Payload{}
		case 3:
			payload.ChainingContextFmt3 = &GSubChainingContextFmt3Payload{}
		}
	case GSubLookupTypeExtensionSubs:
		if format == 1 {
			payload.ExtensionFmt1 = &GSubExtensionFmt1Payload{}
		}
	case GSubLookupTypeReverseChaining:
		if format == 1 {
			payload.ReverseChainingFmt1 = &GSubReverseChainingFmt1Payload{}
		}
	}
	return payload
}

func parseConcreteGPosPayloadScaffold(lookupType LayoutTableLookupType, format uint16) *GPosLookupPayload {
	payload := &GPosLookupPayload{}
	switch lookupType {
	case GPosLookupTypeSingle:
		if format == 1 {
			payload.SingleFmt1 = &GPosSingleFmt1Payload{}
		} else if format == 2 {
			payload.SingleFmt2 = &GPosSingleFmt2Payload{}
		}
	case GPosLookupTypePair:
		if format == 1 {
			payload.PairFmt1 = &GPosPairFmt1Payload{}
		} else if format == 2 {
			payload.PairFmt2 = &GPosPairFmt2Payload{}
		}
	case GPosLookupTypeCursive:
		if format == 1 {
			payload.CursiveFmt1 = &GPosCursiveFmt1Payload{}
		}
	case GPosLookupTypeMarkToBase:
		if format == 1 {
			payload.MarkToBaseFmt1 = &GPosMarkToBaseFmt1Payload{}
		}
	case GPosLookupTypeMarkToLigature:
		if format == 1 {
			payload.MarkToLigatureFmt1 = &GPosMarkToLigatureFmt1Payload{}
		}
	case GPosLookupTypeMarkToMark:
		if format == 1 {
			payload.MarkToMarkFmt1 = &GPosMarkToMarkFmt1Payload{}
		}
	case GPosLookupTypeContextPos:
		switch format {
		case 1:
			payload.ContextFmt1 = &GPosContextFmt1Payload{}
		case 2:
			payload.ContextFmt2 = &GPosContextFmt2Payload{}
		case 3:
			payload.ContextFmt3 = &GPosContextFmt3Payload{}
		}
	case GPosLookupTypeChainedContextPos:
		switch format {
		case 1:
			payload.ChainingContextFmt1 = &GPosChainingContextFmt1Payload{}
		case 2:
			payload.ChainingContextFmt2 = &GPosChainingContextFmt2Payload{}
		case 3:
			payload.ChainingContextFmt3 = &GPosChainingContextFmt3Payload{}
		}
	case GPosLookupTypeExtensionPos:
		if format == 1 {
			payload.ExtensionFmt1 = &GPosExtensionFmt1Payload{}
		}
	}
	return payload
}
