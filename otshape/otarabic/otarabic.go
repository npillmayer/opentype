package otarabic

import (
	"strings"
	"sync"
	"unicode"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/unicode/runenames"
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
	formNone  = -1
	formIsol  = 0
	formFina  = 1
	formFin2  = 2
	formFin3  = 3
	formMedi  = 4
	formMed2  = 5
	formInit  = 6
	formCount = 7
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
	font          *ot.Font
	script        language.Script
	maskArray     [formCount]uint32
	formMask      uint32
	hasStch       bool
	stchMask      uint32
	hasRligFbk    bool
	fallbackGlyph map[rune]glyphForms
}

// Shaper is the Arabic/Syriac shaping engine.
//
// This step ports plan-time Arabic feature staging and runtime form-mask
// assignment. Joining details are intentionally conservative and may be
// extended in follow-up steps.
type Shaper struct {
	plan         shaperPlanState
	preparedForm []int
}

var _ otshape.ShapingEngine = (*Shaper)(nil)
var _ otshape.ShapingEnginePolicy = (*Shaper)(nil)
var _ otshape.ShapingEnginePlanHooks = (*Shaper)(nil)
var _ otshape.ShapingEnginePostResolveHook = (*Shaper)(nil)
var _ otshape.ShapingEnginePreGSUBHook = (*Shaper)(nil)
var _ otshape.ShapingEngineReorderHook = (*Shaper)(nil)
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
		stchMask:   plan.FeatureMask1(tagStch),
		hasRligFbk: plan.FeatureNeedsFallback(tagRlig),
	}
	for i, tag := range arabicFormFeatureTags {
		m := plan.FeatureMask1(tag)
		s.plan.maskArray[i] = m
		s.plan.formMask |= m
	}
	if s.plan.hasRligFbk {
		s.plan.fallbackGlyph = buildFallbackGlyphMap(s.plan.font)
	}
}

func (s *Shaper) PostResolveFeatures(plan otshape.ResolvedFeaturePlanner, _ otshape.ResolvedFeatureView, ctx otshape.SelectionContext) {
	_ = plan.AddGSUBPauseAfter(tagStch, noPauseHook)
	if ctx.Script == arabicScript {
		_ = plan.AddGSUBPauseAfter(tagRlig, noPauseHook)
	}
}

func (s *Shaper) PrepareGSUB(run otshape.RunContext) {
	n := run.Len()
	if n == 0 {
		s.preparedForm = s.preparedForm[:0]
		return
	}
	cps := codepointsFromRun(run, s.plan.font)
	forms := resolveJoiningForms(cps)
	if cap(s.preparedForm) < len(forms) {
		s.preparedForm = make([]int, len(forms))
	}
	s.preparedForm = s.preparedForm[:len(forms)]
	copy(s.preparedForm, forms)
}

var modifierCombiningMarks = map[rune]struct{}{
	0x0654: {}, // ARABIC HAMZA ABOVE
	0x0655: {}, // ARABIC HAMZA BELOW
	0x0658: {}, // ARABIC MARK NOON GHUNNA
	0x06DC: {}, // ARABIC SMALL HIGH SEEN
	0x06E3: {}, // ARABIC SMALL LOW SEEN
	0x06E7: {}, // ARABIC SMALL HIGH YEH
	0x06E8: {}, // ARABIC SMALL HIGH NOON
	0x08CA: {}, // ARABIC SMALL HIGH FARSI YEH
	0x08CB: {}, // ARABIC SMALL HIGH YEH BARREE WITH TWO DOTS BELOW
	0x08CD: {}, // ARABIC SMALL HIGH ZAH
	0x08CE: {}, // ARABIC LARGE ROUND DOT ABOVE
	0x08CF: {}, // ARABIC LARGE ROUND DOT BELOW
	0x08D3: {}, // ARABIC SMALL LOW WAW
	0x08F3: {}, // ARABIC SMALL HIGH WAW
}

func (s *Shaper) ReorderMarks(run otshape.RunContext, start, end int) {
	_ = s
	if run == nil {
		return
	}
	n := run.Len()
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if end-start < 2 {
		return
	}
	i := start
	for _, cc := range []uint8{220, 230} {
		for i < end && arabicModifiedCombiningClass(run.Codepoint(i)) < cc {
			i++
		}
		if i == end {
			break
		}
		if arabicModifiedCombiningClass(run.Codepoint(i)) > cc {
			continue
		}
		j := i
		for j < end &&
			arabicModifiedCombiningClass(run.Codepoint(j)) == cc &&
			isModifierCombiningMark(run.Codepoint(j)) {
			j++
		}
		if i == j {
			continue
		}
		run.MergeClusters(start, j)
		moveBlockToFront(run, start, i, j)
		moved := j - i
		start += moved
		i = j
	}
}

