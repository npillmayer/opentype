package ot

import (
	"reflect"
	"sync"
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func asSequenceContext(v any) (SequenceContext, bool) {
	switch ctx := v.(type) {
	case SequenceContext:
		return ctx, true
	case *SequenceContext:
		if ctx == nil {
			return SequenceContext{}, false
		}
		return *ctx, true
	default:
		return SequenceContext{}, false
	}
}

func sequenceLookupRecordsEqual(a, b []SequenceLookupRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].SequenceIndex != b[i].SequenceIndex || a[i].LookupListIndex != b[i].LookupListIndex {
			return false
		}
	}
	return true
}

func effectiveConcreteLookupNode(n *LookupNode) *LookupNode {
	if n == nil {
		return nil
	}
	if p := n.GSubPayload(); p != nil && p.ExtensionFmt1 != nil && p.ExtensionFmt1.Resolved != nil {
		return p.ExtensionFmt1.Resolved
	}
	if p := n.GPosPayload(); p != nil && p.ExtensionFmt1 != nil && p.ExtensionFmt1.Resolved != nil {
		return p.ExtensionFmt1.Resolved
	}
	return n
}

func supportStructField(v any, name string) (reflect.Value, bool) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return reflect.Value{}, false
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	f := rv.FieldByName(name)
	if !f.IsValid() {
		return reflect.Value{}, false
	}
	return f, true
}

func TestConcreteExtensionParityGSUB(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otf := parseFont(t, "Calibri")
	gsub := otf.tables[T("GSUB")].Self().AsGSub()
	if gsub == nil || gsub.LookupGraph() == nil {
		t.Fatalf("expected GSUB lookup graph")
	}

	found := 0
	for i := 0; i < gsub.LookupList.Len(); i++ {
		legacyLookup := gsub.LookupList.Navigate(i)
		concreteLookup := gsub.LookupGraph().Lookup(i)
		if concreteLookup == nil {
			continue
		}
		for j := 0; j < int(legacyLookup.SubTableCount); j++ {
			legacySub := legacyLookup.Subtable(j)
			concreteSub := concreteLookup.Subtable(j)
			if legacySub == nil || concreteSub == nil {
				continue
			}
			payload := concreteSub.GSubPayload()
			if payload == nil || payload.ExtensionFmt1 == nil {
				continue
			}
			ext := payload.ExtensionFmt1
			if ext.Resolved == nil {
				t.Fatalf("GSUB lookup[%d]/subtable[%d]: expected resolved extension node", i, j)
			}
			if ext.ResolvedType != legacySub.LookupType {
				t.Fatalf("GSUB lookup[%d]/subtable[%d]: resolved type mismatch legacy=%d concrete=%d",
					i, j, legacySub.LookupType, ext.ResolvedType)
			}
			if ext.Resolved.LookupType != legacySub.LookupType {
				t.Fatalf("GSUB lookup[%d]/subtable[%d]: resolved node type mismatch legacy=%d concrete=%d",
					i, j, legacySub.LookupType, ext.Resolved.LookupType)
			}
			if concreteSub.Coverage.Count != legacySub.Coverage.Count {
				t.Fatalf("GSUB lookup[%d]/subtable[%d]: coverage count mismatch legacy=%d concrete=%d",
					i, j, legacySub.Coverage.Count, concreteSub.Coverage.Count)
			}
			found++
		}
	}
	if found == 0 {
		t.Fatalf("expected at least one GSUB extension subtable for parity check")
	}
}

