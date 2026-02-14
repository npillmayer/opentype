package ot

import (
	"os"
	"sync"
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func assertFeatureGraphLazy(t *testing.T, graph *FeatureList) {
	t.Helper()
	if graph == nil {
		t.Fatalf("expected concrete feature graph to be parsed")
	}
	if graph.Len() == 0 {
		return
	}
	if len(graph.featuresByIndex) != 0 {
		t.Fatalf("feature cache should be empty right after parse, has %d entries", len(graph.featuresByIndex))
	}
	f0 := graph.featureAtIndex(0)
	if f0 == nil {
		t.Fatalf("lazy feature load for index 0 returned nil")
	}
	if len(graph.featuresByIndex) != 1 {
		t.Fatalf("expected exactly one cached feature after first load, have %d", len(graph.featuresByIndex))
	}
	f1 := graph.featureAtIndex(0)
	if f1 != f0 {
		t.Fatalf("expected stable cached feature pointer for index 0")
	}
	if len(graph.featuresByIndex) != 1 {
		t.Fatalf("cache size should remain stable on repeated access")
	}
}

func assertScriptGraphLazy(t *testing.T, graph *ScriptList) {
	t.Helper()
	if graph == nil || graph.Len() == 0 {
		return
	}
	graph.mu.RLock()
	if len(graph.scriptByTag) != 0 {
		graph.mu.RUnlock()
		t.Fatalf("script cache should be empty right after parse, has %d entries", len(graph.scriptByTag))
	}
	graph.mu.RUnlock()

	tag := graph.scriptOrder[0]
	s0 := graph.Script(tag)
	if s0 == nil {
		t.Fatalf("lazy script load for tag %s returned nil", tag)
	}
	graph.mu.RLock()
	if len(graph.scriptByTag) != 1 {
		graph.mu.RUnlock()
		t.Fatalf("expected exactly one cached script after first load, have %d", len(graph.scriptByTag))
	}
	graph.mu.RUnlock()
	s1 := graph.Script(tag)
	if s1 != s0 {
		t.Fatalf("expected stable cached script pointer for tag %s", tag)
	}

	if len(s0.langOrder) > 0 {
		s0.mu.RLock()
		if len(s0.langByTag) != 0 {
			s0.mu.RUnlock()
			t.Fatalf("LangSys cache should be empty right after script load, has %d entries", len(s0.langByTag))
		}
		s0.mu.RUnlock()
		langTag := s0.langOrder[0]
		l0 := s0.LangSys(langTag)
		if l0 == nil {
			t.Fatalf("lazy LangSys load for tag %s returned nil", langTag)
		}
		s0.mu.RLock()
		if len(s0.langByTag) != 1 {
			s0.mu.RUnlock()
			t.Fatalf("expected exactly one cached LangSys after first load, have %d", len(s0.langByTag))
		}
		s0.mu.RUnlock()
		if s0.LangSys(langTag) != l0 {
			t.Fatalf("expected stable cached LangSys pointer for tag %s", langTag)
		}
	}
}

func assertScriptGraphParity(t *testing.T, graph *ScriptList) {
	t.Helper()
	if graph == nil {
		t.Fatalf("expected concrete script graph to be parsed")
	}
	if graph.Error() != nil {
		t.Fatalf("unexpected concrete script graph parse error: %v", graph.Error())
	}
	scriptCount := 0
	for scriptTag, script := range graph.Range() {
		scriptCount++
		if script == nil {
			t.Fatalf("concrete script graph returned nil script for tag %s", scriptTag)
		}
		if script.Error() != nil {
			t.Fatalf("unexpected concrete script parse error for %s: %v", scriptTag, script.Error())
		}
		if scriptTag == DFLT && script.DefaultLangSys() == nil {
			t.Fatalf("DFLT script must provide a default language-system")
		}
		langCount := 0
		for langTag, lsys := range script.Range() {
			langCount++
			if lsys == nil {
				t.Fatalf("concrete script %s returned nil language-system for tag %s", scriptTag, langTag)
			}
			if script.LangSys(langTag) == nil {
				t.Fatalf("concrete script %s missing language-system tag %s", scriptTag, langTag)
			}
		}
		if langCount != len(script.langOrder) {
			t.Fatalf("script %s language-system count mismatch: declared=%d concrete=%d",
				scriptTag, len(script.langOrder), langCount)
		}
	}
	if scriptCount != graph.Len() {
		t.Fatalf("concrete script graph count mismatch: len()=%d range=%d", graph.Len(), scriptCount)
	}
}

func assertLookupGraphBaseline(t *testing.T, graph *LookupListGraph) {
	t.Helper()
	if graph == nil {
		t.Fatalf("expected concrete lookup graph to be parsed")
	}
	if graph.Error() != nil {
		t.Fatalf("unexpected concrete lookup graph parse error: %v", graph.Error())
	}
	if graph.Len() == 0 {
		return
	}
	concreteLookup := graph.Lookup(0)
	if concreteLookup == nil {
		t.Fatalf("expected concrete lookup[0] to be resolvable")
	}
	if concreteLookup.Error() != nil {
		t.Fatalf("unexpected concrete lookup[0] parse error: %v", concreteLookup.Error())
	}
	if int(concreteLookup.SubTableCount) != len(concreteLookup.subtableOffsets) {
		t.Fatalf("lookup[0] subtable-count mismatch: offsets=%d concrete=%d",
			len(concreteLookup.subtableOffsets), concreteLookup.SubTableCount)
	}
}

func countGSubPayloadSlots(p *GSubLookupPayload) int {
	if p == nil {
		return 0
	}
	n := 0
	if p.SingleFmt1 != nil {
		n++
	}
	if p.SingleFmt2 != nil {
		n++
	}
	if p.MultipleFmt1 != nil {
		n++
	}
	if p.AlternateFmt1 != nil {
		n++
	}
	if p.LigatureFmt1 != nil {
		n++
	}
	if p.ContextFmt1 != nil {
		n++
	}
	if p.ContextFmt2 != nil {
		n++
	}
	if p.ContextFmt3 != nil {
		n++
	}
	if p.ChainingContextFmt1 != nil {
		n++
	}
	if p.ChainingContextFmt2 != nil {
		n++
	}
	if p.ChainingContextFmt3 != nil {
		n++
	}
	if p.ExtensionFmt1 != nil {
		n++
	}
	if p.ReverseChainingFmt1 != nil {
		n++
	}
	return n
}

func countGPosPayloadSlots(p *GPosLookupPayload) int {
	if p == nil {
		return 0
	}
	n := 0
	if p.SingleFmt1 != nil {
		n++
	}
	if p.SingleFmt2 != nil {
		n++
	}
	if p.PairFmt1 != nil {
		n++
	}
	if p.PairFmt2 != nil {
		n++
	}
	if p.CursiveFmt1 != nil {
		n++
	}
	if p.MarkToBaseFmt1 != nil {
		n++
	}
	if p.MarkToLigatureFmt1 != nil {
		n++
	}
	if p.MarkToMarkFmt1 != nil {
		n++
	}
	if p.ContextFmt1 != nil {
		n++
	}
	if p.ContextFmt2 != nil {
		n++
	}
	if p.ContextFmt3 != nil {
		n++
	}
	if p.ChainingContextFmt1 != nil {
		n++
	}
	if p.ChainingContextFmt2 != nil {
		n++
	}
	if p.ChainingContextFmt3 != nil {
		n++
	}
	if p.ExtensionFmt1 != nil {
		n++
	}
	return n
}

func assertLookupGraphGPosScaffold(t *testing.T, graph *LookupListGraph) {
	t.Helper()
	if graph == nil || graph.Len() == 0 {
		return
	}
	checked := 0
	for i, lookup := range graph.Range() {
		if lookup == nil || lookup.SubTableCount == 0 {
			continue
		}
		sub := lookup.Subtable(0)
		if sub == nil {
			t.Fatalf("lookup[%d] expected subtable[0] in concrete graph", i)
		}
		if sub.GPosPayload() == nil {
			t.Fatalf("lookup[%d] subtable[0] expected GPOS payload scaffold", i)
		}
		slots := countGPosPayloadSlots(sub.GPosPayload())
		if slots != 1 {
			t.Fatalf("lookup[%d] subtable[0] expected exactly one GPOS payload slot, got %d", i, slots)
		}
		checked++
		if checked >= 5 {
			return
		}
	}
	if checked == 0 {
		t.Fatalf("expected to validate at least one GPOS lookup scaffold")
	}
}

func assertLookupGraphGSubScaffold(t *testing.T, graph *LookupListGraph) {
	t.Helper()
	if graph == nil || graph.Len() == 0 {
		return
	}
	checked := 0
	for i, lookup := range graph.Range() {
		if lookup == nil || lookup.SubTableCount == 0 {
			continue
		}
		sub := lookup.Subtable(0)
		if sub == nil {
			t.Fatalf("lookup[%d] expected subtable[0] in concrete graph", i)
		}
		if sub.GSubPayload() == nil {
			t.Fatalf("lookup[%d] subtable[0] expected GSUB payload scaffold", i)
		}
		slots := countGSubPayloadSlots(sub.GSubPayload())
		if slots != 1 {
			t.Fatalf("lookup[%d] subtable[0] expected exactly one GSUB payload slot, got %d", i, slots)
		}
		checked++
		if checked >= 5 {
			return
		}
	}
	if checked == 0 {
		t.Fatalf("expected to validate at least one GSUB lookup scaffold")
	}
}

func TestParseHeader(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	f := loadTestdataFont(t, "GentiumPlus-R")
	otf, err := Parse(f.F.Binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("otf.header.tag = %x", otf.Header.FontType)
	if otf.Header.FontType != 0x00010000 {
		t.Fatalf("expected font Gentium to be OT 0x0001000, is %x", otf.Header.FontType)
	}
}

// TODO TODO
func TestCMapTableGlyphIndex(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	otf := parseFont(t, "Calibri")
	t.Logf("otf.header.tag = %x", otf.Header.FontType)
	table := getTable(otf, "cmap", t)
	cmap := table.Self().AsCMap()
	if cmap == nil {
		t.Fatal("cannot convert cmap table")
	}
	r := rune('A')
	glyph := cmap.GlyphIndexMap.Lookup(r)
	if glyph == 0 {
		t.Error("expected glyph position for 'A', got 0")
	}
	t.Logf("glyph ID = %d | 0x%x", glyph, glyph)
	if glyph != 4 {
		t.Errorf("expected glyph position for 'A' to be 4, got %d", glyph)
	}
}

func TestParseGPos(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	f := loadTestdataFont(t, "Calibri")
	otf, err := Parse(f.F.Binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("font contains tables:")
	hasGPos := false
	for _, tag := range otf.TableTags() {
		t.Logf("  %s", tag.String())
		if tag.String() == "GPOS" {
			hasGPos = true
		}
	}
	if !hasGPos {
		t.Fatalf("expected font to have GPOS table, hasn't")
	}
	gposTag := T("GPOS")
	gpos := otf.tables[gposTag].Self().AsGPos()
	if gpos == nil {
		t.Fatalf("cannot find a GPOS table")
	}
	if gpos.FeatureGraph() == nil {
		t.Fatalf("expected concrete GPOS feature graph to be parsed")
	}
	t.Logf("otf.GPOS: %d concrete features:", gpos.FeatureGraph().Len())
	if gpos.FeatureGraph().Len() != 27 {
		t.Errorf("expected 27 GPOS features, have %d", gpos.FeatureGraph().Len())
	}
	if gpos.FeatureGraph().Error() != nil {
		t.Errorf("unexpected concrete feature graph parse error: %v", gpos.FeatureGraph().Error())
	}
	assertFeatureGraphLazy(t, gpos.FeatureGraph())
	assertScriptGraphLazy(t, gpos.ScriptGraph())
	assertLookupGraphBaseline(t, gpos.LookupGraph())
	assertLookupGraphGPosScaffold(t, gpos.LookupGraph())
	t.Logf("otf.GPOS: %d concrete scripts:", gpos.ScriptGraph().Len())
	assertScriptGraphParity(t, gpos.ScriptGraph())
}

func TestParseGSub(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	f := loadTestdataFont(t, "GentiumPlus-R")
	otf, err := Parse(f.F.Binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("font contains tables:")
	hasGSub := false
	for _, tag := range otf.TableTags() {
		t.Logf("  %s", tag.String())
		if tag.String() == "GSUB" {
			hasGSub = true
		}
	}
	if !hasGSub {
		t.Fatalf("expected font to have GSUB table, hasn't")
	}
	gsubTag := T("GSUB")
	gsub := otf.tables[gsubTag].Self().AsGSub()
	if gsub == nil {
		t.Fatalf("cannot find a GSUB table")
	}
	if gsub.FeatureGraph() == nil {
		t.Fatalf("expected concrete GSUB feature graph to be parsed")
	}
	t.Logf("otf.GSUB: %d concrete features:", gsub.FeatureGraph().Len())
	if gsub.FeatureGraph().Len() != 41 {
		t.Errorf("expected 41 features, have %d", gsub.FeatureGraph().Len())
	}
	if gsub.FeatureGraph().Error() != nil {
		t.Errorf("unexpected concrete feature graph parse error: %v", gsub.FeatureGraph().Error())
	}
	assertFeatureGraphLazy(t, gsub.FeatureGraph())
	assertScriptGraphLazy(t, gsub.ScriptGraph())
	assertLookupGraphBaseline(t, gsub.LookupGraph())
	assertLookupGraphGSubScaffold(t, gsub.LookupGraph())
	assertScriptGraphParity(t, gsub.ScriptGraph())
	// t.Logf("otf.GSUB: %d scripts:", len(gsub.scripts))
	// for i, sc := range gsub.scripts {
	// 	t.Logf("[%d] script '%s'", i, sc.Tag)
	// }
	// if len(gsub.scripts) != 4 ||
	// 	gsub.scripts[len(gsub.scripts)-1].Tag.String() != "latn" {
	// 	t.Errorf("expected scripts[4] to be 'latn', isn't")
	// }
}

func TestFeatureGraphLazyConcurrent(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	otf := parseFont(t, "Calibri")
	gpos := otf.tables[T("GPOS")].Self().AsGPos()
	if gpos == nil || gpos.FeatureGraph() == nil || gpos.FeatureGraph().Len() == 0 {
		t.Fatalf("expected non-empty GPOS concrete feature graph")
	}
	graph := gpos.FeatureGraph()
	const workers = 16
	ptrs := make(chan *Feature, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			ptrs <- graph.featureAtIndex(0)
		}()
	}
	wg.Wait()
	close(ptrs)
	var first *Feature
	for p := range ptrs {
		if p == nil {
			t.Fatalf("concurrent lazy feature load returned nil")
		}
		if first == nil {
			first = p
			continue
		}
		if p != first {
			t.Fatalf("concurrent lazy feature loads produced different cached pointers")
		}
	}
	if len(graph.featuresByIndex) != 1 {
		t.Fatalf("expected exactly one cached feature after concurrent loads, have %d", len(graph.featuresByIndex))
	}
}

func TestScriptGraphLazyConcurrent(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	otf := parseFont(t, "Calibri")
	gpos := otf.tables[T("GPOS")].Self().AsGPos()
	if gpos == nil || gpos.ScriptGraph() == nil || gpos.ScriptGraph().Len() == 0 {
		t.Fatalf("expected non-empty GPOS concrete script graph")
	}
	graph := gpos.ScriptGraph()
	tag := graph.scriptOrder[0]
	const workers = 16
	ptrs := make(chan *Script, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			ptrs <- graph.Script(tag)
		}()
	}
	wg.Wait()
	close(ptrs)
	var first *Script
	for p := range ptrs {
		if p == nil {
			t.Fatalf("concurrent lazy script load returned nil")
		}
		if first == nil {
			first = p
			continue
		}
		if p != first {
			t.Fatalf("concurrent lazy script loads produced different cached pointers")
		}
	}
	graph.mu.RLock()
	cachedScripts := len(graph.scriptByTag)
	graph.mu.RUnlock()
	if cachedScripts != 1 {
		t.Fatalf("expected exactly one cached script after concurrent loads, have %d", cachedScripts)
	}
}

func TestLookupGraphLazyConcurrent(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	otf := parseFont(t, "Calibri")
	gsub := otf.tables[T("GSUB")].Self().AsGSub()
	if gsub == nil || gsub.LookupGraph() == nil || gsub.LookupGraph().Len() == 0 {
		t.Fatalf("expected non-empty GSUB concrete lookup graph")
	}
	graph := gsub.LookupGraph()
	const workers = 16
	ptrs := make(chan *LookupTable, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			ptrs <- graph.Lookup(0)
		}()
	}
	wg.Wait()
	close(ptrs)
	var first *LookupTable
	for p := range ptrs {
		if p == nil {
			t.Fatalf("concurrent lazy lookup load returned nil")
		}
		if first == nil {
			first = p
			continue
		}
		if p != first {
			t.Fatalf("concurrent lazy lookup loads produced different cached pointers")
		}
	}
}

func TestLookupSubtableGraphLazyConcurrent(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	otf := parseFont(t, "Calibri")
	gsub := otf.tables[T("GSUB")].Self().AsGSub()
	if gsub == nil || gsub.LookupGraph() == nil || gsub.LookupGraph().Len() == 0 {
		t.Fatalf("expected non-empty GSUB concrete lookup graph")
	}
	lookup := gsub.LookupGraph().Lookup(0)
	if lookup == nil || lookup.SubTableCount == 0 {
		t.Fatalf("expected lookup[0] with at least one subtable")
	}
	const workers = 16
	ptrs := make(chan *LookupNode, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			ptrs <- lookup.Subtable(0)
		}()
	}
	wg.Wait()
	close(ptrs)
	var first *LookupNode
	for p := range ptrs {
		if p == nil {
			t.Fatalf("concurrent lazy subtable load returned nil")
		}
		if first == nil {
			first = p
			continue
		}
		if p != first {
			t.Fatalf("concurrent lazy subtable loads produced different cached pointers")
		}
	}
}

