package ot

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/internal/ttxtest"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestTTXGSUBStructural_AlternateSubstFmt1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otfPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub3_1_simple_f1.otf")
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub3_1_simple_f1.ttx.GSUB")

	data, err := os.ReadFile(otfPath)
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	otf, err := Parse(data, IsTestfont)
	if err != nil {
		t.Logf("Parse: %v", err)
	}
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatalf("font missing GSUB table")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot convert GSUB table")
	}
	exp, err := ttxtest.ParseTTXGSUB(ttxPath)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}

	if err := compareExpectedGSUB(gsub, exp); err != nil {
		t.Fatalf("GSUB compare failed: %v", err)
	}
}

func TestTTXGSUBStructural_AlternateSubstFmt1_LookupFlag(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otfPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub3_1_lookupflag_f1.otf")
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub3_1_lookupflag_f1.ttx.GSUB")

	data, err := os.ReadFile(otfPath)
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	otf, err := Parse(data, IsTestfont)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatalf("font missing GSUB table")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot convert GSUB table")
	}
	exp, err := ttxtest.ParseTTXGSUB(ttxPath)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}

	if err := compareExpectedGSUB(gsub, exp); err != nil {
		t.Fatalf("GSUB compare failed: %v", err)
	}
}

func TestTTXGSUBStructural_SingleAndLigature(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otfPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_chaining2_next_glyph_f1.otf")
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_chaining2_next_glyph_f1.ttx.GSUB")

	data, err := os.ReadFile(otfPath)
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	otf, err := Parse(data, IsTestfont)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatalf("font missing GSUB table")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot convert GSUB table")
	}
	exp, err := ttxtest.ParseTTXGSUB(ttxPath)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}

	if err := compareExpectedGSUBLookups(gsub, exp, []int{0, 1}); err != nil {
		t.Fatalf("GSUB compare failed: %v", err)
	}
}

func TestTTXGSUBStructural_LigatureIgnoreMarks(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otfPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_chaining2_next_glyph_f1.otf")
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_chaining2_next_glyph_f1.ttx.GSUB")

	data, err := os.ReadFile(otfPath)
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	otf, err := Parse(data, IsTestfont)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatalf("font missing GSUB table")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot convert GSUB table")
	}
	exp, err := ttxtest.ParseTTXGSUB(ttxPath)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}

	if err := compareExpectedGSUBLookups(gsub, exp, []int{2}); err != nil {
		t.Fatalf("GSUB compare failed: %v", err)
	}
}

func TestTTXGSUBStructural_ContextSubstFmt1_IgnoreMarks(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otfPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_context1_lookupflag_f1.otf")
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_context1_lookupflag_f1.ttx.GSUB")

	data, err := os.ReadFile(otfPath)
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	otf, err := Parse(data, IsTestfont)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatalf("font missing GSUB table")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot convert GSUB table")
	}
	exp, err := ttxtest.ParseTTXGSUB(ttxPath)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}

	if err := compareExpectedGSUBLookups(gsub, exp, []int{4}); err != nil {
		t.Fatalf("GSUB compare failed: %v", err)
	}
}

func TestTTXGSUBStructural_ContextSubstFmt1_NextGlyph(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otfPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_context1_next_glyph_f1.otf")
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gsub_context1_next_glyph_f1.ttx.GSUB")

	data, err := os.ReadFile(otfPath)
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	otf, err := Parse(data, IsTestfont)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatalf("font missing GSUB table")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot convert GSUB table")
	}
	exp, err := ttxtest.ParseTTXGSUB(ttxPath)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}

	if err := compareExpectedGSUBLookups(gsub, exp, []int{4}); err != nil {
		t.Fatalf("GSUB compare failed: %v", err)
	}
}

func TestTTXGSUBStructural_ContextSubstFmt2_ClassDef2Font4(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otfPath := filepath.Join("..", "testdata", "fonttools-tests", "classdef2_font4.otf")
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "classdef2_font4.ttx.GSUB")

	data, err := os.ReadFile(otfPath)
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	otf, err := Parse(data, IsTestfont)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	table := otf.Table(T("GSUB"))
	if table == nil {
		t.Fatalf("font missing GSUB table")
	}
	gsub := table.Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot convert GSUB table")
	}
	exp, err := ttxtest.ParseTTXGSUB(ttxPath)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}

	if err := compareExpectedGSUBLookups(gsub, exp, []int{3}); err != nil {
		t.Fatalf("GSUB compare failed: %v", err)
	}
}

