# Refactoring Plans

## Normalization Refactor Checklist (x/text)

This checklist describes a safe migration path to treat `golang.org/x/text/unicode/norm` as the
canonical Unicode normalization source while preserving HarfBuzz-specific shaping behavior.

- Scope: focus on OpenType shaping for Latin/Cyrillic/Hebrew/Arabic. Chinese-specific behavior is
  out of scope for this refactor.
- Non-goal: do not replace glyph-availability gating, variation-selector handling, cluster merging,
  or script-specific mark reordering with generic Unicode normalization calls.

Phase 0 (baseline and guardrails):

1. Capture current behavior with `go test .` and keep a list of representative shaping cases for
   Latin/Hebrew/Cyrillic/Arabic.
2. Record key invariants to preserve: cluster monotonicity, no spurious glyph-not-found regressions,
   stable fallback behavior for space/U+2011 handling.

Phase 0 baseline commands:

```bash
go version
go test . -count=1
go test . -run TestPhase0 -count=1 -v
```

- Record the date and Go version next to the baseline run in this file.
- Treat `ot_shape_normalize_phase0_test.go` as the frozen guardrail suite for:
  canonical-equivalence checks (Latin/Cyrillic/Arabic), cluster monotonicity, `.notdef` regressions,
  and space/U+2011 fallback behavior.

Phase 1 (backend abstraction, no behavior change):

1. Introduce an internal normalization backend abstraction used only by `ot_shape_normalize.go`.
2. Keep the current backend as default (`internal/unicodedata`) so runtime behavior is unchanged.
3. Add a second backend implemented with `x/text/unicode/norm` primitives for canonical data access.

Status checkpoint (2026-02-07):

- Phase 0 is complete.
- Phase 0 guardrails are implemented in `ot_shape_normalize_phase0_test.go`.
- Phase 1 is complete.
- Backend abstraction and implementations are in `ot_normalize_backend.go`.
- `ot_shape_normalize.go` at this checkpoint built an `otNormalizeContext` with
  `defaultOTNormalizeBackend()` (before Phase 2 selector wiring).
- `complexShaperNil` and Hebrew compose hooks now call normalization primitives through context
  methods (`decomposeUnicode` / `composeUnicode`) instead of direct `uni.compose/decompose`.
- Runtime behavior is unchanged: default backend remains `internal/unicodedata`.
- `x/text` backend is present but not selected as default yet.
- Validation pass at this checkpoint:
  `go test . -run 'TestOTNormalizeBackendParitySmoke|TestPhase0' -count=1 -v`
  and `go test . -count=1`.

Phase 2 (hybrid integration):

1. Route canonical combining-class lookup through the backend.
2. Route per-rune canonical decomposition through the backend.
3. Route pair composition checks through the backend.
4. Keep these paths unchanged: glyph checks (`NominalGlyph`), shaper hooks (`compose`/`decompose`/
   `reorderMarks`), variation selector clusters, and cluster bookkeeping.

Status checkpoint (2026-02-07):

- Phase 2 is complete.
- `ot_shape_normalize.go` now initializes normalization with `currentOTNormalizeBackend()` and
  applies modified combining classes from backend canonical CCC values inside normalization output
  paths.
- Canonical decomposition and composition continue to flow through shaper hooks backed by the
  selected normalization backend.
- Variation-selector handling, glyph availability gating, and cluster bookkeeping are unchanged.
- Backend parity test coverage was expanded in `ot_normalize_backend_test.go`:
  representative-script parity and reference-case parity (legacy vs `x/text`).
- Midpoint reevaluation gate passed:
  `go test . -run 'TestOTNormalizeBackendParitySmoke|TestPhase2NormalizationBackendParityRepresentative|TestPhase2NormalizationBackendParityReferenceCases|TestPhase0' -count=1 -v`
  and `go test . -count=1`.

Midpoint reevaluation gate (mandatory before cleanup):

1. Run `go test .`.
2. Diff shaping outputs for the representative script set.
3. Stop and fix before proceeding if regressions are not clearly attributable to intended canonical
   Unicode behavior.

