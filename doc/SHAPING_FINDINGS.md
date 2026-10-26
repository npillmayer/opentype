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
Latin or Hebrew shaping conformance. The discovery batches add tests and record
findings, without production fixes or new external test dependencies.

## Test location and execution

The first four reproductions live in
[`otshape/otcore/shared_api_knownbugs_test.go`](../otshape/otcore/shared_api_knownbugs_test.go),
in external package `otcore_test`. They exercise exported `NewShaper`/`Shape`
and engine hooks. This keeps the base `otshape` tests independent of concrete
engine imports. The file reuses `loadMiniOTFont` from `gpos_test.go` and
`glyphCollector` from `otcore_test.go`, adding only small fixture/assertion
helpers and two test engines.

Those four cases use
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

# Discovery suite: thirteen failing top-level tests across the two packages.
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1

# Inspect one finding (replace 001 with 002, 003, or 004).
go test -tags shaping_knownbugs ./otshape/otcore -run '^TestKnownBugSHAPE001' -count=1 -v
```

Original shared-API batch verification baseline: production source at
`19aa02407cd4066f2dd48baa933c1b0d0ce57fdd`, tested on **2026-10-26** with
the new regression file and documentation changes in the working tree.
The following validation statement refers to that original batch; the AOTS
batch has a separate baseline recorded under SHAPE-005/006.

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
| [SHAPE-005](#shape-005--gdef-classification-prevents-ignoremarks) | reproduced | GDEF range endpoints and class constants break IgnoreMarks/IgnoreBaseGlyphs filtering |
| [SHAPE-006](#shape-006--contextual-lookup-revisits-matched-input) | reproduced | Contextual lookup progression applies overlapping matches |
| [SHAPE-007](#shape-007--mark-attachments-do-not-resolve-to-origins) | reproduced | Mark attachment metadata does not become final anchor offsets |
| [SHAPE-008](#shape-008--marks-retain-nominal-advances) | reproduced | Mark advances shift following glyph origins |

The AOTS baseline batch adds SHAPE-005/006 in
[`internal/hbcmp/aots_knownbugs_test.go`](../internal/hbcmp/aots_knownbugs_test.go).
It reads unchanged upstream records through a test-only adapter. See the
[adapter report](../testdata/harfbuzz/aots/ADAPTER.md) for the explicit context,
independent HarfBuzz verification, passing case, and output limitations.
At the AOTS batch revision, the ordinary full suite passes and the combined
tagged command above fails on exactly SHAPE-001 through SHAPE-006.
The alternate-substitution extension at revision
`88627506dae374b163968c67cf97601a4ada18c1` adds three tagged tests for existing
findings SHAPE-002/003/005, bringing the total to nine failing top-level tests
for six findings. The ordinary full suite still passes.
The [GPOS batch](../testdata/harfbuzz/aots/GPOS.md) at revision
`f560698c2ecf09ed0c317a78327862707e5d8f46` adds three more tagged tests:
SHAPE-007/008 and a further SHAPE-005 reproduction. The combined tagged suite
now has twelve failing top-level tests for eight findings; the ordinary suite
still passes. All batch revisions refer to source plus the tests added at that step.
The [pair-positioning batch](../testdata/harfbuzz/aots/PAIR.md) at revision
`a16f51a27adce48fcd0346f96de0bb8b0231faa6` adds another SHAPE-005 regression,
bringing the tagged suite to thirteen failing tests for the same eight findings.
Its two ordinary pair cases and the full ordinary suite pass.

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

**Independent AOTS extension (2026-10-26):**
`TestKnownBugSHAPE002AOTSAlternateRangesStreaming` runs the unchanged active
[`gsub3_1_simple.tests:2`](../testdata/harfbuzz/aots/tests/gsub3_1_simple.tests)
with high/low/max watermarks `2/1/4`, using Latin/English/LTR and the core engine.
Expected GIDs are `[17,18,17,20,17,21,17,22,17,18,17,20,17]`; actual GIDs are
`[17,18,17,18,17,18,17,18,17,18,17,18,17]`, with nil error. Positions 3 and 11
request the first alternate and work in the whole-input run, but fail here.
This supports the range-coordinate finding independently of alternate values
2/3, which are also broken (SHAPE-003). The upstream record combines both
concerns; the original four-rune test remains the isolated range reproduction.
See [ADAPTER.md](../testdata/harfbuzz/aots/ADAPTER.md#alternate-substitution-batch)
for revision, exact option translation, and independent HarfBuzz verification.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE002AOTS' -count=1 -v
```

