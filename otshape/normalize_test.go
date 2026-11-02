package otshape

import (
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestCanonicalNormalizationSources(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		mode              NormalizationMode
		sources           []uint32
	}{
		{"reordered", "a\u0301\u0323X", "\u1ea1\u0301X", NormalizationComposed, []uint32{10, 11, 13}},
		{"hidden_accent", "\u00e1\u0323X", "\u1ea1\u0301X", NormalizationComposed, []uint32{10, 10, 12}},
		{"unblocked", "a\u0316\u0301X", "\u00e1\u0316X", NormalizationComposed, []uint32{10, 11, 13}},
		{"blocked_equal_class", "a\u0305\u0301X", "a\u0305\u0301X", NormalizationComposed, []uint32{10, 11, 12, 13}},
		{"stable_equal_class", "a\u0301\u0300", "a\u0301\u0300", NormalizationDecomposed, []uint32{10, 11, 12}},
		{"leading_marks", "\u0301\u0323X", "\u0323\u0301X", NormalizationComposed, []uint32{11, 10, 12}},
		{"cgj_barrier", "a\u0301\u034f\u0323X", "\u00e1\u034f\u0323X", NormalizationComposed, []uint32{10, 12, 13, 14}},
		{"none", "a\u0301\u0323X", "a\u0301\u0323X", NormalizationNone, []uint32{10, 11, 12, 13}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []rune(tc.input)
			sources := make([]uint32, len(input))
			for i := range sources {
				sources[i] = uint32(10 + i)
			}
			ws := newShapeWorkspace(16)
			r, groups, gotSources := ws.normalize(input, sources, nil, SelectionContext{}, clusterPolicyEngine{tc.mode}, nil)
			if string(r) != tc.want || !slices.Equal(gotSources, tc.sources) {
				t.Fatalf("got %U sources=%v, want %U sources=%v", r, gotSources, []rune(tc.want), tc.sources)
			}
			for i, cp := range r {
				want := uint32(10)
				if cp == 'X' {
					want = uint32(10 + len(input) - 1)
				}
				if groups[i] != want {
					t.Fatalf("groups=%v at %d: want %d", groups, i, want)
				}
			}
		})
	}
}

type normalizationCMap map[rune]ot.GlyphIndex

func (m normalizationCMap) Lookup(r rune) (ot.GlyphIndex, error) { return m[r], nil }
func (m normalizationCMap) ReverseLookup(ot.GlyphIndex) rune     { return 0 }

func TestNormalizationFontCoverage(t *testing.T) {
	for _, tc := range []struct {
		name, available, input, want string
		mode                         NormalizationMode
	}{
		{"missing_composite_nfc", "e\u0301X", "\u00e9X", "e\u0301X", NormalizationComposed},
		{"missing_composite_nfd", "e\u0301X", "e\u0301X", "e\u0301X", NormalizationComposed},
		{"missing_reordered_composite", "a\u00e1\u0301\u0323X", "\u00e1\u0323X", "\u00e1\u0323X", NormalizationComposed},
		{"partial_decomposition", "A\u00c5\u0301X", "\u01faX", "\u00c5\u0301X", NormalizationComposed},
		{"retain_supported_component", "A\u00c5\u0301X", "\u00c5\u0301X", "\u00c5\u0301X", NormalizationComposed},
		{"missing_mark_decomposed_mode", "\u00e9", "\u00e9", "\u00e9", NormalizationDecomposed},
		{"standalone_exclusion", "\u212b\u00c5A\u030a", "\u212b", "\u212b", NormalizationComposed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmap := normalizationCMap{}
			for i, cp := range []rune(tc.available) {
				cmap[cp] = ot.GlyphIndex(i + 1)
			}
			font := &ot.Font{CMap: &ot.CMapTable{GlyphIndexMap: cmap}}
			r, _ := normalizeRuneStream([]rune(tc.input), nil, font, SelectionContext{}, clusterPolicyEngine{tc.mode}, nil)
			if string(r) != tc.want {
				t.Fatalf("got %U, want %U", r, []rune(tc.want))
			}
		})
	}
}

type unavailableComposeHook struct{ clusterPolicyEngine }

func (unavailableComposeHook) Compose(NormalizeContext, rune, rune) (rune, bool) {
	return '\u00e9', true
}

func TestNormalizationRejectsUnavailableHookOutput(t *testing.T) {
	font := &ot.Font{CMap: &ot.CMapTable{GlyphIndexMap: normalizationCMap{'e': 1, '\u0301': 2}}}
	r, _ := normalizeRuneStream([]rune("e\u0301"), nil, font, SelectionContext{}, unavailableComposeHook{}, nil)
	if string(r) != "e\u0301" {
		t.Fatalf("unmapped hook result accepted: %U", r)
	}
}

