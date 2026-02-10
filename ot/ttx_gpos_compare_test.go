package ot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/npillmayer/opentype/internal/ttxtest"
)

func TestTTXGPOSStructural_SinglePosFmt1(t *testing.T) {
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos_chaining3_boundary_f2.ttx.GPOS")
	fontPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos_chaining3_boundary_f2.otf")
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
	if err := compareExpectedGPOSLookups(gpos, exp, []int{0}); err != nil {
		t.Fatalf("GPOS compare failed: %v", err)
	}
}

func TestTTXGPOSStructural_PairPosFmt1(t *testing.T) {
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos_chaining3_boundary_f2.ttx.GPOS")
	fontPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos_chaining3_boundary_f2.otf")
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
	if err := compareExpectedGPOSLookups(gpos, exp, []int{1}); err != nil {
		t.Fatalf("GPOS compare failed: %v", err)
	}
}

func TestTTXGPOSStructural_ChainContextPosFmt3(t *testing.T) {
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos_chaining3_boundary_f2.ttx.GPOS")
	fontPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos_chaining3_boundary_f2.otf")
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
	if err := compareExpectedGPOSLookups(gpos, exp, []int{4}); err != nil {
		t.Fatalf("GPOS compare failed: %v", err)
	}
}

func TestTTXGPOSStructural_MarkBasePosFmt1(t *testing.T) {
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos4_simple_1.ttx.GPOS")
	fontPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos4_simple_1.otf")
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
	if err := compareExpectedGPOSLookups(gpos, exp, []int{0}); err != nil {
		t.Fatalf("GPOS compare failed: %v", err)
	}
}

func TestTTXGPOSStructural_MarkLigPosFmt1(t *testing.T) {
	ttxPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos5_font1.ttx.GPOS")
	fontPath := filepath.Join("..", "testdata", "fonttools-tests", "gpos5_font1.otf")
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
	if err := compareExpectedGPOSLookups(gpos, exp, []int{0}); err != nil {
		t.Fatalf("GPOS compare failed: %v", err)
	}
}

