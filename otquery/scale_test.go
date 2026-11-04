package otquery

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestFontScale(t *testing.T) {
	for _, upem := range []uint16{16, 1000, 2048, 16384} {
		font := scaleTestFont(t, upem)
		for _, size := range []float64{1, 12, 16, 19.5} {
			scale, err := FontScale(font, size)
			if err != nil {
				t.Fatal(err)
			}
			// One em, a negative quarter em, and zero must convert without
			// rounding or changing the coordinate orientation.
			for _, fraction := range []float64{1, -0.25, 0} {
				got := (float64(upem) * fraction) * scale
				want := size * fraction
				if math.Abs(got-want) > 1e-12 {
					t.Errorf("upem=%d size=%g fraction=%g: got %g, want %g", upem, size, fraction, got, want)
				}
			}
		}
	}
}

func TestFontScaleInvalidInputs(t *testing.T) {
	font := scaleTestFont(t, 2048)
	for _, size := range []float64{0, -12, math.NaN(), math.Inf(1), math.Inf(-1), math.SmallestNonzeroFloat64} {
		if scale, err := FontScale(font, size); err == nil || scale != 0 {
			t.Errorf("size=%g: got %g, %v; want zero and error", size, scale, err)
		}
	}
	for _, font := range []*ot.Font{nil, {}} {
		if scale, err := FontScale(font, 16); err == nil || scale != 0 {
			t.Errorf("missing font/head: got %g, %v; want zero and error", scale, err)
		}
	}
	for _, upem := range []uint16{0, 15, 16385, 65535} {
		if scale, err := FontScale(scaleTestFont(t, upem), 16); err == nil || scale != 0 {
			t.Errorf("upem=%d: got %g, %v; want zero and error", upem, scale, err)
		}
	}
}

func TestFontScaleParsedFont(t *testing.T) {
	font := loadLocalFont(t, "Calibri.ttf")
	scale, err := FontScale(font, 16)
	if err != nil {
		t.Fatal(err)
	}
	// The checked-in Calibri font has 2048 units/em and an 'A' advance of 1185.
	gid, err := GlyphIndex(font, 'A')
	if err != nil {
		t.Fatal(err)
	}
	if got := float64(GlyphMetrics(font, gid).Advance) * scale; got != 9.2578125 {
		t.Fatalf("scaled A advance = %g; want 9.2578125 pixels", got)
	}
}

// Change UPEM in a private copy before parsing, leaving the original font's
// backing bytes immutable. Parsing currently accepts out-of-range UPEM values,
// so FontScale must validate them itself.
func scaleTestFont(t *testing.T, upem uint16) *ot.Font {
	t.Helper()
	original := loadLocalFont(t, "Calibri.ttf")
	headOffset, _ := original.Table(ot.T("head")).Extent()
	b := append([]byte(nil), original.Binary()...)
	binary.BigEndian.PutUint16(b[headOffset+18:], upem)
	font, err := ot.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return font
}
