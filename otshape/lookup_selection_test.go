package otshape

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestSHAPE020SharedAlternateSelection(t *testing.T) {
	// AOTS's independent TTX lists glyph 18 -> [20,21,22]. Both synthetic
	// feature bindings address that same lookup, to exercise contributor masks.
	a, b := ot.T("salt"), ot.T("ss01")
	for _, tc := range []struct {
		name                               string
		av, bv                             int
		offB, unmasked, required, conflict bool
		want                               ot.GlyphIndex
	}{
		{"equal", 2, 2, false, false, false, false, 21},
		{"default", 0, 1, false, false, false, false, 20},
		{"negative", -1, -7, false, false, false, false, 0},
		{"oversized", 99, 99, false, false, false, false, 18},
		{"conflict", 1, 2, false, false, false, true, 18},
		{"random_conflict", -1, 2, false, false, false, true, 18},
		{"inactive", 2, 1, true, false, false, false, 21},
		{"unmasked", 1, 1, true, true, false, false, 20},
		{"unmasked_conflict", 1, 2, false, true, false, true, 18},
		{"required_conflict", 1, 2, false, false, true, true, 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
			ranges := []FeatureRange{{Feature: a, On: true, Arg: tc.av}, {Feature: b, On: true, Arg: tc.bv}}
			if tc.offB {
				ranges[1].Start = 1
				ranges[1].End = 2
			}
			if tc.unmasked {
				ranges = ranges[1:]
			}
			masks, err := compileUserFeatureMasks(ranges)
			if err != nil {
				t.Fatal(err)
			}
			features := []otlayout.Feature{nil, fakeFeature{tag: a, lookups: []int{0}}, fakeFeature{tag: b, lookups: []int{0}}}
			if tc.required {
				features = features[1:]
			}
			prog, _, err := compileTableProgram(features, planGSUB, []ot.Tag{a, b}, collectUserFeatureToggles(ranges), nil, masks, planPolicy{}, tableStagePlan{})
			if err != nil {
				t.Fatal(err)
			}
			pl := &plan{font: font, Masks: masks, GSUB: prog, featureRanges: ranges}
			run := newRunBuffer(1)
			run.Glyphs = otlayout.GlyphBuffer{18}
			run.Clusters = []uint32{0}
			e := &planExecutor{}
			e.acquireBuffer(run)
			defer e.releaseBuffer()
			e.ensureRunMasks(pl)
			// A retained random flag must not hide incompatible feature values.
			randomOp := prog.Lookups[0]
			randomOp.Flags |= lookupRandom
			value, selectionErr := e.lookupAlternateSelection(pl, randomOp, 0, 0)
			if errors.Is(selectionErr, ErrConflictingAlternates) != tc.conflict || (!tc.conflict && value != -1) {
				t.Fatalf("forced random: value=%d error=%v", value, selectionErr)
			}
			err = e.applyGSUB(pl)
			if errors.Is(err, ErrConflictingAlternates) != tc.conflict || (err != nil && !tc.conflict) {
				t.Fatalf("error=%v conflict=%v", err, tc.conflict)
			}
			if tc.want == 0 {
				if run.Glyphs[0] < 20 || run.Glyphs[0] > 22 {
					t.Fatalf("random output=%v", run.Glyphs)
				}
			} else if run.Glyphs[0] != tc.want {
				t.Fatalf("glyph=%v want=%d", run.Glyphs, tc.want)
			}
		})
	}
}

func TestSHAPE020AlternateConflictPreservesProgress(t *testing.T) {
	a, b := ot.T("salt"), ot.T("ss01")
	ranges := []FeatureRange{{Feature: a, On: true, Arg: 1}, {Feature: b, On: true, Arg: 2, Start: 1, End: 2}}
	masks, err := compileUserFeatureMasks(ranges)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := compileTableProgram([]otlayout.Feature{nil, fakeFeature{tag: a, lookups: []int{0}}, fakeFeature{tag: b, lookups: []int{0}}}, planGSUB, nil, collectUserFeatureToggles(ranges), nil, masks, planPolicy{}, tableStagePlan{})
	if err != nil {
		t.Fatal(err)
	}
	for _, covered := range []bool{false, true} {
		pl := &plan{font: loadMiniOTFont(t, "gsub3_1_simple_f1.otf"), Masks: masks, GSUB: prog, featureRanges: ranges}
		run := newRunBuffer(3)
		run.Glyphs = otlayout.GlyphBuffer{18, 17, 18}
		run.Clusters = []uint32{0, 1, 2}
		want := otlayout.GlyphBuffer{20, 17, 20}
		if covered {
			run.Glyphs[1] = 18
			want = otlayout.GlyphBuffer{20, 18, 18}
		}
		e := &planExecutor{}
		e.acquireBuffer(run)
		e.ensureRunMasks(pl)
		err = e.applyGSUB(pl)
		e.releaseBuffer()
		if errors.Is(err, ErrConflictingAlternates) != covered || !reflect.DeepEqual(run.Glyphs, want) {
			t.Fatalf("covered=%v err=%v glyphs=%v want=%v", covered, err, run.Glyphs, want)
		}
	}
}
