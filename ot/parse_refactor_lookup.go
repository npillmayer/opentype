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
	node := &LookupNode{
		LookupType: lookupType,
		raw:        b,
	}
	if len(b) < 4 {
		node.err = errBufferBounds
		return node
	}
	node.Format = b.U16(0)
	return node
}
