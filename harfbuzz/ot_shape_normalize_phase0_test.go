package harfbuzz

import (
	"strings"
	"testing"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
)

type phase0ShapeCase struct {
	name      string
	fontFile  string
	direction Direction
	script    language.Script
	lang      language.Language
	text      []rune
}

func shapePhase0Case(t *testing.T, tc phase0ShapeCase, opt formatOpts) (serialized string, notFound int) {
	t.Helper()

	ft := openFontFileTT(t, tc.fontFile)
	face := font.NewFace(ft)
	hbFont := NewFont(face)

	buf := NewBuffer()
	buf.Props.Direction = tc.direction
	buf.Props.Script = tc.script
	buf.Props.Language = tc.lang
	buf.AddRunes(tc.text, 0, -1)
	buf.Shape(hbFont, nil)

	if err := buf.verifyMonotone(); err != nil {
		t.Fatalf("%s: monotone-cluster check failed: %v", tc.name, err)
	}

	for _, info := range buf.Info {
		if info.Glyph == 0 {
			notFound++
		}
	}

	return buf.serialize(hbFont, opt), notFound
}

func TestPhase0NormalizationCanonicalEquivalence(t *testing.T) {
	opt := formatOpts{
		hideGlyphNames: true,
		hideClusters:   true,
	}

	tests := []struct {
		name       string
		fontFile   string
		direction  Direction
		script     language.Script
		lang       language.Language
		composed   []rune
		decomposed []rune
	}{
		{
			name:       "latin_A_ring",
			fontFile:   "common/DejaVuSans.ttf",
			direction:  LeftToRight,
			script:     language.Latin,
			lang:       language.NewLanguage("en"),
			composed:   []rune{0x00C5},
			decomposed: []rune{0x0041, 0x030A},
		},
		{
			name:       "cyrillic_short_i",
			fontFile:   "common/DejaVuSans.ttf",
			direction:  LeftToRight,
			script:     language.Cyrillic,
			lang:       language.NewLanguage("ru"),
			composed:   []rune{0x0419},
			decomposed: []rune{0x0418, 0x0306},
		},
		{
			name:       "arabic_alef_hamza_above",
			fontFile:   "common/NotoSansArabic.ttf",
			direction:  RightToLeft,
			script:     language.Arabic,
			lang:       language.NewLanguage("ar"),
			composed:   []rune{0x0623},
			decomposed: []rune{0x0627, 0x0654},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			left, leftNotFound := shapePhase0Case(t, phase0ShapeCase{
				name:      tc.name + "_composed",
				fontFile:  tc.fontFile,
				direction: tc.direction,
				script:    tc.script,
				lang:      tc.lang,
				text:      tc.composed,
			}, opt)
			right, rightNotFound := shapePhase0Case(t, phase0ShapeCase{
				name:      tc.name + "_decomposed",
				fontFile:  tc.fontFile,
				direction: tc.direction,
				script:    tc.script,
				lang:      tc.lang,
				text:      tc.decomposed,
			}, opt)

			if leftNotFound != 0 || rightNotFound != 0 {
				t.Fatalf("%s: unexpected .notdef glyphs (composed=%d decomposed=%d)", tc.name, leftNotFound, rightNotFound)
			}
			if left != right {
				t.Fatalf("%s: composed/decomposed mismatch\ncomposed:   %s\ndecomposed: %s", tc.name, left, right)
			}
		})
	}
}

func TestPhase0NormalizationReferenceBaselines(t *testing.T) {
	refCases := []struct {
		name string
		dir  string
		line string
	}{
		{
			name: "hebrew_diacritics",
			dir:  "harfbuzz_reference/in-house/tests",
			line: "../fonts/b895f8ff06493cc893ec44de380690ca0074edfa.ttf;;U+05D4,U+05B2,U+05D1,U+05B5,U+05DC;[lamed=4+901|tsere=2@512,0+0|bet=2+967|hatafpatah=0@600,0+0|he=0+1071]",
		},
		{
			name: "arabic_normalization",
			dir:  "harfbuzz_reference/in-house/tests",
			line: "../fonts/872d2955d326bd6676a06f66b8238ebbaabc212f.ttf;;U+0627,U+0653;[uni0622=0+217]",
		},
		{
			name: "hyphen_u2010",
			dir:  "harfbuzz_reference/in-house/tests",
			line: "../fonts/1c04a16f32a39c26c851b7fc014d2e8d298ba2b8.ttf;;U+2010;[gid1=0+739]",
		},
		{
			name: "hyphen_u2011_fallback",
			dir:  "harfbuzz_reference/in-house/tests",
			line: "../fonts/1c04a16f32a39c26c851b7fc014d2e8d298ba2b8.ttf;;U+2011;[gid1=0+739]",
		},
		{
			name: "space_u202f_fallback",
			dir:  "harfbuzz_reference/in-house/tests",
			line: "../fonts/1c2c3fc37b2d4c3cb2ef726c6cdaaabd4b7f3eb9.ttf;--font-funcs=ot;U+202F;[gid1=0+280]",
		},
	}

	for _, tc := range refCases {
		t.Run(tc.name, func(t *testing.T) {
			td := newTestData(t, tc.dir, tc.line)
			got, err := td.input.shape(t, true)
			if err != nil {
				t.Fatalf("%s: shaping failed: %v", tc.name, err)
			}
			got = strings.TrimSpace(got)
			if got != td.expected {
				t.Fatalf("%s: output mismatch\nexpected: %s\nactual:   %s", tc.name, td.expected, got)
			}
		})
	}
}
