package otquery

import "golang.org/x/image/font/sfnt"

// FontMetricsInfo contains selected metric information for a font.
type FontMetricsInfo struct {
	UnitsPerEm      sfnt.Units // ad-hoc units per em
	Ascent, Descent sfnt.Units // ascender and descender
	MaxAdvance      sfnt.Units // maximum advance width value in 'hmtx' table
	LineGap         sfnt.Units // typographic line gap
}

// GlyphMetricsInfo contains all metric information for a glyph.
type GlyphMetricsInfo struct {
	Advance  sfnt.Units  // advance width
	LSB, RSB sfnt.Units  // side bearings
	BBox     BoundingBox // bounding box
}

// BoundingBox describes the bounding box of a glyph.
type BoundingBox struct {
	MinX, MinY sfnt.Units
	MaxX, MaxY sfnt.Units
}

// IsEmpty reports whether this box has zero area.
func (bbox BoundingBox) IsEmpty() bool {
	return bbox.MaxX-bbox.MinX == 0 || bbox.MaxY-bbox.MinY == 0
}

// Dx returns the horizontal extent of this box.
func (bbox BoundingBox) Dx() sfnt.Units {
	return bbox.MaxX - bbox.MinX
}

// Dy returns the vertical extent of this box.
func (bbox BoundingBox) Dy() sfnt.Units {
	return bbox.MaxY - bbox.MinY
}
