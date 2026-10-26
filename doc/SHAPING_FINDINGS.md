# Shaping API findings

## Process and scope

This file records evidence from tests of the shaping API: bugs, missing
functionality, and contract questions. [TESTING.md](TESTING.md) describes the
test architecture and priorities; [TO_DO.md](TO_DO.md) remains the broader
roadmap and feature-scope record. Keep detailed reproductions here and link to
their stable `SHAPE-NNN` IDs from those documents instead of maintaining several
copies of each issue.

Work proceeds in small batches:

1. State the expected API behavior and its basis before writing an assertion.
   Identify an unresolved contract choice explicitly rather than presenting it
   as an established guarantee.
2. Reuse checked-in mini-fonts, existing loaders, and output collectors. Add the
   smallest public-API reproduction and a passing control where useful. Check
   glyph IDs and input correspondence, plus positions/masks/errors when relevant.
   Use lower-level tests to localize a problem without substituting them for an
   integration test.
3. Run the reproduction and record the input, options, expected and observed
   behavior, test command, implementation locations, and verification revision.
   Source-inspection suspicions remain distinct from test-confirmed findings.
4. Keep desired-behavior assertions for known failures under the
   `shaping_knownbugs` build tag. This opt-in suite is expected to fail while
   these findings remain open; the ordinary suite must remain green. Never
   change the expectation to the buggy output merely to make a test pass.
5. Handle fixes separately from discovery. Once fixed, move each regression
   into the default suite (and move any needed helpers with it), rerun the
   relevant tests and ordinary suite, and mark the finding fixed. Retain the ID
   and a short resolution history. A tagged test unexpectedly passing is a
   reason to investigate and promote it, not evidence to leave it hidden.

Statuses are **reproduced**, **needs decision**, **fixed**, or **deferred**.
Use **needs decision** when the required behavior is not settled, even if the
current behavior is reproducible. Deferred items retain their evidence and
reason for deferral. Inspection-only notes should say so and must not claim a
successful reproduction. Each new entry should follow the fields used below.

The current scope is shared API correctness needed by Latin and Hebrew.
Arabic/Syriac and other script-specific extensions are deferred. Synthetic
fonts are suitable for isolating shared behavior; these tests do not establish
Latin or Hebrew shaping conformance. This first batch adds tests and records
findings, without production fixes or new external test dependencies.

## Test location and execution

The reproductions live in
[`otshape/otcore/shared_api_knownbugs_test.go`](../otshape/otcore/shared_api_knownbugs_test.go),
in external package `otcore_test`. They exercise exported `NewShaper`/`Shape`
and engine hooks. This keeps the base `otshape` tests independent of concrete
engine imports. The file reuses `loadMiniOTFont` from `gpos_test.go` and
`glyphCollector` from `otcore_test.go`, adding only small fixture/assertion
helpers and two test engines.

All four cases use
[`gsub3_1_simple_f1.otf`](../testdata/fonttools/gsub3_1_simple_f1.otf).
The helper finds the cmap-backed rune for GID 18 and verifies that it maps back
to 18; missing/broken fixtures fail the test. The paired
[`TTX dump`](../testdata/fonttools/gsub3_1_simple_f1.ttx.GSUB) lists the ordered
alternatives 20, 21, and 22. Unless overridden below, parameters are LTR,
Latin, English, and feature `test` enabled. These are synthetic inputs, not
natural-language examples.

Run from the repository root:

```sh
# Ordinary regression suite: must pass.
go test ./... -count=1

# Discovery suite: currently expected to exit nonzero on four top-level tests.
go test -tags shaping_knownbugs ./otshape/... -count=1

# Inspect one finding (replace 001 with 002, 003, or 004).
go test -tags shaping_knownbugs ./otshape/otcore -run '^TestKnownBugSHAPE001' -count=1 -v
```

Verification baseline: production source at
`19aa02407cd4066f2dd48baa933c1b0d0ce57fdd`, tested on **2026-10-26** with
the new regression file and documentation changes in the working tree.
The commands below refer to that snapshot, not a guarantee about later revisions.

