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
| [`internal/hbcmp/`](../internal/hbcmp/) | Shapes Unicode through the public API and compares glyph IDs, clusters, advances, and offsets with stored HarfBuzz-style JSON output. It does not invoke HarfBuzz during tests. |
| [`internal/hbcmp/testdata/`](../internal/hbcmp/testdata/) | End-to-end expected-output fixtures. Currently only `Gentium_AB.json`, covering Latin `AB`. |
| [`testdata/fonttools/`](../testdata/fonttools/) | Mini-fonts and paired `*.ttx.GSUB` / `*.ttx.GPOS` dumps for narrowly controlled parsing and lookup tests. |
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

### Structural parsing

The TTX path loads a mini-font with `ot.Parse(data, ot.IsTestfont)`, parses its
paired XML with `internal/ttxtest`, and compares selected structures.
`IsTestfont` relaxes font completeness requirements for reduced fixtures while
retaining validation of present data. Assertions include lookup types/formats,
coverage, substitution records, classes, and selected positioning/anchor data.

| Table | Stored TTX comparison scope | Example fixtures |
| --- | --- | --- |
| GSUB | Single substitution (delta and explicit mapping), alternate format 1, ligature format 1, contextual formats 1 and 2 | `gsub3_1_simple_f1`, `gsub_chaining2_next_glyph_f1`, `gsub_context1_lookupflag_f1`, `gsub_context1_next_glyph_f1`, `classdef2_font4` |
| GPOS | SinglePos format 1, PairPos format 1, MarkToBase format 1, MarkToLigature format 1, ChainContextPos format 3 | `gpos_chaining3_boundary_f2`, `gpos4_simple_1`, `gpos5_font1` |

See `ot/ttx_*_compare_test.go` and `ot/parse_concrete_golden_ttx_test.go`.
Synthetic matrices in `ot/parse_concrete_golden_synth_test.go` and real-font
checks in `ot/parse_concrete_golden_fonts_test.go` extend structural coverage
beyond the TTX subset. Structural coverage is distinct from lookup-runtime or
end-to-end shaping coverage.

### Lookup behavior and pipeline mechanics

`otlayout/feature_functional_test.go` supplies `loadTestFont` and
`applyGSUBLookup`. The latter selects a concrete lookup by index, seeds a glyph
and position buffer, and runs the concrete dispatcher. Embedded vectors cover
alternate selection and contextual substitution with ignored marks. Other
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

`internal/hbcmp` loads JSON and runs `Shaper.Shape` with all three engines.
It compares ordered numeric glyph IDs, clusters, X/Y offsets, and X/Y advances.
The corpus currently contains only `Gentium_AB.json`: useful infrastructure,
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
go test -tags shaping_knownbugs ./otshape/... -count=1

# Optional concurrency and allocation checks
go test -race ./otshape/... ./otlayout ./internal/hbcmp
go test ./otshape -run '^$' -bench 'Benchmark(LookupApplication|ShapeOrdinary|PositionReplacement)$' -benchmem
```

Go tests consume checked-in fonts, TTX dumps, and JSON; they require no Python,
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
2. **Independent Latin/Hebrew baselines.** Expand the HarfBuzz corpus beyond
   `AB`: Latin ligatures, kerning, feature values/ranges, and accented text;
   Hebrew letters, combining marks, and RTL glyph/cluster order. Compare glyph
   IDs, clusters, advances, and offsets, not just output lengths.
3. **Normalization and cluster preservation.** Exercise composed/decomposed
   Latin accents and Hebrew combining sequences, canonical ordering, unavailable
   composed glyphs, and length-changing normalization. Assert original input
   correspondence and feature-range behavior alongside glyph output.
4. **Final mark positioning.** Complete attachment positioning needed by Latin
   accents and Hebrew marks, including mark-to-base and mark-to-mark cases and
   marks on ligatures where applicable. Test final coordinates and advances,
   not only `AttachKind`, `AttachTo`, and anchor references.
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
