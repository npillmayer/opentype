# OT-Tools: Local Workbench, Proofs and Diagnostics

`ot-tools` is a small command-line application for exercising this library's
shaper and inspecting the result. It provides textual glyph output, PNG proofs,
saved feature comparisons, font diagnostics, and a local browser workbench.
Its visual target is one explicitly configured Latin or Hebrew run, with one font.
It does not perform paragraph layout.

## Build and first proofs

From the repository root:

```sh
go build -o /tmp/ot-tools ./ot-tools

/tmp/ot-tools shape testdata/fonts/GentiumPlus-R.ttf 'office AVATAR'
/tmp/ot-tools view testdata/fonts/GentiumPlus-R.ttf 'office AVATAR' \
  --output /tmp/latin.png
/tmp/ot-tools view testdata/fonts/NotoSansHebrew-Regular.ttf 'שָׁלוֹם' \
  --preset hebrew --diagnostics \
  --output /tmp/hebrew.png
/tmp/ot-tools font testdata/fonts/GentiumPlus-R.ttf GSUB,GPOS
/tmp/ot-tools proof testdata/proof-cases/latin-features.json --output /tmp/latin-comparison
/tmp/ot-tools proof testdata/proof-cases/hebrew-marks.json --output /tmp/hebrew-comparison
```

Quote the entire text argument: the `commando` CLI library joins separate
variadic arguments with commas. `--preset latin` (the default) selects
Latn/en/ltr; `--preset hebrew` selects Hebr/he/rtl. Explicit `--script`, `--lang`
and `--direction` override the corresponding preset field. There is no automatic
script detection, bidi itemization or font fallback.

Use `--input-file sample.txt` to read UTF-8 text from a file, or `--stdin` /
`--input-file -` to read standard input:

```sh
printf 'office AVATAR' | /tmp/ot-tools view testdata/fonts/GentiumPlus-R.ttf \
  --stdin --output /tmp/from-stdin.png
```

Choose exactly one input source: positional text, codepoints, file or stdin.
Input bytes must be valid UTF-8. Whitespace and newlines are preserved; newlines
do not introduce line layout. Use a single-run sample (for example, `printf`
without a trailing newline) when that is what you intend to inspect.

## `serve`: interactive browser workbench

From the repository root:

```sh
go build -o /tmp/ot-tools ./ot-tools
/tmp/ot-tools serve --case testdata/proof-cases/latin-features.json
# Or start with a blank editor on a chosen port:
/tmp/ot-tools serve --port 8080
```

Open the **complete URL printed in the terminal**, including its `#` session
suffix. The server binds only to `127.0.0.1`; the default port `0` selects an
available port. Ctrl-C stops it. No Node installation, frontend build, CDN or
network service is needed to run the workbench. JavaScript modules, HTML and CSS
are embedded in the Go executable; rebuild after editing those files.

1. Open a version-1 case by its local path, use **Latin example** / **Hebrew
   example**, or start **New** and enter a font path and source text. Example
   buttons assume the server was launched from the repository root. Case-relative
   font and `text_file` paths retain CLI semantics; edited relative font paths
   resolve against the displayed server directory. File selection is by path,
   not a native file picker. The server reads only explicitly chosen case/font
   inputs and the text file referenced by a selected case; it exposes no directory
   browser or general file-download endpoint.
2. Edit shared features, scalar ranges, ISO script, BCP 47 language, direction,
   and labeled variants. Add/remove variant rows as needed. Preset buttons set
   the run fields; they do not silently choose a different font. Open **Buffer &
   fixture controls** for flush/watermark/capacity settings and relaxed test-font
   parsing. Text and normalization are preserved; existing CR/CRLF line endings
   survive loading and edits outside them. Newly entered line breaks use LF.
3. Edits refresh after 300 ms, or use **Update proof**. Validation, font-hash,
   shaping and rendering failures appear above the proof. Pending edits disable
   selection/export and dim the previous preview; errors clear it. A revision
   changes immediately on every edit, before debounce, and rejects stale success
   **and** error responses. Aborting superseded requests also reduces wasted work.
   Automatic refresh leaves the typing caret alone.