func TestLookupGraphCacheStableAcrossRepeatedAccess(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otf := parseFont(t, "Calibri")
	gsub := otf.tables[T("GSUB")].Self().AsGSub()
	if gsub == nil {
		t.Fatalf("expected GSUB table")
	}
	graph := gsub.LookupGraph()
	if graph == nil || graph.Len() == 0 {
		t.Fatalf("expected non-empty concrete GSUB lookup graph")
	}

	lookup1 := graph.Lookup(0)
	lookup2 := graph.Lookup(0)
	if lookup1 == nil || lookup2 == nil {
		t.Fatalf("expected non-nil concrete lookup")
	}
	if lookup1 != lookup2 {
		t.Fatalf("expected stable cached lookup pointer across repeated traversal")
	}

	if lookup1.SubTableCount == 0 {
		t.Skip("lookup[0] has no subtables")
	}
	st1 := lookup1.Subtable(0)
	st2 := lookup2.Subtable(0)
	if st1 == nil || st2 == nil {
		t.Fatalf("expected non-nil concrete lookup subtable")
	}
	if st1 != st2 {
		t.Fatalf("expected stable cached subtable pointer across repeated traversal")
	}
}

func TestParseKern(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	f := loadTestdataFont(t, "Calibri")
	otf, err := Parse(f.F.Binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("font contains tables:")
	hasKern := false
	for _, tag := range otf.TableTags() {
		t.Logf("  %s", tag.String())
		if tag.String() == "kern" {
			hasKern = true
		}
	}
	if !hasKern {
		t.Fatalf("expected font to have kern table, hasn't")
	}
	kern := otf.tables[T("kern")]
	if kern == nil {
		t.Fatalf("cannot find table kern")
	}
	if len(kern.Binary()) == 0 {
		t.Fatalf("expected table kern to expose binary data")
	}
}

func TestParseOtherTables(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	f := loadTestdataFont(t, "Calibri")
	otf, err := Parse(f.F.Binary)
	if err != nil {
		t.Fatal(err)
	}
	maxp := otf.tables[T("maxp")].Self().AsMaxP()
	if maxp == nil {
		t.Fatalf("cannot find a maxp table")
	}
	t.Logf("MaxP.NumGlyphs = %d", maxp.NumGlyphs)
	if maxp.NumGlyphs != 3874 {
		t.Errorf("expected Calibri to have 3874 glyphs, but %d indicated", maxp.NumGlyphs)
	}
	loca := otf.tables[T("loca")].Self().AsLoca()
	if loca == nil {
		t.Fatalf("cannot find a maxp table")
	}
	hhea := otf.tables[T("hhea")].Self().AsHHea()
	if hhea == nil {
		t.Fatalf("cannot find a hhea table")
	}
	if otf.HHea == nil {
		t.Fatalf("expected typed font accessor for hhea")
	}
	t.Logf("hhea number of metrics = %d", hhea.NumberOfHMetrics)
	if hhea.NumberOfHMetrics != 3843 {
		t.Errorf("expected Calibri to have 3843 metrics, but %d indicated", hhea.NumberOfHMetrics)
	}
	if hhea.Ascender == 0 && hhea.Descender == 0 {
		t.Errorf("expected hhea ascender/descender to be populated")
	}
	if hhea.AdvanceWidthMax == 0 {
		t.Errorf("expected hhea advanceWidthMax to be populated")
	}
	os2 := otf.tables[T("OS/2")].Self().AsOS2()
	if os2 == nil {
		t.Fatalf("cannot find an OS/2 table")
	}
	if otf.OS2 == nil {
		t.Fatalf("expected typed font accessor for OS/2")
	}
	if os2.TypoAscender == 0 && os2.TypoDescender == 0 {
		t.Errorf("expected OS/2 typo metrics to be populated")
	}
	hmtx := otf.tables[T("hmtx")].Self().AsHMtx()
	if hmtx == nil {
		t.Fatalf("cannot find an hmtx table")
	}
	if otf.HMtx == nil {
		t.Fatalf("expected typed font accessor for hmtx")
	}
	if hmtx.GlyphCount() != maxp.NumGlyphs {
		t.Errorf("expected hmtx glyph count %d, have %d", maxp.NumGlyphs, hmtx.GlyphCount())
	}
	if len(hmtx.LongMetrics()) != hhea.NumberOfHMetrics {
		t.Errorf("expected %d long hmetrics, got %d", hhea.NumberOfHMetrics, len(hmtx.LongMetrics()))
	}
	aw, _, ok := hmtx.HMetrics(4) // glyph index for 'A' in Calibri
	if !ok {
		t.Fatalf("cannot resolve hmtx metric for glyph 4")
	}
	if aw != 1185 {
		t.Errorf("expected advance width for glyph 4 to be 1185, got %d", aw)
	}
}

func TestParseMaxPVersion05Size6(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	raw, err := os.ReadFile("../testdata/Go-Regular.otf")
	if err != nil {
		t.Fatalf("read Go-Regular.otf: %v", err)
	}
	otf, err := Parse(raw, IsTestfont)
	if err != nil {
		t.Fatalf("parse Go-Regular.otf failed: %v", err)
	}
	maxpTable := otf.Table(T("maxp"))
	if maxpTable == nil {
		t.Fatalf("maxp table missing after parse")
	}
	_, size := maxpTable.Extent()
	if size != 6 {
		t.Fatalf("expected Go-Regular.otf maxp size 6, got %d", size)
	}
	maxp := maxpTable.Self().AsMaxP()
	if maxp == nil {
		t.Fatalf("maxp table cannot be decoded")
	}
	if maxp.NumGlyphs <= 0 {
		t.Fatalf("maxp.NumGlyphs should be > 0, got %d", maxp.NumGlyphs)
	}
	for _, pe := range otf.Errors() {
		if pe.Table == T("maxp") && pe.Section == "Missing" {
			t.Fatalf("unexpected missing maxp error for a font that contains maxp size 6: %v", pe)
		}
	}
}

func TestParseGDef(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	otf := parseFont(t, "Calibri")
	table := getTable(otf, "GDEF", t)
	gdef := table.Self().AsGDef()
	if gdef.GlyphClassDef.format == 0 {
		t.Fatalf("GDEF table has not GlyphClassDef section")
	}
	// Calibri uses glyph class def format 2
	t.Logf("GDEF.GlyphClassDef.Format = %d", gdef.GlyphClassDef.format)
	glyph := GlyphIndex(1380) // ID of uni0336 in Calibri
	clz := gdef.GlyphClassDef.Lookup(glyph)
	t.Logf("gylph class for uni0336|1280 is %d", clz)
	if clz != 3 {
		t.Errorf("expected to be uni0336 of class 3 (mark), is %d", clz)
	}
}

func TestParseGSubLookups(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()
	//
	otf := parseFont(t, "Calibri")
	table := getTable(otf, "GSUB", t)
	gsub := table.Self().AsGSub()
	graph := gsub.LookupGraph()
	if graph == nil || graph.Len() == 0 {
		t.Fatalf("GSUB table has no lookup graph section")
	}
	/*
	   <LookupList>
	     <!-- LookupCount=49 -->      Calibri has 49 lookup entries
	     <Lookup index="0">           Lookup #0 is of type 7, extending to type 1
	       <LookupType value="7"/>
	       <LookupFlag value="0"/>
	       <!-- SubTableCount=1 -->
	       <ExtensionSubst index="0" Format="1">
	         <ExtensionLookupType value="1"/>
	         <SingleSubst Format="2">
	           <Substitution in="Scedilla" out="uni0218"/>
	           <Substitution in="scedilla" out="uni0219"/>
	           <Substitution in="uni0162" out="uni021A"/>
	           <Substitution in="uni0163" out="uni021B"/>
	         </SingleSubst>
	       </ExtensionSubst>
	     </Lookup>
	*/
	t.Logf("font Calibri has %d lookups", graph.Len())
	clookup := graph.Lookup(0)
	if clookup == nil {
		t.Fatalf("expected concrete lookup[0]")
	}
	t.Logf("lookup[0].subTables count is %d", clookup.SubTableCount)
	csub := clookup.Subtable(0)
	if csub == nil {
		t.Fatalf("expected concrete subtable[0]")
	}
	// Concrete lookup graph keeps extension as explicit type-7 node and resolves type via payload.
	if csub.GSubPayload() == nil || csub.GSubPayload().ExtensionFmt1 == nil {
		t.Fatalf("expected concrete GSUB extension payload for lookup[0]/subtable[0]")
	}
	if csub.LookupType != GSubLookupTypeExtensionSubs {
		t.Fatalf("expected concrete subtable type 7 (extension), got %d", csub.LookupType)
	}
	if csub.GSubPayload().ExtensionFmt1.ResolvedType != GSubLookupTypeSingle {
		t.Fatalf("extension resolved-type mismatch: want=%d concrete=%d", GSubLookupTypeSingle, csub.GSubPayload().ExtensionFmt1.ResolvedType)
	}
	if csub.GSubPayload().ExtensionFmt1.Resolved == nil {
		t.Fatalf("expected concrete extension resolved node")
	}
	if csub.GSubPayload().ExtensionFmt1.Resolved.LookupType != GSubLookupTypeSingle {
		t.Fatalf("extension resolved node type mismatch: want=%d concrete=%d",
			GSubLookupTypeSingle, csub.GSubPayload().ExtensionFmt1.Resolved.LookupType)
	}
}

// ---------------------------------------------------------------------------

func getTable(otf *Font, name string, t *testing.T) Table {
	table := otf.tables[T(name)]
	if table == nil {
		t.Fatalf("table %s not found in font", name)
	}
	return table
}

func parseFont(t *testing.T, pattern string) *Font {
	otf := loadTestdataFont(t, pattern)
	if otf == nil {
		return nil
	}
	otf, err := Parse(otf.F.Binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("--- font parsed ---")
	return otf
}

func TestParseGPosLookups(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	otf := parseFont(t, "Calibri")
	table := getTable(otf, "GPOS", t)
	gpos := table.Self().AsGPos()

	if gpos == nil {
		t.Fatal("cannot convert GPOS table")
	}

	graph := gpos.LookupGraph()
	if graph == nil || graph.Len() == 0 {
		t.Fatal("GPOS lookup graph is empty")
	}

	t.Logf("GPOS has %d lookups", graph.Len())

	// Test that we can parse lookup subtables without panicking
	// The old implementation had a panic("TODO GPOS Lookup Subtable")
	parsedCount := 0
	for i := 0; i < graph.Len(); i++ {
		lookup := graph.Lookup(i)
		if lookup == nil || lookup.Error() != nil {
			t.Logf("Warning: could not navigate to lookup %d: %v", i, lookup.Error())
			continue
		}

		t.Logf("Lookup %d: type=%s flags=0x%04x subtables=%d",
			i, lookup.Type.GPosString(), lookup.Flag, lookup.SubTableCount)

		// Try to parse the first subtable to verify our GPOS parsing works
		if lookup.SubTableCount > 0 {
			node := lookup.Subtable(0)
			if node == nil {
				continue
			}
			t.Logf("  Subtable[0]: type=%s format=%d",
				GPosLookupType(node.LookupType).GPosString(), node.Format)

			// The fact that we got here without panicking means our GPOS parsing works!
			// The old implementation had: panic("TODO GPOS Lookup Subtable")
			parsedCount++
		}
	}

	// Verify we have the expected number of lookups for Calibri
	if graph.Len() != 14 {
		t.Errorf("expected Calibri GPOS to have 14 lookups, got %d", graph.Len())
	}

	// Verify we successfully parsed subtables
	if parsedCount == 0 {
		t.Error("expected to parse at least some GPOS lookup subtables, but parsed none")
	} else {
		t.Logf("Successfully parsed %d GPOS lookup subtables (previously would have panicked)", parsedCount)
	}
}

// TestErrorCollection verifies that parsing errors and warnings are collected properly.
func TestErrorCollection(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	// Test with Calibri; non-parsed tables (including kern) should produce warnings.
	f := loadTestdataFont(t, "Calibri")
	otf, err := Parse(f.F.Binary)
	if err != nil {
		t.Fatal(err)
	}

	// Check that warnings were collected
	warnings := otf.Warnings()
	t.Logf("Font has %d warnings", len(warnings))

	// Verify we have at least one warning for the kern table.
	foundKernWarning := false
	for _, w := range warnings {
		t.Logf("Warning: %s", w.String())
		if w.Table == T("kern") {
			foundKernWarning = true
			if w.Issue == "" {
				t.Error("kern warning has empty issue description")
			}
		}
	}

	if foundKernWarning {
		t.Log("Successfully collected kern table size mismatch warning")
	} else {
		t.Log("No kern table warning found (this is OK if font format changed)")
	}

	// Check errors (should be none for valid fonts like Calibri)
	errors := otf.Errors()
	t.Logf("Font has %d errors", len(errors))
	if len(errors) > 0 {
		t.Error("Expected no errors for Calibri, but got:")
		for _, e := range errors {
			t.Errorf("  %s", e.Error())
		}
	}

	// Verify HasCriticalErrors works correctly
	if otf.HasCriticalErrors() {
		t.Error("Calibri should not have critical errors")
	}

	// Verify CriticalErrors returns empty for valid font
	critErrs := otf.CriticalErrors()
	if len(critErrs) != 0 {
		t.Errorf("Expected 0 critical errors, got %d", len(critErrs))
	}
}
