package ot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/npillmayer/opentype/internal/ttxtest"
)

func TestTTXGSUBConcreteGolden(t *testing.T) {
	cases := []struct {
		name    string
		font    string
		ttx     string
		lookups []int
	}{
		{"alternate-simple", "gsub3_1_simple_f1.otf", "gsub3_1_simple_f1.ttx.GSUB", nil},
		{"alternate-lookupflag", "gsub3_1_lookupflag_f1.otf", "gsub3_1_lookupflag_f1.ttx.GSUB", nil},
		{"single-ligature", "gsub_chaining2_next_glyph_f1.otf", "gsub_chaining2_next_glyph_f1.ttx.GSUB", []int{0, 1}},
		{"ligature-ignore-marks", "gsub_chaining2_next_glyph_f1.otf", "gsub_chaining2_next_glyph_f1.ttx.GSUB", []int{2}},
		{"context-fmt1-lookupflag", "gsub_context1_lookupflag_f1.otf", "gsub_context1_lookupflag_f1.ttx.GSUB", []int{4}},
		{"context-fmt1-next-glyph", "gsub_context1_next_glyph_f1.otf", "gsub_context1_next_glyph_f1.ttx.GSUB", []int{4}},
		{"context-fmt2-classdef2", "classdef2_font4.otf", "classdef2_font4.ttx.GSUB", []int{3}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fontPath := filepath.Join("..", "testdata", "fonttools", tc.font)
			ttxPath := filepath.Join("..", "testdata", "fonttools", tc.ttx)
			data, err := os.ReadFile(fontPath)
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
			indices := tc.lookups
			if len(indices) == 0 {
				indices = make([]int, len(exp.Lookups))
				for i := range len(exp.Lookups) {
					indices[i] = i
				}
			}
			if err := compareExpectedGSUBLookupsConcrete(gsub, exp, indices); err != nil {
				t.Fatalf("GSUB concrete compare failed: %v", err)
			}
		})
	}
}

func TestTTXGPOSConcreteGolden(t *testing.T) {
	cases := []struct {
		name    string
		font    string
		ttx     string
		lookups []int
	}{
		{"single-pos-fmt1", "gpos_chaining3_boundary_f2.otf", "gpos_chaining3_boundary_f2.ttx.GPOS", []int{0}},
		{"pair-pos-fmt1", "gpos_chaining3_boundary_f2.otf", "gpos_chaining3_boundary_f2.ttx.GPOS", []int{1}},
		{"chain-context-fmt3", "gpos_chaining3_boundary_f2.otf", "gpos_chaining3_boundary_f2.ttx.GPOS", []int{4}},
		{"mark-base-fmt1", "gpos4_simple_1.otf", "gpos4_simple_1.ttx.GPOS", []int{0}},
		{"mark-lig-fmt1", "gpos5_font1.otf", "gpos5_font1.ttx.GPOS", []int{0}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fontPath := filepath.Join("..", "testdata", "fonttools", tc.font)
			ttxPath := filepath.Join("..", "testdata", "fonttools", tc.ttx)
			data, err := os.ReadFile(fontPath)
			if err != nil {
				t.Fatalf("read font: %v", err)
			}
			otf, err := Parse(data, IsTestfont)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			table := otf.Table(T("GPOS"))
			if table == nil {
				t.Fatalf("font missing GPOS table")
			}
			gpos := table.Self().AsGPos()
			if gpos == nil {
				t.Fatalf("cannot convert GPOS table")
			}
			exp, err := ttxtest.ParseTTXGPOS(ttxPath)
			if err != nil {
				t.Fatalf("ParseTTXGPOS: %v", err)
			}
			if err := compareExpectedGPOSLookupsConcrete(gpos, exp, tc.lookups); err != nil {
				t.Fatalf("GPOS concrete compare failed: %v", err)
			}
		})
	}
}

