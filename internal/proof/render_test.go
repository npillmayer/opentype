package proof

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func proofTestFont(t *testing.T) *sfnt.Font {
	t.Helper()
	data, err := os.ReadFile("../../testdata/fonts/GentiumPlus-R.ttf")
	if err != nil {
		t.Fatal(err)
	}
	sf, err := sfnt.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return sf
}

func TestProofFinalAdvancesAndRTLMarks(t *testing.T) {
	sf := proofTestFont(t)
	// A base, two zero-advance marks in the same cluster, then a space.
	// Deliberately use synthetic metrics distinct from this font's metrics.
	glyphs := []otshape.GlyphRecord{
		{GID: 82, Cluster: 0, Pos: otlayout.PosItem{XAdvance: 600, YAdvance: 5}},
		{GID: 72, Cluster: 0, Pos: otlayout.PosItem{XOffset: 20, YOffset: 100}},
		{GID: 70, Cluster: 0, Pos: otlayout.PosItem{XOffset: 30, YOffset: 150}},
		{GID: 3, Cluster: 2, Pos: otlayout.PosItem{XAdvance: 200, YAdvance: 7}},
	}
	before := append([]otshape.GlyphRecord(nil), glyphs...)
	cases := []struct {
		dir    bidi.Direction
		order  []int
		xs, ys []float64
	}{
		{bidi.LeftToRight, []int{0, 1, 2, 3}, []float64{0, 620, 630, 600}, []float64{0, -105, -155, -5}},
		{bidi.RightToLeft, []int{3, 2, 1, 0}, []float64{0, 230, 220, 200}, []float64{0, -157, -107, -7}},
	}
	for _, c := range cases {
		p, err := layoutProof(sf, glyphs, Options{PPEM: int(sf.UnitsPerEm()), Direction: c.dir})
		if err != nil {
			t.Fatal(err)
		}
		for j, g := range p.glyphs {
			if g.logicalIndex != c.order[j] || g.x != c.xs[j] || g.y != c.ys[j] {
				t.Fatalf("dir=%v glyph=%d: %+v", c.dir, j, g)
			}
		}
		if p.maxX < 800 || p.minY > -157 {
			t.Fatalf("lost advance/ink extents: %+v", p)
		}
	}
	if !reflect.DeepEqual(glyphs, before) {
		t.Fatal("renderer mutated logical records")
	}
}

func TestProofCanvasAndErrors(t *testing.T) {
	sf := proofTestFont(t)
	space := []otshape.GlyphRecord{{GID: 3, Pos: otlayout.PosItem{XAdvance: 500}}}
	opts := Options{PPEM: 96, Direction: bidi.LeftToRight}
	img, err := Render(sf, space, opts)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() < 2*proofPadding+23 {
		t.Fatalf("space advance lost: %v", img.Bounds())
	}
	for i := 0; i < len(img.Pix); i++ {
		if img.Pix[i] != 255 {
			t.Fatal("space proof is not blank white")
		}
	}
	opts.ShowBoxes, opts.ShowOrigins, opts.ShowBaseline, opts.ShowLabels = true, true, true, true
	diagnostic, err := Render(sf, space, opts)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.Bounds().Dy() <= img.Bounds().Dy() {
		t.Fatal("legend has no reserved space")
	}
	for _, c := range []struct {
		name    string
		opts    Options
		glyphs  []otshape.GlyphRecord
		message string
	}{
		{"small", Options{PPEM: 96, Width: 1}, space, "canvas too small"},
		{"huge", Options{PPEM: 96, Width: maxProofPixels, Height: maxProofPixels}, space, "image budget"},
		{"negative", Options{PPEM: 96, Width: -1}, space, "nonnegative"},
		{"scale", Options{PPEM: maxProofPPEM + 1}, space, "--ppem"},
		{"direction", Options{PPEM: 96, Direction: bidi.Mixed}, space, "ltr or rtl"},
		{"outline", Options{PPEM: 96}, []otshape.GlyphRecord{{GID: 65535}}, "cannot load glyph[0]"},
		{"advance", Options{PPEM: 96}, []otshape.GlyphRecord{{GID: 3, Pos: otlayout.PosItem{XAdvance: math.MaxInt32}}}, "image budget"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Render(sf, c.glyphs, c.opts)
			if err == nil || !strings.Contains(err.Error(), c.message) {
				t.Fatalf("error=%v, want %q", err, c.message)
			}
		})
	}
}