func TestNormalizationPreservesOrderedHebrewMarks(t *testing.T) {
	input := []rune("\u05e9\u05c1\u05b8")
	r, sources := normalizeRuneStream(input, []uint32{10, 11, 12}, nil,
		SelectionContext{ScriptTag: ot.T("hebr")}, clusterPolicyEngine{NormalizationComposed}, nil)
	if !slices.Equal(r, input) || !slices.Equal(sources, []uint32{10, 11, 12}) {
		t.Fatalf("Hebrew ordering changed: %U sources=%v", r, sources)
	}
}

func TestCanonicalOrderingDoesNotInsertStreamSafeCharacters(t *testing.T) {
	// The buffer policy has not authorized silent CGJ insertion or truncation.
	input := "x" + strings.Repeat("\u0301\u0323", 40)
	r, _ := normalizeRuneStream([]rune(input), nil, nil, SelectionContext{}, clusterPolicyEngine{NormalizationDecomposed}, nil)
	want := "x" + strings.Repeat("\u0323", 40) + strings.Repeat("\u0301", 40)
	if string(r) != want {
		t.Fatalf("got %U, want %U", r, []rune(want))
	}
}

func TestHebrewNormalizationSourcesAndBarriers(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		sources           []uint32
	}{
		{"shin_dot_first", "\u05e9\u05b8\u05c1", "\u05e9\u05c1\u05b8", []uint32{10, 12, 11}},
		{"sin_dot_first", "\u05e9\u05b8\u05c2", "\u05e9\u05c2\u05b8", []uint32{10, 12, 11}},
		{"equal_class_stable", "\u05d0\u05c7\u05b8", "\u05d0\u05c7\u05b8", []uint32{10, 11, 12}},
		{"leading_marks", "\u05b8\u05c1", "\u05c1\u05b8", []uint32{11, 10}},
		{"cgj_barrier", "\u05e9\u05b8\u034f\u05c1", "\u05e9\u05b8\u034f\u05c1", []uint32{10, 11, 12, 13}},
		{"starter_barrier", "\u05e9\u05b8\u05d0\u05c1", "\u05e9\u05b8\u05d0\u05c1", []uint32{10, 11, 12, 13}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []rune(tc.input)
			sources := make([]uint32, len(input))
			for i := range sources {
				sources[i] = uint32(10 + i)
			}
			ws := newShapeWorkspace(16)
			r, groups, gotSources := ws.normalize(input, sources, nil, SelectionContext{ScriptTag: ot.T("hebr")}, clusterPolicyEngine{NormalizationComposed}, nil)
			if string(r) != tc.want || !slices.Equal(gotSources, tc.sources) {
				t.Fatalf("got %U sources=%v, want %U sources=%v", r, gotSources, []rune(tc.want), tc.sources)
			}
			for i, group := range groups {
				want := uint32(10)
				if tc.name == "starter_barrier" && i >= 2 {
					want = 12
				}
				if group != want {
					t.Fatalf("groups=%v at %d, want %d", groups, i, want)
				}
			}
		})
	}
}

type normalizationReorderProbe struct {
	clusterPolicyEngine
	spans     []string
	lateCalls int
}

func (e *normalizationReorderProbe) ReorderNormalizationMarks(runes []rune, _ []uint32) {
	e.spans = append(e.spans, string(runes))
}
func (e *normalizationReorderProbe) ReorderMarks(RunContext, int, int) { e.lateCalls++ }

func TestNormalizationReorderHookTiming(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	for _, mode := range []NormalizationMode{NormalizationComposed, NormalizationNone} {
		e := &normalizationReorderProbe{clusterPolicyEngine: clusterPolicyEngine{mode}}
		pl, err := compile(planRequest{Font: font, Engine: e})
		if err != nil {
			t.Fatal(err)
		}
		r, c := normalizeRuneStream([]rune("x\u0301\u0323X\u0300\u0301"), nil, font, SelectionContext{}, e, pl)
		want := []string{"\u0323\u0301", "\u0300\u0301"}
		if mode == NormalizationNone {
			want = nil
		}
		if !slices.Equal(e.spans, want) {
			t.Fatalf("mode=%v spans=%q, want %q", mode, e.spans, want)
		}
		run := mapRunesToRunBufferInto(newRunBuffer(len(r)), r, c, nil, font)
		if err := shapeMappedRun(run, e, pl); err != nil {
			t.Fatal(err)
		}
		if e.lateCalls != 0 {
			t.Fatal("normalization reorder repeated after mapping")
		}
	}
}