func TestConcreteExtensionParityGPOS(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otf := parseFont(t, "Calibri")
	gpos := otf.tables[T("GPOS")].Self().AsGPos()
	if gpos == nil || gpos.LookupGraph() == nil {
		t.Fatalf("expected GPOS lookup graph")
	}

	found := 0
	for i := 0; i < gpos.LookupList.Len(); i++ {
		legacyLookup := gpos.LookupList.Navigate(i)
		concreteLookup := gpos.LookupGraph().Lookup(i)
		if concreteLookup == nil {
			continue
		}
		for j := 0; j < int(legacyLookup.SubTableCount); j++ {
			legacySub := legacyLookup.Subtable(j)
			concreteSub := concreteLookup.Subtable(j)
			if legacySub == nil || concreteSub == nil {
				continue
			}
			payload := concreteSub.GPosPayload()
			if payload == nil || payload.ExtensionFmt1 == nil {
				continue
			}
			ext := payload.ExtensionFmt1
			if ext.Resolved == nil {
				t.Fatalf("GPOS lookup[%d]/subtable[%d]: expected resolved extension node", i, j)
			}
			expectedResolvedType := MaskGPosLookupType(legacySub.LookupType)
			if ext.ResolvedType != expectedResolvedType {
				t.Fatalf("GPOS lookup[%d]/subtable[%d]: resolved type mismatch legacy=%d concrete=%d",
					i, j, expectedResolvedType, ext.ResolvedType)
			}
			if ext.Resolved.LookupType != expectedResolvedType {
				t.Fatalf("GPOS lookup[%d]/subtable[%d]: resolved node type mismatch legacy=%d concrete=%d",
					i, j, expectedResolvedType, ext.Resolved.LookupType)
			}
			if concreteSub.Coverage.Count != legacySub.Coverage.Count {
				t.Fatalf("GPOS lookup[%d]/subtable[%d]: coverage count mismatch legacy=%d concrete=%d",
					i, j, legacySub.Coverage.Count, concreteSub.Coverage.Count)
			}
			found++
		}
	}
	if found == 0 {
		t.Fatalf("expected at least one GPOS extension subtable for parity check")
	}
}

