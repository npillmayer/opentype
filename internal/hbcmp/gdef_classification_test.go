package hbcmp

import (
	"github.com/npillmayer/opentype/ot"
	"testing"
)

func TestAOTSGDEFClassification(t *testing.T) {
	t.Run("gdef_singleton_ranges", func(t *testing.T) {
		c, err := loadAOTSGSUBCase("gsub_context1_lookupflag.tests:2")
		if err != nil {
			t.Fatal(err)
		}
		font, err := ot.Parse(c.fontData, ot.IsTestfont)
		if err != nil {
			t.Fatal(err)
		}
		if font.Layout.GDef == nil {
			t.Fatal("fixture has no GDEF table")
		}
		// The fixture has inclusive singleton ranges [90,90], [91,91],
		// [92,92], each with class 3 (mark). These are not empty ranges.
		for _, gid := range []ot.GlyphIndex{90, 91, 92} {
			if class := font.Layout.GDef.GlyphClassDef.Lookup(gid); class != 3 {
				t.Errorf("GDEF class for GID %d = %d, want mark class 3", gid, class)
			}
		}
	})
	t.Run("gdef_class_values", func(t *testing.T) {
		// ClassDefinitions returns the font's OpenType numbers directly, so the
		// constants used by skipGlyph must agree with the on-disk encoding.
		for _, tc := range []struct {
			name string
			got  ot.GlyphClassDefEnum
			want int
		}{
			{"base", ot.BaseGlyph, 1}, {"ligature", ot.LigatureGlyph, 2},
			{"mark", ot.MarkGlyph, 3}, {"component", ot.ComponentGlyph, 4},
		} {
			if int(tc.got) != tc.want {
				t.Errorf("%s class constant = %d, want %d", tc.name, tc.got, tc.want)
			}
		}
	})
}
