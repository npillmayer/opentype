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
	"sort"
	"strconv"
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otarabic"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
	"github.com/thatisuday/commando"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func main() {
	commando.
		SetExecutableName("ot-tools").
		SetVersion("v0.0.1").
		SetDescription("CLI for testing OpenType shaping and font diagnostics.")

	commando.
		Register(nil).
		AddFlag("verbose,V", "display additional output", commando.Bool, nil)

	commando.
		Register("shape").
		SetDescription("Shape text with a given OpenType font and print glyph stream output.").
		SetShortDescription("shape text").
		AddArgument("font", "OpenType font file path", "").
		AddArgument("text...", "text to shape (variadic argument parts joined by comma by commando)", "").
		AddFlag("script,s", "script (ISO 15924, e.g. Latn, Arab, Hebr)", commando.String, "Latn").
		AddFlag("lang,l", "language tag (BCP 47, e.g. en, ar, he)", commando.String, "en").
		AddFlag("direction,d", "direction: ltr|rtl", commando.String, "ltr").
		AddFlag("features,f", "feature list (e.g. liga=1,kern=0,+rlig,-calt)", commando.String, "-").
		AddFlag("codepoints,c", "codepoints instead of text (comma/space separated, e.g. U+0627,U+0644)", commando.String, "-").
		AddFlag("testfont,t", "parse font as relaxed test font fixture", commando.Bool, nil).
		AddFlag("flush", "flush mode: run|cluster", commando.String, "run").
		AddFlag("high-watermark", "stream high watermark (0 uses default)", commando.Int, 0).
		AddFlag("low-watermark", "stream low watermark (0 uses default)", commando.Int, 0).
		AddFlag("max-buffer", "stream max buffer (0 uses default)", commando.Int, 0).
		SetAction(runShapeCommand)

	commando.
		Register("view").
		SetDescription("Render a shaped glyph to a PNG image.").
		SetShortDescription("shape to image").
		AddArgument("font", "OpenType font file path", "").
		AddArgument("text...", "text to shape before rendering one glyph", "").
		AddFlag("script,s", "script (ISO 15924, e.g. Latn, Arab, Hebr)", commando.String, "Latn").
		AddFlag("lang,l", "language tag (BCP 47, e.g. en, ar, he)", commando.String, "en").
		AddFlag("direction,d", "direction: ltr|rtl", commando.String, "ltr").
		AddFlag("features,f", "feature list (e.g. liga=1,kern=0,+rlig,-calt)", commando.String, "-").
		AddFlag("codepoints,c", "codepoints instead of text (comma/space separated, e.g. U+0627,U+0644)", commando.String, "-").
		AddFlag("testfont,t", "parse font as relaxed test font fixture", commando.Bool, nil).
		AddFlag("output,o", "output PNG file", commando.String, "ot-tools-view.png").
		AddFlag("index,i", "glyph index in shaped output (0-based)", commando.Int, 0).
		AddFlag("all,a", "render all shaped glyphs instead of only --index", commando.Bool, nil).
		AddFlag("show-bboxes,B", "draw red bounding-box outlines per rendered glyph", commando.Bool, nil).
		AddFlag("ppem,p", "render scale in pixels-per-em", commando.Int, 96).
		AddFlag("width,W", "image width in pixels", commando.Int, 320).
		AddFlag("height,H", "image height in pixels", commando.Int, 240).
		SetAction(runViewCommand)

	commando.
		Register("font").
		SetDescription("Print diagnostics and table information for an OpenType font.").
		SetShortDescription("font diagnostics").
		AddArgument("font", "OpenType font file path", "").
		AddArgument("tables...", "optional list of table tags (e.g. GSUB,GPOS,head)", "").
		AddFlag("testfont,t", "parse font as relaxed test font fixture", commando.Bool, nil).
		AddFlag("errors,e", "print parse errors and warnings", commando.Bool, nil).
		SetAction(runFontCommand)

	commando.Parse(nil)
}

