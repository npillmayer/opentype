package otshape

import (
	"fmt"
	"slices"

	"golang.org/x/text/unicode/bidi"
)

// VisualOrder converts logical shaping output for one horizontal directional
// run to visual glyph order. direction must be bidi.LeftToRight or
// bidi.RightToLeft and must match the direction used to shape the run.
// Left-to-right order is unchanged. Right-to-left order reverses all records,
// including records within the same cluster; it does not sort by cluster.
//
// If inPlace is true, VisualOrder reorders glyphs in its existing backing array
// and returns the same slice. Otherwise it returns a copy independent of glyphs,
// even for left-to-right input. Nil input returns nil; empty input is valid.
// Invalid directions return nil and an error without modifying glyphs.
//
// G may be GlyphRecord, PositionedGlyph, or a client-defined glyph record. No
// fields are inspected: each complete record moves with its positions, cluster,
// and metadata. Copy mode copies records shallowly, not any referenced data.
// Advances and offsets are neither negated nor recomputed. Attachment diagnostic
// indexes retain their original shaping meaning; they are not remapped to visual
// indexes. Accumulate advances for rendering after this conversion.
//
// The caller must supply a complete run, or a line fragment selected by layout,
// in logical shaping order. Do not call this independently on arbitrary streaming
// chunks: reversing chunks does not reverse the complete run. This function does
// not resolve bidi levels, arrange multiple runs, choose line breaks, or validate
// run boundaries. Calling it again on RTL output reverses that output back.
func VisualOrder[G any](glyphs []G, direction bidi.Direction, inPlace bool) ([]G, error) {
	if direction != bidi.LeftToRight && direction != bidi.RightToLeft {
		return nil, fmt.Errorf("visual order: direction must be left-to-right or right-to-left")
	}
	if !inPlace {
		glyphs = slices.Clone(glyphs)
	}
	if direction == bidi.RightToLeft {
		slices.Reverse(glyphs)
	}
	return glyphs, nil
}
