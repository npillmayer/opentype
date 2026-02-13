package otarabic

import (
	"unicode"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

var (
	arabicScript = language.MustParseScript("Arab")
	syriacScript = language.MustParseScript("Syrc")
)

var (
	tagStch = ot.T("stch")
	tagCCMP = ot.T("ccmp")
	tagLocl = ot.T("locl")
	tagIsol = ot.T("isol")
	tagFina = ot.T("fina")
	tagFin2 = ot.T("fin2")
	tagFin3 = ot.T("fin3")
	tagMedi = ot.T("medi")
	tagMed2 = ot.T("med2")
	tagInit = ot.T("init")
	tagRlig = ot.T("rlig")
	tagCalt = ot.T("calt")
	tagRclt = ot.T("rclt")
	tagLiga = ot.T("liga")
	tagClig = ot.T("clig")
	tagMset = ot.T("mset")
)

var arabicFormFeatureTags = [...]ot.Tag{
	tagIsol, tagFina, tagFin2, tagFin3, tagMedi, tagMed2, tagInit,
}

const (
	formNone = -1
	formIsol = iota
	formFina
	formFin2
	formFin3
	formMedi
	formMed2
	formInit
	formCount
)

type joiningType uint8

const (
	joiningTypeU joiningType = iota
	joiningTypeR
	joiningTypeD
	joiningTypeT
	joiningTypeC
)

type shaperPlanState struct {
	font       *ot.Font
	script     language.Script
	maskArray  [formCount]uint32
	formMask   uint32
	hasStch    bool
	hasRligFbk bool
}

// Shaper is the Arabic/Syriac shaping engine.
//
// This step ports plan-time Arabic feature staging and runtime form-mask
// assignment. Joining details are intentionally conservative and may be
// extended in follow-up steps.
type Shaper struct {
	plan shaperPlanState
}

var _ otshape.ShapingEngine = (*Shaper)(nil)
var _ otshape.ShapingEnginePolicy = (*Shaper)(nil)
var _ otshape.ShapingEnginePlanHooks = (*Shaper)(nil)
var _ otshape.ShapingEnginePostResolveHook = (*Shaper)(nil)
var _ otshape.ShapingEnginePreGSUBHook = (*Shaper)(nil)
var _ otshape.ShapingEngineMaskHook = (*Shaper)(nil)
var _ otshape.ShapingEnginePostprocessHook = (*Shaper)(nil)

// New returns the Arabic shaping engine.
func New() otshape.ShapingEngine {
	return &Shaper{}
}

func (Shaper) Name() string {
	return "arabic"
}

func (Shaper) Match(ctx otshape.SelectionContext) otshape.ShaperConfidence {
	if ctx.Direction != bidi.LeftToRight && ctx.Direction != bidi.RightToLeft {
		return otshape.ShaperConfidenceNone
	}
	if ctx.Script == arabicScript || ctx.ScriptTag == ot.T("arab") {
		return otshape.ShaperConfidenceCertain
	}
	if ctx.Script == syriacScript || ctx.ScriptTag == ot.T("syrc") {
		return otshape.ShaperConfidenceHigh
	}
	return otshape.ShaperConfidenceNone
}

func (Shaper) New() otshape.ShapingEngine {
	return &Shaper{}
}

func (Shaper) NormalizationPreference() otshape.NormalizationMode {
	return otshape.NormalizationAuto
}

func (Shaper) ApplyGPOS() bool {
	return true
}

func featureIsSyriac(tag ot.Tag) bool {
	return byte(tag) >= '2' && byte(tag) <= '3'
}

func noPauseHook(otshape.PauseContext) error {
	return nil
}

func (s *Shaper) CollectFeatures(plan otshape.FeaturePlanner, ctx otshape.SelectionContext) {
	plan.AddFeature(tagStch, otshape.FeatureNone, 1)
	plan.AddGSUBPause(noPauseHook)

	plan.AddFeature(tagCCMP, otshape.FeatureManualZWJ, 1)
	plan.AddFeature(tagLocl, otshape.FeatureManualZWJ, 1)
	plan.AddGSUBPause(noPauseHook)

	for _, tag := range arabicFormFeatureTags {
		flags := otshape.FeatureManualZWJ
		if ctx.Script == arabicScript && !featureIsSyriac(tag) {
			flags |= otshape.FeatureHasFallback
		}
		plan.AddFeature(tag, flags, 1)
		plan.AddGSUBPause(noPauseHook)
	}

	plan.AddFeature(tagRlig, otshape.FeatureManualZWJ|otshape.FeatureHasFallback, 1)
	if ctx.Script == arabicScript {
		plan.AddGSUBPause(noPauseHook)
	}

	plan.AddFeature(tagCalt, otshape.FeatureManualZWJ, 1)
	if !plan.HasFeature(tagRclt) {
		plan.AddGSUBPause(noPauseHook)
		plan.AddFeature(tagRclt, otshape.FeatureManualZWJ, 1)
	}
	plan.AddFeature(tagLiga, otshape.FeatureManualZWJ, 1)
	plan.AddFeature(tagClig, otshape.FeatureManualZWJ, 1)
	plan.AddFeature(tagMset, otshape.FeatureManualZWJ, 1)
}

func (Shaper) OverrideFeatures(plan otshape.FeaturePlanner) {
	_ = plan
}

func (s *Shaper) InitPlan(plan otshape.PlanContext) {
	s.plan = shaperPlanState{
		font:       plan.Font(),
		script:     plan.Selection().Script,
		hasStch:    plan.FeatureMask1(tagStch) != 0,
		hasRligFbk: plan.FeatureNeedsFallback(tagRlig),
	}
	for i, tag := range arabicFormFeatureTags {
		m := plan.FeatureMask1(tag)
		s.plan.maskArray[i] = m
		s.plan.formMask |= m
	}
}

func (s *Shaper) PostResolveFeatures(plan otshape.ResolvedFeaturePlanner, _ otshape.ResolvedFeatureView, ctx otshape.SelectionContext) {
	_ = plan.AddGSUBPauseAfter(tagStch, noPauseHook)
	if ctx.Script == arabicScript {
		_ = plan.AddGSUBPauseAfter(tagRlig, noPauseHook)
	}
}

func (s *Shaper) PrepareGSUB(run otshape.RunContext) {
	_ = run
}

func (s *Shaper) SetupMasks(run otshape.RunContext) {
	if s.plan.font == nil || s.plan.formMask == 0 {
		return
	}
	n := run.Len()
	if n == 0 {
		return
	}
	cps := make([]rune, n)
	for i := 0; i < n; i++ {
		cps[i] = otquery.CodePointForGlyph(s.plan.font, run.Glyph(i))
	}
	forms := resolveJoiningForms(cps)
	for i := 0; i < n; i++ {
		m := run.Mask(i) &^ s.plan.formMask
		mask := s.maskForForm(forms[i])
		if mask != 0 {
			m |= mask
		}
		run.SetMask(i, m)
	}
}

func (s *Shaper) maskForForm(form int) uint32 {
	if form < 0 || form >= len(s.plan.maskArray) {
		return 0
	}
	return s.plan.maskArray[form]
}

func (s *Shaper) PostprocessRun(run otshape.RunContext) {
	_ = run
	_ = s.plan.hasStch
	_ = s.plan.hasRligFbk
}

func resolveJoiningForms(cps []rune) []int {
	n := len(cps)
	forms := make([]int, n)
	if n == 0 {
		return forms
	}
	for i := range forms {
		forms[i] = formNone
	}
	types := make([]joiningType, n)
	for i, cp := range cps {
		types[i] = classifyJoiningType(cp)
	}
	for i := 0; i < n; i++ {
		t := types[i]
		if t != joiningTypeD && t != joiningTypeR {
			continue
		}

		prev := previousJoinType(types, i)
		next := nextJoinType(types, i)

		joinPrev := prev >= 0 && canJoinFollowing(types[prev]) && canJoinPreceding(t)
		joinNext := next >= 0 && canJoinFollowing(t) && canJoinPreceding(types[next])

		switch {
		case joinPrev && joinNext:
			forms[i] = formMedi
		case joinPrev:
			forms[i] = formFina
		case joinNext:
			forms[i] = formInit
		default:
			forms[i] = formIsol
		}
	}
	return forms
}

func previousJoinType(types []joiningType, i int) int {
	for j := i - 1; j >= 0; j-- {
		if types[j] != joiningTypeT {
			return j
		}
	}
	return -1
}

func nextJoinType(types []joiningType, i int) int {
	for j := i + 1; j < len(types); j++ {
		if types[j] != joiningTypeT {
			return j
		}
	}
	return -1
}

func canJoinPreceding(t joiningType) bool {
	return t == joiningTypeD || t == joiningTypeR || t == joiningTypeC
}

func canJoinFollowing(t joiningType) bool {
	return t == joiningTypeD || t == joiningTypeC
}

func classifyJoiningType(cp rune) joiningType {
	if cp == 0 {
		return joiningTypeU
	}
	if cp == '\u200C' { // ZWNJ explicitly breaks joining.
		return joiningTypeU
	}
	if cp == '\u200D' || cp == '\u0640' { // ZWJ, Tatweel
		return joiningTypeC
	}
	if unicode.Is(unicode.M, cp) {
		return joiningTypeT
	}
	if isRightJoining(cp) {
		return joiningTypeR
	}
	if isArabicJoiningLetter(cp) {
		return joiningTypeD
	}
	return joiningTypeU
}

func isArabicJoiningLetter(cp rune) bool {
	if unicode.IsLetter(cp) && (unicode.In(cp, unicode.Arabic) || unicode.In(cp, unicode.Syriac)) {
		return true
	}
	return false
}

var rightJoiningRunes = map[rune]struct{}{
	'\u0622': {}, '\u0623': {}, '\u0624': {}, '\u0625': {}, '\u0627': {}, '\u0629': {},
	'\u062F': {}, '\u0630': {}, '\u0631': {}, '\u0632': {}, '\u0648': {},
	'\u0671': {}, '\u0672': {}, '\u0673': {}, '\u0675': {}, '\u0676': {}, '\u0677': {},
	'\u0688': {}, '\u0689': {}, '\u0691': {}, '\u06C0': {}, '\u06C3': {}, '\u06C4': {}, '\u06C5': {}, '\u06C6': {}, '\u06C7': {}, '\u06C8': {}, '\u06C9': {}, '\u06CA': {}, '\u06CB': {}, '\u06CD': {},
	'\u0710': {}, '\u0715': {}, '\u0716': {}, '\u0718': {}, '\u0719': {}, '\u071A': {}, '\u071D': {}, '\u072A': {}, '\u072B': {}, '\u072C': {}, '\u072D': {}, '\u072E': {}, '\u072F': {},
}

func isRightJoining(cp rune) bool {
	_, ok := rightJoiningRunes[cp]
	return ok
}