func runShapeCommand(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	fontPath := strings.TrimSpace(args["font"].Value)
	if fontPath == "" {
		fatalf("font path is required")
	}
	otf := mustLoadFont(fontPath, mustFlagBool(flags["testfont"], "testfont"))

	script, err := parseScript(flags["script"])
	if err != nil {
		fatalf("%v", err)
	}
	lang, err := parseLanguage(flags["lang"])
	if err != nil {
		fatalf("%v", err)
	}
	dir, err := parseDirection(flags["direction"])
	if err != nil {
		fatalf("%v", err)
	}
	flush, err := parseFlushMode(flags["flush"])
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

	sink := &glyphCollector{}
	req := otshape.ShapeRequest{
		Options: otshape.ShapeOptions{
			Params: otshape.Params{
				Font:      otf,
				Direction: dir,
				Script:    script,
				Language:  lang,
				Features:  features,
			},
			FlushBoundary: flush,
			HighWatermark: mustFlagInt(flags["high-watermark"], "high-watermark"),
			LowWatermark:  mustFlagInt(flags["low-watermark"], "low-watermark"),
			MaxBuffer:     mustFlagInt(flags["max-buffer"], "max-buffer"),
		},
		Source: strings.NewReader(input),
		Sink:   sink,
		Shapers: []otshape.ShapingEngine{
			otarabic.New(),
			othebrew.New(),
			otcore.New(),
		},
	}
	if err := otshape.Shape(req); err != nil {
		fatalf("shape failed: %v", err)
	}
	fmt.Println(formatGlyphOutput(sink.glyphs))
}

func runFontCommand(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	fontPath := strings.TrimSpace(args["font"].Value)
	if fontPath == "" {
		fatalf("font path is required")
	}
	otf := mustLoadFont(fontPath, mustFlagBool(flags["testfont"], "testfont"))

	fmt.Printf("Path: %s\n", fontPath)
	fmt.Printf("Type: %s\n", otquery.FontType(otf))
	names := otquery.NameInfo(otf, 0)
	if family := names["family"]; family != "" {
		fmt.Printf("Family: %s\n", family)
	}
	if sub := names["subfamily"]; sub != "" {
		fmt.Printf("Subfamily: %s\n", sub)
	}
	if version := names["version"]; version != "" {
		fmt.Printf("Version: %s\n", version)
	}

	tags := otf.TableTags()
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	fmt.Printf("Tables (%d):", len(tags))
	for _, tag := range tags {
		fmt.Printf(" %s", tag.String())
	}
	fmt.Println()

	layoutTables := otquery.LayoutTables(otf)
	sort.Strings(layoutTables)
	fmt.Printf("Layout: %s\n", strings.Join(layoutTables, ","))

	errs := otf.Errors()
	warns := otf.Warnings()
	crit := otf.CriticalErrors()
	fmt.Printf("Issues: errors=%d warnings=%d critical=%d\n", len(errs), len(warns), len(crit))

	if len(args["tables"].Value) > 0 {
		printSelectedTables(otf, args["tables"].Value)
	}
	showIssues, err := flags["errors"].GetBool()
	if err != nil {
		fatalf("invalid --errors flag: %v", err)
	}
	if showIssues {
		for _, e := range errs {
			fmt.Printf("error: %s\n", e.Error())
		}
		for _, w := range warns {
			fmt.Printf("warning: %s\n", w.String())
		}
	}
}

