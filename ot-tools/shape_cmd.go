package main

import (
	"fmt"
	"strings"

	"github.com/npillmayer/opentype/internal/proof"
	"github.com/npillmayer/opentype/otshape"

	"github.com/thatisuday/commando"
)

func runShapeCommand(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	fontPath := strings.TrimSpace(args["font"].Value)
	if fontPath == "" {
		fatalf("font path argument is required")
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

	source := strings.NewReader(input)
	sink := &proof.Collector{}
	params := otshape.Params{
		Font:      otf,
		Direction: dir,
		Script:    script,
		Language:  lang,
		Features:  features,
	}
	bufOpts := parseBufferOptions(flags)
	if err := proof.Shape(proof.IO{Source: source, Sink: sink}, params, bufOpts); err != nil {
		fatalf("%v", err)
	}
	fmt.Println(formatGlyphOutput(sink.Glyphs))
}

// ---Parsing flags and arguments ---------------------------------------

func parseBufferOptions(flags map[string]commando.FlagValue) otshape.BufferOptions {
	flush, err := parseFlushMode(flags["flush"])
	if err != nil {
		fatalf("%v", err)
	}
	return otshape.BufferOptions{
		FlushBoundary: flush,
		HighWatermark: mustFlagInt(flags["high-watermark"], "high-watermark"),
		LowWatermark:  mustFlagInt(flags["low-watermark"], "low-watermark"),
		MaxBuffer:     mustFlagInt(flags["max-buffer"], "max-buffer"),
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
	parts := proof.SplitCSVSpace(spec)
	out := make([]otshape.FeatureRange, 0, len(parts))
	for _, p := range parts {
		f, err := proof.ParseFeatureItem(p)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
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
