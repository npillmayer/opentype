package otshape

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/unicode/bidi"
)

type planHookProbe struct {
	collectCalled     bool
	overrideCalled    bool
	initCalled        bool
	postResolveCalled bool

	lastSelection SelectionContext
	lastInitMask  uint32
	postSeenGSUB  bool
	postSeenGPOS  bool

	postAddBeforeTag ot.Tag
	postAddAfterTag  ot.Tag
}

func (p *planHookProbe) Name() string { return "plan-hook-probe" }
func (p *planHookProbe) Match(SelectionContext) ShaperConfidence {
	return ShaperConfidenceLow
}
func (p *planHookProbe) New() ShapingEngine { return p }

func (p *planHookProbe) CollectFeatures(plan FeaturePlanner, ctx SelectionContext) {
	p.collectCalled = true
	p.lastSelection = ctx
	plan.EnableFeature(ot.T("test"))
}

func (p *planHookProbe) OverrideFeatures(plan FeaturePlanner) {
	p.overrideCalled = true
	plan.DisableFeature(ot.T("liga"))
}

func (p *planHookProbe) InitPlan(plan PlanContext) {
	p.initCalled = true
	p.lastInitMask = plan.FeatureMask1(ot.T("test"))
}

func (p *planHookProbe) PostResolveFeatures(plan ResolvedFeaturePlanner, view ResolvedFeatureView, _ SelectionContext) {
	p.postResolveCalled = true
	p.postSeenGSUB = view.HasSelectedFeature(LayoutGSUB, ot.T("test"))
	p.postSeenGPOS = view.HasSelectedFeature(LayoutGPOS, ot.T("test"))
	if p.postAddBeforeTag != 0 {
		plan.AddGSUBPauseBefore(p.postAddBeforeTag, func(PauseContext) error { return nil })
	}
	if p.postAddAfterTag != 0 {
		plan.AddGSUBPauseAfter(p.postAddAfterTag, func(PauseContext) error { return nil })
	}
}

type fallbackProbe struct {
	tag          ot.Tag
	fallbackFlag bool
	initCalled   bool
	needsFbk     bool
}

func (p *fallbackProbe) Name() string { return "fallback-probe" }
func (p *fallbackProbe) Match(SelectionContext) ShaperConfidence {
	return ShaperConfidenceLow
}
func (p *fallbackProbe) New() ShapingEngine { return p }

func (p *fallbackProbe) CollectFeatures(plan FeaturePlanner, _ SelectionContext) {
	flags := FeatureNone
	if p.fallbackFlag {
		flags |= FeatureHasFallback
	}
	plan.AddFeature(p.tag, flags, 1)
}

func (p *fallbackProbe) OverrideFeatures(FeaturePlanner) {}

func (p *fallbackProbe) InitPlan(plan PlanContext) {
	p.initCalled = true
	p.needsFbk = plan.FeatureNeedsFallback(p.tag)
}

type validatingProbe struct {
	fallbackProbe
	validateCalled bool
	validateErr    error
}

func (p *validatingProbe) ValidatePlan(PlanContext) error {
	p.validateCalled = true
	return p.validateErr
}

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

func TestCompileInvokesPlanHooksAndSelectsHookEnabledFeature(t *testing.T) {
	otf := loadMiniOTFont(t, "gpos3_font1.otf")
	probe := &planHookProbe{}
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Selection: SelectionContext{
			Direction: bidi.LeftToRight,
			ScriptTag: ot.T("latn"),
			LangTag:   ot.T("ENG"),
		},
		Engine: probe,
		Policy: planPolicy{ApplyGPOS: true},
	}
	p, err := compile(req)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if !probe.collectCalled || !probe.overrideCalled || !probe.postResolveCalled || !probe.initCalled {
		t.Fatalf("plan hook calls: collect=%t override=%t postResolve=%t init=%t",
			probe.collectCalled, probe.overrideCalled, probe.postResolveCalled, probe.initCalled)
	}
	if !probe.postSeenGPOS {
		t.Fatalf("post-resolve view did not report selected GPOS feature 'test'")
	}
	if probe.lastSelection.ScriptTag != ot.T("latn") || probe.lastSelection.LangTag != ot.T("ENG") {
		t.Fatalf("unexpected selection context propagated to CollectFeatures: script=%s lang=%s",
			probe.lastSelection.ScriptTag, probe.lastSelection.LangTag)
	}
	if p.GPOS.lookupCount() == 0 {
		t.Fatalf("expected hook-enabled GPOS feature to produce lookup program")
	}
}

