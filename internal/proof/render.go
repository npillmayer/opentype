package proof

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/npillmayer/opentype/otshape"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
	"golang.org/x/text/unicode/bidi"
)

// This layer consumes final glyph records only. It never maps or reshapes text.
// Layout, image drawing, and file output are separate so later front ends can
// reuse the same placement contract without invoking command-line actions.
type Options struct {
	PPEM, Width, Height                              int
	Direction                                        bidi.Direction
	ShowBoxes, ShowBaseline, ShowOrigins, ShowLabels bool
	ShowPens, ShowAdvances, ColorClusters            bool
	IndexBase                                        int
}

const (
	proofPadding     = 24
	maxProofPixels   = 16 * 1024 * 1024
	maxProofPPEM     = 4096
	legendLineHeight = 16
)

func (o *Options) Diagnostics() {
	o.ShowBoxes, o.ShowBaseline, o.ShowOrigins, o.ShowLabels = true, true, true, true
	o.ShowPens, o.ShowAdvances, o.ColorClusters = true, true, true
}

type proofGlyph struct {
	record                   otshape.GlyphRecord
	logicalIndex             int
	segments                 sfnt.Segments
	x, y                     float64 // positioned origin in pixels, Y downward
	penX, penY, nextX, nextY float64
}

type proofLayout struct {
	glyphs                 []proofGlyph // visual order, left to right for horizontal runs
	minX, minY, maxX, maxY float64
	advanceX               float64
}

func layoutProof(sf *sfnt.Font, glyphs []otshape.GlyphRecord, opts Options) (proofLayout, error) {
	var layout proofLayout
	if sf == nil || sf.UnitsPerEm() == 0 {
		return layout, fmt.Errorf("invalid font")
	}
	if opts.PPEM < 1 || opts.PPEM > maxProofPPEM {
		return layout, fmt.Errorf("--ppem must be between 1 and %d", maxProofPPEM)
	}
	if opts.Direction != bidi.LeftToRight && opts.Direction != bidi.RightToLeft {
		return layout, fmt.Errorf("proof rendering requires ltr or rtl direction")
	}
	var buf sfnt.Buffer
	metrics, err := sf.Metrics(&buf, fixed.I(opts.PPEM), font.HintingNone)
	if err != nil {
		return layout, fmt.Errorf("cannot read font metrics: %w", err)
	}
	// Include the baseline, typographic ascent/descent, and pen extents. Spaces
	// and empty outlines remain measurable even when there is no visible ink.
	layout.minY = -float64(metrics.Ascent) / 64
	layout.maxY = float64(metrics.Descent) / 64
	scale := float64(opts.PPEM) / float64(sf.UnitsPerEm())
	var penX, penY float64
	for visualIndex := range glyphs {
		i := visualIndex
		if opts.Direction == bidi.RightToLeft {
			i = len(glyphs) - 1 - visualIndex
		}
		gr := glyphs[i]
		segs, err := sf.LoadGlyph(&buf, sfnt.GlyphIndex(gr.GID), fixed.I(opts.PPEM), nil)
		if err != nil {
			return proofLayout{}, fmt.Errorf("cannot load glyph[%d] gid=%d: %w", opts.IndexBase+i, gr.GID, err)
		}
		// LoadGlyph's storage belongs to buf and is invalidated by its next use.
		pg := proofGlyph{record: gr, logicalIndex: opts.IndexBase + i,
			segments: append(sfnt.Segments(nil), segs...),
			penX:     penX, penY: penY,
			nextX: penX + float64(gr.Pos.XAdvance)*scale,
			nextY: penY - float64(gr.Pos.YAdvance)*scale,
			x:     penX + float64(gr.Pos.XOffset)*scale,
			y:     penY - float64(gr.Pos.YOffset)*scale,
		}
		layout.glyphs = append(layout.glyphs, pg)
		if len(segs) != 0 {
			b := segs.Bounds()
			layout.include(pg.x+float64(b.Min.X)/64, pg.y+float64(b.Min.Y)/64)
			layout.include(pg.x+float64(b.Max.X)/64, pg.y+float64(b.Max.Y)/64)
		}
		if opts.ShowOrigins || opts.ShowLabels {
			layout.include(pg.x-4, pg.y-4)
			layout.include(pg.x+4, pg.y+4)
		}
		if opts.ShowLabels {
			label := fmt.Sprintf("%d", pg.logicalIndex)
			layout.include(pg.x, pg.y+7)
			layout.include(pg.x+float64(font.MeasureString(basicfont.Face7x13, label))/64, pg.y+23)
		}
		if opts.ShowPens {
			layout.include(pg.penX-4, pg.penY-5)
			layout.include(pg.penX+4, pg.penY+5)
		}
		if opts.ShowAdvances {
			layout.include(pg.penX-4, pg.penY+28)
			layout.include(pg.nextX+4, pg.nextY+38)
			label := fmt.Sprint(gr.Pos.XAdvance)
			labelW := float64(font.MeasureString(basicfont.Face7x13, label)) / 64
			mid := (pg.penX + pg.nextX) / 2
			layout.include(mid-labelW/2, pg.penY+35)
			layout.include(mid+labelW/2, pg.penY+52)
		}
		// Public output already includes nominal advances. Never query and add
		// font advances again, including for spaces or zero-advance marks.
		penX += float64(gr.Pos.XAdvance) * scale
		penY -= float64(gr.Pos.YAdvance) * scale
		layout.include(penX, penY)
	}
	layout.advanceX = penX
	return layout, nil
}

