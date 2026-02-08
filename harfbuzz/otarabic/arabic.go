package otarabic

import (
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"

	"github.com/npillmayer/opentype/harfbuzz"
)

func featureIsSyriac(tag ot.Tag) bool {
	return '2' <= byte(tag) && byte(tag) <= '3'
}

var arabicFeatures = [...]ot.Tag{
	ot.NewTag('i', 's', 'o', 'l'),
	ot.NewTag('f', 'i', 'n', 'a'),
	ot.NewTag('f', 'i', 'n', '2'),
	ot.NewTag('f', 'i', 'n', '3'),
	ot.NewTag('m', 'e', 'd', 'i'),
	ot.NewTag('m', 'e', 'd', '2'),
	ot.NewTag('i', 'n', 'i', 't'),
}

// Features ordered the same as the internal Arabic shaping rows, followed by rlig.
var arabicFallbackFeatures = [...]ot.Tag{
	ot.NewTag('i', 'n', 'i', 't'),
	ot.NewTag('m', 'e', 'd', 'i'),
	ot.NewTag('f', 'i', 'n', 'a'),
	ot.NewTag('i', 's', 'o', 'l'),
	ot.NewTag('r', 'l', 'i', 'g'),
	ot.NewTag('r', 'l', 'i', 'g'),
	ot.NewTag('r', 'l', 'i', 'g'),
}

const arabicFallbackMaxLookups = len(arabicFallbackFeatures)

/* Same order as arabicFeatures. */
const (
	arabIsol = iota
	arabFina
	arabFin2
	araFin3
	arabMedi
	arabMed2
	arabInit

	arabNone

	arabStchFixed
	arabStchRepeating
)

const (
	joiningTypeU = iota
	joiningTypeL
	joiningTypeR
	joiningTypeD
	joiningGroupAlaph
	joiningGroupDalathRish
	numStateMachineCols
	joiningTypeT
	joiningTypeC = joiningTypeD
)

var arabicStateTable = [...][numStateMachineCols]struct {
	prevAction uint8
	currAction uint8
	nextState  uint16
}{
	/*   jt_U,          jt_L,          jt_R,          jt_D,          jg_ALAPH,      jg_DALATH_RISH */

	/* State 0: prev was U, not willing to join. */
	{{arabNone, arabNone, 0}, {arabNone, arabIsol, 2}, {arabNone, arabIsol, 1}, {arabNone, arabIsol, 2}, {arabNone, arabIsol, 1}, {arabNone, arabIsol, 6}},

	/* State 1: prev was R or ISOL/ALAPH, not willing to join. */
	{{arabNone, arabNone, 0}, {arabNone, arabIsol, 2}, {arabNone, arabIsol, 1}, {arabNone, arabIsol, 2}, {arabNone, arabFin2, 5}, {arabNone, arabIsol, 6}},

	/* State 2: prev was D/L in ISOL form, willing to join. */
	{{arabNone, arabNone, 0}, {arabNone, arabIsol, 2}, {arabInit, arabFina, 1}, {arabInit, arabFina, 3}, {arabInit, arabFina, 4}, {arabInit, arabFina, 6}},

	/* State 3: prev was D in FINA form, willing to join. */
	{{arabNone, arabNone, 0}, {arabNone, arabIsol, 2}, {arabMedi, arabFina, 1}, {arabMedi, arabFina, 3}, {arabMedi, arabFina, 4}, {arabMedi, arabFina, 6}},

	/* State 4: prev was FINA ALAPH, not willing to join. */
	{{arabNone, arabNone, 0}, {arabNone, arabIsol, 2}, {arabMed2, arabIsol, 1}, {arabMed2, arabIsol, 2}, {arabMed2, arabFin2, 5}, {arabMed2, arabIsol, 6}},

	/* State 5: prev was FIN2/FIN3 ALAPH, not willing to join. */
	{{arabNone, arabNone, 0}, {arabNone, arabIsol, 2}, {arabIsol, arabIsol, 1}, {arabIsol, arabIsol, 2}, {arabIsol, arabFin2, 5}, {arabIsol, arabIsol, 6}},

	/* State 6: prev was DALATH/RISH, not willing to join. */
	{{arabNone, arabNone, 0}, {arabNone, arabIsol, 2}, {arabNone, arabIsol, 1}, {arabNone, arabIsol, 2}, {arabNone, araFin3, 5}, {arabNone, arabIsol, 6}},
}

