package hbcmp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
)

type textBaseline struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	FontPath   string  `json:"font_path"`
	FontSHA256 string  `json:"font_sha256"`
	Fixture    fixture `json:"fixture"`
	fontData   []byte
}

// Empty means ordinary regression; other entries name the findings which
// currently block exact parity. Tagged tests retain the original HB output.
var textBaselineFindings = map[string]string{
	"latin_office_liga_on": "", "latin_office_liga_off": "",
	"latin_avatar_kern_on": "", "latin_avatar_kern_off": "",
	"latin_cafe_nfc": "", "latin_cafe_nfd": "", "hebrew_shin": "",
	"hebrew_shalom":         "SHAPE-009",
	"hebrew_shin_pointed":   "SHAPE-007/009/011",
	"hebrew_vav_holam":      "SHAPE-010",
	"hebrew_shalom_pointed": "SHAPE-007/009/010/011",
}

// Load every record even for a filtered run: missing fonts, fields or cases
// must fail rather than silently shrink the offline corpus.
func loadTextBaselines(t *testing.T) []textBaseline {
	t.Helper()
	data, err := os.ReadFile("testdata/text/baselines.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		SchemaVersion int `json:"schema_version"`
		Capture       struct {
			Version        string `json:"version"`
			ClusterIndices string `json:"cluster_indices"`
			ClusterLevel   int    `json:"cluster_level"`
			Units          string `json:"units"`
			Normalization  string `json:"normalization"`
		} `json:"capture"`
		Cases []textBaseline `json:"cases"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != 1 || corpus.Capture.Version != "hb-shape (HarfBuzz) 15.0.0" ||
		corpus.Capture.ClusterIndices != "unicode_scalars" || corpus.Capture.ClusterLevel != 0 ||
		corpus.Capture.Units != "font_units" || corpus.Capture.Normalization != "HarfBuzz default; input unmodified" {
		t.Fatalf("unsupported text baseline capture: %+v", corpus.Capture)
	}
	// Check field presence separately: absent positions must not become zero
	// expectations through Go's default values.
	var raw struct {
		Cases []struct {
			Fixture struct {
				Output []map[string]json.RawMessage `json:"output"`
			} `json:"fixture"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for i := range corpus.Cases {
		c := &corpus.Cases[i]
		if c.ID == "" || seen[c.ID] {
			t.Fatalf("empty/duplicate case ID %q", c.ID)
		}
		seen[c.ID] = true
		if _, ok := textBaselineFindings[c.ID]; !ok {
			t.Fatalf("%s: case has no test assignment", c.ID)
		}
		f := c.Fixture
		if err := f.validate(); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		if f.SchemaVersion != 1 || f.Context.TestFont || f.Context.Normalization != "" ||
			c.FontPath != "testdata/fonts/"+filepath.Base(f.Context.Font) || f.Context.Font != filepath.Base(f.Context.Font) {
			t.Fatalf("%s: unsupported fixture context/path", c.ID)
		}
		runes, err := f.runes()
		if err != nil || string(runes) != c.Text {
			t.Fatalf("%s: text/input mismatch: %v", c.ID, err)
		}
		c.fontData, err = readAOTSHashedFile(filepath.Join("..", "..", filepath.FromSlash(c.FontPath)), c.FontSHA256)
		if err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		if len(f.Output) == 0 {
			t.Fatalf("%s: empty oracle", c.ID)
		}
		for j, g := range f.Output {
			if g.G <= 0 || g.G > 65535 || int(g.Cl) >= len(runes) {
				t.Fatalf("%s: invalid glyph/cluster at %d", c.ID, j)
			}
			fields := raw.Cases[i].Fixture.Output[j]
			if len(fields) != 6 {
				t.Fatalf("%s: expected six output fields at %d", c.ID, j)
			}
			for _, key := range []string{"g", "cl", "dx", "dy", "ax", "ay"} {
				value, ok := fields[key]
				if !ok || string(value) == "null" {
					t.Fatalf("%s: missing %s at %d", c.ID, key, j)
				}
			}
		}
	}
	if len(seen) != len(textBaselineFindings) {
		t.Fatalf("got %d text baselines, want %d assigned cases", len(seen), len(textBaselineFindings))
	}
	return corpus.Cases
}

func runTextBaseline(t *testing.T, c textBaseline) {
	t.Helper()
	font, err := ot.Parse(c.fontData)
	if err != nil {
		t.Fatalf("%s: parse font: %v", c.ID, err)
	}
	params, err := c.Fixture.params(font)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the entire input together. Streaming boundaries are a separate axis.
	n := len(c.Fixture.Input)
	opts := otshape.BufferOptions{HighWatermark: n + 1, LowWatermark: 1, MaxBuffer: n + 2, FlushBoundary: otshape.FlushOnRunBoundary}
	sink := &glyphCollector{}
	err = otshape.NewShaper(otcore.New(), othebrew.New()).Shape(params, strings.NewReader(c.Text), sink, opts)
	got := make([]hbShapedGlyph, len(sink.glyphs))
	for i, g := range sink.glyphs {
		got[i] = glyphRecord2hbShapedGlyph(g)
	}
	if mismatch := comparePositionedGlyphs(got, c.Fixture.Output); err != nil || mismatch != nil {
		t.Fatalf("%s font=%s context=%+v buffers=%+v\nshape error=%v; comparison=%v\ngot=%+v\nwant=%+v\nrecords=%+v", c.ID, c.FontPath, c.Fixture.Context, opts, err, mismatch, got, c.Fixture.Output, sink.glyphs)
	}
}

func TestHarfBuzzTextBaselines(t *testing.T) {
	for _, c := range loadTextBaselines(t) {
		if textBaselineFindings[c.ID] != "" {
			continue
		}
		t.Run(c.ID, func(t *testing.T) { runTextBaseline(t, c) })
	}
}

func runTextBaselineID(t *testing.T, id string) {
	t.Helper()
	for _, c := range loadTextBaselines(t) {
		if c.ID == id {
			runTextBaseline(t, c)
			return
		}
	}
	t.Fatalf("text baseline %q not found", id)
}