func compareExpectedGSUBLookups(gsub *GSubTable, exp *ttxtest.ExpectedGSUB, indices []int) error {
	if gsub == nil {
		return fmt.Errorf("nil GSUB")
	}
	if exp == nil {
		return fmt.Errorf("nil expected GSUB")
	}
	graph := gsub.LookupGraph()
	if graph == nil {
		return fmt.Errorf("nil GSUB lookup graph")
	}
	for _, i := range indices {
		if i < 0 || i >= len(exp.Lookups) {
			return fmt.Errorf("expected lookup index %d out of range", i)
		}
		if i < 0 || i >= graph.Len() {
			return fmt.Errorf("actual lookup index %d out of range", i)
		}
		el := exp.Lookups[i]
		lookup := graph.Lookup(i)
		if lookup == nil {
			return fmt.Errorf("lookup[%d] missing", i)
		}
		if lookup.Type != LayoutTableLookupType(el.Type) {
			return fmt.Errorf("lookup[%d] type mismatch: got %d, want %d", i, lookup.Type, el.Type)
		}
		if el.Flag != 0 && uint16(lookup.Flag) != el.Flag {
			return fmt.Errorf("lookup[%d] flag mismatch: got %d, want %d", i, lookup.Flag, el.Flag)
		}
		if int(lookup.SubTableCount) != len(el.Subtables) {
			return fmt.Errorf("lookup[%d] subtable count mismatch: got %d, want %d",
				i, lookup.SubTableCount, len(el.Subtables))
		}
		for j, est := range el.Subtables {
			node := lookup.Subtable(j)
			if node == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] missing", i, j)
			}
			enode := effectiveGSubNode(node)
			if enode == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] effective node missing", i, j)
			}
			if enode.Format != uint16(est.Format) {
				return fmt.Errorf("lookup[%d] subtable[%d] format mismatch: got %d, want %d",
					i, j, enode.Format, est.Format)
			}
			if LayoutTableLookupType(est.Type) != enode.LookupType {
				return fmt.Errorf("lookup[%d] subtable[%d] type mismatch: got %d, want %d",
					i, j, enode.LookupType, est.Type)
			}
			switch enode.LookupType {
			case GSubLookupTypeSingle:
				if err := compareSingleSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeAlternate:
				if err := compareAlternateSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeLigature:
				if err := compareLigatureSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeContext:
				if err := compareContextSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			}
		}
	}
	return nil
}

func compareExpectedGSUB(gsub *GSubTable, exp *ttxtest.ExpectedGSUB) error {
	if gsub == nil {
		return fmt.Errorf("nil GSUB")
	}
	if exp == nil {
		return fmt.Errorf("nil expected GSUB")
	}
	graph := gsub.LookupGraph()
	if graph == nil {
		return fmt.Errorf("nil GSUB lookup graph")
	}
	if graph.Len() != len(exp.Lookups) {
		return fmt.Errorf("lookup count mismatch: got %d, want %d", graph.Len(), len(exp.Lookups))
	}
	for i, el := range exp.Lookups {
		lookup := graph.Lookup(i)
		if lookup == nil {
			return fmt.Errorf("lookup[%d] missing", i)
		}
		if lookup.Type != LayoutTableLookupType(el.Type) {
			return fmt.Errorf("lookup[%d] type mismatch: got %d, want %d", i, lookup.Type, el.Type)
		}
		if el.Flag != 0 && uint16(lookup.Flag) != el.Flag {
			return fmt.Errorf("lookup[%d] flag mismatch: got %d, want %d", i, lookup.Flag, el.Flag)
		}
		if int(lookup.SubTableCount) != len(el.Subtables) {
			return fmt.Errorf("lookup[%d] subtable count mismatch: got %d, want %d",
				i, lookup.SubTableCount, len(el.Subtables))
		}
		for j, est := range el.Subtables {
			node := lookup.Subtable(j)
			if node == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] missing", i, j)
			}
			enode := effectiveGSubNode(node)
			if enode == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] effective node missing", i, j)
			}
			if enode.Format != uint16(est.Format) {
				return fmt.Errorf("lookup[%d] subtable[%d] format mismatch: got %d, want %d",
					i, j, enode.Format, est.Format)
			}
			if LayoutTableLookupType(est.Type) != enode.LookupType {
				return fmt.Errorf("lookup[%d] subtable[%d] type mismatch: got %d, want %d",
					i, j, enode.LookupType, est.Type)
			}
			switch enode.LookupType {
			case GSubLookupTypeSingle:
				if err := compareSingleSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeAlternate:
				if err := compareAlternateSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeLigature:
				if err := compareLigatureSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeContext:
				if err := compareContextSubst(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			}
		}
	}
	return nil
}