type arabicShapePlan struct {
	fallbackPlan *harfbuzz.ArabicFallbackPlan
	/* The +1 slot is for arabNone, which is not an OT feature. */
	maskArray         [len(arabicFeatures) + 1]harfbuzz.GlyphMask
	fallbackMaskArray [arabicFallbackMaxLookups]harfbuzz.GlyphMask
	doFallback        bool
	hasStch           bool
}

type arabicPlanState struct {
	plan arabicShapePlan
}

func (cs *arabicPlanState) CollectFeatures(plan harfbuzz.FeaturePlanner, script language.Script) {
	plan.EnableFeature(ot.NewTag('s', 't', 'c', 'h'))
	plan.AddGSUBPause(cs.recordStchPause)

	plan.EnableFeatureExt(ot.NewTag('c', 'c', 'm', 'p'), harfbuzz.FeatureManualZWJ, 1)
	plan.EnableFeatureExt(ot.NewTag('l', 'o', 'c', 'l'), harfbuzz.FeatureManualZWJ, 1)

	plan.AddGSUBPause(nil)

	for _, arabFeat := range arabicFeatures {
		hasFallback := script == language.Arabic && !featureIsSyriac(arabFeat)
		fl := harfbuzz.FeatureNone
		if hasFallback {
			fl = harfbuzz.FeatureHasFallback
		}
		plan.AddFeatureExt(arabFeat, harfbuzz.FeatureManualZWJ|fl, 1)
		plan.AddGSUBPause(nil)
	}

	plan.EnableFeatureExt(ot.NewTag('r', 'l', 'i', 'g'), harfbuzz.FeatureManualZWJ|harfbuzz.FeatureHasFallback, 1)

	if script == language.Arabic {
		plan.AddGSUBPause(cs.arabicFallbackShapePause)
	}
	plan.EnableFeatureExt(ot.NewTag('c', 'a', 'l', 't'), harfbuzz.FeatureManualZWJ, 1)
	if !plan.HasFeature(ot.NewTag('r', 'c', 'l', 't')) {
		plan.AddGSUBPause(nil)
		plan.EnableFeatureExt(ot.NewTag('r', 'c', 'l', 't'), harfbuzz.FeatureManualZWJ, 1)
	}

	plan.EnableFeatureExt(ot.NewTag('l', 'i', 'g', 'a'), harfbuzz.FeatureManualZWJ, 1)
	plan.EnableFeatureExt(ot.NewTag('c', 'l', 'i', 'g'), harfbuzz.FeatureManualZWJ, 1)
	plan.EnableFeatureExt(ot.NewTag('m', 's', 'e', 't'), harfbuzz.FeatureManualZWJ, 1)
}

func newArabicPlan(plan harfbuzz.PlanContext) arabicShapePlan {
	var arabicPlan arabicShapePlan

	arabicPlan.doFallback = plan.Script() == language.Arabic
	arabicPlan.hasStch = plan.FeatureMask1(ot.NewTag('s', 't', 'c', 'h')) != 0
	for i, arabFeat := range arabicFeatures {
		arabicPlan.maskArray[i] = plan.FeatureMask1(arabFeat)
		arabicPlan.doFallback = arabicPlan.doFallback &&
			(featureIsSyriac(arabFeat) || plan.FeatureNeedsFallback(arabFeat))
	}
	for i, fallbackFeat := range arabicFallbackFeatures {
		arabicPlan.fallbackMaskArray[i] = plan.FeatureMask1(fallbackFeat)
	}
	return arabicPlan
}

func (cs *arabicPlanState) InitPlan(plan harfbuzz.PlanContext) {
	cs.plan = newArabicPlan(plan)
}

func getJoiningType(u rune, genCat uint8) uint8 {
	return harfbuzz.ArabicJoiningType(u, genCat)
}

