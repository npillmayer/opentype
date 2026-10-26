//go:build shaping_knownbugs

package hbcmp

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
)

// These assert unchanged upstream expectations. Move passing baselines into
// TestAOTSGSUBBaselines or TestAOTSGPOSBaselines once fixed; see doc/SHAPING_FINDINGS.md.
func TestKnownBugSHAPE005AOTSIgnoreMarks(t *testing.T) {
	t.Run("upstream", func(t *testing.T) {
		runAOTSGSUBCase(t, "gsub_context1_lookupflag.tests:2")
	})
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

func TestKnownBugSHAPE006AOTSContextProgression(t *testing.T) {
	runAOTSGSUBCase(t, "gsub_context1_next_glyph.tests:2")
}

// This combines ranges and values; it complements the isolated API tests.
func TestKnownBugSHAPE003AOTSAlternates(t *testing.T) {
	runAOTSGSUBCase(t, "gsub3_1_simple.tests:2")
}

func TestKnownBugSHAPE002AOTSAlternateRangesStreaming(t *testing.T) {
	runAOTSGSUBCaseWithBuffers(t, "gsub3_1_simple.tests:2", otshape.BufferOptions{HighWatermark: 2, LowWatermark: 1, MaxBuffer: 4})
}

// Unclassified GID 19 must not be filtered by IgnoreBaseGlyphs. Currently raw
// class zero equals ot.BaseGlyph, so filtering hides alternate-value behavior.
func TestKnownBugSHAPE005AOTSIgnoreBaseGlyphs(t *testing.T) {
	runAOTSGSUBCase(t, "gsub3_1_lookupflag.tests:2")
}

// GSUB must form the ligature across an ignored mark before GPOS can position
// that mark on its component. Classification currently blocks that first step.
func TestKnownBugSHAPE005AOTSLigatureAcrossMark(t *testing.T) {
	runAOTSGPOSCase(t, "gpos5.tests:2")
}

// Pair lookup 0 changes the first glyph's advance. Incorrect BaseGlyph=0
// filtering skips the unclassified first glyph before the pair can execute.
func TestKnownBugSHAPE005AOTSPairAdvance(t *testing.T) {
	runAOTSGPOSCase(t, "gpos2_1_lookupflag.tests:3")
}

func TestKnownBugSHAPE007AOTSMarkAttachmentOrigins(t *testing.T) {
	for _, id := range []string{"gpos4_simple.tests:2", "gpos5.tests:3"} {
		t.Run(id, func(t *testing.T) { runAOTSGPOSCase(t, id) })
	}
}

// These records have no covered base to attach to. The mark's own origin is
// correct, but its advance incorrectly moves the following glyph.
func TestKnownBugSHAPE008AOTSUnattachedMarkAdvances(t *testing.T) {
	for _, id := range []string{"gpos4_simple.tests:3", "gpos4_simple.tests:4"} {
		t.Run(id, func(t *testing.T) { runAOTSGPOSCase(t, id) })
	}
}
