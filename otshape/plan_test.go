package otshape

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"golang.org/x/text/unicode/bidi"
)

func TestPlanCompileLookupOrdering(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Policy: planPolicy{ApplyGPOS: true},
	}
	p, err := compile(req)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	assertStagePartition(t, "GSUB", p.GSUB)
	assertStagePartition(t, "GPOS", p.GPOS)
}

func TestPlanCompileStrictMissingFeatureFails(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Policy: planPolicy{
			Strict:    true,
			ApplyGPOS: true,
		},
		UserFeatures: []FeatureRange{
			{Feature: ot.T("zzzz"), On: true},
		},
	}
	_, err := compile(req)
	if err == nil {
		t.Fatalf("expected strict compile error for missing feature")
	}
}

func TestPlanCompileUserDisableRemovesFeature(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	baseReq := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Policy:    planPolicy{ApplyGPOS: true},
	}
	base, err := compile(baseReq)
	if err != nil {
		t.Fatalf("baseline compile failed: %v", err)
	}
	var chosen ot.Tag
	for _, fb := range base.GSUB.FeatureBinds {
		if !fb.Required {
			chosen = fb.Tag
			break
		}
	}
	if chosen == 0 {
		t.Skip("font exposes no non-required GSUB features in current defaults")
	}

	offReq := baseReq
	offReq.UserFeatures = []FeatureRange{
		{Feature: chosen, On: false},
	}
	off, err := compile(offReq)
	if err != nil {
		t.Fatalf("compile with disabled feature failed: %v", err)
	}
	if containsFeatureBind(off.GSUB.FeatureBinds, chosen) {
		t.Fatalf("feature %s still active after explicit disable", chosen)
	}
}

func TestPlanCompileRangeFeatureDoesNotEmitGlobalOnlyWarning(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Policy:    planPolicy{ApplyGPOS: true},
		UserFeatures: []FeatureRange{
			{Feature: ot.T("liga"), On: true, Start: 1, End: 5},
		},
	}
	p, err := compile(req)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	for _, n := range p.Notes {
		if strings.Contains(n.Message, "global-only") {
			t.Fatalf("unexpected obsolete PR1 warning note: %q", n.Message)
		}
	}
}

func TestPlanExecutorStagePauseOrder(t *testing.T) {
	run := newRunBuffer(0)
	exec := &planExecutor{}
	exec.acquireBuffer(run)
	defer exec.releaseBuffer()

	var order []int
	hooks := newPlanHookSet()
	p1 := hooks.addPause(func(run *runBuffer) error {
		order = append(order, 1)
		return nil
	})
	p2 := hooks.addPause(func(run *runBuffer) error {
		order = append(order, 2)
		return nil
	})

	p := &plan{
		Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}},
		Hooks: hooks,
		Policy: planPolicy{
			ApplyGPOS: false,
		},
		GSUB: tableProgram{
			Stages: []stage{
				{FirstLookup: 0, LastLookup: 0, Pause: p1},
				{FirstLookup: 0, LastLookup: 0, Pause: p2},
			},
		},
	}
	if err := exec.apply(p); err != nil {
		t.Fatalf("executor apply failed: %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("pause hook order = %v, want [1 2]", order)
	}
}

type fakeFeature struct {
	tag     ot.Tag
	typ     otlayout.LayoutTagType
	lookups []int
}

func (f fakeFeature) Tag() ot.Tag                  { return f.tag }
func (f fakeFeature) Type() otlayout.LayoutTagType { return f.typ }
func (f fakeFeature) LookupCount() int             { return len(f.lookups) }
func (f fakeFeature) LookupIndex(i int) int {
	if i < 0 || i >= len(f.lookups) {
		return -1
	}
	return f.lookups[i]
}

