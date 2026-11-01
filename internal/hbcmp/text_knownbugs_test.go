//go:build shaping_knownbugs

package hbcmp

import "testing"

// Cluster level 0 and logical sink order are agreed contracts. The ordinary
// runner now owns the passing plain-shalom regression (SHAPE-009 resolved).

// Historical SHAPE-010 reproduction: composition, glyph count, clusters and
// per-glyph positions now agree. The separate Hebrew mark-ordering follow-up
// still blocks full parity at the initial shin cluster; retain the full oracle.
func TestKnownBugSHAPE010HebrewComposition(t *testing.T) {
	t.Run("hebrew_shalom_pointed", func(t *testing.T) { runTextBaselineID(t, "hebrew_shalom_pointed") })
}

// Historical SHAPE-007 reproduction: offsets now agree with HarfBuzz, but
// internal mark order still differs (SHAPE-011 follow-up). Keep the complete
// oracle and this test tagged until that remaining difference is resolved.
func TestKnownBugSHAPE007HebrewMarkOffsets(t *testing.T) {
	runTextBaselineID(t, "hebrew_shin_pointed")
}

// The normal parser deliberately requires GPOS. This baseline records that
// capability boundary, not an accidental parser regression; see SHAPE-014.
func TestKnownBugSHAPE014AbsentGPOS(t *testing.T) {
	runTextBaselineID(t, "hebrew_vav_holam_no_gpos")
}

// Default features share one GSUB stage in HarfBuzz, so their lookups run in
// LookupList order. The ordinary control latin_lookup_order_calt_first uses
// the same rules with the lookup order swapped.