func compareExpectedGPOSLookups(gpos *GPosTable, exp *ttxtest.ExpectedGPOS, indices []int) error {
	if gpos == nil {
		return fmt.Errorf("nil GPOS")
	}
	if exp == nil {
		return fmt.Errorf("nil expected GPOS")
	}
	graph := gpos.LookupGraph()
	if graph == nil {
		return fmt.Errorf("nil GPOS lookup graph")
	}
	for _, i := range indices {
		if i >= graph.Len() {
			return fmt.Errorf("lookup index %d out of range (GPOS has %d)", i, graph.Len())
		}
		if i >= len(exp.Lookups) {
			return fmt.Errorf("lookup index %d out of range (expected has %d)", i, len(exp.Lookups))
		}
		el := exp.Lookups[i]
		lookup := graph.Lookup(i)
		if lookup == nil {
			return fmt.Errorf("lookup[%d] missing", i)
		}
		if GPosLookupType(lookup.Type) != LayoutTableLookupType(el.Type) {
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
			node := lookup.Subtable(j)
			if node == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] missing", i, j)
			}
			enode := effectiveGPosNode(node)
			if enode == nil {
				return fmt.Errorf("lookup[%d] subtable[%d] effective node missing", i, j)
			}
			if enode.Format != uint16(est.Format) {
				return fmt.Errorf("lookup[%d] subtable[%d] format mismatch: got %d, want %d",
					i, j, enode.Format, est.Format)
			}
			if GPosLookupType(enode.LookupType) != LayoutTableLookupType(est.Type) {
				return fmt.Errorf("lookup[%d] subtable[%d] type mismatch: got %d, want %d",
					i, j, GPosLookupType(enode.LookupType), est.Type)
			}
			switch GPosLookupType(enode.LookupType) {
			case GPosLookupTypeSingle:
				if err := compareSinglePos(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypePair:
				if err := comparePairPos(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypeMarkToBase:
				if err := compareMarkBasePos(lookup, j, enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypeMarkToLigature:
				if err := compareMarkLigPos(lookup, j, enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			case GPosLookupTypeChainedContextPos:
				if err := compareChainContextPos(enode, est); err != nil {
					return fmt.Errorf("lookup[%d] subtable[%d]: %w", i, j, err)
				}
			}
		}
	}
	return nil
}

func effectiveGPosNode(node *LookupNode) *LookupNode {
	if node == nil {
		return nil
	}
	payload := node.GPosPayload()
	if payload != nil && payload.ExtensionFmt1 != nil && payload.ExtensionFmt1.Resolved != nil {
		return payload.ExtensionFmt1.Resolved
	}
	return node
}

func compareSinglePos(node *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	payload := node.GPosPayload()
	if payload == nil {
		return fmt.Errorf("missing GPOS payload")
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
	switch node.Format {
	case 1:
		if payload.SingleFmt1 == nil {
			return fmt.Errorf("missing single pos support")
		}
		if uint16(payload.SingleFmt1.ValueFormat) != est.ValueFormat {
			return fmt.Errorf("value format mismatch: got %d, want %d", payload.SingleFmt1.ValueFormat, est.ValueFormat)
		}
		if err := compareValueRecord(ValueFormat(est.ValueFormat), payload.SingleFmt1.Value, est.Value); err != nil {
			return fmt.Errorf("value record: %w", err)
		}
	default:
		return fmt.Errorf("unsupported single pos format %d", node.Format)
	}
	return nil
}

func comparePairPos(node *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	if node.Format != 1 {
		return fmt.Errorf("unsupported pair pos format %d", node.Format)
	}
	payload := node.GPosPayload()
	if payload == nil || payload.PairFmt1 == nil {
		return fmt.Errorf("missing pair pos payload")
	}
	formats := [2]ValueFormat{payload.PairFmt1.ValueFormat1, payload.PairFmt1.ValueFormat2}
	if uint16(formats[0]) != est.ValueFormat1 || uint16(formats[1]) != est.ValueFormat2 {
		return fmt.Errorf("value format mismatch: got %d/%d, want %d/%d",
			formats[0], formats[1], est.ValueFormat1, est.ValueFormat2)
	}
	coverage, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("coverage parse: %w", err)
	}
	if len(coverage) != len(est.Coverage) {
		return fmt.Errorf("coverage length mismatch: got %d, want %d", len(coverage), len(est.Coverage))
	}
	for i, name := range est.Coverage {
		first, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("coverage glyph %q: %w", name, err)
		}
		if coverage[i] != first {
			return fmt.Errorf("coverage[%d] mismatch: got %d, want %d", i, coverage[i], first)
		}
		expPairs := est.PairValues[name]
		if i >= len(payload.PairFmt1.PairSets) {
			return fmt.Errorf("pair set %q missing", name)
		}
		actualPairs := payload.PairFmt1.PairSets[i]
		if len(actualPairs) != len(expPairs) {
			return fmt.Errorf("pair set %q length mismatch: got %d, want %d", name, len(actualPairs), len(expPairs))
		}
		for j, exp := range expPairs {
			sec, err := glyphNameToID(exp.SecondGlyph)
			if err != nil {
				return fmt.Errorf("pair set %q second glyph %q: %w", name, exp.SecondGlyph, err)
			}
			if actualPairs[j].SecondGlyph != uint16(sec) {
				return fmt.Errorf("pair set %q[%d] second glyph mismatch: got %d, want %d",
					name, j, actualPairs[j].SecondGlyph, sec)
			}
			if err := compareValueRecord(formats[0], actualPairs[j].Value1, exp.Value1); err != nil {
				return fmt.Errorf("pair set %q[%d] value1: %w", name, j, err)
			}
			if err := compareValueRecord(formats[1], actualPairs[j].Value2, exp.Value2); err != nil {
				return fmt.Errorf("pair set %q[%d] value2: %w", name, j, err)
			}
		}
	}
	return nil
}

func compareChainContextPos(node *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	if node.Format != 3 {
		return fmt.Errorf("unsupported chain context pos format %d", node.Format)
	}
	payload := node.GPosPayload()
	if payload == nil || payload.ChainingContextFmt3 == nil {
		return fmt.Errorf("missing chain context pos payload")
	}
	seq := payload.ChainingContextFmt3
	if err := compareCoverageSeq(seq.BacktrackCoverages, est.BacktrackCoverage); err != nil {
		return fmt.Errorf("backtrack coverage: %w", err)
	}
	if err := compareCoverageSeq(seq.InputCoverages, est.InputCoverage); err != nil {
		return fmt.Errorf("input coverage: %w", err)
	}
	if err := compareCoverageSeq(seq.LookaheadCoverages, est.LookAheadCoverage); err != nil {
		return fmt.Errorf("lookahead coverage: %w", err)
	}
	if len(seq.Records) != len(est.PosLookupRecords) {
		return fmt.Errorf("lookup record count mismatch: got %d, want %d", len(seq.Records), len(est.PosLookupRecords))
	}
	for i := range est.PosLookupRecords {
		exp := est.PosLookupRecords[i]
		got := seq.Records[i]
		if int(got.SequenceIndex) != exp.SequenceIndex || int(got.LookupListIndex) != exp.LookupListIndex {
			return fmt.Errorf("lookup record[%d] mismatch: got (%d,%d), want (%d,%d)",
				i, got.SequenceIndex, got.LookupListIndex, exp.SequenceIndex, exp.LookupListIndex)
		}
	}
	return nil
}

func compareMarkBasePos(lookup *LookupTable, subIndex int, node *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	if node.Format != 1 {
		return fmt.Errorf("unsupported mark-to-base format %d", node.Format)
	}
	payload := node.GPosPayload()
	if payload == nil || payload.MarkToBaseFmt1 == nil {
		return fmt.Errorf("missing mark-to-base payload")
	}
	support := payload.MarkToBaseFmt1
	if int(support.MarkClassCount) != est.MarkClassCount {
		return fmt.Errorf("mark class count mismatch: got %d, want %d", support.MarkClassCount, est.MarkClassCount)
	}
	markCov, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("mark coverage parse: %w", err)
	}
	if err := compareCoverageNames(markCov, est.MarkCoverage); err != nil {
		return fmt.Errorf("mark coverage: %w", err)
	}
	baseCov, err := coverageGlyphs(support.BaseCoverage)
	if err != nil {
		return fmt.Errorf("base coverage parse: %w", err)
	}
	if err := compareCoverageNames(baseCov, est.BaseCoverage); err != nil {
		return fmt.Errorf("base coverage: %w", err)
	}
	subBytes, err := lookupSubtableBytes(lookup, subIndex)
	if err != nil {
		return err
	}
	markAnchors, markClasses, baseAnchors, err := parseMarkBaseAnchors(subBytes, int(support.MarkClassCount))
	if err != nil {
		return err
	}
	if len(markAnchors) != len(est.MarkAnchors) {
		return fmt.Errorf("mark anchor count mismatch: got %d, want %d", len(markAnchors), len(est.MarkAnchors))
	}
	for i, exp := range est.MarkAnchors {
		if int(markClasses[i]) != exp.Class {
			return fmt.Errorf("mark anchor[%d] class mismatch: got %d, want %d", i, markClasses[i], exp.Class)
		}
		if err := compareAnchor(markAnchors[i], exp.Anchor); err != nil {
			return fmt.Errorf("mark anchor[%d]: %w", i, err)
		}
	}
	if len(baseAnchors) != len(est.BaseAnchors) {
		return fmt.Errorf("base anchor record count mismatch: got %d, want %d", len(baseAnchors), len(est.BaseAnchors))
	}
	for i := range est.BaseAnchors {
		if len(baseAnchors[i]) != len(est.BaseAnchors[i]) {
			return fmt.Errorf("base anchor[%d] count mismatch: got %d, want %d",
				i, len(baseAnchors[i]), len(est.BaseAnchors[i]))
		}
		for j := range est.BaseAnchors[i] {
			if err := compareAnchor(baseAnchors[i][j], est.BaseAnchors[i][j]); err != nil {
				return fmt.Errorf("base anchor[%d][%d]: %w", i, j, err)
			}
		}
	}
	return nil
}

