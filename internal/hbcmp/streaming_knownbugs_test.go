//go:build shaping_knownbugs

package hbcmp

import "testing"

// Every cell compares against independent whole-text output. These do not
// assert buggy output or auto-skip newly passing cells: promote them instead.
func TestKnownBugSHAPE015StreamingContext(t *testing.T)    { runStreamingMatrix(t, "015") }
func TestKnownBugSHAPE016FeatureScopeContext(t *testing.T) { runStreamingMatrix(t, "016") }
func TestKnownBugSHAPE017GSUBDeletion(t *testing.T)        { runStreamingMatrix(t, "017") }
func TestKnownBugSHAPE018MaskedLookupScan(t *testing.T)    { runStreamingMatrix(t, "018") }
func TestKnownBugSHAPE002StreamingBaselines(t *testing.T)  { runStreamingMatrix(t, "002") }
func TestKnownBugSHAPE011StreamingBaselines(t *testing.T)  { runStreamingMatrix(t, "011") }
