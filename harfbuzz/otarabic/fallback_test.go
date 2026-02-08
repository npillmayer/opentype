package otarabic

import (
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/npillmayer/opentype/harfbuzz"
)

type mockPlanContext struct {
	script        language.Script
	featureMask1  map[ot.Tag]harfbuzz.GlyphMask
	needsFallback map[ot.Tag]bool
}

func (m mockPlanContext) Script() language.Script { return m.script }
func (m mockPlanContext) Direction() harfbuzz.Direction {
	return harfbuzz.LeftToRight
}
func (m mockPlanContext) FeatureMask1(tag ot.Tag) harfbuzz.GlyphMask {
	if m.featureMask1 == nil {
		return 0
	}
	return m.featureMask1[tag]
}
func (m mockPlanContext) FeatureNeedsFallback(tag ot.Tag) bool {
	if m.needsFallback == nil {
		return false
	}
	return m.needsFallback[tag]
}

func TestArabicFallbackEngineEnablement(t *testing.T) {
	makeFallbackMap := func(value bool) map[ot.Tag]bool {
		m := make(map[ot.Tag]bool, len(arabicFeatures))
		for _, feat := range arabicFeatures {
			m[feat] = value
		}
		return m
	}

	t.Run("non_arabic_script_disabled", func(t *testing.T) {
		engine := newArabicFallbackEngine(mockPlanContext{
			script:        language.Hebrew,
			needsFallback: makeFallbackMap(true),
		})
		if engine.enabled {
			t.Fatal("fallback engine should be disabled for non-Arabic script")
		}
		if engine.Apply(nil, nil) {
			t.Fatal("disabled fallback engine should not apply")
		}
	})

	t.Run("arabic_script_without_missing_features_disabled", func(t *testing.T) {
		engine := newArabicFallbackEngine(mockPlanContext{
			script:        language.Arabic,
			needsFallback: makeFallbackMap(false),
		})
		if engine.enabled {
			t.Fatal("fallback engine should be disabled when no fallback features are needed")
		}
	})

	t.Run("arabic_script_with_missing_features_enabled", func(t *testing.T) {
		engine := newArabicFallbackEngine(mockPlanContext{
			script:        language.Arabic,
			needsFallback: makeFallbackMap(true),
		})
		if !engine.enabled {
			t.Fatal("fallback engine should be enabled when fallback features are needed")
		}
	})
}

func TestArabicFallbackEngineCopiesFeatureMasks(t *testing.T) {
	maskMap := make(map[ot.Tag]harfbuzz.GlyphMask, len(arabicFallbackFeatures))
	for i, feat := range arabicFallbackFeatures {
		maskMap[feat] = harfbuzz.GlyphMask(i + 1)
	}

	engine := newArabicFallbackEngine(mockPlanContext{
		script:        language.Arabic,
		featureMask1:  maskMap,
		needsFallback: map[ot.Tag]bool{},
	})

	for i, feat := range arabicFallbackFeatures {
		if got, want := engine.featureMasks[i], maskMap[feat]; got != want {
			t.Fatalf("feature mask[%d] = %v, want %v", i, got, want)
		}
	}
}