func effectiveGSubNode(node *LookupNode) *LookupNode {
	if node == nil {
		return nil
	}
	payload := node.GSubPayload()
	if payload != nil && payload.ExtensionFmt1 != nil && payload.ExtensionFmt1.Resolved != nil {
		return payload.ExtensionFmt1.Resolved
	}
	return node
}

func compareSingleSubst(node *LookupNode, est ttxtest.ExpectedSubtable) error {
	payload := node.GSubPayload()
	if payload == nil {
		return fmt.Errorf("missing GSUB payload")
	}
	if len(est.SingleSubst) == 0 {
		return fmt.Errorf("expected single substitutions missing")
	}
	coverage, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.SingleSubst) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.SingleSubst))
	}
	for i, name := range est.Coverage {
		in, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != in {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], in)
		}
		outName, ok := est.SingleSubst[name]
		if !ok {
			return fmt.Errorf("missing substitution for %q", name)
		}
		out, err := glyphNameToID(outName)
		if err != nil {
			return fmt.Errorf("substitution out %q: %w", outName, err)
		}
		switch node.Format {
		case 1:
			if payload.SingleFmt1 == nil {
				return fmt.Errorf("missing delta for format 1")
			}
			delta := payload.SingleFmt1.DeltaGlyphID
			got := GlyphIndex(int(in) + int(delta))
			if got != out {
				return fmt.Errorf("substitution %q mismatch: got %d, want %d", name, got, out)
			}
		case 2:
			if payload.SingleFmt2 == nil || i >= len(payload.SingleFmt2.SubstituteGlyphIDs) {
				return fmt.Errorf("missing substitution list entry %d", i)
			}
			if got := payload.SingleFmt2.SubstituteGlyphIDs[i]; got != out {
				return fmt.Errorf("substitution %q mismatch: got %d, want %d", name, got, out)
			}
		default:
			return fmt.Errorf("unsupported single subst format %d", node.Format)
		}
	}
	return nil
}

func compareAlternateSubst(node *LookupNode, est ttxtest.ExpectedSubtable) error {
	payload := node.GSubPayload()
	if payload == nil || payload.AlternateFmt1 == nil {
		return fmt.Errorf("missing alternate payload")
	}
	coverage, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Coverage) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Coverage))
	}
	for i, name := range est.Coverage {
		gid, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != gid {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], gid)
		}
	}

	for i, name := range est.Coverage {
		base, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("alternate set glyph %q: %w", name, err)
		}
		if i >= len(payload.AlternateFmt1.Alternates) {
			return fmt.Errorf("alternate set %q missing", name)
		}
		actual := payload.AlternateFmt1.Alternates[i]
		expNames := est.Alternates[name]
		exp := make([]GlyphIndex, 0, len(expNames))
		for _, altName := range expNames {
			gid, err := glyphNameToID(altName)
			if err != nil {
				return fmt.Errorf("alternate glyph %q: %w", altName, err)
			}
			exp = append(exp, gid)
		}
		if len(actual) != len(exp) {
			return fmt.Errorf("alternate set %d (glyph %d) length mismatch: got %d, want %d",
				i, base, len(actual), len(exp))
		}
		for k := range exp {
			if actual[k] != exp[k] {
				return fmt.Errorf("alternate set %d (glyph %d) mismatch at %d: got %d, want %d",
					i, base, k, actual[k], exp[k])
			}
		}
	}
	return nil
}

