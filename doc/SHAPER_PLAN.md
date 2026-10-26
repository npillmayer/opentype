# otshape Plan Compilation and Execution

Validated against the source on 2026-10-26. This document describes the current
implementation and distinguishes internal machinery from public guarantees.
[TESTING.md](TESTING.md) records test evidence and reproduced correctness gaps.

## Scope and design

The plan is a compiled lookup schedule for one font, script, language, direction,
and feature set. Planning is separate from mutable glyph/position buffers.
Script engines depend on the base `otshape` package and contribute through
hooks; clients inject engines.

The earlier version of this document contained a detailed walkthrough of a
HarfBuzz-derived implementation and a proposed exported `Plan` API. The current
repository has no local `harfbuzz/` package. That walkthrough was architectural
background, not a description of the current code. The reused ideas are feature
collection, masks, table-specific schedules, and staged execution. Caching,
variation selection, direction-dependent feature programs, and broad fallback
behavior must not be inferred from that reference.

Current shaping executes GSUB/GPOS through `otlayout`; there is no legacy
`kern` fallback in this pipeline. Plans are built for each request, with
additional plans for event feature pushes. There is no cross-request plan cache.

## Source map and data model

| Source | Responsibility |
| --- | --- |
| [shape_api.go](../otshape/shape_api.go) | Selects an engine and creates the internal plan request from public parameters. |
| [shape_runtime.go](../otshape/shape_runtime.go) | Internal concrete `planCompiler` adapter used by ordinary and event input. |
| [plan.go](../otshape/plan.go) | Plan types, feature discovery, mask allocation, lookup scheduling, validation, and stage executor. |
| [plan_hooks.go](../otshape/plan_hooks.go) | Feature intents, fallback metadata, and direct/anchored pause adapters. |
| [lookups.go](../otshape/lookups.go) | Runtime mask filtering, lookup traversal, syllable isolation, and metadata edit propagation. |
| [plan_stack.go](../otshape/plan_stack.go) | Nested feature sets and plans for event input. |

All plan types are private. There is no exported `Plan`, `PlanRequest`,
`PlanCompiler`, or `PlanExecutor` API.

- `plan` contains the font, segment properties, script/language tags, masks,
  GSUB/GPOS programs, policy, pause hooks, diagnostic notes, feature ranges, and
  joiner glyph classification.
- `tableProgram` contains a flat `[]lookupOp`, `[]stage` ranges, and
  `[]featureBind`. Stages use inclusive `FirstLookup` and exclusive
  `LastLookup`, with an optional pause-hook ID.
- `lookupOp` carries lookup index, feature tag, mask, and runtime flags.
- `maskLayout` carries `GlobalMask` and per-tag mask/shift/default values.
- `planExecutor` acquires a mutable `runBuffer`; it does not store glyph data in
  the plan.

The schedule is compiled before use, but this is not an enforced immutable,
self-contained cache object: it contains slices/maps/hooks, and engines can keep
mutable plan state. `ShapeEvents` currently reuses one engine while compiling
multiple plans.

Some fields have weaker semantics than their names suggest:
`VarIndex` is copied but not used for variation selection;
`FoundScript` is based on the extracted feature-slice length rather than whether
the requested script matched; `featureBind.FeatureIndex` is an index in that
extracted slice, not necessarily the original font FeatureList index.

## Compilation path

`Shaper.Shape` selects an engine once, creates a `planCompiler`, and compiles
the default plan before reading input. `ShapeEvents` compiles a root plan and a
new plan for each push; pop restores a stored frame.

The current `compile(planRequest)` sequence is:

1. Resolve script/language tags when absent and initialize internal policy/hooks.
2. Create a planner seeded with base defaults and user toggles.
3. Call optional `CollectFeatures`, then `OverrideFeatures`.
4. Combine user ranges with engine mask requests and allocate masks.
5. Extract features independently from GSUB and GPOS.
6. Compile each table's selected features into lookup stages.
7. Append direct GSUB pause stages.
8. Compute unresolved-feature fallback needs and a resolved-feature view.
9. Call optional `PostResolveFeatures` to anchor GSUB pauses.
10. Construct the plan and joiner map; call `InitPlan`.
11. Call `ValidatePlan` if present, inside the `ShapingEnginePlanHooks` branch.
12. Validate structural plan invariants.

Thus a validation-only engine does not currently get `ValidatePlan` invoked.
User-facing strict mode and public plan diagnostics are not exposed.

## Feature discovery and scheduling

Default GSUB order is `locl, ccmp, rlig, rclt, calt, clig, liga`.
Default GPOS order is `abvm, blwm, mark, mkmk, curs, dist, kern`.
Engine additions extend these lists; adding a tag already present does not move
it. This differs from assuming that every engine collection call establishes a
new stage at that position.

`fontFeaturesForTable` selects the requested script or falls back to `DFLT`.
It then selects the requested language or the script's default LangSys.
There is no additional `dflt -> latn` search chain here. Script-tag conversion
itself is incomplete; see [TESTING.md](TESTING.md).

The extracted feature slice reserves slot zero for the required feature.
`compileTableProgram` activates required features, available defaults, and user
toggles. Required features are retained in the schedule when a global disable is
requested; the compiler records a warning. This scheduling rule alone does not
prove immunity from runtime mask filtering.