4. Click a glyph in the preview or its logical index in the glyph list. Marks
   use ink-based hit targets, including when their advance is zero; spaces and
   empty outlines have minimum targets. Repeated clicks cycle overlapping
   bounding-box targets, smallest first. This is not exact outline hit testing.
   The selected glyph exposes GID, cluster, logical/visual index, advances,
   offsets, pen and origin. Its overlay shows ink bounds, baseline, pen, advance
   and positioned origin even when those global guides are disabled. Use **Zoom**
   and the preview scroll area for large comparisons.
5. The source highlight runs from the selected cluster anchor to the next greater
   output anchor, or input end. Repeated anchors share the same highlight. It is
   an inferred **cluster span**, including ligatures, marks and merged/deleted
   clusters, not an exact character-to-glyph mapping. Scalar indexes are explicitly
   converted to JavaScript UTF-16 positions and textarea newline coordinates.
   The original-source display is ordinary browser text for inspection; only the
   Go-rendered PNG is the shaping proof.
6. **Save case**, **Export PNG** and **Export glyph data** download the resolved
   `.case.json`, comparison image and numerical report through the browser.
   Browser download settings determine the destination. Exports use the same
   completed execution currently displayed, without shaping again. Reopen a
   saved case by its downloaded path, or replay it with the CLI `proof` command.
   The font stays external; snapshots retain its absolute path and SHA-256.
   Changing the path retains the expected fingerprint. Clear the fingerprint
   explicitly when choosing a different font, rather than bypassing a mismatch.

The preview uses the shared Go renderer's final glyph IDs, positions and row
alignment. Pixel geometry for hit testing and guides is generated from the same
layouts and canvas translations; JavaScript does not reshape text or reconstruct
RTL placement. PNG exports contain the configured proof guides; the transient
selected-glyph overlay is an inspection aid and is not burned into the export.
Reports remain in logical order and font units with upward Y.

Random alternate selections may differ between updates or replays. A note is
shown for those requests. Saving settings does not freeze their observed choices;
export the PNG and glyph report to retain a particular result.

The local HTTP interface checks the exact Host, same-origin requests and an
unguessable session token. Only the embedded assets are served publicly; API
requests require the session. Font files are limited to 32 MiB, cases/text files
and JSON requests to 1 MiB, and interactive input to 8192 original scalars.
The shared 1–64-variant, 1–4096-ppem and 16,777,216-pixel limits still apply,
as do the shaper's safe-buffer/error contracts. One preview executes at a time.
Cancellation is checked during ingestion/emission and between variants; an
in-progress font parse, lookup or raster operation is not immediately preempted.
These limits are not a total-memory guarantee. Workbench scripts are limited to
Latn/Hebr; existing CLI behavior is retained.

## Shared shaping options

Both `shape` and `view` invoke `(*otshape.Shaper).Shape`, with the Core, Hebrew
and Arabic engines registered. Registration does not imply complete script
support; Latin/Hebrew remain the current proofing scope.

| Option | Meaning |
| --- | --- |
| `--preset` | `latin` (default) or `hebrew`; supplies script/language/direction defaults. |
| `--script,-s` | ISO 15924 script; overrides preset. |
| `--lang,-l` | BCP 47 language; overrides preset. |
| `--direction,-d` | `ltr` or `rtl`; overrides preset. |
| `--features,-f` | Global or ranged feature settings, separated by commas or spaces. |
| `--codepoints,-c` | Hex Unicode scalars, separated by commas or spaces; e.g. `U+05E9,U+05B8,U+05C1`. Surrogates and values above U+10FFFF are rejected. |
| `--testfont,-t` | Relaxed parser rules for test fixtures; not a workaround for unsupported production fonts. |
| `--flush` | `run` or `cluster`; default `run`. |
| `--high-watermark`, `--low-watermark`, `--max-buffer` | Shaper buffer options; zero selects library defaults. |

Feature examples: `--features='liga,kern=1'`, `--features='-liga'`,
`--features='liga=0'`, `--features='salt=2'`. A bare tag or `+tag` enables value
one. `tag=0` disables it. Positive alternate selections are one-based; negative
values request random selection. With an explicit `=value`, that numeric value
determines enablement even if a prefix is present.

