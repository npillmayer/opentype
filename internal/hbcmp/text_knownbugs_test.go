//go:build shaping_knownbugs

package hbcmp

import "testing"

// Cluster level 0 and logical sink order are agreed contracts. The ordinary
// runner now owns the passing plain-shalom regression (SHAPE-009 resolved).

func TestKnownBugSHAPE010HebrewComposition(t *testing.T) {
	for _, id := range []string{"hebrew_vav_holam", "hebrew_shalom_pointed", "hebrew_vav_holam_mark_off", "hebrew_vav_holam_no_mark_requested"} {
		t.Run(id, func(t *testing.T) { runTextBaselineID(t, id) })
	}
}

// Historical SHAPE-007 reproduction: offsets now agree with HarfBuzz, but
// clusters and internal mark order still differ (SHAPE-011). Keep the complete
// oracle and this test tagged until both remaining differences are resolved.
func TestKnownBugSHAPE007HebrewMarkOffsets(t *testing.T) {
	runTextBaselineID(t, "hebrew_shin_pointed")
}

func TestKnownBugSHAPE011CombiningClusters(t *testing.T) {
	for _, id := range []string{"latin_marks_canonical", "latin_uncomposable_mark", "hebrew_vav_holam_mark_on", "hebrew_shin_reordered"} {
		t.Run(id, func(t *testing.T) { runTextBaselineID(t, id) })
	}
}

func TestKnownBugSHAPE012CanonicalOrdering(t *testing.T) {
	runTextBaselineID(t, "latin_marks_reordered")
}

func TestKnownBugSHAPE013UnavailableComposition(t *testing.T) {
	for _, id := range []string{"latin_missing_composite_nfc", "latin_missing_composite_nfd"} {
		t.Run(id, func(t *testing.T) { runTextBaselineID(t, id) })
	}
}

// The normal parser deliberately requires GPOS. This baseline records that
// capability boundary, not an accidental parser regression; see SHAPE-014.
func TestKnownBugSHAPE014AbsentGPOS(t *testing.T) {
	runTextBaselineID(t, "hebrew_vav_holam_no_gpos")
}