func TestConcretePayloadParityGPOS(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	fonts := []string{"Calibri", "GentiumPlus-R"}
	checked := 0

	for _, fontName := range fonts {
		otf := parseFont(t, fontName)
		gpos := otf.tables[T("GPOS")].Self().AsGPos()
		if gpos == nil || gpos.LookupGraph() == nil {
			continue
		}

		for i := 0; i < gpos.LookupList.Len(); i++ {
			legacyLookup := gpos.LookupList.Navigate(i)
			concreteLookup := gpos.LookupGraph().Lookup(i)
			if concreteLookup == nil {
				continue
			}
			for j := 0; j < int(legacyLookup.SubTableCount); j++ {
				legacySub := legacyLookup.Subtable(j)
				concreteSub := concreteLookup.Subtable(j)
				if legacySub == nil || concreteSub == nil {
					continue
				}
				effective := effectiveConcreteLookupNode(concreteSub)
				if effective == nil || effective.GPosPayload() == nil {
					continue
				}
				if effective.Error() != nil {
					// Legacy path can be more permissive for malformed internals.
					// Parity checks focus on cleanly parsed effective payloads.
					continue
				}
				lookupType := legacySub.LookupType
				switch lookupType {
				case GPosLookupTypeSingle:
					switch legacySub.Format {
					case 1:
						payload := effective.GPosPayload().SingleFmt1
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected single fmt1 payload", fontName, i, j)
						}
						fv, ok := supportStructField(legacySub.Support, "Format")
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: missing legacy support format", fontName, i, j)
						}
						rv, ok := supportStructField(legacySub.Support, "Record")
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: missing legacy support value-record", fontName, i, j)
						}
						if payload.ValueFormat != fv.Interface().(ValueFormat) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: value-format mismatch legacy=0x%x concrete=0x%x",
								fontName, i, j, fv.Interface().(ValueFormat), payload.ValueFormat)
						}
						if payload.Value != rv.Interface().(ValueRecord) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: value-record mismatch", fontName, i, j)
						}
						checked++
					case 2:
						payload := effective.GPosPayload().SingleFmt2
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected single fmt2 payload", fontName, i, j)
						}
						fv, ok := supportStructField(legacySub.Support, "Format")
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: missing legacy support format", fontName, i, j)
						}
						rv, ok := supportStructField(legacySub.Support, "Records")
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: missing legacy support records", fontName, i, j)
						}
						legacyValues := rv.Interface().([]ValueRecord)
						if payload.ValueFormat != fv.Interface().(ValueFormat) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: value-format mismatch legacy=0x%x concrete=0x%x",
								fontName, i, j, fv.Interface().(ValueFormat), payload.ValueFormat)
						}
						if len(payload.Values) != len(legacyValues) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: value-record count mismatch legacy=%d concrete=%d",
								fontName, i, j, len(legacyValues), len(payload.Values))
						}
						if len(legacyValues) > 0 && payload.Values[0] != legacyValues[0] {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: first value-record mismatch", fontName, i, j)
						}
						checked++
					}

				case GPosLookupTypePair:
					switch legacySub.Format {
					case 1:
						payload := effective.GPosPayload().PairFmt1
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected pair fmt1 payload", fontName, i, j)
						}
						formats, ok := legacySub.Support.([2]ValueFormat)
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected legacy support [2]ValueFormat", fontName, i, j)
						}
						if payload.ValueFormat1 != formats[0] || payload.ValueFormat2 != formats[1] {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: pair value-format mismatch", fontName, i, j)
						}
						if legacySub.Index != nil && len(payload.PairSets) > 0 {
							legacyPairs, err := pairSetRecords(legacySub, 0, formats[0], formats[1])
							if err == nil {
								if len(payload.PairSets[0]) != len(legacyPairs) {
									t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: pair-set record count mismatch legacy=%d concrete=%d",
										fontName, i, j, len(legacyPairs), len(payload.PairSets[0]))
								}
								if len(legacyPairs) > 0 {
									if payload.PairSets[0][0] != legacyPairs[0] {
										t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: first pair record mismatch", fontName, i, j)
									}
								}
							}
						}
						checked++
					case 2:
						payload := effective.GPosPayload().PairFmt2
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected pair fmt2 payload", fontName, i, j)
						}
						fv1, ok1 := supportStructField(legacySub.Support, "ValueFormat1")
						fv2, ok2 := supportStructField(legacySub.Support, "ValueFormat2")
						cd1, ok3 := supportStructField(legacySub.Support, "ClassDef1")
						cd2, ok4 := supportStructField(legacySub.Support, "ClassDef2")
						c1, ok5 := supportStructField(legacySub.Support, "Class1Count")
						c2, ok6 := supportStructField(legacySub.Support, "Class2Count")
						cr, ok7 := supportStructField(legacySub.Support, "ClassRecords")
						if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: incomplete legacy support for pair fmt2", fontName, i, j)
						}
						if payload.ValueFormat1 != fv1.Interface().(ValueFormat) || payload.ValueFormat2 != fv2.Interface().(ValueFormat) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: pair fmt2 value-format mismatch", fontName, i, j)
						}
						if payload.ClassDef1.format != cd1.Interface().(ClassDefinitions).format ||
							payload.ClassDef2.format != cd2.Interface().(ClassDefinitions).format {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: class-def format mismatch", fontName, i, j)
						}
						if payload.Class1Count != c1.Interface().(uint16) || payload.Class2Count != c2.Interface().(uint16) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: class-count mismatch", fontName, i, j)
						}
						rows := cr.Len()
						if len(payload.ClassRecords) != rows {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: class-record row count mismatch legacy=%d concrete=%d",
								fontName, i, j, rows, len(payload.ClassRecords))
						}
						if rows > 0 {
							cols := cr.Index(0).Len()
							if len(payload.ClassRecords[0]) != cols {
								t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: class-record column count mismatch legacy=%d concrete=%d",
									fontName, i, j, cols, len(payload.ClassRecords[0]))
							}
						}
						checked++
					}

				case GPosLookupTypeCursive:
					if legacySub.Format == 1 {
						payload := effective.GPosPayload().CursiveFmt1
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected cursive fmt1 payload", fontName, i, j)
						}
						cnt, ok := supportStructField(legacySub.Support, "EntryExitCount")
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: missing legacy entry/exit count", fontName, i, j)
						}
						if len(payload.Entries) != int(cnt.Interface().(uint16)) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: entry/exit count mismatch legacy=%d concrete=%d",
								fontName, i, j, cnt.Interface().(uint16), len(payload.Entries))
						}
						checked++
					}

				case GPosLookupTypeMarkToBase:
					if legacySub.Format == 1 {
						payload := effective.GPosPayload().MarkToBaseFmt1
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected mark-to-base fmt1 payload", fontName, i, j)
						}
						mcc, ok1 := supportStructField(legacySub.Support, "MarkClassCount")
						marr, ok2 := supportStructField(legacySub.Support, "MarkArray")
						barr, ok3 := supportStructField(legacySub.Support, "BaseArray")
						bcov, ok4 := supportStructField(legacySub.Support, "BaseCoverage")
						if !(ok1 && ok2 && ok3 && ok4) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: incomplete legacy support for mark-to-base", fontName, i, j)
						}
						legacyMarks := marr.Interface().(MarkArray)
						legacyBases := barr.Interface().([]BaseRecord)
						legacyBaseCov := bcov.Interface().(Coverage)
						if payload.MarkClassCount != mcc.Interface().(uint16) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: mark-class-count mismatch", fontName, i, j)
						}
						if len(payload.MarkRecords) != len(legacyMarks.MarkRecords) || len(payload.BaseRecords) != len(legacyBases) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: mark/base record count mismatch", fontName, i, j)
						}
						if payload.BaseCoverage.Count != legacyBaseCov.Count {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: base-coverage count mismatch legacy=%d concrete=%d",
								fontName, i, j, legacyBaseCov.Count, payload.BaseCoverage.Count)
						}
						checked++
					}

				case GPosLookupTypeMarkToLigature:
					if legacySub.Format == 1 {
						payload := effective.GPosPayload().MarkToLigatureFmt1
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected mark-to-ligature fmt1 payload", fontName, i, j)
						}
						mcc, ok1 := supportStructField(legacySub.Support, "MarkClassCount")
						marr, ok2 := supportStructField(legacySub.Support, "MarkArray")
						larr, ok3 := supportStructField(legacySub.Support, "LigatureArray")
						lcov, ok4 := supportStructField(legacySub.Support, "LigatureCoverage")
						if !(ok1 && ok2 && ok3 && ok4) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: incomplete legacy support for mark-to-ligature", fontName, i, j)
						}
						legacyMarks := marr.Interface().(MarkArray)
						legacyLig := larr.Interface().([]LigatureAttach)
						legacyLigCov := lcov.Interface().(Coverage)
						if payload.MarkClassCount != mcc.Interface().(uint16) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: mark-class-count mismatch", fontName, i, j)
						}
						if len(payload.MarkRecords) != len(legacyMarks.MarkRecords) || len(payload.LigatureRecords) != len(legacyLig) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: mark/ligature record count mismatch", fontName, i, j)
						}
						if payload.LigatureCoverage.Count != legacyLigCov.Count {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: ligature-coverage count mismatch legacy=%d concrete=%d",
								fontName, i, j, legacyLigCov.Count, payload.LigatureCoverage.Count)
						}
						checked++
					}

				case GPosLookupTypeMarkToMark:
					if legacySub.Format == 1 {
						payload := effective.GPosPayload().MarkToMarkFmt1
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected mark-to-mark fmt1 payload", fontName, i, j)
						}
						mcc, ok1 := supportStructField(legacySub.Support, "MarkClassCount")
						marr, ok2 := supportStructField(legacySub.Support, "Mark1Array")
						m2arr, ok3 := supportStructField(legacySub.Support, "Mark2Array")
						m2cov, ok4 := supportStructField(legacySub.Support, "Mark2Coverage")
						if !(ok1 && ok2 && ok3 && ok4) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: incomplete legacy support for mark-to-mark", fontName, i, j)
						}
						legacyMarks := marr.Interface().(MarkArray)
						legacyMark2 := m2arr.Interface().([]BaseRecord)
						legacyMark2Cov := m2cov.Interface().(Coverage)
						if payload.MarkClassCount != mcc.Interface().(uint16) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: mark-class-count mismatch", fontName, i, j)
						}
						if len(payload.Mark1Records) != len(legacyMarks.MarkRecords) || len(payload.Mark2Records) != len(legacyMark2) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: mark1/mark2 record count mismatch", fontName, i, j)
						}
						if payload.Mark2Coverage.Count != legacyMark2Cov.Count {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: mark2-coverage count mismatch legacy=%d concrete=%d",
								fontName, i, j, legacyMark2Cov.Count, payload.Mark2Coverage.Count)
						}
						checked++
					}

				case GPosLookupTypeContextPos:
					switch legacySub.Format {
					case 2:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok || len(legacyCtx.ClassDefs) != 1 {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected legacy context-pos fmt2 class context", fontName, i, j)
						}
						payload := effective.GPosPayload().ContextFmt2
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected concrete context-pos fmt2 payload", fontName, i, j)
						}
						if payload.ClassDef.format != legacyCtx.ClassDefs[0].format {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: context-pos fmt2 class-def format mismatch", fontName, i, j)
						}
						checked++
					case 3:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected legacy context-pos fmt3 sequence context", fontName, i, j)
						}
						payload := effective.GPosPayload().ContextFmt3
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected concrete context-pos fmt3 payload", fontName, i, j)
						}
						if len(payload.InputCoverages) != len(legacyCtx.InputCoverage) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: context-pos fmt3 coverage-count mismatch legacy=%d concrete=%d",
								fontName, i, j, len(legacyCtx.InputCoverage), len(payload.InputCoverages))
						}
						if len(legacySub.LookupRecords) > 0 && !sequenceLookupRecordsEqual(legacySub.LookupRecords, payload.Records) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: context-pos fmt3 lookup-record mismatch", fontName, i, j)
						}
						checked++
					}

				case GPosLookupTypeChainedContextPos:
					switch legacySub.Format {
					case 2:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok || len(legacyCtx.ClassDefs) != 3 {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected legacy chain-context-pos fmt2 class context", fontName, i, j)
						}
						payload := effective.GPosPayload().ChainingContextFmt2
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected concrete chain-context-pos fmt2 payload", fontName, i, j)
						}
						if payload.BacktrackClassDef.format != legacyCtx.ClassDefs[0].format ||
							payload.InputClassDef.format != legacyCtx.ClassDefs[1].format ||
							payload.LookaheadClassDef.format != legacyCtx.ClassDefs[2].format {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: chain-context-pos fmt2 class-def format mismatch", fontName, i, j)
						}
						checked++
					case 3:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected legacy chain-context-pos fmt3 sequence context", fontName, i, j)
						}
						payload := effective.GPosPayload().ChainingContextFmt3
						if payload == nil {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: expected concrete chain-context-pos fmt3 payload", fontName, i, j)
						}
						if len(payload.BacktrackCoverages) != len(legacyCtx.BacktrackCoverage) ||
							len(payload.InputCoverages) != len(legacyCtx.InputCoverage) ||
							len(payload.LookaheadCoverages) != len(legacyCtx.LookaheadCoverage) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: chain-context-pos fmt3 coverage-count mismatch", fontName, i, j)
						}
						if len(legacySub.LookupRecords) > 0 && !sequenceLookupRecordsEqual(legacySub.LookupRecords, payload.Records) {
							t.Fatalf("%s GPOS lookup[%d]/subtable[%d]: chain-context-pos fmt3 lookup-record mismatch", fontName, i, j)
						}
						checked++
					}
				}
			}
		}
	}

	if checked == 0 {
		t.Fatalf("expected at least one effective GPOS payload parity check")
	}
}

