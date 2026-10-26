package hbcmp

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
)

const aotsRevision = "29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b"

// This adapter accepts glyph-only AOTS cases with global or single-index test
// features. It is not a general HarfBuzz command-line parser.
// Missing positions and clusters never become zero-valued expectations.
type aotsGSUBCase struct {
	id       string
	fontPath string
	fontData []byte
	options  string
	features []otshape.FeatureRange
	input    []rune
	want     []ot.GlyphIndex
}

type aotsManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Revision      string `json:"upstream_revision"`
	Fonts         []struct {
		LocalPath string `json:"local_path"`
		SHA256    string `json:"sha256"`
	} `json:"fonts"`
	Sources []struct {
		LocalPath string `json:"local_path"`
		SHA256    string `json:"sha256"`
	} `json:"source_files"`
	Cases []struct {
		ID             string `json:"id"`
		SourceFile     string `json:"source_file"`
		SourceLine     int    `json:"source_line"`
		Font           string `json:"font"`
		LocalFontPath  string `json:"local_font_path"`
		Active         bool   `json:"upstream_active"`
		Status         string `json:"recovery_status"`
		ExpectedFields string `json:"expected_fields"`
	} `json:"cases"`
}

func readAOTSHashedFile(path, hash string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != hash {
		return nil, fmt.Errorf("%s: SHA-256 %s does not match manifest %s", path, got, hash)
	}
	return data, nil
}

func loadAOTSGSUBCase(id string) (aotsGSUBCase, error) {
	root := filepath.Join("..", "..")
	dir := filepath.Join(root, "testdata", "harfbuzz", "aots")
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return aotsGSUBCase{}, err
	}
	var manifest aotsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return aotsGSUBCase{}, err
	}
	if manifest.SchemaVersion != 1 || manifest.Revision != aotsRevision {
		return aotsGSUBCase{}, fmt.Errorf("unsupported AOTS manifest schema/revision: %d/%s", manifest.SchemaVersion, manifest.Revision)
	}
	for _, entry := range manifest.Cases {
		if entry.ID != id {
			continue
		}
		if !entry.Active || entry.Status != "candidate" || entry.LocalFontPath == "" {
			return aotsGSUBCase{}, fmt.Errorf("%s: not an active local candidate (%s)", id, entry.Status)
		}
		if entry.ExpectedFields != "glyph_ids" {
			return aotsGSUBCase{}, fmt.Errorf("%s: unsupported expected fields %q", id, entry.ExpectedFields)
		}
		if id != fmt.Sprintf("%s:%d", filepath.Base(entry.SourceFile), entry.SourceLine) ||
			entry.SourceFile != "tests/"+filepath.Base(entry.SourceFile) ||
			entry.LocalFontPath != "testdata/fonttools/"+entry.Font ||
			entry.Font != filepath.Base(entry.Font) {
			return aotsGSUBCase{}, fmt.Errorf("%s: inconsistent source/font identity", id)
		}
		var sourceHash, fontHash string
		for _, source := range manifest.Sources {
			if source.LocalPath == entry.SourceFile {
				sourceHash = source.SHA256
			}
		}
		for _, font := range manifest.Fonts {
			if font.LocalPath == entry.LocalFontPath {
				fontHash = font.SHA256
			}
		}
		data, err := readAOTSHashedFile(filepath.Join(dir, filepath.FromSlash(entry.SourceFile)), sourceHash)
		if err != nil {
			return aotsGSUBCase{}, err
		}
		lines := strings.Split(string(data), "\n")
		if entry.SourceLine < 2 || entry.SourceLine > len(lines) {
			return aotsGSUBCase{}, fmt.Errorf("%s: invalid source line", id)
		}
		// The selected files use exactly one directive before their records.
		if strings.TrimSpace(lines[0]) != "@shaper=ot" {
			return aotsGSUBCase{}, fmt.Errorf("%s: expected @shaper=ot", id)
		}
		for _, line := range lines[1:entry.SourceLine] {
			if strings.HasPrefix(strings.TrimSpace(line), "@") {
				return aotsGSUBCase{}, fmt.Errorf("%s: unsupported additional directive", id)
			}
		}
		c, err := parseAOTSGSUBRecord(lines[entry.SourceLine-1])
		if err != nil {
			return aotsGSUBCase{}, fmt.Errorf("%s: %w", id, err)
		}
		if c.fontPath != "../fonts/"+entry.Font {
			return aotsGSUBCase{}, fmt.Errorf("%s: source font differs from manifest", id)
		}
		c.id, c.fontPath = id, entry.LocalFontPath
		c.fontData, err = readAOTSHashedFile(filepath.Join(root, filepath.FromSlash(c.fontPath)), fontHash)
		return c, err
	}
	return aotsGSUBCase{}, fmt.Errorf("AOTS case %q not found", id)
}

