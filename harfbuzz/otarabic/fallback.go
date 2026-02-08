package otarabic

import (
	"github.com/go-text/typesetting/language"
	"github.com/npillmayer/opentype/harfbuzz"
)

// ArabicFallbackEngine encapsulates fallback orchestration for Arabic shaping.
// It is intentionally narrow: callers only request "apply when needed".
type ArabicFallbackEngine interface {
	Apply(font *harfbuzz.Font, buffer *harfbuzz.Buffer) bool
}

type arabicFallbackEngine struct {
	enabled      bool
	featureMasks [arabicFallbackMaxLookups]harfbuzz.GlyphMask
	program      *harfbuzz.SyntheticGSUBProgram
}

var _ ArabicFallbackEngine = (*arabicFallbackEngine)(nil)

func newArabicFallbackEngine(plan harfbuzz.PlanContext) *arabicFallbackEngine {
	engine := &arabicFallbackEngine{
		enabled: plan.Script() == language.Arabic,
	}

	for _, arabFeat := range arabicFeatures {
		engine.enabled = engine.enabled &&
			(featureIsSyriac(arabFeat) || plan.FeatureNeedsFallback(arabFeat))
	}
	for i, fallbackFeat := range arabicFallbackFeatures {
		engine.featureMasks[i] = plan.FeatureMask1(fallbackFeat)
	}

	return engine
}

func (e *arabicFallbackEngine) Apply(font *harfbuzz.Font, buffer *harfbuzz.Buffer) bool {
	if e == nil || !e.enabled {
		return false
	}
	if e.program == nil {
		e.program = newArabicFallbackProgram(e.featureMasks, font)
	}
	return e.program.Apply(font, buffer)
}
