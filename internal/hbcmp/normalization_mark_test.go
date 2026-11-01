package hbcmp

import (
	"testing"

	"github.com/npillmayer/opentype/otshape"
)

func TestSHAPE010EventBaselines(t *testing.T) {
	// Shape's whole-input runner also covers these unchanged oracles. Ranged
	// captures remain Shape-only: event boundaries split normalization spans
	// (SHAPE-016), so they are not equivalent to indexed feature ranges.
	for _, id := range []string{
		"hebrew_vav_holam", "hebrew_vav_holam_mark_on", "hebrew_vav_holam_mark_off",
		"hebrew_vav_holam_no_mark", "hebrew_vav_holam_no_mark_requested",
		"hebrew_vav_holam_mark_only", "hebrew_vav_holam_mkmk_only",
	} {
		t.Run(id, func(t *testing.T) {
			c := textBaselineByID(t, id)
			n := len(c.Fixture.Input)
			runStreamCase(t, c, streamProfile{"whole", n + 1, 1, n + 2}, "events", otshape.FlushOnRunBoundary)
		})
	}
}