Phase 3 (switch default backend):

1. Make the `x/text` backend the default for canonical normalization primitives.
2. Keep legacy backend available behind a temporary debug switch for side-by-side comparison.

Status checkpoint (2026-02-07):

- Phase 3 is complete.
- Default canonical normalization backend selection is now `x/text`.
- A temporary process-level debug switch was introduced for side-by-side comparison
  during Phase 3; it is removed in Phase 4 cleanup.
- Selection/parsing checks were added in `ot_normalize_backend_test.go`.
- Validation pass at this checkpoint:
  `go test . -run 'TestParseOTNormalizeBackendKind|TestCurrentOTNormalizeBackendSelection|TestOTNormalizeBackendParitySmoke|TestPhase2NormalizationBackendParityRepresentative|TestPhase2NormalizationBackendParityReferenceCases|TestPhase0' -count=1 -v`
  and `go test . -count=1`.

Phase 4 (cleanup):

1. Remove temporary comparison wiring once behavior is accepted.
2. Minimize direct reliance on `internal/unicodedata` for canonical compose/decompose in this
   package, while retaining HarfBuzz-specific modified combining class and shaping heuristics.

Status checkpoint (2026-02-07):

- Phase 4 is complete.
- Temporary comparison wiring was removed from `ot_normalize_backend.go`:
  no backend kind selector, no environment override, no testing swap helper.
- Normalization backend selection is now fixed to `x/text` through
  `currentOTNormalizeBackend()`.
- Legacy canonical backend implementation used only for comparison was removed.
- `ot_normalize_backend_test.go` now contains focused `x/text` smoke tests only.
- HarfBuzz-specific modified combining class mapping and script heuristics in shaping are unchanged.

Risk notes:

1. Avoid full-cluster `NFD`/`NFC` transforms in shaping loops; they may trigger stream-safe behavior
   not suitable for cluster-level shaping normalization.
2. Keep script-specific overrides (notably Hebrew/Arabic shaping nuances) as first-class behavior.


## Refactoring of Shaper Engine

The current architecture is one shared OpenType pipeline with script-specific hooks, not two fully
separate shaping engines. A single script selector chooses one shaper implementation, then the
shared GSUB/GPOS pipeline calls hook methods at defined points.

### Implemented Interface Boundary

`ShapingEngine` is now the minimal selector/factory contract (`Name`, `Match`, `New`) in
`refactoring.go`. The shared OT executor remains unchanged.

Current hook groups:

- Policy hooks: `ShapingEnginePolicy` (`marksBehavior`, `normalizationPreference`, `gposTag`).
- Plan-time hooks: `ShapingEnginePlanHooks` (`collectFeatures`, `overrideFeatures`, `initPlan`).
- Runtime hooks: preprocess, mask setup, mark reorder, postprocess.
- Normalization hooks: split into separate optional compose/decompose hooks.
- Dispatch model: base pipeline call-sites use explicit helper dispatch with default behavior when
  a hook interface is not implemented.

### Extracted Narrow Interfaces

- `FeaturePlanner`: exposes only feature-map and GSUB-pause operations needed during
  `collectFeatures`/`overrideFeatures`.
- `NormalizeContext`: exposes normalization primitives (`decomposeUnicode`, `composeUnicode`) and
  `hasGposMark`.

`RuntimeContext` was introduced temporarily and then removed; runtime hooks now use explicit
parameters only.

### Script Handling Decision

`language.Script` is now threaded explicitly to only the hook paths that actually need it:

- `collectFeatures(plan, script)`
- `setupMasks(buffer, font, script)`

The previous `script()` accessor on temporary interfaces was removed. This matches the project’s
preference for explicit dataflow over dynamic context access.

### Status Against Plan

Completed:
1. Reduced `ShapingEngine` to minimal selector/factory contract (`Name`, `Match`, `New`).
2. Kept one shared executor (`shaperOpentype.shape` + `otMap`) unchanged.
3. Introduced optional hook interfaces (policy/plan/runtime/normalization) and rewired planner,
   shaping, and normalization call-sites through explicit dispatch helpers.