func TestCompileTableProgramBuildsMultipleStagesAndRandomFlag(t *testing.T) {
	features := []otlayout.Feature{
		fakeFeature{tag: ot.T("rlig"), typ: otlayout.GSubFeatureType, lookups: []int{4}}, // required slot 0
		fakeFeature{tag: ot.T("liga"), typ: otlayout.GSubFeatureType, lookups: []int{2, 5}},
		fakeFeature{tag: ot.T("calt"), typ: otlayout.GSubFeatureType, lookups: []int{3}},
		fakeFeature{tag: ot.T("rand"), typ: otlayout.GSubFeatureType, lookups: []int{6}},
	}
	masks := maskLayout{
		ByFeature: map[ot.Tag]maskSpec{
			ot.T("rlig"): {Mask: 1, Shift: 0},
			ot.T("liga"): {Mask: 2, Shift: 1},
			ot.T("calt"): {Mask: 4, Shift: 2},
			ot.T("rand"): {Mask: 8, Shift: 3},
		},
	}
	prog, _, err := compileTableProgram(
		features,
		planGSUB,
		[]ot.Tag{ot.T("liga"), ot.T("calt"), ot.T("rand")},
		map[ot.Tag]userFeatureToggle{},
		masks,
		planPolicy{},
	)
	if err != nil {
		t.Fatalf("compileTableProgram failed: %v", err)
	}
	if len(prog.Stages) < 3 {
		t.Fatalf("expected multiple stages, got %d", len(prog.Stages))
	}
	assertStagePartition(t, "GSUB/fake", prog)
	if !containsFeatureBind(prog.FeatureBinds, ot.T("rand")) {
		t.Fatalf("rand feature bind missing")
	}
	var randFlagged bool
	for _, op := range prog.Lookups {
		if op.FeatureTag == ot.T("rand") && op.Flags.has(lookupRandom) {
			randFlagged = true
		}
	}
	if !randFlagged {
		t.Fatalf("expected lookupRandom flag on rand feature lookup")
	}
}

func TestCompileTableProgramAssignsJoinerAndSyllableFlags(t *testing.T) {
	features := []otlayout.Feature{
		fakeFeature{tag: ot.T("mark"), typ: otlayout.GSubFeatureType, lookups: []int{1}},
		fakeFeature{tag: ot.T("rphf"), typ: otlayout.GSubFeatureType, lookups: []int{2}},
		fakeFeature{tag: ot.T("rand"), typ: otlayout.GSubFeatureType, lookups: []int{3}},
	}
	prog, _, err := compileTableProgram(
		features,
		planGSUB,
		[]ot.Tag{ot.T("mark"), ot.T("rphf"), ot.T("rand")},
		map[ot.Tag]userFeatureToggle{},
		maskLayout{ByFeature: map[ot.Tag]maskSpec{}},
		planPolicy{},
	)
	if err != nil {
		t.Fatalf("compileTableProgram failed: %v", err)
	}

	var (
		seenMark bool
		seenRphf bool
		seenRand bool
	)
	for _, op := range prog.Lookups {
		switch op.FeatureTag {
		case ot.T("mark"):
			seenMark = true
			if op.Flags.has(lookupAutoZWJ) || op.Flags.has(lookupAutoZWNJ) {
				t.Fatalf("mark lookup flags = %08b, expected manual joiner handling", op.Flags)
			}
		case ot.T("rphf"):
			seenRphf = true
			if !op.Flags.has(lookupPerSyllable) {
				t.Fatalf("rphf lookup flags = %08b, expected per-syllable", op.Flags)
			}
		case ot.T("rand"):
			seenRand = true
			if !op.Flags.has(lookupRandom) {
				t.Fatalf("rand lookup flags = %08b, expected random alternate support", op.Flags)
			}
		}
	}
	if !seenMark || !seenRphf || !seenRand {
		t.Fatalf("compiled lookups missing expected features: mark=%t rphf=%t rand=%t", seenMark, seenRphf, seenRand)
	}
}

