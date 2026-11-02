package otshape

import (
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

// Check values at original scalar anchors, independently of glyph output.
// This complements the public HarfBuzz regressions with signed values and
// verifies that lookup inclusion, masks and retained stages agree.
func TestFeaturePrecedencePlanValues(t *testing.T) {
	tag := ot.T("salt")
	for _, tc := range []struct {
		name     string
		user     []FeatureRange
		override int   // 0: none, 1: enable, -1: disable
		want     []int // 0: disabled; nonzero: decoded feature value
	}{
		{"collected_default_outside_range", []FeatureRange{{Feature: tag, On: true, Arg: 3, Start: 1, End: 2}}, 0, []int{2, 3, 2}},
		{"default_alternate_zero", []FeatureRange{{Feature: tag, On: true, Arg: 0}}, 0, []int{1, 1, 1}},
		// All negative values denote the same random-selection mode (SHAPE-020).
		{"negative_random_value", []FeatureRange{{Feature: tag, On: true, Arg: -7, Start: 1, End: 2}}, 0, []int{2, -1, 2}},
		{"oversized_positive_value", []FeatureRange{{Feature: tag, On: true, Arg: 100000}}, 0, []int{100000, 100000, 100000}},
		{"override_clears_ranges", []FeatureRange{{Feature: tag, On: false}, {Feature: tag, On: true, Arg: -7, Start: 1, End: 2}}, 1, []int{1, 1, 1}},
		{"override_disables_ranges", []FeatureRange{{Feature: tag, On: true, Arg: 3, Start: 1, End: 2}}, -1, []int{0, 0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hooks := newPlanHookSet()
			planner := newPlanFeaturePlanner(nil, SelectionContext{}, &hooks)
			planner.AddFeature(tag, FeatureNone, 2)
			planner.AddGSUBPause(nil)
			planner.applyUserFeatures(tc.user)
			switch tc.override {
			case 1:
				planner.EnableFeature(tag)
			case -1:
				planner.DisableFeature(tag)
			}
			ranges := planner.maskFeatures()
			layout, err := compileUserFeatureMasks(ranges)
			if err != nil {
				t.Fatal(err)
			}
			masks := []uint32{layout.GlobalMask, layout.GlobalMask, layout.GlobalMask}
			applyFeatureRangesToMasks(masks, []uint32{0, 1, 2}, layout.ByFeature, ranges)
			spec := layout.ByFeature[tag]
			got := make([]int, len(masks))
			for i, mask := range masks {
				if mask&spec.Mask != 0 {
					got[i] = spec.decode(mask)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("values=%v, want %v", got, tc.want)
			}
			if tc.override != 0 && (len(ranges) != 1 || !reflect.DeepEqual(spec.Values, []int{0, 1})) {
				t.Fatalf("overridden values retained: ranges=%+v codes=%v", ranges, spec.Values)
			}
			features := []otlayout.Feature{nil, fakeFeature{tag: tag, lookups: []int{0}}}
			prog, _, err := compileTableProgram(features, planGSUB, planner.defaultTags(planGSUB), planner.toggles(), nil, layout, planPolicy{}, planner.tableStages(planGSUB))
			if err != nil {
				t.Fatal(err)
			}
			if tc.override == -1 {
				if len(prog.Lookups) != 0 {
					t.Fatal("disabled feature retains lookup")
				}
			} else if len(prog.Lookups) != 1 || len(prog.FeatureBinds) != 1 || prog.FeatureBinds[0].Stage != 0 {
				t.Fatalf("active feature lost lookup or earliest stage: %+v", prog)
			}
		})
	}
}
