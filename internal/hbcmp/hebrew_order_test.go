package hbcmp

import (
	"testing"

	"github.com/npillmayer/opentype/otshape"
)

var hebrewOrderFindings = map[string]string{
	"hebrew_order_sin": "", "hebrew_order_dagesh": "", "hebrew_order_equal_class": "",
	"hebrew_order_meteg": "", "hebrew_order_two_sequences": "",
	"hebrew_compose_across_dot": "", "hebrew_compose_blocked_equal": "",
	"hebrew_meteg_before_compose": "", "hebrew_order_range_inside": "",
}

func TestSHAPE023HebrewOrdering(t *testing.T) {
	cases := loadTextBaselineFile(t, "testdata/text/hebrew_order.json", hebrewOrderFindings, nil)
	for _, id := range []string{"hebrew_shin_pointed", "hebrew_shalom_pointed", "hebrew_shin_reordered"} {
		cases = append(cases, textBaselineByID(t, id))
	}
	for _, c := range cases {
		for _, entry := range []string{"runes", "events"} {
			t.Run(c.ID+"/"+entry, func(t *testing.T) {
				n := len(c.Fixture.Input)
				runStreamCase(t, c, streamProfile{"whole", n + 1, 1, n + 2}, entry, otshape.FlushOnRunBoundary)
			})
		}
	}
}
