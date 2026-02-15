package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/thatisuday/commando"
	"golang.org/x/image/font/sfnt"
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

// --- Helpers ----------------------------------------------------------

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

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "ot-tools: "+format+"\n", args...)
	os.Exit(1)
}

func parseTypesetFlags(flags map[string]commando.FlagValue) (
	script language.Script, lang language.Tag, dir bidi.Direction, err error) {
	//
	if script, err = parseScript(flags["script"]); err != nil {
		return
	}
	if lang, err = parseLanguage(flags["lang"]); err != nil {
		return
	}
	dir, err = parseDirection(flags["direction"])
	return
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