func parseAOTSGSUBRecord(line string) (aotsGSUBCase, error) {
	var c aotsGSUBCase
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#") {
		return c, fmt.Errorf("upstream-disabled record")
	}
	fields := strings.Split(line, ";")
	if len(fields) != 4 {
		return c, fmt.Errorf("expected font;options;unicodes;glyphs")
	}
	c.fontPath, c.options = fields[0], fields[1]
	required := map[string]bool{
		"--features": false, "--single-par": false, "--no-clusters": false,
		"--no-glyph-names": false, "--no-positions": false,
	}
	for _, option := range strings.Fields(c.options) {
		if option == "--features" {
			return c, fmt.Errorf("missing feature value")
		}
		if value, ok := strings.CutPrefix(option, "--features="); ok {
			if required["--features"] || len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
				return c, fmt.Errorf("unsupported or duplicate feature option %q", option)
			}
			var err error
			c.features, err = parseAOTSFeatures(value[1 : len(value)-1])
			if err != nil {
				return c, err
			}
			required["--features"] = true
			continue
		}
		seen, supported := required[option]
		if !supported || seen {
			return c, fmt.Errorf("unsupported or duplicate option %q", option)
		}
		required[option] = true
	}
	for option, seen := range required {
		if !seen {
			return c, fmt.Errorf("missing required option %q", option)
		}
	}
	for _, cp := range strings.Split(fields[2], ",") {
		if !strings.HasPrefix(cp, "U+") {
			return c, fmt.Errorf("invalid Unicode scalar %q", cp)
		}
		n, err := strconv.ParseUint(cp[2:], 16, 32)
		if err != nil || n > utf8.MaxRune || !utf8.ValidRune(rune(n)) {
			return c, fmt.Errorf("invalid Unicode scalar %q", cp)
		}
		c.input = append(c.input, rune(n))
	}
	for _, feature := range c.features {
		if feature.End > len(c.input) {
			return c, fmt.Errorf("feature range [%d,%d) exceeds input length %d", feature.Start, feature.End, len(c.input))
		}
	}
	output := fields[3]
	if !strings.HasPrefix(output, "[") || !strings.HasSuffix(output, "]") {
		return c, fmt.Errorf("expected a bracketed glyph sequence, got %q", output)
	}
	if output = output[1 : len(output)-1]; output != "" {
		for _, token := range strings.Split(output, "|") {
			n, err := strconv.ParseUint(token, 10, 16)
			if err != nil {
				return c, fmt.Errorf("expected glyph ID only, got %q", token)
			}
			c.want = append(c.want, ot.GlyphIndex(n))
		}
	}
	return c, nil
}

var aotsFeaturePattern = regexp.MustCompile(`^(-?)test(?:\[([0-9]+)\])?(?:=([0-9]+))?$`)

func parseAOTSFeatures(spec string) ([]otshape.FeatureRange, error) {
	var features []otshape.FeatureRange
	for _, item := range strings.Split(spec, ",") {
		m := aotsFeaturePattern.FindStringSubmatch(item)
		if m == nil || (m[1] != "" && m[3] != "") {
			return nil, fmt.Errorf("unsupported AOTS feature %q", item)
		}
		f := otshape.FeatureRange{Feature: ot.T("test"), On: m[1] == "", Arg: 1}
		if !f.On {
			f.Arg = 0
		}
		if m[2] != "" {
			// Bound indexes so the exclusive end cannot overflow on 32-bit Go.
			n, err := strconv.ParseUint(m[2], 10, 31)
			if err != nil || n == 1<<31-1 {
				return nil, fmt.Errorf("unsupported AOTS feature index %q", m[2])
			}
			f.Start, f.End = int(n), int(n)+1
		}
		if m[3] != "" {
			n, err := strconv.ParseUint(m[3], 10, 31)
			if err != nil {
				return nil, fmt.Errorf("unsupported AOTS feature value %q", m[3])
			}
			// HB zero disables; it does not mean our enabled Arg=0 default.
			f.Arg, f.On = int(n), n != 0
		}
		features = append(features, f)
	}
	return features, nil
}

func runAOTSGSUBCase(t *testing.T, id string) {
	t.Helper()
	runAOTSGSUBCaseWithBuffers(t, id, otshape.BufferOptions{})
}

func runAOTSGSUBCaseWithBuffers(t *testing.T, id string, opts otshape.BufferOptions) {
	t.Helper()
	c, err := loadAOTSGSUBCase(id)
	if err != nil {
		t.Fatalf("%s: load upstream case: %v", id, err)
	}
	font, err := ot.Parse(c.fontData, ot.IsTestfont)
	if err != nil {
		t.Fatalf("%s: parse %s: %v", id, c.fontPath, err)
	}
	// AOTS generator context, independently checked against the recorded output.
	// See testdata/harfbuzz/aots/ADAPTER.md; no host locale or script guessing.
	f := fixture{Context: fixtureContext{Script: "Latn", Language: "en", Dir: "ltr"}}
	params, err := f.params(font)
	if err != nil {
		t.Fatal(err)
	}
	params.Features = c.features
	// Zero options here select an explicit whole-input window, not API defaults.
	if opts == (otshape.BufferOptions{}) {
		opts = otshape.BufferOptions{HighWatermark: len(c.input) + 1, LowWatermark: 1, MaxBuffer: len(c.input) + 2}
	}
	sink := &glyphCollector{}
	err = otshape.NewShaper(otcore.New()).Shape(params, strings.NewReader(string(c.input)), sink, opts)
	got := make([]ot.GlyphIndex, len(sink.glyphs))
	for i, glyph := range sink.glyphs {
		got[i] = glyph.GID
	}
	if err != nil || !slices.Equal(got, c.want) {
		t.Fatalf("upstream=%s@%s font=%s\noptions=%s\nfeatures=%+v\ncontext=Latn/en/ltr/core, buffers=%+v input=%U\nerror=%v\ngot=%v\nwant=%v",
			c.id, aotsRevision, c.fontPath, c.options, c.features, opts, c.input, err, got, c.want)
	}
}

