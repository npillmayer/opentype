package ot

import (
	"reflect"
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestLegacyLookupSubtableAdapterUnwrapsGSUBExtension(t *testing.T) {
	// GSUB extension subtable:
	// format=1, extensionLookupType=1 (single substitution), extensionOffset=8
	//
	// Wrapped GSUB type-1 format-1 payload:
	// format=1, coverageOffset=6, deltaGlyphID=3, coverage=[5]
	b := make([]byte, 20)
	putU16(b, 0, 1)
	putU16(b, 2, uint16(GSubLookupTypeSingle))
	b[4] = 0
	b[5] = 0
	b[6] = 0
	b[7] = 8
	putU16(b, 8, 1)
	putU16(b, 10, 6)
	putU16(b, 12, 3)
	copy(b[14:], coverageFmt1(5))

	legacy := parseLookupSubtable(binarySegm(b), GSubLookupTypeExtensionSubs)
	concrete := parseConcreteLookupNode(binarySegm(b), GSubLookupTypeExtensionSubs)
	if concrete == nil || concrete.Error() != nil {
		t.Fatalf("expected concrete extension node, err=%v", concrete.Error())
	}
	adapted := legacyLookupSubtableFromConcrete(concrete)

	assertLookupSubtableAdapterParity(t, "synthetic", "GSUB", 0, 0, legacy, adapted)
}

func TestLegacyLookupSubtableAdapterParityOnFonts(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	fonts := []string{"Calibri", "GentiumPlus-R"}
	checked := 0

	for _, fontName := range fonts {
		otf := parseFont(t, fontName)

		if gsub := otf.tables[T("GSUB")].Self().AsGSub(); gsub != nil && gsub.LookupGraph() != nil {
			for i := 0; i < gsub.LookupList.Len(); i++ {
				legacyLookup := gsub.LookupList.Navigate(i)
				concreteLookup := gsub.LookupGraph().Lookup(i)
				if concreteLookup == nil {
					continue
				}
				for j := 0; j < int(legacyLookup.SubTableCount); j++ {
					legacySub := legacyLookup.Subtable(j)
					concreteNode := concreteLookup.Subtable(j)
					if legacySub == nil || concreteNode == nil || concreteNode.Error() != nil {
						continue
					}
					adapted := legacyLookupSubtableFromConcrete(concreteNode)
					assertLookupSubtableAdapterParity(t, fontName, "GSUB", i, j, *legacySub, adapted)
					checked++
				}
			}
		}

		if gpos := otf.tables[T("GPOS")].Self().AsGPos(); gpos != nil && gpos.LookupGraph() != nil {
			for i := 0; i < gpos.LookupList.Len(); i++ {
				legacyLookup := gpos.LookupList.Navigate(i)
				concreteLookup := gpos.LookupGraph().Lookup(i)
				if concreteLookup == nil {
					continue
				}
				for j := 0; j < int(legacyLookup.SubTableCount); j++ {
					legacySub := legacyLookup.Subtable(j)
					concreteNode := concreteLookup.Subtable(j)
					if legacySub == nil || concreteNode == nil || concreteNode.Error() != nil {
						continue
					}
					adapted := legacyLookupSubtableFromConcrete(concreteNode)
					assertLookupSubtableAdapterParity(t, fontName, "GPOS", i, j, *legacySub, adapted)
					checked++
				}
			}
		}
	}

	if checked == 0 {
		t.Fatalf("expected adapter parity checks to cover at least one lookup subtable")
	}
}

func assertLookupSubtableAdapterParity(
	t *testing.T,
	fontName string,
	tableKind string,
	lookupIndex int,
	subtableIndex int,
	legacy LookupSubtable,
	adapted LookupSubtable,
) {
	t.Helper()

	prefix := tableKind + " " + fontName
	if legacy.LookupType != adapted.LookupType {
		t.Fatalf("%s lookup[%d]/subtable[%d]: lookup type mismatch legacy=%d adapted=%d",
			prefix, lookupIndex, subtableIndex, legacy.LookupType, adapted.LookupType)
	}
	if legacy.Format != adapted.Format {
		t.Fatalf("%s lookup[%d]/subtable[%d]: format mismatch legacy=%d adapted=%d",
			prefix, lookupIndex, subtableIndex, legacy.Format, adapted.Format)
	}
	if legacy.Coverage.CoverageFormat != adapted.Coverage.CoverageFormat {
		t.Fatalf("%s lookup[%d]/subtable[%d]: coverage format mismatch legacy=%d adapted=%d",
			prefix, lookupIndex, subtableIndex, legacy.Coverage.CoverageFormat, adapted.Coverage.CoverageFormat)
	}
	if legacy.Coverage.Count != adapted.Coverage.Count {
		t.Fatalf("%s lookup[%d]/subtable[%d]: coverage count mismatch legacy=%d adapted=%d",
			prefix, lookupIndex, subtableIndex, legacy.Coverage.Count, adapted.Coverage.Count)
	}

	legacyIndexSize := 0
	if legacy.Index != nil {
		legacyIndexSize = legacy.Index.Size()
	}
	adaptedIndexSize := 0
	if adapted.Index != nil {
		adaptedIndexSize = adapted.Index.Size()
	}
	if legacyIndexSize != adaptedIndexSize {
		t.Fatalf("%s lookup[%d]/subtable[%d]: index-size mismatch legacy=%d adapted=%d",
			prefix, lookupIndex, subtableIndex, legacyIndexSize, adaptedIndexSize)
	}
	for k := 0; k < legacyIndexSize; k++ {
		assertVarArrayEntryParity(t, prefix, lookupIndex, subtableIndex, k, legacy.Index, adapted.Index, false)
		assertVarArrayEntryParity(t, prefix, lookupIndex, subtableIndex, k, legacy.Index, adapted.Index, true)
	}

	if !reflect.DeepEqual(legacy.Support, adapted.Support) {
		lctx, lok := asSequenceContext(legacy.Support)
		actx, aok := asSequenceContext(adapted.Support)
		if !lok || !aok || !sequenceContextEqual(lctx, actx) {
			t.Fatalf("%s lookup[%d]/subtable[%d]: support mismatch legacy=%T adapted=%T",
				prefix, lookupIndex, subtableIndex, legacy.Support, adapted.Support)
		}
	}
	if !sequenceLookupRecordsEqual(legacy.LookupRecords, adapted.LookupRecords) {
		t.Fatalf("%s lookup[%d]/subtable[%d]: lookup-record mismatch", prefix, lookupIndex, subtableIndex)
	}
}

func assertVarArrayEntryParity(
	t *testing.T,
	prefix string,
	lookupIndex int,
	subtableIndex int,
	entryIndex int,
	legacy VarArray,
	adapted VarArray,
	deep bool,
) {
	t.Helper()

	legacyLoc, legacyErr := legacy.Get(entryIndex, deep)
	adaptedLoc, adaptedErr := adapted.Get(entryIndex, deep)

	if (legacyErr == nil) != (adaptedErr == nil) {
		t.Fatalf("%s lookup[%d]/subtable[%d]: index[%d] deep=%t error mismatch legacy=%v adapted=%v",
			prefix, lookupIndex, subtableIndex, entryIndex, deep, legacyErr, adaptedErr)
	}
	if legacyErr != nil {
		return
	}
	if !reflect.DeepEqual(legacyLoc.Bytes(), adaptedLoc.Bytes()) {
		t.Fatalf("%s lookup[%d]/subtable[%d]: index[%d] deep=%t payload mismatch",
			prefix, lookupIndex, subtableIndex, entryIndex, deep)
	}
}

func sequenceContextEqual(a, b SequenceContext) bool {
	if len(a.BacktrackCoverage) != len(b.BacktrackCoverage) ||
		len(a.InputCoverage) != len(b.InputCoverage) ||
		len(a.LookaheadCoverage) != len(b.LookaheadCoverage) ||
		len(a.ClassDefs) != len(b.ClassDefs) {
		return false
	}
	for i := range a.BacktrackCoverage {
		if a.BacktrackCoverage[i].CoverageFormat != b.BacktrackCoverage[i].CoverageFormat ||
			a.BacktrackCoverage[i].Count != b.BacktrackCoverage[i].Count {
			return false
		}
	}
	for i := range a.InputCoverage {
		if a.InputCoverage[i].CoverageFormat != b.InputCoverage[i].CoverageFormat ||
			a.InputCoverage[i].Count != b.InputCoverage[i].Count {
			return false
		}
	}
	for i := range a.LookaheadCoverage {
		if a.LookaheadCoverage[i].CoverageFormat != b.LookaheadCoverage[i].CoverageFormat ||
			a.LookaheadCoverage[i].Count != b.LookaheadCoverage[i].Count {
			return false
		}
	}
	for i := range a.ClassDefs {
		for _, glyph := range []GlyphIndex{0, 1, 10, 100, 500} {
			if a.ClassDefs[i].Lookup(glyph) != b.ClassDefs[i].Lookup(glyph) {
				return false
			}
		}
	}
	return true
}
