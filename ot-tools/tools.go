package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/npillmayer/opentype/internal/proof"
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
		AddFlag("preset", "run defaults: latin|hebrew; explicit script/language/direction override", commando.String, "latin").
		AddFlag("input-file", "read UTF-8 text from a file (- for stdin)", commando.String, "").
		AddFlag("stdin", "read UTF-8 text from stdin", commando.Bool, nil).
		AddFlag("script,s", "script (ISO 15924, e.g. Latn, Arab, Hebr)", commando.String, "").
		AddFlag("lang,l", "language tag (BCP 47, e.g. en, ar, he)", commando.String, "").
		AddFlag("direction,d", "direction: ltr|rtl", commando.String, "").
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
		SetDescription("Render a shaped glyph or single-script run to a diagnostic PNG image.").
		SetShortDescription("shape to image").
		AddArgument("font", "OpenType font file path", "").
		AddArgument("text...", "text to shape and render", "").
		AddFlag("preset", "run defaults: latin|hebrew; explicit script/language/direction override", commando.String, "latin").
		AddFlag("input-file", "read UTF-8 text from a file (- for stdin)", commando.String, "").
		AddFlag("stdin", "read UTF-8 text from stdin", commando.Bool, nil).
		AddFlag("script,s", "script (ISO 15924, e.g. Latn, Arab, Hebr)", commando.String, "").
		AddFlag("lang,l", "language tag (BCP 47, e.g. en, ar, he)", commando.String, "").
		AddFlag("direction,d", "direction: ltr|rtl", commando.String, "").
		AddFlag("features,f", "feature list (e.g. liga=1,kern=0,+rlig,-calt)", commando.String, "-").
		AddFlag("codepoints,c", "codepoints instead of text (comma/space separated, e.g. U+0627,U+0644)", commando.String, "-").
		AddFlag("testfont,t", "parse font as relaxed test font fixture", commando.Bool, nil).
		AddFlag("flush", "flush mode: run|cluster", commando.String, "run").
		AddFlag("high-watermark", "stream high watermark (0 uses default)", commando.Int, 0).
		AddFlag("low-watermark", "stream low watermark (0 uses default)", commando.Int, 0).
		AddFlag("max-buffer", "stream max buffer (0 uses default)", commando.Int, 0).
		AddFlag("diagnostics", "show all proof guides and cluster colors", commando.Bool, nil).
		AddFlag("show-pens", "mark pen positions before offsets in orange", commando.Bool, nil).
		AddFlag("show-advances", "draw advance spans and font-unit widths", commando.Bool, nil).
		AddFlag("color-clusters", "color glyphs by source cluster", commando.Bool, nil).
		AddFlag("show-baseline", "draw the run baseline in blue", commando.Bool, nil).
		AddFlag("show-origins", "mark positioned glyph origins in blue", commando.Bool, nil).
		AddFlag("show-labels", "add logical glyph indexes and a glyph/cluster/position legend", commando.Bool, nil).
		AddFlag("output,o", "output PNG file", commando.String, "ot-tools-view.png").
		AddFlag("index,i", "inspect one logical glyph index (default: full run)", commando.Int, -1).
		AddFlag("all,a", "render all shaped glyphs instead of only --index", commando.Bool, nil).
		AddFlag("show-bboxes,B", "draw red bounding-box outlines per rendered glyph", commando.Bool, nil).
		AddFlag("ppem,p", "render scale in pixels-per-em", commando.Int, 96).
		AddFlag("width,W", "image width in pixels (0: automatic)", commando.Int, 0).
		AddFlag("height,H", "image height in pixels (0: automatic)", commando.Int, 0).
		SetAction(runViewCommand)

	commando.
		Register("proof").
		SetDescription("Run a saved JSON case and write labeled comparison PNG and glyph data.").
		SetShortDescription("run a saved comparison case").
		AddArgument("case", "proof-case JSON file", "").
		AddFlag("output,o", "output prefix for .png, .json and .case.json", commando.String, "ot-tools-proof").
		SetAction(runProofCommand)

	commando.Register("serve").
		SetDescription("Serve the local interactive shaping workbench on loopback.").
		SetShortDescription("open a browser workbench").
		AddFlag("port", "loopback port (0 chooses an available port)", commando.Int, 0).
		AddFlag("case", "optional initial proof-case path", commando.String, "").
		SetAction(runServeCommand)
	commando.DefaultCommandRegistry.Commands["serve"].Flags["case"].IsRequired = false

	commando.
		Register("font").
		SetDescription("Print diagnostics and table information for an OpenType font.").
		SetShortDescription("font diagnostics").
		AddArgument("font", "OpenType font file path", "").
		AddArgument("tables...", "optional list of table tags (e.g. GSUB,GPOS,head)", "").
		AddFlag("testfont,t", "parse font as relaxed test font fixture", commando.Bool, nil).
		AddFlag("errors,e", "print parse errors and warnings", commando.Bool, nil).
		SetAction(runFontCommand)

	// commando treats empty string defaults as required. These are deliberately
	// optional: absent selection fields inherit the preset; absent input-file
	// leaves the positional/codepoint/stdin input choices available.
	for _, name := range []string{"shape", "view"} {
		for _, flag := range []string{"script", "lang", "direction", "input-file"} {
			commando.DefaultCommandRegistry.Commands[name].Flags[flag].IsRequired = false
		}
	}
	commando.Parse(featureCLIArgs(os.Args[1:]))
}

// --- Helpers ----------------------------------------------------------

// clapper (commando's parser) splits flag tokens at every '=' and treats a
// leading '-' in a string value as another flag. Preserve feature expressions
// such as --features=liga=0 and --features -liga as one string value. A leading
// space protects the value from flag recognition; parseFeatureList trims it.
func featureCLIArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		if name != "--features" && name != "-f" {
			out = append(out, args[i])
			continue
		}
		if !hasValue {
			i++
			if i == len(args) {
				fatalf("%s requires a feature list", name)
			}
			value = args[i]
		}
		out = append(out, name, " "+value)
	}
	return out
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

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "ot-tools: "+format+"\n", args...)
	os.Exit(1)
}

func parseTypesetFlags(flags map[string]commando.FlagValue) (
	script language.Script, lang language.Tag, dir bidi.Direction, err error) {
	settings := proof.Settings{}
	settings.Preset, _ = flags["preset"].GetString()
	settings.Script, _ = flags["script"].GetString()
	settings.Language, _ = flags["lang"].GetString()
	settings.Direction, _ = flags["direction"].GetString()
	settings, err = settings.Resolve()
	if err != nil {
		return
	}
	script, err = language.ParseScript(settings.Script)
	if err != nil {
		return
	}
	lang, err = language.Parse(settings.Language)
	if settings.Direction == "rtl" {
		dir = bidi.RightToLeft
	} else {
		dir = bidi.LeftToRight
	}
	return
}
