package otcomplex

import (
	"errors"
	"fmt"

	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"
)

type noOpHooks struct{}

func (noOpHooks) GposTag() tables.Tag                                                  { return 0 }
func (noOpHooks) CollectFeatures(plan harfbuzz.FeaturePlanner, script language.Script) {}
func (noOpHooks) OverrideFeatures(plan harfbuzz.FeaturePlanner)                        {}
func (noOpHooks) InitPlan(plan harfbuzz.PlanContext)                                   {}
func (noOpHooks) PreprocessText(*harfbuzz.Buffer, *harfbuzz.Font)                      {}
func (noOpHooks) Decompose(c harfbuzz.NormalizeContext, ab rune) (a, b rune, ok bool) {
	return c.DecomposeUnicode(ab)
}
func (noOpHooks) Compose(c harfbuzz.NormalizeContext, a, b rune) (ab rune, ok bool) {
	return c.ComposeUnicode(a, b)
}
func (noOpHooks) SetupMasks(*harfbuzz.Buffer, *harfbuzz.Font, language.Script) {}
func (noOpHooks) ReorderMarks(*harfbuzz.Buffer, int, int)                      {}
func (noOpHooks) PostprocessGlyphs(*harfbuzz.Buffer, *harfbuzz.Font)           {}

type HebrewShaper struct {
	noOpHooks
}

func (HebrewShaper) Name() string { return "hebrew" }

func (HebrewShaper) Match(ctx harfbuzz.SelectionContext) int {
	if ctx.Script == language.Hebrew {
		return 100
	}
	return -1
}

func (HebrewShaper) New() harfbuzz.ShapingEngine { return HebrewShaper{} }

func (HebrewShaper) MarksBehavior() (harfbuzz.ZeroWidthMarksMode, bool) {
	return harfbuzz.ZeroWidthMarksByGDEFLate, true
}

func (HebrewShaper) NormalizationPreference() harfbuzz.NormalizationMode {
	return harfbuzz.NormalizationDefault
}

func (HebrewShaper) GposTag() tables.Tag {
	return hebrewGposTag()
}

func (HebrewShaper) Compose(c harfbuzz.NormalizeContext, a, b rune) (rune, bool) {
	return hebrewCompose(c, a, b)
}

func (HebrewShaper) ReorderMarks(buffer *harfbuzz.Buffer, start, end int) {
	hebrewReorderMarks(buffer, start, end)
}

type ArabicShaper struct {
	noOpHooks
	plan arabicPlanState
}

func (ArabicShaper) Name() string { return "arabic" }

func (ArabicShaper) Match(ctx harfbuzz.SelectionContext) int {
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

func (ArabicShaper) New() harfbuzz.ShapingEngine { return &ArabicShaper{} }

func (ArabicShaper) MarksBehavior() (harfbuzz.ZeroWidthMarksMode, bool) {
	return harfbuzz.ZeroWidthMarksByGDEFLate, true
}

func (ArabicShaper) NormalizationPreference() harfbuzz.NormalizationMode {
	return harfbuzz.NormalizationDefault
}

func (s *ArabicShaper) CollectFeatures(plan harfbuzz.FeaturePlanner, script language.Script) {
	s.plan.CollectFeatures(plan, script)
}

func (s *ArabicShaper) InitPlan(plan harfbuzz.PlanContext) {
	s.plan.InitPlan(plan)
}

func (s *ArabicShaper) SetupMasks(buffer *harfbuzz.Buffer, font *harfbuzz.Font, script language.Script) {
	s.plan.SetupMasks(buffer, font, script)
}

func (s *ArabicShaper) ReorderMarks(buffer *harfbuzz.Buffer, start, end int) {
	s.plan.ReorderMarks(buffer, start, end)
}

func (s *ArabicShaper) PostprocessGlyphs(buffer *harfbuzz.Buffer, font *harfbuzz.Font) {
	s.plan.PostprocessGlyphs(buffer, font)
}

// NewHebrew returns the Hebrew complex shaping engine.
func NewHebrew() harfbuzz.ShapingEngine { return HebrewShaper{} }

// NewArabic returns the Arabic complex shaping engine.
func NewArabic() harfbuzz.ShapingEngine { return &ArabicShaper{} }

// Register registers all complex shapers in the global registry.
func Register() error {
	if err := harfbuzz.RegisterShaper(NewHebrew()); err != nil {
		if !errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
			return fmt.Errorf("register otcomplex hebrew shaper: %w", err)
		}
	}
	if err := harfbuzz.RegisterShaper(NewArabic()); err != nil {
		if !errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
			return fmt.Errorf("register otcomplex arabic shaper: %w", err)
		}
	}
	return nil
}