Validation at this snapshot: `go test ./... -count=1` passed. The tagged
shaping suite failed on exactly the four listed top-level tests; their control
cases passed, as did the other shaping packages. These are assertion failures
demonstrating the recorded behavior, not fixture-loading or build failures.

| ID | Status | Finding |
| --- | --- | --- |
| [SHAPE-001](#shape-001--setupmasks-changes-are-overwritten) | reproduced | `SetupMasks` changes are overwritten before lookup execution |
| [SHAPE-002](#shape-002--feature-ranges-use-chunk-local-coordinates) | reproduced | Ranged feature behavior changes with buffer size |
| [SHAPE-003](#shape-003--feature-values-do-not-select-alternates) | reproduced | Ignored values; contract agreed: one-based positives, zero default, negatives random |
| [SHAPE-004](#shape-004--zero-confidence-engines-are-selected) | reproduced | Engines reporting no match can still shape input |

## SHAPE-001 — SetupMasks changes are overwritten

**Status:** reproduced. **Test:** `TestKnownBugSHAPE001SetupMasksPreserved`.

**Expected behavior and basis:** `SetupMasks` is the engine hook for assigning
per-glyph feature masks before GSUB. Clearing the first glyph's mask should
disable the enabled lookup for that glyph while preserving it for the second.

**Reproduction:** repeat the fixture rune twice; high/low/max watermarks
`8/4/8` keep the input together. A test engine embeds the core engine and clears
the first glyph's mask through `RunContext.SetMask(0, 0)`. A separate core-engine
control expects `[20,20]`, and the test checks that the hook was called.

**Observed:** the control passes. The hook case also emits `[20,20]`, with
first-glyph mask `1`; expected `[18,20]` with first-glyph mask `0`. Both retain
clusters `[0,1]`. Engine-specific mask decisions have no effect in this case.

**Implementation evidence:** `shapeMappedRun` initializes masks, calls the
hook, and invokes the executor in [shape_api.go](../otshape/shape_api.go).
`planExecutor.apply` calls `ensureRunMasks` again in [plan.go](../otshape/plan.go),
resetting them via [lookups.go](../otshape/lookups.go).

**Reproduce:** use the individual command above with `SHAPE001`.

## SHAPE-002 — Feature ranges use chunk-local coordinates

**Status:** reproduced. **Test:** `TestKnownBugSHAPE002FeatureRangeUsesInputCoordinates`.

**Expected behavior and basis:** `FeatureRange.Start/End` select input
codepoints, so an unchanged input and range should have the same result across
buffer configurations. Range `[2,3)` should enable only the third input rune.

**Reproduction:** repeat the fixture rune four times, with `test` enabled for
`[2,3)`. Compare explicit whole-input watermarks `8/4/8` with streaming
watermarks `2/1/4`. Each subtest asserts the independently specified output
`[18,18,20,18]` and clusters `[0,1,2,3]`.

**Observed:** `whole_input` passes; `streaming` emits `[18,18,18,18]`.
Clusters remain correct, but the requested substitution is lost.

**Implementation evidence:** `ensureRunMasks` and `applyFeatureRangesToMasks`
in [lookups.go](../otshape/lookups.go) apply ranges to current-buffer indexes
instead of original input coordinates. This reproduction covers a one-to-one
mapping; normalization and length-changing substitution require further cases.

**Reproduce:** use the individual command above with `SHAPE002`.

## SHAPE-003 — Feature values do not select alternates

**Status:** reproduced; selection contract agreed on 2026-10-26. Implementation
and test changes are reserved for a separate session.
**Test:** `TestKnownBugSHAPE003FeatureValueSelectsAlternate`.

**Agreed contract:**

- Positive alternate-selection values are **one-based**: `1` selects the first
  alternate, `2` the second, and so on.
- Zero means **default/first alternate** in both APIs, including
  `ApplyFeature(..., alt=0)` and enabled `FeatureRange.Arg == 0`.
- Any negative value requests **random selection** from the glyph's available
  alternates. Negative values are an explicit exception to the out-of-range
  rule, not invalid indexes or requests for the last alternate.
- An out-of-range positive selection leaves the glyph unchanged by that alternate
  substitution. It must not clamp to an available alternate or wrap around.
  The range is determined by the alternate set for the particular glyph;
  other applicable shaping operations may still change that glyph.
- Both public `FeatureRange.Arg` and lower-level `ApplyFeature` must use these
  same value semantics. Updating only the public pipeline to translate into
  the existing zero-based implementation would not fulfill this decision.

`FeatureRange.Arg == 0` retains its documented default-on meaning when `On` is
true; `On: false` disables the feature. For this fixture, the public API should
therefore select GID 20 for `0` and `1`, GID 21 for `2`, GID 22 for `3`, and
leave input GID 18 unchanged for values above `3`.

**Implementation consequences:** the value categories above are settled.
The shaping pipeline already passes `-1` for `lookupRandom`, but the lower-level
implementation currently selects the **last** alternate for every negative
value. That is not actual random selection and does not fulfill the agreed
contract. The implementation session must provide random selection and a
reproducible way to test it; no particular random generator, seed policy, or
distribution has been specified by this decision.

The future implementation should update lower-level callers, comments, and
tests together with the public pipeline. Add boundary tests for each glyph's
alternate count, negative/random and oversized values, and the default-zero behavior.
Preserve values through mask encoding without truncating them into a different
valid selection or losing the negative/random request. Overlapping feature ranges and event scopes need further
coverage but are not additional alternate-indexing decisions in this finding.

**Reproduction:** shape one fixture rune with `Arg` values `0`, `1`, `2`, and
`3`, using default buffer options. Each case asserts its agreed public GID and
cluster zero against the alternatives in the paired TTX.
The existing test covers neither out-of-range/random values nor the requested
lower-level API migration; those extensions remain for the implementation session.

**Observed:** all values emit GID 20. The `arg_0` and `arg_1` controls pass;
`arg_2` and `arg_3` fail. The public pipeline cannot select the later alternates
in this fixture regardless of which positive-index convention is chosen.

**Implementation evidence:** `applyLookups` in [lookups.go](../otshape/lookups.go)
passes `alt=0` for ordinary lookups rather than translating the requested
feature value. The lower-level `ApplyFeature` execution currently uses zero-based
alternate indexes; [feature_functional_test.go](../otlayout/feature_functional_test.go)
exercises that existing convention successfully. Those assertions describe the
current implementation and must change with the agreed contract. See also
`gsubLookupType3Fmt1` in [gsub.go](../otlayout/gsub.go) for the negative sentinel
and out-of-range handling.

**Reproduce:** use the individual command above with `SHAPE003`.

## SHAPE-004 — Zero-confidence engines are selected

**Status:** reproduced. **Test:** `TestKnownBugSHAPE004RejectNonmatchingEngines`.

**Expected behavior and basis:** an engine returning `ShaperConfidenceNone`
reports no match. If all supplied engines return it, `Shape` should return
`ErrNoMatchingShaper` before consuming input or emitting output, consistent
with that error's documented purpose and selection preceding execution.

**Reproduction:** shape one fixture rune using a minimal test engine whose
`Match` returns a configurable confidence. The positive control returns
`ShaperConfidenceLow`; failure cases supply either one or two engines returning
`ShaperConfidenceNone`. Default buffer options are sufficient.

**Observed:** the positive control emits GID 20 at cluster zero and passes.
Both no-match cases return nil, consume the input, and emit one glyph. A caller
cannot rely on the error to detect unsupported engine selection.

**Implementation evidence:** `selectShapingEngine` in
[shape_api.go](../otshape/shape_api.go) rejects scores below zero, but accepts
zero; `ShaperConfidenceNone` is zero in [engine_api.go](../otshape/engine_api.go).

**Reproduce:** use the individual command above with `SHAPE004`.

## Remaining discovery work

This is the first shared-API batch, not a complete defect inventory.
[TESTING.md](TESTING.md#next-testing-priorities) retains the wider priorities
and earlier inspection/review observations. Next shared-API cases include
Latin/Hebrew script and language selection, range behavior under normalization,
and feature scopes through `ShapeEvents`. Add stable findings here as those
cases are investigated; do not silently equate missing coverage with a bug.
