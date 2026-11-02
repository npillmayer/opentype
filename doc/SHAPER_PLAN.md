# otshape Plan Compilation and Execution

Stage scheduling updated against the source on 2026-11-01 (SHAPE-020). This
document describes the current implementation and distinguishes internal machinery from public guarantees.
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
a combined plan rebuilt for each event carry from immutable scope assignments.
Push/pop does not compile plans. There is no cross-request plan cache.

## Source map and data model

| Source | Responsibility |
| --- | --- |
| [shape_api.go](../otshape/shape_api.go) | Selects an engine and creates the internal plan request from public parameters. |
| [shape_runtime.go](../otshape/shape_runtime.go) | Internal concrete `planCompiler` adapter used by ordinary and event input. |
| [plan.go](../otshape/plan.go) | Plan types and validation. |
| [plan_compile.go](../otshape/plan_compile.go) | Feature discovery, mask allocation, and lookup scheduling. |
| [plan_exec.go](../otshape/plan_exec.go) | Stage executor and run-mask preparation, including feature ranges. |
| [position_policies.go](../otshape/position_policies.go) | Post-GPOS position policies such as GDEF mark advance zeroing. |
| [plan_hooks.go](../otshape/plan_hooks.go) | Feature intents, fallback metadata, and direct/anchored pause adapters. |
| [lookups.go](../otshape/lookups.go) | Runtime mask filtering, lookup traversal, syllable isolation, and metadata edit propagation. |
| [plan_stack.go](../otshape/plan_stack.go) | Nested immutable feature sets and scope IDs for event input. |

All plan types are private. There is no exported `Plan`, `PlanRequest`,
`PlanCompiler`, or `PlanExecutor` API.

- `plan` contains the font, segment properties, script/language tags, masks,
  GSUB/GPOS programs, policy, pause hooks, diagnostic notes, feature ranges, and
  joiner glyph classification.
- `tableProgram` contains a flat `[]lookupOp`, `[]stage` ranges, and
  `[]featureBind`. Stages use inclusive `FirstLookup` and exclusive
  `LastLookup`, with an optional pause-hook ID.
- `lookupOp` carries lookup index, representative feature tag, union mask, runtime
  flags, and every contributing feature with its individual mask/required status.
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
2. Create a planner seeded with base defaults.
3. Call optional `CollectFeatures`, apply user settings, then call `OverrideFeatures`.
4. Derive lookup-selection toggles and allocate masks from the resolved settings.
5. Extract features independently from GSUB and GPOS.
6. Compile each table with its recorded stage assignments and pause boundaries.
7. Compute unresolved-feature fallback needs and a resolved-feature view.
8. Call optional `PostResolveFeatures` to anchor GSUB pauses.
9. Construct the plan and joiner map; call `InitPlan`.
10. Call `ValidatePlan` if present, inside the `ShapingEnginePlanHooks` branch.
11. Validate structural plan invariants.

Thus a validation-only engine does not currently get `ValidatePlan` invoked.
User-facing strict mode and public plan diagnostics are not exposed.

## Feature discovery and scheduling

Default GSUB features are `locl, ccmp, rlig, rclt, calt, clig, liga`;
default GPOS features are `abvm, blwm, mark, mkmk, curs, dist, kern`.
These lists select features; they do not set execution order. Explicit pauses
are the stage boundaries. Engine registrations use the current table stage,
with repeated registrations retaining the earliest stage. Unregistered defaults
and user features join the final stage. GSUB pauses do not advance GPOS stages;
there is currently no public GPOS pause API.

`fontFeaturesForTable` selects the requested script or falls back to `DFLT`.
It then selects the requested language or the script's default LangSys.
There is no additional `dflt -> latn` search chain here. Script-tag conversion
itself is incomplete; see [TESTING.md](TESTING.md).

The extracted feature slice reserves slot zero for the required feature.
`compileTableProgram` activates required features, available defaults, and user
toggles. Required features cannot be disabled and use an unconditional mask.
They join stage zero unless their tag was explicitly registered in another stage.
SHAPE-021 resolves settings in collect → user → override order. User ranges
inherit explicit collected defaults outside their bounds. Engine overrides are
tag-wide, superseding user ranges and scopes as well as globals. Shadowed
settings are discarded before mask allocation; stage registration stays separate.

