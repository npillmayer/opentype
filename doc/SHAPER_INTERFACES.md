# Shaper Interfaces and Pipeline Interaction (harfbuzz)

## 1. Architecture and Dependency Direction

The codebase uses a split design:

1. `harfbuzz/` (base package) owns:
   - shaping pipeline and plan execution,
   - shaper registry and selection,
   - core interfaces and hook wrappers.
2. `harfbuzz/otcore`, `harfbuzz/othebrew`, `harfbuzz/otarabic` own script-/engine-specific behavior.
3. Dependency direction is one-way:
   - shaper packages import `harfbuzz`,
   - `harfbuzz` does not import those shaper packages in non-test code.

This matches the desired direction for `otshape`: engine packages depend on the base package, never the reverse.  
  
**Remarks:** 
- Package `harfbuzz` is not even in test-code allowed to depend on a concrete shaper sub-package.
- Shaper registry and selection should only be ported partially: No registry (parameters instead, see below), no "registration" of shapers, but “voting” for confidence via `Match()`.
- For `otshape`, keep this strict rule for tests as well: shaper-specific tests belong to the shaper package, not the base package.

## 2. Interface Model in `harfbuzz`

### 2.1 Mandatory engine interface

`ShapingEngine` is intentionally small:

1. `Name() string`
2. `Match(ctx SelectionContext) int`
3. `New() ShapingEngine`

Selection is score-based (`Match >= 0`), highest score wins; tie-break is deterministic by name/order.

### 2.2 Optional hook interfaces

Behavior is extended via optional interfaces (type assertions in wrapper helpers):

1. `ShapingEnginePolicy`
   - `MarksBehavior()`
   - `NormalizationPreference()`
   - `GposTag()`
2. `ShapingEnginePlanHooks`
   - `CollectFeatures()`
   - `OverrideFeatures()`
   - `InitPlan()`
3. `ShapingEnginePostResolveHook`
   - `PostResolveFeatures()`
4. Runtime hooks
   - `ShapingEnginePreprocessHook` (`PreprocessText`)
   - `ShapingEnginePreGSUBHook` (`PrepareGSUB`)
   - `ShapingEngineDecomposeHook` / `ShapingEngineComposeHook`
   - `ShapingEngineMaskHook` (`SetupMasks`)
   - `ShapingEngineReorderHook` (`ReorderMarks`)
   - `ShapingEnginePostprocessHook` (`PostprocessGlyphs`)

### 2.3 Narrow context interfaces passed to hooks

The base package does not expose full internals to engines. It passes narrow views:

1. `FeaturePlanner` (plan-time feature collection)
2. `ResolvedFeaturePlanner` + `ResolvedFeatureView` (post-resolution stage anchor control)
3. `PlanContext` (engine plan init)
4. `NormalizeContext` (compose/decompose decisions)
5. `PauseContext` (GSUB stage pause callback context)

## 3. Plan-Time Interaction Between Pipeline and Engine

Plan building path:

1. `Buffer.Shape(...)` creates/uses `shapePlan`.
2. `shapePlan.init` initializes `shaperOpentype`.
3. `shaperOpentype.compile(...)` -> `otShapePlan.init0(...)`.
4. `newOtShapePlanner(...)` selects engine using `SelectionContext`.
5. `otShapePlanner.CollectFeatures(...)`:
   - adds base default features,
   - calls engine `CollectFeatures(...)`,
   - then engine `OverrideFeatures(...)`.
6. `otMapBuilder.compile(...)` resolves features and stages.
7. During map compile, engine `PostResolveFeatures(...)` may add pause anchors (`before/after feature tag`).
8. After compile, engine `InitPlan(...)` runs with `PlanContext`.

## 4. Runtime Interaction (Pipeline Phase Order)

`shaperOpentype.shape(...)` executes phases in this order:

1. Buffer prelude:
   - initialize masks,
   - set unicode props,
   - insert dotted circle,
   - form clusters,
   - ensure native direction.
2. `PreprocessText(...)` hook.
3. Substitution pre-position (`substituteBeforePosition`):
   - normalization (`otShapeNormalize`) where:
     - compose/decompose hooks are consulted,
     - mark reorder hook runs after combining-class sort,
   - mask setup (`SetupMasks`) where:
     - `PrepareGSUB(...)` hook runs,
     - then `SetupMasks(...)` hook runs,
     - then user ranged masks are applied.
4. GSUB/GPOS application through compiled `otMap.apply(...)`:
   - lookups run by stage,
   - stage pause callbacks run between stages.
5. Positioning phase.
6. Post-substitution cleanup:
   - hide default ignorables,
   - `PostprocessGlyphs(...)` hook.
7. Cluster flag propagation and direction restore.

## 5. GSUB Pause Hooks and Stage Anchoring

Pause callbacks are first-class plan controls:

1. Planner can add direct pauses (`AddGSUBPause`).
2. Post-resolve hook can add anchored pauses (`AddGSUBPauseBefore/After(tag)`).
3. `otMapBuilder` resolves anchors to concrete stage indices.
4. `otMap.apply` executes pauses at stage boundaries.

Note: `GSUBPauseFunc` returns `bool`, but current runtime path does not branch on that return value.

## 6. Engine Package Responsibilities

### 6.1 `otcore`

1. Thin adapter over default engine (`harfbuzz.NewDefaultShapingEngine`).
2. No custom script behavior.

### 6.2 `othebrew`

Implements a narrow set:

