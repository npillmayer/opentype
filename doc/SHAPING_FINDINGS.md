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
   and original heading anchor in a compact entry under **Resolved findings**,
   with the cause, fix, and permanent regression links. Remove resolved issues
   from active-gap discussions in `TESTING.md`, retaining coverage descriptions.
   Git preserves the detailed investigation history. A tagged test unexpectedly
   passing is a reason to investigate and promote it, not evidence to leave it hidden.

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

The remaining shared-API reproductions (SHAPE-004/018) live in
[`otshape/otcore/shared_api_knownbugs_test.go`](../otshape/otcore/shared_api_knownbugs_test.go),
in external package `otcore_test`. They exercise exported `NewShaper`/`Shape`
and engine hooks. This keeps the base `otshape` tests independent of concrete
engine imports. The ordinary [`shared_api_test.go`](../otshape/otcore/shared_api_test.go) now
contains SHAPE-001/002 and the shared fixture/assertion helpers;
[`alternate_values_test.go`](../otshape/otcore/alternate_values_test.go) contains
the ordinary SHAPE-003 value/range/scope regressions. These tests reuse
`loadMiniOTFont` from `gpos_test.go` and `glyphCollector` from `otcore_test.go`.

These shared-API cases use
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

# Discovery suite: twenty failing top-level tests across the two packages.
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1

# Fixed mask regression, covering both public entry points.
go test ./otshape/otcore -run '^TestSHAPE001' -count=1 -v