## SHAPE-003 — Feature values do not select alternates

**Status:** reproduced; selection contract agreed on 2026-10-26. Production
fixes and the full contract-test expansion are reserved for a separate session.
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

**Independent AOTS extension (2026-10-26):**
`TestKnownBugSHAPE003AOTSAlternates` uses active upstream
[`gsub3_1_simple.tests:2`](../testdata/harfbuzz/aots/tests/gsub3_1_simple.tests),
whose input alternates U+0011 and U+0012 over 13 scalars. Features are exactly
`-test[1],test[3],test[5]=2,test[7]=3,-test[9],test[11]`: zero-based scalar
indexes, one-based positive alternate values, no global enable request.
The public API receives each `[i]` as `[i,i+1)`, with `On:false` for a minus
prefix and `Arg:1/2/3` for enabled requests. With Latin/English/LTR, the core
engine, and whole-input watermarks `14/1/15`, expected GIDs are
`[17,18,17,20,17,21,17,22,17,18,17,20,17]`; actual GIDs are
`[17,18,17,20,17,20,17,20,17,18,17,20,17]`, with nil error. Ranges select the
correct positions, but values 2 and 3 still produce the first alternate.

This upstream case adds independent evidence for the positive-value contract;
it does not cover enabled zero/default, out-of-range positives, random selection,
or the lower-level API. Adapter mapping tests are separate from shaping-contract
tests. The other recovered alternate record is currently blocked by SHAPE-005's
classification defect and is tracked there rather than treated as an isolated
alternate-value failure. Exact provenance and results are in
[ADAPTER.md](../testdata/harfbuzz/aots/ADAPTER.md#alternate-substitution-batch).

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE003AOTS' -count=1 -v
```

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

## SHAPE-005 — GDEF classification prevents IgnoreMarks

**Status:** reproduced. **Test:** `TestKnownBugSHAPE005AOTSIgnoreMarks`.
**Verified:** 2026-10-26, production revision
`7b9df6582ee7154a8f9be2a2ba718689e4a7681e` plus the new tests.

**Expected behavior and basis:** active upstream record
[`gsub_context1_lookupflag.tests:2`](../testdata/harfbuzz/aots/tests/gsub_context1_lookupflag.tests)
at HarfBuzz revision `29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b` requires
contextual substitution across interleaved mark glyphs. Lookup 4 has
`IgnoreMarks`; nested single substitutions change 20/21/22 to 60/61/62 while
leaving marks 90/91/92 in place. The source/font hashes are checked by the test.

**Reproduction:** `gsub_context1_lookupflag_f1.otf`, input
`U+0000 U+0014 U+005A U+0015 U+005B U+005C U+0016 U+0000`, global `test` enabled,
Latin/English/LTR with the core engine and watermarks `9/1/10`.

**Observed:** `Shape` returns nil and emits `[0,20,90,21,91,92,22,0]`;
expected `[0,60,90,61,91,92,62,0]`. The contextual substitution never applies.
Clusters and positions are not asserted by this upstream record.

**Diagnostic evidence:** two independently testable classification problems
contribute to this failure:

1. The font's GDEF ClassDef format 2 contains singleton ranges `[90,90]`,
   `[91,91]`, and `[92,92]`, each with class 3. The exported class lookup returns
   zero for each. `classDefinitionsFormat2.Lookup` in
   [layout.go](../ot/layout.go) checks `glyph < endGlyphID`, excluding the range
   endpoint. The range includes both endpoints under the
   [OpenType ClassDef definition](https://learn.microsoft.com/en-us/typography/opentype/spec/chapter2#class-definition-table).
2. `GlyphClassDefEnum` in [layout_gdef.go](../ot/layout_gdef.go) assigns
   base/ligature/mark/component the values 0/1/2/3, while GDEF encodes them as
   1/2/3/4. `glyphClass` and `skipGlyph` in
   [feature.go](../otlayout/feature.go) compare the raw class values with those
   constants. Correcting the endpoint alone therefore does not establish proper
   mark filtering. See the [GDEF class definitions](https://learn.microsoft.com/en-us/typography/opentype/spec/gdef#glyph-class-definition-table).

The tagged test asserts the unchanged upstream output, the three class lookups,
and the four class constant values. Existing
`TestGSUBContextSubstFmt1IgnoreMarks` vectors contain no interleaved marks, so
they do not expose this case despite the test name. The broader effects of these
classification defects on other GSUB/GPOS operations remain to be tested,
beyond the additional alternate-lookup case below.

**IgnoreBaseGlyphs extension (2026-10-26):**
`TestKnownBugSHAPE005AOTSIgnoreBaseGlyphs` uses active upstream
[`gsub3_1_lookupflag.tests:2`](../testdata/harfbuzz/aots/tests/gsub3_1_lookupflag.tests).
Input is `U+0011 U+0012 U+0012 U+0012 U+0013 U+0013 U+0013 U+0013 U+0011`;
features are `-test[4],test[5],test[6]=2,-test[7]`. With Latin/English/LTR,
the core engine, and whole-input watermarks `10/1/11`, expected GIDs are
`[17,18,18,18,19,22,23,19,17]`; actual GIDs are
`[17,18,18,18,19,19,19,19,17]`, with nil error.

The font's GDEF ClassDef format 2 has just `[18,18]` with class 1; GID 19 is
unclassified (zero). Lookup 0 has `IgnoreBaseGlyphs`. Because `ot.BaseGlyph`
incorrectly equals zero, `skipGlyph` filters the unclassified GID 19, preventing
both requested substitutions. The singleton defect also affects GID 18, but
the record never enables substitution at its positions, so it cannot establish
correct base filtering. Once classification is fixed, this case will also
exercise SHAPE-003's second-alternate selection; do not assume a single fix
will make it pass. See the [adapter report](../testdata/harfbuzz/aots/ADAPTER.md#alternate-substitution-batch)
for the verification revision and unchanged upstream oracle.

**Interleaved-mark ligature extension (2026-10-26):**
`TestKnownBugSHAPE005AOTSLigatureAcrossMark` runs
[`gpos5.tests:2`](../testdata/harfbuzz/aots/tests/gpos5.tests), whose GSUB lookup
forms ligature 18 from 30 and 31 with `IgnoreMarks`. Input is
`U+0011 U+001E U+0013 U+001F U+0011`, Latin/English/LTR, global `test`, buffers
`6/1/7`. GID 19 lies between the components and has GDEF singleton `[19,19]`
class 3, but is currently classified as zero. The output GIDs remain
`[17,30,19,31,17]` instead of `[17,18,19,17]`, with nil error. The adjacent
component control (`gpos5.tests:3`) forms the ligature but fails final positioning
(SHAPE-007/008). The tagged assertion retains the complete origin expectation;
see [GPOS.md](../testdata/harfbuzz/aots/GPOS.md) for actual/expected origins.

Classification is the first blocker here. Source inspection also shows
`gsubLookupType4Fmt1` replacing the full matched span and GPOS selecting the
last ligature component by default. Preservation of skipped marks and component
association will need separate verification after this blocker is fixed;
this run does not establish those later behaviors.

**Pair-advance extension (2026-10-26):**
`TestKnownBugSHAPE005AOTSPairAdvance` uses active upstream
[`gpos2_1_lookupflag.tests:3`](../testdata/harfbuzz/aots/tests/gpos2_1_lookupflag.tests)
with newly recovered `gpos2_1_lookupflag_f2.otf`. Input is
`U+0011 U+0013 U+0014 U+0011 U+0013 U+0012 U+0014 U+0011`, core engine,
Latin/English/LTR, global `test`, buffers `9/1/10`. The only lookup is GPOS
type 2 format 1, with IgnoreBaseGlyphs. Pair 19/20 changes the first advance by
−200 and the second Y offset by −100. GDEF assigns only singleton `[18,18]`
class 1; GIDs 19/20 are unclassified (zero), so the incorrect `BaseGlyph=0`
constant skips them and no pair executes.

`Shape` returns nil and nominal positions. The first trailing GID 17 starts
at X=4500 rather than 4300; the last starts at 10500 rather than 10100. The
complete exact expectations, actual origins, hashes, import procedure, and
HarfBuzz verification are in [PAIR.md](../testdata/harfbuzz/aots/PAIR.md).
The companion unflagged pair font passes matching and nonmatching public-API
cases. This failure is therefore tracked under classification; it does not
isolate an advance-arithmetic defect. Both class constants and range endpoints
must be considered before claiming that the flagged case is fixed.

**Reproduce:**

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE005' -count=1 -v
```

## SHAPE-006 — Contextual lookup revisits matched input

**Status:** reproduced. **Test:** `TestKnownBugSHAPE006AOTSContextProgression`.
**Verified:** 2026-10-26, production revision
`7b9df6582ee7154a8f9be2a2ba718689e4a7681e` plus the new tests.

**Expected behavior and basis:** active upstream record
[`gsub_context1_next_glyph.tests:2`](../testdata/harfbuzz/aots/tests/gsub_context1_next_glyph.tests)
at the same pinned HarfBuzz revision expects successive non-overlapping matches
of a two-glyph context. The fixture's lookup 4 matches `[20,20]` and invokes
lookup 0 to replace the first glyph with 60. Processing then continues beyond
the matched input pair.

**Reproduction:** `gsub_context1_next_glyph_f1.otf`, input
`U+0000 U+0014 U+0014 U+0014 U+0014 U+0014 U+0000`, global `test` enabled,
Latin/English/LTR with the core engine and watermarks `8/1/9`.

**Observed:** `Shape` returns nil and emits `[0,60,60,60,60,20,0]`;
expected `[0,60,20,60,20,20,0]`. The pipeline substitutes extra glyphs by
revisiting positions inside previously matched contexts. Clusters and positions
are not asserted by this upstream record.

**Implementation evidence:** `gsubLookupType5Fmt1` in
[gsub.go](../otlayout/gsub.go) returns its incoming `pos` after successful nested
application rather than the position following the matched context.
`applySequenceLookupRecords` in [feature.go](../otlayout/feature.go) restores
the caller's index; `applyLookupSpanInternal` in
[lookups.go](../otshape/lookups.go) increments an unchanged index by one.
This path explains the overlapping matches observed here. The fix must preserve
partial-progress and edit-remapping contracts; length-changing contexts and
other contextual formats are outside this reproduction's scope.

**Reproduce:**

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE006' -count=1 -v
```

## SHAPE-007 — Mark attachments do not resolve to origins

**Status:** reproduced. **Test:** `TestKnownBugSHAPE007AOTSMarkAttachmentOrigins`.
**Verified:** 2026-10-26, source revision
`f560698c2ecf09ed0c317a78327862707e5d8f46` plus new tests.

**Expected behavior and basis:** active upstream `gpos4_simple.tests:2` and
`gpos5.tests:3` require a mark origin aligned with its base or ligature anchor.
The public output must contain the offsets needed to reproduce those origins;
attachment references alone do not satisfy the baseline.

**Reproduction:** core engine, Latin/English/LTR, global `test`, one shaping
window. Mark-to-base input is `U+0011 U+0012 U+0013 U+0011`, buffers `5/1/6`;
mark-to-ligature input is `U+0011 U+001E U+001F U+0013 U+0011`, buffers `6/1/7`.
The font/source hashes and exact upstream expectations are checked at runtime.

**Observed:** both calls return nil and the correct GIDs `[17,18,19,17]`.
GID 19 has `AttachTo:1` and the appropriate mark-to-base/mark-to-ligature kind
and anchor references, but `XOffset/YOffset` remain zero. Its origin is
`(3000,0)`, rather than `(1400,-80)` for the base or `(1401,-79)` for the ligature.
The final glyph also has the wrong origin due to SHAPE-008. Complete sequences
and independent HarfBuzz checks are in [GPOS.md](../testdata/harfbuzz/aots/GPOS.md).

**Implementation evidence:** `gposLookupType4Fmt1`/`gposLookupType5Fmt1` in
[gpos.go](../otlayout/gpos.go) call `setMarkAttachment` in
[gpos_helpers.go](../otlayout/gpos_helpers.go). That helper records metadata
without resolving anchor coordinates. The public execution/output path does
not subsequently turn those references into offsets. Existing attachment tests
assert metadata, so their success does not establish final positioning.

**Fix scope:** resolve the appropriate anchors and relative pen positions before
emitting public output. Retain separate checks for mark advance handling,
ligature component association, mark-to-mark chains, and streaming boundaries;
the two cases here do not establish those other behaviors.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE007' -count=1 -v
```

## SHAPE-008 — Marks retain nominal advances

**Status:** reproduced. **Test:** `TestKnownBugSHAPE008AOTSUnattachedMarkAdvances`.
**Verified:** 2026-10-26, source revision
`f560698c2ecf09ed0c317a78327862707e5d8f46` plus new tests.

**Expected behavior and basis:** active upstream `gpos4_simple.tests:3` and `:4`
place an unattached mark and the following glyph at `(3000,0)`. The mark must
not add its nominal 1500-unit horizontal advance in these cases. This expectation
comes from the synthetic font's upstream baseline, not from treating its control
codepoint as a Unicode combining character.

**Reproduction:** `gpos4_simple_1.otf`, input `U+0011 U+0011 U+0013 U+0011` or
`U+0019 U+0019 U+0013 U+0011`, core engine, Latin/English/LTR, global `test`,
buffers `5/1/6`. Neither input has a covered attachment base for GID 19.

**Observed:** nil errors; GIDs and the mark's own origin match. The following
GID 17 starts at `(4500,0)`, not `(3000,0)`. Emitted GID 19 has `XAdvance:1500`,
zero offsets, and no attachment. These controls isolate the extra displacement
from SHAPE-007's unresolved anchor problem. Attached marks in SHAPE-007 also
retain 1500-unit advances. See [GPOS.md](../testdata/harfbuzz/aots/GPOS.md) for
the full outputs and supplementary HarfBuzz advance diagnostics.

**Implementation evidence:** `compileShapePlanWithFeatures` in
[shape_api.go](../otshape/shape_api.go) enables GPOS but leaves `ZeroMarks` false.
The helpers in [plan.go](../otshape/plan.go) can zero internal position values
when that policy is set, but the public output path later adds each glyph's
nominal metric in `materializeGlyphRecord`. Merely enabling that helper is
therefore not enough to establish zero final advances. GDEF classification
(SHAPE-005) also affects unattached mark recognition. A fix must address the
policy, classification, and nominal-versus-adjustment representation together.
This finding does not prescribe zero advances for every mark in every script.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE008' -count=1 -v
```

## Remaining discovery work

The shared-API and initial AOTS batches are not a complete defect inventory.
[TESTING.md](TESTING.md#next-testing-priorities) retains the wider priorities
and earlier inspection/review observations. Next shared-API cases include
Latin/Hebrew script and language selection, range behavior under normalization,
and feature scopes through `ShapeEvents`. Add stable findings here as those
cases are investigated; do not silently equate missing coverage with a bug.
