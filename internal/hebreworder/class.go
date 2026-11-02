// Package hebreworder supplies the Hebrew shaping combining classes shared by
// normalization and the Hebrew engine. These are not Unicode normalization CCCs.
package hebreworder

import "golang.org/x/text/unicode/norm"

// Class follows HarfBuzz 15's Hebrew permutation of fixed-position CCCs 10–26.
// Other classes, including class-zero barriers, remain unchanged.
// https://github.com/harfbuzz/harfbuzz/blob/15.0.0/src/hb-unicode.hh#L301-L333
func Class(cp rune) uint8 {
	ccc := norm.NFD.PropertiesString(string(cp)).CCC()
	if ccc >= 10 && ccc <= 26 {
		return [...]uint8{22, 15, 16, 17, 23, 18, 19, 20, 21, 14, 24, 12, 25, 13, 10, 11, 26}[ccc-10]
	}
	return ccc
}
