package othebrew

import (
	"github.com/npillmayer/opentype/internal/hebreworder"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"golang.org/x/text/language"
)

var hebrewScript = language.MustParseScript("Hebr")

// Hebrew presentation forms with dagesh, for characters U+05D0..U+05EA.
// Some letters intentionally map to zero because no encoded form exists.
var dageshForms = [0x05EA - 0x05D0 + 1]rune{
	0xFB30, // ALEF
	0xFB31, // BET
	0xFB32, // GIMEL
	0xFB33, // DALET
	0xFB34, // HE
	0xFB35, // VAV
	0xFB36, // ZAYIN
	0x0000, // HET
	0xFB38, // TET
	0xFB39, // YOD
	0xFB3A, // FINAL KAF
	0xFB3B, // KAF
	0xFB3C, // LAMED
	0x0000, // FINAL MEM
	0xFB3E, // MEM
	0x0000, // FINAL NUN
	0xFB40, // NUN
	0xFB41, // SAMEKH
	0x0000, // AYIN
	0xFB43, // FINAL PE
	0xFB44, // PE
	0x0000, // FINAL TSADI
	0xFB46, // TSADI
	0xFB47, // QOF
	0xFB48, // RESH
	0xFB49, // SHIN
	0xFB4A, // TAV
}

const (
	mccSheva  = 22
	mccHiriq  = 23
	mccPatah  = 20
	mccQamats = 21
	mccMeteg  = 25

	combiningClassBelow = 220
)

// Shaper is the Hebrew shaping engine.
//
// It adds Hebrew-specific composition and mark reordering behavior on top of
// the shared otshape pipeline.
type Shaper struct{}

var _ otshape.ShapingEngine = Shaper{}
var _ otshape.ShapingEnginePolicy = Shaper{}
var _ otshape.ShapingEngineComposeHook = Shaper{}
var _ otshape.ShapingEngineReorderHook = Shaper{}
var _ otshape.ShapingEngineNormalizationReorderHook = Shaper{}

// New returns a new Hebrew shaping engine instance.
func New() otshape.ShapingEngine {
	return Shaper{}
}

// Name returns the stable engine name.
func (Shaper) Name() string {
	return "hebrew"
}

// Match reports how suitable this engine is for ctx.
//
// It returns certain confidence for Hebrew script and no confidence otherwise.
func (Shaper) Match(ctx otshape.SelectionContext) otshape.ShaperConfidence {
	if ctx.Script == hebrewScript || ctx.ScriptTag == ot.T("hebr") {
		return otshape.ShaperConfidenceCertain
	}
	return otshape.ShaperConfidenceNone
}

// New returns a new independent Hebrew engine instance.
func (Shaper) New() otshape.ShapingEngine {
	return Shaper{}
}

// NormalizationPreference reports the engine's normalization policy.
func (Shaper) NormalizationPreference() otshape.NormalizationMode {
	return otshape.NormalizationAuto
}

// ApplyGPOS reports whether GPOS should be applied for Hebrew shaping.
func (Shaper) ApplyGPOS() bool {
	return true
}

// Compose attempts to compose runes a and b for Hebrew shaping.
//
// It first delegates to Unicode composition and then applies Hebrew-specific
// fallback compositions when appropriate. The returned bool reports success.
func (Shaper) Compose(c otshape.NormalizeContext, a, b rune) (rune, bool) {
	return hebrewCompose(c, a, b)
}

// ReorderNormalizationMarks adjusts an already sorted mark sequence before
// recomposition. Source indexes move with runes; the pipeline separately retains
// the initial level-0 group, including all marks even across feature events.
func (Shaper) ReorderNormalizationMarks(runes []rune, sources []uint32) {
	hebrewReorder(0, len(runes), func(i int) rune { return runes[i] }, func(i, j int) {
		runes[i], runes[j] = runes[j], runes[i]
		sources[i], sources[j] = sources[j], sources[i]
	})
}

// ReorderMarks retains the standalone adjustment for callers using RunContext.
// The shaping pipeline uses ReorderNormalizationMarks instead, before composition.
func (Shaper) ReorderMarks(run otshape.RunContext, start, end int) {
	hebrewReorderMarks(run, start, end)
}

func hebrewCompose(c otshape.NormalizeContext, a, b rune) (rune, bool) {
	ab, found := c.ComposeUnicode(a, b)
	if found {
		return ab, true
	}
	if c.HasGposMark() {
		return 0, false
	}

	// Special-case Hebrew presentation forms excluded from normalization.
	switch b {
	case 0x05B4: // HIRIQ
		if a == 0x05D9 { // YOD
			return 0xFB1D, true
		}
	case 0x05B7: // PATAH
		if a == 0x05F2 { // YIDDISH YOD YOD
			return 0xFB1F, true
		}
		if a == 0x05D0 { // ALEF
			return 0xFB2E, true
		}
	case 0x05B8: // QAMATS
		if a == 0x05D0 { // ALEF
			return 0xFB2F, true
		}
	case 0x05B9: // HOLAM
		if a == 0x05D5 { // VAV
			return 0xFB4B, true
		}
	case 0x05BC: // DAGESH
		if a >= 0x05D0 && a <= 0x05EA {
			ab = dageshForms[a-0x05D0]
			return ab, ab != 0
		}
		if a == 0xFB2A { // SHIN WITH SHIN DOT
			return 0xFB2C, true
		}
		if a == 0xFB2B { // SHIN WITH SIN DOT
			return 0xFB2D, true
		}
	case 0x05BF: // RAFE
		switch a {
		case 0x05D1: // BET
			return 0xFB4C, true
		case 0x05DB: // KAF
			return 0xFB4D, true
		case 0x05E4: // PE
			return 0xFB4E, true
		}
	case 0x05C1: // SHIN DOT
		if a == 0x05E9 { // SHIN
			return 0xFB2A, true
		}
		if a == 0xFB49 { // SHIN WITH DAGESH
			return 0xFB2C, true
		}
	case 0x05C2: // SIN DOT
		if a == 0x05E9 { // SHIN
			return 0xFB2B, true
		}
		if a == 0xFB49 { // SHIN WITH DAGESH
			return 0xFB2D, true
		}
	}
	return 0, false
}

func hebrewReorderMarks(run otshape.RunContext, start, end int) {
	if run == nil {
		return
	}
	if start < 0 {
		start = 0
	}
	if end > run.Len() {
		end = run.Len()
	}
	if end-start < 3 {
		return
	}
	hebrewReorder(start, end, run.Codepoint, func(i, j int) {
		run.MergeClusters(i, j+1)
		run.Swap(i, j)
	})
}

// HarfBuzz 15 reorder_marks_hebrew: one adjustment per mark sequence.
func hebrewReorder(start, end int, codepoint func(int) rune, swap func(int, int)) {
	for i := start + 2; i < end; i++ {
		c0 := hebreworder.Class(codepoint(i - 2))
		c1 := hebreworder.Class(codepoint(i - 1))
		c2 := hebreworder.Class(codepoint(i))
		if (c0 == mccPatah || c0 == mccQamats) &&
			(c1 == mccSheva || c1 == mccHiriq) &&
			(c2 == mccMeteg || c2 == combiningClassBelow) {
			swap(i-1, i)
			break
		}
	}
}

// StreamingNormalization reports the locality of the Hebrew normalization hooks.
// Presentation-form fallback also depends on plan-wide GPOS mark support.
func (Shaper) StreamingNormalization() (bool, bool) { return true, true }
