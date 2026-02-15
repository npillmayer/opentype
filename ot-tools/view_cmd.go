package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otarabic"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
	"github.com/thatisuday/commando"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

func runViewCommand(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	fontPath := strings.TrimSpace(args["font"].Value)
	if fontPath == "" {
		fatalf("font path is required")
	}
	otf := mustLoadFont(fontPath, mustFlagBool(flags["testfont"], "testfont"))

	script, lang, dir, err := parseTypesetFlags(flags)
	if err != nil {
		fatalf("%v", err)
	}
	features, err := parseFeatureList(flags["features"])
	if err != nil {
		fatalf("%v", err)
	}
	input, err := parseShapeInput(args["text"], flags["codepoints"])
	if err != nil {
		fatalf("%v", err)
	}
	if input == "" {
		fatalf("input text is empty")
	}
	outPath, err := flags["output"].GetString()
	if err != nil {
		fatalf("invalid --output flag: %v", err)
	}
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		fatalf("output path is empty")
	}
	ppem := mustFlagInt(flags["ppem"], "ppem")
	width := mustFlagInt(flags["width"], "width")
	height := mustFlagInt(flags["height"], "height")
	glyphIndex := mustFlagInt(flags["index"], "index")
	renderAll := mustFlagBool(flags["all"], "all")
	showBBoxes := mustFlagBool(flags["show-bboxes"], "show-bboxes")
	if ppem <= 0 {
		fatalf("--ppem must be > 0")
	}
	if width <= 0 || height <= 0 {
		fatalf("--width and --height must be > 0")
	}
	if glyphIndex < 0 {
		fatalf("--index must be >= 0")
	}

	source := strings.NewReader(input)
	sink := &glyphCollector{}
	params := otshape.Params{
		Font:      otf,
		Direction: dir,
		Script:    script,
		Language:  lang,
		Features:  features,
	}
	options := otshape.BufferOptions{
		FlushBoundary: otshape.FlushOnRunBoundary,
	}
	engines := []otshape.ShapingEngine{
		otarabic.New(),
		othebrew.New(),
		otcore.New(),
	}
	shaper := otshape.NewShaper(engines...)
	err = shaper.Shape(params, source, sink, options)
	if err != nil {
		fatalf("shape failed: %v", err)
	}
	if len(sink.glyphs) == 0 {
		fatalf("shaping produced no glyphs")
	}
	if renderAll {
		if err := renderGlyphRunPNG(fontPath, sink.glyphs, outPath, width, height, ppem, showBBoxes); err != nil {
			fatalf("render failed: %v", err)
		}
		fmt.Printf("wrote %s (glyphs=%d)\n", outPath, len(sink.glyphs))
		return
	}
	if glyphIndex >= len(sink.glyphs) {
		fatalf("glyph index %d out of range (glyphs: %d)", glyphIndex, len(sink.glyphs))
	}
	if err := renderGlyphPNG(fontPath, sink.glyphs[glyphIndex], outPath, width, height, ppem, showBBoxes); err != nil {
		fatalf("render failed: %v", err)
	}
	gr := sink.glyphs[glyphIndex]
	fmt.Printf("wrote %s (glyph[%d]=%d, cluster=%d)\n", outPath, glyphIndex, gr.GID, gr.Cluster)
}

