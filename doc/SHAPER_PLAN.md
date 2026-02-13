# otshape/ Considerations

Our `otshape/` shaper should be much simpler than Harfbuzz for a variety of reasons:

- We do OpenType (GSUB/GPOS) shaping exclusively; other font formats or font tables are disregarded (e.g., not `kern` table).
- We do not strive to cache shaping plans (at least for the near future), but rather focus on constructing plans on the fly each time the shaper is called (or some important property in the rune input stream changes).
- We will be less lenient for broken or incomplete fonts than Harfbuzz is. I expect our shaper to more often flag an error than Harfbuzz in cases where Harfbuzz would try to apply some fallback behaviour.
- We will outsource complex script shaping (Hebrew, Arabic, and others) into their own shaper packages (already done for the Harfbuzz copy included in this package `opentype/`. The core shaper pipeline should be more or less agnostic of the script/language and its special requirements for shaping.

# Harfbuzz Shaper Plan
(from `harfbuzz/ot_shaper.go` and `harfbuzz/ot_map.go`)

## 1. High-Level Idea

The shaper plan is a compiled execution program for one shaping context:

1. font face (+ variation coordinates)
2. segment properties (script, language, direction)
3. user feature set

Instead of deciding everything during glyph traversal, the plan precomputes:

1. which script/langsys entries are used in GSUB and GPOS
2. which features are active and how they map to bit masks
3. which lookups are run, in which stage, with which runtime flags
4. where stage pause hooks run
5. fallback switches (no GDEF classes, no GPOS mark handling, etc.)

At runtime, shaping mostly executes this precompiled plan against a buffer.

## 2. Where Plan Build Starts

Plan creation starts in `(*Buffer).Shape(...)`:

1. `newShapePlanCached(...)` key is built from props + user features + face
2. existing plan is reused if equal, otherwise a new one is compiled
3. compiled plan is executed with `shapePlan.execute(...)`

Important: caching normalizes non-global feature ranges to canonical sentinels in the cache key path, so equality is based on semantics rather than exact range offsets.

## 3. Plan Object Layers

There are three relevant layers:

1. `shapePlan` (outer cacheable object)
2. `shaperOpentype` (OpenType shaper instance + variation key)
3. `otShapePlan` (compiled OpenType-specific execution data)

`otShapePlan` stores:

1. compiled `otMap` (features, masks, lookups, stages)
2. fast masks (`frac`, `numr`, `dnom`, `rtlm`)
3. booleans driving runtime behavior (`applyGpos`, `hasVert`, `zeroMarks`, fallback toggles)

## 4. Planner Build Sequence

The inner plan build path is:

1. `shaperOpentype.compile(...)`
2. `otShapePlan.init0(...)`
3. `newOtShapePlanner(...)`
4. `CollectFeatures(userFeatures)`
5. `planner.compile(...)` (compiles `otMap`)
6. `shaperInitPlan(...)` hook

`newOtShapePlanner(...)` also selects script engine and mark behavior policy early.

## 5. Feature Collection Model

`CollectFeatures(...)` adds features into the map builder in a deliberate order:

1. required defaults (like `rvrn`)
2. direction-driven features (`ltra/ltrm` or `rtla/rtlm`)
3. automatic fraction features (`frac/numr/dnom`)
4. common defaults (`abvm/blwm/ccmp/locl/mark/mkmk/rlig`)
5. horizontal defaults (`calt/clig/curs/dist/kern/liga/rclt`) or vertical `vert`
6. user features (global or ranged)
7. shaper hooks (`CollectFeatures`, `OverrideFeatures`)

Feature entries carry flags such as:

1. global vs per-glyph mask semantics
2. manual ZWJ/ZWNJ handling
3. global search if langsys lookup fails
4. random alternates
5. per-syllable containment
6. fallback availability

## 6. Script and Language Resolution

`otMapBuilder` resolves GSUB and GPOS independently:

1. derive candidate OpenType tags from segment script/language
2. select script with fallback chain (`requested -> DFLT -> dflt -> latn`)
3. select language under selected script, with default fallback
4. store script/lang indices and whether requested script was actually found

This is done up front so unavailable features can be skipped before bit allocation.

## 7. Feature Resolution and Bit Allocation

In `otMapBuilder.compile(...)`:

1. feature requests are sorted by tag
2. duplicates are merged (global/local semantics and max value merged)
3. required feature indices are retrieved from selected langsys
4. each feature is resolved in GSUB/GPOS for selected langsys
5. optional global-search fallback is attempted for flagged features
6. if unresolved and no fallback flag, feature is dropped
7. mask bits are allocated

Mask strategy:

1. one global bit is reserved
2. small global binary features can reuse global bit
3. other features get a dedicated bit range (bounded by `otMapMaxBits`)
4. `globalMask` is prefilled with default values for non-global features

Output per feature is `featureMap`:

1. table indices (GSUB/GPOS)
2. stage numbers
3. mask/shift/mask1
4. lookup behavior flags
5. unresolved-needs-fallback marker

## 8. Stage Construction and Pause Anchors

Stages come from two sources:

1. explicit stage increments via `AddGSUBPause` / `addGPOSPause`
2. anchored pauses via `AddGSUBPauseBefore(tag)` / `AddGSUBPauseAfter(tag)`

Anchored pauses are resolved after feature selection:

1. tag -> stage mapping is built from resolved features
2. before-tag anchors map to `stage-1`, after-tag anchors map to `stage`
3. resulting stage list is sorted by stage index

The builder always appends terminal pauses for GSUB/GPOS to close stage boundaries.

## 9. Lookup Scheduling

For each table (GSUB, GPOS):

1. process stages in order
2. inject required-feature lookups in the required stage
3. add lookups of all features assigned to that stage
4. sort stage-local lookups by lookup index
5. merge duplicate lookup indices by OR-ing masks and combining lookup flags
6. seal stage by writing `stageMap{lastLookup, pauseFunc}`

This produces:

1. a flat ordered lookup array per table
2. a stage boundary array that slices into that lookup array

## 10. Runtime Plan Application

The runtime path in `shaperOpentype.shape(...)` is:

1. initialize masks from plan `globalMask`
2. set Unicode props, dotted-circle insertion, cluster formation, native direction
3. shaper preprocess hook
4. substitution phase (`substituteBeforePosition`)
5. positioning phase (`position`)
6. postprocess phase (`substituteAfterPosition`)
7. cluster-level flag propagation
8. restore original direction

### 10.1 Substitution-side plan use

Before GSUB:

1. rotate/mirror characters depending on direction and `vert` availability
2. normalize and map codepoints to glyphs
3. set masks (fraction logic + shaper mask hooks + ranged user feature masks)
4. initialize layout substitution state (`layoutSubstituteStart`)
5. synthesize glyph classes if GDEF classes are absent

Then execute `plan.substitute(...)`, which delegates to `otMap.apply(...)` on GSUB.

### 10.2 Positioning-side plan use

Positioning runs as:

1. default advances/origins are initialized
2. `plan.position(...)` applies GPOS only when `applyGpos=true`
3. mark-width zeroing policy is driven by plan + shaper mark behavior
4. fallback mark positioning is used when needed
5. reverse output for backward direction at the end

## 11. `otMap.apply(...)` Execution Mechanics

`otMap.apply(...)` is the runtime executor for both GSUB and GPOS:

1. reset reusable apply context for selected table
2. iterate stage maps in order
3. run lookups `[prevStage.lastLookup : stage.lastLookup)` sequentially
4. for each lookup, load precompiled mask + lookup flags into context
5. apply lookup through accelerator (`applyString(...)`)
6. execute stage pause callback if present

So the execution loop is:

1. stage boundary
2. ordered lookup batch
3. optional pause hook
4. next stage

## 12. Data and Hooking Principles to Reuse in `otshape`

For our own `otshape` plan design, the strongest reusable ideas are:

1. treat plan as compiled, cacheable execution state
2. split planning into feature collection then resolution/compilation
3. encode feature activation as glyph masks
4. compile stage boundaries explicitly, not implicitly
5. keep lookup scheduling table-specific but structurally identical
6. support pause hooks as first-class stage controls
7. keep runtime loop simple and data-driven by plan

This gives deterministic behavior and makes plan-vs-runtime responsibilities clean.

# Proposed `otshape` Plan Structure

For `otshape`, a plan should be an immutable execution program built per run (or per segment in a stream), not a mutable runtime object.

```go
type Plan struct {
    Props      SegmentProps
    ScriptTag  ot.Tag
    LangTag    ot.Tag
    VarIndex   [2]int // GSUB, GPOS variation selection (-1 if none)

    Masks      MaskLayout
    GSUB       TableProgram
    GPOS       TableProgram

    Policy     PlanPolicy
    Hooks      PlanHooks
    Notes      []PlanNote // optional diagnostics/warnings
}

type SegmentProps struct {
    Direction bidi.Direction
    Script    language.Script
    Language  language.Tag
}

type MaskLayout struct {
    GlobalMask uint32
    ByFeature  map[ot.Tag]MaskSpec
}

type MaskSpec struct {
    Mask         uint32
    Shift        uint8
    DefaultValue uint32
}

type TableProgram struct {
    FoundScript  bool
    Stages       []Stage
    Lookups      []LookupOp // flat array; stages slice this range
    FeatureBinds []FeatureBind
}

type Stage struct {
    FirstLookup int // inclusive
    LastLookup  int // exclusive
    Pause       PauseHookID // optional
}

type LookupOp struct {
    LookupIndex uint16
    FeatureTag  ot.Tag
    Mask        uint32
    Flags       LookupRunFlags
}

type FeatureBind struct {
    Tag          ot.Tag
    FeatureIndex uint16
    Stage        int
    Mask         uint32
    Required     bool
}
```

### Why this shape

1. Separates plan-time resolution data from runtime mutable buffer state.
2. Keeps stage scheduling explicit and inspectable.
3. Encodes feature activation in compact masks usable at runtime.
4. Keeps GSUB/GPOS symmetric while still independent.

## 1. What “Compile a Plan” Means

Compile is the transformation from declarative shaping inputs to a ready-to-run lookup program.

### 1.1 Compile input

1. `PlanRequest{Font, Props, UserFeatures, ScriptEngine, StrictMode}`
2. OpenType layout tables (`GSUB`, `GPOS`, `GDEF`)
3. Variation coordinates (optional)

### 1.2 Compile transformations

1. Resolve OpenType script/language tags and selected LangSys for GSUB and GPOS.
2. Collect feature intents:
   - engine defaults
   - script-specific additions/overrides
   - user feature toggles/ranges
3. Normalize and deduplicate feature intents by tag.
4. Resolve each feature to GSUB/GPOS feature indices for the selected LangSys.
5. Validate in strict mode (error out where policy requires).
6. Allocate mask bits and compute global default mask.
7. Expand features to lookup ops (including feature-variation substitution where applicable).
8. Sort lookups by lookup index per stage; merge duplicates by OR-ing masks and combining flags.
9. Build stage boundaries and attach pause hooks.
10. Freeze into immutable plan object.

### 1.3 Compile output

1. `*Plan` (executable)
2. `error` for hard failures
3. optional diagnostics/notes for non-fatal conditions

## 2. What belongs in a Plan (and what does not)

### 2.1 In plan

1. Resolved tags, feature indices, lookup indices, stage boundaries.
2. Feature mask layout (`GlobalMask`, per-feature masks and shifts).
3. Runtime policy switches (`applyGPOS`, mark-zeroing mode, strict policy decisions).
4. Hook binding points used by the script module.

### 2.2 Not in plan

1. Mutable run buffer content (`Glyphs`, `Pos`, clusters, per-glyph masks).
2. Runtime cursors/iterators/apply context.
3. External segmenter state (paragraph bidi, line-break context, etc.).

## 3. Plan API for the Pipeline

Suggested interfaces:

```go
type PlanCompiler interface {
    Compile(req PlanRequest) (*Plan, error)
}

type PlanExecutor interface {
    Apply(plan *Plan, run *RunBuffer) error
    ApplyGSUB(plan *Plan, run *RunBuffer) error
    ApplyGPOS(plan *Plan, run *RunBuffer) error
}
```

Pipeline integration flow:

1. Segment stream by stable shaping properties (script/lang/dir/features/font).
2. Compile plan for that segment.
3. Map runes to initial glyph sequence.
4. Execute plan on `RunBuffer`.
5. Emit shaped AoS records to sink.
6. Recompile when segment-defining properties change.

## 4. Consequences of `otshape` Constraints

Given current `otshape` constraints (OT-only, no plan cache for now, stricter error policy, complex scripts outsourced):

1. Treat plan as compiled execution state (ephemeral now; cacheable later).
2. Keep compiler deterministic and transparent.
3. Prefer explicit compile errors over implicit fallback behavior.
4. Keep core plan generic and script-agnostic; move script complexity behind narrow hook interfaces.
5. Make caching a wrapper around `Compile`, not a core assumption inside plan internals.

## 5. Suggested Delivery Order

1. PR 1: compile lookup programs + executor loop (global features only).
2. PR 2: edit-span plumbing + ranged feature masks + side-array alignment.
3. PR 3: advanced flags (autoZWJ, autoZWNJ, perSyllable, random) and stricter script-module integration.

## 6. Next GPOS-Focused PRs

1. PR 4.1: lock GPOS behavior with end-to-end tests in shaper packages (`otshape/otcore`), covering single-adjust, pair-adjust, mark attachment, and cursive attachment with assertions on `GlyphRecord.Pos`.
2. PR 4.2: wire plan-time shaper hooks into compile (`CollectFeatures`, `OverrideFeatures`, `PostResolveFeatures`, `InitPlan`) so script shapers can influence GPOS feature/stage selection.
3. PR 4.3: implement runtime GPOS policy execution (`ZeroMarks`, `FallbackMarkPos`) in the plan executor.
4. PR 4.4: harden GPOS mask/range semantics (ranged feature toggles, feature args, overlap behavior) with focused tests.
5. PR 4.5: finalize streaming/run-boundary semantics for positioning and test flush behavior (`FlushOnRunBoundary`, cluster boundaries).

## 7. Strict Arabic Fallback Policy (Implemented)

The Arabic shaper now uses a strict, deficiency-driven `.notdef` fallback model:

1. Fallback is requested only when compile-time feature resolution marks Arabic shaping features as unresolved (`FeatureNeedsFallback`), not merely when a feature carries `FeatureHasFallback`.
2. Structural defects are fail-fast: if fallback is requested but the font has no usable cmap, plan validation fails during compile.
3. Runtime fallback repair is narrow: only unresolved glyphs (`gid == .notdef`) are candidates for replacement.
4. Quality misses are non-fatal: if no presentation-form mapping exists for a `.notdef`, the glyph stays `.notdef` and shaping continues.
5. Fallback activation considers both `rlig` and Arabic form features (`isol`, `fina`, `fin2`, `fin3`, `medi`, `med2`, `init`).
