# otshape Codebase Guide

Validated against the source on 2026-10-26. This replaces the initial read-through
that described only the legacy normalization mapper. A public shaping pipeline,
feature plans, GSUB/GPOS execution, script hooks, clusters, and streaming now
exist. Completeness remains limited; [TESTING.md](TESTING.md) records the review,
test results, and reproduced failures.

## Public entry points and scope

Construct `otshape.NewShaper(engines...)`, then call `(*Shaper).Shape` or
`(*Shaper).ShapeEvents`. Candidate engines are injected explicitly; there is no
registry or package-level `Shape(req)` API.

`Params` contains a font, direction, script, language, and feature ranges.
`BufferOptions` controls streaming. `RuneSource` / `InputEventSource` supply
input and `GlyphSink.WriteGlyph` receives `GlyphRecord` values.

Each request assumes fixed font/script/language/direction. Automatic itemization,
font fallback, paragraph bidi layout, and vertical-metrics shaping are absent.
The sink contract is logical shaping order; visual ordering belongs to the
caller. HarfBuzz level-0 cluster grouping is agreed but not yet complete
(SHAPE-011). Variation fields in internal plans do not implement variable
font shaping. Layout execution uses GSUB/GPOS; there is no legacy kern fallback.

## Source navigation

| Files / packages | Current responsibility |
| --- | --- |
| [doc.go](../otshape/doc.go), [shaper.go](../otshape/shaper.go) | Package documentation, helpers, `Params`, and `FeatureRange`. |
| [pipeline.go](../otshape/pipeline.go) | Rune input, glyph sink/record, flush modes, and buffering options. |
| [shape_api.go](../otshape/shape_api.go) | Orchestrator construction, engine selection, ordinary streaming loop, normalization/mapping, hook execution, and output materialization. |
| [shape_events.go](../otshape/shape_events.go), [input_events.go](../otshape/input_events.go), [plan_stack.go](../otshape/plan_stack.go) | Event source adapter/validation, nested feature scopes, plan IDs, and event-shaped spans. |
| [shape_streaming.go](../otshape/shape_streaming.go), [shape_runtime.go](../otshape/shape_runtime.go) | Watermarks, raw carry, cut selection, compaction, shared adapters, and reusable workspace. |
| [plan.go](../otshape/plan.go), [plan_hooks.go](../otshape/plan_hooks.go) | Private plan structures, feature/mask compilation, stages, pauses, fallback metadata, and execution. |
| [lookups.go](../otshape/lookups.go), [annotate.go](../otshape/annotate.go) | Lookup traversal/mask gating, metadata edits, cluster-derived syllables, and glyph-based joiner annotations. |
| [runbuffer.go](../otshape/runbuffer.go) | Internal glyph/position storage with aligned metadata arrays and edit/insertion support. |
| [engine_api.go](../otshape/engine_api.go), [engine_internal.go](../otshape/engine_internal.go) | Public engine hooks and narrow context interfaces; adapters to private pipeline state. |
| [language.go](../otshape/language.go) | Script/language tag mapping and default decomposition preferences. |
| [buffer.go](../otshape/buffer.go) | Legacy `Buffer` mapping/representation helpers; not the active shaping pipeline. |
| [otcore](../otshape/otcore/), [othebrew](../otshape/othebrew/), [otarabic](../otshape/otarabic/) | Concrete engines and their script/public-API tests. |
| [otlayout](../otlayout/), [otquery](../otquery/), [ot](../ot/) | Lookup execution, cmap/metrics queries, and parsed font tables respectively. |

## Data flow and storage

The implemented architecture uses separate arrays internally and per-glyph
records at the output boundary.

`runBuffer` contains `Glyphs`, optional `Pos`, and aligned `Codepoints`,
`Clusters`, `PlanIDs`, `Masks`, `UnsafeFlags`, `Syllables`, and `Joiners`.
Active side arrays must have the same length as glyph storage. During layout,
`otlayout` owns glyph/position edits and reports completed edits; `otshape`
updates metadata through `applyLayoutEdit`. Runtime hook edits use `RunContext`.

The public output is `GlyphRecord`: GID, `otlayout.PosItem`, cluster, mask, and
unsafe flags. It is richer than legacy `ShapedGlyph{Index, Advance}`.
Position materialization adds nominal horizontal advance to layout deltas.
After GPOS in Latin/Hebrew plans, GDEF marks receive a delta that cancels this
metric, producing zero final advances without changing placement offsets.
Then mark-to-base/ligature/mark attachment offsets are finalized from design-unit
anchors, target placement, and final pen differences, before `PostprocessRun`.
Cursive and hint/device/variation adjustments remain unresolved; ligature
component association and streaming context remain separate limitations.