func (p *proofLayout) include(x, y float64) {
	p.minX, p.maxX = math.Min(p.minX, x), math.Max(p.maxX, x)
	p.minY, p.maxY = math.Min(p.minY, y), math.Max(p.maxY, y)
}

func glyphLegend(g proofGlyph) string {
	p := g.record.Pos
	return fmt.Sprintf("%d: gid=%d cl=%d adv=(%d,%d) off=(%d,%d)", g.logicalIndex, g.record.GID, g.record.Cluster, p.XAdvance, p.YAdvance, p.XOffset, p.YOffset)
}

func Render(sf *sfnt.Font, glyphs []otshape.GlyphRecord, opts Options) (*image.RGBA, error) {
	if opts.Width < 0 || opts.Height < 0 {
		return nil, fmt.Errorf("canvas dimensions must be nonnegative")
	}
	layout, err := layoutProof(sf, glyphs, opts)
	if err != nil {
		return nil, err
	}
	// Dimensions are checked as floats before conversion/allocation, including
	// extreme advances supplied by a caller or font. This is a PNG allocation
	// budget, separate from the shaper's buffering contract.
	contentW := math.Ceil(layout.maxX - layout.minX)
	contentH := math.Ceil(layout.maxY - layout.minY)
	legendH := 0
	var legends []string
	if opts.ShowLabels {
		legends = append(legends, "logical index: glyph ID, scalar cluster, advance and offset in font units")
		// Legend stays in logical order even though paths are visually ordered.
		for i, gr := range glyphs {
			legends = append(legends, glyphLegend(proofGlyph{record: gr, logicalIndex: opts.IndexBase + i}))
		}
		legendH = (len(legends) + 1) * legendLineHeight
		for _, line := range legends {
			contentW = math.Max(contentW, float64(font.MeasureString(basicfont.Face7x13, line))/64)
		}
	}
	needW, needH := contentW+2*proofPadding, contentH+2*proofPadding+float64(legendH)
	w, h := float64(opts.Width), float64(opts.Height)
	if w == 0 {
		w = needW
	}
	if h == 0 {
		h = needH
	}
	if w < needW || h < needH {
		return nil, fmt.Errorf("canvas too small: need at least %.0fx%.0f pixels; use automatic dimensions or reduce --ppem", needW, needH)
	}
	if w > maxProofPixels || h > maxProofPixels || w*h > maxProofPixels {
		return nil, fmt.Errorf("proof exceeds %d-pixel image budget; reduce --ppem or input length", maxProofPixels)
	}
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)
	tx := (w-(layout.maxX-layout.minX))/2 - layout.minX
	ty := (h-float64(legendH)-(layout.maxY-layout.minY))/2 - layout.minY
	drawProofLayout(img, layout, opts, tx, ty)
	for i, line := range legends {
		drawProofLabel(img, proofPadding, int(h)-proofPadding-legendH+(i+1)*legendLineHeight, line, color.Black)
	}
	return img, nil
}

