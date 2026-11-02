package hbcmp

import (
	"fmt"
	"testing"

	"github.com/npillmayer/opentype/otshape"
)

// Groups keep the original SHAPE-016 cases and promoted SHAPE-022 regression
// independently addressable. Every group now belongs to the ordinary suite.
func runFeatureScopeBaselines(t *testing.T, group string) {
	assignments := map[string]string{
		"scope_liga_second_off": "", "scope_liga_first_off": "",
		"scope_kern_second_off": "", "scope_kern_first_off": "",
		"scope_unrelated_ligature": "", "scope_composition": "",
		"scope_mark_inside": "022", "scope_mark_inside_no_mkmk": "", "scope_implicit_restore": "",
		"scope_nested_ligatures": "", "scope_hebrew_group": "",
	}
	for _, c := range loadTextBaselineFile(t, "testdata/text/scopes.json", assignments, nil) {
		if assignments[c.ID] != group {
			continue
		}
		for _, entry := range []string{"runes", "events"} {
			for _, boundary := range []otshape.FlushBoundary{otshape.FlushOnRunBoundary, otshape.FlushOnClusterBoundary} {
				t.Run(c.ID+"/"+entry+"/"+fmt.Sprint(boundary), func(t *testing.T) {
					n := len(c.Fixture.Input)
					runStreamCase(t, c, streamProfile{"whole", n + 1, 1, n + 2}, entry, boundary)
				})
			}
		}
	}
}

func TestSHAPE016FeatureScopes(t *testing.T) { runFeatureScopeBaselines(t, "") }

func TestSHAPE022MarkAttachmentCrossesBase(t *testing.T) {
	runFeatureScopeBaselines(t, "022")
}