func compareMarkLigPos(lookup *LookupTable, subIndex int, node *LookupNode, est ttxtest.ExpectedGPosSubtable) error {
	if node.Format != 1 {
		return fmt.Errorf("unsupported mark-to-ligature format %d", node.Format)
	}
	payload := node.GPosPayload()
	if payload == nil || payload.MarkToLigatureFmt1 == nil {
		return fmt.Errorf("missing mark-to-ligature payload")
	}
	support := payload.MarkToLigatureFmt1
	if int(support.MarkClassCount) != est.MarkClassCount {
		return fmt.Errorf("mark class count mismatch: got %d, want %d", support.MarkClassCount, est.MarkClassCount)
	}
	markCov, err := coverageGlyphs(node.Coverage)
	if err != nil {
		return fmt.Errorf("mark coverage parse: %w", err)
	}
	if err := compareCoverageNames(markCov, est.MarkCoverage); err != nil {
		return fmt.Errorf("mark coverage: %w", err)
	}
	ligCov, err := coverageGlyphs(support.LigatureCoverage)
	if err != nil {
		return fmt.Errorf("ligature coverage parse: %w", err)
	}
	if err := compareCoverageNames(ligCov, est.LigatureCoverage); err != nil {
		return fmt.Errorf("ligature coverage: %w", err)
	}
	subBytes, err := lookupSubtableBytes(lookup, subIndex)
	if err != nil {
		return err
	}
	markAnchors, markClasses, ligAnchors, err := parseMarkLigAnchors(subBytes, int(support.MarkClassCount))
	if err != nil {
		return err
	}
	if len(markAnchors) != len(est.MarkAnchors) {
		return fmt.Errorf("mark anchor count mismatch: got %d, want %d", len(markAnchors), len(est.MarkAnchors))
	}
	for i, exp := range est.MarkAnchors {
		if int(markClasses[i]) != exp.Class {
			return fmt.Errorf("mark anchor[%d] class mismatch: got %d, want %d", i, markClasses[i], exp.Class)
		}
		if err := compareAnchor(markAnchors[i], exp.Anchor); err != nil {
			return fmt.Errorf("mark anchor[%d]: %w", i, err)
		}
	}
	if len(ligAnchors) != len(est.LigatureAnchors) {
		return fmt.Errorf("ligature anchor count mismatch: got %d, want %d", len(ligAnchors), len(est.LigatureAnchors))
	}
	for i := range est.LigatureAnchors {
		if len(ligAnchors[i]) != len(est.LigatureAnchors[i]) {
			return fmt.Errorf("ligature[%d] component count mismatch: got %d, want %d",
				i, len(ligAnchors[i]), len(est.LigatureAnchors[i]))
		}
		for j := range est.LigatureAnchors[i] {
			if len(ligAnchors[i][j]) != len(est.LigatureAnchors[i][j]) {
				return fmt.Errorf("ligature[%d] component[%d] anchor count mismatch: got %d, want %d",
					i, j, len(ligAnchors[i][j]), len(est.LigatureAnchors[i][j]))
			}
			for k := range est.LigatureAnchors[i][j] {
				if err := compareAnchor(ligAnchors[i][j][k], est.LigatureAnchors[i][j][k]); err != nil {
					return fmt.Errorf("ligature[%d] component[%d] anchor[%d]: %w", i, j, k, err)
				}
			}
		}
	}
	return nil
}

