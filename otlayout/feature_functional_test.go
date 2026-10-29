package otlayout

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type testFeature struct {
	tag ot.Tag
	typ LayoutTagType
}

func (f testFeature) Tag() ot.Tag         { return f.tag }
func (f testFeature) Type() LayoutTagType { return f.typ }
func (f testFeature) LookupCount() int    { return 0 }
func (f testFeature) LookupIndex(int) int { return 0 }

func loadTestFont(t *testing.T, filename string) *ot.Font {
	t.Helper()
	path := filepath.Join("..", "testdata", "fonttools", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read font %s: %v", path, err)
	}
	otf, err := ot.Parse(data, ot.IsTestfont)
	if err != nil {
		t.Fatalf("parse font %s: %v", path, err)
	}
	return otf
}

func applyGSUBLookup(t *testing.T, otf *ot.Font, lookupIndex int, input []ot.GlyphIndex, pos, alt int) (GlyphBuffer, bool) {
	t.Helper()
	if otf.Layout.GSub == nil {
		t.Fatalf("font has no GSUB table")
	}
	var clookup *ot.LookupTable
	if graph := otf.Layout.GSub.LookupGraph(); graph != nil {
		clookup = graph.Lookup(lookupIndex)
	}
	feat := testFeature{tag: ot.T("test"), typ: GSubFeatureType}
	buf := append(GlyphBuffer(nil), input...)
	st := NewBufferState(buf, NewPosBuffer(len(buf)))
	st.Index = pos
	env, err := newApplyEnv(otf)
	if err != nil {
		t.Fatal(err)
	}
	_, ok, _, err := applyLookupConcrete(clookup, otf.Layout.GSub.LookupGraph(), feat, st, alt, otf.Layout.GDef, env)
	if err != nil {
		t.Fatal(err)
	}
	out := st.Glyphs
	return out, ok
}

func TestGSUBAlternateSimple(t *testing.T) {
	otf := loadTestFont(t, "gsub3_1_simple_f1.otf")

	cases := []struct {
		name     string
		input    []ot.GlyphIndex
		alt      int
		expected []ot.GlyphIndex
		applied  bool
	}{
		{
			name:     "alt0",
			input:    []ot.GlyphIndex{18},
			alt:      0,
			expected: []ot.GlyphIndex{20},
			applied:  true,
		},
		{
			name:     "alt1",
			input:    []ot.GlyphIndex{18},
			alt:      1,
			expected: []ot.GlyphIndex{20},
			applied:  true,
		},
		{
			name:     "alt3",
			input:    []ot.GlyphIndex{18},
			alt:      3,
			expected: []ot.GlyphIndex{22},
			applied:  true,
		},
		{
			name:     "not-covered",
			input:    []ot.GlyphIndex{19},
			alt:      0,
			expected: []ot.GlyphIndex{19},
			applied:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, applied := applyGSUBLookup(t, otf, 0, tc.input, 0, tc.alt)
			if applied != tc.applied {
				t.Fatalf("expected applied=%v, got %v", tc.applied, applied)
			}
			if len(out) != len(tc.expected) {
				t.Fatalf("expected %d glyphs, got %d", len(tc.expected), len(out))
			}
			for i, gid := range tc.expected {
				if out[i] != gid {
					t.Fatalf("glyph[%d]: expected %d, got %d", i, gid, out[i])
				}
			}
		})
	}
}

func TestGSUBContextSubstFmt1IgnoreMarks(t *testing.T) {
	otf := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")

	cases := []struct {
		name     string
		input    []ot.GlyphIndex
		pos      int
		expected []ot.GlyphIndex
		applied  bool
	}{
		{
			name:     "match",
			input:    []ot.GlyphIndex{20, 21, 22},
			pos:      0,
			expected: []ot.GlyphIndex{60, 61, 62},
			applied:  true,
		},
		{
			name:     "mismatch-last",
			input:    []ot.GlyphIndex{20, 21, 23},
			pos:      0,
			expected: []ot.GlyphIndex{20, 21, 23},
			applied:  false,
		},
		{
			name:     "match-offset",
			input:    []ot.GlyphIndex{10, 20, 21, 22},
			pos:      1,
			expected: []ot.GlyphIndex{10, 60, 61, 62},
			applied:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, applied := applyGSUBLookup(t, otf, 4, tc.input, tc.pos, 0)
			if applied != tc.applied {
				t.Fatalf("expected applied=%v, got %v", tc.applied, applied)
			}
			if len(out) != len(tc.expected) {
				t.Fatalf("expected %d glyphs, got %d", len(tc.expected), len(out))
			}
			for i, gid := range tc.expected {
				if out[i] != gid {
					t.Fatalf("glyph[%d]: expected %d, got %d", i, gid, out[i])
				}
			}
		})
	}
}

// The paired TTX gives GID 18 the alternatives [20, 21, 22]. These exercise
// the exported entry point, including values that previously wrapped in masks.
func TestApplyFeatureAlternateValues(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	feat := errorTestFeature{GSubFeatureType, []int{0}}
	for _, tc := range []struct {
		value int
		want  ot.GlyphIndex
	}{
		{0, 20}, {1, 20}, {2, 21}, {3, 22}, {4, 18}, {256, 18}, {257, 18}, {math.MaxInt, 18},
	} {
		t.Run(fmt.Sprint(tc.value), func(t *testing.T) {
			st := NewBufferState(GlyphBuffer{18}, nil)
			_, applied, err := ApplyFeature(font, feat, st, tc.value)
			if err != nil || applied != (tc.want != 18) || st.Glyphs[0] != tc.want {
				t.Fatalf("value %d: glyphs=%v applied=%v err=%v; want %d", tc.value, st.Glyphs, applied, err, tc.want)
			}
		})
	}
	for _, value := range []int{-1, -2, math.MinInt} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			st := NewBufferState(nil, nil)
			st.SetRandomSeed(42, 19)
			oracle := rand.New(rand.NewPCG(42, 19))
			seen := map[ot.GlyphIndex]bool{}
			for i := 0; i < 32; i++ {
				st.Glyphs, st.Index = GlyphBuffer{18}, 0
				want := []ot.GlyphIndex{20, 21, 22}[oracle.IntN(3)]
				_, applied, err := ApplyFeature(font, feat, st, value)
				if err != nil || !applied || st.Glyphs[0] != want {
					t.Fatalf("value %d draw %d: got %v, %v, %v; want %d", value, i, st.Glyphs, applied, err, want)
				}
				seen[want] = true
			}
			if len(seen) != 3 {
				t.Fatal("fixed seed must exercise all three alternatives")
			}
		})
	}
}