func applyArabicJoining(buffer *harfbuzz.Buffer) {
	info := buffer.Info
	prev, state := -1, uint16(0)

	for _, u := range buffer.PreContext() {
		thisType := getJoiningType(u, harfbuzz.UnicodeGeneralCategory(u))
		if thisType == joiningTypeT {
			continue
		}

		entry := &arabicStateTable[state][thisType]
		state = entry.nextState
		break
	}

	for i := 0; i < len(info); i++ {
		thisType := getJoiningType(info[i].Codepoint(), info[i].GeneralCategory())

		if thisType == joiningTypeT {
			info[i].SetComplexAux(arabNone)
			continue
		}

		entry := &arabicStateTable[state][thisType]

		if entry.prevAction != arabNone && prev != -1 {
			info[prev].SetComplexAux(entry.prevAction)
			buffer.SafeToInsertTatweel(prev, i+1)
		} else {
			if prev == -1 {
				if thisType >= joiningTypeR {
					buffer.UnsafeToConcatFromOutbuffer(0, i+1)
				}
			} else {
				if thisType >= joiningTypeR ||
					(2 <= state && state <= 5) {
					buffer.UnsafeToConcat(prev, i+1)
				}
			}
		}

		info[i].SetComplexAux(entry.currAction)
		prev = i
		state = entry.nextState
	}

	for _, u := range buffer.PostContext() {
		thisType := getJoiningType(u, harfbuzz.UnicodeGeneralCategory(u))
		if thisType == joiningTypeT {
			continue
		}

		entry := &arabicStateTable[state][thisType]
		if entry.prevAction != arabNone && prev != -1 {
			info[prev].SetComplexAux(entry.prevAction)
			buffer.SafeToInsertTatweel(prev, len(buffer.Info))
		} else if 2 <= state && state <= 5 {
			buffer.UnsafeToConcat(prev, len(buffer.Info))
		}
		break
	}
}

func mongolianVariationSelectors(buffer *harfbuzz.Buffer) {
	info := buffer.Info
	for i := 1; i < len(info); i++ {
		if cp := info[i].Codepoint(); 0x180B <= cp && cp <= 0x180D || cp == 0x180F {
			info[i].SetComplexAux(info[i-1].ComplexAux())
		}
	}
}

func (arabicPlan arabicShapePlan) SetupMasks(buffer *harfbuzz.Buffer, script language.Script) {
	applyArabicJoining(buffer)
	if script == language.Mongolian {
		mongolianVariationSelectors(buffer)
	}

	info := buffer.Info
	for i := range info {
		info[i].Mask |= arabicPlan.maskArray[info[i].ComplexAux()]
	}
}

func (cs *arabicPlanState) SetupMasks(buffer *harfbuzz.Buffer, _ *harfbuzz.Font, script language.Script) {
	cs.plan.SetupMasks(buffer, script)
}

func (cs *arabicPlanState) arabicFallbackShapePause(ctx harfbuzz.PauseContext) bool {
	if !cs.plan.doFallback {
		return false
	}

	font := ctx.Font()
	buffer := ctx.Buffer()

	fallbackPlan := cs.plan.fallbackPlan
	if fallbackPlan == nil {
		fallbackPlan = harfbuzz.NewArabicFallbackPlan(cs.plan.fallbackMaskArray[:], font)
		cs.plan.fallbackPlan = fallbackPlan
	}

	fallbackPlan.Shape(font, buffer)
	return true
}

func (cs *arabicPlanState) recordStchPause(ctx harfbuzz.PauseContext) bool {
	if !cs.plan.hasStch {
		return false
	}

	buffer := ctx.Buffer()
	info := buffer.Info
	for i := range info {
		if info[i].Multiplied() {
			comp := info[i].LigComp()
			if comp%2 != 0 {
				info[i].SetComplexAux(arabStchRepeating)
			} else {
				info[i].SetComplexAux(arabStchFixed)
			}
		}
	}

	return false
}

func inRange(sa uint8) bool {
	return arabStchFixed <= sa && sa <= arabStchRepeating
}

