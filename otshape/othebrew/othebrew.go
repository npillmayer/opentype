package othebrew

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
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
	mccSheva  = 10
	mccHiriq  = 14
	mccPatah  = 17
	mccQamats = 18
	mccMeteg  = 22

	combiningClassBelow = 220
)

// Shaper is the Hebrew shaping engine.
type Shaper struct{}

var _ otshape.ShapingEngine = Shaper{}
var _ otshape.ShapingEnginePolicy = Shaper{}
var _ otshape.ShapingEngineComposeHook = Shaper{}
var _ otshape.ShapingEngineReorderHook = Shaper{}

// New returns the Hebrew shaping engine.
func New() otshape.ShapingEngine {
	return Shaper{}
}

func (Shaper) Name() string {
	return "hebrew"
}

func (Shaper) Match(ctx otshape.SelectionContext) otshape.ShaperConfidence {
	if ctx.Script == hebrewScript || ctx.ScriptTag == ot.T("hebr") {
		return otshape.ShaperConfidenceCertain
	}
	return otshape.ShaperConfidenceNone
}

func (Shaper) New() otshape.ShapingEngine {
	return Shaper{}
}

func (Shaper) NormalizationPreference() otshape.NormalizationMode {
	return otshape.NormalizationAuto
}

func (Shaper) ApplyGPOS() bool {
	return true
}

func (Shaper) Compose(c otshape.NormalizeContext, a, b rune) (rune, bool) {
	return hebrewCompose(c, a, b)
}

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
	for i := start + 2; i < end; i++ {
		c0 := hebrewModifiedCombiningClass(run.Codepoint(i - 2))
		c1 := hebrewModifiedCombiningClass(run.Codepoint(i - 1))
		c2 := hebrewModifiedCombiningClass(run.Codepoint(i))

		if (c0 == mccPatah || c0 == mccQamats) &&
			(c1 == mccSheva || c1 == mccHiriq) &&
			(c2 == mccMeteg || c2 == combiningClassBelow) {
			run.MergeClusters(i-1, i+1)
			run.Swap(i-1, i)
			break
		}
	}
}

func hebrewModifiedCombiningClass(cp rune) uint8 {
	switch cp {
	case 0x05B0: // SHEVA
		return mccSheva
	case 0x05B4: // HIRIQ
		return mccHiriq
	case 0x05B7: // PATAH
		return mccPatah
	case 0x05B8: // QAMATS
		return mccQamats
	case 0x05BD: // METEG
		return mccMeteg
	}
	return norm.NFD.PropertiesString(string(cp)).CCC()
}