func renderGlyphPNG(fontPath string, g otshape.GlyphRecord, outPath string, width int, height int, ppem int, showBBoxes bool) error {
	sf, err := parseSFNT(fontPath)
	if err != nil {
		return err
	}
	var buf sfnt.Buffer
	segs, err := sf.LoadGlyph(&buf, sfnt.GlyphIndex(g.GID), fixed.I(ppem), nil)
	if err != nil {
		return fmt.Errorf("cannot load glyph %d: %w", g.GID, err)
	}

	upem := float32(sf.UnitsPerEm())
	if upem <= 0 {
		return errors.New("invalid units-per-em")
	}
	xOffset := float32(g.Pos.XOffset) * float32(ppem) / upem
	// Positive yOffset in OT is upward; image Y grows downward.
	yOffset := -float32(g.Pos.YOffset) * float32(ppem) / upem

	bounds := segs.Bounds()
	glyphCenterX := (float32(bounds.Min.X) + float32(bounds.Max.X)) / 128
	glyphCenterY := (float32(bounds.Min.Y) + float32(bounds.Max.Y)) / 128
	tx := float32(width)/2 - glyphCenterX + xOffset
	ty := float32(height)/2 - glyphCenterY + yOffset

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{255, 255, 255, 255}), image.Point{}, draw.Src)

	rast := vector.NewRasterizer(width, height)
	rast.DrawOp = draw.Over
	for _, seg := range segs {
		switch seg.Op {
		case sfnt.SegmentOpMoveTo:
			rast.MoveTo(tx+float32(seg.Args[0].X)/64, ty+float32(seg.Args[0].Y)/64)
		case sfnt.SegmentOpLineTo:
			rast.LineTo(tx+float32(seg.Args[0].X)/64, ty+float32(seg.Args[0].Y)/64)
		case sfnt.SegmentOpQuadTo:
			rast.QuadTo(
				tx+float32(seg.Args[0].X)/64, ty+float32(seg.Args[0].Y)/64,
				tx+float32(seg.Args[1].X)/64, ty+float32(seg.Args[1].Y)/64,
			)
		case sfnt.SegmentOpCubeTo:
			rast.CubeTo(
				tx+float32(seg.Args[0].X)/64, ty+float32(seg.Args[0].Y)/64,
				tx+float32(seg.Args[1].X)/64, ty+float32(seg.Args[1].Y)/64,
				tx+float32(seg.Args[2].X)/64, ty+float32(seg.Args[2].Y)/64,
			)
		}
	}
	rast.Draw(img, img.Bounds(), image.Black, image.Point{})
	if showBBoxes {
		drawRectOutline(img, bounds.Min.X.Floor()+int(tx), bounds.Min.Y.Floor()+int(ty), bounds.Max.X.Ceil()+int(tx), bounds.Max.Y.Ceil()+int(ty), color.RGBA{255, 0, 0, 255})
	}

	if dir := filepath.Dir(outPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("cannot create output directory: %w", err)
		}
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("cannot encode png: %w", err)
	}
	return nil
}