4. Replaced hard-wired script switch with registry-based shaper resolution (`categorizeComplex` now resolves from `SelectionContext`).
5. Added deterministic resolver behavior (score first, then name, then registration order) and resolver tests.
6. Added focused end-to-end parity fixtures for Latin/Hebrew/Arabic shaping invariants in `ot_shaper_parity_test.go`.
7. Added package split wiring boundary with new subpackages:
   - `otcore` (default/core shaper registration helpers)
   - `othebrew` (Hebrew shaper)
   - `otarabic` (Arabic shaper)
8. Moved Hebrew/Arabic `ShapingEngine` implementations out of base and switched base built-ins to core-only (`default`).
9. Removed duplicated legacy base Arabic engine implementation (`ot_arabic.go`) after migrating parity fixtures to registered split shapers.
10. Removed `otcomplex` compatibility facade; tests now register split shapers directly.
11. Removed migration-only `LegacyShapingEngine` type.
12. Reduced Hebrew shaper surface to minimal optional hooks (policy + compose + reorder).

Pending:
1. Continue contract cleanup by removing remaining migration-oriented defaults once Arabic hooks are
   narrowed similarly to Hebrew.
2. Add explicit Hebrew-isolation regression checks (Hebrew-only registry path and dependency guards).
3. Optional migration from global registry to constructor-injected registries in the future API redesign.

Current validation state: `go test .` passes after each refactor step.

### Package Split Status (Detailed)

Current state is an **engine split** with helper internals still in base.

What is already split:

- New subpackages exist and compile:
  - `harfbuzz/otcore` (`New()`, `Register()`)
  - `harfbuzz/othebrew` (`New()`, `Register()`)
  - `harfbuzz/otarabic` (`New()`, `Register()`)
- `othebrew` and `otarabic` own concrete Hebrew/Arabic `ShapingEngine` types and their runtime/plan logic.
- Base registry built-ins are now core-only (`default`); complex shapers are registered from split packages.
- Dependency direction is correct for modularity:
  - `otcore`/`othebrew`/`otarabic` import `harfbuzz`
  - `harfbuzz` does **not** import split shaper packages
- Registry duplicate handling now has an explicit sentinel (`ErrShaperAlreadyRegistered`), allowing idempotent `Register()` calls in split packages.

What remains in base package (not yet split):

- Default/core shaper implementation (`ot_shape_complex.go`).
- Generic synthetic GSUB execution remains in base (`ot_synthetic_gsub.go`) as shared runtime
  infrastructure used by split shapers.
- Generic Unicode category export remains in base (`unicode_export.go`: `UnicodeGeneralCategory`)
  for split shapers that need pre/post-context classification without exposing Unicode internals.
- Default startup registration still happens in base (`builtInShapers()` in `shaper_registry.go`), and now registers only core/default.

Main blockers to a full source move:

- `ShapingEngine` externalization blocker is mostly resolved:
  - hook contracts are exported (`ShapingEngine`, `FeaturePlanner`, `NormalizeContext`, `PlanContext`, `PauseContext`),
    and split packages now ship real Hebrew/Arabic engines against that surface.
  - remaining work here is API hardening/cleanup (temporary broad interface), not basic external implementability.
- Plan/runtime coupling blocker is mostly resolved:
  - complex plan/runtime behavior now lives in split packages (`othebrew`, `otarabic`).
  - remaining coupling is intentionally narrow shared runtime primitives from base:
    synthetic-GSUB executor and generic Unicode category lookup.
  - cleanup focus is API hardening/tests, not Arabic-specific bridge removal.

Practical interpretation:

- Package boundary and registration API are working.
- Concrete complex-engine ownership moved to `othebrew` / `otarabic`.
- Remaining split work is focused on contract hardening and migration-cleanup removal.

### Low-Level Exports Avoided (Arabic)

The Arabic move into `otarabic` is now done without exporting low-level OT apply internals.
The following internals are still intentionally not exported:

