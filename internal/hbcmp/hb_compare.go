package hbcmp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otarabic"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// hbShapedGlyph is one positioned glyph from an hb-shape JSON result.
type hbShapedGlyph struct {
	G  int    `json:"g"`  // glyph index
	Cl uint32 `json:"cl"` // cluster index
	DX int32  `json:"dx"` // x offset
	DY int32  `json:"dy"` // y offset
	AX int32  `json:"ax"` // x advance
	AY int32  `json:"ay"` // y advance
}

type fixtureContext struct {
	Font          string   `json:"font"`
	Script        string   `json:"script"`
	Language      string   `json:"language"`
	Dir           string   `json:"dir"`
	Normalization string   `json:"normalization,omitempty"`
	Features      []string `json:"features,omitempty"`
	TestFont      bool     `json:"testfont,omitempty"`
}

type fixture struct {
	SchemaVersion int             `json:"schema_version,omitempty"`
	Context       fixtureContext  `json:"context"`
	Input         []uint32        `json:"input"` // Unicode codepoints
	Output        []hbShapedGlyph `json:"output"`
}

func (f fixture) validate() error {
	if f.Context.Font == "" {
		return fmt.Errorf("fixture: context.font is required")
	}
	if f.Context.Script == "" {
		return fmt.Errorf("fixture: context.script is required")
	}
	if f.Context.Language == "" {
		return fmt.Errorf("fixture: context.language is required")
	}
	if f.Context.Dir == "" {
		return fmt.Errorf("fixture: context.dir is required")
	}
	if len(f.Input) == 0 {
		return fmt.Errorf("fixture: input must not be empty")
	}
	return nil
}

func (f fixture) runes() ([]rune, error) {
	runes := make([]rune, len(f.Input))
	for i, cp := range f.Input {
		if cp > 0x10FFFF || (cp >= 0xD800 && cp <= 0xDFFF) {
			return nil, fmt.Errorf("fixture: input[%d]=%d is not a valid Unicode scalar", i, cp)
		}
		runes[i] = rune(cp)
	}
	return runes, nil
}

func (f fixture) params(font *ot.Font) (otshape.Params, error) {
	script, err := parseScript(f.Context.Script)
	if err != nil {
		return otshape.Params{}, err
	}
	dir, err := parseDirection(f.Context.Dir)
	if err != nil {
		return otshape.Params{}, err
	}
	features, err := parseFeatures(f.Context.Features)
	if err != nil {
		return otshape.Params{}, err
	}
	return otshape.Params{
		Font:      font,
		Direction: dir,
		Script:    script,
		Language:  language.Make(f.Context.Language),
		Features:  features,
	}, nil
}

func glyphRecord2hbShapedGlyph(glyph otshape.GlyphRecord) hbShapedGlyph {
	return hbShapedGlyph{
		G:  int(glyph.GID),
		Cl: glyph.Cluster,
		DX: glyph.Pos.XOffset,
		DY: glyph.Pos.YOffset,
		AX: glyph.Pos.XAdvance,
		AY: glyph.Pos.YAdvance,
	}
}

func comparePositionedGlyphs(got, want []hbShapedGlyph) error {
	if len(got) != len(want) {
		return fmt.Errorf("unequal number of glyphs: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Errorf("glyph[%d] mismatch: got=%+v want=%+v", i, got[i], want[i])
		}
	}
	return nil
}

func loadFixture(path string) (fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fixture{}, err
	}
	var f fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return fixture{}, err
	}
	if err := f.validate(); err != nil {
		return fixture{}, err
	}
	return f, nil
}

func loadFixtures(dir string) ([]fixture, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".json") {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	sort.Strings(paths)
	out := make([]fixture, 0, len(paths))
	for _, p := range paths {
		f, err := loadFixture(p)
		if err != nil {
			return nil, fmt.Errorf("load fixture %s: %w", p, err)
		}
		out = append(out, f)
	}
	return out, nil
}

func shapeFixture(f fixture) ([]hbShapedGlyph, error) {
	font, err := loadFixtureFont(f.Context.Font, f.Context.TestFont)
	if err != nil {
		return nil, err
	}
	params, err := f.params(font)
	if err != nil {
		return nil, err
	}
	runes, err := f.runes()
	if err != nil {
		return nil, err
	}
	shaper := otshape.NewShaper(
		otcore.New(),
		otarabic.New(),
		othebrew.New(),
	)
	sink := &glyphCollector{}
	if err := shaper.Shape(
		params,
		strings.NewReader(string(runes)),
		sink,
		otshape.BufferOptions{FlushBoundary: otshape.FlushOnRunBoundary},
	); err != nil {
		return nil, err
	}
	got := make([]hbShapedGlyph, len(sink.glyphs))
	for i, g := range sink.glyphs {
		got[i] = glyphRecord2hbShapedGlyph(g)
	}
	return got, nil
}

type glyphCollector struct {
	glyphs []otshape.GlyphRecord
}

func (c *glyphCollector) WriteGlyph(g otshape.GlyphRecord) error {
	c.glyphs = append(c.glyphs, g)
	return nil
}

func parseScript(s string) (script language.Script, err error) {
	s = strings.TrimSpace(s)
	if len(s) != 4 {
		return script, fmt.Errorf("invalid script %q", s)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid script %q", s)
		}
	}()
	script = language.MustParseScript(s)
	return script, nil
}

func parseDirection(s string) (bidi.Direction, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ltr", "left-to-right":
		return bidi.LeftToRight, nil
	case "rtl", "right-to-left":
		return bidi.RightToLeft, nil
	default:
		return bidi.LeftToRight, fmt.Errorf("invalid direction %q (expected ltr|rtl)", s)
	}
}

func parseFeatures(spec []string) ([]otshape.FeatureRange, error) {
	out := make([]otshape.FeatureRange, 0, len(spec))
	for _, item := range spec {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		on := true
		if strings.HasPrefix(item, "+") {
			item = strings.TrimPrefix(item, "+")
		} else if strings.HasPrefix(item, "-") {
			item = strings.TrimPrefix(item, "-")
			on = false
		}
		tagPart, value, hasEq := strings.Cut(item, "=")
		tagPart = strings.TrimSpace(tagPart)
		if len(tagPart) != 4 {
			return nil, fmt.Errorf("invalid feature tag %q", tagPart)
		}
		arg := 1
		if hasEq {
			value = strings.TrimSpace(value)
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid feature value %q in %q", value, item)
			}
			arg = n
			on = n != 0
		}
		out = append(out, otshape.FeatureRange{
			Feature: ot.T(tagPart),
			Arg:     arg,
			On:      on,
		})
	}
	return out, nil
}

func loadFixtureFont(name string, isTestFont bool) (*ot.Font, error) {
	var candidates []string
	if filepath.IsAbs(name) {
		candidates = append(candidates, name)
	} else {
		candidates = append(candidates,
			filepath.Join("..", "..", "testdata", name),
			filepath.Join("testdata", name),
			name,
		)
	}
	var (
		raw  []byte
		path string
	)
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			raw = b
			path = p
			break
		}
	}
	if raw == nil {
		return nil, fmt.Errorf("fixture font not found: %q", name)
	}
	var (
		font *ot.Font
		err  error
	)
	if isTestFont {
		font, err = ot.Parse(raw, ot.IsTestfont)
	} else {
		font, err = ot.Parse(raw)
	}
	if err != nil {
		return nil, fmt.Errorf("parse font %s: %w", path, err)
	}
	return font, nil
}