func TestApplyFeatureRangesToMasks(t *testing.T) {
	masks := []uint32{1, 1, 1, 1, 1}
	specs := map[ot.Tag]maskSpec{
		ot.T("liga"): {Mask: 0x6, Shift: 1},
	}
	ranges := []FeatureRange{
		{Feature: ot.T("liga"), On: true, Arg: 1, Start: 1, End: 4},
		{Feature: ot.T("liga"), On: false, Start: 2, End: 3},
	}
	applyFeatureRangesToMasks(masks, specs, ranges)
	want := []uint32{
		1, // untouched
		3, // global(1) + liga(1<<1)
		1, // toggled off in second range
		3, // still on
		1, // untouched
	}
	for i := range want {
		if masks[i] != want[i] {
			t.Fatalf("mask[%d] = 0x%x, want 0x%x", i, masks[i], want[i])
		}
	}
}

func TestEnsureRunMasksUsesGlobalAndRanges(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11, 12, 13)
	exec := &planExecutor{run: run}
	pl := &plan{
		Masks: maskLayout{
			GlobalMask: 1,
			ByFeature: map[ot.Tag]maskSpec{
				ot.T("liga"): {Mask: 0x6, Shift: 1},
			},
		},
		featureRanges: []FeatureRange{
			{Feature: ot.T("liga"), On: true, Arg: 1, Start: 1, End: 3},
		},
	}
	exec.ensureRunMasks(pl)
	want := []uint32{1, 3, 3, 1}
	if len(run.Masks) != len(want) {
		t.Fatalf("mask length = %d, want %d", len(run.Masks), len(want))
	}
	for i := range want {
		if run.Masks[i] != want[i] {
			t.Fatalf("mask[%d] = 0x%x, want 0x%x", i, run.Masks[i], want[i])
		}
	}
}

func TestRealignSideArraysAfterLengthChange(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11)
	run.Clusters = []uint32{0, 1}
	run.Masks = []uint32{7, 7}
	run.UnsafeFlags = []uint16{1, 1}
	run.Syllables = []uint16{3, 4}
	run.Joiners = []uint8{0, joinerClassZWJ}

	exec := &planExecutor{run: run}
	st := otlayout.NewBufferState(otlayout.GlyphBuffer{10, 11, 12, 13}, nil)
	pl := &plan{
		Masks: maskLayout{
			GlobalMask: 5,
			ByFeature:  map[ot.Tag]maskSpec{},
		},
	}

	exec.realignSideArrays(pl, st)
	if run.Len() != 4 {
		t.Fatalf("run length = %d, want 4", run.Len())
	}
	if len(run.Clusters) != 4 || len(run.Masks) != 4 || len(run.UnsafeFlags) != 4 {
		t.Fatalf("side array lengths = clusters:%d masks:%d unsafe:%d, want all 4",
			len(run.Clusters), len(run.Masks), len(run.UnsafeFlags))
	}
	if len(run.Syllables) != 4 || len(run.Joiners) != 4 {
		t.Fatalf("side array lengths = syllables:%d joiners:%d, want both 4",
			len(run.Syllables), len(run.Joiners))
	}
	for i, m := range run.Masks {
		if m != 5 {
			t.Fatalf("mask[%d] = 0x%x, want 0x5", i, m)
		}
	}
}

func TestLookupShouldSkipJoiner(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11, 12)
	run.Joiners = []uint8{0, joinerClassZWNJ, joinerClassZWJ}
	exec := &planExecutor{run: run}

	if !exec.lookupShouldSkipJoiner(lookupOp{Flags: lookupAutoZWJ | lookupAutoZWNJ}, 1) {
		t.Fatalf("expected ZWNJ index to be skipped with autoZWNJ enabled")
	}
	if !exec.lookupShouldSkipJoiner(lookupOp{Flags: lookupAutoZWJ | lookupAutoZWNJ}, 2) {
		t.Fatalf("expected ZWJ index to be skipped with autoZWJ enabled")
	}
	if exec.lookupShouldSkipJoiner(lookupOp{Flags: lookupAutoZWJ}, 1) {
		t.Fatalf("did not expect ZWNJ index skip without autoZWNJ")
	}
	if exec.lookupShouldSkipJoiner(lookupOp{Flags: 0}, 2) {
		t.Fatalf("did not expect joiner skip when auto flags are disabled")
	}
}

