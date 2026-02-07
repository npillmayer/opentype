package harfbuzz

import "testing"

func TestCurrentOTNormalizeBackendIsXText(t *testing.T) {
	if _, ok := currentOTNormalizeBackend().(otNormalizeBackendXText); !ok {
		t.Fatalf("expected current normalization backend to be x/text")
	}
}

func TestOTNormalizeBackendXTextSmoke(t *testing.T) {
	b := otNormalizeBackendXText{}

	if got := b.canonicalCombiningClass(0x0301); got != 230 {
		t.Fatalf("ccc mismatch for U+0301: got=%d want=230", got)
	}

	a, r, ok := b.decompose(0x00C5) // LATIN CAPITAL LETTER A WITH RING ABOVE
	if !ok || a != 0x0041 || r != 0x030A {
		t.Fatalf("decompose U+00C5: got=(U+%04X,U+%04X,%v), want=(U+0041,U+030A,true)", a, r, ok)
	}

	ab, ok := b.compose(0x0041, 0x030A)
	if !ok || ab != 0x00C5 {
		t.Fatalf("compose U+0041+U+030A: got=(U+%04X,%v), want=(U+00C5,true)", ab, ok)
	}
}
