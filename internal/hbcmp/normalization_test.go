package hbcmp

import (
	"testing"

	"github.com/npillmayer/opentype/otshape"
)

var normalizationBaselineFindings = map[string]string{
	"norm_precomposed_marks": "", "norm_reordered_marks": "", "norm_canonical_marks": "",
	"norm_blocked": "", "norm_unblocked": "", "norm_equal_class": "",
	"norm_no_adot_precomposed": "", "norm_no_adot_reordered": "", "norm_no_adot_canonical": "",
	"norm_partial_decomposition": "", "norm_retain_component": "", "norm_retain_standalone": "",
	"norm_range_after_expansion": "", "norm_range_after_reordering": "",
}

func TestSHAPE012013NormalizationBaselines(t *testing.T) {
	for _, c := range loadTextBaselineFile(t, "testdata/text/normalization.json", normalizationBaselineFindings, nil) {
		for _, entry := range []string{"runes", "events"} {
			t.Run(c.ID+"/"+entry, func(t *testing.T) {
				n := len(c.Fixture.Input)
				runStreamCase(t, c, streamProfile{"whole", n + 1, 1, n + 2}, entry, otshape.FlushOnRunBoundary)
			})
		}
	}
}

func TestSHAPE012013OriginalEventBaselines(t *testing.T) {
	for _, id := range []string{"latin_marks_canonical", "latin_marks_reordered", "latin_missing_composite_nfc", "latin_missing_composite_nfd"} {
		t.Run(id, func(t *testing.T) {
			c := textBaselineByID(t, id)
			n := len(c.Fixture.Input)
			runStreamCase(t, c, streamProfile{"whole", n + 1, 1, n + 2}, "events", otshape.FlushOnRunBoundary)
		})
	}
}