The ordinary request path is:

1. Validate inputs, select an engine, and compile a private plan.
2. Fill raw carry, normalize, and map through cmap.
3. Run script hooks and staged GSUB/GPOS.
4. Choose and emit a prefix, compact raw carry, and repeat.

The workspace reuses allocations, but reshapes uncommitted carry each cycle.
See [STREAMING_API.md](STREAMING_API.md) for exact cut semantics and limitations.

## Normalization and the legacy mapper

Active normalization is in `shape_api.go`. Engine policy selects auto, none,
composed, or decomposed mode. Auto defaults to composed, with decomposition
preferences for `dev2` and `bng2`. Decomposition operates per rune; composition
uses adjacent pairs and optional engine composition hooks.

This is not full font-aware normalization: general canonical ordering across
combining sequences and cmap-aware Unicode composition/decomposition selection
are incomplete.

The recursive representation search in `buffer.go` tries a nucleus/mark
combination supported by the font. It is legacy code, not reused by the active
streaming normalizer. Its old `mapGlyphs` writes into a preallocated `Buffer`,
and `applySubstitutionFeature` remains empty. These facts no longer imply that
the whole package lacks feature execution.

## Script/language mapping and engines

Script mapping works for entries such as `Latn`, `Arab`, `Hebr`, and
`Deva`. Many remaining keys use full names instead of ISO codes, so `Syrc`,
`Taml`, and `Mlym`, for example, return `DFLT`.

Language matching uses a small curated set: Arabic, Chinese, English, Greek,
German, Hebrew, Japanese, Portuguese, Romanian, Russian, and Turkish.
Unmatched/low-confidence cases fall back to `DFLT`.

Core provides a generic baseline; Hebrew contributes composition/reordering;
Arabic/Syriac contributes feature staging, joining forms/masks, reordering, and
selected postprocessing/fallback operations. There are no dedicated Indic
engines. Default syllable annotations are cluster-derived.
See [SHAPER_INTERFACES.md](SHAPER_INTERFACES.md) for exact hooks and lifetimes.

## Implementation state versus remaining work

Implemented: public source/sink API, feature planning and ordering, GSUB/GPOS
dispatch, edit-aware metadata, script hook interfaces, event feature stacks,
streaming mechanics, error/partial-progress tests, exact forward lookup starts,
contextual continuation after the edited input span, and benchmarks.

Known limitations include:

- Ranges splitting merged source sequences remain unresolved. Mask preservation,
  source-coordinate ranges, alternate selection, and exact forward lookup starts
  have SHAPE-001/002/003/018 ordinary regressions.
- Streaming can discard needed shaping context; forced flushing does not prove
  equivalence to a complete run.
- GPOS cursive attachments lack final coordinate resolution. Mark attachments
  are finalized for horizontal Latin/Hebrew; component association, streaming
  context, and normalization remain partial.
- Internal policy support is not fully wired to the public API; declared flags
  and hooks do not all have their intended behavior.
- Multiple event plans share mutable engine-local state.

[SHAPER_PLAN.md](SHAPER_PLAN.md) describes compilation, scheduling, flags,
policies, and pauses as actually implemented. It supersedes the old proposals
for exported compiler/executor APIs and claims of complete script behavior.

## Testing and diagnostics

The former proof-image failure and proposed first GSUB milestone are historical,
not the current test state. `TestBufferFunctions` is skipped, while the active
pipeline has substantial tests. The review's full suite passed, but public-API
probes found failures outside the committed regression coverage.

[TESTING.md](TESTING.md) is the test directory guide, coverage snapshot, and
regression backlog. It distinguishes structural font tests, lookup tests,
pipeline/script tests, and the HarfBuzz fixture runner. The latter currently has
only a Gentium `AB` fixture, so passing it does not establish complex-script
parity.

`ot-tools` provides `shape`, `view`, and `font` diagnostics; see
[OT_TOOLS.md](OT_TOOLS.md). Allocation benchmarks and their scope are described
in [SHAPING_BENCHMARKS.md](SHAPING_BENCHMARKS.md).

The next milestone is correcting end-to-end behavior and expanding independent
expected-output tests. The initial plan to add an entry point and first feature
execution loop has already been implemented.