func TestAOTSGSUBBaselines(t *testing.T) {
	for _, id := range []string{
		"classdef2.tests:2",
	} {
		t.Run(id, func(t *testing.T) { runAOTSGSUBCase(t, id) })
	}
}

func TestAOTSAdapterRejectsUnsupportedRecords(t *testing.T) {
	const record = `../fonts/example.otf;--features="test" --single-par --no-clusters --no-glyph-names --no-positions;U+0000,U+005A;[0|90]`
	// NUL must survive conversion to Go text; glyph zero is a real expectation.
	c, err := parseAOTSGSUBRecord(record)
	if err != nil || !slices.Equal(c.input, []rune{0, 'Z'}) || !slices.Equal(c.want, []ot.GlyphIndex{0, 90}) {
		t.Fatalf("lost scalar or glyph data: %+v, %v", c, err)
	}
	for name, line := range map[string]string{
		"disabled":       "#" + record,
		"unknown_option": strings.Replace(record, "--single-par", "--single-par --normalize-glyphs", 1),
		"span_feature":   strings.Replace(record, `--features="test"`, `--features="test[0:2]=2"`, 1),
		"range_bounds":   strings.Replace(record, `--features="test"`, `--features="test[2]=2"`, 1),
		"positions":      strings.Replace(record, "[0|90]", "[0|90@10,0]", 1),
		"clusters":       strings.Replace(record, "[0|90]", "[0=0|90=1]", 1),
		"no_oracle":      strings.Replace(record, "[0|90]", "*", 1),
		"surrogate":      strings.Replace(record, "U+0000", "U+D800", 1),
		"out_of_unicode": strings.Replace(record, "U+0000", "U+110000", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseAOTSGSUBRecord(line); err == nil {
				t.Fatalf("unsupported record silently accepted: %s", line)
			}
		})
	}
}

func TestAOTSAdapterRejectsUnavailableCases(t *testing.T) {
	for _, tc := range []struct{ id, reason string }{
		{"missing.tests:2", "not found"},
		{"gpos3_lookupflag.tests:2", "not an active local candidate"},
		{"gsub_context1_lookupflag.tests:3", "not an active local candidate"},
		{"gpos4_simple.tests:2", "unsupported expected fields"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if _, err := loadAOTSGSUBCase(tc.id); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("error=%v, want %q", err, tc.reason)
			}
		})
	}
}

// These are adapter-contract assertions, not additional upstream shaping cases.
func TestAOTSFeatureMapping(t *testing.T) {
	feature := ot.T("test")
	for _, tc := range []struct {
		spec string
		want []otshape.FeatureRange
	}{
		{"test", []otshape.FeatureRange{{Feature: feature, On: true, Arg: 1}}},
		{"test[0],-test[1],test[2]=2,test[3]=3", []otshape.FeatureRange{
			{Feature: feature, On: true, Arg: 1, Start: 0, End: 1},
			{Feature: feature, On: false, Start: 1, End: 2},
			{Feature: feature, On: true, Arg: 2, Start: 2, End: 3},
			{Feature: feature, On: true, Arg: 3, Start: 3, End: 4},
		}},
		{"test[0]=0,test[0]=4", []otshape.FeatureRange{
			{Feature: feature, On: false, Start: 0, End: 1},
			{Feature: feature, On: true, Arg: 4, Start: 0, End: 1},
		}},
	} {
		t.Run(tc.spec, func(t *testing.T) {
			got, err := parseAOTSFeatures(tc.spec)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("features=%+v, error=%v; want %+v", got, err, tc.want)
			}
		})
	}
	for _, spec := range []string{"", "test,", "liga", "test[-1]", "test[0:2]", "test[]", "test=", "test=-1", "-test=2", "test[2147483647]", "test=2147483648"} {
		t.Run("reject_"+spec, func(t *testing.T) {
			if _, err := parseAOTSFeatures(spec); err == nil {
				t.Fatalf("unsupported feature accepted: %q", spec)
			}
		})
	}
	const options = `;--features="test[0],test[1]=2" --single-par --no-clusters --no-glyph-names --no-positions;`
	// First and last indexes refer to scalars, even for multibyte UTF-8 text.
	c, err := parseAOTSGSUBRecord("../fonts/example.otf" + options + "U+00E9,U+05D0;[1|2]")
	if err != nil || len(c.features) != 2 || c.features[1].Start != 1 || c.features[1].End != 2 {
		t.Fatalf("scalar range mapping: %+v, %v", c, err)
	}
}