func (s *Shaper) SetupMasks(run otshape.RunContext) {
	if s.plan.formMask == 0 {
		return
	}
	n := run.Len()
	if n == 0 {
		return
	}
	forms := s.preparedForm
	if len(forms) != n {
		cps := codepointsFromRun(run, s.plan.font)
		forms = resolveJoiningForms(cps)
	}
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
	defer func() {
		if s.preparedForm != nil {
			s.preparedForm = s.preparedForm[:0]
		}
	}()
	if run == nil {
		return
	}
	if s.plan.hasStch {
		tatweel := otshape.NOTDEF
		if s.plan.font != nil {
			tatweel = otquery.GlyphIndex(s.plan.font, '\u0640')
		}
		if tatweel != otshape.NOTDEF {
			_ = expandTatweelForStch(run, tatweel, s.plan.stchMask)
		}
	}
	if !s.plan.hasRligFbk || len(s.plan.fallbackGlyph) == 0 {
		return
	}
	n := run.Len()
	if n == 0 {
		return
	}
	forms := s.preparedForm
	if len(forms) != n {
		cps := codepointsFromRun(run, s.plan.font)
		forms = resolveJoiningForms(cps)
	}
	for i := 0; i < n; i++ {
		if run.Glyph(i) != otshape.NOTDEF {
			continue
		}
		cp := run.Codepoint(i)
		if cp == 0 {
			continue
		}
		form := formIsol
		if i < len(forms) {
			form = forms[i]
		}
		if gid, ok := fallbackGlyphFor(s.plan.fallbackGlyph, cp, form); ok {
			run.SetGlyph(i, gid)
		}
	}
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

func isModifierCombiningMark(cp rune) bool {
	_, ok := modifierCombiningMarks[cp]
	return ok
}

func arabicModifiedCombiningClass(cp rune) uint8 {
	if cp == 0 {
		return 0
	}
	return norm.NFD.PropertiesString(string(cp)).CCC()
}

func moveBlockToFront(run otshape.RunContext, start, i, j int) {
	moved := 0
	for k := i; k < j; k++ {
		target := start + moved
		for p := k; p > target; p-- {
			run.Swap(p-1, p)
		}
		moved++
	}
}

type glyphForms [formCount]ot.GlyphIndex
type presentationForms [formCount]rune

var (
	presentationFormsOnce sync.Once
	presentationByBase    map[rune]presentationForms
)

func buildFallbackGlyphMap(font *ot.Font) map[rune]glyphForms {
	if font == nil {
		return nil
	}
	presentationFormsOnce.Do(func() {
		presentationByBase = buildPresentationFormMap()
	})
	if len(presentationByBase) == 0 {
		return nil
	}
	out := make(map[rune]glyphForms, len(presentationByBase))
	for base, forms := range presentationByBase {
		var gfs glyphForms
		hasAny := false
		for formInx, pres := range forms {
			if pres == 0 {
				continue
			}
			gid := otquery.GlyphIndex(font, pres)
			if gid == otshape.NOTDEF {
				continue
			}
			gfs[formInx] = gid
			hasAny = true
		}
		if hasAny {
			out[base] = gfs
		}
	}
	return out
}

func buildPresentationFormMap() map[rune]presentationForms {
	out := make(map[rune]presentationForms, 256)
	addRange := func(from, to rune) {
		for u := from; u <= to; u++ {
			form, ok := presentationFormFromName(u)
			if !ok {
				continue
			}
			base := presentationBaseRune(u)
			if base == 0 {
				continue
			}
			forms := out[base]
			forms[form] = u
			out[base] = forms
		}
	}
	addRange(0xFB50, 0xFDFF) // Arabic Presentation Forms-A
	addRange(0xFE70, 0xFEFF) // Arabic Presentation Forms-B
	return out
}

func presentationFormFromName(u rune) (int, bool) {
	name := runenames.Name(u)
	if name == "" || !strings.Contains(name, "ARABIC") {
		return 0, false
	}
	switch {
	case strings.Contains(name, "ISOLATED FORM"):
		return formIsol, true
	case strings.Contains(name, "FINAL FORM"):
		return formFina, true
	case strings.Contains(name, "INITIAL FORM"):
		return formInit, true
	case strings.Contains(name, "MEDIAL FORM"):
		return formMedi, true
	default:
		return 0, false
	}
}

func presentationBaseRune(u rune) rune {
	// NFKD compatibility decomposition for Arabic presentation forms starts with
	// the base Arabic/Syriac letter.
	for _, x := range []rune(norm.NFKD.String(string(u))) {
		if unicode.Is(unicode.M, x) {
			continue
		}
		if unicode.In(x, unicode.Arabic) || unicode.In(x, unicode.Syriac) {
			return x
		}
	}
	return 0
}

func fallbackGlyphFor(table map[rune]glyphForms, cp rune, form int) (ot.GlyphIndex, bool) {
	forms, ok := table[cp]
	if !ok {
		return otshape.NOTDEF, false
	}
	switch form {
	case formFin2, formFin3:
		form = formFina
	case formMed2:
		form = formMedi
	case formNone:
		form = formIsol
	}
	if form >= 0 && form < formCount {
		if gid := forms[form]; gid != otshape.NOTDEF {
			return gid, true
		}
	}
	if gid := forms[formIsol]; gid != otshape.NOTDEF {
		return gid, true
	}
	return otshape.NOTDEF, false
}

func expandTatweelForStch(run otshape.RunContext, tatweel ot.GlyphIndex, stchMask uint32) int {
	if run == nil || tatweel == otshape.NOTDEF {
		return 0
	}
	inserted := 0
	for i := 0; i < run.Len(); i++ {
		if run.Glyph(i) != tatweel {
			continue
		}
		if stchMask != 0 && run.Mask(i)&stchMask == 0 {
			continue
		}
		run.InsertGlyphCopies(i+1, i, 1)
		inserted++
		i++ // skip the freshly inserted copy to avoid geometric growth
	}
	return inserted
}

func codepointsFromRun(run otshape.RunContext, font *ot.Font) []rune {
	n := run.Len()
	cps := make([]rune, n)
	for i := 0; i < n; i++ {
		cp := run.Codepoint(i)
		if cp == 0 && font != nil {
			cp = otquery.CodePointForGlyph(font, run.Glyph(i))
		}
		cps[i] = cp
	}
	return cps
}