func compareExpectedGSUBLookupsConcrete(gsub *GSubTable, exp *ttxtest.ExpectedGSUB, indices []int) error {
	if gsub == nil {
		return fmt.Errorf("nil GSUB")
	}
	if exp == nil {
		return fmt.Errorf("nil expected GSUB")
	}
	graph := gsub.LookupGraph()
	if graph == nil {
		return fmt.Errorf("nil concrete GSUB lookup graph")
	}
	for _, i := range indices {
		if i < 0 || i >= len(exp.Lookups) {
			return fmt.Errorf("expected lookup index %d out of range", i)
		}
		if i < 0 || i >= graph.Len() {
			return fmt.Errorf("concrete lookup index %d out of range", i)
		}
		el := exp.Lookups[i]
		lookup := graph.Lookup(i)
		if lookup == nil {
			return fmt.Errorf("lookup[%d] missing", i)
		}
		if lookup.Error() != nil {
			return fmt.Errorf("lookup[%d] parse error: %w", i, lookup.Error())
		}
		if int(lookup.Type) != el.Type {
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
			if sub.Error() != nil {
				return fmt.Errorf("lookup[%d] subtable[%d] parse error: %w", i, j, sub.Error())
			}
			if sub.Format != uint16(est.Format) {
				return fmt.Errorf("lookup[%d] subtable[%d] format mismatch: got %d, want %d",
					i, j, sub.Format, est.Format)
			}
			if int(sub.LookupType) != est.Type {
				return fmt.Errorf("lookup[%d] subtable[%d] type mismatch: got %d, want %d",
					i, j, sub.LookupType, est.Type)
			}
			switch sub.LookupType {
			case GSubLookupTypeSingle:
				if err := compareSingleSubstConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeAlternate:
				if err := compareAlternateSubstConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeLigature:
				if err := compareLigatureSubstConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GSubLookupTypeContext:
				if err := compareContextSubstConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			}
		}
	}
	return nil
}

func compareSingleSubstConcrete(sub *LookupNode, est ttxtest.ExpectedSubtable) error {
	p := sub.GSubPayload()
	if p == nil {
		return fmt.Errorf("missing GSUB payload")
	}
	coverage, err := coverageGlyphs(sub.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Coverage) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Coverage))
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
			if p.SingleFmt1 == nil {
				return fmt.Errorf("missing single fmt1 payload")
			}
			got := GlyphIndex(int(in) + int(p.SingleFmt1.DeltaGlyphID))
			if got != out {
				return fmt.Errorf("substitution %q mismatch: got %d, want %d", name, got, out)
			}
		case 2:
			if p.SingleFmt2 == nil {
				return fmt.Errorf("missing single fmt2 payload")
			}
			if i >= len(p.SingleFmt2.SubstituteGlyphIDs) {
				return fmt.Errorf("substitute index %d out of range", i)
			}
			if p.SingleFmt2.SubstituteGlyphIDs[i] != out {
				return fmt.Errorf("substitution %q mismatch: got %d, want %d", name, p.SingleFmt2.SubstituteGlyphIDs[i], out)
			}
		default:
			return fmt.Errorf("unsupported single subst format %d", sub.Format)
		}
	}
	return nil
}

func compareAlternateSubstConcrete(sub *LookupNode, est ttxtest.ExpectedSubtable) error {
	p := sub.GSubPayload()
	if p == nil || p.AlternateFmt1 == nil {
		return fmt.Errorf("missing alternate fmt1 payload")
	}
	coverage, err := coverageGlyphs(sub.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Coverage) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Coverage))
	}
	if len(p.AlternateFmt1.Alternates) != len(est.Coverage) {
		return fmt.Errorf("alternate set count mismatch: got %d, want %d", len(p.AlternateFmt1.Alternates), len(est.Coverage))
	}
	for i, name := range est.Coverage {
		gid, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != gid {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], gid)
		}
		expNames := est.Alternates[name]
		actual := p.AlternateFmt1.Alternates[i]
		if len(actual) != len(expNames) {
			return fmt.Errorf("alternate set %q length mismatch: got %d, want %d", name, len(actual), len(expNames))
		}
		for k, altName := range expNames {
			altID, err := glyphNameToID(altName)
			if err != nil {
				return fmt.Errorf("alternate glyph %q: %w", altName, err)
			}
			if actual[k] != altID {
				return fmt.Errorf("alternate set %q[%d] mismatch: got %d, want %d", name, k, actual[k], altID)
			}
		}
	}
	return nil
}

