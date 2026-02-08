package otarabic

// arabicJoining is a property used to shape Arabic runes.
// See the generated table arabicJoinings in joining_table.go.
type arabicJoining byte

const (
	ajU          arabicJoining = 'U' // Un-joining, e.g. Full Stop
	ajR          arabicJoining = 'R' // Right-joining, e.g. Arabic Letter Dal
	ajAlaph      arabicJoining = 'a' // Alaph group (included in kind R)
	ajDalathRish arabicJoining = 'd' // Dalat Rish group (included in kind R)
	ajD          arabicJoining = 'D' // Dual-joining, e.g. Arabic Letter Ain
	ajC          arabicJoining = 'C' // Join-Causing, e.g. Tatweel, ZWJ
	ajL          arabicJoining = 'L' // Left-joining, i.e. fictional
	ajT          arabicJoining = 'T' // Transparent, e.g. Arabic Fatha
)

// Local copy of harfbuzz.generalCategory numeric codes used by Arabic logic.
const (
	gcFormat         = 1
	gcUnassigned     = 2
	gcPrivateUse     = 3
	gcModifierLetter = 6
	gcOtherLetter    = 7
	gcSpacingMark    = 10
	gcEnclosingMark  = 11
	gcNonSpacingMark = 12
	gcDecimalNumber  = 13
	gcLetterNumber   = 14
	gcOtherNumber    = 15
	gcCurrencySymbol = 23
	gcModifierSymbol = 24
	gcMathSymbol     = 25
	gcOtherSymbol    = 26
)

/* See:
 * https://github.com/harfbuzz/harfbuzz/commit/6e6f82b6f3dde0fc6c3c7d991d9ec6cfff57823d#commitcomment-14248516
 */
func arabicIsWord(genCat uint8) bool {
	const mask = 1<<gcUnassigned |
		1<<gcPrivateUse |
		1<<gcModifierLetter |
		1<<gcOtherLetter |
		1<<gcSpacingMark |
		1<<gcEnclosingMark |
		1<<gcNonSpacingMark |
		1<<gcDecimalNumber |
		1<<gcLetterNumber |
		1<<gcOtherNumber |
		1<<gcCurrencySymbol |
		1<<gcModifierSymbol |
		1<<gcMathSymbol |
		1<<gcOtherSymbol
	return (1<<genCat)&mask != 0
}

func getJoiningType(u rune, genCat uint8) uint8 {
	if jType, ok := arabicJoinings[u]; ok {
		switch jType {
		case ajU:
			return joiningTypeU
		case ajL:
			return joiningTypeL
		case ajR:
			return joiningTypeR
		case ajD:
			return joiningTypeD
		case ajAlaph:
			return joiningGroupAlaph
		case ajDalathRish:
			return joiningGroupDalathRish
		case ajT:
			return joiningTypeT
		case ajC:
			return joiningTypeC
		}
	}

	const mask = 1<<gcNonSpacingMark | 1<<gcEnclosingMark | 1<<gcFormat
	if 1<<genCat&mask != 0 {
		return joiningTypeT
	}
	return joiningTypeU
}