- GSUB fallback construction/runtime internals:
  `lookupGSUB` (`ot_layout_gsub.go`), `otLayoutLookupAccelerator` + `init`
  (`ot_layout_gsubgpos.go`), and `otApplyContext` plus
  `reset`/`setLookupMask`/`substituteLookup` (`ot_layout_gsubgpos.go`).
- Lookup-flag internals:
  `otIgnoreMarks` (`ot_layout.go`).
- Internal glyph-ID alias:
  `gID` (`harfbuzz.go`).
- Arabic shaping fallback data symbols:
  `firstArabicShape`, `lastArabicShape`, `arabicShaping`, and ligature table
  types/data (`arabicLig`, `arabicTableEntry`, `arabicLigatureTable`,
  `arabicLigatureMarkTable`, `arabicLigature3Table`) from
  generated Arabic data sources.
- Potential font-storage internals:
  direct `Font.face` access patterns in fallback code (`fonts.go`).
  Note: this can be avoided by using `Font.Face()` instead of exporting the
  field.

Kept shared exports are intentionally narrow and generic (`SyntheticGSUBProgram`,
`CompileSyntheticGSUBProgram`, `Font.NominalGlyph`, `UnicodeGeneralCategory`).

### Minimal Hook Contracts: Phase-Ordered Patch Plan

Goal: make Hebrew/Arabic shapers externally implementable in split packages while keeping the shared
OT pipeline in base and avoiding broad internals export.

#### Phase A: Export naming-only API boundary (no behavior change)

1. Export current hook method names and enum/type names used by engines:
   `ZeroWidthMarksMode`, `NormalizationMode`, `FeatureFlags`, exported `FeaturePlanner` and
   `NormalizeContext` method names.
2. Keep behavior and call graph unchanged; this is a pure API-surface rename/alias step.
3. Add compile-time assertions and targeted tests to ensure old built-ins still satisfy the
   updated interfaces.

Acceptance:
- `go test .` remains green.
- No shaping diffs in existing parity tests.

Status:
- Phase A is complete.

#### Phase B: Replace `dataCreate(*otShapePlan)` with narrow plan view

1. Introduce exported `PlanContext` to expose only what Hebrew/Arabic need at plan-init time:
   script/direction, feature mask lookup (`FeatureMask1`), and feature fallback status
   (`FeatureNeedsFallback`).
2. Replace engine hook `dataCreate(plan *otShapePlan)` with `InitPlan(ctx PlanContext)`.
3. Move Arabic per-plan state initialization (`newArabicPlan` equivalent) behind this narrow
   context.

Acceptance:
- No engine type outside base references `otShapePlan`.
- Existing Arabic/Hebrew parity tests stay green.

Status:
- Phase B is complete.
- `ShapingEngine` now uses `InitPlan(plan PlanContext)` instead of
  `DataCreate(plan *otShapePlan)`.
- Base `otShapePlan` now implements `PlanContext` (`Script`, `Direction`,
  `FeatureMask1`, `FeatureNeedsFallback`) and Arabic per-plan setup consumes only
  that narrow view.

#### Phase C: Export minimal runtime buffer/glyph operations for complex shapers

1. Add a minimal exported method set needed by Hebrew/Arabic runtime hooks:
   - buffer cluster/flag operations (`MergeClusters`, `UnsafeToBreak`, `UnsafeToConcat`,
     `UnsafeToConcatFromOutbuffer`, `SafeToInsertTatweel`, `PreContext`, `PostContext`)
   - glyph accessors/mutators (`Codepoint`, `SetCodepoint`, `ComplexAux`, `SetComplexAux`,
     `ModifiedCombiningClass`, `SetModifiedCombiningClass`, `GeneralCategory`,
     `IsDefaultIgnorable`, `Multiplied`, `LigComp`).
2. Keep storage layout unchanged; add wrappers only.

Acceptance:
- Arabic/Hebrew implementations can compile using only exported members.
- No measurable behavior drift in parity fixtures.

