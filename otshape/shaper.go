package otshape

import (
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"

	"github.com/npillmayer/opentype/ot"
)

// Params collects shaping parameters.
type Params struct {
	Font      *ot.Font        // use a font at a given point-size
	Direction bidi.Direction  // writing direction
	Script    language.Script // 4-letter ISO 15924 script identifier
	Language  language.Tag    // BCP 47 language tag
	Features  []FeatureRange  // OpenType features to apply
}

// FeatureRange tells a shaper to turn a certain OpenType feature on or off for a
// run of code-points.
type FeatureRange struct {
	Feature    ot.Tag // 4-letter feature tag
	Arg        int    // optional argument for this feature
	On         bool   // turn it on or off?
	Start, End int    // position of code-points to apply feature for
}
