# Shaping Engine and Test Strategy

## Assessment as of 2026-10-26

The shaping pipeline is substantially implemented and easy to exercise through
tests, but it is not yet a complete, reliable general-purpose shaper. Feature
planning, sequenced GSUB/GPOS execution, streaming output, script-specific
engines, and an offline HarfBuzz comparison harness exist. The earlier milestone
plan in this document predated these implementations; its restrictions on
multiple lookups and its proposal to create `internal/ttxtest` are obsolete.

The review included source inspection, existing tests, and temporary public-API
probes. The full `go test ./... -count=1` run passed. Four probes now have
reproducible, opt-in regression tests under the `shaping_knownbugs` build tag.
The first three recovered AOTS baseline cases add one passing ordinary test
and two further tagged findings (GDEF mark classification and contextual
lookup progression). Two subsequent alternate records add independent evidence
for existing range/value and GDEF findings; both remain tagged. The full
ordinary suite still passes. The GPOS batch adds one passing chained single-
positioning case and five tagged records, exposing unresolved mark origins
(SHAPE-007), retained mark advances (SHAPE-008), and another SHAPE-005 blocker.
The pair batch adds two passing pair-positioning cases and a flagged
advance-changing case blocked by SHAPE-005, using two newly recovered fonts.
[SHAPING_FINDINGS.md](SHAPING_FINDINGS.md) records their workflow, evidence, and
open contract questions. These tests expose correctness gaps; they do not fix
the implementation and are excluded from the ordinary suite.

## Modules, directories, and testing responsibilities

Paths are relative to the repository root. These are packages and directories
within the same Go module.

| Location | Role in shaping and testing |
| --- | --- |
| [`ot/`](../ot/) | Parses font tables and concrete lookup graphs. Structural, synthetic, negative, and TTX tests check whether GSUB/GPOS data is read correctly, independently of shaping output. |
| [`otquery/`](../otquery/) | Supplies cmap lookup, glyph metrics, and font/script queries. Tests cover mapping, metrics, and query errors; failures can appear as incorrect glyphs or advances downstream. |
| [`otlayout/`](../otlayout/) | Executes GSUB/GPOS features and lookups on glyph/position buffers. Tests isolate substitution and positioning, nested lookups, edits, attachment remapping, errors, glyph bounds, and execution limits. |
| [`otshape/`](../otshape/) | Owns the public API, engine selection, normalization, feature plans/masks, streaming, and output. Tests cover orchestration, hooks, ranges, events, buffer correspondence, partial progress, and flushing. |
| [`otshape/otcore/`](../otshape/otcore/) | Generic engine. External-package tests exercise the public API with GSUB/GPOS fixtures, including single/pair adjustments and attachment metadata. `shared_api_knownbugs_test.go` adds opt-in reproductions for shared API findings. |
| [`otshape/otarabic/`](../otshape/otarabic/) | Arabic/Syriac staging, joining forms, masks, mark reordering, fallback repair, and postprocessing. Unit tests exercise these components; its streaming test uses non-joining digits. |
| [`otshape/othebrew/`](../otshape/othebrew/) | Hebrew composition and mark-reordering hooks. Tests cover selected script behaviors and streaming parity. |
| [`internal/ttxtest/`](../internal/ttxtest/) | Parses stored GSUB/GPOS TTX XML into expected models. Comparisons against parsed fonts live primarily in `ot/`; this helper is not a shaping oracle. |
| [`internal/hbcmp/`](../internal/hbcmp/) | Shapes Unicode through the public API. The JSON harness compares glyph IDs, clusters, advances, and offsets; the AOTS adapter compares glyph IDs or glyph IDs plus origins according to the stored format. Ten failing records are behind `shaping_knownbugs`. Neither invokes HarfBuzz during tests. |
| [`internal/hbcmp/testdata/`](../internal/hbcmp/testdata/) | End-to-end expected-output fixtures. Currently only `Gentium_AB.json`, covering Latin `AB`. |
| [`testdata/harfbuzz/aots/`](../testdata/harfbuzz/aots/) | Pinned upstream AOTS shaping records, font hashes, and provenance. Fourteen selected records are executed (five GSUB, nine GPOS; four passing, ten tagged failures, plus a GSUB streaming execution). `ADAPTER.md`, `GPOS.md`, and `PAIR.md` record semantics and results. |
| [`testdata/fonttools/`](../testdata/fonttools/) | Twelve original mini-fonts with paired `*.ttx.GSUB` / `*.ttx.GPOS` dumps, plus two subsequently recovered pair fonts without TTX dumps. Used for parsing, lookup, and public shaping tests. |
| [`testdata/fonts/`](../testdata/fonts/) | Larger fonts, including Calibri, Gentium Plus, and Go fonts, for mapping, metrics, parser, shaping, and diagnostic exercises. |
| [`testdata/scripts/`](../testdata/scripts/) | Fixture discovery/preparation helpers. `find_test_fonts.py` indexes fonts by lookup type; `fix-maxp.py` is a preparation utility. These are not Go test drivers. |
| [`ot-tools/`](../ot-tools/) | Manual diagnostics: `shape` prints glyph output, `view` renders glyph proofs, and `font` inspects tables. Useful for investigation; visual inspection does not replace output assertions. |
| [`internal/fontload/`](../internal/fontload/) | Shared font-byte/SFNT loading support. It does not select or execute shaping features. |

