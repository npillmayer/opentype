package main

import (
	"errors"
	"fmt"
	"os"
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
		SetDescription("Render shaped output to graphics (planned).").
		SetShortDescription("shape to image").
		SetAction(func(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
			_ = args
			_ = flags
			fatalf("view command is not implemented yet")
		})

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
