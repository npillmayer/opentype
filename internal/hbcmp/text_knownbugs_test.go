//go:build shaping_knownbugs

package hbcmp

import "testing"

// Cluster level 0 and logical sink order are agreed contracts. The ordinary
// runner now owns the passing plain-shalom regression (SHAPE-009 resolved).

// The normal parser deliberately requires GPOS. This baseline records that
// capability boundary, not an accidental parser regression; see SHAPE-014.
func TestKnownBugSHAPE014AbsentGPOS(t *testing.T) {
	runTextBaselineID(t, "hebrew_vav_holam_no_gpos")
}

// Default features share one GSUB stage in HarfBuzz, so their lookups run in
// LookupList order. The ordinary control latin_lookup_order_calt_first uses
// the same rules with the lookup order swapped.