# Inspect an unresolved shared-API finding.
go test -tags shaping_knownbugs ./otshape/otcore -run '^TestKnownBugSHAPE018AlternateValueAtMatchedGlyph$' -count=1 -v
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
| [SHAPE-002](#shape-002--feature-ranges-use-chunk-local-coordinates) | fixed (agreed scope) | Ranges use original scalar anchors across compaction and normalization |
| [SHAPE-004](#shape-004--zero-confidence-engines-are-selected) | reproduced | Engines reporting no match can still shape input |
| [SHAPE-005](#shape-005--gdef-classification-prevents-ignoremarks) | reproduced | GDEF range endpoints and class constants break IgnoreMarks/IgnoreBaseGlyphs filtering |
| [SHAPE-006](#shape-006--contextual-lookup-revisits-matched-input) | reproduced | Contextual lookup progression applies overlapping matches |
| [SHAPE-007](#shape-007--mark-attachments-do-not-resolve-to-origins) | reproduced | Mark attachment metadata does not become final anchor offsets |
| [SHAPE-008](#shape-008--marks-retain-nominal-advances) | reproduced | Mark advances shift following glyph origins |
| [SHAPE-009](#shape-009--rtl-output-order-differs-from-harfbuzz) | fixed (contract resolved) | Logical sink order agreed; plain Hebrew passes after explicit oracle adaptation |
| [SHAPE-010](#shape-010--default-mark-positioning-is-missed-during-normalization) | reproduced | Hebrew presentation-form fallback ignores default-enabled GPOS mark support |
| [SHAPE-011](#shape-011--combining-mark-clusters-differ-from-harfbuzz-level-0) | reproduced | HarfBuzz level-0 grouping agreed; Latin/Hebrew marks still retain individual scalar clusters |
| [SHAPE-012](#shape-012--canonical-equivalent-mark-orders-shape-differently) | reproduced | Canonical ordering is missing before composition |
| [SHAPE-013](#shape-013--normalization-ignores-font-coverage) | reproduced | Composition/decomposition ignores available cmap mappings |
| [SHAPE-014](#shape-014--fonts-without-gpos-stop-at-the-parser) | deferred | Existing parser scope excludes fonts without GPOS |
| [SHAPE-015](#shape-015--streaming-cuts-lose-shaping-context) | reproduced | Buffer cuts change ligatures, pair positioning, and normalization |
| [SHAPE-016](#shape-016--feature-scopes-sever-unrelated-lookup-context) | reproduced | Feature-event spans break kerning across scope boundaries |
| [SHAPE-017](#shape-017--gsub-deletion-is-ignored) | reproduced | Empty GSUB multiple-substitution sequences leave glyphs unchanged |
| [SHAPE-018](#shape-018--forward-lookup-scans-bypass-feature-masks) | reproduced | A lookup enabled at its starting index can match a later disabled glyph |

Archived fixes in [Resolved findings](#resolved-findings):
[SHAPE-001 — mask preservation](#shape-001--setupmasks-changes-are-overwritten),
[SHAPE-002 — source ranges](#shape-002--feature-ranges-use-chunk-local-coordinates),
and [SHAPE-003 — alternate selection](#shape-003--feature-values-do-not-select-alternates).

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

The [real-text batch](../internal/hbcmp/testdata/text/README.md), verified on
2026-10-26 at source revision `f1deff457223a87754f86d8e17ab133682566787` plus
the new tests, adds eleven complete HarfBuzz captures: six Latin cases and five
Hebrew cases. Seven pass; four fail in three tagged top-level tests. The full
ordinary suite passes; the combined tagged suite has sixteen failing top-level
tests for eleven findings, including the two unresolved contract choices.
Expected outputs come directly from HarfBuzz 15.0.0; the Go suite runs offline.

The [normalization extension](../internal/hbcmp/testdata/text/NORMALIZATION.md)
at source revision `3acce4440f5a215e9752cd2f7c25430b3279994a` adds thirteen
captures and applies the agreed logical-order comparison. The corpus now has
24 cases: 10 passing and 14 tagged, in seven text tests. The combined discovery
suite has twenty failing top-level tests: nineteen shaping assertion failures
and one deferred parser-scope failure (SHAPE-014). The full ordinary suite passes.
SHAPE-009 is resolved by contract; SHAPE-011 is now an agreed requirement;
SHAPE-012/013 are new normalization findings. Only API comments, tests, fixtures,
tools, and documentation changed; production behavior is unchanged.

The [streaming/scope batch](../internal/hbcmp/testdata/text/STREAMING.md), verified
on **2026-10-26** at source `f36813fa22e1b0795a66a4860975612f4c7ce9cc` plus
this batch's tests, adds twelve captures and reuses four existing controls.
Five buffer profiles, two entry points, and two flush modes produce 320 matrix
executions: 134 pass and 186 fail under the discovery tag. Ten additional
empty-input/event checks pass. Six new tagged groups cover SHAPE-015–018 and
extensions of SHAPE-002/011, bringing the combined total to **twenty-six failing
top-level tests**. The full ordinary suite passes; all new failures are output
assertions, with complete input consumption, nil shaping errors, and no panic
or timeout. No production behavior changed. Each new finding below uses this
verification baseline.

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
exercise the now-implemented SHAPE-003 second-alternate selection; preserve
the full expectation until that combination passes. See the [adapter report](../testdata/harfbuzz/aots/ADAPTER.md#alternate-substitution-batch)
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

**Real Hebrew extension (2026-10-26):**
`TestKnownBugSHAPE007HebrewMarkOffsets` uses Noto Sans Hebrew and input
U+05E9 U+05B8 U+05C1 (shin, qamats, shin dot), Hebr/he/RTL, default features,
buffers `4/1/5`. HarfBuzz emits marks GID 79 and 100 with X offsets 227 and 539
and zero advances, followed by base GID 96 with advance 730. Our output is in
logical order and retains separate mark clusters; both marks have zero offsets
despite `AttachTo:0`, classes 4/5, and anchor references. The full assertion
originally exposed SHAPE-009/011. Following the logical-order decision, only
the oracle's final RTL reversal is undone; internal mark ordering, clusters,
and offsets remain exact assertions. This font already has zero nominal mark advances, so it
does not reproduce SHAPE-008. Exact output and provenance are in the
[real-text report](../internal/hbcmp/testdata/text/README.md).

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

## SHAPE-009 — RTL output order differs from HarfBuzz

**Status:** fixed by contract resolution, not by a production reversal.
**Decision (2026-10-26):** the sink receives **logical shaping order**, including
RTL runs. The caller owns visual ordering. Logical order follows normalization
and script-specific reordering; it does not promise original scalar order
inside a cluster. Cluster grouping is independently settled under SHAPE-011.

**Original evidence:** Noto Sans Hebrew, `שלום` (U+05E9 U+05DC U+05D5 U+05DD),
Hebr/he/RTL, default features, buffers `5/1/6`. HarfBuzz's visual output was
`[23/3,124/2,55/1,96/0]` (GID/cluster); our output was the reverse sequence.
Every per-glyph advance/offset already matched. This was an ordering-contract
mismatch, not a reason to add final reversal to the public shaper.

**Resolution:** `textLogicalOutput` copies the unchanged HarfBuzz capture and
undoes its final whole-buffer RTL reversal, including records within a cluster.
Positions remain attached to their glyphs; no sorting, cluster merging, or
position correction is performed. The ordinary
`TestHarfBuzzTextBaselines/hebrew_shalom` now passes all fields and replaces
`TestKnownBugSHAPE009HebrewRTLOrder`. `TestTextLogicalOutputContract` verifies
that the conversion preserves the raw oracle and handles repeated clusters.
The rendering convention and source reference are in the
[normalization report](../internal/hbcmp/testdata/text/NORMALIZATION.md).

This resolves ownership of final ordering, not mark reordering, paragraph bidi,
or streaming safety. Pointed Hebrew still fails for independently recorded
composition, cluster, mark-order, and positioning differences.

```sh
go test ./internal/hbcmp -run 'TestHarfBuzzTextBaselines/hebrew_shalom$|TestTextLogicalOutputContract' -count=1 -v
```

## SHAPE-010 — Default mark positioning is missed during normalization

**Status:** reproduced. **Test:** `TestKnownBugSHAPE010HebrewComposition`
(`hebrew_vav_holam` and `hebrew_shalom_pointed`).
**Verified:** real-text batch revision/date above.

**Expected behavior and basis:** the Hebrew engine's own fallback policy
rejects presentation-form composition when GPOS mark support is present.
HarfBuzz likewise retains separate vav and holam glyphs for this font. Existing
`TestComposeFallbackDisabledWhenGposMarkPresent` tests that policy with a mock
context, but does not test the context supplied by the public shaping pipeline.

**Reproduction:** Noto Sans Hebrew, U+05D5 U+05B9, Hebr/he/RTL, default
features, buffers `3/1/4`. Expected GIDs are `[46,124]`, both cluster 0;
the holam has X offset 92 and advance 0, the vav advance 301. Actual output
is one GID 126, cluster 0, offset 0, advance 301, with nil error. GID 126
maps from the presentation form U+FB4B. The same contraction occurs at input
positions 4/5 of pointed shalom, whose complete baseline expects seven glyphs
but receives six. All exact records are in the
[real-text report](../internal/hbcmp/testdata/text/README.md).

**Implementation evidence:** `planHasGposMark` in
[shape_api.go](../otshape/shape_api.go) tests `plan.Masks.ByFeature` for a nonzero
`mark`/`mkmk` mask. `compileUserFeatureMasks` in [plan.go](../otshape/plan.go)
allocates those masks only for explicit user features. Default GPOS features
can execute with mask zero and therefore do not satisfy this check. The
result reaches `NormalizeContext.HasGposMark`, permitting `hebrewCompose` in
[othebrew.go](../otshape/othebrew/othebrew.go) to map vav + holam to U+FB4B.
The pointed-shin case independently demonstrates active default mark lookups
through its attachment metadata, despite the absence of user feature masks.

**Normalization extension (2026-10-26):** the vav/holam matrix now checks
default, explicitly enabled, disabled, and absent mark features. With explicit
`mark=1,mkmk=1`, the two GIDs match HarfBuzz, but SHAPE-007/011 remain. With
`mark=0,mkmk=0`, HarfBuzz uses fallback GID 126 while our output stays
`[124,46]`: allocating masks is incorrectly treated as enabled support.
The NoMark fixture removes those features while retaining GPOS. Its default
fallback passes; explicitly requesting the absent features again incorrectly
suppresses fallback and emits `[124,46]` instead of `[126]`. The last two
cases extend `TestKnownBugSHAPE010HebrewComposition`. Exact fixtures and output
are in the [normalization report](../internal/hbcmp/testdata/text/NORMALIZATION.md).

**Fix scope:** derive effective mark support from resolved font/plan state,
including enablement, rather than mask allocation alone. Resolving composition
will not by itself fix mark offsets, grouping, or internal mark order; the
tagged tests retain those expectations. Absence of the entire GPOS table is
separately blocked at the parser and deferred under SHAPE-014.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE010' -count=1 -v
```

## SHAPE-011 — Combining-mark clusters differ from HarfBuzz level 0

**Status:** reproduced; cluster contract agreed on 2026-10-26.
**Tests:** `TestKnownBugSHAPE011CombiningClusters`, plus the pointed Hebrew
cases in `TestKnownBugSHAPE007HebrewMarkOffsets` and
`TestKnownBugSHAPE010HebrewComposition`.
**Verified:** real-text batch revision/date above.

**Baseline and reproduction:** pointed shin U+05E9 U+05B8 U+05C1 in Noto Sans
Hebrew, same context as SHAPE-007's extension. HarfBuzz cluster level 0 assigns
the base and both marks cluster 0. Our glyphs 96, 79, 100 retain clusters
0, 1, 2 respectively. Pointed shalom has the same difference for its initial
three scalars. This discrepancy is independent of RTL sequence order.

**Implementation evidence:** input clusters start as scalar indexes; normalization
merges clusters when composing, and selected reordering hooks merge their
changed spans. The ordinary mapping/output path preserves the remaining
individual indexes. No general base/combining-mark cluster grouping is applied
to these inputs. These observations do not imply lost Unicode input: they show
a different grouping convention.

**Agreed contract:** use HarfBuzz **cluster level 0** grouping/merging, anchored
at zero-based original Unicode scalar indexes, not UTF-8 byte offsets. Base
letters and combining marks share a source cluster. Composition, decomposition,
substitution, and reordering must preserve the appropriate input association.
Logical sink order is independent of this grouping (SHAPE-009). A cluster
boundary alone is not proof of a safe shaping/streaming break.

The normalization extension adds canonical `a + dot below + acute`,
`x + acute`, explicitly enabled Hebrew vav/holam marks, and reordered pointed
shin. All have correct GID sequences under the logical-order comparison but
retain individual mark clusters and fail final positioning (SHAPE-007).
`TestKnownBugSHAPE011CombiningClusters` preserves the complete oracle; it does
not ignore positioning to report partial success. See the
[normalization report](../internal/hbcmp/testdata/text/NORMALIZATION.md) for exact
clusters/offsets. Source correspondence must remain available to feature-mask
assignment even when output clusters merge. SHAPE-002 now resolves ranges
against retained scalar anchors; ranges splitting merged source sequences remain
outside that fix's agreed scope.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE011' -count=1 -v
```

**Streaming matrix extension:** `TestKnownBugSHAPE011StreamingBaselines` repeats
`x + acute` and explicitly enabled Hebrew vav/holam marks over all five buffer
profiles, both entry points, and both flush modes. All forty executions fail
the complete oracle. Since whole-input output already fails grouping and final
positions (SHAPE-007), these are not isolated streaming-context reproductions.

## SHAPE-012 — Canonical-equivalent mark orders shape differently

**Status:** reproduced. **Test:** `TestKnownBugSHAPE012CanonicalOrdering`.
**Verified:** 2026-10-26, source `3acce4440f5a215e9752cd2f7c25430b3279994a`
plus this batch's tests. This verification baseline also applies to SHAPE-013/014.

**Expected behavior and basis:** independent HarfBuzz captures of Gentium
`a + U+0323 + U+0301` and `a + U+0301 + U+0323` yield the same GIDs
`[277,2173]`, clusters `[0,0]`, advances `[940,0]`, and X offsets `[0,83]`.
Both inputs are Latn/en/LTR with default features and buffers `4/1/5`.

**Observed:** canonical input gets the expected GIDs, but noncanonical input
gets `[105,2289]`. Both currently retain clusters `[0,2]` and zero offsets.
The canonical case therefore isolates the GID discrepancy from the additional
cluster/position failures; it is not claimed as a fully passing control.

**Implementation evidence:** `normalizeRuneStreamWithScratch` and
`composeRuneStreamInto` in [shape_api.go](../otshape/shape_api.go) use adjacent
pair composition without a preceding general canonical ordering pass.
Composed mode also does not first decompose existing composites. `ComposeUnicode`
in [engine_internal.go](../otshape/engine_internal.go) NFC-normalizes only the
current pair. An earlier composition can consume a mark before a later
lower-combining-class mark is seen. Full sequence normalization and script-specific
ordering need separate attention; this reproduction does not establish all
Hebrew reordering rules.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE012' -count=1 -v
```

## SHAPE-013 — Normalization ignores font coverage

**Status:** reproduced. **Test:** `TestKnownBugSHAPE013UnavailableComposition`.

**Reproduction:** the derived `ShaperTestLatin-NoEacute.ttf` removes only
U+00E9's Unicode cmap entries, retaining `e`, U+0301, glyph outlines, and layout
features. Input is U+00E9 + `X` or `e` + U+0301 + `X`, Latn/fr/LTR, defaults,
whole-input buffers `3/1/4` or `4/1/5`. Hashes, derivation and licensing are in
[font provenance](../testdata/fonts/NORMALIZATION.md).

**Expected behavior and basis:** HarfBuzz emits GIDs `[72,2173,59]`, advances
`[946,0,1260]`, X offsets `[0,107,0]`. Clusters are `[0,0,1]` for NFC and
`[0,0,2]` for NFD, preserving the following `X`'s original input position.

**Observed:** both calls return nil but emit `[0,59]`, with advances
`[1400,1260]`. The last clusters remain 1/2 respectively. NFC fails to decompose
the unavailable character; NFD composes available components into an unavailable
character. The missing glyph is not evidence of missing component mappings.
The existing full-Gentium NFC/NFD café cases remain passing controls for an
available composite, while the independent NoEacute capture establishes the
required fallback path.

**Implementation evidence:** the composed-mode path in
[shape_api.go](../otshape/shape_api.go) does not decompose a mapped-missing
composite. `normalizeContext.ComposeUnicode` in
[engine_internal.go](../otshape/engine_internal.go) and acceptance of composition
results do not check cmap coverage. Fixes must preserve original cluster anchors
through expansion and contraction; resolving the missing glyph will still leave
mark positioning/grouping to be checked against the complete oracle.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE013' -count=1 -v
```

## SHAPE-014 — Fonts without GPOS stop at the parser

**Status:** deferred scope limitation. **Test:** `TestKnownBugSHAPE014AbsentGPOS`.

**Reproduction:** `ShaperTestHebrew-NoGPOS.ttf` removes the entire GPOS table
from pinned Noto Sans Hebrew, preserving GSUB and cmap. HarfBuzz shapes vav +
holam to presentation-form GID 126, cluster 0, advance 301. The baseline uses
Hebr/he/RTL and default features, with intended buffers `3/1/4`.

**Observed:** normal `ot.Parse` returns
`OpenType font format: missing advanced layout table GPOS`; shaping is never
called. This is a deliberately documented library restriction in
[parse.go](../ot/parse.go), which requires both GSUB and GPOS. It is not a new
parser regression or a malformed fixture. The tagged expectation records a
HarfBuzz capability outside that current supported subset; ordinary tests
continue to validate its stored capture and font hash.

**Scope:** extending production font acceptance requires a separate parser-scope
decision and is deferred here. The adapter does not bypass the restriction with
`IsTestfont`. A companion NoMark fixture retains GPOS but removes `mark`/`mkmk`
features: its default fallback passes, and explicit requests for those absent
features reproduce SHAPE-010. Thus the in-scope fallback tests are not blocked
by this broader font-acceptance limitation.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE014' -count=1 -v
```

## SHAPE-015 — Streaming cuts lose shaping context

**Status:** reproduced. **Test:** `TestKnownBugSHAPE015StreamingContext`.

**Contract decision (2026-10-27):** require a generous minimum buffer capacity
and impose documented practical sequence/context limits. Hundreds or thousands
of slots are acceptable; exact limits remain open. Supported runs must preserve
the oracle output, while invalid configurations or exhausted safe buffering
must fail explicitly. Forced incorrect output is not acceptable. See the
[agreed buffer contract](STREAMING_API.md#agreed-buffer-contract-2026-10-27-not-yet-implemented).
The tiny-buffer reproductions below describe currently accepted configurations;
after enforcement, applicable cases become specific rejection tests, accompanied
by boundary regressions at supported sizes. No production or test behavior was
changed by this decision.

**Expected behavior and basis:** buffer sizes and flush modes should not alter
the complete shaped output. Independently captured whole-text HarfBuzz results
supply all GIDs, original scalar clusters, advances, and offsets, in the agreed
logical order. Passing whole-input controls distinguish these cases from
pre-existing normalization/mark failures.

**Reproduction:** Gentium `office office office`, `AVATAR AVATAR`, and three
decomposed `café` words; Noto Hebrew `שלום שלום שלום`; and synthetic `afifiafifi`.
The [matrix report](../internal/hbcmp/testdata/text/STREAMING.md) specifies every
feature setting, watermark, and failing cell. Both public entry points and
both flush modes are affected.

**Observed:** with high/low/max `3/1/8`, the office case emits 15 glyphs instead
of 14: a final `ffi` dependency is split. The accented text emits 15 instead of
14, splitting the second expected `é` at cluster 9 into `e` at 9 and acute at
10. With `3/3/3`, Hebrew loses the first vav's pair adjustment: offset/advance
`0/301` instead of `-10/291`. Whole-input controls pass every field. Disabled
ligature/kerning controls and synthetic expansion pass every configuration.
Some particular alignments pass, including accented text with `3/3/3`;
passing at one small size does not establish safe cuts generally.

**Implementation evidence:** [shape_streaming.go](../otshape/shape_streaming.go)
selects and flushes prefixes using the current run's boundary information,
with forced progress at the cap. These results show that emitted prefixes and
retained context are insufficient for the tested lookup/normalization
relationships. A complete repair must consider both safe-cut information and
hard-cap policy; this discovery batch does not prescribe that implementation.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE015' -count=1 -timeout=30s -v
```

## SHAPE-016 — Feature scopes sever unrelated lookup context

**Status:** reproduced. **Test:** `TestKnownBugSHAPE016FeatureScopeContext`.

**Expected behavior and basis:** a `liga` scope must not disable an otherwise
enabled `kern` relationship across its boundary. Event settings are independently
checked against the equivalent HarfBuzz scalar ranges before shaping.

**Reproduction:** Gentium `AVATAR`, Latn/en/LTR, global `kern=1,liga=1`;
push `liga=0` at scalar 1 and pop at 2. Whole-input buffers `7/1/8` rule out
streaming cuts. HarfBuzz receives `kern=1,liga=1,liga[1:2]=0`.

**Observed:** event output preserves GIDs and clusters but gives the first A
advance 1219 instead of 1059 and V advance 1331 instead of 1130. The remaining
advances match. Equivalent `Shape` ranges pass whole/lookahead/carry profiles;
`ShapeEvents` fails every profile and both flush modes.

**Implementation evidence:** `shapeEventCarry` in
[shape_events.go](../otshape/shape_events.go) partitions input by plan ID,
normalizes/maps/shapes each span separately, and appends the results. Lookup
context cannot cross those spans even for features unchanged by the push/pop.
This reproduces a scope-boundary problem independently of SHAPE-015.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE016' -count=1 -timeout=30s -v
```

## SHAPE-017 — GSUB deletion is ignored

**Status:** reproduced. **Test:** `TestKnownBugSHAPE017GSUBDeletion`.

**Expected behavior and basis:** GSUB type 2 with an empty replacement sequence
removes the matched glyph. The repository-authored
[progress font](../testdata/fonts/STREAMING.md) contains `sub d by NULL` and
both required layout tables; normal parsing is used. HarfBuzz supplies the
independent empty/nonempty outputs.

**Reproduction:** `adadada` and `dddddd`, Latn/en/LTR, `test=1`, all five
buffer profiles, both APIs, both flush modes.

**Observed:** `adadada` remains seven glyphs `[1,3,1,3,1,3,1]`, each advancing
600. Expected are four `a` glyphs at clusters `[0,2,4,6]`, with advances
`[580,580,580,600]` after pair positioning. The all-deletion case emits six `d`
glyphs instead of none. Both return nil and consume through EOF. Expansion
`b → b b` passes every configuration, and ligature contraction passes the
whole/lookahead/carry profiles.

**Implementation evidence:** `gsubLookupType2Fmt1` in
[gsub.go](../otlayout/gsub.go) returns “not applied” when the replacement
sequence is empty, bypassing deletion. Consequently the all-deletion test is
installed but cannot yet verify actual zero-output progress. Empty input and
balanced event-only scopes pass separately. HB's exact newline serialization
for all-deleted output is preserved in capture metadata and accepted only for
this designated case; missing/null output is rejected.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE017' -count=1 -timeout=30s -v
```

## SHAPE-018 — Forward lookup scans bypass feature masks

**Status:** reproduced. **Tests:** `TestKnownBugSHAPE018MaskedLookupScan` and
`TestKnownBugSHAPE018AlternateValueAtMatchedGlyph`.

**SHAPE-003 dependency (2026-10-27):** the scan also bypasses value selection.
With GIDs `[17,18]`, global `test=1`, and `test[1:2]=3`, the first uncovered
glyph starts a lookup that reaches GID 18 using value 1. Output is `[17,20]`
instead of the TTX-derived `[17,22]`. The new public-API regression retains
both glyph and original-cluster assertions. Random selection has the same
starting-versus-matched-anchor limitation. The user chose to keep this scan
fix separate from SHAPE-003.

**Expected behavior and basis:** a disabled feature range must prevent a lookup
from starting a substitution within that range, even when an earlier glyph
has the feature enabled. Independent HB captures specify the range behavior.

**Reproduction:** five Gentium `office` words with
`liga=1,liga[0:21]=0,liga[7:14]=1`, whole-input buffers `35/1/36`. The nested
settings enable the second word, disable the third, then restore global enable
for the fourth/fifth. A second case uses `e + U+0301 + office office` with
`liga=1,liga[3:10]=0` and buffers `17/1/18`.

**Observed:** `Shape` produces 26 rather than 28 glyphs in the nested case,
incorrectly forming `ffi` at cluster 15 in the disabled third word. The first
disabled word and enabled second word work. The accented case produces 11
rather than 13 glyphs, also forming a disabled `ffi`. Equivalent event scopes
pass whole/lookahead/carry profiles. The nested whole-input case has neither
normalization nor compaction, isolating this from SHAPE-002's coordinate drift.

**Implementation evidence:** `applyLookupSpanInternal` in
[lookups.go](../otshape/lookups.go) checks the feature mask at `st.Index`, then
calls lower-level `ApplyFeature`. `gsubLookupType4Fmt1` in
[gsub.go](../otlayout/gsub.go) uses `matchCoverageForward` from
[feature.go](../otlayout/feature.go), which can find a later covered glyph
without checking its API feature mask. The enabled starting index therefore
does not guarantee the eventual match is enabled.

**SHAPE-002 promotion (2026-10-27):** the nested-scope `lookahead` and `carry`
rune-input profiles now pass every field in both flush modes and moved to the
ordinary matrix. Correct source coordinates repair those particular buffer
alignments. The whole-input, tiny, and forced nested profiles still fail;
the accented case still fails every rune-input profile. SHAPE-018 remains open.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE018' -count=1 -timeout=30s -v
```

## Remaining discovery work

The shared-API and initial AOTS batches are not a complete defect inventory.
[TESTING.md](TESTING.md#next-testing-priorities) retains the wider priorities
and earlier inspection/review observations. Next shared-API cases include
Latin/Hebrew script and language selection, ranges/scopes inside merged
clusters, additional feature values, and broader buffer sweeps. The initial
streaming/event/progress setup is complete; successful all-deletion progress
remains blocked by SHAPE-017. Whole-input range drift after normalization is
covered by an ordinary SHAPE-002 regression. Add stable findings here as those
cases are investigated; do not silently equate missing coverage with a bug.

## Resolved findings

Resolved entries retain stable IDs and heading anchors for reference. Their
regression tests remain in the ordinary suite; detailed investigation history
is available in Git.

### SHAPE-001 — SetupMasks changes are overwritten

**Fixed:** 2026-10-27.

**Cause and fix:** `planExecutor.apply` initialized masks a second time, erasing
`SetupMasks` decisions. Masks are now initialized once in
[shapeMappedRun](../otshape/shape_api.go) before the hook;
[the executor](../otshape/plan.go) consumes the prepared masks. Preserving them
also exposed nondeterministic engine feature-bit allocation. Engine-only
requests are now sorted by tag, retaining user order/precedence.

**Permanent regressions:**
[TestSHAPE001SetupMasksPreserved](../otshape/otcore/shared_api_test.go) checks
GIDs, clusters, and disabled/enabled masks through both `Shape` and `ShapeEvents`.
[TestEngineFeatureMaskAllocationStable](../otshape/plan_test.go) checks allocation
stability, user precedence, and disabled features. Review confirmed that restoring
the old executor makes both public-API subtests fail; the full ordinary suite,
including substitution-mask inheritance and Arabic streaming parity, passes.

```sh
go test ./otshape/otcore -run '^TestSHAPE001' -count=1 -v
go test ./otshape -run '^TestEngineFeatureMaskAllocationStable$' -count=1 -v
```

### SHAPE-002 — Feature ranges use chunk-local coordinates

**Fixed:** 2026-10-27, within the agreed scope of ranges that do not split merged
source sequences.

**Cause and fix:** `applyFeatureRangesToMasks` indexed current buffer slots, so
compaction or normalization shifted a requested range onto the wrong glyphs.
It now compares unchanged `FeatureRange` bounds against each glyph's retained
original Unicode-scalar anchor in `run.Clusters`. Nonpositive bounds retain their
start/end-of-input meaning. Masks are assigned before `SetupMasks` and inherited
through lookup edits; no ranges are reinterpreted after substitutions. The
input-coordinate contract is explicit in [FeatureRange](../otshape/shaper.go).

**Permanent regressions:**
[TestSHAPE002FeatureRangeUsesInputCoordinates](../otshape/otcore/shared_api_test.go)
checks `[18,18,20,18]` at clusters `[0,1,2,3]` in whole-input and streaming runs.
[TestHarfBuzzTextBaselines/latin_range_after_nfd](../internal/hbcmp/text_baselines_test.go)
now matches the full unchanged HarfBuzz output for `e + acute + office` with
`liga[3:6]=1`: GIDs/clusters `[112/0,82/2,741/3,70/6,72/7]`.
[TestFeatureRangesFollowSourceAnchors](../otshape/plan_test.go) covers compaction,
composition/decomposition correspondence, reordered anchors, and open/empty bounds;
existing tests retain overlap precedence and substitution-mask inheritance.

Eight streaming-matrix cells moved to the ordinary suite: the `lookahead` and
`carry` rune profiles for both `stream_range` and `scope_nested`, in both flush
modes. All compare complete independent output. The tiny/forced `stream_range`
cases still split ligature context (SHAPE-015); their complete assertions now
live in that tagged group. The AOTS streaming alternate case subsequently
passed with SHAPE-003 and was promoted unchanged to
`TestSHAPE003AOTSAlternatesStreaming`; SHAPE-015 remains unresolved.

**Limitations and provisional preference:** ranges splitting a composed character
or merged source cluster are not part of this resolution. The user's tentative
preference is to exclude a combined character when the range starts inside it
and begin with the next eligible character. Anchor comparison naturally does
that for a composite anchored before `Start`, but this is not a settled general
policy for all merged clusters, range ends, or reordering. SHAPE-011 must preserve
source correspondence when introducing broader level-0 grouping.

**Verification at SHAPE-002 completion (2026-10-27):** focused
shaping/lookup/HarfBuzz tests and `go test ./... -count=1` passed. The tagged
suite then had 22 remaining top-level failures, with no unexpected
top-level passes; the matrix has 142 passing and 178 failing executions. Captures
are unchanged and no assertions were weakened.

```sh
go test ./otshape/otcore -run '^TestSHAPE002' -count=1 -v
go test ./internal/hbcmp -run '^TestHarfBuzzTextBaselines/latin_range_after_nfd$' -count=1 -v
```

### SHAPE-003 — Feature values do not select alternates

**Fixed:** implemented 2026-10-27; reviewed and accepted 2026-10-29, with the
separate forward-scan dependency retained under SHAPE-018.

**Agreed contract:** positive values are one-based; zero selects the first/default
alternate; every negative value requests random selection. Out-of-range positive
values leave the glyph unchanged by that alternate substitution, without clamping
or wrapping. Other shaping operations may still change it. This applies equally
to `ApplyFeature`, enabled `FeatureRange.Arg`, and enabled `FeatureSetting.Value`;
`On:false` / `Enabled:false` disables the feature.

**Cause and fix:** public lookup execution ignored per-glyph values; lower-level
selection was zero-based and treated negatives as “last”. Mask fields now encode
references to retained `int` values, avoiding truncation of large requests, and
execution decodes them before applying the lookup. Code zero is off, code one is
default; equivalent default/random requests share codes. Allocation respects the
31-bit mask budget. Event scopes preserve negative values. The lower-level API
implements the same selection contract and retains selected-output validation.

**Random selection:** a fresh request seed is shared by its event plans. Random
lookup attempts seed PCG from that seed, the original scalar anchor, and outer
lookup index; nested lookups share the stream. Rebuilding the same carry repeats
choices. Shared anchors can correlate choices; independent draws per output glyph
are not guaranteed. `BufferState.SetRandomSeed` enables repeatable lower-level
checks; unseeded direct calls use Go's default random source. Tests inject a
fixed request seed internally; there is no public shaper seed option. Randomness
is for typography, not cryptography.

**Permanent regressions:**

- [Public value, overlapping-range, and nested-scope tests](../otshape/otcore/alternate_values_test.go)
  cover both APIs, default/one-based/random/oversized values, clusters, disabling,
  and restoration. The fixture TTX supplies the independent alternate list.
- [Lower-level selection tests](../otlayout/feature_functional_test.go) check the
  same contract and fixed-seed draws, including `MinInt`; existing
  [output-bounds tests](../otlayout/gsub_output_bounds_test.go) still validate
  selected outputs.
- [Carry/scoped-plan and mask-value tests](../otshape/alternate_values_test.go)
  check repeatable choices, inherited/pushed negatives, more than 255 distinct
  requests, and `MaxInt` without wraparound.
- [AOTS regressions](../internal/hbcmp/aots_gsub_test.go) compare the unchanged
  `gsub3_1_simple.tests:2` oracle in whole-input and streaming configurations,
  completing the alternate-value dependency left by SHAPE-002.

**Remaining dependencies:** [SHAPE-018](#shape-018--forward-lookup-scans-bypass-feature-masks)
can select a value or random seed at an attempted start rather than the later
matched glyph. Its full desired-behavior regression remains tagged; carry
equivalence for such scans is not established. The flagged AOTS alternate case
remains blocked by [SHAPE-005](#shape-005--gdef-classification-prevents-ignoremarks).
Neither dependency is claimed fixed here.

**Review verification (2026-10-29):** the full ordinary suite and
`go test -race ./otlayout ./otshape/... ./internal/hbcmp -count=1` pass.
The discovery suite has 20 remaining top-level failures, without build failures,
panics, or timeouts. Restoring the old lower-level selector through a temporary
Go overlay makes the positive-value and fixed-seed random regressions fail.
No captured expectations were changed or weakened.

```sh
go test ./otlayout ./otshape/... ./internal/hbcmp -run 'SHAPE003|ApplyFeatureAlternateValues|AOTSGSUBBaselines|OutputBoundsSelection' -count=1
```