Ranges use zero-based original Unicode scalar indexes: `liga[3:5]=1` selects
`[3,5)`, `kern[:4]=0` starts at zero, and `salt[2:]=2` runs to the end. The end
must exceed the start when supplied; empty/reversed/negative ranges are rejected.
Only `[start:end]` syntax is supported, not single-index shorthand. Combined
characters follow the library's range-boundary contract; the CLI does not split
a base/mark group. For example, `--features='liga=0,liga[7:13]=1'` enables
ligatures only in the second word of `office office`. Event input remains outside
the CLI. The CLI's zero syntax
means disabled, whereas the library can separately express enabled/default zero;
see [compatibility conventions](SHAPER_INTERFACES.md#harfbuzz-compatibility-and-intentional-differences).

Buffer options retain the library's [supported limits and error contract](STREAMING_API.md#agreed-buffer-contract-2026-11-02).
`view` collects the completed shaped run before rendering. A shaping error
produces no PNG from partial output. The CLI collector and image are additional
storage, independent of the shaper's raw-buffer budget.

## `shape`: numerical output

```sh
/tmp/ot-tools shape testdata/fonts/GentiumPlus-R.ttf office --features=liga
```

Output is `[gid=cluster+xAdvance|...]`; nonzero Y advances and X/Y offsets are
included. Advances and offsets are final font-unit values. Clusters refer to
original Unicode scalar indexes. Output remains in the shaper's **logical order**,
including RTL; `shape` does not rearrange it for display.

## `view`: glyph and run proofs

`view` renders the entire shaped run by default. Use `--index 0` (or another
logical output index) to inspect one glyph. `--all` remains a compatibility alias
for full-run rendering; combining it with `--index` is an error. A selected
glyph is shown in isolation; its neighbors and attachment targets are absent.

| Option | Meaning |
| --- | --- |
| `--all,-a` | Render the full shaped run (already the default). |
| `--index,-i` | Select one logical output glyph; omit to show the full run. |
| `--output,-o` | PNG path; default `ot-tools-view.png`. Parent directories are created. |
| `--ppem,-p` | Pixels per em; default 96, supported range 1–4096. |
| `--width,-W`, `--height,-H` | Canvas dimensions. Each defaults to zero for automatic sizing. |
| `--diagnostics` | Enable all guides, labels and cluster colors together. |
| `--show-pens` | Orange boxes at pen positions, before glyph offsets. |
| `--show-advances` | Orange spans/end ticks and horizontal advance values in font units. Zero advances retain ticks; their values remain in the glyph legend. |
| `--color-clusters` | Apply a repeating six-color palette keyed by scalar cluster anchor; marks in the same cluster share a color. |
| `--show-bboxes,-B` | Red outline bounds for each glyph with visible path geometry. |
| `--show-baseline` | Blue run baseline. |
| `--show-origins` | Blue crosses at positioned glyph origins, after applying offsets. |
| `--show-labels` | Logical indexes near origins and a legend of glyph IDs, scalar clusters, and final font-unit advances/offsets. |

Automatic sizing includes ink overhangs, font ascent/descent, pen extents, and
24-pixel margins. Spaces retain their advances even without ink. Diagnostic
labels reserve extra space. An explicitly undersized canvas is an error rather
than silent clipping. Images are limited to 16,777,216 pixels; reduce scale or
input length if necessary. These are tool-specific allocation limits, not
HarfBuzz compatibility rules or a total-memory guarantee. Labels can overlap
near tightly positioned marks; the separate legend retains their full values.

The renderer consumes glyph IDs and final positioning directly. It loads
outlines with `sfnt.LoadGlyph` and draws them with `vector.Rasterizer`; it never
passes shaped text through another rune-to-glyph mapper. Nominal advances are
already included in the shaper's output and are **not added again**. Positive
OpenType Y positions point upward and are inverted for image coordinates.
For RTL, it reverses the complete logical record sequence, including records
within a cluster, preserving every record's positions. This is single-run visual
ordering, not paragraph bidi layout.

Each glyph is rasterized independently so overlapping glyph contours do not
cancel. Outline-loading errors are reported with the logical index and glyph ID;
missing outlines are not silently skipped. Diagnostic ASCII labels alone use a
small bitmap font. PNG output is unhinted and is not a promise of pixel equality
with a browser, an OS text renderer, or `hb-view`.

## `proof`: saved comparison cases

A version-1 JSON case keeps one font, text, script/language/direction and scale
fixed while comparing labeled feature variants. Start with the checked-in
[Latin features](../testdata/proof-cases/latin-features.json) or
[Hebrew marks](../testdata/proof-cases/hebrew-marks.json) example, or write:

```json
{
  "version": 1,
  "font": "/absolute/path/to/font.ttf",
  "text": "office office AVATAR",
  "preset": "latin",
  "features": ["liga=1", "kern=1"],
  "render": {"ppem": 72, "show_baseline": true, "color_clusters": true},
  "variants": [
    {"label": "Defaults"},
    {"label": "Ligatures off", "features": ["liga=0"]},
    {"label": "Kerning off", "features": ["kern=0"]},
    {"label": "Second word only", "features": ["liga=0", "liga[7:13]=1"]}
  ]
}
```

Run `ot-tools proof case.json --output /tmp/comparison`. Outputs are:

- `/tmp/comparison.png`: labeled rows with common scale and vertical plot extents.
  Runs share a logical-start anchor: the left pen edge for LTR, the right pen edge
  for RTL. Different advances or ink overhangs do not independently center rows.
- `/tmp/comparison.json`: resolved case, font fingerprint, available Go/module/VCS
  build metadata, and each variant's requested features and full numerical glyph
  data. Glyph records are in logical order with a separate `visual_index`.
- `/tmp/comparison.case.json`: standalone resolved input for replay, with inline
  text, explicit script/language/direction, rendering scale and font SHA-256.

Replay with `ot-tools proof /tmp/comparison.case.json --output /tmp/replayed`.
Choose a different output prefix when necessary: the command refuses to overwrite
its input case. Reusing an output prefix otherwise replaces its generated files.
The font remains an external file; snapshots store its absolute path and hash.
You can relocate it and update the path while retaining the fingerprint check.

Additional case fields:

| Field | Meaning |
| --- | --- |
| `text_file` | Alternative to `text`; read exact UTF-8 contents. Relative font/text paths resolve against the case file's directory. Snapshots embed file text. |
| `font_sha256` | Optional expected font hash. A mismatch is an error. Every result/snapshot records the actual hash. |
| `script`, `language`, `direction` | Override the corresponding preset field. |
| `testfont` | Relaxed fixture parsing; defaults to false. |
| `buffer` | Object with `flush` (`run`/`cluster`), `high_watermark`, `low_watermark`, `max_buffer`. Zero values retain library-default semantics. |
| `render` | `ppem` (default 96), `diagnostics`, `show_bboxes`, `show_baseline`, `show_origins`, `show_labels`, `show_pens`, `show_advances`, `color_clusters`. Unspecified switches are false. Comparison canvases are sized automatically. |
| `variants` | One to 64 rows with nonempty unique `label` values. Each `features` array is appended to the case-level feature array; library precedence applies in that order. Each array element is one feature expression. |

Unknown JSON fields and unsupported schema versions are errors. Captions display
Unicode labels as ASCII escapes so the small diagnostic font cannot silently
misrender them; the sample itself is rendered with the selected font. Long
captions, many glyph labels or many rows may exceed the image pixel budget.

The report preserves GID, cluster, X/Y advances and offsets, plus visual pen and
positioned-origin coordinates in **font units with Y upward**. Pen coordinates
start at zero at the visual beginning of each run, before image alignment and
scaling; `origin = pen + offset`. The glyph array remains in logical order.
These are observed results, not independent correctness expectations.

Snapshots reproduce the input and settings, not a frozen shaper implementation.
Buffer zeros still mean library defaults; build metadata identifies the producer
when available. Random alternate selections can differ between runs because the
public shaper has no fixed request-seed option; reports flag such requests.
There is no HarfBuzz comparison command or runtime dependency.

## `font`: font diagnostics

```sh
/tmp/ot-tools font path/to/font.ttf GSUB,GPOS,head --errors
```

Reports path, font type, family/subfamily when available, table tags, layout
tables, and counts of parser errors, warnings and critical errors. Optional
table arguments report offsets and sizes. `--errors,-e` prints parser errors
and warnings; `--testfont,-t` enables relaxed fixture parsing.

## Implementation and verification

- [internal/proof](../internal/proof/) owns version-1 case decoding/validation,
  presets, feature/scalar parsing, shaper assembly, fingerprint checks, variant
  execution, numerical reports, geometry and rasterization. `Execute` /
  `ExecuteContext` take a resolved case and immutable font bytes and return an
  atomic `Execution` with `Report`, `Image` and pixel `Geometry`. They perform no
  file/network I/O and never exit the process. `Render` also serves CLI `view`.
- [tools.go](../ot-tools/tools.go), [shape_cmd.go](../ot-tools/shape_cmd.go),
  [input.go](../ot-tools/input.go), [view_cmd.go](../ot-tools/view_cmd.go),
  [case.go](../ot-tools/case.go) and [proof.go](../ot-tools/proof.go) keep CLI
  flags, path resolution, file/stdin loading, formatting, process exits and file
  output at the frontend edge.
- [serve_cmd.go](../ot-tools/serve_cmd.go) implements the loopback HTTP frontend,
  bounded input reads, session checks, request cancellation and embedded assets.
- [workbench](../ot-tools/workbench/) contains plain browser controls, source
  correspondence and revision-gated updates. There is no frontend framework.

Run focused checks with:

```sh
go test ./ot-tools ./internal/proof -count=1
go test -race ./ot-tools ./internal/proof -count=1
go vet ./...
node --test ot-tools/workbench/*_test.mjs
go test ./... -count=1 -timeout=180s
```

Tests cover final advance handling, complete RTL reversal with repeated clusters
and zero-advance marks, Y offsets/advances, spaces, canvas limits, invalid glyphs,
feature switches and Unicode scalar validation. Seven existing independent
HarfBuzz 15.0.0 captures exercise Latin ligatures, kerning and marks, plus plain
and pointed Hebrew at 48 and 96 ppem. They check every glyph/cluster/position
field and absolute placement, then compare images rendered from our logical
output against images rendered from the captured visual output. Captures are
unchanged, font hashes are verified, and tests require no HarfBuzz executable.
These tests validate placement against the oracle; the shared rasterizer is
not an independent test of rasterization quality. CLI smoke tests also exercise
command wiring, flags, PNG output and error reporting. Saved-case tests cover
feature ranges, preset overrides, relative file paths, preserved Unicode and
newlines, fingerprint validation, snapshot replay, variant effects, common LTR/RTL
start alignment, and logical versus visual report coordinates.

HTTP tests verify session/host/origin rejection, bounded inputs, validation and
safe-buffer failures, cancellation, and CLI/HTTP report and PNG equality for both
saved examples. Shared geometry tests cover logical indexes, Y inversion,
zero-advance targets and alignment; source/update module tests use Node's built-in
test runner (Node is a development-only requirement) for scalar/UTF-16 conversion,
CR/CRLF retention, and delayed successes/errors arriving after newer edits.

For a browser smoke check, open the Latin example, select the `ffi` ligature and
confirm span `[1,4)`. Enter `😀office b́`, select its zero-advance acute mark, and
confirm span `[8,10)` and preserved typing caret after refresh. Open the Hebrew
example, select a vowel mark both in the preview and list, and confirm shared
span `[0,3)` plus reversed visual indexes. Try `שׁׁ` for overlapping mark targets.
Edit a variant and a feature range, confirm invalid ranges show an error and
block export, then correct them. Download all three outputs, reopen the downloaded
case and replay it with CLI `proof`; PNG bytes and glyph records should match
for these nonrandom cases. This is manual UI verification, not an independent
shaping oracle.

Still outside this iteration: event input, mixed-script paragraphs, line breaking,
font fallback, new script engines, detailed lookup tracing and cloud hosting. HarfBuzz comparison is explicitly outside
the tool's planned scope; existing offline oracle tests remain regression checks.