func compareCoverageNames(actual []GlyphIndex, exp []string) error {
	if len(actual) != len(exp) {
		return fmt.Errorf("length mismatch: got %d, want %d", len(actual), len(exp))
	}
	for i, name := range exp {
		gid, err := glyphNameToID(name)
		if err != nil {
			return fmt.Errorf("glyph %q: %w", name, err)
		}
		if actual[i] != gid {
			return fmt.Errorf("glyph[%d] mismatch: got %d, want %d", i, actual[i], gid)
		}
	}
	return nil
}

func lookupSubtableBytes(lookup *LookupTable, i int) (binarySegm, error) {
	if lookup == nil || lookup.err != nil || i >= int(lookup.SubTableCount) {
		return nil, fmt.Errorf("lookup subtable %d out of range", i)
	}
	n := int(lookup.subtableOffsets[i])
	if n <= 0 || n >= len(lookup.raw) {
		return nil, fmt.Errorf("lookup subtable %d offset out of range", i)
	}
	return lookup.raw[n:], nil
}

func parseMarkBaseAnchors(b binarySegm, classCount int) ([]Anchor, []uint16, [][]Anchor, error) {
	if len(b) < 12 {
		return nil, nil, nil, fmt.Errorf("mark-to-base subtable too small")
	}
	markArrayOffset := b.U16(8)
	baseArrayOffset := b.U16(10)
	if markArrayOffset == 0 || baseArrayOffset == 0 {
		return nil, nil, nil, fmt.Errorf("mark/base array missing")
	}
	markBase := b[markArrayOffset:]
	baseBase := b[baseArrayOffset:]
	markArray := parseMarkArray(markBase)
	baseArray := parseBaseArray(baseBase, classCount)
	markAnchors := make([]Anchor, 0, markArray.MarkCount)
	markClasses := make([]uint16, 0, markArray.MarkCount)
	for _, rec := range markArray.MarkRecords {
		if rec.MarkAnchor == 0 || int(rec.MarkAnchor) >= len(markBase) {
			return nil, nil, nil, fmt.Errorf("mark anchor offset out of bounds")
		}
		anchor := parseAnchor(markBase[rec.MarkAnchor:])
		markAnchors = append(markAnchors, anchor)
		markClasses = append(markClasses, rec.Class)
	}
	baseAnchors := make([][]Anchor, 0, len(baseArray))
	for _, rec := range baseArray {
		row := make([]Anchor, 0, len(rec.BaseAnchors))
		for _, off := range rec.BaseAnchors {
			if off == 0 || int(off) >= len(baseBase) {
				return nil, nil, nil, fmt.Errorf("base anchor offset out of bounds")
			}
			row = append(row, parseAnchor(baseBase[off:]))
		}
		baseAnchors = append(baseAnchors, row)
	}
	return markAnchors, markClasses, baseAnchors, nil
}