func TestLookupSpanEndUsesSyllablesAndClustersFallback(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11, 12, 13)
	run.Syllables = []uint16{1, 1, 2, 2}
	run.Clusters = []uint32{9, 9, 9, 9}
	exec := &planExecutor{run: run}

	if end := exec.lookupSpanEnd(0, run.Len()); end != 2 {
		t.Fatalf("syllable span end = %d, want 2", end)
	}
	if end := exec.lookupSpanEnd(2, run.Len()); end != 4 {
		t.Fatalf("syllable span end = %d, want 4", end)
	}

	run.Syllables = nil
	run.Clusters = []uint32{3, 3, 4, 4}
	if end := exec.lookupSpanEnd(0, run.Len()); end != 2 {
		t.Fatalf("cluster span end = %d, want 2", end)
	}
	if end := exec.lookupSpanEnd(2, run.Len()); end != 4 {
		t.Fatalf("cluster span end = %d, want 4", end)
	}

	run.Clusters = nil
	if end := exec.lookupSpanEnd(1, run.Len()); end != 4 {
		t.Fatalf("fallback span end = %d, want 4", end)
	}
}

func TestApplyGSUBPreAnnotatesJoinersForLookupGating(t *testing.T) {
	otf := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	buildPlan := func(flags lookupRunFlags) *plan {
		return &plan{
			font:  otf,
			Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}},
			Hooks: newPlanHookSet(),
			GSUB: tableProgram{
				Stages: []stage{
					{FirstLookup: 0, LastLookup: 1, Pause: noPauseHook},
				},
				Lookups: []lookupOp{
					{
						LookupIndex: 0,
						FeatureTag:  ot.T("test"),
						Flags:       flags,
					},
				},
			},
			joinerGlyphClass: map[ot.GlyphIndex]uint8{
				18: joinerClassZWNJ,
			},
		}
	}

	t.Run("auto-joiner-skip", func(t *testing.T) {
		run := newRunBuffer(0)
		run.Glyphs = append(run.Glyphs, 18)
		exec := &planExecutor{}
		exec.acquireBuffer(run)
		defer exec.releaseBuffer()
		if err := exec.applyGSUB(buildPlan(lookupAutoZWNJ)); err != nil {
			t.Fatalf("applyGSUB failed: %v", err)
		}
		if len(run.Joiners) != 1 || run.Joiners[0] != joinerClassZWNJ {
			t.Fatalf("expected joiner annotation [ZWNJ], got %v", run.Joiners)
		}
		if got := run.Glyphs[0]; got != 18 {
			t.Fatalf("expected lookup skipped by joiner gate, got glyph %d", got)
		}
	})

	t.Run("manual-joiner-allowed", func(t *testing.T) {
		run := newRunBuffer(0)
		run.Glyphs = append(run.Glyphs, 18)
		exec := &planExecutor{}
		exec.acquireBuffer(run)
		defer exec.releaseBuffer()
		if err := exec.applyGSUB(buildPlan(0)); err != nil {
			t.Fatalf("applyGSUB failed: %v", err)
		}
		if got := run.Glyphs[0]; got != 20 {
			t.Fatalf("expected lookup application with manual joiner handling, got glyph %d", got)
		}
	})
}

