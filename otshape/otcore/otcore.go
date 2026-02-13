package otcore

import (
	"github.com/npillmayer/opentype/otshape"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// Shaper is the default OpenType shaping engine.
type Shaper struct{}

var _ otshape.ShapingEngine = Shaper{}
var _ otshape.ShapingEnginePolicy = Shaper{}

// New returns the core/default shaping engine.
func New() otshape.ShapingEngine {
	return Shaper{}
}

func (Shaper) Name() string {
	return "core"
}

// Match returns a neutral score so script-specific shapers can outvote core.
func (Shaper) Match(ctx otshape.SelectionContext) otshape.ShaperConfidence {
	if ctx.Script == language.MustParseScript("Latn") {
		return otshape.ShaperConfidenceHigh
	}
	if ctx.Direction != bidi.LeftToRight {
		return otshape.ShaperConfidenceNone
	}
	return otshape.ShaperConfidenceLow
}

func (Shaper) New() otshape.ShapingEngine {
	return Shaper{}
}

func (Shaper) NormalizationPreference() otshape.NormalizationMode {
	return otshape.NormalizationAuto
}

func (Shaper) ApplyGPOS() bool {
	return true
}
