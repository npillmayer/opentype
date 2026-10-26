# Shaper Interfaces and Pipeline Interaction

Validated against the source on 2026-10-26. The authoritative declarations are
in [engine_api.go](../otshape/engine_api.go); this document describes their
current use by the pipeline, including mismatches between comments and behavior.

The previous version mixed a HarfBuzz-derived reference design with an
`otshape` API proposal. The local `harfbuzz/` package is absent; names such as
`ShapeRequest`, `ShapeOptions`, `PreprocessText`, and `PostprocessGlyphs` do
not describe the current API.

## Dependencies and client wiring

`otshape` owns orchestration, engine selection, interfaces, and internal plans.
`otshape/otcore`, `otshape/othebrew`, and `otshape/otarabic` import the base
package. The base package does not import concrete engines, including in its
current tests. Script-specific tests belong in the engine subpackages.
Clients such as `ot-tools` and `internal/hbcmp` assemble the engine list.

There is no registry, `Register` call, or automatic discovery.

```go
shaper := otshape.NewShaper(otcore.New(), otarabic.New(), othebrew.New())
err := shaper.Shape(params, source, sink, options)
```

Current entry-point signatures are:

```go
func NewShaper(engines ...ShapingEngine) *Shaper
func (s *Shaper) Shape(params Params, src RuneSource, sink GlyphSink, bufOpts BufferOptions) error
func (s *Shaper) ShapeEvents(params Params, src InputEventSource, sink GlyphSink, bufOpts BufferOptions) error
```

These are method-based calls, not package-level `Shape(req)` functions.
`ShapeEventsRequest` remains as a deprecated type but is not accepted by these
methods. See [STREAMING_API.md](STREAMING_API.md) for sources, sinks, and events.

## Engine selection and lifetime

```go
type ShapingEngine interface {
    Name() string
    Match(ctx SelectionContext) ShaperConfidence
    New() ShapingEngine
}
```

`SelectionContext` supplies direction, ISO script, BCP 47 language, and derived
OpenType script/language tags. `ShaperConfidence` is a named integer type with
`None, Low, Medium, High, Certain` values starting at zero.

Current selection behavior in [shape_api.go](../otshape/shape_api.go):

1. `NewShaper` copies candidates and ignores nil interface entries.
2. An empty list returns `ErrNoShaper`.
3. Even a single engine is scored. Highest score wins; equal scores use the
   lexicographically smaller name, with the first candidate retained for equal
   names.
4. Negative scores are skipped. **Known bug:** zero (`None`) is accepted, so an
   explicitly nonmatching engine can still win. `ErrNoMatchingShaper` does not
   currently protect the ordinary all-`None` case.
5. The winner's `New` is called once per shaping request. If it returns nil,
   the candidate itself is reused.

This is not fresh-per-plan construction. One selected engine is reused across
streaming cycles and, for `ShapeEvents`, all compiled feature-scope plans.
Stateful engines should supply independent instances from `New`; event scope
state isolation remains an implementation gap.

## Optional hooks

Hooks are discovered with type assertions. They are not all mandatory methods
on an engine.

| Interface | Method(s) and current role |
| --- | --- |
| `ShapingEnginePolicy` | `NormalizationPreference() NormalizationMode`; `ApplyGPOS() bool`. |
| `ShapingEnginePlanHooks` | `CollectFeatures(FeaturePlanner, SelectionContext)`, `OverrideFeatures(FeaturePlanner)`, `InitPlan(PlanContext)`. |
| `ShapingEnginePlanValidateHook` | `ValidatePlan(PlanContext) error`, called after init only when the engine also implements `ShapingEnginePlanHooks`. |
| `ShapingEnginePostResolveHook` | `PostResolveFeatures(ResolvedFeaturePlanner, ResolvedFeatureView, SelectionContext)`. |
| `ShapingEnginePreprocessHook` | `PreprocessRun(RunContext)`; currently runs after normalization/mapping, despite its pre-normalization source comment. |
| `ShapingEngineComposeHook` | `Compose(NormalizeContext, rune, rune) (rune, bool)`, during pair composition. |
| `ShapingEngineReorderHook` | `ReorderMarks(RunContext, int, int)`, currently invoked once over the mapped run. |
| `ShapingEnginePreGSUBHook` | `PrepareGSUB(RunContext)`, after reorder. |
| `ShapingEngineMaskHook` | `SetupMasks(RunContext)`, after initial mask setup; its changes are currently overwritten by executor mask initialization. |
| `ShapingEnginePostprocessHook` | `PostprocessRun(RunContext)`, after lookup/position-policy execution. |

There is no custom decomposition hook, `MarksBehavior`, `GposTag`, or
script-specific streaming-boundary hook. Normalization modes are `Auto`,
`None`, `Composed`, and `Decomposed` (with the `Normalization` prefix).

The public path reads `ApplyGPOS`, but a false value produces an all-zero
internal policy that compilation resets to GPOS-on. Internal strictness,
mark-zeroing, and fallback-positioning switches are not exposed by the policy
interface. See [SHAPER_PLAN.md](SHAPER_PLAN.md).