func compareLigatureSubst(node *LookupNode, est ttxtest.ExpectedSubtable) error {
	payload := node.GSubPayload()
	if payload == nil || payload.LigatureFmt1 == nil {
		return fmt.Errorf("missing ligature payload")
	}
	if len(est.Ligatures) == 0 {
		return fmt.Errorf("expected ligatures missing")
	}
	coverage, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Ligatures) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Ligatures))
	}
	for i, name := range est.Coverage {
		first, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != first {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], first)
		}
		exp := est.Ligatures[name]
		if i >= len(payload.LigatureFmt1.LigatureSets) {
			return fmt.Errorf("ligature set %q missing", name)
		}
		actual := toParsedLigatures(payload.LigatureFmt1.LigatureSets[i])
		if len(actual) != len(exp) {
			return fmt.Errorf("ligature count mismatch for %q: got %d, want %d", name, len(actual), len(exp))
		}
		for k := range exp {
			if err := compareLigature(actual[k], exp[k]); err != nil {
				return fmt.Errorf("ligature %q[%d]: %w", name, k, err)
			}
		}
	}
	return nil
}

func compareContextSubst(node *LookupNode, est ttxtest.ExpectedSubtable) error {
	switch node.Format {
	case 1:
		return compareContextSubstFmt1(node, est)
	case 2:
		return compareContextSubstFmt2(node, est)
	default:
		return fmt.Errorf("unsupported context subst format %d", node.Format)
	}
}

func compareContextSubstFmt1(node *LookupNode, est ttxtest.ExpectedSubtable) error {
	payload := node.GSubPayload()
	if payload == nil || payload.ContextFmt1 == nil {
		return fmt.Errorf("missing context format 1 payload")
	}
	if est.ContextSubst == nil {
		return fmt.Errorf("expected context subst missing")
	}
	if node.Format != 1 {
		return fmt.Errorf("unsupported context subst format %d", node.Format)
	}
	coverage, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Coverage) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Coverage))
	}
	for i, name := range est.Coverage {
		gid, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != gid {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], gid)
		}
	}

	for i := range est.Coverage {
		var expRules []ttxtest.ExpectedSequenceRule
		if i < len(est.ContextSubst.RuleSets) {
			expRules = est.ContextSubst.RuleSets[i].Rules
		}
		rules := parseSequenceRules(payload.ContextFmt1, i)
		if len(rules) != len(expRules) {
			return fmt.Errorf("rule set %d count mismatch: got %d, want %d", i, len(rules), len(expRules))
		}
		for r := range expRules {
			if err := compareSequenceRule(rules[r], expRules[r]); err != nil {
				return fmt.Errorf("rule set %d rule %d: %w", i, r, err)
			}
		}
	}
	return nil
}

func compareContextSubstFmt2(node *LookupNode, est ttxtest.ExpectedSubtable) error {
	payload := node.GSubPayload()
	if payload == nil || payload.ContextFmt2 == nil {
		return fmt.Errorf("missing context format 2 payload")
	}
	if est.ContextSubst == nil {
		return fmt.Errorf("expected context subst missing")
	}
	if node.Format != 2 {
		return fmt.Errorf("unsupported context subst format %d", node.Format)
	}
	coverage, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Coverage) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Coverage))
	}
	for i, name := range est.Coverage {
		gid, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != gid {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], gid)
		}
	}

	if len(est.ContextSubst.ClassDefs) > 0 {
		for name, class := range est.ContextSubst.ClassDefs {
			gid, err := glyphNameToID(name)
			if err != nil {
				return fmt.Errorf("classdef glyph %q: %w", name, err)
			}
			if got := int(payload.ContextFmt2.ClassDef.Lookup(gid)); got != class {
				return fmt.Errorf("classdef %q mismatch: got %d, want %d", name, got, class)
			}
		}
	}

	for i := range est.ContextSubst.ClassRuleSets {
		expRules := est.ContextSubst.ClassRuleSets[i].Rules
		rules := parseClassSequenceRules(payload.ContextFmt2, i)
		if len(rules) != len(expRules) {
			return fmt.Errorf("class rule set %d count mismatch: got %d, want %d", i, len(rules), len(expRules))
		}
		for r := range expRules {
			if err := compareClassSequenceRule(rules[r], expRules[r]); err != nil {
				return fmt.Errorf("class rule set %d rule %d: %w", i, r, err)
			}
		}
	}
	return nil
}

type parsedSequenceRule struct {
	Input   []GlyphIndex
	Records []SequenceLookupRecord
}

