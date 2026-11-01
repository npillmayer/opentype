package otshape

import (
	"errors"
	"fmt"

	"github.com/npillmayer/opentype/ot"
)

// ErrConflictingAlternates reports incompatible values from active features
// sharing an alternate lookup in one stage. Zero/default equals one; all negative
// values mean random. The error is raised only when an alternate set matches,
// including through contextual/extension lookups. Earlier edits/emissions remain.
var ErrConflictingAlternates = errors.New("otshape: conflicting alternate values")

func (e *planExecutor) lookupAlternateSelection(pl *plan, op lookupOp, index, fallback int) (int, error) {
	// Hand-built/internal lookup programs may have only the representative tag.
	if len(op.Features) == 0 {
		if op.Flags.has(lookupRandom) {
			return -1, nil
		}
		if spec, ok := pl.maskForFeature(op.FeatureTag); ok && spec.Mask != 0 {
			return spec.decode(e.run.Masks[index]), nil
		}
		return fallback, nil
	}
	var selected bool
	var value int
	var tag ot.Tag
	for _, feature := range op.Features {
		if feature.Mask != 0 && e.run.Masks[index]&feature.Mask == 0 {
			continue
		}
		v := 1
		// Required and unmasked defaults contribute the default alternate,
		// regardless of a user's mask for another contributor to this lookup.
		if !feature.Required && feature.Mask != 0 {
			if spec, ok := pl.maskForFeature(feature.Tag); ok {
				v = spec.decode(e.run.Masks[index])
			}
		}
		if v == 0 {
			v = 1
		}
		if v < 0 {
			v = -1
		}
		if selected && v != value {
			return value, fmt.Errorf("%w: lookup %d, glyph %d, %s=%d and %s=%d", ErrConflictingAlternates, op.LookupIndex, index, tag, value, feature.Tag, v)
		}
		selected, value, tag = true, v, feature.Tag
	}
	// The retained lookup flag forces random selection, but does not suppress
	// conflicts between explicitly selected contributor values.
	if op.Flags.has(lookupRandom) {
		return -1, nil
	}
	return value, nil
}
