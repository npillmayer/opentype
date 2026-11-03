package proof

import (
	"context"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"

	"golang.org/x/text/unicode/bidi"
)

func TestClusterSpans(t *testing.T) {
	for _, c := range []struct {
		anchors []uint32
		n       int
		want    [][2]int
	}{
		{[]uint32{0, 1, 4, 5}, 6, [][2]int{{0, 1}, {1, 4}, {4, 5}, {5, 6}}}, // office, ffi ligature
		{[]uint32{0, 0, 0, 3, 4, 4, 6}, 7, [][2]int{{0, 3}, {0, 3}, {0, 3}, {3, 4}, {4, 6}, {4, 6}, {6, 7}}},
		{[]uint32{4, 0, 0}, 6, [][2]int{{4, 6}, {0, 4}, {0, 4}}}, // no sorting records
		{nil, 0, [][2]int{}},
	} {
		if got := ClusterSpans(c.anchors, c.n); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%v -> %v, want %v", c.anchors, got, c.want)
		}
	}
}
func TestScreenGeometryUsesFinalPlacement(t *testing.T) {
	sf := proofTestFont(t)
	glyphs := []otshape.GlyphRecord{
		{GID: 82, Cluster: 0, Pos: otlayout.PosItem{XAdvance: 600, YAdvance: 5}},
		{GID: 72, Cluster: 0, Pos: otlayout.PosItem{XOffset: 20, YOffset: 100}},
		{GID: 70, Cluster: 0, Pos: otlayout.PosItem{XOffset: 30, YOffset: 150}},
		{GID: 3, Cluster: 3, Pos: otlayout.PosItem{XAdvance: 200}},
	}
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		rows := []comparisonRow{{label: "base and marks", glyphs: glyphs}, {label: "short", glyphs: glyphs[:1]}}
		opts := Options{PPEM: 96, Direction: dir}
		opts.Diagnostics()
		img, geometry, err := renderComparison(sf, rows, opts)
		if err != nil {
			t.Fatal(err)
		}
		if geometry.Width != img.Bounds().Dx() || geometry.Height != img.Bounds().Dy() {
			t.Fatal("image mismatch")
		}
		scale := float64(opts.PPEM) / float64(sf.UnitsPerEm())
		data := glyphData(glyphs, dir)
		for i, g := range geometry.Rows[0].Glyphs {
			if g.Index != i || g.Hit.Width <= 0 || g.Hit.Height <= 0 {
				t.Fatalf("unselectable logical glyph %d: %+v", i, g)
			}
			if math.Abs(g.Origin.X-g.Pen.X-float64(data[i].XOffset)*scale) > 1e-9 || math.Abs(g.Origin.Y-g.Pen.Y+float64(data[i].YOffset)*scale) > 1e-9 {
				t.Fatal("screen offset sign/scale incorrect")
			}
			if math.Abs(g.Next.X-g.Pen.X-float64(data[i].XAdvance)*scale) > 1e-9 {
				t.Fatal("nominal advance added twice")
			}
			if g.Ink != nil && (g.Ink.X < 0 || g.Ink.Y < 0 || g.Ink.X+g.Ink.Width > float64(geometry.Width) || g.Ink.Y+g.Ink.Height > float64(geometry.Height)) {
				t.Fatal("clipped ink")
			}
		}
		long, short := geometry.Rows[0].Glyphs, geometry.Rows[1].Glyphs
		if dir == bidi.LeftToRight && long[0].Pen.X != short[0].Pen.X {
			t.Fatal("LTR start drift")
		}
		if dir == bidi.RightToLeft && math.Abs(long[0].Next.X-short[0].Next.X) > 1e-9 {
			t.Fatal("RTL start drift")
		}
	}
}
func TestExecuteCancellationAndRandomNotice(t *testing.T) {
	data, err := os.ReadFile("../../testdata/fonts/GentiumPlus-R.ttf")
	if err != nil {
		t.Fatal(err)
	}
	text := "office"
	c := Case{Version: 1, Font: "font.ttf", Text: &text, Variants: []Variant{{Label: "random", Features: []string{"salt=-1"}}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := ExecuteContext(ctx, c, data); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled result: %v, %v", got, err)
	}
	result, err := Execute(c, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Report.Notes) != 1 {
		t.Fatal("random replay limitation not surfaced")
	}
}