func parseSequenceRules(payload *GSubContextFmt1Payload, coverageIndex int) []parsedSequenceRule {
	if payload == nil || coverageIndex < 0 || coverageIndex >= len(payload.RuleSets) {
		return nil
	}
	ruleSet := payload.RuleSets[coverageIndex]
	out := make([]parsedSequenceRule, 0, len(ruleSet))
	for _, r := range ruleSet {
		input := make([]GlyphIndex, len(r.InputGlyphs))
		copy(input, r.InputGlyphs)
		records := make([]SequenceLookupRecord, len(r.Records))
		copy(records, r.Records)
		out = append(out, parsedSequenceRule{Input: input, Records: records})
	}
	return out
}

func compareSequenceRule(actual parsedSequenceRule, exp ttxtest.ExpectedSequenceRule) error {
	if len(actual.Input) != len(exp.Input) {
		return fmt.Errorf("input count mismatch: got %d, want %d", len(actual.Input), len(exp.Input))
	}
	for i, name := range exp.Input {
		gid, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("input[%d] %q: %w", i, name, err)
		}
		if actual.Input[i] != gid {
			return fmt.Errorf("input[%d] mismatch: got %d, want %d", i, actual.Input[i], gid)
		}
	}
	if len(actual.Records) != len(exp.LookupRecords) {
		return fmt.Errorf("lookup record count mismatch: got %d, want %d", len(actual.Records), len(exp.LookupRecords))
	}
	for i, rec := range exp.LookupRecords {
		if actual.Records[i].SequenceIndex != uint16(rec.SequenceIndex) {
			return fmt.Errorf("lookup record[%d] sequence index mismatch: got %d, want %d",
				i, actual.Records[i].SequenceIndex, rec.SequenceIndex)
		}
		if actual.Records[i].LookupListIndex != uint16(rec.LookupListIndex) {
			return fmt.Errorf("lookup record[%d] list index mismatch: got %d, want %d",
				i, actual.Records[i].LookupListIndex, rec.LookupListIndex)
		}
	}
	return nil
}

type parsedClassSequenceRule struct {
	Classes []uint16
	Records []SequenceLookupRecord
}

func parseClassSequenceRules(payload *GSubContextFmt2Payload, setIndex int) []parsedClassSequenceRule {
	if payload == nil || setIndex < 0 || setIndex >= len(payload.RuleSets) {
		return nil
	}
	ruleSet := payload.RuleSets[setIndex]
	out := make([]parsedClassSequenceRule, 0, len(ruleSet))
	for _, r := range ruleSet {
		classes := make([]uint16, len(r.InputClasses))
		copy(classes, r.InputClasses)
		records := make([]SequenceLookupRecord, len(r.Records))
		copy(records, r.Records)
		out = append(out, parsedClassSequenceRule{Classes: classes, Records: records})
	}
	return out
}

func compareClassSequenceRule(actual parsedClassSequenceRule, exp ttxtest.ExpectedClassSequenceRule) error {
	if len(actual.Classes) != len(exp.Classes) {
		return fmt.Errorf("class count mismatch: got %d, want %d", len(actual.Classes), len(exp.Classes))
	}
	for i, cls := range exp.Classes {
		if actual.Classes[i] != uint16(cls) {
			return fmt.Errorf("class[%d] mismatch: got %d, want %d", i, actual.Classes[i], cls)
		}
	}
	if len(actual.Records) != len(exp.LookupRecords) {
		return fmt.Errorf("lookup record count mismatch: got %d, want %d", len(actual.Records), len(exp.LookupRecords))
	}
	for i, rec := range exp.LookupRecords {
		if actual.Records[i].SequenceIndex != uint16(rec.SequenceIndex) {
			return fmt.Errorf("lookup record[%d] sequence index mismatch: got %d, want %d",
				i, actual.Records[i].SequenceIndex, rec.SequenceIndex)
		}
		if actual.Records[i].LookupListIndex != uint16(rec.LookupListIndex) {
			return fmt.Errorf("lookup record[%d] list index mismatch: got %d, want %d",
				i, actual.Records[i].LookupListIndex, rec.LookupListIndex)
		}
	}
	return nil
}

type parsedLigature struct {
	Components []GlyphIndex
	Glyph      GlyphIndex
}

