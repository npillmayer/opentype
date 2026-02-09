package ot

import (
	"iter"
	"sync"
)

// LookupListGraph is the concrete, typed lookup-list graph for GSUB/GPOS.
// It is parsed in parallel with the legacy LookupList during transition.
type LookupListGraph struct {
	lookupOffsets []uint16
	lookupTables  []*LookupTable
	lookupOnce    []sync.Once
	isGPos        bool

	raw binarySegm
	err error
}

// LookupTable is the concrete, typed lookup table model.
type LookupTable struct {
	Type             LayoutTableLookupType
	Flag             LayoutTableLookupFlag
	SubTableCount    uint16
	markFilteringSet uint16

	subtableOffsets []uint16
	subtables       []*LookupNode
	subtableOnce    []sync.Once

	raw binarySegm
	err error
}

// LookupNode is a concrete lookup-subtable node with shared metadata.
// Typed GSUB/GPOS payload fields will be added in later slices.
type LookupNode struct {
	LookupType LayoutTableLookupType
	Format     uint16
	Coverage   Coverage

	raw binarySegm
	err error
}

// Len returns number of lookups.
func (lg *LookupListGraph) Len() int {
	if lg == nil {
		return 0
	}
	return len(lg.lookupOffsets)
}

// Lookup returns a concrete lookup by index, lazily instantiated.
func (lg *LookupListGraph) Lookup(i int) *LookupTable {
	if lg == nil || i < 0 || i >= len(lg.lookupOffsets) {
		return nil
	}
	lg.lookupOnce[i].Do(func() {
		off := int(lg.lookupOffsets[i])
		if off <= 0 || off >= len(lg.raw) {
			lg.lookupTables[i] = &LookupTable{err: errBufferBounds}
			return
		}
		lg.lookupTables[i] = parseConcreteLookupTable(lg.raw[off:], lg.isGPos)
	})
	return lg.lookupTables[i]
}

// Range iterates lookups in declaration order.
func (lg *LookupListGraph) Range() iter.Seq2[int, *LookupTable] {
	return func(yield func(int, *LookupTable) bool) {
		if lg == nil {
			return
		}
		for i := range len(lg.lookupOffsets) {
			if !yield(i, lg.Lookup(i)) {
				return
			}
		}
	}
}

// Error returns an accumulated parse/validation error for the lookup list graph.
func (lg *LookupListGraph) Error() error {
	if lg == nil {
		return nil
	}
	return lg.err
}

// MarkFilteringSet returns the optional mark-filtering-set index.
func (lt *LookupTable) MarkFilteringSet() uint16 {
	if lt == nil {
		return 0
	}
	return lt.markFilteringSet
}

// Subtable returns a concrete lookup-subtable node by index, lazily instantiated.
func (lt *LookupTable) Subtable(i int) *LookupNode {
	if lt == nil || i < 0 || i >= len(lt.subtableOffsets) {
		return nil
	}
	lt.subtableOnce[i].Do(func() {
		off := int(lt.subtableOffsets[i])
		if off <= 0 || off >= len(lt.raw) {
			lt.subtables[i] = &LookupNode{err: errBufferBounds}
			return
		}
		lt.subtables[i] = parseConcreteLookupNode(lt.raw[off:], lt.Type)
	})
	return lt.subtables[i]
}

// Range iterates lookup-subtables in declaration order.
func (lt *LookupTable) Range() iter.Seq2[int, *LookupNode] {
	return func(yield func(int, *LookupNode) bool) {
		if lt == nil {
			return
		}
		for i := range len(lt.subtableOffsets) {
			if !yield(i, lt.Subtable(i)) {
				return
			}
		}
	}
}

// Error returns an accumulated parse/validation error for this lookup table.
func (lt *LookupTable) Error() error {
	if lt == nil {
		return nil
	}
	return lt.err
}

// Error returns an accumulated parse/validation error for this lookup node.
func (ln *LookupNode) Error() error {
	if ln == nil {
		return nil
	}
	return ln.err
}