func compareLigatureSubstConcrete(sub *LookupNode, est ttxtest.ExpectedSubtable) error {
	p := sub.GSubPayload()
	if p == nil || p.LigatureFmt1 == nil {
		return fmt.Errorf("missing ligature fmt1 payload")
	}
	coverage, err := coverageGlyphs(sub.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Coverage) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Coverage))
	}
	if len(p.LigatureFmt1.LigatureSets) != len(est.Coverage) {
		return fmt.Errorf("ligature set count mismatch: got %d, want %d", len(p.LigatureFmt1.LigatureSets), len(est.Coverage))
	}
	for i, name := range est.Coverage {
		first, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != first {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], first)
		}
		expLigatures := est.Ligatures[name]
		actual := p.LigatureFmt1.LigatureSets[i]
		if len(actual) != len(expLigatures) {
			return fmt.Errorf("ligature set %q count mismatch: got %d, want %d", name, len(actual), len(expLigatures))
		}
		for k, expLig := range expLigatures {
			if len(actual[k].Components) != len(expLig.Components) {
				return fmt.Errorf("ligature %q[%d] component count mismatch: got %d, want %d",
					name, k, len(actual[k].Components), len(expLig.Components))
			}
			for m, compName := range expLig.Components {
				compID, err := glyphNameToID(compName)
				if err != nil {
					return fmt.Errorf("ligature component %q: %w", compName, err)
				}
				if actual[k].Components[m] != compID {
					return fmt.Errorf("ligature %q[%d] component[%d] mismatch: got %d, want %d",
						name, k, m, actual[k].Components[m], compID)
				}
			}
			glyphID, err := glyphNameToID(expLig.Glyph)
			if err != nil {
				return fmt.Errorf("ligature glyph %q: %w", expLig.Glyph, err)
			}
			if actual[k].Ligature != glyphID {
				return fmt.Errorf("ligature %q[%d] glyph mismatch: got %d, want %d", name, k, actual[k].Ligature, glyphID)
			}
		}
	}
	return nil
}