func toParsedLigatures(in []GSubLigatureRule) []parsedLigature {
	if len(in) == 0 {
		return nil
	}
	out := make([]parsedLigature, len(in))
	for i, lig := range in {
		components := make([]GlyphIndex, len(lig.Components))
		copy(components, lig.Components)
		out[i] = parsedLigature{
			Components: components,
			Glyph:      lig.Ligature,
		}
	}
	return out
}

func compareLigature(actual parsedLigature, exp ttxtest.ExpectedLigature) error {
	if len(exp.Components) != len(actual.Components) {
		return fmt.Errorf("component count mismatch: got %d, want %d", len(actual.Components), len(exp.Components))
	}
	for i, name := range exp.Components {
		gid, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("component %q: %w", name, err)
		}
		if actual.Components[i] != gid {
			return fmt.Errorf("component[%d] mismatch: got %d, want %d", i, actual.Components[i], gid)
		}
	}
	gid, err := glyphNameToID(exp.Glyph)
	if err != nil {
		return fmt.Errorf("glyph %q: %w", exp.Glyph, err)
	}
	if actual.Glyph != gid {
		return fmt.Errorf("ligature glyph mismatch: got %d, want %d", actual.Glyph, gid)
	}
	return nil
}

func coverageGlyphs(c Coverage) ([]GlyphIndex, error) {
	if c.GlyphRange == nil {
		return nil, nil
	}
	switch r := c.GlyphRange.(type) {
	case *glyphRangeArray:
		out := make([]GlyphIndex, 0, r.count)
		// if r.is32 {
		// 	for i := 0; i < r.count; i++ {
		// 		v, err := r.data.u32(i * 4)
		// 		if err != nil {
		// 			return nil, err
		// 		}
		// 		out = append(out, GlyphIndex(v))
		// 	}
		// } else {
		for i := range r.count {
			v, err := r.data.u16(i * 2)
			if err != nil {
				return nil, err
			}
			out = append(out, GlyphIndex(v))
		}
		// }
		return out, nil
	case *glyphRangeRecords:
		if r.count == 0 {
			return nil, nil
		}
		recordSize := 6
		// if r.is32 {
		// 	recordSize = 12
		// }
		maxIndex := 0
		for i := range r.count {
			base := i * recordSize
			// if r.is32 {
			// 	start, _ := r.data.u32(base)
			// 	end, _ := r.data.u32(base + 4)
			// 	startIndex, _ := r.data.u32(base + 8)
			// 	last := int(startIndex) + int(end-start)
			// 	if last > maxIndex {
			// 		maxIndex = last
			// 	}
			// } else {
			start, _ := r.data.u16(base)
			end, _ := r.data.u16(base + 2)
			startIndex, _ := r.data.u16(base + 4)
			last := int(startIndex) + int(end-start)
			if last > maxIndex {
				maxIndex = last
			}
			// }
		}
		out := make([]GlyphIndex, maxIndex+1)
		for i := range r.count {
			base := i * recordSize
			// if r.is32 {
			// 	start, _ := r.data.u32(base)
			// 	end, _ := r.data.u32(base + 4)
			// 	startIndex, _ := r.data.u32(base + 8)
			// 	for g := start; g <= end; g++ {
			// 		idx := int(startIndex) + int(g-start)
			// 		out[idx] = GlyphIndex(g)
			// 	}
			// } else {
			start, _ := r.data.u16(base)
			end, _ := r.data.u16(base + 2)
			startIndex, _ := r.data.u16(base + 4)
			for g := start; g <= end; g++ {
				idx := int(startIndex) + int(g-start)
				out[idx] = GlyphIndex(g)
			}
			// }
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported coverage glyph range type %T", c.GlyphRange)
	}
}

func glyphNameToID(name string) (GlyphIndex, error) {
	if name == ".notdef" {
		return 0, nil
	}
	if strings.HasPrefix(name, "g") && len(name) > 1 {
		n, err := strconv.Atoi(name[1:])
		if err != nil {
			return 0, err
		}
		return GlyphIndex(n), nil
	}
	if strings.HasPrefix(name, "glyph") && len(name) > 5 {
		n, err := strconv.Atoi(name[5:])
		if err != nil {
			return 0, err
		}
		return GlyphIndex(n), nil
	}
	return 0, fmt.Errorf("unsupported glyph name %q (expected gNN)", name)
}
