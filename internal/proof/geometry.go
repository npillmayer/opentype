package proof

import (
	"image"
	"sort"
	"unicode/utf8"
)

// Execution is one atomic observed result. Export Image, Report and Report.Case
// from this same execution, especially when random alternates are requested.
// Execute reads no files and neither writes output nor exits the process.
type Execution struct {
	Report   Result
	Image    *image.RGBA
	Geometry Geometry
}

// Geometry uses image pixels with Y downward, after comparison alignment.
// It comes from the very same layouts/translations used to draw Image.
type Geometry struct {
	Width  int         `json:"width"`
	Height int         `json:"height"`
	Rows   []ScreenRow `json:"rows"`
}
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type ScreenRow struct {
	Baseline float64       `json:"baseline"`
	Glyphs   []ScreenGlyph `json:"glyphs"` // logical order; never sorted by cluster
}
type ScreenGlyph struct {
	Index        int   `json:"index"`
	Ink          *Box  `json:"ink,omitempty"`
	Hit          Box   `json:"hit"`
	Pen          Point `json:"pen"`
	Origin       Point `json:"origin"`
	Next         Point `json:"next"`
	ClusterStart int   `json:"cluster_start"`
	ClusterEnd   int   `json:"cluster_end"`
}

func screenRow(l proofLayout, tx, ty float64) ScreenRow {
	row := ScreenRow{Baseline: ty, Glyphs: make([]ScreenGlyph, len(l.glyphs))}
	for _, g := range l.glyphs {
		p := ScreenGlyph{Index: g.logicalIndex, Pen: Point{tx + g.penX, ty + g.penY}, Origin: Point{tx + g.x, ty + g.y}, Next: Point{tx + g.nextX, ty + g.nextY}}
		// A minimum target around the origin keeps even empty zero-advance glyphs
		// selectable. For spaces use the advance span. Visible marks use ink bounds.
		p.Hit = Box{min(p.Pen.X, p.Next.X) - 4, p.Pen.Y - 12, max(8, absFloat(p.Next.X-p.Pen.X)+8), 24}
		if len(g.segments) > 0 {
			b := g.segments.Bounds()
			box := Box{p.Origin.X + float64(b.Min.X)/64, p.Origin.Y + float64(b.Min.Y)/64, float64(b.Max.X-b.Min.X) / 64, float64(b.Max.Y-b.Min.Y) / 64}
			p.Ink = &box
			p.Hit = Box{box.X - 3, box.Y - 3, max(8, box.Width+6), max(8, box.Height+6)}
		} else if g.penX == g.nextX {
			p.Hit = Box{p.Origin.X - 5, p.Origin.Y - 8, 10, 16}
		}
		row.Glyphs[g.logicalIndex] = p
	}
	return row
}
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ClusterSpans derives only cluster-level source correspondence: an anchor to
// the next greater output anchor (or input end). It cannot identify individual
// characters within ligatures, composed glyphs, or merged/deleted clusters.
// Sorting anchors here never changes the logical/visual glyph record order.
func ClusterSpans(anchors []uint32, scalarCount int) [][2]int {
	unique := append([]uint32(nil), anchors...)
	sort.Slice(unique, func(i, j int) bool { return unique[i] < unique[j] })
	out := make([][2]int, len(anchors))
	for i, a := range anchors {
		start := min(int(a), scalarCount)
		end := scalarCount
		next := sort.Search(len(unique), func(j int) bool { return unique[j] > a })
		if next < len(unique) {
			end = min(int(unique[next]), scalarCount)
		}
		out[i] = [2]int{start, end}
	}
	return out
}
func addClusterSpans(geometry *Geometry, report Result) {
	for i, row := range report.Rows {
		anchors := make([]uint32, len(row.Glyphs))
		for j, g := range row.Glyphs {
			anchors[j] = g.Cluster
		}
		spans := ClusterSpans(anchors, utf8.RuneCountInString(*report.Case.Text))
		for j, span := range spans {
			geometry.Rows[i].Glyphs[j].ClusterStart = span[0]
			geometry.Rows[i].Glyphs[j].ClusterEnd = span[1]
		}
	}
}
