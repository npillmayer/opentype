package otshape

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestSHAPE020StageAssignment(t *testing.T) {
	a, b, required := ot.T("aaaa"), ot.T("bbbb"), ot.T("reqd")
	hooks := newPlanHookSet()
	planner := newPlanFeaturePlanner(nil, SelectionContext{}, &hooks, nil)
	planner.EnableFeature(a)
	planner.AddGSUBPause(nil)
	planner.AddGSUBPause(nil) // retain an empty stage
	planner.EnableFeature(a)  // earliest registration wins
	planner.EnableFeature(b)
	features := []otlayout.Feature{
		fakeFeature{tag: required, lookups: []int{4}},
		fakeFeature{tag: a, lookups: []int{3, 1}},
		fakeFeature{tag: b, lookups: []int{1, 0}},
		fakeFeature{tag: ot.T("liga"), lookups: []int{2}},
	}
	for _, table := range []planTable{planGSUB, planGPOS} {
		prog, _, err := compileTableProgram(features, table, []ot.Tag{a, b, ot.T("liga")}, nil, nil, maskLayout{}, planPolicy{}, planner.tableStages(table))
		if err != nil {
			t.Fatal(err)
		}
		var got [][]uint16
		for _, st := range prog.Stages {
			ids := []uint16{}
			for _, op := range prog.Lookups[st.FirstLookup:st.LastLookup] {
				ids = append(ids, op.LookupIndex)
			}
			got = append(got, ids)
		}
		want := [][]uint16{{1, 3, 4}, {}, {0, 1, 2}}
		if table == planGPOS {
			want = [][]uint16{{0, 1, 2, 3, 4}}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s stages=%v, want %v", table, got, want)
		}
	}
	// Explicit registration also controls required-feature placement.
	planner.EnableFeature(required)
	prog, _, err := compileTableProgram(features, planGSUB, nil, nil, nil, maskLayout{}, planPolicy{}, planner.tableStages(planGSUB))
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.FeatureBinds) != 1 || prog.FeatureBinds[0].Stage != 2 {
		t.Fatalf("required placement: %+v", prog.FeatureBinds)
	}
}

func TestSHAPE020SharedLookupMetadata(t *testing.T) {
	a, b := ot.T("aaaa"), ot.T("bbbb")
	for _, required := range []bool{false, true} {
		features := []otlayout.Feature{nil, fakeFeature{tag: a, lookups: []int{0}}, fakeFeature{tag: b, lookups: []int{0}}}
		if required {
			features = features[1:]
		}
		masks := maskLayout{ByFeature: map[ot.Tag]maskSpec{b: {Mask: 2}}}
		var toggles map[ot.Tag]userFeatureToggle
		if required {
			// Even an explicit disabled mask cannot gate the required feature.
			masks.ByFeature[a] = maskSpec{Mask: 4, Shift: 2}
			toggles = map[ot.Tag]userFeatureToggle{a: {hasGlobal: true, on: false}}
		}
		flags := map[ot.Tag]FeatureFlags{a: FeatureRandom, b: FeatureManualZWJ | FeatureManualZWNJ | FeaturePerSyllable}
		prog, _, err := compileTableProgram(features, planGSUB, []ot.Tag{b, a}, toggles, flags, masks, planPolicy{}, tableStagePlan{})
		if err != nil {
			t.Fatal(err)
		}
		if len(prog.Lookups) != 1 {
			t.Fatalf("shared lookup repeated: %+v", prog.Lookups)
		}
		op := prog.Lookups[0]
		// Unmasked/required contributor makes the union unconditional. HB 15
		// ANDs joiner flags and retains the first random/per-syllable flags.
		if op.Mask != 0 || op.Flags != lookupRandom || len(op.Features) != 2 {
			t.Fatalf("merged lookup: %+v", op)
		}
		view := newResolvedFeatureView(prog, tableProgram{}, nil, nil)
		if !view.HasSelectedFeature(LayoutGSUB, a) || !view.HasSelectedFeature(LayoutGSUB, b) {
			t.Fatal("shared contributor missing from view")
		}
		resolved := view.SelectedFeatures(LayoutGSUB)
		if !resolved[1].PerSyllable || resolved[1].AutoZWJ || resolved[1].SupportsRandom {
			t.Fatalf("per-feature flags lost: %+v", resolved)
		}
		hooks := newPlanHookSet()
		p := newResolvedPausePlanner(nil, &hooks, &prog)
		if !p.AddGSUBPauseBefore(b, func(PauseContext) error { return nil }) || !p.AddGSUBPauseAfter(a, func(PauseContext) error { return nil }) {
			t.Fatal("shared tags must anchor the containing stage")
		}
		if len(prog.Stages) != 2 || prog.Stages[0].LastLookup != 0 || prog.Stages[1].LastLookup != 1 {
			t.Fatalf("anchored stages: %+v", prog.Stages)
		}
		for _, bind := range prog.FeatureBinds {
			if bind.Stage != 1 {
				t.Fatalf("binding not shifted: %+v", bind)
			}
		}
	}
}

func TestSHAPE020DirectPauseExecution(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	for _, stop := range []bool{false, true} {
		hooks := newPlanHookSet()
		planner := newPlanFeaturePlanner(font, SelectionContext{}, &hooks, nil)
		var seen []ot.GlyphIndex
		cause := errors.New("pause stopped")
		observe := func(ctx PauseContext) error {
			seen = append(seen, ctx.Run().Glyph(0))
			if stop && len(seen) == 2 {
				return cause
			}
			return nil
		}
		planner.AddGSUBPause(observe) // empty initial stage
		planner.EnableFeature(ot.T("test"))
		planner.AddGSUBPause(observe)
		planner.AddGSUBPause(observe) // consecutive pause, empty stage
		features := []otlayout.Feature{nil, fakeFeature{tag: ot.T("test"), lookups: []int{0}}}
		prog, _, err := compileTableProgram(features, planGSUB, planner.defaultTags(planGSUB), planner.toggles(), nil, maskLayout{}, planPolicy{}, planner.tableStages(planGSUB))
		if err != nil {
			t.Fatal(err)
		}
		run := newRunBuffer(1)
		run.Glyphs = otlayout.GlyphBuffer{18}
		run.Clusters = []uint32{0}
		exec := &planExecutor{}
		exec.acquireBuffer(run)
		pl := &plan{font: font, GSUB: prog, Hooks: hooks, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
		exec.ensureRunMasks(pl)
		err = exec.applyGSUB(pl)
		exec.releaseBuffer()
		want := []ot.GlyphIndex{18, 20, 20}
		if stop {
			want = want[:2]
			if !errors.Is(err, cause) {
				t.Fatalf("error=%v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(seen, want) || run.Glyphs[0] != 20 {
			t.Fatalf("seen=%v glyphs=%v want=%v", seen, run.Glyphs, want)
		}
	}
}
