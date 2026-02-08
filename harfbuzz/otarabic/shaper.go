package otarabic

import (
	"errors"
	"fmt"

	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
	"github.com/npillmayer/opentype/harfbuzz"
)

type noOpHooks struct{}

func (noOpHooks) GposTag() tables.Tag                                                  { return 0 }
func (noOpHooks) CollectFeatures(plan harfbuzz.FeaturePlanner, script language.Script) {}
func (noOpHooks) OverrideFeatures(plan harfbuzz.FeaturePlanner)                        {}
func (noOpHooks) PostResolveFeatures(plan harfbuzz.ResolvedFeaturePlanner, view harfbuzz.ResolvedFeatureView, script language.Script) {
}
func (noOpHooks) InitPlan(plan harfbuzz.PlanContext)                            {}
func (noOpHooks) PreprocessText(*harfbuzz.Buffer, *harfbuzz.Font)               {}
func (noOpHooks) PrepareGSUB(*harfbuzz.Buffer, *harfbuzz.Font, language.Script) {}
func (noOpHooks) Decompose(c harfbuzz.NormalizeContext, ab rune) (a, b rune, ok bool) {
	return c.DecomposeUnicode(ab)
}
func (noOpHooks) Compose(c harfbuzz.NormalizeContext, a, b rune) (ab rune, ok bool) {
	return c.ComposeUnicode(a, b)
}
func (noOpHooks) SetupMasks(*harfbuzz.Buffer, *harfbuzz.Font, language.Script) {}
func (noOpHooks) ReorderMarks(*harfbuzz.Buffer, int, int)                      {}
func (noOpHooks) PostprocessGlyphs(*harfbuzz.Buffer, *harfbuzz.Font)           {}

type Shaper struct {
	noOpHooks
	plan arabicPlanState
}

var _ harfbuzz.ShapingEngine = (*Shaper)(nil)
var _ harfbuzz.ShapingEnginePostResolveHook = (*Shaper)(nil)
var _ harfbuzz.ShapingEnginePreGSUBHook = (*Shaper)(nil)

func (Shaper) Name() string { return "arabic" }

func (Shaper) Match(ctx harfbuzz.SelectionContext) int {
	if ctx.Direction != harfbuzz.LeftToRight && ctx.Direction != harfbuzz.RightToLeft {
		return -1
	}

	switch ctx.Script {
	case language.Arabic:
		return 110
	case language.Syriac:
		// Use Arabic shaper for Syriac only when GSUB did not pick DFLT.
		if ctx.ChosenScript[0] != ot.NewTag('D', 'F', 'L', 'T') {
			return 110
		}
	}
	return -1
}

func (Shaper) New() harfbuzz.ShapingEngine { return &Shaper{} }

func (Shaper) MarksBehavior() (harfbuzz.ZeroWidthMarksMode, bool) {
	return harfbuzz.ZeroWidthMarksByGDEFLate, true
}

func (Shaper) NormalizationPreference() harfbuzz.NormalizationMode {
	return harfbuzz.NormalizationDefault
}

func (s *Shaper) CollectFeatures(plan harfbuzz.FeaturePlanner, script language.Script) {
	s.plan.CollectFeatures(plan, script)
}

func (s *Shaper) InitPlan(plan harfbuzz.PlanContext) {
	s.plan.InitPlan(plan)
}

func (s *Shaper) PostResolveFeatures(plan harfbuzz.ResolvedFeaturePlanner, view harfbuzz.ResolvedFeatureView, script language.Script) {
	s.plan.PostResolveFeatures(plan, view, script)
}

func (s *Shaper) PrepareGSUB(buffer *harfbuzz.Buffer, font *harfbuzz.Font, script language.Script) {
	s.plan.PrepareGSUB(buffer, font, script)
}

func (s *Shaper) SetupMasks(buffer *harfbuzz.Buffer, font *harfbuzz.Font, script language.Script) {
	s.plan.SetupMasks(buffer, font, script)
}

func (s *Shaper) ReorderMarks(buffer *harfbuzz.Buffer, start, end int) {
	s.plan.ReorderMarks(buffer, start, end)
}

func (s *Shaper) PostprocessGlyphs(buffer *harfbuzz.Buffer, font *harfbuzz.Font) {
	s.plan.PostprocessGlyphs(buffer, font)
}

// New returns the Arabic shaping engine.
func New() harfbuzz.ShapingEngine { return &Shaper{} }

// Register registers the Arabic shaping engine in the global registry.
func Register() error {
	if err := harfbuzz.RegisterShaper(New()); err != nil {
		if errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
			return nil
		}
		return fmt.Errorf("register otarabic shaper: %w", err)
	}
	return nil
}
