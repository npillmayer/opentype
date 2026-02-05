package otlayout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type testFeature struct {
	tag ot.Tag
	typ LayoutTagType
}

func (f testFeature) Tag() ot.Tag          { return f.tag }
func (f testFeature) Type() LayoutTagType  { return f.typ }
func (f testFeature) Params() ot.Navigator { return nil }
func (f testFeature) LookupCount() int     { return 0 }
func (f testFeature) LookupIndex(int) int  { return 0 }

func loadTestFont(t *testing.T, filename string) *ot.Font {
	t.Helper()
	path := filepath.Join("..", "testdata", "fonttools-tests", filename)
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
	lookup := otf.Layout.GSub.LookupList.Navigate(lookupIndex)
	feat := testFeature{tag: ot.T("test"), typ: GSubFeatureType}
	buf := append(GlyphBuffer(nil), input...)
	_, ok, out, _ := applyLookup(&lookup, feat, buf, pos, alt, otf.Layout.GDef, otf.Layout.GSub.LookupList)
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
			expected: []ot.GlyphIndex{21},
			applied:  true,
		},
		{
			name:     "alt-last",
			input:    []ot.GlyphIndex{18},
			alt:      -1,
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