// These records are read from the existing independent HB 15 capture, not
// recaptured or generated from our shaper. Render captured visual order as LTR
// and our logical records with their actual direction; compare every record
// field, absolute glyph origin, and resulting pixels.
func TestProofHarfBuzzCaptures(t *testing.T) {
	type hbGlyph struct {
		G  int    `json:"g"`
		CL uint32 `json:"cl"`
		DX int32  `json:"dx"`
		DY int32  `json:"dy"`
		AX int32  `json:"ax"`
		AY int32  `json:"ay"`
	}
	var corpus struct {
		Cases []struct {
			ID       string `json:"id"`
			Text     string `json:"text"`
			FontPath string `json:"font_path"`
			Hash     string `json:"font_sha256"`
			Fixture  struct {
				Context struct {
					Script, Language, Dir string
					Features              []string
				}
				Output []hbGlyph
			}
		}
	}
	data, err := os.ReadFile("../hbcmp/testdata/text/baselines.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{
		"latin_office_liga_on": false, "latin_avatar_kern_on": false,
		"latin_cafe_nfd": false, "latin_uncomposable_mark": false,
		"hebrew_shalom": false, "hebrew_shin_pointed": false, "hebrew_shalom_pointed": false,
	}
	for _, c := range corpus.Cases {
		if _, ok := selected[c.ID]; !ok {
			continue
		}
		selected[c.ID] = true
		t.Run(c.ID, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../..", c.FontPath))
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != c.Hash {
				t.Fatal("font differs from independent capture")
			}
			otf, err := ot.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			sf, err := sfnt.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			script, err := language.ParseScript(c.Fixture.Context.Script)
			if err != nil {
				t.Fatal(err)
			}
			lang, err := language.Parse(c.Fixture.Context.Language)
			if err != nil {
				t.Fatal(err)
			}
			dir := bidi.LeftToRight
			if c.Fixture.Context.Dir == "rtl" {
				dir = bidi.RightToLeft
			}
			var features []otshape.FeatureRange
			for _, item := range c.Fixture.Context.Features {
				f, err := ParseFeatureItem(item)
				if err != nil {
					t.Fatal(err)
				}
				features = append(features, f)
			}
			sink := &Collector{}
			if err := Shape(IO{strings.NewReader(c.Text), sink}, otshape.Params{Font: otf, Script: script, Language: lang, Direction: dir, Features: features}, otshape.BufferOptions{}); err != nil {
				t.Fatal(err)
			}
			if len(sink.Glyphs) != len(c.Fixture.Output) {
				t.Fatalf("glyph count=%d want %d", len(sink.Glyphs), len(c.Fixture.Output))
			}
			want := make([]otshape.GlyphRecord, len(c.Fixture.Output))
			for i, g := range c.Fixture.Output {
				want[i] = otshape.GlyphRecord{GID: ot.GlyphIndex(g.G), Cluster: g.CL, Pos: otlayout.PosItem{XAdvance: g.AX, YAdvance: g.AY, XOffset: g.DX, YOffset: g.DY}}
			}
			for _, ppem := range []int{48, 96} {
				opts := Options{PPEM: ppem, Direction: dir}
				layout, err := layoutProof(sf, sink.Glyphs, opts)
				if err != nil {
					t.Fatal(err)
				}
				var px, py int64
				scale := float64(ppem) / float64(sf.UnitsPerEm())
				for i, pg := range layout.glyphs {
					w := want[i]
					p := pg.record.Pos
					if pg.record.GID != w.GID || pg.record.Cluster != w.Cluster || p.XAdvance != w.Pos.XAdvance || p.YAdvance != w.Pos.YAdvance || p.XOffset != w.Pos.XOffset || p.YOffset != w.Pos.YOffset {
						t.Fatalf("visual glyph %d differs from capture: got %+v want %+v", i, pg.record, w)
					}
					wx, wy := float64(px+int64(w.Pos.XOffset))*scale, -float64(py+int64(w.Pos.YOffset))*scale
					if math.Abs(pg.x-wx) > 1e-9 || math.Abs(pg.y-wy) > 1e-9 {
						t.Fatalf("glyph %d origin (%v,%v), want (%v,%v)", i, pg.x, pg.y, wx, wy)
					}
					px += int64(w.Pos.XAdvance)
					py += int64(w.Pos.YAdvance)
				}
				gotImage, err := Render(sf, sink.Glyphs, opts)
				if err != nil {
					t.Fatal(err)
				}
				opts.Direction = bidi.LeftToRight
				wantImage, err := Render(sf, want, opts)
				if err != nil {
					t.Fatal(err)
				}
				if gotImage.Rect != wantImage.Rect || !bytes.Equal(gotImage.Pix, wantImage.Pix) {
					t.Fatal("render differs from independently captured visual glyph stream")
				}
			}
		})
	}
	for id, found := range selected {
		if !found {
			t.Errorf("missing capture %s", id)
		}
	}
}