Status:
- Phase C is complete.
- Exported runtime buffer operations are now available:
  `MergeClusters`, `UnsafeToBreak`, `UnsafeToConcat`,
  `UnsafeToConcatFromOutbuffer`, `SafeToInsertTatweel`, `PreContext`,
  `PostContext`.
- Exported glyph accessors/mutators are now available:
  `Codepoint`, `SetCodepoint`, `ComplexAux`, `SetComplexAux`,
  `ModifiedCombiningClass`, `SetModifiedCombiningClass`,
  `GeneralCategory`, `IsDefaultIgnorable`, `Multiplied`, `LigComp`.
- External compile-surface checks were added in split-package tests.

#### Phase D: Extract GSUB pause context and Arabic fallback integration

1. Replace internal `pauseFunc(plan *otShapePlan, font *Font, buffer *Buffer)` dependency with
   exported `GSUBPauseFunc(PauseContext) bool`.
2. Provide a narrow pause context exposing only `Font()` and `Buffer()`.
3. For Arabic fallback, expose a narrow base helper API for fallback lookup application, rather
   than exporting low-level layout accelerator internals.

Acceptance:
- Arabic pause hooks (`stch` recorder and fallback shape pause) no longer require `otShapePlan`
  type assertions.

Status:
- Phase D is complete.
- Pause hooks now use exported pause contracts:
  `GSUBPauseFunc(PauseContext) bool` with `PauseContext` exposing only
  `Font()` and `Buffer()`.
- Planner/map pause plumbing was rewired to the new type (`FeaturePlanner.AddGSUBPause`,
  `otShapePlanner.AddGSUBPause`, `otMapBuilder`/`stageMap` pause storage and execution).
- Arabic pause hooks were decoupled from `*otShapePlan`:
  `recordStch` and fallback shape pause now run as bound engine methods and read
  per-plan state from Arabic engine-owned plan state.
- Arabic fallback lookup mask selection is now initialized during `InitPlan` and passed through
  narrow data (`fallbackMaskArray`) into fallback-plan creation; no pause-time plan assertions remain.
- Validation: `go test .` and `go test ./...` pass.

#### Phase E: Move implementations out of base

1. Move Hebrew and Arabic engine implementations into split packages.
2. Keep selection/registration behavior identical (`Name`, `Match`, `New` unchanged).
3. Base package continues to host pipeline execution and default/core engine only.

Acceptance:
- Active complex-engine path (`othebrew`/`otarabic`) owns Hebrew/Arabic behavior without delegating to base plan-state helpers.
- `go test ./...` passes.

Status:
- Phase E is complete.
- Completed:
  - Hebrew/Arabic `ShapingEngine` implementations now live outside base in `harfbuzz/othebrew` and `harfbuzz/otarabic`.
  - Hebrew shaping behavior is implemented directly inside `othebrew` (compose/reorder/tag), rather than delegated to base helpers.
  - Arabic shaping behavior is implemented directly inside `otarabic` (feature collection, plan init, joining/setup masks, mark reordering, stretch postprocess, fallback pause orchestration), rather than delegating to `harfbuzz.ArabicPlanState`.
  - Base built-ins are now core-only (`default`); complex engines are provided/registered via split packages.
  - Legacy duplicated base Arabic engine file (`ot_arabic.go`) was removed.
  - Parity fixtures rely on registered split shapers instead of base Arabic plan-state types.
  - Validation: `go test .` and `go test ./...` pass.

#### Phase F: Cleanup and hardening

1. Remove temporary adapters/aliases introduced during A-D.
2. Add explicit API docs for exported shaper contracts.
3. Add split-focused regression tests (selection, Hebrew compose fallback, Arabic joining/stch and
   fallback pause invariants).

Acceptance:
- Public contract is minimal, documented, and sufficient for external complex shapers.
- No functional regressions in existing and new parity suites.

### Registry/Factory Wiring (Implemented)

Goal: keep the OpenType pipeline in the base package while allowing shaper implementations to live
in sub-packages that depend on base. The base package must not import those sub-packages.

Implemented contracts (base package):

- `ShapingEngine` now includes selection hooks:
  `Name() string`, `Match(SelectionContext) int`, `New() ShapingEngine`
