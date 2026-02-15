package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otarabic"
	"github.com/npillmayer/opentype/otshape/otcore"
	"github.com/npillmayer/opentype/otshape/othebrew"
	"github.com/thatisuday/commando"
)

type IO struct {
	Source otshape.RuneSource
	Sink   otshape.GlyphSink
}

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

	source := strings.NewReader(input)
	sink := &glyphCollector{}
	params := otshape.Params{
		Font:      otf,
		Direction: dir,
		Script:    script,
		Language:  lang,
		Features:  features,
	}
	bufOpts := otshape.BufferOptions{
		FlushBoundary: flush,
		HighWatermark: mustFlagInt(flags["high-watermark"], "high-watermark"),
		LowWatermark:  mustFlagInt(flags["low-watermark"], "low-watermark"),
		MaxBuffer:     mustFlagInt(flags["max-buffer"], "max-buffer"),
	}
	if err := doShape(IO{source, sink}, params, bufOpts); err != nil {
		fatalf("%v", err)
	}
	fmt.Println(formatGlyphOutput(sink.glyphs))
}

func doShape(io IO, params otshape.Params, bufOpts otshape.BufferOptions) error {
	engines := []otshape.ShapingEngine{
		otcore.New(),
		otarabic.New(),
		othebrew.New(),
	}
	shaper := otshape.NewShaper(engines...)
	err := shaper.Shape(params, io.Source, io.Sink, bufOpts)
	return err
}

// ---Parsing flags and arguments ---------------------------------------

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

var noFeatures = otshape.FeatureRange{}

// We try to follow Harfbuzz's `hb-shape` features parameter syntax, which is
// unfortunately not well documented.
//
// Disable ligatures:      --features="-liga"
// or                      --features="liga=0"
//
// Feature ranges are not supported yet ("liga[3:5]").
func parseFeatureItem(item string) (otshape.FeatureRange, error) {
	if item = strings.TrimSpace(item); item == "" {
		return noFeatures, errors.New("empty feature entry in --features")
	}
	on, isMinus := true, false
	if item, on = strings.CutPrefix(item, "+"); !on {
		if item, isMinus = strings.CutPrefix(item, "-"); isMinus {
			on = false
		}
	}
	tagPart, value, arg := item, "", 1
	hasEqual := false
	if tagPart, value, hasEqual = strings.Cut(item, "="); hasEqual {
		if value == "" {
			return noFeatures, fmt.Errorf("empty feature value in %q", item)
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return noFeatures, fmt.Errorf("invalid feature value in %q: %w", item, err)
		}
		arg = n
		on = n != 0
	}
	tagPart = strings.TrimSpace(tagPart)
	if len(tagPart) != 4 {
		return noFeatures, fmt.Errorf("feature tag %q is not 4 characters", tagPart)
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

// ---Formatting the Output ---------------------------------------------

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