func TestConcreteContextParityGSUB(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	fonts := []string{"Calibri", "GentiumPlus-R"}
	found := 0

	for _, fontName := range fonts {
		otf := parseFont(t, fontName)
		gsub := otf.tables[T("GSUB")].Self().AsGSub()
		if gsub == nil || gsub.LookupGraph() == nil {
			continue
		}
		for i := 0; i < gsub.LookupList.Len(); i++ {
			legacyLookup := gsub.LookupList.Navigate(i)
			concreteLookup := gsub.LookupGraph().Lookup(i)
			if concreteLookup == nil {
				continue
			}
			for j := 0; j < int(legacyLookup.SubTableCount); j++ {
				legacySub := legacyLookup.Subtable(j)
				concreteSub := concreteLookup.Subtable(j)
				if legacySub == nil || concreteSub == nil {
					continue
				}
				effective := effectiveConcreteLookupNode(concreteSub)
				if effective == nil || effective.GSubPayload() == nil {
					continue
				}
				switch legacySub.LookupType {
				case GSubLookupTypeContext:
					switch legacySub.Format {
					case 2:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok || len(legacyCtx.ClassDefs) != 1 {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected legacy format-2 class context", fontName, i, j)
						}
						payload := effective.GSubPayload().ContextFmt2
						if payload == nil {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected concrete context fmt2 payload", fontName, i, j)
						}
						if payload.ClassDef.format != legacyCtx.ClassDefs[0].format {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: class-def format mismatch legacy=%d concrete=%d",
								fontName, i, j, legacyCtx.ClassDefs[0].format, payload.ClassDef.format)
						}
						found++
					case 3:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected legacy format-3 sequence context", fontName, i, j)
						}
						payload := effective.GSubPayload().ContextFmt3
						if payload == nil {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected concrete context fmt3 payload", fontName, i, j)
						}
						if len(payload.InputCoverages) != len(legacyCtx.InputCoverage) {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: input coverage count mismatch legacy=%d concrete=%d",
								fontName, i, j, len(legacyCtx.InputCoverage), len(payload.InputCoverages))
						}
						// Legacy type-5 format-3 does not always materialize lookup records yet.
						if len(legacySub.LookupRecords) > 0 && !sequenceLookupRecordsEqual(legacySub.LookupRecords, payload.Records) {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: sequence lookup records mismatch", fontName, i, j)
						}
						found++
					}
				case GSubLookupTypeChainingContext:
					switch legacySub.Format {
					case 2:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok || len(legacyCtx.ClassDefs) != 3 {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected legacy chaining format-2 class context", fontName, i, j)
						}
						payload := effective.GSubPayload().ChainingContextFmt2
						if payload == nil {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected concrete chaining fmt2 payload", fontName, i, j)
						}
						if payload.BacktrackClassDef.format != legacyCtx.ClassDefs[0].format ||
							payload.InputClassDef.format != legacyCtx.ClassDefs[1].format ||
							payload.LookaheadClassDef.format != legacyCtx.ClassDefs[2].format {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: class-def format mismatch in chaining fmt2", fontName, i, j)
						}
						found++
					case 3:
						legacyCtx, ok := asSequenceContext(legacySub.Support)
						if !ok {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected legacy chaining format-3 sequence context", fontName, i, j)
						}
						payload := effective.GSubPayload().ChainingContextFmt3
						if payload == nil {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: expected concrete chaining fmt3 payload", fontName, i, j)
						}
						if len(payload.BacktrackCoverages) != len(legacyCtx.BacktrackCoverage) ||
							len(payload.InputCoverages) != len(legacyCtx.InputCoverage) ||
							len(payload.LookaheadCoverages) != len(legacyCtx.LookaheadCoverage) {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: coverage-count mismatch in chaining fmt3", fontName, i, j)
						}
						if !sequenceLookupRecordsEqual(legacySub.LookupRecords, payload.Records) {
							t.Fatalf("%s GSUB lookup[%d]/subtable[%d]: sequence lookup records mismatch in chaining fmt3", fontName, i, j)
						}
						found++
					}
				}
			}
		}
	}
	if found == 0 {
		t.Fatalf("expected at least one GSUB contextual/chaining format-2/3 subtable for parity check")
	}
}