The main dependency path is `otshape` → `otlayout` → `ot`, with `otquery`
providing mapping and metrics. Script engines plug into `otshape`. Localize
failures at the lowest relevant layer, then cover them through the public
shaping API when integration behavior matters.

## Current API and functionality

The entry points are methods on an explicitly constructed shaper:

```go
shaper := otshape.NewShaper(otcore.New(), otarabic.New(), othebrew.New())
err := shaper.Shape(params, source, sink, options)
// Advanced input with nested feature scopes:
err = shaper.ShapeEvents(params, eventSource, sink, options)
```

See [shape_api.go](../otshape/shape_api.go),
[shape_events.go](../otshape/shape_events.go),
[shaper.go](../otshape/shaper.go), and [pipeline.go](../otshape/pipeline.go).

- `NewShaper` accepts candidate engines; there is no global registration step.
  Selection uses confidence and name tie-breaking, then calls the engine's `New`.
- `Params` supplies one font, script, language, direction, and feature ranges.
  The caller supplies run metadata. Automatic script itemization, font fallback,
  paragraph bidi processing, and final RTL output reversal are absent.
- `RuneSource` has `ReadRune`; `strings.Reader` and `bufio.Reader` can supply
  input. `GlyphSink.WriteGlyph` receives IDs, positions, clusters, masks, and
  optional unsafe flags. Clusters originate as codepoint indexes, not UTF-8 byte
  offsets. Position values are in font units.
- `Shape` compiles a plan, repeatedly reads input, normalizes/maps it, runs
  script hooks and GSUB/GPOS stages, emits a prefix, and reshapes raw carry on the
  next cycle. Feature sequencing and stage pauses are implemented.
- `ShapeEvents` supports nested feature push/pop scopes. `Params.Features` must
  be global in this mode. Contiguous spans with different plan IDs are shaped
  separately.
- `BufferOptions` controls watermarks and the raw input buffer cap. Current
  high/low/max defaults are 3500 / 1024 / 4096 codepoints. `FlushExplicit` is
  unsupported. Despite its name, `FlushOnRunBoundary` can emit streaming prefixes
  before EOF; it does not provide atomic whole-request delivery.
- A shaping error prevents emission of the current in-progress chunk, but
  previously emitted prefixes remain. A sink error can leave part of a prefix
  written. Atomic consumers must collect output and commit only after success.

[engine_api.go](../otshape/engine_api.go) exposes hooks for feature planning,
validation, composition, reordering, pre-GSUB processing, masks, stage pauses,
and postprocessing. These interfaces make small test engines straightforward.

The core engine supplies common features. Arabic/Syriac adds joining and staged
features; Hebrew adds composition and reordering. No dedicated Indic or other
complex-script engines exist. Default per-syllable annotations derive from
clusters rather than a script syllable recognizer.

The older `Buffer`, `ShapedGlyph`, and normalization helpers in
[buffer.go](../otshape/buffer.go) are separate from the streaming path.
`Buffer.applySubstitutionFeature` is empty and `TestBufferFunctions` is skipped.
That stub does not describe the implementation state of `Shaper.Shape`.

## Confirmed gaps and regression targets

### Failures reproduced through the public API

These began as temporary review probes. Mask preservation, feature ranges,
alternate values, and engine selection now have tagged public-API regressions;
see [SHAPING_FINDINGS.md](SHAPING_FINDINGS.md) for stable IDs, exact commands,
and current status. Streaming-context and script-tag observations below remain
review probes outside that first regression batch.

| Finding | Reproduction / result | Implementation location |
| --- | --- | --- |
| Script-generated masks are overwritten | With `gsub3_1_simple_f1.otf`, feature `test` enabled, and the cmap-backed rune for GID 18, a custom `SetupMasks` hook clearing masks still produces GID 20. Execution reinitializes masks after the hook. Arabic form selection depends on this hook. | `shapeMappedRun` in [shape_api.go](../otshape/shape_api.go), `planExecutor.apply` in [plan.go](../otshape/plan.go) |
| Feature ranges use chunk-local indexes | Repeat that rune four times and enable `test` for `[2,3)`. Default buffering gives `[18,18,20,18]`; high/low/max `2/1/4` gives `[18,18,18,18]`. Range assignment indexes the current buffer rather than original input clusters. | `ensureRunMasks` and `applyFeatureRangesToMasks` in [lookups.go](../otshape/lookups.go) |
| Feature values do not select alternates | On the same alternate fixture/rune, `Arg=1`, `2`, and `3` all produce GID 20. Values reach masks, but ordinary execution passes `alt=0`. Lower-level tests can select GIDs 20, 21, and 22. | `applyLookups` in [lookups.go](../otshape/lookups.go); [feature_functional_test.go](../otlayout/feature_functional_test.go) |
| Streaming loses context | With `gpos3_font1.otf`, input U+0012 U+0013, and `test` enabled, default buffering records a cursive attachment on the second glyph. High/low/max `1/1/2` loses it. | [shape_streaming.go](../otshape/shape_streaming.go) |
| A nonmatching engine can be selected | Supply only an engine returning `ShaperConfidenceNone`. Shaping succeeds instead of returning `ErrNoMatchingShaper`; selection skips negative scores but accepts zero. | `selectShapingEngine` in [shape_api.go](../otshape/shape_api.go) |
| Script tags are incomplete | `ScriptTagForScript` returns `DFLT` for `Syrc`, `Taml`, and `Mlym`. Much of the map uses full names instead of ISO script codes. Syriac engine matching does not repair the layout script tag. | [language.go](../otshape/language.go) |

