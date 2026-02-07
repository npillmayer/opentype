package harfbuzz

import (
	"strings"
	"testing"

	"github.com/go-text/typesetting/language"
)

func captureComplexShapersForParity(t *testing.T) (hebrew ShapingEngine, arabic ShapingEngine) {
	t.Helper()

	hebrew = resolveShaperForContext(SelectionContext{
		Script:    language.Hebrew,
		Direction: RightToLeft,
	})
	if hebrew == nil || hebrew.Name() == "default" {
		t.Fatal("hebrew complex shaper not registered for parity fixtures")
	}

	arabic = resolveShaperForContext(SelectionContext{
		Script:    language.Arabic,
		Direction: RightToLeft,
	})
	if arabic == nil || arabic.Name() == "default" {
		t.Fatal("arabic complex shaper not registered for parity fixtures")
	}

	return hebrew, arabic
}

func resetDefaultRegistryToBuiltins(t *testing.T) {
	t.Helper()

	hebrew, arabic := captureComplexShapersForParity(t)

	restoreBuiltins := func() {
		ClearRegistry()
		for _, shaper := range builtInShapers() {
			if err := RegisterShaper(shaper); err != nil {
				t.Fatalf("failed to register builtin shaper %q: %v", shaper.Name(), err)
			}
		}
		if err := RegisterShaper(hebrew.New()); err != nil {
			t.Fatalf("failed to register parity shaper hebrew: %v", err)
		}
		if err := RegisterShaper(arabic.New()); err != nil {
			t.Fatalf("failed to register parity shaper arabic: %v", err)
		}
	}

	restoreBuiltins()
	t.Cleanup(restoreBuiltins)
}

func TestShaperRefactorParityFixtures(t *testing.T) {
	resetDefaultRegistryToBuiltins(t)

	fixtures := []struct {
		name        string
		scriptClass string
		originDir   string
		line        string
	}{
		{
			name:        "latin_gsub_contextual",
			scriptClass: "latin",
			originDir:   "harfbuzz_reference/text-rendering-tests/tests",
			line:        "../fonts/TestGSUBOne.otf;--font-size=1000 --ned --remove-default-ignorables --font-funcs=ft;U+0061,U+0020,U+0061;[a.alt|space@500,0|a@1000,0]",
		},
		{
			name:        "latin_gpos_kerning",
			scriptClass: "latin",
			originDir:   "harfbuzz_reference/text-rendering-tests/tests",
			line:        "../fonts/TestGPOSOne.ttf;--font-size=1000 --ned --remove-default-ignorables --font-funcs=ft;U+0056,U+0061;[V|a@594,0]",
		},
		{
			name:        "hebrew_marks",
			scriptClass: "hebrew",
			originDir:   "harfbuzz_reference/in-house/tests",
			line:        "../fonts/b895f8ff06493cc893ec44de380690ca0074edfa.ttf;;U+05D4,U+05B2,U+05D1,U+05B5,U+05DC;[lamed=4+901|tsere=2@512,0+0|bet=2+967|hatafpatah=0@600,0+0|he=0+1071]",
		},
		{
			name:        "hebrew_dagesh",
			scriptClass: "hebrew",
			originDir:   "harfbuzz_reference/in-house/tests",
			line:        "../fonts/b895f8ff06493cc893ec44de380690ca0074edfa.ttf;;U+05D1,U+05BC,U+05B7,U+05D1,U+05BC,U+05B9,U+05E7,U+05B6,U+05E8;[resh=8+883|segol=6@618,0+0|kof=6+997|holam=3@422,0+0|betdagesh=3+967|patah=0@505,0+0|betdagesh=0+967]",
		},
		{
			name:        "arabic_normalization_decomposed",
			scriptClass: "arabic",
			originDir:   "harfbuzz_reference/in-house/tests",
			line:        "../fonts/872d2955d326bd6676a06f66b8238ebbaabc212f.ttf;;U+0627,U+0653;[uni0622=0+217]",
		},
		{
			name:        "arabic_normalization_joined",
			scriptClass: "arabic",
			originDir:   "harfbuzz_reference/in-house/tests",
			line:        "../fonts/872d2955d326bd6676a06f66b8238ebbaabc212f.ttf;;U+0628,U+0622;[uni0622.fina=1+327|uni0628.init=0+190]",
		},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.scriptClass+"_"+fixture.name, func(t *testing.T) {
			td := newTestData(t, fixture.originDir, fixture.line)
			got, err := td.input.shape(t, true)
			if err != nil {
				t.Fatalf("shaping failed: %v", err)
			}

			got = strings.TrimSpace(got)
			if got != td.expected {
				t.Fatalf("output mismatch\nexpected: %s\nactual:   %s", td.expected, got)
			}
		})
	}
}