func TestConcreteExtensionAndContextConcurrentAccess(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otf := parseFont(t, "Calibri")
	gsub := otf.tables[T("GSUB")].Self().AsGSub()
	gpos := otf.tables[T("GPOS")].Self().AsGPos()
	if gsub == nil || gsub.LookupGraph() == nil || gpos == nil || gpos.LookupGraph() == nil {
		t.Fatalf("expected GSUB+GPOS lookup graphs")
	}

	// Extension-heavy concurrent access (GPOS, extension wrappers are common).
	extLookup, extSub := -1, -1
	for i, lookup := range gpos.LookupGraph().Range() {
		if lookup == nil {
			continue
		}
		for j, sub := range lookup.Range() {
			if sub == nil || sub.GPosPayload() == nil || sub.GPosPayload().ExtensionFmt1 == nil {
				continue
			}
			extLookup, extSub = i, j
			break
		}
		if extLookup >= 0 {
			break
		}
	}
	if extLookup < 0 {
		t.Fatalf("expected at least one extension subtable in concrete GPOS graph")
	}
	lookup := gpos.LookupGraph().Lookup(extLookup)
	const workers = 16
	var wg sync.WaitGroup
	nodes := make(chan *LookupNode, workers)
	resolved := make(chan *LookupNode, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			n := lookup.Subtable(extSub)
			nodes <- n
			if n != nil && n.GPosPayload() != nil && n.GPosPayload().ExtensionFmt1 != nil {
				resolved <- n.GPosPayload().ExtensionFmt1.Resolved
			} else {
				resolved <- nil
			}
		}()
	}
	wg.Wait()
	close(nodes)
	close(resolved)
	var firstNode, firstResolved *LookupNode
	for n := range nodes {
		if n == nil {
			t.Fatalf("concurrent extension subtable access returned nil")
		}
		if firstNode == nil {
			firstNode = n
		} else if n != firstNode {
			t.Fatalf("concurrent extension subtable access produced different cached node pointers")
		}
	}
	for rn := range resolved {
		if rn == nil {
			t.Fatalf("concurrent extension access returned nil resolved node")
		}
		if firstResolved == nil {
			firstResolved = rn
		} else if rn != firstResolved {
			t.Fatalf("concurrent extension access produced different resolved node pointers")
		}
	}

	// Context-heavy concurrent access (GSUB effective node).
	ctxLookup, ctxSub := -1, -1
	for i, lookup := range gsub.LookupGraph().Range() {
		if lookup == nil {
			continue
		}
		for j, sub := range lookup.Range() {
			effective := effectiveConcreteLookupNode(sub)
			if effective == nil || effective.GSubPayload() == nil {
				continue
			}
			p := effective.GSubPayload()
			if p.ContextFmt2 != nil || p.ContextFmt3 != nil || p.ChainingContextFmt2 != nil || p.ChainingContextFmt3 != nil {
				ctxLookup, ctxSub = i, j
				break
			}
		}
		if ctxLookup >= 0 {
			break
		}
	}
	if ctxLookup < 0 {
		t.Fatalf("expected at least one contextual/chaining subtable in concrete GSUB graph")
	}
	ctxNodes := make(chan *LookupNode, workers)
	wg = sync.WaitGroup{}
	wg.Add(workers)
	ctxLookupPtr := gsub.LookupGraph().Lookup(ctxLookup)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			ctxNodes <- ctxLookupPtr.Subtable(ctxSub)
		}()
	}
	wg.Wait()
	close(ctxNodes)
	var firstCtxNode *LookupNode
	for n := range ctxNodes {
		if n == nil {
			t.Fatalf("concurrent context subtable access returned nil")
		}
		if firstCtxNode == nil {
			firstCtxNode = n
		} else if n != firstCtxNode {
			t.Fatalf("concurrent context subtable access produced different cached node pointers")
		}
		effective := effectiveConcreteLookupNode(n)
		if effective == nil || effective.GSubPayload() == nil {
			t.Fatalf("concurrent context access returned node without effective GSUB payload")
		}
	}
}