func parseMarkLigAnchors(b binarySegm, classCount int) ([]Anchor, []uint16, [][][]Anchor, error) {
	if len(b) < 12 {
		return nil, nil, nil, fmt.Errorf("mark-to-ligature subtable too small")
	}
	markArrayOffset := b.U16(8)
	ligArrayOffset := b.U16(10)
	if markArrayOffset == 0 || ligArrayOffset == 0 {
		return nil, nil, nil, fmt.Errorf("mark/ligature array missing")
	}
	markBase := b[markArrayOffset:]
	ligBase := b[ligArrayOffset:]
	markArray := parseMarkArray(markBase)
	markAnchors := make([]Anchor, 0, markArray.MarkCount)
	markClasses := make([]uint16, 0, markArray.MarkCount)
	for _, rec := range markArray.MarkRecords {
		if rec.MarkAnchor == 0 || int(rec.MarkAnchor) >= len(markBase) {
			return nil, nil, nil, fmt.Errorf("mark anchor offset out of bounds")
		}
		anchor := parseAnchor(markBase[rec.MarkAnchor:])
		markAnchors = append(markAnchors, anchor)
		markClasses = append(markClasses, rec.Class)
	}
	ligAnchors := make([][][]Anchor, 0)
	if len(ligBase) < 2 {
		return nil, nil, nil, fmt.Errorf("ligature array too small")
	}
	ligCount := int(ligBase.U16(0))
	offs := make([]uint16, 0, ligCount)
	offset := 2
	for i := 0; i < ligCount; i++ {
		if offset+2 > len(ligBase) {
			return nil, nil, nil, fmt.Errorf("ligature array offsets out of bounds")
		}
		offs = append(offs, ligBase.U16(offset))
		offset += 2
	}
	for _, o := range offs {
		if o == 0 || int(o)+2 > len(ligBase) {
			return nil, nil, nil, fmt.Errorf("ligature attach offset out of bounds")
		}
		lig := ligBase[o:]
		compCount := int(binarySegm(lig).U16(0))
		compOff := 2
		comps := make([][]Anchor, 0, compCount)
		for c := 0; c < compCount; c++ {
			if compOff+classCount*2 > len(lig) {
				return nil, nil, nil, fmt.Errorf("component anchors out of bounds")
			}
			anchors := make([]Anchor, 0, classCount)
			for k := 0; k < classCount; k++ {
				off := binarySegm(lig).U16(compOff + k*2)
				if off == 0 || int(o+off) >= len(ligBase) {
					return nil, nil, nil, fmt.Errorf("ligature anchor offset out of bounds")
				}
				anchors = append(anchors, parseAnchor(ligBase[o+off:]))
			}
			comps = append(comps, anchors)
			compOff += classCount * 2
		}
		ligAnchors = append(ligAnchors, comps)
	}
	return markAnchors, markClasses, ligAnchors, nil
}

