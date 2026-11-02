package hbcmp

import (
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
)

// Agreed contract (SHAPE-021): settings made in CollectFeatures are engine
// defaults which user feature settings override; OverrideFeatures runs after
// the user settings and may override them, as in HarfBuzz's collect_features
// and override_features.

// featurePlanEngine is the core engine plus feature-planning hooks.
type featurePlanEngine struct {
	otcore.Shaper
	collect  func(otshape.FeaturePlanner)
	override func(otshape.FeaturePlanner)
}

var _ otshape.ShapingEnginePlanHooks = featurePlanEngine{}

func (featurePlanEngine) Name() string { return "feature-plan-probe" }

// New must return the hooks too; the embedded core engine's New would not.
func (e featurePlanEngine) New() otshape.ShapingEngine { return e }

func (e featurePlanEngine) CollectFeatures(plan otshape.FeaturePlanner, _ otshape.SelectionContext) {
	if e.collect != nil {
		e.collect(plan)
	}
}

func (e featurePlanEngine) OverrideFeatures(plan otshape.FeaturePlanner) {
	if e.override != nil {
		e.override(plan)
	}
}

func (featurePlanEngine) InitPlan(otshape.PlanContext) {}

var (
	enableLiga  = func(p otshape.FeaturePlanner) { p.EnableFeature(ot.T("liga")) }
	disableLiga = func(p otshape.FeaturePlanner) { p.DisableFeature(ot.T("liga")) }
)

// Each case shapes Gentium "office" and expects the HarfBuzz capture whose
// effective liga setting matches the contract's outcome. HarfBuzz enables liga
// by default, so latin_office_liga_on (liga=1) is also the no-user-setting result.
func TestFeaturePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		engine  featurePlanEngine
		user    []string
		oracle  string
		changes []scopeChange
	}{
		{"collect_enable_without_user_setting", featurePlanEngine{collect: enableLiga}, nil, "latin_office_liga_on", nil},
		{"collect_enable_user_off", featurePlanEngine{collect: enableLiga}, []string{"liga=0"}, "latin_office_liga_off", nil},
		{"collect_disable_user_on", featurePlanEngine{collect: disableLiga}, []string{"liga=1"}, "latin_office_liga_on", nil},
		{"override_disable_user_on", featurePlanEngine{override: disableLiga}, []string{"liga=1"}, "latin_office_liga_off", nil},
		{"override_enable_user_off", featurePlanEngine{override: enableLiga}, []string{"liga=0"}, "latin_office_liga_on", nil},
		// The sole ligature in office starts and ends within [1:4]. Thus these
		// ranged settings have the same complete output as the global capture.
		{"collect_disable_user_range_on", featurePlanEngine{collect: disableLiga}, []string{"liga[1:4]=1"}, "latin_office_liga_on", []scopeChange{scopePush(1, "liga", true), {at: 4}}},
		{"collect_enable_user_range_off", featurePlanEngine{collect: enableLiga}, []string{"liga[1:4]=0"}, "latin_office_liga_off", []scopeChange{scopePush(1, "liga", false), {at: 4}}},
		{"override_disable_user_range_on", featurePlanEngine{override: disableLiga}, []string{"liga=0", "liga[1:4]=1"}, "latin_office_liga_off", []scopeChange{scopePush(1, "liga", true), {at: 4}}},
		{"override_enable_user_range_off", featurePlanEngine{override: enableLiga}, []string{"liga=1", "liga[1:4]=0"}, "latin_office_liga_on", []scopeChange{scopePush(1, "liga", false), {at: 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runPrecedenceBaseline(t, textBaselineByID(t, tc.oracle), tc.user, tc.engine, tc.changes)
		})
	}
}

func TestFeaturePrecedenceNestedScopes(t *testing.T) {
	for _, c := range loadStreamingBaselines(t) {
		if c.ID != "scope_nested" {
			continue
		}
		// Replace the capture's explicit global liga=1 with an engine default.
		// Both APIs must preserve it outside the user ranges and after pop.
		runPrecedenceBaseline(t, c, []string{"liga[0:21]=0", "liga[7:14]=1"}, featurePlanEngine{collect: enableLiga}, scopeRecipe(c.ID))
		return
	}
	t.Fatal("missing scope_nested capture")
}

func TestFeaturePrecedenceAlternateValues(t *testing.T) {
	setSalt := func(value uint32) func(otshape.FeaturePlanner) {
		return func(p otshape.FeaturePlanner) { p.AddFeature(ot.T("salt"), otshape.FeatureNone, value) }
	}
	for _, tc := range []struct {
		name   string
		engine featurePlanEngine
		user   []string
	}{
		{"user_value_over_collect", featurePlanEngine{collect: setSalt(2)}, []string{"salt=1", "ss01=0"}},
		{"override_value_over_user", featurePlanEngine{override: setSalt(1)}, []string{"salt=2", "ss01=0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runPrecedenceBaseline(t, textBaselineByID(t, "latin_shared_alternate_salt"), tc.user, tc.engine, nil)
		})
	}
}

// The oracle is always an unchanged HB capture. Callers state why the effective
// feature settings select that output. Events are built from an explicit scope
// recipe independently of the shaper; Shape receives the equivalent ranges.
func runPrecedenceBaseline(t *testing.T, c textBaseline, user []string, engine featurePlanEngine, changes []scopeChange) {
	t.Helper()
	t.Run("runes", func(t *testing.T) { runTextBaselineWith(t, c, user, engine) })
	t.Run("events", func(t *testing.T) {
		font, err := ot.Parse(c.fontData)
		if err != nil {
			t.Fatal(err)
		}
		f := c.Fixture
		f.Context.Features = nil
		params, err := f.params(font)
		if err != nil {
			t.Fatal(err)
		}
		features, err := parseTextFeatures(user, len(f.Input))
		if err != nil {
			t.Fatal(err)
		}
		for _, feature := range features {
			if feature.Start == 0 && feature.End == 0 {
				params.Features = append(params.Features, feature)
			}
		}
		source := &streamSource{runes: []rune(c.Text)}
		cursor, depth := 0, 0
		for i := 0; i <= len(source.runes); i++ {
			for cursor < len(changes) && changes[cursor].at == i {
				change := changes[cursor]
				if change.settings == nil {
					depth--
					if depth < 0 {
						t.Fatal("scope recipe underflow")
					}
					source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventPopFeatures})
				} else {
					depth++
					source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventPushFeatures, Push: change.settings})
				}
				cursor++
			}
			if i < len(source.runes) {
				source.events = append(source.events, otshape.InputEvent{Kind: otshape.InputEventRune, Rune: source.runes[i]})
			}
		}
		if depth != 0 || cursor != len(changes) {
			t.Fatal("invalid scope recipe")
		}
		n := len(f.Input)
		sink := &glyphCollector{}
		err = otshape.NewShaper(engine).ShapeEvents(params, source, sink, otshape.BufferOptions{HighWatermark: n + 1, LowWatermark: 1, MaxBuffer: n + 2})
		got := make([]hbShapedGlyph, len(sink.glyphs))
		for i, g := range sink.glyphs {
			got[i] = glyphRecord2hbShapedGlyph(g)
		}
		want := textLogicalOutput(f)
		if mismatch := comparePositionedGlyphs(got, want); err != nil || mismatch != nil {
			t.Fatalf("%s features=%s error=%v comparison=%v\ngot=%+v\nwant=%+v", c.ID, strings.Join(user, ","), err, mismatch, got, want)
		}
	})
}
