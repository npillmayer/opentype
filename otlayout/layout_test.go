package otlayout

import (
	"errors"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestConcreteGraphHelpers(t *testing.T) {
	otf := loadTestFont(t, "gsub3_1_simple_f1.otf")
	gsub := otf.Table(ot.T("GSUB"))
	if gsub == nil {
		t.Fatal("font has no GSUB table")
	}

	scriptGraph, err := GetScriptGraph(gsub)
	if err != nil {
		t.Fatalf("GetScriptGraph failed: %v", err)
	}
	featureGraph, err := GetFeatureGraph(gsub)
	if err != nil {
		t.Fatalf("GetFeatureGraph failed: %v", err)
	}
	lookupGraph, err := GetLookupGraph(gsub)
	if err != nil {
		t.Fatalf("GetLookupGraph failed: %v", err)
	}

	if scriptGraph.Len() == 0 {
		t.Fatal("script graph is unexpectedly empty")
	}
	if featureGraph.Len() == 0 {
		t.Fatal("feature graph is unexpectedly empty")
	}
	if lookupGraph.Len() == 0 {
		t.Fatal("lookup graph is unexpectedly empty")
	}

	scriptTags := ScriptTags(scriptGraph)
	if len(scriptTags) != scriptGraph.Len() {
		t.Fatalf("ScriptTags length mismatch: got %d, want %d", len(scriptTags), scriptGraph.Len())
	}
	featureTags := FeatureTags(featureGraph)
	if len(featureTags) != featureGraph.Len() {
		t.Fatalf("FeatureTags length mismatch: got %d, want %d", len(featureTags), featureGraph.Len())
	}
}

func TestConcreteGraphHelpersRejectNonLayoutTable(t *testing.T) {
	otf := loadTestFont(t, "gsub3_1_simple_f1.otf")
	cmap := otf.Table(ot.T("cmap"))
	if cmap == nil {
		t.Fatal("font has no cmap table")
	}

	if _, err := GetScriptGraph(cmap); err == nil {
		t.Fatal("GetScriptGraph should fail for non-layout table")
	}
	if _, err := GetFeatureGraph(cmap); err == nil {
		t.Fatal("GetFeatureGraph should fail for non-layout table")
	}
	if _, err := GetLookupGraph(cmap); err == nil {
		t.Fatal("GetLookupGraph should fail for non-layout table")
	}
}

func TestFeaturesAndLookupsHelpers(t *testing.T) {
	otf := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	gsub := otf.Table(ot.T("GSUB"))
	if gsub == nil {
		t.Fatal("font has no GSUB table")
	}

	scriptGraph, err := GetScriptGraph(gsub)
	if err != nil {
		t.Fatalf("GetScriptGraph failed: %v", err)
	}
	lookupGraph, err := GetLookupGraph(gsub)
	if err != nil {
		t.Fatalf("GetLookupGraph failed: %v", err)
	}

	var candidate *ot.Feature
	for _, script := range scriptGraph.Range() {
		if script == nil {
			continue
		}
		checkLangSys := func(lsys *ot.LangSys) bool {
			if lsys == nil {
				return false
			}
			features, err := FeaturesForLangSys(lsys)
			if err != nil || len(features) == 0 {
				return false
			}
			for _, feat := range features {
				if feat != nil && feat.LookupCount() > 0 {
					candidate = feat
					return true
				}
			}
			return false
		}
		if checkLangSys(script.DefaultLangSys()) {
			break
		}
		for _, lsys := range script.Range() {
			if checkLangSys(lsys) {
				break
			}
		}
		if candidate != nil {
			break
		}
	}

	if candidate == nil {
		t.Fatal("could not find a usable feature with lookup refs")
	}

	lookups, err := LookupsForFeature(candidate, lookupGraph)
	if err != nil {
		t.Fatalf("LookupsForFeature failed: %v", err)
	}
	if len(lookups) == 0 {
		t.Fatal("LookupsForFeature unexpectedly returned no lookups")
	}
	for i, lookup := range lookups {
		if lookup == nil {
			t.Fatalf("LookupsForFeature returned nil lookup at index %d", i)
		}
	}
}

func TestLayoutHelperErrors(t *testing.T) {
	if _, err := FeaturesForLangSys(nil); !errors.Is(err, ErrVoid) {
		t.Fatalf("FeaturesForLangSys(nil) error = %v, want %v", err, ErrVoid)
	}
	if _, err := LookupsForFeature(nil, nil); !errors.Is(err, ErrVoid) {
		t.Fatalf("LookupsForFeature(nil,nil) error = %v, want %v", err, ErrVoid)
	}

	zeroFeature := &ot.Feature{}
	if _, err := LookupsForFeature(zeroFeature, nil); !errors.Is(err, ErrNoLookupGraph) {
		t.Fatalf("LookupsForFeature(feature,nil) error = %v, want %v", err, ErrNoLookupGraph)
	}

	otf := loadTestFont(t, "gsub3_1_simple_f1.otf")
	gsub := otf.Table(ot.T("GSUB"))
	if gsub == nil {
		t.Fatal("font has no GSUB table")
	}
	lookupGraph, err := GetLookupGraph(gsub)
	if err != nil {
		t.Fatalf("GetLookupGraph failed: %v", err)
	}
	if _, err := LookupsForFeature(zeroFeature, lookupGraph); !errors.Is(err, ErrFeatureHasNoRefs) {
		t.Fatalf("LookupsForFeature(zeroFeature,lookupGraph) error = %v, want %v", err, ErrFeatureHasNoRefs)
	}

	if got := ScriptTags(nil); got != nil {
		t.Fatalf("ScriptTags(nil) = %v, want nil", got)
	}
	if got := FeatureTags(nil); got != nil {
		t.Fatalf("FeatureTags(nil) = %v, want nil", got)
	}
}