func compareAnchor(actual Anchor, exp ttxtest.ExpectedAnchor) error {
	if exp.Format != 0 && int(actual.Format) != exp.Format {
		return fmt.Errorf("format mismatch: got %d, want %d", actual.Format, exp.Format)
	}
	if int(actual.XCoordinate) != exp.XCoordinate {
		return fmt.Errorf("XCoordinate mismatch: got %d, want %d", actual.XCoordinate, exp.XCoordinate)
	}
	if int(actual.YCoordinate) != exp.YCoordinate {
		return fmt.Errorf("YCoordinate mismatch: got %d, want %d", actual.YCoordinate, exp.YCoordinate)
	}
	if exp.HasPoint && int(actual.AnchorPoint) != exp.AnchorPoint {
		return fmt.Errorf("AnchorPoint mismatch: got %d, want %d", actual.AnchorPoint, exp.AnchorPoint)
	}
	if exp.HasXDevice && int(actual.XDeviceOffset) != exp.XDevice {
		return fmt.Errorf("XDeviceOffset mismatch: got %d, want %d", actual.XDeviceOffset, exp.XDevice)
	}
	if exp.HasYDevice && int(actual.YDeviceOffset) != exp.YDevice {
		return fmt.Errorf("YDeviceOffset mismatch: got %d, want %d", actual.YDeviceOffset, exp.YDevice)
	}
	return nil
}

func compareCoverageSeq(actual []Coverage, exp [][]string) error {
	if len(actual) != len(exp) {
		return fmt.Errorf("coverage count mismatch: got %d, want %d", len(actual), len(exp))
	}
	for i := range exp {
		ids, err := coverageGlyphs(actual[i])
		if err != nil {
			return fmt.Errorf("coverage[%d] parse: %w", i, err)
		}
		if len(ids) != len(exp[i]) {
			return fmt.Errorf("coverage[%d] length mismatch: got %d, want %d", i, len(ids), len(exp[i]))
		}
		for j, name := range exp[i] {
			gid, err := glyphNameToID(name)
			if err != nil {
				return fmt.Errorf("coverage[%d] glyph %q: %w", i, name, err)
			}
			if ids[j] != gid {
				return fmt.Errorf("coverage[%d][%d] mismatch: got %d, want %d", i, j, ids[j], gid)
			}
		}
	}
	return nil
}

