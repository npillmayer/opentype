package hbcmp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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
	"hebrew_shalom":         "",
	"hebrew_shin_pointed":   "SHAPE-011",
	"hebrew_vav_holam":      "SHAPE-010",
	"hebrew_shalom_pointed": "SHAPE-010",
	"latin_marks_canonical": "", "latin_marks_reordered": "SHAPE-012",
	"latin_uncomposable_mark":     "",
	"latin_missing_composite_nfc": "SHAPE-013", "latin_missing_composite_nfd": "SHAPE-013",
	"latin_range_after_nfc": "", "latin_range_after_nfd": "",
	"hebrew_vav_holam_mark_on": "", "hebrew_vav_holam_mark_off": "SHAPE-010",
	"hebrew_vav_holam_no_gpos": "SHAPE-014",
	"hebrew_vav_holam_no_mark": "", "hebrew_vav_holam_no_mark_requested": "SHAPE-010",
	"hebrew_shin_reordered": "",
}

// Load every record even for a filtered run: missing fonts, fields or cases
// must fail rather than silently shrink the offline corpus.
func loadTextBaselines(t *testing.T) []textBaseline {
	return loadTextBaselineFile(t, "testdata/text/baselines.json", textBaselineFindings, nil)
}

func loadTextBaselineFile(t *testing.T, path string, assignments map[string]string, emptyOutputs map[string]bool) []textBaseline {
	t.Helper()
	data, err := os.ReadFile(path)
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
		if _, ok := assignments[c.ID]; !ok {
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
		if f.Output == nil || (len(f.Output) == 0 && !emptyOutputs[c.ID]) {
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
	if len(seen) != len(assignments) {
		t.Fatalf("got %d text baselines, want %d assigned cases", len(seen), len(assignments))
	}
	return corpus.Cases
}

func runTextBaseline(t *testing.T, c textBaseline) {
	t.Helper()
	font, err := ot.Parse(c.fontData)
	if err != nil {
		t.Fatalf("%s: parse font: %v", c.ID, err)
	}
	// The legacy fixture converter supports only global features. Convert the
	// text corpus's explicitly bounded scalar ranges separately, before shaping.
	f := c.Fixture
	f.Context.Features = nil
	params, err := f.params(font)
	if err != nil {
		t.Fatal(err)
	}
	params.Features, err = parseTextFeatures(c.Fixture.Context.Features, len(c.Fixture.Input))
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
	want := textLogicalOutput(c.Fixture)
	if mismatch := comparePositionedGlyphs(got, want); err != nil || mismatch != nil {
		t.Fatalf("%s font=%s context=%+v buffers=%+v\nshape error=%v; comparison=%v\ngot=%+v\nwant logical=%+v\nraw HarfBuzz=%+v\nrecords=%+v", c.ID, c.FontPath, c.Fixture.Context, opts, err, mismatch, got, want, c.Fixture.Output, sink.glyphs)
	}
}

// Undo HB's final whole-buffer RTL reversal, including records within a
// cluster. Never sort by cluster: that would lose the oracle's internal order.
// Positions remain HB rendering data; a renderer must perform visual ordering
// before accumulating advances. The raw capture is never modified.
func textLogicalOutput(f fixture) []hbShapedGlyph {
	out := slices.Clone(f.Output)
	if f.Context.Dir == "rtl" {
		slices.Reverse(out)
	}
	return out
}

var textFeaturePattern = regexp.MustCompile(`^([A-Za-z0-9]{4})(?:\[([0-9]+):([0-9]+)\])?=([0-9]+)$`)

func parseTextFeatures(specs []string, inputLen int) ([]otshape.FeatureRange, error) {
	var out []otshape.FeatureRange
	for _, spec := range specs {
		m := textFeaturePattern.FindStringSubmatch(spec)
		if m == nil {
			return nil, fmt.Errorf("unsupported text feature %q", spec)
		}
		value, err := strconv.ParseUint(m[4], 10, 31)
		if err != nil {
			return nil, err
		}
		f := otshape.FeatureRange{Feature: ot.T(m[1]), On: value != 0, Arg: int(value)}
		if m[2] != "" {
			start, e1 := strconv.ParseUint(m[2], 10, 31)
			end, e2 := strconv.ParseUint(m[3], 10, 31)
			if e1 != nil || e2 != nil || start >= end || end > uint64(inputLen) {
				return nil, fmt.Errorf("invalid scalar range %q for length %d", spec, inputLen)
			}
			f.Start, f.End = int(start), int(end)
		}
		out = append(out, f)
	}
	return out, nil
}

func TestTextLogicalOutputContract(t *testing.T) {
	// A repeated cluster must reverse internally too; distinct offsets must
	// stay attached to their glyph. LTR captures are copied unchanged.
	raw := []hbShapedGlyph{{G: 79, Cl: 0, DX: 227}, {G: 100, Cl: 0, DX: 539}, {G: 96, Cl: 0, AX: 730}}
	for _, dir := range []string{"ltr", "rtl"} {
		f := fixture{Context: fixtureContext{Dir: dir}, Output: slices.Clone(raw)}
		want := slices.Clone(raw)
		if dir == "rtl" {
			want = []hbShapedGlyph{raw[2], raw[1], raw[0]}
		}
		got := textLogicalOutput(f)
		if !slices.Equal(got, want) || !slices.Equal(f.Output, raw) {
			t.Fatalf("%s: got %+v, want %+v; raw %+v", dir, got, want, f.Output)
		}
	}
}

func TestTextFeatureScalarRanges(t *testing.T) {
	got, err := parseTextFeatures([]string{"liga=0", "liga[3:6]=1"}, len([]rune("e\u0301office")))
	want := []otshape.FeatureRange{{Feature: ot.T("liga")}, {Feature: ot.T("liga"), On: true, Arg: 1, Start: 3, End: 6}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
	for _, spec := range []string{"liga[3:9]=1", "liga[3:3]=1", "liga[-1:3]=1", "liga[0:]=1", "liga[2]=1", "liga=2147483648"} {
		if _, err := parseTextFeatures([]string{spec}, 8); err == nil {
			t.Errorf("accepted %q", spec)
		}
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
