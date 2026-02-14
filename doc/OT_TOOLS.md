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

### Initial Implementation Status

- command is registered, but currently intentionally unimplemented
- returns a clear error message placeholder

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