- `SelectionContext`:
  carries only data needed for selection (`Script`, `Direction`, chosen/found GSUB/GPOS script tags).
- Registry API:
  `RegisterShaper(ShapingEngine) error`, `ClearRegistry()`, internal `resolveShaperForContext(ctx SelectionContext)`.

Selection model:

1. Build `SelectionContext` in planner creation.
2. Resolve a shaper through registry.
3. Fall back to built-in default shaper if no engine matches (or if `New()` returns `nil`).

Design intent:

- `Match(ctx)` supports script-only and conditional logic (for example, Arabic-specific direction or script-tag behavior).
- Deterministic tie-break is stable: score, then name, then registration order.
- `New()` returns per-plan instances to avoid shared mutable shaper state in runtime hooks.

Wiring options:

- Current: global default registry initialized with built-ins (`default` only).
- Split helpers now available as packages:
  - `harfbuzz/otcore`: `New()` + `Register()` for default/core engine
  - `harfbuzz/othebrew`: `New()` + `Register()` for Hebrew engine
  - `harfbuzz/otarabic`: `New()` + `Register()` for Arabic engine
- Future option: constructor injection of registries for non-global wiring.
- In a later stage, we will de-compose `ShapingEngine` into narrower interfaces, allowing more modularity and shared functionality between shaper packages. 

Registration semantics:

- each sub-package can export a type implementing `ShapingEngine`.
- clients can register extra engines through `RegisterShaper(...)`.
- if no registered engine matches a context, the default shaper is used.
- `ClearRegistry()` clears registered engines in the default registry (used primarily for controlled setup in tests).

Determinism/safety rules:

- registration is startup-time only; registry treated read-only during shaping.
- conflict resolution is deterministic (score, then name, then registration order).
- tests should use fresh registries rather than shared global state.

### Hebrew Isolation Plan (Next)

Goal: complete Hebrew/Arabic disentanglement so Hebrew shaping is isolated behind a minimal contract.

1. Freeze split baseline:
   keep `othebrew` and `otarabic` as engine owners with direct registration.
2. Define isolation rule:
   Hebrew must not depend on Arabic package internals or Arabic-specific data files.
3. Decompose base shaper contracts (Hebrew-first):
   split `ShapingEngine` into narrower optional interfaces (selection/policy, feature collection,
   plan init, normalization hooks, runtime hooks), with temporary dispatch helpers.
4. Minimize Hebrew implementation surface:
   refactor `othebrew` to implement only required interfaces (selection, policy, compose/reorder),
   removing broad no-op scaffolding.
5. Keep Arabic on full hook surface temporarily:
   keep `otarabic` on the broad hook set until Hebrew isolation is complete and stable.
6. Add isolation regression checks:
   add tests for registry selection with Hebrew-only and Hebrew+Arabic registration, and explicit
   checks that Hebrew has no Arabic-package usage.
7. Migrate docs/tests to split-native terminology:
   remove remaining transitional terminology in docs and helper tests.
8. Tighten split contracts (later phase):
   remove remaining migration-only interfaces/helpers after Hebrew and Arabic
   packages are fully minimal.

Acceptance targets:

- Hebrew shaping path compiles and runs without Arabic-package dependencies.
- Base pipeline remains shared and behaviorally stable for existing parity fixtures.
- split registration and behavior stay stable without compatibility wrappers.
- split registration and behavior stay stable with split-native terminology.

Status checkpoint (2026-02-08):

- Step 3 started and first incremental pass is in place:
  `ShapingEngine` now exposes minimal selection hooks, optional hook interfaces
  are defined for policy/plan/runtime behavior, and base OT pipeline call-sites
  dispatch through explicit optional-hook helpers with sane defaults.
- Hebrew step-4 prep started:
  Hebrew shaper now relies on minimal optional hooks (policy + compose +
  reorder) instead of broad no-op hook scaffolding.
- Compatibility facade removed:
  `otcomplex` package was deleted and tests now register `othebrew` and
  `otarabic` directly.

