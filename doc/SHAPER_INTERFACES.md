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
func (s *Shaper) ShapeInto(params Params, src RuneSource, dst []GlyphRecord) (n int, err error)
func (s *Shaper) ShapeEvents(params Params, src InputEventSource, sink GlyphSink, bufOpts BufferOptions) error
func NewStreamingShaper(capacity int, engines ...ShapingEngine) (*StreamingShaper, error)
func (s *StreamingShaper) Shape(ctx context.Context, params Params, src RuneSource, opts BufferOptions) error
```

These are method-based calls, not package-level `Shape(req)` functions.
`ShapeInto` fills caller-owned output storage using default buffering; see
[the slice API contract](STREAMING_API.md#caller-owned-output-slice) for bounds,
overflow, and partial-result behavior.
`StreamingShaper` wraps an ordinary shaper with a buffered output channel for
sequential runs and a concurrent consumer. See
[STREAMING_API.md](STREAMING_API.md#buffered-channel-api) for its completion,
cancellation, and cluster-offset contract, as well as sources, sinks, and events.

## Output order and clusters

The agreed public output contract (2026-10-26) is **logical shaping order**,
including RTL runs. Normalization and script-specific reordering may change
order within a source cluster; visual ordering belongs to the caller.
`GlyphRecord.Cluster` uses HarfBuzz level-0 grouping/merging semantics, with
zero-based original Unicode scalar indexes as anchors. This is the required
contract, not a claim that current combining-mark grouping is complete:
[SHAPE-011](SHAPING_FINDINGS.md#shape-011--combining-mark-clusters-differ-from-harfbuzz-level-0)
records the implementation gap. Feature ranges address original source scalars,
not normalized buffer positions (SHAPE-002). A cluster boundary alone does not
establish a safe streaming cut.

### Converting a single run to visual order

`otshape.VisualOrder(glyphs, direction, inPlace)` returns `([]GlyphRecord, error)`.
Pass the resolved horizontal direction used for shaping. LTR preserves order;
RTL reverses the entire record sequence, including records within a cluster.
Positions, clusters, and metadata travel with their glyphs unchanged. Attachment
diagnostic indexes are not remapped to visual indexes.

With `inPlace=false`, the result is an independent copy, even for LTR. With
`inPlace=true`, it reuses and modifies the original slice. Nil and empty slices
are accepted. Mixed, neutral, and invalid directions return an error without
modifying the input.

```go
visual, err := otshape.VisualOrder(logical, params.Direction, false)
if err != nil {
    return err
}
// Accumulate advances and apply offsets from visual when rendering.
```

Supply a complete directional run or a line fragment selected by layout, in
logical shaping order. Never reverse arbitrary streaming chunks separately.
The helper does not resolve bidi levels, arrange multiple runs on a line, or
choose line breaks. It is not idempotent for RTL: applying it twice restores
the original order. The shaper's sink continues to emit logical order.

### Compact rendering output

`GlyphRecord.PositionedGlyph()` returns an `otshape.PositionedGlyph` value
containing `GID`, `Cluster`, `XAdvance`, `YAdvance`, `XOffset`, and `YOffset`.
The cluster is copied from `GlyphRecord.Cluster`, preserving its original
Unicode scalar index. Position values remain signed font units, with shaping
adjustments already included. The font and size remain caller-owned context.

For rendering, first arrange a directional run with `VisualOrder`, then convert:

```go
visual, err := otshape.VisualOrder(logical, params.Direction, false)
if err != nil {
    return err
}
positioned := make([]otshape.PositionedGlyph, len(visual))
for i, record := range visual {
    positioned[i] = record.PositionedGlyph()
}
```

Conversion copies values without modifying the record, reordering glyphs,
scaling, or rounding. Use `otquery.FontScale` for size conversion. Feature masks,
break-safety flags, and attachment diagnostics are omitted; retain the detailed
records if those are needed. All shaping APIs continue to emit `GlyphRecord`.

## HarfBuzz compatibility and intentional differences

The compatibility target is the complete shaping result within the supported
shared/Latin/Hebrew scope, subject to the explicit conventions below. Streaming
must preserve that result: resource limits permit an explicit error, not
approximate output, truncation, or a forced unsafe flush. This is not a claim of
complete HarfBuzz conformance across scripts, fonts, or custom engines.

The HarfBuzz observations below refer to the checked-in captures verified with
**HarfBuzz 15.0.0**. Older upstream AOTS expectations retain their separately
recorded revision. See [independent expected output](TESTING.md#independent-expected-output)
for provenance and comparison rules; these observations are not promises about
every HarfBuzz version.

| Topic | Our contract and relationship to HarfBuzz | Rationale and evidence |
| --- | --- | --- |
| Output order | The sink emits logical shaping order, including RTL. The comparison harness adapts RTL HarfBuzz output to this convention; normalization and script reordering may still change order within a cluster. | Visual ordering belongs to the caller. See [output order and clusters](#output-order-and-clusters) and the [text comparisons](../internal/hbcmp/text_baselines_test.go). |
| Alternate values | Enablement is separate from the numeric argument. Enabled zero selects the first alternate; positives are one-based; all negatives request random selection; oversized positives leave the glyph unchanged by that substitution. The HarfBuzz adapter maps feature value zero to disabled, rather than to our enabled-zero request. Random choices are stable when a carry is reshaped within a request; matching HarfBuzz's random sequence is not promised. | Preserve the lower-level `ApplyFeature` contract and stable streaming results. See [SHAPE-003](SHAPING_FINDINGS.md#shape-003--feature-values-do-not-select-alternates), [public regressions](../otshape/otcore/alternate_values_test.go), and [carry repeatability tests](../otshape/alternate_values_test.go). Random and oversized values have local contract tests, not an extended upstream alternate oracle. |
| Shared alternate lookups | Equal active values execute a shared lookup once, with zero equivalent to one and all negatives equivalent to random. Incompatible values return `ErrConflictingAlternates` when a nonempty alternate set matches, including through context or extension lookups. This deliberately differs from the observed HarfBuzz packed-mask numeric behavior. | Avoid exposing mask encoding as alternate-selection semantics. See [SHAPE-020](SHAPING_FINDINGS.md#shape-020--default-features-run-in-feature-order-instead-of-lookup-order), [fixture evidence](../testdata/fonts/LOOKUP_ORDER.md), and [conflict tests](../otlayout/alternate_selection_error_test.go). |
| Coordinates | Clusters use HarfBuzz level-0 grouping/merging with zero-based original Unicode scalar anchors. Feature ranges also address original scalars, through normalization and substitution. This fixes our API coordinate convention; it is not an assertion that HarfBuzz clusters must use byte offsets. | Keep source correspondence independent of UTF-8 encoding and streaming windows. See [SHAPE-002](SHAPING_FINDINGS.md#shape-002--feature-ranges-use-chunk-local-coordinates) and the scalar-index captures described in [TESTING.md](TESTING.md). |
| Resource limits and failure | Raw capacity, initial-group and intermediate-buffer limits, conservative whole-run fallback, and partial-output error handling are our own supported-operation policies. They do not promise to reproduce HarfBuzz's limits or acceptance of unusually large inputs. | Bound work while emitting only final output. See the [buffer contract](STREAMING_API.md#agreed-buffer-contract-2026-11-02), [safe-cut policy](STREAMING_API.md#safe-cut-selection), and [limit regressions](../otshape/stream_limits_test.go). |

Intentional semantic differences, API conventions, and resource policies are
distinguished above so that a limit or representation choice does not become
permission to change supported glyphs, clusters, advances, or offsets. Keep
unresolved defects and deferred functionality in [SHAPING_FINDINGS.md](SHAPING_FINDINGS.md);
they are not accepted compatibility exceptions. A new intentional difference
requires an explicit decision and corresponding documentation/tests. Preserve
independent HarfBuzz expectations rather than replacing them with current output.

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
4. Scores at or below `ShaperConfidenceNone` are skipped. If no candidate
   matches, both APIs return `ErrNoMatchingShaper` before creating an engine,
   consuming input or emitting output
   ([SHAPE-004, resolved](SHAPING_FINDINGS.md#shape-004--zero-confidence-engines-are-selected)).
5. The winner's `New` initializes the selected engine. Event carries may create
   additional instances for their combined plans. If `New` returns nil, the
   candidate itself is reused.

Ordinary `Shape` reuses that engine and plan across streaming cycles.
`ShapeEvents` additionally calls `New` for each combined carry plan, then runs
`InitPlan`/validation before shaping that carry. Stateful engines should return
independent instances; nil retains the fallback to reuse. Event pushes no longer
compile separate plans or leave a later scope's plan state active for earlier
text. The root plan is still compiled up front to validate defaults.

## Streaming locality and growth limits

SHAPE-015 gives run/plan hooks, pauses, and per-syllable work conservative
whole-run buffering: EOF must fit within raw capacity, or shaping returns
`ErrBufferFull`. Engines with no such hooks can use the shared dependency trace.
Normalization hooks may opt in with:

```go
type ShapingEngineStreamingNormalization interface {
    StreamingNormalization() (local bool, dependsOnGposMark bool)
}
```

This promises stateless behavior depending only on the supplied pair/mark
sequence, font, selection, and declared plan-wide mark support. It does not
certify arbitrary run or plan hooks. The Hebrew engine reports both values true;
its event requests with no root mark support use whole-run fallback because a
future feature event could change earlier normalization. See
[STREAMING_API.md](STREAMING_API.md#safe-cut-selection).

`RunContext.InsertGlyphs` and `InsertGlyphCopies` cannot grow a public shaping
run beyond 65536 entries. A rejected insertion leaves buffers unchanged and
records a supported-limit error returned by the pipeline before any current
carry is emitted. Earlier output remains final. The hooks keep their existing
void signatures; callers need not handle an additional insertion return value.

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
| `ShapingEngineNormalizationReorderHook` | `ReorderNormalizationMarks([]rune, []uint32)`, once per sorted nonzero-class span before composition; supersedes the late reorder hook. Only permutes aligned runes and original scalar sources. |
| `ShapingEngineReorderHook` | `ReorderMarks(RunContext, int, int)`, invoked once over the mapped run unless the engine implements the normalization reorder hook. |
| `ShapingEnginePreGSUBHook` | `PrepareGSUB(RunContext)`, after reorder. |
| `ShapingEngineMaskHook` | `SetupMasks(RunContext)`, after initial mask setup; execution preserves its changes, with mask inheritance through substitutions. |
| `ShapingEnginePostprocessHook` | `PostprocessRun(RunContext)`, after lookup/position-policy execution. |

There is no custom decomposition hook, `MarksBehavior`, `GposTag`, or
script-specific streaming-boundary hook. Normalization modes are `Auto`,
`None`, `Composed`, and `Decomposed` (with the `Normalization` prefix).
Composed/decomposed modes are font-aware representation preferences. `None`
bypasses normalization, early reordering and composition hooks. `ComposeUnicode` and accepted
`Compose` results require a nonzero cmap mapping; unavailable compositions leave
the supported components intact. Canonical ordering precedes starter-based
composition. Hebrew uses modified combining classes for sorting and blocking,
and its special meteg adjustment runs between sorting and recomposition. See
[normalization](SHAPING_OVERVIEW.md#normalization).

The public path reads `ApplyGPOS`, but a false value produces an all-zero
internal policy that compilation resets to GPOS-on. The internal strictness
switch is not exposed by the policy interface. See [SHAPER_PLAN.md](SHAPER_PLAN.md).

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

`HasGposMark` reports enabled, resolved GPOS `mark` lookup contributors for the
selected font/script/language plan. Defaults and required features count even
without a mask; `mkmk` alone, empty bindings, and absent requested features do
not. A ranged enablement counts across the entire plan for normalization;
positioning still respects the ranges. This is not a per-glyph coverage query
and cannot predict later mask hooks. Feature events share one combined plan
and normalization run per buffered carry (SHAPE-016).

## Actual plan-time order

In [plan_compile.go](../otshape/plan_compile.go):

1. Seed base feature defaults.
2. `CollectFeatures`, then user settings, then `OverrideFeatures`.
3. Allocate masks and resolve table features/stages.
4. Include registered pause boundaries and compute fallback/resolved-feature metadata.
5. `PostResolveFeatures` may anchor GSUB pauses.
6. `InitPlan`, followed by `ValidatePlan` when both interfaces are implemented.
7. Validate the internal plan.

**Feature precedence (SHAPE-021, fixed and accepted 2026-11-02):** settings made in
`CollectFeatures` are engine defaults. User globals replace those settings;
user ranges override them within their bounds. Explicit `OverrideFeatures`
assignments replace all user globals/ranges/scopes for the tag across the whole
input. Lookup selection and glyph masks derive from the same resolved settings.
Scope pops restore enclosing settings before applying the final engine override.
Required features remain unconditional; existing range-only defaults are retained
when the engine supplied no explicit setting. See
[SHAPE-021](SHAPING_FINDINGS.md#shape-021--engine-and-user-feature-settings-combine-as-off-wins).

`PauseHook` is `func(PauseContext) error`; an error stops execution.
`AddGSUBPause` closes the current stage at its registration position, even with
a nil callback. Empty and consecutive stages are retained. A feature keeps its
earliest registered stage; unregistered defaults/user features join the final
stage, while a required feature defaults to stage zero. Lookups within each
stage run once in LookupList order. Anchored Before/After callbacks surround
the whole stage containing the tag, including its other features. Shared-lookup
contributors remain available as anchors. The public API has no GPOS pauses;
GSUB pauses do not advance GPOS stages. See [SHAPER_PLAN.md](SHAPER_PLAN.md).

## Actual runtime order

For each current raw carry buffer, including all its feature-event scopes:

1. Form initial base/mark clusters, then decompose, stably sort, call
   `ReorderNormalizationMarks` for each nonzero-class span when implemented, and
   recompose (consulting `Compose`). Retain normalized source anchors
   separately from output clusters.
2. Map runes through cmap into a `runBuffer`.
3. `PreprocessRun`.
4. `ReorderMarks(run, 0, run.Len())`, only for engines without the early hook.
5. `PrepareGSUB`.
6. Initialize feature masks and apply indexed ranges using cluster starts.
   A start inside an initial base/mark group excludes it; an end inside includes it.
7. `SetupMasks`.
8. Enter `planExecutor.apply` with the prepared, engine-customized masks.
9. Annotate GSUB joiners/syllables and execute GSUB stages and pauses.
10. Execute GPOS when the compiled policy enables it.
11. Apply internal position policies. GPOS-enabled Latin/Hebrew plans cancel
    final advances of GDEF-classified marks by storing negative nominal
    horizontal metrics as adjustments; offsets and attachments are preserved.
    Missing GDEF/class tables do not trigger Unicode or attachment inference.
12. Finalize horizontal Latin/Hebrew mark attachment offsets from anchor
    placement, target X placement, and final pen differences.
13. `PostprocessRun`.
14. Select an output prefix and emit `GlyphRecord` values, adding nominal
    horizontal advances during materialization.

This order is taken from [shape_api.go](../otshape/shape_api.go),
[normalize.go](../otshape/normalize.go), [output.go](../otshape/output.go),
[plan_exec.go](../otshape/plan_exec.go), and
[position_policies.go](../otshape/position_policies.go), not from the earlier
proposed hook order.
There is no general combining-class sort before the reorder hook, dotted-circle
prelude, default-ignorable cleanup, or direction-restore phase here.
Horizontal Latin/Hebrew mark anchors are finalized before `PostprocessRun`.
Cursive and hinted/device/variation anchor adjustments remain incomplete.
SHAPE-019 adds GSUB provenance for GPOS mark-to-ligature component selection,
including preserved marks and nested ligatures. It travels with position
records across stages; final output strips this transient state. Hooks that
adjust existing positions should retain the `PosItem` returned by `Pos` and
modify its fields rather than replace it with a new record if later GPOS still
needs the association. See SHAPE-007/019 in [the findings](SHAPING_FINDINGS.md).

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
SHAPE-001 now preserves these mask decisions. This alone does not establish
correct joining behavior or context-safe streaming.

In event mode, SHAPE-016 initializes the engine for the combined plan immediately
before its run hooks. Synthetic-engine tests verify shared masks, whole-carry
hooks, instance isolation, and validation failure; they do not establish Arabic
joining or other deferred script conformance.

## Testing and pending contracts

Base-package tests use synthetic engines; concrete-engine behavior is tested in
the engine subpackages. Existing tests cover composition/reorder hooks,
validation, pauses, masks, and script helpers. The public-API probes and coverage
limits are recorded in [TESTING.md](TESTING.md).

Mask preservation is covered by the ordinary SHAPE-001 regression for both public
entry points. Remaining interface concerns include validation-hook gating,
preprocessing order, GPOS opt-out, and engine/user precedence. Combined event-plan
state now has SHAPE-016 regression coverage. Source comments and a
declared interface alone do not demonstrate that every policy is honored.
