//go:build shaping_knownbugs

package hbcmp

import "testing"

// These assert unchanged upstream expectations. Move passing baselines into
// TestAOTSGSUBBaselines or TestAOTSGPOSBaselines once fixed; see doc/SHAPING_FINDINGS.md.
// GSUB must form the ligature across an ignored mark before GPOS can position
// that mark on its component. Classification is fixed; span replacement still
// deletes the ignored mark. Retain the full upstream expectation.
func TestKnownBugSHAPE005AOTSLigatureAcrossMark(t *testing.T) {
	runAOTSGPOSCase(t, "gpos5.tests:2")
}
