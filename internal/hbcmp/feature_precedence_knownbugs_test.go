//go:build shaping_knownbugs

package hbcmp

import "testing"

// An engine's DisableFeature removes the feature's lookups from the plan even
// when the user enabled it; an engine's EnableFeature cannot activate a feature
// the user disabled, because the user's setting alone decides the glyph mask.
// Both contradict the agreed precedence contract (see feature_precedence_test.go).
func TestKnownBugSHAPE021FeaturePrecedence(t *testing.T) {
	runFeaturePrecedenceCases(t, true)
}