Stages are assigned to required features first, then active default tags, then
remaining active tags sorted by tag. Within a stage, lookup indexes are sorted
and duplicates merge masks and flags with bitwise OR. Duplicate lookups in
different stages remain separate executions.

Mask allocation processes user/engine mask requests before font feature
resolution. It allocates up to 31 bits in a `uint32`, with at most eight bits per
feature value. It does not implement shared HarfBuzz-style global-bit allocation.
Default features without an explicit mask can have a zero lookup mask, meaning
no mask gate. Engine-generated mask requests come from a map, so stable numeric
mask assignments across compilations are not a supported guarantee.

## Flags and pauses: implemented semantics

| Flag/control | Current behavior |
| --- | --- |
| `FeatureManualZWNJ`, `FeatureManualZWJ` | Adjust automatic joiner skipping flags. |
| `FeaturePerSyllable` | Isolates GSUB spans. Default syllable IDs derive from clusters, not script syllable recognition. |
| `FeatureHasFallback` | Contributes unresolved-feature metadata for engine fallback decisions. |
| `FeatureRandom` | Sets a runtime flag that passes `alt=-1`; the GSUB alternate implementation selects the last alternative, not a random one. |
| `FeatureGlobal`, `FeatureGlobalSearch` | Declared but have no implemented handling in the planner/executor. |
| `AddGSUBPause` | Appends empty stages after the compiled GSUB lookup stages. It does not interleave pauses at feature-collection call sites. |
| `AddGSUBPauseBefore/After` | Anchors callbacks to resolved GSUB stages; returns false when no anchor can be attached. |
| `PauseHook` | Returns `error`; execution stops on error. There is no ignored boolean return. |

Anchored pauses are resolved by retained lookup feature tags. Public hooks expose
GSUB pauses only; there is no public GPOS pause API. Arabic's registered pause
callbacks are currently no-ops, so their presence does not implement fallback
work at those boundaries.

## Runtime execution and policy limits

After normalization/mapping, `shapeMappedRun` calls preprocess, reorder, and
pre-GSUB hooks, initializes masks, calls `SetupMasks`, then invokes the executor.
The executor runs GSUB and optionally GPOS, applies internal position policies,
and returns to the postprocess hook. See
[SHAPER_INTERFACES.md](SHAPER_INTERFACES.md) for the exact order.

Lookup application delegates to `otlayout.ApplyFeature`. The layout layer owns
glyph/position edits; `runBuffer.applyLayoutEdit` mirrors completed edits into
metadata. This includes edits completed before a later error. The public API
suppresses the failing chunk while preserving earlier sink output.

Important limits:

- The executor initializes masks again after `SetupMasks`, overwriting script
  masks. Ordinary feature arguments also do not reach alternate selection, and
  indexed ranges are applied in chunk-local coordinates.
- Internal `Strict`, `ZeroMarks`, and `FallbackMarkPos` policies and tests
  exist, but the public compile path does not enable them.
- `ApplyGPOS() == false` becomes an all-zero internal policy in the public path;
  `compile` replaces an all-zero policy with `ApplyGPOS=true`. The hook cannot
  currently reliably disable GPOS. This follows from source inspection.
- Without strict mode, missing tables or feature-extraction failures become
  private plan notes; they are not universally fatal. This contradicts the old
  blanket claim that the public shaper is strict about incomplete fonts.
- Mark/cursive positioning retains unresolved attachment references. Internal
  fallback positioning only records an attachment. Mark-zeroing helpers operate
  before nominal horizontal advances are added at output; their unit tests do
  not establish final zero advances through the public API.
- The sink contract is logical shaping order, including RTL. Final visual
  ordering belongs to the caller (SHAPE-009 resolved). A vertical-metrics
  pipeline and variation application remain absent from plan execution.

## Arabic fallback policy

The implemented Arabic fallback is narrow and deficiency-driven:

1. Feature resolution marks requested fallback-capable features as unresolved.
2. `InitPlan` records fallback needs and builds presentation-form mappings when
   cmap is usable.
3. `ValidatePlan` fails if fallback is needed but cmap is unavailable.
4. Postprocessing repairs only `.notdef` glyphs for which a mapping exists.
5. Missing presentation mappings leave `.notdef` unchanged.

Fallback needs include `rlig` and Arabic form features. This local validation
policy does not imply global strict mode or general Arabic shaping correctness.

## Status and next work

The old delivery steps for plan compilation, staged execution, edit propagation,
hook wiring, and basic GPOS tests have implementations. Remaining work includes
correct mask/value/range behavior, stage/pause semantics, public policy wiring,
final attachment coordinates, and context-safe streaming.

Event-plan state also needs isolation: compilation calls `InitPlan` repeatedly
on the same engine, while replay of a stored plan does not restore engine-local
state. A stateful engine such as Arabic can therefore retain state from a
different feature scope. This is an integration risk identified by inspection,
not a new regression test.

Use [TESTING.md](TESTING.md) for current evidence and regression priorities.
Passing structural and component tests does not establish HarfBuzz parity.