### Arabic Interdependency Investigation (2026-02-08)

Purpose: identify where Arabic shaping is coupled to the shared OT pipeline, and
separate hard architectural coupling from soft/replaceable coupling.

Pipeline-phase dependency map:

1. Selection-time coupling:
   `otarabic.Shaper.Match` depends on `SelectionContext` script/direction and
   GSUB chosen-script fallback (`DFLT` vs non-`DFLT`) for Syriac routing. **Thoughts**: This should not be a reason for coupling. The base package will ask each shaper to `Match` and receive a priority/confidence. The Arabic Shaper should give a very hight confidence, in contrast to Hebrew- or Lating-shaper. `SelectionContext` should aim to include all necessary information available until the point of feature-resolution, i.e. processing of tables *ScriptList*, *Script* and *LangSys* should already have been handled.
2. Plan-time feature/stage coupling:
   Arabic injects a custom GSUB schedule (`stch`, joining features, `rlig`,
   `calt`, `rclt`, etc.) and inserts GSUB pauses (`recordStchPause`,
   `arabicFallbackShapePause`) that become explicit stage boundaries in
   `ot_map` execution. **Thoughts**: I do not understand this completely, but it sounds like the Shaper should get a hook directly after feature selection to alter the features list. For this the shaper needs the currently selected features and the list of features available (possibly narrowed down per GSUB and GPOS respectively).
3. Pre-lookup preparation coupling:
   Arabic mandatory joining (`applyArabicJoining`) computes per-glyph shaping
   state and safety flags, then `SetupMasks` maps that state into lookup masks.
   This runs immediately before GSUB application. **Thoughts**: I am not sure I understand this. Does that mean the (map-)plan is done and we are at the point of applying GSUB-driven lookups for the next glyph? Do we need a hook right before feature application to execute kind of a pseude-lookup (`applyArabicJoining`) as the first lookup of GSUB?
4. Lookup-application coupling:
   The core lookup loop is generic, but Arabic controls behavior through
   feature-derived lookup flags and masks (`autoZWJ`, `autoZWNJ`,
   `perSyllable`, lookup mask bits) and through pause-time mutations. **Thoughts**: The pause-timing seems to be the critical one here. We should discuss Arabic pauses in detail, as I see not reference to it in the OpenType spec and therefore do not have knowledge about this.
5. Coverage/matching coupling:
   Coverage matching itself is generic and script-agnostic. Arabic influence
   enters indirectly via iterator skip policy (joiner handling and mask gating),
   not via Arabic-specific coverage code paths. **Thoughts**: I see no problem in terms of coupling here.
6. Pause/fallback coupling:
   Arabic fallback shaping is executed at a GSUB pause and constructs/applies a
   synthesized fallback plan. This is where coupling is strongest because
   fallback execution relies on base internal lookup-application machinery. **Thoughts**: Let's therefore think about this as soon as all the previous problems have been addressed. Same for the next one (7.). However, in the implementation plan below ("Decision implications") there is a proposal to proceed exactly the other way around: decompose fallback shaping first. Let's discuss this.
7. Post-position/postprocess coupling:
   Arabic `stch` postprocessing depends on GSUB-produced glyph metadata and
   mutates glyph stream/positions after positioning. Final glyph-flag
   propagation also contains Arabic-specific tatweel safety interactions.

Background: HarfBuzz Arabic feature injection model

- Arabic shaping uses two layers, not one:
  1. LangSys/script resolution determines what the font exposes for the current
     run (`ScriptList`/`Script`/`LangSys` path).
  2. The Arabic shaper still requests a fixed shaping program (for example
     `stch`, `ccmp`, `locl`, joining-form features, `rlig`, `calt`, `rclt`)
     because shaping needs stable phase ordering even when some features are
     absent.
- During map compilation, requested features are intersected with available
  lookups. Missing features are dropped unless marked with fallback capability
  (`FeatureHasFallback`), in which case mask/fallback metadata is still kept.
- Arabic fallback is then applied at pause checkpoints to compensate for missing
  OpenType feature coverage. This behavior is an engine-level execution policy;
  it is not specified by the OpenType spec itself.

