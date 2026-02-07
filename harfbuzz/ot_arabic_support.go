package harfbuzz

/*
 * Arabic joining support used by generated joining tables and exported bridges.
 */

/* See:
 * https://github.com/harfbuzz/harfbuzz/commit/6e6f82b6f3dde0fc6c3c7d991d9ec6cfff57823d#commitcomment-14248516
 */
func isWord(genCat generalCategory) bool {
	const mask = 1<<unassigned |
		1<<privateUse |
		/*1 <<  LowercaseLetter |*/
		1<<modifierLetter |
		1<<otherLetter |
		/*1 <<  TitlecaseLetter |*/
		/*1 <<  UppercaseLetter |*/
		1<<spacingMark |
		1<<enclosingMark |
		1<<nonSpacingMark |
		1<<decimalNumber |
		1<<letterNumber |
		1<<otherNumber |
		1<<currencySymbol |
		1<<modifierSymbol |
		1<<mathSymbol |
		1<<otherSymbol
	return (1<<genCat)&mask != 0
}

// arabicJoining is a property used to shape Arabic runes.
// See the generated table arabicJoinings in ot_arabic_table.go.
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

func getJoiningType(u rune, genCat generalCategory) uint8 {
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

	const mask = 1<<nonSpacingMark | 1<<enclosingMark | 1<<format
	if 1<<genCat&mask != 0 {
		return joiningTypeT
	}
	return joiningTypeU
}
