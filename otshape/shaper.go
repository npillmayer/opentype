package otshape

import (
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"

	"github.com/npillmayer/opentype/ot"
)

// Params bundles font and segment metadata used for one shaping request.
type Params struct {
	Font      *ot.Font        // Font is the OpenType font used for mapping and layout.
	Direction bidi.Direction  // Direction is the segment writing direction.
	Script    language.Script // Script is the ISO 15924 script for shaper selection.
	Language  language.Tag    // Language is the BCP 47 language tag for language-system lookup.
	Features  []FeatureRange  // Features requests per-feature on/off state and optional ranges.
}

// FeatureRange toggles one OpenType feature for an optional span of the original
// input's Unicode scalars. Indexes are zero-based, not UTF-8 byte offsets or
// current glyph-buffer positions; they remain fixed across streaming windows.
// Range boundaries inside composed characters or merged source clusters do not
// yet have a specified general policy.
type FeatureRange struct {
	Feature ot.Tag // Feature is the 4-byte OpenType feature tag.
	// Arg selects a one-based alternate; zero means first/default and negatives
	// request random selection. Oversized positives leave the glyph unchanged
	// by that alternate substitution. Arg is ignored when On is false.
	Arg   int
	On    bool // On enables (true) or disables (false) the feature for the selected range.
	Start int  // Start is the inclusive original scalar index; values <= 0 mean start of input.
	End   int  // End is the exclusive original scalar index; values <= 0 mean end of input.
}