func TestApplyGSUBPerSyllableUsesClusterDerivedAnnotations(t *testing.T) {
	otf := loadMiniOTFont(t, "gsub_context1_lookupflag_f1.otf")
	buildPlan := func(flags lookupRunFlags) *plan {
		return &plan{
			font:  otf,
			Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}},
			Hooks: newPlanHookSet(),
			GSUB: tableProgram{
				Stages: []stage{
					{FirstLookup: 0, LastLookup: 1, Pause: noPauseHook},
				},
				Lookups: []lookupOp{
					{
						LookupIndex: 4,
						FeatureTag:  ot.T("test"),
						Flags:       flags,
					},
				},
			},
		}
	}

	t.Run("split-syllables-block-contextual-match", func(t *testing.T) {
		run := newRunBuffer(0)
		run.Glyphs = append(run.Glyphs, 20, 21, 22)
		run.Clusters = []uint32{1, 2, 3}
		exec := &planExecutor{}
		exec.acquireBuffer(run)
		defer exec.releaseBuffer()
		if err := exec.applyGSUB(buildPlan(lookupPerSyllable)); err != nil {
			t.Fatalf("applyGSUB failed: %v", err)
		}
		wantGlyphs := []ot.GlyphIndex{20, 21, 22}
		for i := range wantGlyphs {
			if run.Glyphs[i] != wantGlyphs[i] {
				t.Fatalf("glyph[%d] = %d, want %d", i, run.Glyphs[i], wantGlyphs[i])
			}
		}
		wantSyll := []uint16{1, 2, 3}
		for i := range wantSyll {
			if run.Syllables[i] != wantSyll[i] {
				t.Fatalf("syllable[%d] = %d, want %d", i, run.Syllables[i], wantSyll[i])
			}
		}
	})

	t.Run("single-syllable-allows-contextual-match", func(t *testing.T) {
		run := newRunBuffer(0)
		run.Glyphs = append(run.Glyphs, 20, 21, 22)
		run.Clusters = []uint32{1, 1, 1}
		exec := &planExecutor{}
		exec.acquireBuffer(run)
		defer exec.releaseBuffer()
		if err := exec.applyGSUB(buildPlan(lookupPerSyllable)); err != nil {
			t.Fatalf("applyGSUB failed: %v", err)
		}
		wantGlyphs := []ot.GlyphIndex{60, 61, 62}
		for i := range wantGlyphs {
			if run.Glyphs[i] != wantGlyphs[i] {
				t.Fatalf("glyph[%d] = %d, want %d", i, run.Glyphs[i], wantGlyphs[i])
			}
		}
	})
}

func assertSortedUniqueLookups(t *testing.T, table string, lookups []lookupOp) {
	t.Helper()
	for i := 1; i < len(lookups); i++ {
		if lookups[i-1].LookupIndex >= lookups[i].LookupIndex {
			t.Fatalf("%s lookups are not strictly sorted at %d: %d then %d",
				table, i, lookups[i-1].LookupIndex, lookups[i].LookupIndex)
		}
	}
}

func assertStagePartition(t *testing.T, table string, prog tableProgram) {
	t.Helper()
	if len(prog.Lookups) == 0 {
		return
	}
	if len(prog.Stages) == 0 {
		t.Fatalf("%s has lookups but no stages", table)
	}
	prevEnd := 0
	for i, st := range prog.Stages {
		if st.FirstLookup != prevEnd {
			t.Fatalf("%s stage[%d] starts at %d, want %d", table, i, st.FirstLookup, prevEnd)
		}
		if st.LastLookup < st.FirstLookup || st.LastLookup > len(prog.Lookups) {
			t.Fatalf("%s stage[%d] bounds invalid [%d:%d) for %d lookups",
				table, i, st.FirstLookup, st.LastLookup, len(prog.Lookups))
		}
		assertSortedUniqueLookups(t, table, prog.Lookups[st.FirstLookup:st.LastLookup])
		prevEnd = st.LastLookup
	}
	if prevEnd != len(prog.Lookups) {
		t.Fatalf("%s stages end at %d, want %d", table, prevEnd, len(prog.Lookups))
	}
}

func containsFeatureBind(bindings []featureBind, tag ot.Tag) bool {
	for _, b := range bindings {
		if b.Tag == tag {
			return true
		}
	}
	return false
}

func loadMiniOTFont(t *testing.T, filename string) *ot.Font {
	t.Helper()
	path := filepath.Join("..", "testdata", "fonttools-tests", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mini font %s: %v", path, err)
	}
	otf, err := ot.Parse(data, ot.IsTestfont)
	if err != nil {
		t.Fatalf("parse mini font %s: %v", path, err)
	}
	return otf
}
