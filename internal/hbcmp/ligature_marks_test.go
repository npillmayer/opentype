package hbcmp

import (
	"github.com/npillmayer/opentype/otshape"
	"testing"
)

var ligatureBaselineFindings = map[string]string{
	"ligature_intervening": "", "ligature_trailing": "", "ligature_many_marks": "",
	"ligature_nested_left": "", "ligature_nested_right": "", "ligature_distinct_instances": "",
	"ligature_input_glyph": "", "ligature_later_single": "", "ligature_later_multiple": "",
	"ligature_hebrew_rtl": "",
}

func TestSHAPE019LigatureBaselines(t *testing.T) {
	for _, c := range loadTextBaselineFile(t, "testdata/text/ligatures.json", ligatureBaselineFindings, nil) {
		for _, entry := range []string{"runes", "events"} {
			t.Run(c.ID+"/"+entry, func(t *testing.T) {
				n := len(c.Fixture.Input)
				runStreamCase(t, c, streamProfile{"whole", n + 1, 1, n + 2}, entry, otshape.FlushOnRunBoundary)
			})
		}
	}
}