Hard vs soft coupling:

- Hard coupling:
  - Arabic fallback plan implementation depends on base internal OT apply
    structures and methods (synthetic lookup accelerators + internal GSUB apply
    context).
  - Pause scheduling/execution contract is deeply integrated with `ot_map` stage
    execution order.
  - `stch` postprocess assumes specific glyph metadata lifecycle across GSUB and
    post-position stages.
- Soft coupling:
  - Selection inputs (`SelectionContext`) are already narrow and stable.
  - Feature-planner and plan-context surfaces are narrow enough to keep evolving
    without re-exposing full planner internals.
  - Joiner/coverage behavior is mostly data-driven through lookup flags, not
    Arabic-specialized matcher code.

Decision implications for next refactor cuts:

1. If Arabic is to become independently evolvable, isolate fallback application
   first (replace internal fallback execution dependency with a narrow exported
   executor boundary).
2. Preserve current stage ordering semantics while refactoring; pauses are a
   behavioral contract, not only an implementation detail.
3. Treat pre-GSUB joining/mask preparation as Arabic-owned logic and keep it
   outside base once a stable runtime hook contract exists.
4. Keep coverage/matcher core shared; do not fork it for Arabic unless required
   by a measured regression.

### Arabic Fallback Condensation Plan (2026-02-08)

Goal: treat fallback shaping as a bounded Arabic-owned subsystem rather than
pipeline-internal helper flow.

1. Define a dedicated fallback object contract in `otarabic`.
   - Introduce a narrow `ArabicFallbackEngine` abstraction with one orchestration
     entrypoint (`Apply(font, buffer) bool`).
   - Keep the API focused on pause-time behavior and avoid exposing base apply
     internals.
2. Move fallback orchestration ownership into `otarabic`.
   - Store fallback enablement state, feature-mask snapshot, and lazy fallback-program
     cache inside the Arabic fallback object.
   - Keep `arabicPlanState` delegating pause-time fallback execution to that object.
3. Introduce a generic synthetic-GSUB executor boundary in base.
   - Add an opaque executable program API that compiles/apply synthetic GSUB
     lookups without exposing internal apply context types.
   - Keep the execution API focused on lookup mask + lookup flags + subtables.
4. Migrate Arabic fallback from wrapper-plan internals to the synthetic-GSUB
   executor boundary.
   - Build Arabic fallback substitutions into synthetic lookup specs.
   - Compile once lazily and apply through the generic program API.

Status (2026-02-08):

- Step 1 implemented:
  `otarabic` now defines `ArabicFallbackEngine` as an explicit fallback object
  contract.
- Step 2 implemented:
  fallback orchestration state (enablement decision, feature masks, cached
  fallback program) moved into `otarabic` fallback engine object.
- Step 3 implemented:
  base now exposes an opaque synthetic-GSUB executor API
  (`CompileSyntheticGSUBProgram`, `SyntheticGSUBProgram.Apply`), so split
  shapers do not need direct access to internal lookup-apply structs.
- Step 4 implemented:
  Arabic fallback execution is now compiled/applied through the synthetic-GSUB
  executor path instead of direct internal fallback-plan execution internals.
- Follow-up implemented:
  Arabic-specific fallback lookup builders and fallback table data ownership were
  moved from base (`ot_arabic_fallback.go` and fallback section of
  `ot_arabic_table.go`) into `harfbuzz/otarabic`.
- Additional follow-up implemented:
  Arabic joining classification data/logic (`ot_arabic_support.go`,
  generated joining map from `ot_arabic_table.go`, and Arabic-specific bridge
  exports) were moved/removed from base and are now local to `harfbuzz/otarabic`.
  Base now retains only generic shaping primitives used by split packages.
- Regression guard added:
  `harfbuzz/otarabic/runtime_surface_test.go` now includes
  `TestNoBaseArabicBridgeSelectors`, which fails if `otarabic` source files
  reference forbidden base Arabic selectors (`ArabicJoiningType`, `ArabicIsWord`).
