package otquery

import (
	"fmt"
	"math"

	"github.com/npillmayer/opentype/ot"
)

// FontScale returns the factor for converting font units to the units of size.
// Size is the desired em size: use pixels per em for pixel results, points for
// point results, or 1 for em results. For example, a 12-point font at 96 DPI has
// a pixel em size of 12 * 96 / 72 = 16.
//
// The factor is size / unitsPerEm, using the font's head table. Compute it once
// per font and size, then multiply font-unit values by it:
//
//	advance := float64(pos.XAdvance) * scale
//	offset := float64(pos.XOffset) * scale
//
// Use final shaped advances and offsets when placing shaped glyphs;
// [GlyphMetrics] returns nominal metrics before positioning adjustments.
// [FontMetrics] and [GlyphMetrics] values can also be scaled with this factor.
// Scaling preserves signs and fractions; it does not round, hint, or change
// coordinate orientation. For a screen coordinate system with Y increasing
// downward, the caller must negate Y advances and offsets separately.
//
// FontScale returns zero and an error if the font is nil, its head table is
// unavailable or too short, unitsPerEm is outside 16..16384, or size is not
// positive and finite. It also rejects sizes whose scale underflows to zero.
func FontScale(otf *ot.Font, size float64) (float64, error) {
	if size <= 0 || math.IsNaN(size) || math.IsInf(size, 0) {
		return 0, fmt.Errorf("font scale: size must be positive and finite")
	}
	head, ok := HeadInfo(otf)
	if !ok {
		return 0, fmt.Errorf("font scale: head table unavailable or too short")
	}
	if head.UnitsPerEm < 16 || head.UnitsPerEm > 16384 {
		return 0, fmt.Errorf("font scale: units per em %d outside 16..16384", head.UnitsPerEm)
	}
	scale := size / float64(head.UnitsPerEm)
	if scale == 0 {
		return 0, fmt.Errorf("font scale: size too small to represent scale")
	}
	return scale, nil
}