func TestCompilePostResolveCanAnchorGSUBPause(t *testing.T) {
	otf := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	probe := &planHookProbe{
		postAddAfterTag: ot.T("test"),
	}
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Selection: SelectionContext{
			Direction: bidi.LeftToRight,
			ScriptTag: ot.T("latn"),
			LangTag:   ot.T("ENG"),
		},
		Engine: probe,
		Policy: planPolicy{ApplyGPOS: true},
	}
	p, err := compile(req)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if !probe.postSeenGSUB {
		t.Fatalf("post-resolve view did not report selected GSUB feature 'test'")
	}
	hasPause := false
	for _, st := range p.GSUB.Stages {
		if st.Pause != noPauseHook {
			hasPause = true
			break
		}
	}
	if !hasPause {
		t.Fatalf("expected post-resolve anchor to attach at least one GSUB pause stage")
	}
}

func TestCompileSetsFallbackNeedForMissingFallbackFeature(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	probe := &fallbackProbe{
		tag:          ot.T("init"),
		fallbackFlag: true,
	}
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Engine: probe,
		Policy: planPolicy{ApplyGPOS: true},
	}
	if _, err := compile(req); err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if !probe.initCalled {
		t.Fatalf("InitPlan was not called")
	}
	if !probe.needsFbk {
		t.Fatalf("expected fallback-needed=true for missing fallback feature %s", probe.tag)
	}
}

func TestCompileDoesNotSetFallbackNeedWhenFallbackFeatureIsResolved(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	probe := &fallbackProbe{
		tag:          ot.T("liga"),
		fallbackFlag: true,
	}
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Engine: probe,
		Policy: planPolicy{ApplyGPOS: true},
	}
	if _, err := compile(req); err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if !probe.initCalled {
		t.Fatalf("InitPlan was not called")
	}
	if probe.needsFbk {
		t.Fatalf("expected fallback-needed=false for resolved feature %s", probe.tag)
	}
}

func TestCompileDoesNotSetFallbackNeedForUnknownTagResolvedInOneTable(t *testing.T) {
	otf := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	probe := &fallbackProbe{
		tag:          ot.T("test"),
		fallbackFlag: true,
	}
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Engine: probe,
		Policy: planPolicy{ApplyGPOS: true},
	}
	if _, err := compile(req); err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if !probe.initCalled {
		t.Fatalf("InitPlan was not called")
	}
	if probe.needsFbk {
		t.Fatalf("expected fallback-needed=false when feature %s resolves in one table", probe.tag)
	}
}

func TestCompileInvokesPlanValidateHook(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	wantErr := errShaper("validate hook failure")
	probe := &validatingProbe{
		fallbackProbe: fallbackProbe{
			tag:          ot.T("init"),
			fallbackFlag: true,
		},
		validateErr: wantErr,
	}
	req := planRequest{
		Font:      otf,
		ScriptTag: ot.T("latn"),
		LangTag:   ot.T("ENG"),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Engine: probe,
		Policy: planPolicy{ApplyGPOS: true},
	}
	_, err := compile(req)
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("compile error = %v, want %v", err, wantErr)
	}
	if !probe.validateCalled {
		t.Fatalf("expected ValidatePlan to be called")
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

func TestPlanExecutorZeroMarksByAttachment(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 11)
	run.Pos = otlayout.NewPosBuffer(2)
	run.Pos[0].XAdvance = 500
	run.Pos[1].XAdvance = 80
	run.Pos[1].YAdvance = 12
	run.Pos[1].AttachKind = otlayout.AttachMarkToBase
	run.Pos[1].AttachTo = 0

	exec := &planExecutor{}
	exec.acquireBuffer(run)
	defer exec.releaseBuffer()

	p := &plan{
		Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}},
		Hooks: newPlanHookSet(),
		Policy: planPolicy{
			ApplyGPOS: true,
			ZeroMarks: true,
		},
	}
	if err := exec.apply(p); err != nil {
		t.Fatalf("executor apply failed: %v", err)
	}
	if run.Pos[0].XAdvance != 500 {
		t.Fatalf("base advance changed unexpectedly: got %d, want 500", run.Pos[0].XAdvance)
	}
	if run.Pos[1].XAdvance != 0 || run.Pos[1].YAdvance != 0 {
		t.Fatalf("mark advances not zeroed: got xa=%d ya=%d, want 0/0",
			run.Pos[1].XAdvance, run.Pos[1].YAdvance)
	}
}

