package otcore

import (
	"github.com/npillmayer/opentype/otshape"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// Shaper is the default OpenType shaping engine.
//
// It provides a conservative baseline for scripts that do not have a
// script-specific shaper in the candidate list.
type Shaper struct{}

var _ otshape.ShapingEngine = Shaper{}
var _ otshape.ShapingEnginePolicy = Shaper{}

// New returns a new core shaping engine instance.
func New() otshape.ShapingEngine {
	return Shaper{}
}

// Name returns the stable engine name used for tie-breaking.
func (Shaper) Name() string {
	return "core"
}

// Match returns how suitable the core engine is for ctx.
//
// It prefers Latin segments, rejects non-horizontal directions, and otherwise
// returns a low confidence so script-specific engines can outvote it.
func (Shaper) Match(ctx otshape.SelectionContext) otshape.ShaperConfidence {
	if ctx.Script == language.MustParseScript("Latn") {
		return otshape.ShaperConfidenceHigh
	}
	if ctx.Direction != bidi.LeftToRight {
		return otshape.ShaperConfidenceNone
	}
	return otshape.ShaperConfidenceLow
}

// New returns a new independent core engine instance.
func (Shaper) New() otshape.ShapingEngine {
	return Shaper{}
}

// NormalizationPreference reports the engine's preferred normalization policy.
func (Shaper) NormalizationPreference() otshape.NormalizationMode {
	return otshape.NormalizationAuto
}

// ApplyGPOS reports whether the engine wants GPOS applied.
func (Shaper) ApplyGPOS() bool {
	return true
}
