package hbcmp

import (
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
var featurePrecedenceCases = []struct {
	name     string
	engine   featurePlanEngine
	user     []string // user feature settings
	oracle   string   // text baseline ID
	knownBug bool
}{
	{"collect_enable_without_user_setting", featurePlanEngine{collect: enableLiga}, nil, "latin_office_liga_on", false},
	{"collect_enable_user_off", featurePlanEngine{collect: enableLiga}, []string{"liga=0"}, "latin_office_liga_off", false},
	{"collect_disable_user_on", featurePlanEngine{collect: disableLiga}, []string{"liga=1"}, "latin_office_liga_on", true},
	{"override_disable_user_on", featurePlanEngine{override: disableLiga}, []string{"liga=1"}, "latin_office_liga_off", false},
	{"override_enable_user_off", featurePlanEngine{override: enableLiga}, []string{"liga=0"}, "latin_office_liga_on", true},
}

func runFeaturePrecedenceCases(t *testing.T, knownBugs bool) {
	for _, tc := range featurePrecedenceCases {
		if tc.knownBug != knownBugs {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			runTextBaselineWith(t, textBaselineByID(t, tc.oracle), tc.user, tc.engine)
		})
	}
}

// Controls: cases where current behavior already meets the contract. A user's
// "off" currently wins through the glyph mask, although CollectFeatures keeps
// the lookups in the plan.
func TestFeaturePrecedence(t *testing.T) {
	runFeaturePrecedenceCases(t, false)
}