func (cs *arabicPlanState) PostprocessGlyphs(buffer *harfbuzz.Buffer, font *harfbuzz.Font) {
	hasStch := false
	for i := range buffer.Info {
		if inRange(buffer.Info[i].ComplexAux()) {
			hasStch = true
			break
		}
	}
	if !hasStch {
		return
	}

	sign := harfbuzz.Position(+1)
	if font.XScale < 0 {
		sign = -1
	}
	const (
		measure = iota
		cut
	)
	var (
		originCount       = len(buffer.Info)
		extraGlyphsNeeded = 0
	)
	for step := measure; step <= cut; step++ {
		info := buffer.Info
		pos := buffer.Pos
		j := len(info)
		for i := originCount; i != 0; i-- {
			if sa := info[i-1].ComplexAux(); !inRange(sa) {
				if step == cut {
					j--
					info[j] = info[i-1]
					pos[j] = pos[i-1]
				}
				continue
			}

			var (
				wTotal     harfbuzz.Position
				wFixed     harfbuzz.Position
				wRepeating harfbuzz.Position
				nFixed     = 0
				nRepeating = 0
			)
			end := i
			for i != 0 && inRange(info[i-1].ComplexAux()) {
				i--
				width := font.GlyphHAdvance(info[i].Glyph)
				if info[i].ComplexAux() == arabStchFixed {
					wFixed += width
					nFixed++
				} else {
					wRepeating += width
					nRepeating++
				}
			}
			start := i
			context := i
			for context != 0 && !inRange(info[context-1].ComplexAux()) &&
				(info[context-1].IsDefaultIgnorable() ||
					harfbuzz.ArabicIsWord(info[context-1].GeneralCategory())) {
				context--
				wTotal += pos[context].XAdvance
			}
			i++ // keep outer-loop behavior

			var nCopies int
			wRemaining := wTotal - wFixed
			if sign*wRemaining > sign*wRepeating && sign*wRepeating > 0 {
				nCopies = int((sign*wRemaining)/(sign*wRepeating) - 1)
			}

			var extraRepeatOverlap harfbuzz.Position
			shortfall := sign*wRemaining - sign*wRepeating*(harfbuzz.Position(nCopies)+1)
			if shortfall > 0 && nRepeating > 0 {
				nCopies++
				excess := (harfbuzz.Position(nCopies)+1)*sign*wRepeating - sign*wRemaining
				if excess > 0 {
					extraRepeatOverlap = excess / harfbuzz.Position(nCopies*nRepeating)
				}
			}

			if step == measure {
				extraGlyphsNeeded += nCopies * nRepeating
			} else {
				buffer.UnsafeToBreak(context, end)
				var xOffset harfbuzz.Position
				for k := end; k > start; k-- {
					width := font.GlyphHAdvance(info[k-1].Glyph)

					repeat := 1
					if info[k-1].ComplexAux() == arabStchRepeating {
						repeat += nCopies
					}

					for n := 0; n < repeat; n++ {
						xOffset -= width
						if n > 0 {
							xOffset += extraRepeatOverlap
						}
						pos[k-1].XOffset = xOffset
						j--
						info[j] = info[k-1]
						pos[j] = pos[k-1]
					}
				}
			}
		}

		if step == measure {
			buffer.Info = append(buffer.Info, make([]harfbuzz.GlyphInfo, extraGlyphsNeeded)...)
			buffer.Pos = append(buffer.Pos, make([]harfbuzz.GlyphPosition, extraGlyphsNeeded)...)
		}
	}
}

var modifierCombiningMarks = [...]rune{
	0x0654, /* ARABIC HAMZA ABOVE */
	0x0655, /* ARABIC HAMZA BELOW */
	0x0658, /* ARABIC MARK NOON GHUNNA */
	0x06DC, /* ARABIC SMALL HIGH SEEN */
	0x06E3, /* ARABIC SMALL LOW SEEN */
	0x06E7, /* ARABIC SMALL HIGH YEH */
	0x06E8, /* ARABIC SMALL HIGH NOON */
	0x08CA, /* ARABIC SMALL HIGH FARSI YEH */
	0x08CB, /* ARABIC SMALL HIGH YEH BARREE WITH TWO DOTS BELOW */
	0x08CD, /* ARABIC SMALL HIGH ZAH */
	0x08CE, /* ARABIC LARGE ROUND DOT ABOVE */
	0x08CF, /* ARABIC LARGE ROUND DOT BELOW */
	0x08D3, /* ARABIC SMALL LOW WAW */
	0x08F3, /* ARABIC SMALL HIGH WAW */
}

func infoIsMcm(info *harfbuzz.GlyphInfo) bool {
	u := info.Codepoint()
	for i := 0; i < len(modifierCombiningMarks); i++ {
		if u == modifierCombiningMarks[i] {
			return true
		}
	}
	return false
}

func (cs *arabicPlanState) ReorderMarks(buffer *harfbuzz.Buffer, start, end int) {
	info := buffer.Info

	i := start
	for cc := uint8(220); cc <= 230; cc += 10 {
		for i < end && info[i].ModifiedCombiningClass() < cc {
			i++
		}

		if i == end {
			break
		}

		if info[i].ModifiedCombiningClass() > cc {
			continue
		}

		j := i
		for j < end && info[j].ModifiedCombiningClass() == cc && infoIsMcm(&info[j]) {
			j++
		}

		if i == j {
			continue
		}

		temp := make([]harfbuzz.GlyphInfo, j-i)
		buffer.MergeClusters(start, j)
		copy(temp, info[i:j])
		copy(info[start+j-i:], info[start:i])
		copy(info[start:], temp)

		newStart := start + j - i
		newCc := uint8(26)
		if cc == 220 {
			newCc = 26
		}
		for start < newStart {
			info[start].SetModifiedCombiningClass(newCc)
			start++
		}

		i = j
	}
}
