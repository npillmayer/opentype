package proof

import (
	"math"
	"testing"

	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"

	"golang.org/x/text/unicode/bidi"
)

func TestFeatureRanges(t *testing.T) {
	for _, c := range []struct {
		spec            string
		start, end, arg int
		on              bool
	}{
		{"liga[7:13]=1", 7, 13, 1, true}, {"-kern[:3]", 0, 3, 1, false},
		{"salt[2:]=-1", 2, 0, -1, true}, {"+liga[:]", 0, 0, 1, true},
	} {
		f, err := ParseFeatureItem(c.spec)
		if err != nil || f.Start != c.start || f.End != c.end || f.Arg != c.arg || f.On != c.on {
			t.Errorf("%s: %+v %v", c.spec, f, err)
		}
	}
	for _, s := range []string{"liga[0:0]", "liga[2:2]", "liga[-1:3]", "liga[0:-1]", "liga[3]", "liga[1:2:3]", "liga[1:2", "liga[1:2]x", "liga[1:2][3:4]"} {
		if _, err := ParseFeatureItem(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestRunPresets(t *testing.T) {
	h, err := (Settings{Preset: "hebrew"}).Resolve()
	if err != nil || h.Script != "Hebr" || h.Language != "he" || h.Direction != "rtl" {
		t.Fatalf("Hebrew preset: %+v %v", h, err)
	}
	h, err = (Settings{Preset: "hebrew", Language: "en", Direction: "ltr"}).Resolve()
	if err != nil || h.Script != "Hebr" || h.Language != "en" || h.Direction != "ltr" {
		t.Fatalf("override: %+v %v", h, err)
	}
	if _, err := (Settings{Preset: "automatic"}).Resolve(); err == nil {
		t.Fatal("accepted unknown preset")
	}
}

func TestComparisonStartAlignmentAndPens(t *testing.T) {
	sf := proofTestFont(t)
	a := []otshape.GlyphRecord{{GID: 82, Pos: otlayout.PosItem{XAdvance: 600, XOffset: 10, YOffset: 20}}, {GID: 3, Pos: otlayout.PosItem{XAdvance: 200}}}
	b := []otshape.GlyphRecord{{GID: 82, Pos: otlayout.PosItem{XAdvance: 400, XOffset: 10, YOffset: 20}}}
	rows := []comparisonRow{{label: "long", glyphs: a}, {label: "short", glyphs: b}}
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		opts := Options{PPEM: int(sf.UnitsPerEm()), Direction: dir}
		opts.Diagnostics()
		layouts, _, err := comparisonGeometry(sf, rows, opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range layouts {
			if dir == bidi.LeftToRight {
				if l.glyphs[0].penX != 0 || l.glyphs[0].x != 10 {
					t.Fatalf("LTR start drift: %+v", l)
				}
			} else {
				last := l.glyphs[len(l.glyphs)-1]
				if last.nextX != 0 || math.Abs(last.x-last.penX-10) > 1e-9 {
					t.Fatalf("RTL right edge/offset drift: %+v", l)
				}
			}
		}
	}
	report := glyphData(a, bidi.RightToLeft)
	if report[0].Index != 0 || report[0].VisualIndex != 1 || report[0].PenX != 200 || report[0].OriginX != 210 || report[0].OriginY != 20 || report[1].PenX != 0 {
		t.Fatalf("font-unit logical report: %+v", report)
	}

}
