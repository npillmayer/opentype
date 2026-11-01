package otshape

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestNormalizationMarkSupport(t *testing.T) {
	mark, mkmk, kern := ot.T("mark"), ot.T("mkmk"), ot.T("kern")
	markFeature := fakeFeature{tag: mark, typ: otlayout.GPosFeatureType, lookups: []int{0}}
	mkmkFeature := fakeFeature{tag: mkmk, typ: otlayout.GPosFeatureType, lookups: []int{1}}
	features := []otlayout.Feature{nil, markFeature, mkmkFeature}
	on := FeatureRange{Feature: mark, On: true}
	off := FeatureRange{Feature: mark}
	rangeOn := FeatureRange{Feature: mark, On: true, Start: 3, End: 5}
	rangeOff := FeatureRange{Feature: mark, Start: 0, End: 2}
	for _, tc := range []struct {
		name     string
		features []otlayout.Feature
		ranges   []FeatureRange
		want     bool
	}{
		{"default", features, nil, true},
		{"explicit on", features, []FeatureRange{on}, true},
		{"mark off with mkmk on", features, []FeatureRange{off}, false},
		{"mark on with mkmk off", features, []FeatureRange{on, {Feature: mkmk}}, true},
		{"both off", features, []FeatureRange{off, {Feature: mkmk}}, false},
		{"range on", features, []FeatureRange{rangeOn}, true},
		{"global off with range on", features, []FeatureRange{off, rangeOn}, true},
		{"range off", features, []FeatureRange{rangeOff}, true},
		{"last global off", features, []FeatureRange{on, off}, false},
		{"absent but requested", []otlayout.Feature{nil, mkmkFeature}, []FeatureRange{on}, false},
		{"empty mark feature", []otlayout.Feature{nil, fakeFeature{tag: mark}}, []FeatureRange{on}, false},
		{"required cannot be disabled", []otlayout.Feature{markFeature}, []FeatureRange{off}, true},
		// kern sorts before mark and becomes the representative of lookup 0.
		{"shared nonrepresentative mark", []otlayout.Feature{nil, markFeature,
			fakeFeature{tag: kern, typ: otlayout.GPosFeatureType, lookups: []int{0}}}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			masks, err := compileUserFeatureMasks(tc.ranges)
			if err != nil {
				t.Fatal(err)
			}
			prog, _, err := compileTableProgram(tc.features, planGPOS, defaultGPOSFeatures,
				collectUserFeatureToggles(tc.ranges), nil, masks, planPolicy{}, tableStagePlan{})
			if err != nil {
				t.Fatal(err)
			}
			pl := &plan{GPOS: prog, Masks: masks, featureRanges: tc.ranges, Policy: planPolicy{ApplyGPOS: true}}
			if got := planHasGposMark(pl); got != tc.want {
				t.Fatalf("mark support = %t, want %t; lookups=%+v", got, tc.want, prog.Lookups)
			}
			pl.Policy.ApplyGPOS = false
			if planHasGposMark(pl) {
				t.Fatal("disabled GPOS still reports mark support")
			}
		})
	}
	if planHasGposMark(nil) {
		t.Fatal("nil plan reports mark support")
	}
}

func TestNormalizationMarkContributorEnablement(t *testing.T) {
	// A selected lookup may retain an inactive contributor (e.g. an engine
	// requests mark while its user mask stays off). Do not infer mark support
	// from a different contributor making the shared lookup unconditional.
	mark, mkmk := ot.T("mark"), ot.T("mkmk")
	pl := &plan{
		Policy: planPolicy{ApplyGPOS: true},
		GPOS: tableProgram{Lookups: []lookupOp{{FeatureTag: mark, Features: []lookupFeature{
			{Tag: mark, Mask: 2}, {Tag: mkmk},
		}}}},
		Masks: maskLayout{GlobalMask: 4},
	}
	if planHasGposMark(pl) {
		t.Fatal("inactive mark contributor reports support through mkmk")
	}
	pl.Masks.GlobalMask |= 2
	if !planHasGposMark(pl) {
		t.Fatal("active mark contributor does not report support")
	}
}
