package main

import (
	"fmt"
	"strings"

	"github.com/npillmayer/opentype/internal/proof"
	"github.com/npillmayer/opentype/otshape"

	"github.com/thatisuday/commando"
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
	input, err := readToolInput(args["text"].Value, flags)
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
	if width < 0 || height < 0 {
		fatalf("--width and --height must be >= 0 (zero selects automatic sizing)")
	}
	if glyphIndex < -1 {
		fatalf("--index must be >= 0 when selecting a glyph")
	}

	source := strings.NewReader(input)
	sink := &proof.Collector{}
	params := otshape.Params{
		Font:      otf,
		Direction: dir,
		Script:    script,
		Language:  lang,
		Features:  features,
	}
	options := parseBufferOptions(flags)
	err = proof.Shape(proof.IO{Source: source, Sink: sink}, params, options)
	if err != nil {
		fatalf("shape failed: %v", err)
	}
	if len(sink.Glyphs) == 0 {
		fatalf("shaping produced no glyphs")
	}
	if renderAll && glyphIndex >= 0 {
		fatalf("choose --all or --index, not both")
	}
	renderAll = glyphIndex < 0
	glyphs := sink.Glyphs
	if !renderAll {
		if glyphIndex >= len(glyphs) {
			fatalf("glyph index %d out of range (glyphs: %d)", glyphIndex, len(glyphs))
		}
		glyphs = glyphs[glyphIndex : glyphIndex+1]
	}
	sf, err := parseSFNT(fontPath)
	if err != nil {
		fatalf("%v", err)
	}
	opts := proof.Options{
		PPEM: ppem, Width: width, Height: height, Direction: dir,
		ShowBoxes:     showBBoxes,
		ShowPens:      mustFlagBool(flags["show-pens"], "show-pens"),
		ShowAdvances:  mustFlagBool(flags["show-advances"], "show-advances"),
		ColorClusters: mustFlagBool(flags["color-clusters"], "color-clusters"),
		ShowBaseline:  mustFlagBool(flags["show-baseline"], "show-baseline"),
		ShowOrigins:   mustFlagBool(flags["show-origins"], "show-origins"),
		ShowLabels:    mustFlagBool(flags["show-labels"], "show-labels"),
	}
	if mustFlagBool(flags["diagnostics"], "diagnostics") {
		opts.Diagnostics()
	}
	if !renderAll {
		opts.IndexBase = glyphIndex
	}
	img, err := proof.Render(sf, glyphs, opts)
	if err != nil {
		fatalf("render failed: %v", err)
	}
	if err := writeProofPNG(outPath, img); err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("wrote %s (glyphs=%d, size=%dx%d)\n", outPath, len(glyphs), img.Bounds().Dx(), img.Bounds().Dy())
}