func TestPlanExecutorFallbackMarkPositionAndZeroing(t *testing.T) {
	otf := loadLocalFont(t, "Calibri.ttf")
	base := otquery.GlyphIndex(otf, 'A')
	mark := otquery.GlyphIndex(otf, '\u0301')
	if base == NOTDEF || mark == NOTDEF {
		t.Skip("font does not expose expected base/mark glyphs for fallback test")
	}

	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, base, mark)
	run.Pos = otlayout.NewPosBuffer(2)
	run.Pos[0].XAdvance = 620
	run.Pos[1].XAdvance = 40

	exec := &planExecutor{}
	exec.acquireBuffer(run)
	defer exec.releaseBuffer()

	p := &plan{
		font:  otf,
		Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}},
		Hooks: newPlanHookSet(),
		Props: segmentProps{
			Direction: bidi.LeftToRight,
		},
		Policy: planPolicy{
			ApplyGPOS:       false,
			ZeroMarks:       true,
			FallbackMarkPos: true,
		},
	}
	if err := exec.apply(p); err != nil {
		t.Fatalf("executor apply failed: %v", err)
	}
	markPos := run.Pos[1]
	if markPos.AttachKind != otlayout.AttachMarkToBase || markPos.AttachTo != 0 {
		t.Fatalf("fallback mark attachment not applied: kind=%d to=%d",
			markPos.AttachKind, markPos.AttachTo)
	}
	if markPos.XAdvance != 0 || markPos.YAdvance != 0 {
		t.Fatalf("mark advances not zeroed: got xa=%d ya=%d, want 0/0",
			markPos.XAdvance, markPos.YAdvance)
	}
	if markPos.XOffset != -40 {
		t.Fatalf("mark offset not adjusted while zeroing: got %d, want -40", markPos.XOffset)
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
		map[ot.Tag]FeatureFlags{},
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
		map[ot.Tag]FeatureFlags{},
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

func TestCollectUserFeatureTogglesSeparatesGlobalAndRange(t *testing.T) {
	toggles := collectUserFeatureToggles([]FeatureRange{
		{Feature: ot.T("kern"), On: false, Start: 1, End: 3},
		{Feature: ot.T("kern"), On: true, Start: 3, End: 5},
		{Feature: ot.T("liga"), On: false},
	})
	kern, ok := toggles[ot.T("kern")]
	if !ok {
		t.Fatalf("kern toggle missing")
	}
	if kern.hasGlobal {
		t.Fatalf("range-only kern toggle must not be global")
	}
	if !kern.hasRange || !kern.hasRangeOn || !kern.hasRangeOff || !kern.hasAnyOn {
		t.Fatalf("unexpected kern range flags: %+v", kern)
	}
	liga, ok := toggles[ot.T("liga")]
	if !ok {
		t.Fatalf("liga toggle missing")
	}
	if !liga.hasGlobal || liga.on {
		t.Fatalf("expected global liga off toggle, got %+v", liga)
	}
}

func TestCompileUserFeatureMasksRangeDefaults(t *testing.T) {
	layout, err := compileUserFeatureMasks([]FeatureRange{
		{Feature: ot.T("kern"), On: true, Start: 1, End: 3},
		{Feature: ot.T("liga"), On: false, Start: 1, End: 3},
	})
	if err != nil {
		t.Fatalf("compileUserFeatureMasks failed: %v", err)
	}
	kern, ok := layout.ByFeature[ot.T("kern")]
	if !ok {
		t.Fatalf("kern mask spec missing")
	}
	liga, ok := layout.ByFeature[ot.T("liga")]
	if !ok {
		t.Fatalf("liga mask spec missing")
	}
	if kern.DefaultValue != 0 {
		t.Fatalf("range-on-only feature should default to off, got %d", kern.DefaultValue)
	}
	if liga.DefaultValue != 1 {
		t.Fatalf("range-off-only feature should default to on, got %d", liga.DefaultValue)
	}
	kernDefaultBits := (layout.GlobalMask & kern.Mask) >> kern.Shift
	ligaDefaultBits := (layout.GlobalMask & liga.Mask) >> liga.Shift
	if kernDefaultBits != 0 || ligaDefaultBits != 1 {
		t.Fatalf("unexpected global defaults kern=%d liga=%d", kernDefaultBits, ligaDefaultBits)
	}
}

func TestCompileTableProgramRangeOnKeepsFeatureActive(t *testing.T) {
	features := []otlayout.Feature{
		fakeFeature{tag: ot.T("test"), typ: otlayout.GPosFeatureType, lookups: []int{0}},
	}
	toggles := collectUserFeatureToggles([]FeatureRange{
		{Feature: ot.T("test"), On: true, Start: 1, End: 2},
	})
	prog, _, err := compileTableProgram(
		features,
		planGPOS,
		nil,
		toggles,
		map[ot.Tag]FeatureFlags{},
		maskLayout{ByFeature: map[ot.Tag]maskSpec{
			ot.T("test"): {Mask: 1, Shift: 0, DefaultValue: 0},
		}},
		planPolicy{},
	)
	if err != nil {
		t.Fatalf("compileTableProgram failed: %v", err)
	}
	if len(prog.Lookups) == 0 {
		t.Fatalf("range-on feature should remain active in program")
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
	path := filepath.Join("..", "testdata", "fonttools", filename)
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
