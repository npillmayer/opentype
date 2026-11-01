//go:build shaping_knownbugs

package hbcmp

import "testing"

func TestKnownBugSHAPE022MarkAttachmentCrossesBase(t *testing.T) {
	runFeatureScopeBaselines(t, "022")
}