func runViewCommand(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	fontPath := strings.TrimSpace(args["font"].Value)
	if fontPath == "" {
		fatalf("font path is required")
	}
	otf := mustLoadFont(fontPath, mustFlagBool(flags["testfont"], "testfont"))

	script, err := parseScript(flags["script"])
	if err != nil {
		fatalf("%v", err)
	}
	lang, err := parseLanguage(flags["lang"])
	if err != nil {
		fatalf("%v", err)
	}
	dir, err := parseDirection(flags["direction"])
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

	sink := &glyphCollector{}
	req := otshape.ShapeRequest{
		Options: otshape.ShapeOptions{
			Params: otshape.Params{
				Font:      otf,
				Direction: dir,
				Script:    script,
				Language:  lang,
				Features:  features,
			},
			FlushBoundary: otshape.FlushOnRunBoundary,
		},
		Source: strings.NewReader(input),
		Sink:   sink,
		Shapers: []otshape.ShapingEngine{
			otarabic.New(),
			othebrew.New(),
			otcore.New(),
		},
	}
	if err := otshape.Shape(req); err != nil {
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

func parseShapeInput(textArg commando.ArgValue, cpFlag commando.FlagValue) (string, error) {
	cp, err := cpFlag.GetString()
	if err != nil {
		return "", fmt.Errorf("invalid --codepoints flag: %w", err)
	}
	cp = strings.TrimSpace(cp)
	if cp == "-" {
		cp = ""
	}
	if cp != "" {
		runes, err := parseCodepoints(cp)
		if err != nil {
			return "", err
		}
		return string(runes), nil
	}
	return textArg.Value, nil
}

func parseScript(flag commando.FlagValue) (language.Script, error) {
	s, err := flag.GetString()
	if err != nil {
		return language.Script{}, fmt.Errorf("invalid --script flag: %w", err)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = "Latn"
	}
	scr, err := language.ParseScript(s)
	if err != nil {
		return language.Script{}, fmt.Errorf("invalid script %q: %w", s, err)
	}
	return scr, nil
}

func parseLanguage(flag commando.FlagValue) (language.Tag, error) {
	s, err := flag.GetString()
	if err != nil {
		return language.Und, fmt.Errorf("invalid --lang flag: %w", err)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = "en"
	}
	tag, err := language.Parse(s)
	if err != nil {
		return language.Und, fmt.Errorf("invalid language tag %q: %w", s, err)
	}
	return tag, nil
}

func parseDirection(flag commando.FlagValue) (bidi.Direction, error) {
	s, err := flag.GetString()
	if err != nil {
		return bidi.LeftToRight, fmt.Errorf("invalid --direction flag: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "ltr", "left-to-right":
		return bidi.LeftToRight, nil
	case "rtl", "right-to-left":
		return bidi.RightToLeft, nil
	default:
		return bidi.LeftToRight, fmt.Errorf("unsupported direction %q (expected ltr|rtl)", s)
	}
}

func parseFlushMode(flag commando.FlagValue) (otshape.FlushBoundary, error) {
	s, err := flag.GetString()
	if err != nil {
		return otshape.FlushOnRunBoundary, fmt.Errorf("invalid --flush flag: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "run":
		return otshape.FlushOnRunBoundary, nil
	case "cluster":
		return otshape.FlushOnClusterBoundary, nil
	default:
		return otshape.FlushOnRunBoundary, fmt.Errorf("unsupported flush mode %q (expected run|cluster)", s)
	}
}

func parseFeatureList(flag commando.FlagValue) ([]otshape.FeatureRange, error) {
	spec, err := flag.GetString()
	if err != nil {
		return nil, fmt.Errorf("invalid --features flag: %w", err)
	}
	spec = strings.TrimSpace(spec)
	if spec == "-" {
		spec = ""
	}
	if spec == "" {
		return nil, nil
	}
	parts := splitCSVSpace(spec)
	out := make([]otshape.FeatureRange, 0, len(parts))
	for _, p := range parts {
		f, err := parseFeatureItem(p)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func parseFeatureItem(item string) (otshape.FeatureRange, error) {
	item = strings.TrimSpace(item)
	if item == "" {
		return otshape.FeatureRange{}, errors.New("empty feature entry in --features")
	}
	on := true
	if strings.HasPrefix(item, "+") {
		item = strings.TrimPrefix(item, "+")
		on = true
	} else if strings.HasPrefix(item, "-") {
		item = strings.TrimPrefix(item, "-")
		on = false
	}
	tagPart := item
	arg := 1
	if eq := strings.IndexByte(item, '='); eq >= 0 {
		tagPart = item[:eq]
		v := strings.TrimSpace(item[eq+1:])
		if v == "" {
			return otshape.FeatureRange{}, fmt.Errorf("empty feature value in %q", item)
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return otshape.FeatureRange{}, fmt.Errorf("invalid feature value in %q: %w", item, err)
		}
		arg = n
		on = n != 0
	}
	tagPart = strings.TrimSpace(tagPart)
	if len(tagPart) != 4 {
		return otshape.FeatureRange{}, fmt.Errorf("feature tag %q is not 4 characters", tagPart)
	}
	return otshape.FeatureRange{
		Feature: ot.T(tagPart),
		Arg:     arg,
		On:      on,
	}, nil
}

func parseCodepoints(spec string) ([]rune, error) {
	parts := splitCSVSpace(spec)
	out := make([]rune, 0, len(parts))
	for _, p := range parts {
		r, err := parseCodepointToken(p)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parseCodepointToken(token string) (rune, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, errors.New("empty codepoint token")
	}
	hex := token
	switch {
	case strings.HasPrefix(hex, "U+"), strings.HasPrefix(hex, "u+"):
		hex = hex[2:]
	case strings.HasPrefix(hex, "0x"), strings.HasPrefix(hex, "0X"):
		hex = hex[2:]
	}
	u, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid codepoint %q: %w", token, err)
	}
	r := rune(u)
	if !strconv.IsPrint(r) && r > 0x7f {
		// allow non-printing non-ASCII codepoints (marks/control-like shaping tests)
		return r, nil
	}
	return r, nil
}

func splitCSVSpace(spec string) []string {
	return strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
}

func printSelectedTables(otf *ot.Font, raw string) {
	requested := splitCSVSpace(raw)
	for _, t := range requested {
		tagName := strings.TrimSpace(t)
		if tagName == "" {
			continue
		}
		tag := ot.T(tagName)
		table := otf.Table(tag)
		if table == nil {
			fmt.Printf("table %s: missing\n", tagName)
			continue
		}
		off, size := table.Extent()
		fmt.Printf("table %s: offset=%d size=%d\n", tagName, off, size)
	}
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

func parseSFNT(fontPath string) (*sfnt.Font, error) {
	data, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read font for rasterization: %w", err)
	}
	sf, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse sfnt font for rasterization: %w", err)
	}
	return sf, nil
}

func mustLoadFont(path string, testfont bool) *ot.Font {
	b, err := os.ReadFile(path)
	if err != nil {
		fatalf("cannot read font %s: %v", path, err)
	}
	var otf *ot.Font
	if testfont {
		otf, err = ot.Parse(b, ot.IsTestfont)
	} else {
		otf, err = ot.Parse(b)
	}
	if err != nil {
		fatalf("cannot parse font %s: %v", path, err)
	}
	return otf
}

func mustFlagInt(flag commando.FlagValue, name string) int {
	n, err := flag.GetInt()
	if err != nil {
		fatalf("invalid --%s flag: %v", name, err)
	}
	return n
}

func mustFlagBool(flag commando.FlagValue, name string) bool {
	b, err := flag.GetBool()
	if err != nil {
		fatalf("invalid --%s flag: %v", name, err)
	}
	return b
}

func fatalf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, "ot-tools: "+format+"\n", args...)
	os.Exit(1)
}

type glyphCollector struct {
	glyphs []otshape.GlyphRecord
}

func (c *glyphCollector) WriteGlyph(g otshape.GlyphRecord) error {
	c.glyphs = append(c.glyphs, g)
	return nil
}

type stringGlyphSource struct {
	b strings.Builder
}

func (s *stringGlyphSource) WriteGlyph(g otshape.GlyphRecord) error {
	if s.b.Len() > 0 {
		s.b.WriteString("|")
	}
	part := fmt.Sprintf("%d=%d+%d", g.GID, g.Cluster, g.Pos.XAdvance)
	if g.Pos.YAdvance != 0 {
		part = fmt.Sprintf("%s,%d", part, g.Pos.YAdvance)
	}
	if g.Pos.XOffset != 0 || g.Pos.YOffset != 0 {
		part = fmt.Sprintf("%s@%d,%d", part, g.Pos.XOffset, g.Pos.YOffset)
	}
	s.b.WriteString(part)
	return nil
}

func formatGlyphOutput(glyphs []otshape.GlyphRecord) string {
	sink := &stringGlyphSource{}
	for _, g := range glyphs {
		_ = sink.WriteGlyph(g)
	}
	return "[" + sink.b.String() + "]"
}
