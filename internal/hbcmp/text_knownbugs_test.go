//go:build shaping_knownbugs

package hbcmp

import "testing"

// SHAPE-009/011 are contract questions: these assert HB visual output order
// and cluster level 0, not a settled public API guarantee. See the findings.
func TestKnownBugSHAPE009HebrewRTLOrder(t *testing.T) {
	runTextBaselineID(t, "hebrew_shalom")
}

func TestKnownBugSHAPE010HebrewComposition(t *testing.T) {
	for _, id := range []string{"hebrew_vav_holam", "hebrew_shalom_pointed"} {
		t.Run(id, func(t *testing.T) { runTextBaselineID(t, id) })
	}
}

// A single base with two marks isolates the unresolved attachments from the
// vav+holam composition problem. Order and mark clusters also differ (009/011).
func TestKnownBugSHAPE007HebrewMarkOffsets(t *testing.T) {
	runTextBaselineID(t, "hebrew_shin_pointed")
}
