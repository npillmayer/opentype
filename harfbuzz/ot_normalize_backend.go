package harfbuzz

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// otNormalizeBackend provides canonical Unicode normalization primitives used
// by ot_shape_normalize.go.
type otNormalizeBackend interface {
	canonicalCombiningClass(rune) uint8
	decompose(rune) (rune, rune, bool)
	compose(rune, rune) (rune, bool)
}

// Phase 4: fix normalization to the x/text backend and drop temporary
// side-by-side backend selection wiring.
func currentOTNormalizeBackend() otNormalizeBackend {
	return otNormalizeBackendXText{}
}

type otNormalizeBackendXText struct{}

func (otNormalizeBackendXText) canonicalCombiningClass(u rune) uint8 {
	return norm.NFC.PropertiesString(string(u)).CCC()
}

func (otNormalizeBackendXText) decompose(ab rune) (a, b rune, ok bool) {
	dec := norm.NFD.PropertiesString(string(ab)).Decomposition()
	if len(dec) == 0 {
		return ab, 0, false
	}

	first, n := utf8.DecodeRune(dec)
	if first == utf8.RuneError && n == 1 {
		return ab, 0, false
	}
	if n == len(dec) {
		return first, 0, true
	}

	second, m := utf8.DecodeRune(dec[n:])
	if second == utf8.RuneError && m == 1 {
		return ab, 0, false
	}
	if n+m != len(dec) {
		// Keep the normalization stage conservative: do not emit multi-rune
		// decompositions in this code path.
		return ab, 0, false
	}

	return first, second, true
}

func (otNormalizeBackendXText) compose(a, b rune) (rune, bool) {
	composed := norm.NFC.String(string([]rune{a, b}))
	first, n := utf8.DecodeRuneInString(composed)
	if first == utf8.RuneError && n == 1 {
		return 0, false
	}
	if n != len(composed) {
		return 0, false
	}
	return first, true
}