1. `ShapingEngine` + `ShapingEnginePolicy`
2. `ShapingEngineComposeHook`
3. `ShapingEngineReorderHook`

Hebrew behavior is mostly normalization/mark-order specific (compose/reorder), without custom plan-stage or mask hooks.

### 6.3 `otarabic`

Implements a broad set:

1. `ShapingEngine` + `ShapingEnginePolicy`
2. `ShapingEnginePlanHooks`
3. `ShapingEnginePostResolveHook`
4. `ShapingEnginePreGSUBHook`
5. `ShapingEngineMaskHook`
6. `ShapingEngineReorderHook`
7. `ShapingEnginePostprocessHook`

Arabic uses plan-time feature programming plus runtime state:

1. Collects Arabic/Syriac shaping features, including manual joiner behavior and staged pauses.
2. Adds post-resolve anchored pauses (e.g. after `stch`, after `rlig` for fallback shaping).
3. Computes joining state in `PrepareGSUB`.
4. Applies per-glyph feature masks in `SetupMasks`.
5. Runs fallback synthesis and stretch postprocessing via pause/postprocess logic.

## 7. Registration and Discovery

1. Base registry includes default engine only by default.
2. Script engines are opt-in via `Register()` from subpackages.
3. Tests often register Hebrew/Arabic in `init()` to exercise split-shaper behavior.

**Remarks:** 
- Shapers should be given as arguments to the top-level `Shape(…)` call (possibly packaged together with other parameters)
- There will be no registry for now. Shapers will have to be wired into the call-graph
- The `Register()` call semantics are not to be ported
- A call to `Shape(…)` may be performed without the client supplying a concrete shaper, resulting in an error (for now)
- There is no "automatic discovery" for shapers from the pipeline. The wiring (dependency injection) of a shaper to the pipeline has to be done on client-level

## 8. Practical Pattern to Reuse in `otshape`

A direct transferable pattern is:

1. Keep one minimal mandatory engine interface (`Name/Match/New`).
2. Add optional narrow hook interfaces per pipeline phase.
3. Pass phase-specific context interfaces instead of exposing full internals.
4. Keep shaper packages independent and wired/injected from outside the base package.
5. Keep pipeline->hook call order explicit and stable so engine behavior remains deterministic.

## 9. Draft `otshape` API Proposal

This section proposes concrete API shapes for `otshape`, aligned with the remarks above.

### 9.1 Top-level call (no registry, explicit injection)

```go
type ShapeRequest struct {
	Options ShapeOptions
	Source  RuneSource
	Sink    GlyphSink
	Shapers []ShapingEngine // injected by client; no global registry
}

func Shape(req ShapeRequest) error
```

Semantics:

1. If `len(req.Shapers) == 0`, return `ErrNoShaper`.
2. If one shaper is provided, use it directly.
3. If multiple shapers are provided, call `Match(...)` and select highest score (`<0` means no match).
4. If none match, return `ErrNoMatchingShaper`.

### 9.2 Mandatory shaper interface

```go
type SelectionContext struct {
	Direction bidi.Direction
	Script    language.Script
	Language  language.Tag
	ScriptTag ot.Tag
	LangTag   ot.Tag
}

type ShapingEngine interface {
	Name() string
	Match(ctx SelectionContext) int // voting/confidence
	New() ShapingEngine             // fresh per-plan instance
}
```

### 9.3 Optional hook interfaces (phase-based)

```go
type ShapingEnginePolicy interface {
	NormalizationPreference() NormalizationMode
	ApplyGPOS() bool
}

type ShapingEnginePlanHooks interface {
	CollectFeatures(plan FeaturePlanner, ctx SelectionContext)
	OverrideFeatures(plan FeaturePlanner)
	InitPlan(plan PlanContext)
}

type ShapingEnginePostResolveHook interface {
	PostResolveFeatures(plan ResolvedFeaturePlanner, view ResolvedFeatureView, ctx SelectionContext)
}

type ShapingEnginePreprocessHook interface {
	PreprocessRun(run RunContext)
}

type ShapingEnginePreGSUBHook interface {
	PrepareGSUB(run RunContext)
}

type ShapingEngineMaskHook interface {
	SetupMasks(run RunContext)
}

type ShapingEnginePostprocessHook interface {
	PostprocessRun(run RunContext)
}
```

Notes:

1. Keep contexts narrow; do not expose internal `runBuffer` directly.
2. Keep hooks optional via type assertions in the base pipeline.
3. Add compose/decompose/reorder hooks only if needed by the first shaper ports.

### 9.4 Planner/runtime call sequence

Recommended call order for one shaped segment:

1. Select engine from `req.Shapers`.
2. Build plan:
   - `CollectFeatures` -> feature resolution/compile -> `PostResolveFeatures` -> `InitPlan`.
3. Runtime:
   - `PreprocessRun`
   - normalization/mapping
   - `PrepareGSUB`
   - `SetupMasks`
   - apply GSUB stages (+ pauses)
   - apply GPOS (policy-controlled)
   - `PostprocessRun`
   - emit to `GlyphSink`

### 9.5 Package and testing rules for `otshape`

1. `otshape` must not import concrete shaper packages.
2. Concrete shaper packages (`otshapecore`, `otshapehebrew`, `otshapearabic`, ...) may import `otshape`.
3. `otshape` tests must not import concrete shaper packages.
4. Shaper-specific behavior tests belong in the respective shaper package.
