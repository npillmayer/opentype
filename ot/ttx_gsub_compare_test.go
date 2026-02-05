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
	for _, i := range indices {
		if i < 0 || i >= len(exp.Lookups) {
			return fmt.Errorf("expected lookup index %d out of range", i)
		}
		if i < 0 || i >= gsub.LookupList.Len() {
			return fmt.Errorf("actual lookup index %d out of range", i)
		}
		el := exp.Lookups[i]
		lookup := gsub.LookupList.Navigate(i)
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
			sub := lookup.Subtable(j)
			if sub == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] missing", i, j)
			}
			if sub.Format != uint16(est.Format) {
				return fmt.Errorf("lookup[%d] subtable[%d] format mismatch: got %d, want %d",
					i, j, sub.Format, est.Format)
			}
			if LayoutTableLookupType(est.Type) != sub.LookupType {
				return fmt.Errorf("lookup[%d] subtable[%d] type mismatch: got %d, want %d",
					i, j, sub.LookupType, est.Type)
			}
			switch sub.LookupType {
			case GSubLookupTypeSingle:
				if err := compareSingleSubst(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeAlternate:
				if err := compareAlternateSubst(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeLigature:
				if err := compareLigatureSubst(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeContext:
				if err := compareContextSubst(sub, est); err != nil {
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
	if gsub.LookupList.Len() != len(exp.Lookups) {
		return fmt.Errorf("lookup count mismatch: got %d, want %d", gsub.LookupList.Len(), len(exp.Lookups))
	}
	for i, el := range exp.Lookups {
		lookup := gsub.LookupList.Navigate(i)
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
			sub := lookup.Subtable(j)
			if sub == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] missing", i, j)
			}
			if sub.Format != uint16(est.Format) {
				return fmt.Errorf("lookup[%d] subtable[%d] format mismatch: got %d, want %d",
					i, j, sub.Format, est.Format)
			}
			if LayoutTableLookupType(est.Type) != sub.LookupType {
				return fmt.Errorf("lookup[%d] subtable[%d] type mismatch: got %d, want %d",
					i, j, sub.LookupType, est.Type)
			}
			switch sub.LookupType {
			case GSubLookupTypeSingle:
				if err := compareSingleSubst(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeAlternate:
				if err := compareAlternateSubst(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeLigature:
				if err := compareLigatureSubst(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeContext:
				if err := compareContextSubst(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			}
		}
	}
	return nil
}

func compareSingleSubst(sub *LookupSubtable, est ttxtest.ExpectedSubtable) error {
	if len(est.SingleSubst) == 0 {
		return fmt.Errorf("expected single substitutions missing")
	}
	coverage, err := coverageGlyphs(sub.Coverage)
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
		switch sub.Format {
		case 1:
			delta, ok := sub.Support.(int16)
			if !ok {
				return fmt.Errorf("missing delta for format 1")
			}
			got := GlyphIndex(int(in) + int(delta))
			if got != out {
				return fmt.Errorf("substitution %q mismatch: got %d, want %d", name, got, out)
			}
		case 2:
			if got := lookupGlyphTest(sub.Index, i, false); got != out {
				return fmt.Errorf("substitution %q mismatch: got %d, want %d", name, got, out)
			}
		default:
			return fmt.Errorf("unsupported single subst format %d", sub.Format)
		}
	}
	return nil
}

func compareAlternateSubst(sub *LookupSubtable, est ttxtest.ExpectedSubtable) error {
	coverage, err := coverageGlyphs(sub.Coverage)
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
		actual, err := alternateSetGlyphs(sub.Index, i)
		if err != nil {
			return fmt.Errorf("alternate set %q: %w", name, err)
		}
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

func compareLigatureSubst(sub *LookupSubtable, est ttxtest.ExpectedSubtable) error {
	if len(est.Ligatures) == 0 {
		return fmt.Errorf("expected ligatures missing")
	}
	coverage, err := coverageGlyphs(sub.Coverage)
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
		actual, err := ligatureSet(sub.Index, i)
		if err != nil {
			return fmt.Errorf("ligature set %q: %w", name, err)
		}
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

func compareContextSubst(sub *LookupSubtable, est ttxtest.ExpectedSubtable) error {
	switch sub.Format {
	case 1:
		return compareContextSubstFmt1(sub, est)
	case 2:
		return compareContextSubstFmt2(sub, est)
	default:
		return fmt.Errorf("unsupported context subst format %d", sub.Format)
	}
}

func compareContextSubstFmt1(sub *LookupSubtable, est ttxtest.ExpectedSubtable) error {
	if est.ContextSubst == nil {
		return fmt.Errorf("expected context subst missing")
	}
	if sub.Format != 1 {
		return fmt.Errorf("unsupported context subst format %d", sub.Format)
	}
	coverage, err := coverageGlyphs(sub.Coverage)
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
		rules, err := parseSequenceRules(sub, i)
		if err != nil {
			return fmt.Errorf("rule set %d: %w", i, err)
		}
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

func compareContextSubstFmt2(sub *LookupSubtable, est ttxtest.ExpectedSubtable) error {
	if est.ContextSubst == nil {
		return fmt.Errorf("expected context subst missing")
	}
	if sub.Format != 2 {
		return fmt.Errorf("unsupported context subst format %d", sub.Format)
	}
	coverage, err := coverageGlyphs(sub.Coverage)
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
		seqctx, ok := sub.Support.(*SequenceContext)
		if !ok || seqctx == nil || len(seqctx.ClassDefs) == 0 {
			return fmt.Errorf("missing class definitions in subtable support")
		}
		for name, class := range est.ContextSubst.ClassDefs {
			gid, err := glyphNameToID(name)
			if err != nil {
				return fmt.Errorf("classdef glyph %q: %w", name, err)
			}
			if got := int(seqctx.ClassDefs[0].Lookup(gid)); got != class {
				return fmt.Errorf("classdef %q mismatch: got %d, want %d", name, got, class)
			}
		}
	}

	for i := range est.ContextSubst.ClassRuleSets {
		expRules := est.ContextSubst.ClassRuleSets[i].Rules
		rules, err := parseClassSequenceRules(sub, i)
		if err != nil {
			return fmt.Errorf("class rule set %d: %w", i, err)
		}
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

func parseSequenceRules(sub *LookupSubtable, coverageIndex int) ([]parsedSequenceRule, error) {
	if sub.Index.Size() == 0 {
		return nil, nil
	}
	ruleSetLoc, err := sub.Index.Get(coverageIndex, false)
	if err != nil {
		return nil, err
	}
	if ruleSetLoc.Size() < 2 {
		return nil, nil
	}
	ruleSet := ParseVarArray(ruleSetLoc, 0, 2, "SequenceRuleSet")
	out := make([]parsedSequenceRule, 0, ruleSet.Size())
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 4 {
			continue
		}
		seqrule := sub.SequenceRule(binarySegm(ruleLoc.Bytes()))
		input := make([]GlyphIndex, 0, seqrule.inputSequence.Len())
		for _, loc := range seqrule.inputSequence.Range() {
			input = append(input, GlyphIndex(loc.U16(0)))
		}
		records := make([]SequenceLookupRecord, 0, seqrule.lookupRecords.Len())
		for _, loc := range seqrule.lookupRecords.Range() {
			records = append(records, SequenceLookupRecord{
				SequenceIndex:   loc.U16(0),
				LookupListIndex: loc.U16(2),
			})
		}
		out = append(out, parsedSequenceRule{Input: input, Records: records})
	}
	return out, nil
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

func parseClassSequenceRules(sub *LookupSubtable, setIndex int) ([]parsedClassSequenceRule, error) {
	if sub.Index.Size() == 0 {
		return nil, nil
	}
	ruleSetLoc, err := sub.Index.Get(setIndex, false)
	if err != nil {
		return nil, err
	}
	if ruleSetLoc.Size() < 2 {
		return nil, nil
	}
	ruleSet := ParseVarArray(ruleSetLoc, 0, 2, "ClassSequenceRuleSet")
	out := make([]parsedClassSequenceRule, 0, ruleSet.Size())
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 4 {
			continue
		}
		glyphCount := int(ruleLoc.U16(0))
		seqLookupCount := int(ruleLoc.U16(2))
		if glyphCount < 1 {
			continue
		}
		classCount := glyphCount - 1
		classBytes := classCount * 2
		recBytes := seqLookupCount * 4
		minSize := 4 + classBytes + recBytes
		if ruleLoc.Size() < minSize {
			continue
		}
		classes := make([]uint16, classCount)
		for j := 0; j < classCount; j++ {
			classes[j] = ruleLoc.U16(4 + j*2)
		}
		records := make([]SequenceLookupRecord, seqLookupCount)
		recStart := 4 + classBytes
		for r := 0; r < seqLookupCount; r++ {
			off := recStart + r*4
			records[r] = SequenceLookupRecord{
				SequenceIndex:   ruleLoc.U16(off),
				LookupListIndex: ruleLoc.U16(off + 2),
			}
		}
		out = append(out, parsedClassSequenceRule{Classes: classes, Records: records})
	}
	return out, nil
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

func ligatureSet(index VarArray, setIndex int) ([]parsedLigature, error) {
	loc, err := index.Get(setIndex, false)
	if err != nil {
		return nil, err
	}
	if loc.Size() < 2 {
		return nil, fmt.Errorf("ligature set too small")
	}
	cnt := int(loc.U16(0))
	need := 2 + cnt*2
	if loc.Size() < need {
		return nil, fmt.Errorf("ligature set size %d < %d", loc.Size(), need)
	}
	out := make([]parsedLigature, 0, cnt)
	for i := 0; i < cnt; i++ {
		off := loc.U16(2 + i*2)
		if off == 0 || int(off) >= loc.Size() {
			return nil, fmt.Errorf("ligature offset %d out of bounds", off)
		}
		l := binarySegm(loc.Bytes()[off:])
		if l.Size() < 4 {
			return nil, fmt.Errorf("ligature table too small")
		}
		glyph := GlyphIndex(l.U16(0))
		compCount := int(l.U16(2))
		if compCount < 1 {
			return nil, fmt.Errorf("ligature component count %d", compCount)
		}
		needComp := 4 + (compCount-1)*2
		if l.Size() < needComp {
			return nil, fmt.Errorf("ligature table size %d < %d", l.Size(), needComp)
		}
		comps := make([]GlyphIndex, 0, compCount-1)
		for j := 0; j < compCount-1; j++ {
			comps = append(comps, GlyphIndex(l.U16(4+j*2)))
		}
		out = append(out, parsedLigature{Components: comps, Glyph: glyph})
	}
	return out, nil
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

func alternateSetGlyphs(index VarArray, setIndex int) ([]GlyphIndex, error) {
	loc, err := index.Get(setIndex, false)
	if err != nil {
		return nil, err
	}
	if loc.Size() < 2 {
		return nil, fmt.Errorf("alternate set too small")
	}
	cnt := int(loc.U16(0))
	need := 2 + cnt*2
	if loc.Size() < need {
		return nil, fmt.Errorf("alternate set size %d < %d", loc.Size(), need)
	}
	seg := binarySegm(loc.Bytes()[2:need])
	return seg.Glyphs(), nil
}

func lookupGlyphTest(index VarArray, ginx int, deep bool) GlyphIndex {
	outglyph, err := index.Get(ginx, deep)
	if err != nil {
		return 0
	}
	return GlyphIndex(outglyph.U16(0))
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