func renderGlyphRunPNG(fontPath string, glyphs []otshape.GlyphRecord, outPath string, width int, height int, ppem int, showBBoxes bool) error {
	if len(glyphs) == 0 {
		return errors.New("empty glyph run")
	}
	sf, err := parseSFNT(fontPath)
	if err != nil {
		return err
	}
	upem := float32(sf.UnitsPerEm())
	if upem <= 0 {
		return errors.New("invalid units-per-em")
	}
	scale := float32(ppem) / upem

	type glyphPath struct {
		segs sfnt.Segments
		dx   float32
		dy   float32
		box  fixed.Rectangle26_6
	}
	paths := make([]glyphPath, 0, len(glyphs))
	var (
		penX float32
		penY float32
		minX float32
		minY float32
		maxX float32
		maxY float32
		have bool
		buf  sfnt.Buffer
	)
	for _, gr := range glyphs {
		gid := sfnt.GlyphIndex(gr.GID)
		segs, err := sf.LoadGlyph(&buf, gid, fixed.I(ppem), nil)
		if err != nil {
			continue
		}
		// sfnt.LoadGlyph results become invalid once the buffer is re-used.
		// Copy segments before the next sfnt call.
		segsCopy := append(sfnt.Segments(nil), segs...)
		dx := penX + float32(gr.Pos.XOffset)*scale
		dy := penY - float32(gr.Pos.YOffset)*scale
		paths = append(paths, glyphPath{segs: segsCopy, dx: dx, dy: dy, box: segsCopy.Bounds()})

		b := segsCopy.Bounds()
		sMinX := float32(b.Min.X)/64 + dx
		sMinY := float32(b.Min.Y)/64 + dy
		sMaxX := float32(b.Max.X)/64 + dx
		sMaxY := float32(b.Max.Y)/64 + dy
		if !have {
			minX, minY, maxX, maxY = sMinX, sMinY, sMaxX, sMaxY
			have = true
		} else {
			if sMinX < minX {
				minX = sMinX
			}
			if sMinY < minY {
				minY = sMinY
			}
			if sMaxX > maxX {
				maxX = sMaxX
			}
			if sMaxY > maxY {
				maxY = sMaxY
			}
		}
		nominalAdvance, err := sf.GlyphAdvance(&buf, gid, fixed.I(ppem), font.HintingNone)
		if err == nil {
			penX += float32(nominalAdvance) / 64
		}
		penX += float32(gr.Pos.XAdvance) * scale
		penY -= float32(gr.Pos.YAdvance) * scale
	}
	if len(paths) == 0 {
		return errors.New("no drawable glyph paths found")
	}

	unionW := maxX - minX
	unionH := maxY - minY
	shiftX := (float32(width)-unionW)/2 - minX
	shiftY := (float32(height)-unionH)/2 - minY

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{255, 255, 255, 255}), image.Point{}, draw.Src)

	rast := vector.NewRasterizer(width, height)
	rast.DrawOp = draw.Over
	for _, p := range paths {
		for _, seg := range p.segs {
			switch seg.Op {
			case sfnt.SegmentOpMoveTo:
				rast.MoveTo(shiftX+p.dx+float32(seg.Args[0].X)/64, shiftY+p.dy+float32(seg.Args[0].Y)/64)
			case sfnt.SegmentOpLineTo:
				rast.LineTo(shiftX+p.dx+float32(seg.Args[0].X)/64, shiftY+p.dy+float32(seg.Args[0].Y)/64)
			case sfnt.SegmentOpQuadTo:
				rast.QuadTo(
					shiftX+p.dx+float32(seg.Args[0].X)/64, shiftY+p.dy+float32(seg.Args[0].Y)/64,
					shiftX+p.dx+float32(seg.Args[1].X)/64, shiftY+p.dy+float32(seg.Args[1].Y)/64,
				)
			case sfnt.SegmentOpCubeTo:
				rast.CubeTo(
					shiftX+p.dx+float32(seg.Args[0].X)/64, shiftY+p.dy+float32(seg.Args[0].Y)/64,
					shiftX+p.dx+float32(seg.Args[1].X)/64, shiftY+p.dy+float32(seg.Args[1].Y)/64,
					shiftX+p.dx+float32(seg.Args[2].X)/64, shiftY+p.dy+float32(seg.Args[2].Y)/64,
				)
			}
		}
	}
	rast.Draw(img, img.Bounds(), image.Black, image.Point{})
	if showBBoxes {
		for _, p := range paths {
			minX := p.box.Min.X.Floor() + int(shiftX+p.dx)
			minY := p.box.Min.Y.Floor() + int(shiftY+p.dy)
			maxX := p.box.Max.X.Ceil() + int(shiftX+p.dx)
			maxY := p.box.Max.Y.Ceil() + int(shiftY+p.dy)
			drawRectOutline(img, minX, minY, maxX, maxY, color.RGBA{255, 0, 0, 255})
		}
	}

	if dir := filepath.Dir(outPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("cannot create output directory: %w", err)
		}
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("cannot encode png: %w", err)
	}
	return nil
}

func drawRectOutline(img *image.RGBA, minX int, minY int, maxX int, maxY int, c color.RGBA) {
	if img == nil {
		return
	}
	if maxX < minX {
		minX, maxX = maxX, minX
	}
	if maxY < minY {
		minY, maxY = maxY, minY
	}
	b := img.Bounds()
	if minX < b.Min.X {
		minX = b.Min.X
	}
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	if maxX > b.Max.X {
		maxX = b.Max.X
	}
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}
	if minX >= maxX || minY >= maxY {
		return
	}
	// top and bottom
	for x := minX; x < maxX; x++ {
		img.SetRGBA(x, minY, c)
		img.SetRGBA(x, maxY-1, c)
	}
	// left and right
	for y := minY; y < maxY; y++ {
		img.SetRGBA(minX, y, c)
		img.SetRGBA(maxX-1, y, c)
	}
}
