package ot

import (
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func validateGoldenGSubNode(t *testing.T, node *LookupNode) {
	t.Helper()
	if node == nil {
		t.Fatalf("expected non-nil GSUB lookup node")
	}
	p := node.GSubPayload()
	if p == nil {
		t.Fatalf("expected GSUB payload for lookup type=%d format=%d", node.LookupType, node.Format)
	}
	if slots := countGSubPayloadSlots(p); slots != 1 {
		t.Fatalf("expected exactly one GSUB payload slot for type=%d format=%d, got %d", node.LookupType, node.Format, slots)
	}
	switch node.LookupType {
	case GSubLookupTypeSingle:
		switch node.Format {
		case 1:
			if p.SingleFmt1 == nil {
				t.Fatalf("expected GSUB single fmt1 payload")
			}
		case 2:
			if p.SingleFmt2 == nil {
				t.Fatalf("expected GSUB single fmt2 payload")
			}
		default:
			t.Fatalf("unexpected GSUB single format %d", node.Format)
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GSUB single coverage")
		}
	case GSubLookupTypeMultiple:
		if node.Format != 1 || p.MultipleFmt1 == nil {
			t.Fatalf("expected GSUB multiple fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GSUB multiple coverage")
		}
	case GSubLookupTypeAlternate:
		if node.Format != 1 || p.AlternateFmt1 == nil {
			t.Fatalf("expected GSUB alternate fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GSUB alternate coverage")
		}
	case GSubLookupTypeLigature:
		if node.Format != 1 || p.LigatureFmt1 == nil {
			t.Fatalf("expected GSUB ligature fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GSUB ligature coverage")
		}
	case GSubLookupTypeContext:
		switch node.Format {
		case 1:
			if p.ContextFmt1 == nil {
				t.Fatalf("expected GSUB context fmt1 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GSUB context fmt1 coverage")
			}
		case 2:
			if p.ContextFmt2 == nil {
				t.Fatalf("expected GSUB context fmt2 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GSUB context fmt2 coverage")
			}
		case 3:
			if p.ContextFmt3 == nil {
				t.Fatalf("expected GSUB context fmt3 payload")
			}
			if len(p.ContextFmt3.InputCoverages) == 0 {
				t.Fatalf("expected non-empty GSUB context fmt3 input coverage")
			}
		default:
			t.Fatalf("unexpected GSUB context format %d", node.Format)
		}
	case GSubLookupTypeChainingContext:
		switch node.Format {
		case 1:
			if p.ChainingContextFmt1 == nil {
				t.Fatalf("expected GSUB chaining-context fmt1 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GSUB chaining-context fmt1 coverage")
			}
		case 2:
			if p.ChainingContextFmt2 == nil {
				t.Fatalf("expected GSUB chaining-context fmt2 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GSUB chaining-context fmt2 coverage")
			}
		case 3:
			if p.ChainingContextFmt3 == nil {
				t.Fatalf("expected GSUB chaining-context fmt3 payload")
			}
			if len(p.ChainingContextFmt3.InputCoverages) == 0 {
				t.Fatalf("expected non-empty GSUB chaining-context fmt3 input coverage")
			}
		default:
			t.Fatalf("unexpected GSUB chaining-context format %d", node.Format)
		}
	case GSubLookupTypeExtensionSubs:
		if node.Format != 1 || p.ExtensionFmt1 == nil {
			t.Fatalf("expected GSUB extension fmt1 payload")
		}
		if p.ExtensionFmt1.Resolved == nil {
			t.Fatalf("expected GSUB extension to resolve subtable")
		}
		if p.ExtensionFmt1.Resolved.LookupType != p.ExtensionFmt1.ResolvedType {
			t.Fatalf("GSUB extension resolved type mismatch: payload=%d node=%d",
				p.ExtensionFmt1.ResolvedType, p.ExtensionFmt1.Resolved.LookupType)
		}
		if countGSubPayloadSlots(p.ExtensionFmt1.Resolved.GSubPayload()) != 1 {
			t.Fatalf("expected exactly one payload slot on resolved GSUB extension node")
		}
	case GSubLookupTypeReverseChaining:
		if node.Format != 1 || p.ReverseChainingFmt1 == nil {
			t.Fatalf("expected GSUB reverse-chaining fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GSUB reverse-chaining coverage")
		}
	default:
		t.Fatalf("unexpected GSUB lookup type %d", node.LookupType)
	}
}

func validateGoldenGPosNode(t *testing.T, node *LookupNode) {
	t.Helper()
	if node == nil {
		t.Fatalf("expected non-nil GPOS lookup node")
	}
	p := node.GPosPayload()
	if p == nil {
		t.Fatalf("expected GPOS payload for lookup type=%d format=%d", node.LookupType, node.Format)
	}
	if slots := countGPosPayloadSlots(p); slots != 1 {
		t.Fatalf("expected exactly one GPOS payload slot for type=%d format=%d, got %d", node.LookupType, node.Format, slots)
	}
	switch GPosLookupType(node.LookupType) {
	case GPosLookupTypeSingle:
		switch node.Format {
		case 1:
			if p.SingleFmt1 == nil {
				t.Fatalf("expected GPOS single fmt1 payload")
			}
		case 2:
			if p.SingleFmt2 == nil {
				t.Fatalf("expected GPOS single fmt2 payload")
			}
		default:
			t.Fatalf("unexpected GPOS single format %d", node.Format)
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GPOS single coverage")
		}
	case GPosLookupTypePair:
		switch node.Format {
		case 1:
			if p.PairFmt1 == nil {
				t.Fatalf("expected GPOS pair fmt1 payload")
			}
		case 2:
			if p.PairFmt2 == nil {
				t.Fatalf("expected GPOS pair fmt2 payload")
			}
		default:
			t.Fatalf("unexpected GPOS pair format %d", node.Format)
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GPOS pair coverage")
		}
	case GPosLookupTypeCursive:
		if node.Format != 1 || p.CursiveFmt1 == nil {
			t.Fatalf("expected GPOS cursive fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GPOS cursive coverage")
		}
	case GPosLookupTypeMarkToBase:
		if node.Format != 1 || p.MarkToBaseFmt1 == nil {
			t.Fatalf("expected GPOS mark-to-base fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GPOS mark-to-base coverage")
		}
	case GPosLookupTypeMarkToLigature:
		if node.Format != 1 || p.MarkToLigatureFmt1 == nil {
			t.Fatalf("expected GPOS mark-to-ligature fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GPOS mark-to-ligature coverage")
		}
	case GPosLookupTypeMarkToMark:
		if node.Format != 1 || p.MarkToMarkFmt1 == nil {
			t.Fatalf("expected GPOS mark-to-mark fmt1 payload")
		}
		if node.Coverage.Count == 0 {
			t.Fatalf("expected non-empty GPOS mark-to-mark coverage")
		}
	case GPosLookupTypeContextPos:
		switch node.Format {
		case 1:
			if p.ContextFmt1 == nil {
				t.Fatalf("expected GPOS context fmt1 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GPOS context fmt1 coverage")
			}
		case 2:
			if p.ContextFmt2 == nil {
				t.Fatalf("expected GPOS context fmt2 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GPOS context fmt2 coverage")
			}
		case 3:
			if p.ContextFmt3 == nil {
				t.Fatalf("expected GPOS context fmt3 payload")
			}
			if len(p.ContextFmt3.InputCoverages) == 0 {
				t.Fatalf("expected non-empty GPOS context fmt3 input coverage")
			}
		default:
			t.Fatalf("unexpected GPOS context format %d", node.Format)
		}
	case GPosLookupTypeChainedContextPos:
		switch node.Format {
		case 1:
			if p.ChainingContextFmt1 == nil {
				t.Fatalf("expected GPOS chaining-context fmt1 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GPOS chaining-context fmt1 coverage")
			}
		case 2:
			if p.ChainingContextFmt2 == nil {
				t.Fatalf("expected GPOS chaining-context fmt2 payload")
			}
			if node.Coverage.Count == 0 {
				t.Fatalf("expected non-empty GPOS chaining-context fmt2 coverage")
			}
		case 3:
			if p.ChainingContextFmt3 == nil {
				t.Fatalf("expected GPOS chaining-context fmt3 payload")
			}
			if len(p.ChainingContextFmt3.InputCoverages) == 0 {
				t.Fatalf("expected non-empty GPOS chaining-context fmt3 input coverage")
			}
		default:
			t.Fatalf("unexpected GPOS chaining-context format %d", node.Format)
		}
	case GPosLookupTypeExtensionPos:
		if node.Format != 1 || p.ExtensionFmt1 == nil {
			t.Fatalf("expected GPOS extension fmt1 payload")
		}
		if p.ExtensionFmt1.Resolved == nil {
			t.Fatalf("expected GPOS extension to resolve subtable")
		}
		if p.ExtensionFmt1.Resolved.LookupType != p.ExtensionFmt1.ResolvedType {
			t.Fatalf("GPOS extension resolved type mismatch: payload=%d node=%d",
				p.ExtensionFmt1.ResolvedType, p.ExtensionFmt1.Resolved.LookupType)
		}
		if countGPosPayloadSlots(p.ExtensionFmt1.Resolved.GPosPayload()) != 1 {
			t.Fatalf("expected exactly one payload slot on resolved GPOS extension node")
		}
	default:
		t.Fatalf("unexpected GPOS lookup type %d", GPosLookupType(node.LookupType))
	}
}

func TestConcreteGoldenFontsGSUBStructure(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	fonts := []string{"Calibri", "GentiumPlus-R"}
	checked := 0
	skipped := 0
	for _, fontName := range fonts {
		otf := parseFont(t, fontName)
		table := otf.tables[T("GSUB")]
		if table == nil {
			t.Fatalf("%s: missing GSUB table", fontName)
		}
		gsub := table.Self().AsGSub()
		if gsub == nil || gsub.LookupGraph() == nil {
			t.Fatalf("%s: missing concrete GSUB lookup graph", fontName)
		}
		graph := gsub.LookupGraph()
		for i := 0; i < graph.Len(); i++ {
			lookup := graph.Lookup(i)
			if lookup == nil {
				t.Fatalf("%s: lookup[%d] is nil", fontName, i)
			}
			if lookup.Error() != nil {
				t.Fatalf("%s: lookup[%d] parse error: %v", fontName, i, lookup.Error())
			}
			for j := 0; j < int(lookup.SubTableCount); j++ {
				sub := lookup.Subtable(j)
				if sub == nil {
					t.Fatalf("%s: lookup[%d]/subtable[%d] is nil", fontName, i, j)
				}
				if sub.LookupType != lookup.Type {
					t.Fatalf("%s: lookup[%d]/subtable[%d] type mismatch lookup=%d sub=%d",
						fontName, i, j, lookup.Type, sub.LookupType)
				}
				if sub.Error() != nil {
					skipped++
					continue
				}
				validateGoldenGSubNode(t, sub)
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatalf("expected to validate at least one concrete GSUB subtable")
	}
	if skipped > 0 {
		t.Logf("skipped %d GSUB concrete subtables with parse errors", skipped)
	}
}

func TestConcreteGoldenFontsGPOSStructure(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	fonts := []string{"Calibri", "GentiumPlus-R"}
	checked := 0
	skipped := 0
	for _, fontName := range fonts {
		otf := parseFont(t, fontName)
		table := otf.tables[T("GPOS")]
		if table == nil {
			t.Fatalf("%s: missing GPOS table", fontName)
		}
		gpos := table.Self().AsGPos()
		if gpos == nil || gpos.LookupGraph() == nil {
			t.Fatalf("%s: missing concrete GPOS lookup graph", fontName)
		}
		graph := gpos.LookupGraph()
		for i := 0; i < graph.Len(); i++ {
			lookup := graph.Lookup(i)
			if lookup == nil {
				t.Fatalf("%s: lookup[%d] is nil", fontName, i)
			}
			if lookup.Error() != nil {
				t.Fatalf("%s: lookup[%d] parse error: %v", fontName, i, lookup.Error())
			}
			for j := 0; j < int(lookup.SubTableCount); j++ {
				sub := lookup.Subtable(j)
				if sub == nil {
					t.Fatalf("%s: lookup[%d]/subtable[%d] is nil", fontName, i, j)
				}
				if sub.LookupType != lookup.Type {
					t.Fatalf("%s: lookup[%d]/subtable[%d] type mismatch lookup=%d sub=%d",
						fontName, i, j, lookup.Type, sub.LookupType)
				}
				if sub.Error() != nil {
					skipped++
					continue
				}
				validateGoldenGPosNode(t, sub)
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatalf("expected to validate at least one concrete GPOS subtable")
	}
	if skipped > 0 {
		t.Logf("skipped %d GPOS concrete subtables with parse errors", skipped)
	}
}