### Additional limitations found by inspection

- **Attachment positioning is incomplete.** Single/pair value adjustments are
  applied, but mark/cursive helpers record attachment references without resolving
  anchor coordinates into final offsets. `GlyphRecord.Pos` is therefore not fully
  positioned output for these cases. Device adjustments are ignored, and
  mark-to-ligature component selection has a refinement TODO. See
  [gpos_helpers.go](../otlayout/gpos_helpers.go) and [gpos.go](../otlayout/gpos.go).
- **Streaming safety is incomplete.** Cuts consult clusters and optional unsafe
  flags, but the production pipeline does not populate sufficient contextual
  safety information. When glyph count changes, partial flushing is deferred
  until EOF or a forced full-buffer flush. The raw buffer cap guarantees progress,
  not equivalence to whole-run shaping. See `findFlushCut` and
  `rawFlushForGlyphPrefix` in [shape_streaming.go](../otshape/shape_streaming.go).
- **Normalization is partial.** The active path composes adjacent pairs or
  decomposes individual runes. It does not implement general canonical ordering
  across combining sequences or the older font-aware representation search.
  Unicode composition is not conditioned on availability of the composed glyph.
  See `normalizeRuneStreamWithScratch`, `decomposeRuneStreamInto`, and
  `composeRuneStreamInto` in [shape_api.go](../otshape/shape_api.go).
- **Script support is selective.** Arabic joining uses a hand-maintained
  classification and selected fallback operations. Hebrew implements selected
  composition/reordering cases. Their presence and unit coverage do not establish
  full script conformance.

## What the existing tests establish

### Font discovery, provenance, and actual fixture use

[`tests-found.txt`](../testdata/scripts/tests-found.txt) is a saved discovery
report from [`find_test_fonts.py`](../testdata/scripts/find_test_fonts.py), not
a test manifest or a list of executed tests. The script opens fonts with
FontTools, indexes their GSUB/GPOS lookup types, and prints counts and example
filenames. Its "Coverage" heading describes available lookup data, not this
repository's test coverage. No Go test reads this report to discover cases;
the tests select fixtures explicitly.

The original local FontTools checkout was found at
`/Users/npi/prg/python/fonttools`, with fixtures under
`Tests/ttLib/tables/data/aots`. On 2026-10-26, its HEAD was
`9cec77d49bdb1a1ca346ac5fefdc5e7c30929026` and its origin was
<https://github.com/fonttools/fonttools>. The original 24 files in our
[`testdata/fonttools/`](../testdata/fonttools/) directory were byte-for-byte
identical to their counterparts there: 12 fonts and 12 TTX files. This is a
verified local provenance reference, not a runtime dependency or proof of the
exact historical download command.
The later [pair batch](../testdata/harfbuzz/aots/PAIR.md) imports two additional
fonts directly from the pinned HarfBuzz archive. Neither is listed in the
historical `tests-found.txt`; neither adds a TTX comparison file.

