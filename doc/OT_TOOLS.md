# OT-Tools: Command Line Tools

We need some command-line tools in the spirit of the Harfbuzz Utilities:
[Harfbuzz Ulitilies Page](https://harfbuzz.github.io/utilities.html)

## CLI Subcommand Style

Instead of creating 3 distinct binaries, we will create a single one with subcommands.

The Go library to use for creating the CLI is this one:
[commando](https://github.com/thatisuday/commando).

The CLI will be called like this:

```sh
> ot-tools shape path/to/font.otf Hello
[H=0+1479|e=1+1139|l=2+548|l=3+548|o=4+1139]
>
```

## `ot-tools shape`: Basic Textual Output

Please refer to
[hb-shape](https://harfbuzz.github.io/utilities.html#utilities-command-line-hbshape)

Features:
- Basic text shaping
- Support for multiple scripts and languages
- Customizable features and options

### Initial Implementation Status

Implemented as first baseline:

- command: `ot-tools shape <font> <text...>`
- default shapers: Arabic, Hebrew, Core
- output: glyph stream as
  `[gid=cluster+xAdvance|gid=cluster+xAdvance@xOffset,yOffset|...]`
- optional flags:
  - `--script,-s` (default `Latn`)
  - `--lang,-l` (default `en`)
  - `--direction,-d` (`ltr` or `rtl`, default `ltr`)
  - `--features,-f` (e.g. `liga=1,kern=0,+rlig,-calt`)
  - `--codepoints,-c` (hex input, e.g. `U+0627,U+0644`)
  - `--testfont,-t` (parse with relaxed fixture rules)
  - `--flush` (`run` or `cluster`, default `run`)
  - `--high-watermark`, `--low-watermark`, `--max-buffer`

Notes:

- `text...` is variadic and may be joined by `commando` with commas.
- `--codepoints` is useful for non-printable test inputs.

## `ot-tools view`: Graphics Output

Please refer to
[hb-view](https://harfbuzz.github.io/utilities.html#utilities-command-line-hbview)

Features:
- Create PNG images of text shaping results

This sub-command will need the following Go packages:

- `https://pkg.go.dev/golang.org/x/image/font`
- `https://pkg.go.dev/golang.org/x/image/font/opentype`

It seems fairly cryptic. At least there is this example:
[Example](https://pkg.go.dev/golang.org/x/image/font/opentype#example-NewFace)

### Findings on `x/image` Usage

There are two viable paths:

1. `font/opentype` + `font.Drawer`
2. `font/sfnt` + `vector.Rasterizer`

For our `ot-tools view` use-case (already shaped glyph stream), path 2 is the correct one.

Why:

- `font.Drawer` draws runes (`DrawString`) and internally does its own rune→glyph mapping.
- Our pipeline already produced glyph IDs and GPOS offsets/advances.
- We must therefore render **glyph IDs directly**, not runes.

Concrete API path (validated with a local probe):

1. Shape input with `otshape.Shape` to `[]GlyphRecord`.
2. Parse font bytes with `sfnt.Parse`.
3. Pick rendering scale:
   - `ppem` (pixels-per-em), e.g. 72
   - `scale := fixed.I(ppem)`
4. For each glyph:
   - `segments, err := sfntFont.LoadGlyph(&buf, sfnt.GlyphIndex(gid), scale, nil)`
   - feed segments into a `vector.Rasterizer` (`MoveTo`, `LineTo`, `QuadTo`, `CubeTo`)
   - draw into RGBA/Alpha target using `rasterizer.Draw(...)`
5. Advance pen position using shaper advances:
   - `penX += XAdvance * ppem / unitsPerEm`
   - apply offsets similarly for glyph-local placement.
6. Encode output with `image/png`.

Important conversion notes:

- `otshape` position values are in font units.
- `sfnt.LoadGlyph` path coordinates are returned at the chosen ppem scale in `fixed.Int26_6`.
- Convert `fixed.Int26_6` to float pixels by dividing by `64`.

### Initial Implementation Status

Implemented as rudimentary baseline:

- command: `ot-tools view <font> <text...>`
- shapes input text first, then renders:
  - one selected glyph from shaped output, or
  - the full shaped glyph run (`--all`)
- default output: `ot-tools-view.png`
- optional flags:
  - `--index,-i` glyph index in shaped output (default `0`)
  - `--all,-a` render all shaped glyphs
  - `--show-bboxes,-B` draw red outline of each glyph bounding box
  - `--output,-o` output PNG path
  - `--ppem,-p` render scale (pixels-per-em, default `96`)
  - `--width,-W`, `--height,-H` image size
  - script/language/direction/feature/codepoint/testfont flags like `shape`

Current scope:

- single-glyph mode centers one glyph on a white canvas
- full-run mode composes all shaped glyphs using advances and offsets
- intended as a diagnostics/proof tool, not final text layout rendering

## `ot-tools font`: OpenType Font Diagnostics

Features:
- Extract information about OpenType font tables (font-tools like)

### Initial Implementation Status

Implemented as first baseline:

- command: `ot-tools font <font> [tables...]`
- summary output:
  - font path and type
  - name table fields (family, subfamily, version) when available
  - full table tag list
  - present layout table list
  - parse issue counts (errors, warnings, critical)
- optional `tables...` prints table offset/size for selected tags
- optional `--testfont,-t` parses with relaxed fixture rules (for mini test fonts)
- optional flag `--errors,-e` prints all parser errors/warnings
