package proof

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestFeatureSwitches(t *testing.T) {
	for _, c := range []struct {
		s   string
		on  bool
		arg int
	}{
		{"liga", true, 1}, {"+liga", true, 1}, {"-liga", false, 1},
		{"liga=0", false, 0}, {"liga=1", true, 1}, {"salt=2", true, 2},
		{"salt=-1", true, -1}, {"salt=-9", true, -9},
	} {
		t.Run(c.s, func(t *testing.T) {
			f, err := ParseFeatureItem(c.s)
			if err != nil {
				t.Fatal(err)
			}
			if f.On != c.on || f.Arg != c.arg || (f.Feature != ot.T("liga") && f.Feature != ot.T("salt")) {
				t.Fatalf("wrong feature: %+v", f)
			}
		})
	}
	for _, s := range []string{"", "lig", "liga=", "liga=x", "liga[5:3]"} {
		if _, err := ParseFeatureItem(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestUnicodeScalarInput(t *testing.T) {
	for _, s := range []string{"D800", "U+DFFF", "110000", "FFFFFFFF", "not-hex"} {
		if _, err := ParseCodepointToken(s); err == nil {
			t.Errorf("accepted invalid scalar %q", s)
		}
	}
	for _, c := range []struct {
		s string
		r rune
	}{{"U+05E9", 'ש'}, {"0x0301", '\u0301'}, {"0000", 0}, {"10FFFF", 0x10ffff}} {
		r, err := ParseCodepointToken(c.s)
		if err != nil || r != c.r {
			t.Errorf("%q: %U, %v", c.s, r, err)
		}
	}
}