The upstream fixture README identifies
[Adobe's Annotated OpenType Specification (AOTS)](https://github.com/adobe-type-tools/aots)
revision `1c41fd20d2b020177625541a228c4c7c934879ef` as the source of the `.otf`
fonts. It says they were built with `make` and copied from `tests/*.otf`;
the original AOTS XML was not copied. The FontTools checkout contains 206 AOTS
fonts, with TTX references including 94 GSUB, 93 GPOS, and 148 GDEF files.

The Python driver is FontTools' `Tests/ttLib/tables/tables_test.py`, using
pytest. It checks binary-to-TTX output and XML round-tripping against stored
TTX. Those are font-table reading/serialization tests, not Unicode-to-glyph
shaping tests. No HarfBuzz-style `*.tests` shaping case files were found under
that checkout's `Tests/` directory.

The original TTX inventory from 2026-10-26 (before the pair-font additions) is:

| Item | Count |
| --- | ---: |
| Distinct font filenames in `tests-found.txt` | 31 |
| Listed fonts copied into `testdata/fonttools/` | 12 |
| Copied fonts referenced by Go tests | 12 |
| Copied TTX files consumed by structural comparisons | 9 of 12 |
| Listed fonts not copied into this repository | 19 |

The following map accounts for the 12 original copied fonts. Names below omit `.otf`;
each has a paired `.ttx.GSUB` or `.ttx.GPOS` file. The consumers listed are
representative; additional tests reuse some fonts for errors, bounds, masks,
and streaming.

| Font(s) | Structural TTX comparisons | Lookup execution / public shaping examples |
| --- | --- | --- |
| `gsub3_1_simple_f1` | GSUB structural and concrete comparisons | Alternate selection in `otlayout/feature_functional_test.go`; public GSUB in `otshape/otcore/otcore_test.go`; tagged shared-API and AOTS range/value regressions |
| `gsub3_1_lookupflag_f1` | GSUB structural and concrete comparisons | Tagged AOTS IgnoreBaseGlyphs failure (SHAPE-005); no corresponding case in `feature_functional_test.go` |
| `gsub_chaining2_next_glyph_f1` | Selected single/ligature lookups in both GSUB comparison suites | No corresponding execution case in `feature_functional_test.go` |
| `gsub_context1_lookupflag_f1` | Selected contextual lookup in both GSUB comparison suites | `feature_functional_test.go` exercises the IgnoreMarks flag without interleaved marks; pipeline/edit tests; AOTS SHAPE-005 adds interleaved marks |
| `gsub_context1_next_glyph_f1`, `classdef2_font4` | Selected contextual lookups in both GSUB comparison suites | AOTS adapter: tagged SHAPE-006 progression failure and passing no-change class baseline, respectively |
| `gpos_chaining3_boundary_f2` | Selected single, pair, and chained-context lookups in both GPOS comparison suites | Single/pair execution in `otlayout/gpos_runtime_golden_test.go`; passing AOTS chained single-positioning baseline (not pair positioning) |
| `gpos4_simple_1`, `gpos5_font1` | Mark-to-base and mark-to-ligature comparisons | Attachment execution in `otlayout/gpos_runtime_golden_test.go`; five tagged AOTS origin cases for SHAPE-005/007/008 |
| `gpos3_font1`, `gpos3_font2`, `gpos3_font3` | Their TTX files are currently unused by the comparison tests | Cursive attachment cases in `otshape/otcore/gpos_test.go` |

The later additions `gpos2_1_simple_f1` and `gpos2_1_lookupflag_f2` bring the
current mini-font total to 14. They have no structural TTX comparisons: the
first supplies two passing AOTS pair cases, and the second a tagged pair-advance
case for SHAPE-005. The missing-type/format TTX inventory below is unchanged.

The test entry points are:

| File | Responsibility |
| --- | --- |
| [`internal/ttxtest/parse_ttx_gsub_test.go`](../internal/ttxtest/parse_ttx_gsub_test.go) | One self-test of the TTX-reading helper. It reads the alternate fixture's XML; it does not load its binary font or shape text. |
| [`ot/ttx_gsub_compare_test.go`](../ot/ttx_gsub_compare_test.go) | Seven structural GSUB tests comparing binary parsing with TTX-derived expectations. |
| [`ot/ttx_gpos_compare_test.go`](../ot/ttx_gpos_compare_test.go) | Five structural GPOS tests comparing binary parsing with TTX-derived expectations. |
| [`ot/parse_concrete_golden_ttx_test.go`](../ot/parse_concrete_golden_ttx_test.go) | Two table-driven tests with seven GSUB and five GPOS subcases, covering the concrete lookup representation. These overlap the structural cases, rather than adding twelve distinct scenarios. |
| [`otlayout/feature_functional_test.go`](../otlayout/feature_functional_test.go) | Applies GSUB lookups to glyph buffers; tests alternate selection and contextual substitution with the IgnoreMarks flag, but without interleaved marks. |
| [`internal/hbcmp/aots_gsub_test.go`](../internal/hbcmp/aots_gsub_test.go), [`aots_knownbugs_test.go`](../internal/hbcmp/aots_knownbugs_test.go) | Adapts five upstream GSUB records through public shaping; verifies source/font hashes, maps global/single-index features, and compares recorded glyph IDs. Four failing records and one additional streaming execution are opt-in. |
| [`internal/hbcmp/aots_gpos_test.go`](../internal/hbcmp/aots_gpos_test.go) | Adds origin parsing/comparison to the shared AOTS loader/runner. Compares nine upstream GPOS records with public emitted origins; six failures are in `aots_knownbugs_test.go`. Includes matching and nonmatching pair cases. Arithmetic/schema tests validate the adapter separately. |
| [`otlayout/gpos_runtime_golden_test.go`](../otlayout/gpos_runtime_golden_test.go) | Applies positioning lookups and checks single/pair adjustments and attachment metadata. |
| [`otshape/otcore/otcore_test.go`](../otshape/otcore/otcore_test.go), [`gpos_test.go`](../otshape/otcore/gpos_test.go) | Exercises the public shaping API, including the alternate and cursive mini-fonts; other cases use larger fonts. |

For example, `gsub3_1_simple_f1` is used to test the XML helper, binary GSUB
parsing, lookup execution on glyph IDs, and public shaping from Unicode input.
The helper's single test therefore does not represent the extent of fixture
testing elsewhere in the repository.

The 19 listed but uncopied fonts include `gsub3_1_multiple_f1`,
`gsub_context1_lookupflag_f2`, `gsub7_font1/2`, `gpos6_font1`, `gpos9_font1`,
and all five listed `gpos_context*` fonts. The remaining entries are
`gpos4_multiple_anchors_1`, `gpos4_lookupflag_f1/f2`, `Lobster.subset.otf`,
`TestVGID-Regular.otf`, `I.otf`, `NotoSans-VF-cubic.subset.ttf`, and
`Amstelvar-avar2.subset.ttf`. Absence of these particular fixtures does not
establish absence of lookup-type coverage: other tests construct synthetic
lookup data.

We have therefore adopted a selected subset of the fonts and written Go tests
around them, rather than imported the complete upstream test suite. The
corresponding HarfBuzz/AOTS source cases have now been recovered and their font
versions verified; see [the recovery record](../testdata/harfbuzz/aots/README.md).
The [adapter](../testdata/harfbuzz/aots/ADAPTER.md) now executes five GSUB and
nine GPOS records through `internal/hbcmp`; the remaining records await adaptation.
Keep their upstream provenance and distinguish those behavioral expectations
from the TTX structure checks.

### Structural parsing

The TTX path loads a mini-font with `ot.Parse(data, ot.IsTestfont)`, parses its
paired XML with `internal/ttxtest`, and compares selected structures.
`IsTestfont` relaxes font completeness requirements for reduced fixtures while
retaining validation of present data. Assertions include lookup types/formats,
coverage, substitution records, classes, and selected positioning/anchor data.

See `ot/ttx_*_compare_test.go` and `ot/parse_concrete_golden_ttx_test.go`.
Synthetic matrices in `ot/parse_concrete_golden_synth_test.go` and real-font
checks in `ot/parse_concrete_golden_fonts_test.go` extend structural coverage
beyond the TTX subset. Structural coverage is distinct from lookup-runtime or
end-to-end shaping coverage.

#### TTX coverage by lookup type and format

This inventory was checked against the selected lookup indexes, comparison
functions, and binary fixture subtable formats on **2026-10-26**. "Covered"
means an active test compares that lookup's type-specific data with a TTX-derived
expectation. It does not mean every variation or field is covered. Lookup types
are distinct from feature tags such as `liga`, `kern`, or the fixtures' `test`;
these are structural tests of lookup data, not feature-planning tests.

Both the structural and concrete comparison suites exercise the same type/format
subset. Merely containing a lookup in a loaded font, or having a comparison
helper capable of handling it, does not count as a TTX assertion.

| GSUB type | Operation | Formats with active TTX assertions | Formats still missing | Selected fixture and lookup index (zero-based) |
| --- | --- | --- | --- | --- |
| 1 | Single substitution | 1 | 2 | `gsub_chaining2_next_glyph_f1`, lookup 0 |
| 2 | Multiple substitution | None | 1 | Lookup 3 exists in several copied GSUB fonts, but is not selected for comparison |
| 3 | Alternate substitution | 1 | — | `gsub3_1_simple_f1` and `gsub3_1_lookupflag_f1`, lookup 0 |
| 4 | Ligature substitution | 1 | — | `gsub_chaining2_next_glyph_f1`, lookups 1 and 2 |
| 5 | Contextual substitution | 1, 2 | 3 | Format 1: `gsub_context1_lookupflag_f1` / `gsub_context1_next_glyph_f1`, lookup 4; format 2: `classdef2_font4`, lookup 3 |
| 6 | Chained contextual substitution | None | 1, 2, 3 | Format 2 is present at lookup 4 of `gsub_chaining2_next_glyph_f1`, but is not selected |
| 7 | Extension substitution | None | 1, including wrapped lookup data | No selected fixture |
| 8 | Reverse chained single substitution | None | 1 | No selected fixture |

An earlier summary said single substitution covered both delta and explicit-map
formats. That overstated the TTX coverage: the selected binary lookup is format
1 only. The helper has a format-2 comparison branch, but no active TTX case
exercises it. TTX `SingleSubst` represents mappings without necessarily retaining
the binary format; `normalizeSingleSubst` currently infers format from the
mapping. A new format-2 case must verify the intended binary encoding as well.

| GPOS type | Operation | Formats with active TTX assertions | Formats still missing | Selected fixture and lookup index (zero-based) |
| --- | --- | --- | --- | --- |
| 1 | Single adjustment | 1 | 2 | `gpos_chaining3_boundary_f2`, lookup 0 |
| 2 | Pair adjustment | 1 | 2 (class pairs) | `gpos_chaining3_boundary_f2`, lookup 1 |
| 3 | Cursive attachment | None | 1 | `gpos3_font1/2/3` and paired TTX are already copied, but the TTX files are unused |
| 4 | Mark-to-base attachment | 1 | — | `gpos4_simple_1`, lookup 0 |
| 5 | Mark-to-ligature attachment | 1 | — | `gpos5_font1`, lookup 0 |
| 6 | Mark-to-mark attachment | None | 1 | No selected fixture |
| 7 | Contextual positioning | None | 1, 2, 3 | No selected fixture |
| 8 | Chained contextual positioning | 3 | 1, 2 | `gpos_chaining3_boundary_f2`, lookup 4 |
| 9 | Extension positioning | None | 1, including wrapped lookup data | No selected fixture |

Thus **4 of 8 GSUB lookup types** and **5 of 9 GPOS lookup types** have some
TTX-backed assertions. These counts are not measures of overall parser or
shaper correctness. In particular, runtime cursive tests and synthetic tests
for additional formats do not fill the TTX-specific gaps above.

#### Work left open in the TTX test infrastructure

Adding font files alone will not close these gaps. The XML expectation readers
and the Go comparison dispatchers also need extension:

- [`ParseTTXGSUB`](../internal/ttxtest/parse_ttx_gsub.go) currently models single,
  alternate, ligature, and contextual formats 1/2. Multiple, chained contextual,
  extension, reverse chained, and contextual format 3 need expectation support
  and matching assertions. Its introductory comment claiming only alternate
  support is stale; the implementation is broader.
- [`ParseTTXGPOS`](../internal/ttxtest/parse_ttx_gpos.go) currently models single
  format 1, pair format 1, mark-to-base, mark-to-ligature, and chained-context
  format 3. Each missing type/format in the GPOS table requires reader and
  comparison support. Its introductory support list also omits implemented
  mark-to-base and mark-to-ligature handling.
- Make unsupported selected cases fail explicitly. The XML readers can omit
  unmodeled subtable elements, and the comparison switches have no default
  rejection for unsupported lookup types. A newly added case must assert its
  expected subtable count and payload, not merely pass through a dispatcher.
- Audit common assertions while extending the suite: lookup flags are currently
  compared only when the expected flag is nonzero. A zero expectation should
  also be checked. Existing tests do not constitute an exhaustive matrix of
  coverage/class encodings, lookup flags, anchor formats, or device data.

For Latin/Hebrew needs, useful next TTX additions are GPOS single format 2,
class-pair format 2, and mark-to-mark; GSUB single format 2, multiple substitution,
and missing contextual/chained formats; and extension wrappers for both tables.
The local AOTS collection has candidate pairs such as `gsub1_2_simple_f1`,
`gsub2_1_simple_f1`, `gpos1_2_font1`, `gpos2_2_font1`, `gpos6_font1`,
`gsub7_font1`, and `gpos9_font1`. Confirm each candidate's actual lookup format
before importing it. No GSUB-8 TTX case was found in that local AOTS directory;
it needs another fixture source or a deliberately prepared font and TTX pair.
Cursive TTX comparisons can reuse the three already-copied pairs, but remain
lower priority under the current script focus.

Run the existing TTX comparisons and helper self-test with:

```sh
go test ./ot ./internal/ttxtest -run 'Test(TTX|ParseTTX)' -count=1 -v
```

This command passed at the inventory date. It validates the selected cases,
not the missing rows. The structural inventory itself added no tests or fixes;
the later AOTS adapter adds behavioral tests, not structural TTX comparisons.

### Lookup behavior and pipeline mechanics

`otlayout/feature_functional_test.go` supplies `loadTestFont` and
`applyGSUBLookup`. The latter selects a concrete lookup by index, seeds a glyph
and position buffer, and runs the concrete dispatcher. Embedded vectors cover
alternate selection and contextual substitution with the IgnoreMarks flag,
but those vectors contain no interleaved marks. The upstream baseline in
SHAPE-005 supplies that missing behavior and fails; SHAPE-006 independently
exposes overlapping contextual matches. See
[SHAPING_FINDINGS.md](SHAPING_FINDINGS.md#shape-005--gdef-classification-prevents-ignoremarks).
Other
`otlayout` tests cover synthetic substitutions, positioning, extensions, nested
errors, bounded recursion/work, and partial edits.

In `otshape`, `plan_test.go` covers selection, ordering, stages, masks, joiners,
and syllable isolation. The `shape_*_test.go` files cover entry points, hooks,
events, normalization probes, flushing, and source/sink behavior.
`successful_edits_test.go`, `partial_progress_test.go`,
`output_bounds_test.go`, and run-buffer tests cover metadata correspondence and
failure semantics.

The engine subpackages add public-API and script unit tests. However, Arabic
streaming parity deliberately uses non-joining digits; Hebrew streaming parity
uses plain letters. Neither establishes joining/mark behavior across boundaries.
Comparing small buffers with the same implementation at default buffering also
allows shared bugs to pass. A whole-input baseline must use thresholds larger
than the input; default buffering is not always one-shot.

### Independent expected output

The upstream recovery step covers the original 12 mini-fonts and two added pair fonts.
[`testdata/harfbuzz/aots/`](../testdata/harfbuzz/aots/README.md) preserves 13
unchanged test files from HarfBuzz revision
`29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b`; every local font is byte-identical
to its upstream counterpart. Of the 35 records, 21 are active cases using our
fonts, 9 are upstream-disabled cases using our fonts, and 5 reference fonts
outside our subset. The manifest preserves file/line identities, hashes, and
these distinctions. Recovery did not execute HarfBuzz or our shaper on these
records, and disabled upstream records are not findings about our implementation.

The recovered expectations omit clusters; GSUB records also omit positions,
while GPOS records contain glyph origins with advances suppressed. They cannot
be treated as complete glyph/cluster/advance/offset JSON. The current adapter
supports glyph-only GSUB and glyph-origin GPOS expectations with global and single-index feature
requests, preserving one-based positive values and mapping HarfBuzz zero to
disabled. Broader feature syntax remains unimplemented in this adapter;
richer expectations would require a separate
capture from a pinned HarfBuzz build. Missing expected fields must not be filled
from our own shaper. See the recovery README for option semantics, feature ranges, licensing,
and reproducible recovery commands.

The [first AOTS batch](../testdata/harfbuzz/aots/ADAPTER.md) selected three active
records and ran public `Shaper.Shape` with the core engine in a single shaping
window. `classdef2.tests:2` passes with an unchanged glyph sequence;
`gsub_context1_lookupflag.tests:2` and `gsub_context1_next_glyph.tests:2` fail
and are retained as SHAPE-005/006 under `shaping_knownbugs`. Script/language/
direction are explicitly Latin/English/LTR. Both recorded and explicit-context
commands reproduced the expected output using installed HarfBuzz 15.0.0;
the pinned upstream records remain the oracle. No external executable is
required by the Go tests. That initial batch supplied no positioning, cluster,
Hebrew, or streaming baseline. The alternate extension adds two
active records: `gsub3_1_simple.tests:2` reproduces SHAPE-003 with whole-input
buffers and SHAPE-002 with small buffers; `gsub3_1_lookupflag.tests:2` exposes
SHAPE-005 through IgnoreBaseGlyphs. Both retain exact upstream expectations.
Adapter mapping tests separately check scalar range boundaries and value
translation; they do not extend the upstream oracle to random or out-of-range
alternate selection.

The [GPOS batch](../testdata/harfbuzz/aots/GPOS.md) adds six active records.
The chained single-positioning case passes; mark-to-base/mark-to-ligature
cases expose SHAPE-007 (anchor offsets remain unresolved), SHAPE-008 (marks
retain nominal advances), and SHAPE-005 (ligature formation blocked by GDEF
classification). Origins are compared as preceding emitted advances plus the
current offset, in font units; the adapter does not resolve attachments itself.
These assertions do not independently validate every advance/offset or the
final run advance. The fixture's pair lookups are not reached by its public
feature. The later [pair batch](../testdata/harfbuzz/aots/PAIR.md) uses two new
fonts with `test` selecting their sole pair lookup (type 2 format 1). Matching
18/19, nonmatching 18/20, and incomplete end-of-input cases pass. The separate
advance-changing pair record remains tagged because IgnoreBaseGlyphs wrongly
filters its unclassified glyphs (SHAPE-005). It retains the upstream −200
advance effect on trailing origins; passing pair-advance behavior is not yet
established. Cursive baselines remain deferred.

The separate JSON path in `internal/hbcmp` runs `Shaper.Shape` with all three engines.
It compares ordered numeric glyph IDs, clusters, X/Y offsets, and X/Y advances.
That JSON corpus currently contains only `Gentium_AB.json`: useful infrastructure,
but almost no independent evidence for complex shaping.

Each fixture has `context` (font, ISO script, language, direction, optional
feature strings and `testfont`), Unicode scalar `input`, and expected `output`
records with `g`, `cl`, `dx`, `dy`, `ax`, and `ay`. Use the existing JSON
as the schema example. The declared `normalization` field is not applied by the
runner. There is no watermark/event configuration, and internal masks, unsafe
flags, and unresolved attachments are not compared.

### Measured snapshot

The focused review command was:

```sh
go test ./otshape/... ./otlayout ./internal/ttxtest -count=1 -coverprofile=/tmp/otshape-review.cover
```

| Package | Passing top-level tests | Skipped | Statement coverage |
| --- | ---: | ---: | ---: |
| `otshape` | 83 | 1 | 75.0% |
| `otshape/otcore` | 11 | 0 | 60.0% |
| `otshape/otarabic` | 23 | 0 | 87.5% |
| `otshape/othebrew` | 9 | 0 | 53.6% |
| `otlayout` | 53 | 0 | 56.5% |
| `internal/ttxtest` | 1 | 0 | 9.5% |

Counts exclude subtests; the skipped shaping test is `TestBufferFunctions`.
Coverage is per package in this invocation, without `-coverpkg`. In particular,
the helper's own coverage does not reflect its use by tests in `ot/`. The active
decomposition routine had 0% coverage in this run. Executed statements do not
establish correctness or conformance. The separate full repository run passed,
including the HarfBuzz fixture test. Race and benchmark results are not part of
this review snapshot.

## Running and extending tests

Run from the repository root:

```sh
# Shaping, lookup execution, and stored reference output
go test ./otshape/... ./otlayout ./internal/hbcmp -count=1

# Parsing, TTX helpers, cmap, and metrics
go test ./ot ./otquery ./internal/ttxtest -count=1

# Coverage and full repository checks
go test ./otshape/... -cover
go test ./...

# Known-bug discovery suite: expected to fail while recorded findings are open
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1

# Optional concurrency and allocation checks
go test -race ./otshape/... ./otlayout ./internal/hbcmp
go test ./otshape -run '^$' -bench 'Benchmark(LookupApplication|ShapeOrdinary|PositionReplacement)$' -benchmem
```

Go tests consume checked-in fonts, TTX dumps, JSON, and AOTS records; they require no Python,
FontTools, or HarfBuzz executables at runtime. Those tools are useful for fixture
preparation. FontTools and HarfBuzz test-font collections are sources of small
fixtures. `testdata/scripts/find_test_fonts.py` requires Python FontTools and
currently calls `main("..")`; adjust its root for the collection being inspected
rather than assuming it accepts a CLI path argument.

Prefer small fonts with few glyphs and a deliberately limited lookup set. For a
new regression:

1. Add or reuse a font in `testdata/fonttools/`, retaining provenance/licensing.
   Keep paired TTX dumps beside it when structural assertions are needed.
2. Add an `ot` or `otlayout` test isolating parsing or lookup behavior. Specify
   lookup indexes and input/output glyph IDs explicitly.
3. Add a public-API `otshape` test for planning, masks, normalization, hooks,
   streaming, or output materialization. Vary watermarks and assert clusters and
   positions as well as glyph IDs.
4. Capture independent output with HarfBuzz and store JSON under
   `internal/hbcmp/testdata/`. Record the generating command/version and ensure
   numeric glyph IDs, cluster units, font scale, features, and direction agree
   with the runner. Keep expected output offline so tests need no new external
   dependency.

Embedded vectors remain appropriate for isolated lookups. The earlier proposal
for functional sidecars next to every mini-font is not implemented as a general
loader: the existing end-to-end loader reads `internal/hbcmp/testdata/`.
Extend that convention deliberately rather than placing undiscovered JSON beside
fonts.

For a known failure, follow the discovery and promotion process in
[SHAPING_FINDINGS.md](SHAPING_FINDINGS.md#process-and-scope): assert desired
behavior in the tagged suite, record evidence, and move the regression into the
ordinary suite once fixed. Keep unresolved API choices explicit. The broader
roadmap belongs in [TO_DO.md](TO_DO.md); detailed reproductions belong in the
findings file.

For manual inspection:

```sh
go run ./ot-tools shape testdata/fonts/GentiumPlus-R.ttf AB
```

The CLI also accepts feature, codepoint, and watermark flags, but its feature
parser does not accept indexed range syntax. See [OT_TOOLS.md](OT_TOOLS.md) for
`shape`, `view`, and `font`, and [SHAPING_BENCHMARKS.md](SHAPING_BENCHMARKS.md)
for benchmark scope and historical results. The ordinary-shaping benchmark is a
small fixture, not a representative complex-script workload.

## Next testing priorities

The current focus is Latin-script text and Hebrew. Arabic/Syriac and other
scripts are deferred to a later phase. Shared pipeline defects remain in scope
where they affect Latin or Hebrew; use fixtures from those scripts or small
synthetic cases to isolate them. Existing tests for deferred scripts should
continue to pass, but expanding their coverage is not a current priority.

1. **Shared API correctness.** The first four tagged regressions cover mask
   preservation, feature ranges, alternate values, and engine selection; track
   their resolution in [SHAPING_FINDINGS.md](SHAPING_FINDINGS.md). Implement the
   agreed alternate-value contract in a separate fix session and promote
   regressions to the ordinary suite as fixes land. Continue discovery with
   Latin/Hebrew script and language selection; defer expanding the script map
   and regressions specific to other scripts.
2. **Independent Latin/Hebrew baselines.** The first AOTS GSUB adapter batch
   contributes one passing no-change case and two tagged findings (SHAPE-005/006).
   Two alternate records now also reproduce existing SHAPE-002/003/005, with
   single-index features supported and one streaming comparison added.
   Six GPOS origin records add one passing chained single-positioning baseline
   and five failures for SHAPE-005/007/008. The pair batch adds matching and
   nonmatching format-1 baselines that pass, plus an advance-changing case
   blocked by SHAPE-005. Next add richer position captures and class-based pair
   coverage; the current origins omit individual advances.
   Extend the adapter to further recovered cases, explicitly handling their
   feature options and available expected fields. Expand the corpus to
   Latin ligatures, kerning, feature values/ranges, and accented text;
   Hebrew letters, combining marks, and RTL glyph/cluster order. Compare glyph
   IDs and order for glyph-only records. For richer independently captured
   baselines, also compare clusters, advances, and offsets.
3. **Normalization and cluster preservation.** Exercise composed/decomposed
   Latin accents and Hebrew combining sequences, canonical ordering, unavailable
   composed glyphs, and length-changing normalization. Assert original input
   correspondence and feature-range behavior alongside glyph output.
4. **Final mark positioning.** Complete attachment positioning needed by Latin
   accents and Hebrew marks, including mark-to-base and mark-to-mark cases and
   marks on ligatures where applicable. Test final coordinates and advances,
   not only `AttachKind`, `AttachTo`, and anchor references. SHAPE-007/008 now
   supply independent public-output reproductions for anchor and advance gaps.
5. **Streaming and feature-scope boundaries.** Use the Latin/Hebrew baselines to
   test ligatures, pair positioning, normalization, and combining-mark attachments
   across buffer cuts. Sweep watermarks and feature push/pop boundaries, checking
   clusters, ranges, and positions against independent output. Include
   length-changing and empty-output cases to verify progress. Extend the fixture
   runner or add dedicated public-API tests for event/streaming configurations.

Arabic joining, Syriac shaping, script-specific cursive behavior, and broader
script coverage remain follow-up work after these Latin/Hebrew priorities.

Related design notes: [SHAPER_PLAN.md](SHAPER_PLAN.md),
[SHAPER_INTERFACES.md](SHAPER_INTERFACES.md),
[STREAMING_API.md](STREAMING_API.md), and
[PARTIAL_PROGRESS.md](PARTIAL_PROGRESS.md). Some describe intended behavior or
historical milestones; use implementation and tests to establish current
guarantees.