## Context interfaces

| Context | Available operations |
| --- | --- |
| `FeaturePlanner` | Enable/add/disable feature intents, register direct GSUB pauses, and query `HasFeature`. The latter checks intents/defaults, not actual font support. |
| `ResolvedFeaturePlanner` | Attach GSUB pauses before/after a resolved tag; methods return whether attachment succeeded. |
| `ResolvedFeatureView` | Read selected GSUB/GPOS features and resolution metadata. |
| `PlanContext` | Font, selection metadata, first mask bit for a feature, and whether a feature needs fallback. |
| `RunContext` | Read/write glyphs, codepoints, clusters, positions, and masks; merge clusters; insert glyphs/copies; swap records. |
| `NormalizeContext` | Font/selection, Unicode pair composition, and `HasGposMark`. |
| `PauseContext` | Font and a `RunContext`. |

`RunContext` keeps the private `runBuffer` hidden. It does not expose setters
for unsafe flags, syllables, or plan IDs. Codepoint setters mutate metadata; they
do not automatically remap the glyph.

`HasGposMark` is currently computed from allocated `mark/mkmk` masks, not from
a definitive check that corresponding GPOS lookups will execute. Do not treat it
as a complete font capability query.

## Actual plan-time order

In [plan.go](../otshape/plan.go):

1. Seed base feature defaults and user toggles.
2. `CollectFeatures`, then `OverrideFeatures`.
3. Allocate masks and resolve table features/stages.
4. Append direct GSUB pauses and compute fallback/resolved-feature metadata.
5. `PostResolveFeatures` may anchor GSUB pauses.
6. `InitPlan`, followed by `ValidatePlan` when both interfaces are implemented.
7. Validate the internal plan.

`PauseHook` is `func(PauseContext) error`; an error stops execution.
Direct pauses currently append empty stages after all GSUB lookups. They are
not interleaved at their collection call sites. Anchored pauses attach to
resolved lookup stages. The public API has no GPOS pause-registration methods.

## Actual runtime order

For each current raw carry buffer, or each contiguous event-plan span:

1. Normalize runes; consult `Compose` when enabled by normalization mode.
2. Map runes through cmap into a `runBuffer`.
3. `PreprocessRun`.
4. `ReorderMarks(run, 0, run.Len())`.
5. `PrepareGSUB`.
6. Initialize feature masks and apply indexed ranges.
7. `SetupMasks`.
8. Enter `planExecutor.apply`, which **initializes masks again**.
9. Annotate GSUB joiners/syllables and execute GSUB stages and pauses.
10. Execute GPOS when the compiled policy enables it.
11. Apply internal position policies.
12. `PostprocessRun`.
13. Select an output prefix and emit `GlyphRecord` values, adding nominal
    horizontal advances during materialization.

This order is taken from [shape_api.go](../otshape/shape_api.go) and
[plan.go](../otshape/plan.go), not from the earlier proposed hook order.
There is no general combining-class sort before the reorder hook, dotted-circle
prelude, default-ignorable cleanup, or direction-restore phase here.
Mark/cursive anchor resolution is also incomplete.

Because carry is reshaped, runtime hooks can run more than once for input that
has not yet been emitted. Hooks must not assume one invocation per source rune.

## Supplied engines

| Engine | Current contribution |
| --- | --- |
| [otcore](../otshape/otcore/otcore.go) | Minimal engine and normalization/GPOS policy; no custom run hooks. Latin gets high confidence before the direction check; other LTR scripts get low confidence, other directions get `None`. |
| [othebrew](../otshape/othebrew/othebrew.go) | Hebrew matching, policy, Unicode/presentation-form composition, and selected mark reorder behavior. No custom feature-plan or mask hooks. |
| [otarabic](../otshape/otarabic/otarabic.go) | Arabic/Syriac matching, policy, feature collection, plan init/validation, post-resolve pauses, joining preparation, masks, mark reordering, and postprocessing. |

Arabic stores masks/fallback data on the engine in `InitPlan`, computes joining
forms in `PrepareGSUB`, and uses them in `SetupMasks` and postprocessing.
Its pause callbacks are currently no-ops. Postprocessing includes simplified
tatweel expansion and narrow `.notdef` repair, not a general fallback shaper.
The mask reset prevents assuming correct public-pipeline joining behavior.

In event mode, compiling a later plan mutates this engine-local state.
Processing an older stored plan or popping a feature scope does not restore it.
The multiple-plan/stateful-engine combination needs dedicated integration tests.

## Testing and pending contracts

Base-package tests use synthetic engines; concrete-engine behavior is tested in
the engine subpackages. Existing tests cover composition/reorder hooks,
validation, pauses, masks, and script helpers. The public-API probes and coverage
limits are recorded in [TESTING.md](TESTING.md).

Before treating these interfaces as stable behavioral contracts, resolve mask
preservation, validation-hook gating, preprocessing order, GPOS opt-out,
direct-pause placement, and per-plan engine state. Source comments and a
declared interface alone do not demonstrate that every policy is honored.