func drawProofLayout(img *image.RGBA, layout proofLayout, opts Options, tx, ty float64) {
	blue := color.RGBA{0, 100, 200, 255}
	red := color.RGBA{200, 40, 40, 255}
	if opts.ShowBaseline {
		y := int(math.Round(ty))
		draw.Draw(img, image.Rect(proofPadding, y, img.Bounds().Dx()-proofPadding, y+1), image.NewUniform(blue), image.Point{}, draw.Src)
	}
	var rast vector.Rasterizer
	for _, pg := range layout.glyphs {
		// Rasterize each glyph independently: overlapping contours from
		// different glyphs must not cancel through nonzero winding rules.
		if len(pg.segments) == 0 {
			continue
		}
		b := pg.segments.Bounds()
		box := image.Rect(
			int(math.Floor(tx+pg.x+float64(b.Min.X)/64)), int(math.Floor(ty+pg.y+float64(b.Min.Y)/64)),
			int(math.Ceil(tx+pg.x+float64(b.Max.X)/64)), int(math.Ceil(ty+pg.y+float64(b.Max.Y)/64)),
		)
		if box.Empty() {
			continue
		}
		rast.Reset(box.Dx(), box.Dy())
		rast.DrawOp = draw.Over
		x, y := float32(tx+pg.x-float64(box.Min.X)), float32(ty+pg.y-float64(box.Min.Y))
		for _, seg := range pg.segments {
			a := seg.Args
			switch seg.Op {
			case sfnt.SegmentOpMoveTo:
				rast.MoveTo(x+float32(a[0].X)/64, y+float32(a[0].Y)/64)
			case sfnt.SegmentOpLineTo:
				rast.LineTo(x+float32(a[0].X)/64, y+float32(a[0].Y)/64)
			case sfnt.SegmentOpQuadTo:
				rast.QuadTo(x+float32(a[0].X)/64, y+float32(a[0].Y)/64, x+float32(a[1].X)/64, y+float32(a[1].Y)/64)
			case sfnt.SegmentOpCubeTo:
				rast.CubeTo(x+float32(a[0].X)/64, y+float32(a[0].Y)/64, x+float32(a[1].X)/64, y+float32(a[1].Y)/64, x+float32(a[2].X)/64, y+float32(a[2].Y)/64)
			}
		}
		ink := image.Black
		if opts.ColorClusters {
			ink = image.NewUniform(clusterColor(pg.record.Cluster))
		}
		rast.Draw(img, box, ink, image.Point{})
	}
	for _, pg := range layout.glyphs {
		x, y := tx+pg.x, ty+pg.y
		if opts.ShowBoxes && len(pg.segments) != 0 {
			b := pg.segments.Bounds()
			drawProofRect(img, image.Rect(int(math.Floor(x+float64(b.Min.X)/64)), int(math.Floor(y+float64(b.Min.Y)/64)), int(math.Ceil(x+float64(b.Max.X)/64)), int(math.Ceil(y+float64(b.Max.Y)/64))), red)
		}
		orange := color.RGBA{190, 90, 0, 255}
		if opts.ShowPens {
			px, py := int(math.Round(tx+pg.penX)), int(math.Round(ty+pg.penY))
			drawProofRect(img, image.Rect(px-3, py-4, px+4, py+5), orange)
		}
		if opts.ShowAdvances {
			x0, y0 := int(math.Round(tx+pg.penX)), int(math.Round(ty+pg.penY))+33
			x1, y1 := int(math.Round(tx+pg.nextX)), int(math.Round(ty+pg.nextY))+33
			drawProofLine(img, x0, y0, x1, y1, orange)
			drawProofLine(img, x0, y0-3, x0, y0+3, orange)
			drawProofLine(img, x1, y1-3, x1, y1+3, orange)
			// Zero-advance marks already have an endpoint tick and a zero in
			// the glyph legend. Repeating width labels at the same pen makes
			// them run into the adjacent base's width.
			if pg.record.Pos.XAdvance != 0 {
				label := fmt.Sprint(pg.record.Pos.XAdvance)
				lw := font.MeasureString(basicfont.Face7x13, label).Ceil()
				drawProofLabel(img, (x0+x1-lw)/2, y0+16, label, orange)
			}
		}
		if opts.ShowOrigins {
			px, py := int(math.Round(x)), int(math.Round(y))
			draw.Draw(img, image.Rect(px-3, py, px+4, py+1), image.NewUniform(blue), image.Point{}, draw.Src)
			draw.Draw(img, image.Rect(px, py-3, px+1, py+4), image.NewUniform(blue), image.Point{}, draw.Src)
		}
		if opts.ShowLabels {
			drawProofLabel(img, int(math.Round(x)), int(math.Round(y))+20, fmt.Sprint(pg.logicalIndex), blue)
		}
	}

}

func clusterColor(cluster uint32) color.RGBA {
	palette := [...]color.RGBA{{35, 75, 160, 255}, {145, 40, 100, 255}, {0, 115, 85, 255}, {155, 75, 0, 255}, {85, 55, 150, 255}, {15, 105, 135, 255}}
	return palette[cluster%uint32(len(palette))]
}

func drawProofLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	e := dx + dy
	for {
		img.Set(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func drawProofLabel(img *image.RGBA, x, y int, s string, c color.Color) {
	// Only diagnostic ASCII labels use font.Drawer; shaped text uses glyph IDs.
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

func drawProofRect(img *image.RGBA, r image.Rectangle, c color.Color) {
	src := image.NewUniform(c)
	for _, edge := range []image.Rectangle{
		image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y),
		image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y),
	} {
		draw.Draw(img, edge, src, image.Point{}, draw.Src)
	}
}