Within each stage, lookup indexes are sorted and shared lookups execute once.
Duplicates in different stages remain separate executions. Mask unions preserve
unconditional activation: if any contributor has mask zero, the shared lookup's
mask is zero. All contributing tags remain in the resolved-feature view and can
anchor pauses; feature metadata describes each contributor's own flags.

Feature collection is deterministic: required first, then ascending tag. Shared
lookups AND automatic joiner flags and retain the first contributor's random and
per-syllable flags. This follows the merge operation in
[HarfBuzz 15 hb-ot-map.cc](https://github.com/harfbuzz/harfbuzz/blob/15.0.0/src/hb-ot-map.cc),
which does not OR random/per-syllable flags. The deterministic representative
rule avoids depending on unstable ordering of equal lookup indexes.

Masks allocate up to 31 bits, encoding feature values through per-feature value
tables. Engine requests are sorted for deterministic allocation. Default features
without an explicit mask and required features use zero, meaning unconditional.
For a shared alternate lookup, only active contributors at the checked start
supply values. Zero/default equals one; all negative values mean random. Equal
values apply once. Incompatible values return `ErrConflictingAlternates` when a
nonempty alternate set matches, including nested and extension lookups. Other
lookup kinds and uncovered glyphs do not fail just because values differ.
A retained `FeatureRandom` flag forces random selection after this conflict check.
This is the agreed API contract, intentionally independent of HarfBuzz's packed
mask numeric behavior for shared alternates (see SHAPE-020).

## Flags and pauses: implemented semantics

| Flag/control | Current behavior |
| --- | --- |
| `FeatureManualZWNJ`, `FeatureManualZWJ` | Adjust automatic joiner skipping flags. |
| `FeaturePerSyllable` | Isolates GSUB spans. Default syllable IDs derive from clusters, not script syllable recognition. |
| `FeatureHasFallback` | Contributes unresolved-feature metadata for engine fallback decisions. |
| `FeatureRandom` | Sets a runtime flag that passes `alt=-1` for random selection after checking contributor conflicts. |
| `FeatureGlobal`, `FeatureGlobalSearch` | Declared but have no implemented handling in the planner/executor. |
| `AddGSUBPause` | Closes the current GSUB stage, runs the callback, and opens the next. Nil callbacks still split stages; empty stages are retained. |
| `AddGSUBPauseBefore/After` | Anchors callbacks before/after the whole containing GSUB stage, not an individual feature; returns false when no anchor can be attached. |
| `PauseHook` | Returns `error`; execution stops on error. There is no ignored boolean return. |

Anchored pauses resolve through all selected feature bindings, including shared
contributors. Public hooks expose GSUB pauses only; there is no public GPOS pause API. Arabic's registered pause
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

- Masks initialized before `SetupMasks` are preserved. Feature values and ranges
  follow the SHAPE-001/002/003/011/018 contracts. SHAPE-016 extends the same
  original-coordinate group-start policy to event scopes. SHAPE-015 supplies
  conservative safe streaming within the shared Latin/Hebrew buffer contract.
- An internal `Strict` policy and its test exist, but the public compile path
  does not enable it. The former `ZeroMarks` and `FallbackMarkPos` policies
  were removed as unused on 2026-11-01.
- `ApplyGPOS() == false` becomes an all-zero internal policy in the public path;
  `compile` replaces an all-zero policy with `ApplyGPOS=true`. The hook cannot
  currently reliably disable GPOS. This follows from source inspection.
- Without strict mode, missing tables or feature-extraction failures become
  private plan notes; they are not universally fatal. This contradicts the old
  blanket claim that the public shaper is strict about incomplete fonts.
- Latin/Hebrew mark finalization and mark advances follow SHAPE-007/008. Cursive
  positioning remains separate. SHAPE-015 subsequently implements bounded
  conservative streaming; this scheduling fix makes no broader positioning or
  script-conformance claim.
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
public policy wiring,
script-specific positioning, and context-safe streaming.

Event-plan state also needs isolation: compilation calls `InitPlan` repeatedly
on the same engine, while replay of a stored plan does not restore engine-local
state. A stateful engine such as Arabic can therefore retain state from a
different feature scope. This is an integration risk identified by inspection,
not a new regression test.

Use [TESTING.md](TESTING.md) for current evidence and regression priorities.
Passing structural and component tests does not establish HarfBuzz parity.