func compareContextSubstConcrete(sub *LookupNode, est ttxtest.ExpectedSubtable) error {
	p := sub.GSubPayload()
	if p == nil || est.ContextSubst == nil {
		return fmt.Errorf("missing context payload/expectation")
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
	switch sub.Format {
	case 1:
		if p.ContextFmt1 == nil {
			return fmt.Errorf("missing context fmt1 payload")
		}
		if len(p.ContextFmt1.RuleSets) != len(est.ContextSubst.RuleSets) {
			return fmt.Errorf("context rule-set count mismatch: got %d, want %d",
				len(p.ContextFmt1.RuleSets), len(est.ContextSubst.RuleSets))
		}
		for i, expSet := range est.ContextSubst.RuleSets {
			actual := p.ContextFmt1.RuleSets[i]
			if len(actual) != len(expSet.Rules) {
				return fmt.Errorf("rule set %d count mismatch: got %d, want %d", i, len(actual), len(expSet.Rules))
			}
			for r, expRule := range expSet.Rules {
				if len(actual[r].InputGlyphs) != len(expRule.Input) {
					return fmt.Errorf("rule set %d rule %d input count mismatch: got %d, want %d",
						i, r, len(actual[r].InputGlyphs), len(expRule.Input))
				}
				for k, name := range expRule.Input {
					gid, err := glyphNameToID(name)
					if err != nil {
						return fmt.Errorf("rule set %d rule %d input[%d] %q: %w", i, r, k, name, err)
					}
					if actual[r].InputGlyphs[k] != gid {
						return fmt.Errorf("rule set %d rule %d input[%d] mismatch: got %d, want %d",
							i, r, k, actual[r].InputGlyphs[k], gid)
					}
				}
				if err := compareExpectedLookupRecords(actual[r].Records, expRule.LookupRecords); err != nil {
					return fmt.Errorf("rule set %d rule %d lookup records: %w", i, r, err)
				}
			}
		}
	case 2:
		if p.ContextFmt2 == nil {
			return fmt.Errorf("missing context fmt2 payload")
		}
		if len(est.ContextSubst.ClassDefs) > 0 {
			for name, clz := range est.ContextSubst.ClassDefs {
				gid, err := glyphNameToID(name)
				if err != nil {
					return fmt.Errorf("classdef glyph %q: %w", name, err)
				}
				if got := int(p.ContextFmt2.ClassDef.Lookup(gid)); got != clz {
					return fmt.Errorf("classdef %q mismatch: got %d, want %d", name, got, clz)
				}
			}
		}
		if len(p.ContextFmt2.RuleSets) != len(est.ContextSubst.ClassRuleSets) {
			return fmt.Errorf("class rule-set count mismatch: got %d, want %d",
				len(p.ContextFmt2.RuleSets), len(est.ContextSubst.ClassRuleSets))
		}
		for i, expSet := range est.ContextSubst.ClassRuleSets {
			actual := p.ContextFmt2.RuleSets[i]
			if len(actual) != len(expSet.Rules) {
				return fmt.Errorf("class rule set %d count mismatch: got %d, want %d", i, len(actual), len(expSet.Rules))
			}
			for r, expRule := range expSet.Rules {
				if len(actual[r].InputClasses) != len(expRule.Classes) {
					return fmt.Errorf("class rule set %d rule %d class count mismatch: got %d, want %d",
						i, r, len(actual[r].InputClasses), len(expRule.Classes))
				}
				for k, clz := range expRule.Classes {
					if actual[r].InputClasses[k] != uint16(clz) {
						return fmt.Errorf("class rule set %d rule %d class[%d] mismatch: got %d, want %d",
							i, r, k, actual[r].InputClasses[k], clz)
					}
				}
				if err := compareExpectedLookupRecords(actual[r].Records, expRule.LookupRecords); err != nil {
					return fmt.Errorf("class rule set %d rule %d lookup records: %w", i, r, err)
				}
			}
		}
	default:
		return fmt.Errorf("unsupported context format %d", sub.Format)
	}
	return nil
}

func compareExpectedGPOSLookupsConcrete(gpos *GPosTable, exp *ttxtest.ExpectedGPOS, indices []int) error {
	if gpos == nil {
		return fmt.Errorf("nil GPOS")
	}
	if exp == nil {
		return fmt.Errorf("nil expected GPOS")
	}
	graph := gpos.LookupGraph()
	if graph == nil {
		return fmt.Errorf("nil concrete GPOS lookup graph")
	}
	for _, i := range indices {
		if i < 0 || i >= len(exp.Lookups) {
			return fmt.Errorf("expected lookup index %d out of range", i)
		}
		if i < 0 || i >= graph.Len() {
			return fmt.Errorf("concrete lookup index %d out of range", i)
		}
		el := exp.Lookups[i]
		lookup := graph.Lookup(i)
		if lookup == nil {
			return fmt.Errorf("lookup[%d] missing", i)
		}
		if lookup.Error() != nil {
			return fmt.Errorf("lookup[%d] parse error: %w", i, lookup.Error())
		}
		if int(GPosLookupType(lookup.Type)) != el.Type {
			return fmt.Errorf("lookup[%d] type mismatch: got %d, want %d", i, GPosLookupType(lookup.Type), el.Type)
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
			if sub.Error() != nil {
				return fmt.Errorf("lookup[%d] subtable[%d] parse error: %w", i, j, sub.Error())
			}
			if sub.Format != uint16(est.Format) {
				return fmt.Errorf("lookup[%d] subtable[%d] format mismatch: got %d, want %d",
					i, j, sub.Format, est.Format)
			}
			if int(GPosLookupType(sub.LookupType)) != est.Type {
				return fmt.Errorf("lookup[%d] subtable[%d] type mismatch: got %d, want %d",
					i, j, GPosLookupType(sub.LookupType), est.Type)
			}
			switch GPosLookupType(sub.LookupType) {
			case GPosLookupTypeSingle:
				if err := compareSinglePosConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypePair:
				if err := comparePairPosConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypeMarkToBase:
				if err := compareMarkBasePosConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypeMarkToLigature:
				if err := compareMarkLigPosConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypeChainedContextPos:
				if err := compareChainContextPosConcrete(sub, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			}
		}
	}
	return nil
}

func compareSinglePosConcrete(sub *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	p := sub.GPosPayload()
	if p == nil || p.SingleFmt1 == nil {
		return fmt.Errorf("missing single pos fmt1 payload")
	}
	coverage, err := coverageGlyphs(sub.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if err := compareCoverageNames(coverage, est.Coverage); err != nil {
		return err
	}
	if uint16(p.SingleFmt1.ValueFormat) != est.ValueFormat {
		return fmt.Errorf("value format mismatch: got %d, want %d", p.SingleFmt1.ValueFormat, est.ValueFormat)
	}
	if err := compareValueRecord(p.SingleFmt1.ValueFormat, p.SingleFmt1.Value, est.Value); err != nil {
		return fmt.Errorf("value record: %w", err)
	}
	return nil
}

func comparePairPosConcrete(sub *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	p := sub.GPosPayload()
	if p == nil || p.PairFmt1 == nil {
		return fmt.Errorf("missing pair pos fmt1 payload")
	}
	coverage, err := coverageGlyphs(sub.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if err := compareCoverageNames(coverage, est.Coverage); err != nil {
		return err
	}
	if uint16(p.PairFmt1.ValueFormat1) != est.ValueFormat1 || uint16(p.PairFmt1.ValueFormat2) != est.ValueFormat2 {
		return fmt.Errorf("value format mismatch: got %d/%d, want %d/%d",
			p.PairFmt1.ValueFormat1, p.PairFmt1.ValueFormat2, est.ValueFormat1, est.ValueFormat2)
	}
	if len(p.PairFmt1.PairSets) != len(est.Coverage) {
		return fmt.Errorf("pair set count mismatch: got %d, want %d", len(p.PairFmt1.PairSets), len(est.Coverage))
	}
	for i, name := range est.Coverage {
		expPairs := est.PairValues[name]
		actualPairs := p.PairFmt1.PairSets[i]
		if len(actualPairs) != len(expPairs) {
			return fmt.Errorf("pair set %q length mismatch: got %d, want %d", name, len(actualPairs), len(expPairs))
		}
		for j, expPair := range expPairs {
			sec, err := glyphNameToID(expPair.SecondGlyph)
			if err != nil {
				return fmt.Errorf("pair set %q second glyph %q: %w", name, expPair.SecondGlyph, err)
			}
			if GlyphIndex(actualPairs[j].SecondGlyph) != sec {
				return fmt.Errorf("pair set %q[%d] second glyph mismatch: got %d, want %d",
					name, j, actualPairs[j].SecondGlyph, sec)
			}
			if err := compareValueRecord(p.PairFmt1.ValueFormat1, actualPairs[j].Value1, expPair.Value1); err != nil {
				return fmt.Errorf("pair set %q[%d] value1: %w", name, j, err)
			}
			if err := compareValueRecord(p.PairFmt1.ValueFormat2, actualPairs[j].Value2, expPair.Value2); err != nil {
				return fmt.Errorf("pair set %q[%d] value2: %w", name, j, err)
			}
		}
	}
	return nil
}

func compareChainContextPosConcrete(sub *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	p := sub.GPosPayload()
	if p == nil || p.ChainingContextFmt3 == nil {
		return fmt.Errorf("missing chain-context-pos fmt3 payload")
	}
	if err := compareCoverageSeq(p.ChainingContextFmt3.BacktrackCoverages, est.BacktrackCoverage); err != nil {
		return fmt.Errorf("backtrack coverage: %w", err)
	}
	if err := compareCoverageSeq(p.ChainingContextFmt3.InputCoverages, est.InputCoverage); err != nil {
		return fmt.Errorf("input coverage: %w", err)
	}
	if err := compareCoverageSeq(p.ChainingContextFmt3.LookaheadCoverages, est.LookAheadCoverage); err != nil {
		return fmt.Errorf("lookahead coverage: %w", err)
	}
	if len(p.ChainingContextFmt3.Records) != len(est.PosLookupRecords) {
		return fmt.Errorf("lookup record count mismatch: got %d, want %d", len(p.ChainingContextFmt3.Records), len(est.PosLookupRecords))
	}
	for i := range est.PosLookupRecords {
		expRec := est.PosLookupRecords[i]
		gotRec := p.ChainingContextFmt3.Records[i]
		if int(gotRec.SequenceIndex) != expRec.SequenceIndex || int(gotRec.LookupListIndex) != expRec.LookupListIndex {
			return fmt.Errorf("lookup record[%d] mismatch: got (%d,%d), want (%d,%d)",
				i, gotRec.SequenceIndex, gotRec.LookupListIndex, expRec.SequenceIndex, expRec.LookupListIndex)
		}
	}
	return nil
}

func compareMarkBasePosConcrete(sub *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	p := sub.GPosPayload()
	if p == nil || p.MarkToBaseFmt1 == nil {
		return fmt.Errorf("missing mark-to-base fmt1 payload")
	}
	markCov, err := coverageGlyphs(sub.Coverage)
	if err != nil {
		return fmt.Errorf("mark coverage parse: %w", err)
	}
	if err := compareCoverageNames(markCov, est.MarkCoverage); err != nil {
		return fmt.Errorf("mark coverage: %w", err)
	}
	baseCov, err := coverageGlyphs(p.MarkToBaseFmt1.BaseCoverage)
	if err != nil {
		return fmt.Errorf("base coverage parse: %w", err)
	}
	if err := compareCoverageNames(baseCov, est.BaseCoverage); err != nil {
		return fmt.Errorf("base coverage: %w", err)
	}
	if int(p.MarkToBaseFmt1.MarkClassCount) != est.MarkClassCount {
		return fmt.Errorf("mark class count mismatch: got %d, want %d", p.MarkToBaseFmt1.MarkClassCount, est.MarkClassCount)
	}
	if len(p.MarkToBaseFmt1.MarkRecords) != len(est.MarkAnchors) {
		return fmt.Errorf("mark record count mismatch: got %d, want %d", len(p.MarkToBaseFmt1.MarkRecords), len(est.MarkAnchors))
	}
	for i, expMark := range est.MarkAnchors {
		rec := p.MarkToBaseFmt1.MarkRecords[i]
		if int(rec.Class) != expMark.Class {
			return fmt.Errorf("mark record[%d] class mismatch: got %d, want %d", i, rec.Class, expMark.Class)
		}
		if rec.Anchor == nil {
			return fmt.Errorf("mark record[%d] anchor missing", i)
		}
		if err := compareAnchor(*rec.Anchor, expMark.Anchor); err != nil {
			return fmt.Errorf("mark record[%d] anchor: %w", i, err)
		}
	}
	if len(p.MarkToBaseFmt1.BaseRecords) != len(est.BaseAnchors) {
		return fmt.Errorf("base record count mismatch: got %d, want %d", len(p.MarkToBaseFmt1.BaseRecords), len(est.BaseAnchors))
	}
	for i := range est.BaseAnchors {
		actualAnchors := p.MarkToBaseFmt1.BaseRecords[i].Anchors
		expAnchors := est.BaseAnchors[i]
		if len(actualAnchors) != len(expAnchors) {
			return fmt.Errorf("base record[%d] anchor count mismatch: got %d, want %d", i, len(actualAnchors), len(expAnchors))
		}
		for j := range expAnchors {
			if actualAnchors[j] == nil {
				return fmt.Errorf("base record[%d] anchor[%d] missing", i, j)
			}
			if err := compareAnchor(*actualAnchors[j], expAnchors[j]); err != nil {
				return fmt.Errorf("base record[%d] anchor[%d]: %w", i, j, err)
			}
		}
	}
	return nil
}

func compareMarkLigPosConcrete(sub *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	p := sub.GPosPayload()
	if p == nil || p.MarkToLigatureFmt1 == nil {
		return fmt.Errorf("missing mark-to-ligature fmt1 payload")
	}
	markCov, err := coverageGlyphs(sub.Coverage)
	if err != nil {
		return fmt.Errorf("mark coverage parse: %w", err)
	}
	if err := compareCoverageNames(markCov, est.MarkCoverage); err != nil {
		return fmt.Errorf("mark coverage: %w", err)
	}
	ligCov, err := coverageGlyphs(p.MarkToLigatureFmt1.LigatureCoverage)
	if err != nil {
		return fmt.Errorf("ligature coverage parse: %w", err)
	}
	if err := compareCoverageNames(ligCov, est.LigatureCoverage); err != nil {
		return fmt.Errorf("ligature coverage: %w", err)
	}
	if int(p.MarkToLigatureFmt1.MarkClassCount) != est.MarkClassCount {
		return fmt.Errorf("mark class count mismatch: got %d, want %d", p.MarkToLigatureFmt1.MarkClassCount, est.MarkClassCount)
	}
	if len(p.MarkToLigatureFmt1.MarkRecords) != len(est.MarkAnchors) {
		return fmt.Errorf("mark record count mismatch: got %d, want %d", len(p.MarkToLigatureFmt1.MarkRecords), len(est.MarkAnchors))
	}
	for i, expMark := range est.MarkAnchors {
		rec := p.MarkToLigatureFmt1.MarkRecords[i]
		if int(rec.Class) != expMark.Class {
			return fmt.Errorf("mark record[%d] class mismatch: got %d, want %d", i, rec.Class, expMark.Class)
		}
		if rec.Anchor == nil {
			return fmt.Errorf("mark record[%d] anchor missing", i)
		}
		if err := compareAnchor(*rec.Anchor, expMark.Anchor); err != nil {
			return fmt.Errorf("mark record[%d] anchor: %w", i, err)
		}
	}
	if len(p.MarkToLigatureFmt1.LigatureRecords) != len(est.LigatureAnchors) {
		return fmt.Errorf("ligature record count mismatch: got %d, want %d", len(p.MarkToLigatureFmt1.LigatureRecords), len(est.LigatureAnchors))
	}
	for i := range est.LigatureAnchors {
		actualComps := p.MarkToLigatureFmt1.LigatureRecords[i].ComponentAnchors
		expComps := est.LigatureAnchors[i]
		if len(actualComps) != len(expComps) {
			return fmt.Errorf("ligature[%d] component count mismatch: got %d, want %d", i, len(actualComps), len(expComps))
		}
		for j := range expComps {
			if len(actualComps[j]) != len(expComps[j]) {
				return fmt.Errorf("ligature[%d] component[%d] anchor count mismatch: got %d, want %d",
					i, j, len(actualComps[j]), len(expComps[j]))
			}
			for k := range expComps[j] {
				if actualComps[j][k] == nil {
					return fmt.Errorf("ligature[%d] component[%d] anchor[%d] missing", i, j, k)
				}
				if err := compareAnchor(*actualComps[j][k], expComps[j][k]); err != nil {
					return fmt.Errorf("ligature[%d] component[%d] anchor[%d]: %w", i, j, k, err)
				}
			}
		}
	}
	return nil
}

func compareExpectedLookupRecords(actual []SequenceLookupRecord, exp []ttxtest.ExpectedSequenceLookupRecord) error {
	if len(actual) != len(exp) {
		return fmt.Errorf("lookup record count mismatch: got %d, want %d", len(actual), len(exp))
	}
	for i, rec := range exp {
		if int(actual[i].SequenceIndex) != rec.SequenceIndex {
			return fmt.Errorf("lookup record[%d] sequence index mismatch: got %d, want %d",
				i, actual[i].SequenceIndex, rec.SequenceIndex)
		}
		if int(actual[i].LookupListIndex) != rec.LookupListIndex {
			return fmt.Errorf("lookup record[%d] list index mismatch: got %d, want %d",
				i, actual[i].LookupListIndex, rec.LookupListIndex)
		}
	}
	return nil
}