func compareValueRecord(format ValueFormat, actual ValueRecord, exp ttxtest.ExpectedValueRecord) error {
	if exp.HasXPlacement && format&ValueFormatXPlacement == 0 {
		return fmt.Errorf("unexpected XPlacement in expected value")
	}
	if exp.HasYPlacement && format&ValueFormatYPlacement == 0 {
		return fmt.Errorf("unexpected YPlacement in expected value")
	}
	if exp.HasXAdvance && format&ValueFormatXAdvance == 0 {
		return fmt.Errorf("unexpected XAdvance in expected value")
	}
	if exp.HasYAdvance && format&ValueFormatYAdvance == 0 {
		return fmt.Errorf("unexpected YAdvance in expected value")
	}
	if exp.HasXPlaDevice && format&ValueFormatXPlaDevice == 0 {
		return fmt.Errorf("unexpected XPlaDevice in expected value")
	}
	if exp.HasYPlaDevice && format&ValueFormatYPlaDevice == 0 {
		return fmt.Errorf("unexpected YPlaDevice in expected value")
	}
	if exp.HasXAdvDevice && format&ValueFormatXAdvDevice == 0 {
		return fmt.Errorf("unexpected XAdvDevice in expected value")
	}
	if exp.HasYAdvDevice && format&ValueFormatYAdvDevice == 0 {
		return fmt.Errorf("unexpected YAdvDevice in expected value")
	}
	if format&ValueFormatXPlacement != 0 {
		if !exp.HasXPlacement {
			return fmt.Errorf("missing expected XPlacement")
		}
		if actual.XPlacement != int16(exp.XPlacement) {
			return fmt.Errorf("XPlacement mismatch: got %d, want %d", actual.XPlacement, exp.XPlacement)
		}
	}
	if format&ValueFormatYPlacement != 0 {
		if !exp.HasYPlacement {
			return fmt.Errorf("missing expected YPlacement")
		}
		if actual.YPlacement != int16(exp.YPlacement) {
			return fmt.Errorf("YPlacement mismatch: got %d, want %d", actual.YPlacement, exp.YPlacement)
		}
	}
	if format&ValueFormatXAdvance != 0 {
		if !exp.HasXAdvance {
			return fmt.Errorf("missing expected XAdvance")
		}
		if actual.XAdvance != int16(exp.XAdvance) {
			return fmt.Errorf("XAdvance mismatch: got %d, want %d", actual.XAdvance, exp.XAdvance)
		}
	}
	if format&ValueFormatYAdvance != 0 {
		if !exp.HasYAdvance {
			return fmt.Errorf("missing expected YAdvance")
		}
		if actual.YAdvance != int16(exp.YAdvance) {
			return fmt.Errorf("YAdvance mismatch: got %d, want %d", actual.YAdvance, exp.YAdvance)
		}
	}
	if format&ValueFormatXPlaDevice != 0 {
		if !exp.HasXPlaDevice {
			return fmt.Errorf("missing expected XPlaDevice")
		}
		if actual.XPlaDevice != uint16(exp.XPlaDevice) {
			return fmt.Errorf("XPlaDevice mismatch: got %d, want %d", actual.XPlaDevice, exp.XPlaDevice)
		}
	}
	if format&ValueFormatYPlaDevice != 0 {
		if !exp.HasYPlaDevice {
			return fmt.Errorf("missing expected YPlaDevice")
		}
		if actual.YPlaDevice != uint16(exp.YPlaDevice) {
			return fmt.Errorf("YPlaDevice mismatch: got %d, want %d", actual.YPlaDevice, exp.YPlaDevice)
		}
	}
	if format&ValueFormatXAdvDevice != 0 {
		if !exp.HasXAdvDevice {
			return fmt.Errorf("missing expected XAdvDevice")
		}
		if actual.XAdvDevice != uint16(exp.XAdvDevice) {
			return fmt.Errorf("XAdvDevice mismatch: got %d, want %d", actual.XAdvDevice, exp.XAdvDevice)
		}
	}
	if format&ValueFormatYAdvDevice != 0 {
		if !exp.HasYAdvDevice {
			return fmt.Errorf("missing expected YAdvDevice")
		}
		if actual.YAdvDevice != uint16(exp.YAdvDevice) {
			return fmt.Errorf("YAdvDevice mismatch: got %d, want %d", actual.YAdvDevice, exp.YAdvDevice)
		}
	}
	return nil
}
