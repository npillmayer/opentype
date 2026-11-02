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

The remaining shared-API reproduction (SHAPE-004) lives in
[`otshape/otcore/shared_api_knownbugs_test.go`](../otshape/otcore/shared_api_knownbugs_test.go),
in external package `otcore_test`. They exercise exported `NewShaper`/`Shape`
and engine hooks. This keeps the base `otshape` tests independent of concrete
engine imports. The ordinary [`shared_api_test.go`](../otshape/otcore/shared_api_test.go) now
contains SHAPE-001/002/011 and the shared fixture/assertion helpers;
[`alternate_values_test.go`](../otshape/otcore/alternate_values_test.go) contains
the ordinary SHAPE-003/018 value/range/scope regressions. These tests reuse
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

# Discovery suite: twelve failing top-level tests across the two packages.
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1

# Fixed mask regression, covering both public entry points.
go test ./otshape/otcore -run '^TestSHAPE001' -count=1 -v

# Inspect an unresolved shared-API finding.
go test -tags shaping_knownbugs ./otshape/otcore -run '^TestKnownBugSHAPE004' -count=1 -v
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
| [SHAPE-009](#shape-009--rtl-output-order-differs-from-harfbuzz) | fixed (contract resolved) | Logical sink order agreed; plain Hebrew passes after explicit oracle adaptation |
| [SHAPE-014](#shape-014--fonts-without-gpos-stop-at-the-parser) | deferred | Existing parser scope excludes fonts without GPOS |
| [SHAPE-015](#shape-015--streaming-cuts-lose-shaping-context) | reproduced | Buffer cuts change ligatures, pair positioning, and normalization |
| [SHAPE-023](#shape-023--hebrew-combining-marks-use-the-wrong-internal-order) | reproduced | Hebrew mark order differs despite matching glyphs, clusters and per-glyph positions |

Archived fixes in [Resolved findings](#resolved-findings):
[SHAPE-001 — mask preservation](#shape-001--setupmasks-changes-are-overwritten),
[SHAPE-002 — source ranges](#shape-002--feature-ranges-use-chunk-local-coordinates),
[SHAPE-003 — alternate selection](#shape-003--feature-values-do-not-select-alternates),
[SHAPE-005 — GDEF classification](#shape-005--gdef-classification-prevents-ignoremarks),
[SHAPE-006 — contextual continuation](#shape-006--contextual-lookup-revisits-matched-input),
[SHAPE-007 — final mark positioning](#shape-007--mark-attachments-do-not-resolve-to-origins),
[SHAPE-008 — GDEF mark advances](#shape-008--marks-retain-nominal-advances),
[SHAPE-010 — normalization mark support](#shape-010--default-mark-positioning-is-missed-during-normalization),
[SHAPE-011 — base/mark clusters](#shape-011--combining-mark-clusters-differ-from-harfbuzz-level-0),
[SHAPE-012 — canonical normalization](#shape-012--canonical-equivalent-mark-orders-shape-differently),
[SHAPE-013 — font-aware normalization](#shape-013--normalization-ignores-font-coverage),
[SHAPE-016 — shared feature-scope context](#shape-016--feature-scopes-sever-unrelated-lookup-context),
[SHAPE-017 — GSUB deletion](#shape-017--gsub-deletion-is-ignored),
[SHAPE-018 — forward lookup starts](#shape-018--forward-lookup-scans-bypass-feature-masks),
[SHAPE-019 — ligature mark preservation](#shape-019--ligature-substitution-deletes-ignored-marks),
[SHAPE-020 — stage lookup order](#shape-020--default-features-run-in-feature-order-instead-of-lookup-order),
[SHAPE-021 — engine/user feature precedence](#shape-021--engine-and-user-feature-settings-combine-as-off-wins),
and [SHAPE-022 — mark-to-mark predecessor selection](#shape-022--mark-to-mark-attachment-crosses-an-intervening-base).

The original AOTS baseline batch added SHAPE-005/006 in the former
`internal/hbcmp/aots_knownbugs_test.go` (removed after the SHAPE-019 promotion).
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
features: both default fallback and explicit requests for those absent features
now pass after SHAPE-010. These in-scope regressions are independent of the
broader font-acceptance limitation.

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

**SHAPE-017 update (2026-11-01):** deletion now works. Sixteen original
mixed/all-deletion tiny/forced comparisons, plus eight from the new leading/
trailing-deletion capture, retain their complete expectations here. Mixed
outputs lose kerning or cluster context at cuts; all-deleted carry at capacity
returns the agreed `ErrZeroOutputBufferFull` instead of unsafe progress. This
error is separately tested as a valid conservative limit, not reported as a new
lookup defect. Reclassification of unsupported profiles belongs to the broader
buffer contract; whole/lookahead/carry deletion comparisons are ordinary.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE015' -count=1 -timeout=30s -v
```

## SHAPE-023 — Hebrew combining marks use the wrong internal order

**Status:** reproduced; numbered 2026-11-02 from the existing Hebrew mark-ordering
follow-up. No shaping behavior or expected output changed. **Test:**
`TestKnownBugSHAPE023HebrewMarkOrdering` in
[text_knownbugs_test.go](../internal/hbcmp/text_knownbugs_test.go).

**Expected behavior and basis:** within each initial Hebrew cluster, logical
shaping order should match the unchanged HarfBuzz level-0 oracle after the
agreed reversal of HarfBuzz's final visual output. This is internal mark order,
not paragraph bidi or a reason to change the logical sink contract (SHAPE-009).

**Reproductions:** Noto Sans Hebrew, Hebr/he/RTL, default features, whole-input
buffering, using the complete existing records in `text/baselines.json`:

- `hebrew_shin_pointed`: U+05E9 U+05B8 U+05C1 (shin, qamats, shin dot).
  All three glyphs have cluster 0 and correct per-glyph positions, but their
  order is `[96,79,100]` instead of `[96,100,79]`.
- `hebrew_shalom_pointed`: U+05E9 U+05B8 U+05C1 U+05DC U+05D5 U+05B9 U+05DD.
  The same initial shin-cluster ordering difference remains. Its seven-glyph
  count, vav/holam composition, clusters and per-glyph positions already match.

**Attribution:** these are the former `TestKnownBugSHAPE007HebrewMarkOffsets`
and `TestKnownBugSHAPE010HebrewComposition/hebrew_shalom_pointed` regressions,
previously assigned to the unnumbered SHAPE-011 follow-up. They now form two
subtests under SHAPE-023. The complete six-field assertions and captured records
are unchanged. Combining the wrappers reduces the number of failing top-level
test groups by one; it does not fix either reproduction.

**Scope:** investigate the Hebrew engine's mark-ordering policy and its interaction
with normalization. SHAPE-012/013 explicitly preserved the existing Hebrew order
pending this separate work; simply enabling generic canonical sorting is not an
agreed fix. Specify the correction before implementing it. SHAPE-007/010/011 and
SHAPE-022 stay resolved within their accepted scopes. Streaming attachment/context
loss remains SHAPE-015; broader attachment compatibility is separate.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE023' -count=1 -v
```

## Remaining discovery work

The shared-API and initial AOTS batches are not a complete defect inventory.
[TESTING.md](TESTING.md#next-testing-priorities) retains the wider priorities
and earlier inspection/review observations. Next shared-API cases include
Latin/Hebrew script and language selection, ranges/scopes inside merged
clusters, additional feature values, and broader buffer sweeps. The initial
streaming/event/progress setup is complete; SHAPE-017 now covers successful
all-deletion at EOF and explicit zero-output capacity errors. Whole-input range drift after normalization is
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
[the executor](../otshape/plan_exec.go) consumes the prepared masks. Preserving them
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
input-coordinate contract is explicit in [FeatureRange](../otshape/params.go).

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
policy for all merged clusters, range ends, or reordering. SHAPE-011 subsequently
introduced base/mark grouping while preserving separate source correspondence.

**Subsequent SHAPE-011 decision (2026-10-29):** initial base/mark groups now
use their starting scalar for both bounds: starts inside exclude the group;
ends inside include it. Source anchors remain separate from output grouping.
This supersedes the provisional preference above for those groups only;
SHAPE-016 subsequently applied the same rule across feature events within a
carry. Hebrew reordering remains separate.

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

**Subsequent dependency update (2026-10-29):** the SHAPE-018 forward-start fix
now applies the checked value and random anchor at the actual starting glyph;
its alternate regression was promoted unchanged. SHAPE-003 itself is not
reopened. The flagged AOTS alternate case also passes after the reviewed
[SHAPE-005 classification fix](#shape-005--gdef-classification-prevents-ignoremarks)
and is an ordinary `TestAOTSGSUBBaselines` case.

**Review verification (2026-10-29):** the full ordinary suite and
`go test -race ./otlayout ./otshape/... ./internal/hbcmp -count=1` pass.
The discovery suite has 20 remaining top-level failures, without build failures,
panics, or timeouts. Restoring the old lower-level selector through a temporary
Go overlay makes the positive-value and fixed-seed random regressions fail.
No captured expectations were changed or weakened.

```sh
go test ./otlayout ./otshape/... ./internal/hbcmp -run 'SHAPE003|ApplyFeatureAlternateValues|AOTSGSUBBaselines|OutputBoundsSelection' -count=1
```

### SHAPE-005 — GDEF classification prevents IgnoreMarks

**Fixed:** implemented and reviewed 2026-10-29. No blocking code defects found.

**Cause and fix:** [ClassDef format 2](../ot/layout.go) excluded the final glyph
of each range, including every singleton. Its comparison now includes both
endpoints. [GDEF class constants](../ot/layout_gdef.go) now match the font
encoding: base=1, ligature=2, mark=3, component=4; zero is unclassified.
The exported names are unchanged. These corrections restore IgnoreMarks and
IgnoreBaseGlyphs behavior and apply to the shared GSUB/GPOS class-table reader.
See the [ClassDef specification](https://learn.microsoft.com/en-us/typography/opentype/spec/chapter2#class-definition-table)
and [GDEF enumeration](https://learn.microsoft.com/en-us/typography/opentype/spec/gdef#glyph-class-definition-table).

**Permanent regressions:**

- [Class-definition boundary tests](../ot/class_definition_test.go) cover singleton,
  multi-glyph and adjacent ranges, gaps, glyph zero, and glyph 65535.
- [GDEF classification tests](../internal/hbcmp/gdef_classification_test.go) check
  real-font singleton marks 90/91/92 and independent class-number expectations.
- [GSUB baselines](../internal/hbcmp/aots_gsub_test.go) now include
  `gsub_context1_lookupflag.tests:2` (interleaved IgnoreMarks) and
  `gsub3_1_lookupflag.tests:2` (IgnoreBaseGlyphs and alternate values).
  [GPOS baselines](../internal/hbcmp/aots_gpos_test.go) include
  `gpos2_1_lookupflag.tests:3` (pair advances across an ignored base).
  All three preserve their complete upstream expectations.
- [Cursive metadata tests](../otshape/otcore/gpos_test.go) were corrected using
  binary GDEF and checked-in GPOS TTX: unclassified 18/19 can attach, the
  mixed-anchor fixture leaves 20 unattached, and actual base glyph 21 is skipped.
  These are attachment-metadata tests, not final-positioning baselines.

**Subsequent fixes:** SHAPE-007/008 resolved positioning for surviving marks
within their agreed scope. [SHAPE-019](#shape-019--ligature-substitution-deletes-ignored-marks)
then resolved ignored-mark preservation and component association. The full
upstream oracle now passes in `TestAOTSGPOSBaselines/gpos5.tests:2`; the historical
`TestKnownBugSHAPE005AOTSLigatureAcrossMark` wrapper was removed.

**Review verification (2026-10-29):** `go test ./... -count=1` and
`go test -race ./... -count=1` passed. Reverting either classification correction
separately through temporary Go overlays made the relevant regressions fail.
Discovery retained fifteen failing top-level tests, including the deferred parser
case, without build failures, panics, or timeouts. No fonts or captures changed.

```sh
go test ./ot ./otshape/otcore ./internal/hbcmp -count=1
```

### SHAPE-006 — Contextual lookup revisits matched input

**Fixed:** implemented and reviewed 2026-10-29. No blocking defects found within
the documented forward-contextual scope.

**Cause and fix:** contextual wrappers returned their starting cursor after
successful nested application, so the shaping loop advanced only one glyph and
revisited matched input. All twelve forward paths (GSUB 5/6 and GPOS 7/8,
formats 1–3) now return the exclusive end of the matched input.
[The sequence-record helper](../otlayout/feature.go) tracks that boundary
separately from nested input identities and adjusts it through each glyph edit.
Expansion/contraction shift the boundary; a replacement crossing it moves the
boundary after the replacement. Backtrack/lookahead are excluded. Ignored glyphs
between inputs count toward the span; trailing ignored glyphs do not merely
because lookahead skipped them. Existing partial changes and diagnostic cursors
on error remain intact. See [PARTIAL_PROGRESS.md](PARTIAL_PROGRESS.md#successful-contextual-continuation).

The approved dependency correction makes `matchClassSequence` use its configured
offset: zero forward, minus one backward. Forward class matching had skipped the
first candidate by using the direction instead.

**Permanent regressions:**

- [AOTS GSUB baselines](../internal/hbcmp/aots_gsub_test.go) now include unchanged
  upstream [`gsub_context1_next_glyph.tests:2`](../testdata/harfbuzz/aots/tests/gsub_context1_next_glyph.tests).
  The expected `[0,60,20,60,20,20,0]` replaces the former overlapping result
  `[0,60,60,60,60,20,0]`. No fixture or oracle was regenerated.
- [Lookup progression tests](../otlayout/context_progression_test.go) cover all
  twelve paths, ignored marks, successful/nonapplying children, nested expansion,
  final-input expansion, contraction within/across the endpoint, zero-net-length
  edits, and later failure after completed edits. The class-sequence regression
  checks both directions and a nonmatching first class without scanning ahead.
- [Shaping progression tests](../otshape/context_progression_test.go) check
  non-overlapping substitution, expansion, and positioning in whole-run and
  isolated-syllable execution, including clusters and positions.

**Review verification (2026-10-29):** full ordinary and race suites passed.
Restoring the old continuation through a temporary Go overlay made lookup,
shaping, and upstream regressions fail. Independently restoring the old class
offset made format-2 and class-sequence regressions fail. Discovery retained
fourteen failing top-level tests, without build failures, panics, or timeouts.

**Remaining scope:** empty-record and nonapplying-child rules retain their prior
behavior; a value-preserving substitution still counts as application.
Intentional deletion was subsequently resolved under SHAPE-017, and ignored-mark
preservation/component association under SHAPE-019. Reverse traversal and
streaming context safety remain separate. This resolution does not establish complete contextual shaping
conformance.

```sh
go test ./internal/hbcmp -run '^TestAOTSGSUBBaselines/gsub_context1_next_glyph' -count=1 -v
go test ./otlayout ./otshape -run 'TestContextProgression|TestClassSequenceStarts' -count=1
```

### SHAPE-007 — Mark attachments do not resolve to origins

**Fixed:** implemented, reviewed, and accepted 2026-10-29 within the agreed
horizontal Latin/Hebrew scope, including the review's overflow correction.

**Cause and fix:** GPOS mark lookups recorded attachment references without
resolving anchor coordinates or final pen positions. GPOS 4/5/6 now install the
selected anchor difference, replacing earlier mark placement; later value
records still add to it. Target Y placement is captured at attachment time using
`int64` arithmetic and saturation to `int32`, preventing overflow before storage.
After GPOS and advance policies, [finalization](../otshape/mark_attachments.go)
adds the target's final X placement and compensates for intervening final
advances before `PostprocessRun`. Logical sink order is preserved, including
RTL's different pen interval. This follows
[HarfBuzz attachment application](https://github.com/harfbuzz/harfbuzz/blob/main/src/OT/Layout/GPOS/MarkArray.hh)
and [HarfBuzz 15 offset finalization](https://github.com/harfbuzz/harfbuzz/blob/15.0.0/src/OT/Layout/GPOS/GPOS.hh).

A forward pass and prefix sums resolve earlier-target chains in linear time.
Invalid pending targets fail before positions are mutated; finalization clears
pending state but retains attachment diagnostics and is idempotent. Lower-level
`ApplyFeature` retains anchor placement and pending state, including on later
failure; final nominal metrics and pen compensation remain the shaper's job.

**Permanent regressions:**

- [AOTS GPOS baselines](../internal/hbcmp/aots_gpos_test.go) include unchanged
  `gpos4_simple.tests:2` and `gpos5.tests:3`. Mark origins now match `(1400,-80)`
  and `(1401,-79)`, respectively. See [GPOS.md](../testdata/harfbuzz/aots/GPOS.md)
  for complete sequences and independent provenance. No fonts or oracles changed.
- [Lookup attachment tests](../otlayout/mark_attachments_test.go) cover GPOS
  4/5/6, replacement, subsequent placement, nonmatches, partial failure, edit
  remapping, and Y-offset saturation in both directions, including exact limits
  and nearby values.
- [Finalization tests](../otshape/mark_attachments_test.go) cover chains, lookup
  ordering, final advances, LTR/RTL compensation, long chains, invalid targets,
  and idempotence. [Public-API tests](../otshape/otcore/mark_attachments_test.go)
  check complete positions through both entry points in both directions.

**Verification (2026-10-29):** ordinary and race suites passed during review;
removing finalization through a temporary overlay made public-API and upstream
regressions fail. Both overflow regressions failed before the arithmetic
correction and pass afterwards. Focused attachment tests and the full ordinary
suite passed after that correction. Discovery retained twelve failing top-level
tests (eleven shaping groups and the deferred parser case).

**Remaining scope:** the historical pointed-shin regression now remains tagged
in `TestKnownBugSHAPE023HebrewMarkOrdering` for the [SHAPE-023 Hebrew mark ordering](#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).
Noto Sans Hebrew mark offsets are exactly 227 and 539. SHAPE-011 subsequently
fixed grouping: output is now `[96/0,79/0,100/0]` instead of
`[96/0,100/0,79/0]`. Its four complete combining-mark text cases now pass unchanged
in the ordinary suite. Effective mark support was subsequently fixed under
SHAPE-010; SHAPE-012/013 subsequently resolved canonical ordering and font-aware
representation within their agreed scope. Hebrew ordering remains separate.
SHAPE-019 subsequently
resolved ignored-mark preservation and component association, including nested
ligatures; unassociated marks retain the last-component fallback. Cursive
resolution, hinted anchor points, device/variation adjustments, and
streaming context (SHAPE-015) remains outside this fix. SHAPE-016 subsequently
removed event barriers within each buffered carry.

### SHAPE-008 — Marks retain nominal advances

**Fixed:** implemented and reviewed 2026-10-29 within the agreed Latin/Hebrew
scope. No blocking defects found.

**Cause and fix:** runs store positioning adjustments; output materialization
adds nominal horizontal metrics. Zeroing an internal adjustment alone therefore
leaves the nominal mark advance intact. Public Latin/Hebrew plans whose engine
permits GPOS now enable `ZeroGDEFMarks`. After GPOS,
[the position policy](../otshape/position_policies.go) sets each GDEF mark's horizontal
adjustment to minus its nominal metric and its vertical adjustment to zero.
Materialization then yields zero final advances, including for unattached marks.
The current output path adds no nominal vertical metric; this is not vertical
shaping support.

The policy replaces existing advance adjustments, preserves offsets and all
attachment metadata, and is idempotent. It requires a GDEF glyph-class table;
Unicode category or attachment metadata alone does not opt a glyph in.
Other scripts and GPOS-disabled requests retain their previous policy. The older
internal `ZeroMarks`/fallback-positioning mechanism was unchanged by this fix;
it was removed as unused on 2026-11-01.

**Permanent regressions:**

- [AOTS GPOS baselines](../internal/hbcmp/aots_gpos_test.go) now include unchanged
  [`gpos4_simple.tests:3` and `:4`](../testdata/harfbuzz/aots/tests/gpos4_simple.tests).
  These unattached-mark controls place both the mark and following glyph at
  `(3000,0)`; previously the following glyph started at `(4500,0)`. They isolate
  advance cancellation from SHAPE-007. No fonts, hashes, or captures changed.
- [Internal policy tests](../otshape/gdef_mark_advances_test.go) cover script/GPOS
  scope, missing classification, positive/negative adjustments, nonmark controls,
  offsets/attachments, repeated execution, and materialized output.
- [Public-API tests](../otshape/otcore/mark_advances_test.go) cover both entry
  points, Latin attached/unattached marks, Hebrew policy selection, and an
  unchanged out-of-scope script control. The synthetic Hebrew fixture does not
  establish script-specific attachment or full Hebrew conformance.
- [The GPOS-single public test](../otshape/otcore/gpos_test.go) now expects zero
  final advances for GDEF marks while retaining independently decoded placement
  expectations. Lower-level GPOS value-record behavior is unchanged.

**Review verification (2026-10-29):** full ordinary and race suites passed.
Disabling the cancellation through a temporary Go overlay made the internal,
public-API, and upstream regressions fail. Discovery retained thirteen failing
top-level tests, without build failures, panics, or timeouts.

**Remaining scope:** SHAPE-007 subsequently resolved anchor offsets within the
agreed scope; both attached-mark AOTS records now pass unchanged in the ordinary
suite. SHAPE-019 subsequently resolved intervening-mark preservation and
component association. Missing usable GDEF, Unicode fallback, other scripts,
normalization/cluster differences, and
safe streaming boundaries remain separate.

```sh
go test ./internal/hbcmp -run '^TestAOTSGPOSBaselines/gpos4_simple' -count=1 -v
go test ./otshape ./otshape/otcore -run '^TestGDEFMarkAdvance' -count=1
```

### SHAPE-010 — Default mark positioning is missed during normalization

**Status:** fixed; accepted 2026-11-01. The three standalone vav/holam
regressions now run in the ordinary suite. The historical pointed-shalom
regression now runs in `TestKnownBugSHAPE023HebrewMarkOrdering`
for the separate [SHAPE-023 Hebrew mark ordering](#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).

**Cause:** normalization inferred GPOS mark support from allocated user-feature
masks. It missed unmasked default features and treated disabled or absent
requested features as enabled support. Consequently, Hebrew vav + holam
incorrectly composed to presentation-form U+FB4B with default features, while
explicitly disabled or absent mark features incorrectly suppressed fallback.
The Hebrew engine's fallback policy was correct; the public pipeline supplied
the wrong `NormalizeContext.HasGposMark` signal.

**Fix and agreed contract:** `planHasGposMark` inspects resolved
GPOS lookup contributors, including nonrepresentative contributors of a shared
lookup (SHAPE-020). GPOS must be enabled by the plan policy. Unmasked default
and required `mark` contributors count; masked contributors require an enabled
global value or ranged enablement. Mask allocation, an empty feature binding,
an absent requested feature, and `mkmk` alone do not count.

This is a plan-wide normalization decision: an enabled ranged `mark` lookup
suppresses Hebrew presentation-form fallback throughout that plan, while
positioning still obeys the range masks. Independent HarfBuzz 15 captures
confirm both mixed `mark`/`mkmk` settings and ranged on/off behavior. It is not
a per-glyph coverage guarantee and does not predict later `SetupMasks` hooks.
Engine/user precedence was subsequently resolved under SHAPE-021. SHAPE-016 subsequently gives
event scopes one combined normalization plan per carry. No public interface
signature changed in SHAPE-010.

**Regressions and remaining limitations:** `hebrew_vav_holam`,
`hebrew_vav_holam_mark_off`, and `hebrew_vav_holam_no_mark_requested` are promoted
with unchanged complete HarfBuzz expectations. Four additional captures cover
mark-only, mkmk-only, ranged enablement and ranged disablement. Both APIs cover
the seven global-setting cases; the two indexed-range cases use `Shape`.
`TestNormalizationMarkSupport` and `TestNormalizationMarkContributorEnablement`
cover required/default features, empty/absent features, shared lookup provenance,
contributor masks, and disabled GPOS.

Pointed shalom now has all seven expected glyphs with matching clusters and
per-glyph positions. Only the two marks on its initial shin remain reversed;
the full oracle stays tagged for the
[SHAPE-023 Hebrew mark ordering](#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).
SHAPE-007/011 remain resolved within their accepted scopes. Canonical ordering
and cmap-aware representation were subsequently resolved under SHAPE-012/013,
with Hebrew ordering explicitly excluded. Absence of the entire GPOS
table is separately blocked at the parser and deferred under SHAPE-014.

**Recorded implementation verification (2026-11-01):** `go test ./... -count=1`, `go build ./...`, and `go vet ./...`
passed. Read-only recapture verified all 35 independent HarfBuzz records.
Tagged discovery still has
ten failing top-level tests, with precisely the three promoted subtests removed
and no new failures, fixture failures, panics, or timeouts. All 31 previous
capture records and capture provenance remain unchanged; the corpus now has
35 cases (29 ordinary, six tagged).

```sh
go test ./otshape -run '^TestNormalizationMark' -count=1
go test ./internal/hbcmp -run 'TestSHAPE010EventBaselines|TestHarfBuzzTextBaselines' -count=1
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE010' -count=1 -v
```

### SHAPE-011 — Combining-mark clusters differ from HarfBuzz level 0

**Fixed:** implemented and reviewed 2026-10-29 within the agreed Latin/Hebrew
base/mark grouping scope. No blocking findings; Hebrew internal mark ordering
and context-safe streaming remain separate.

**Cause and fix:** base letters and combining marks retained separate scalar
clusters. [Initial grouping](../otshape/clusters.go) now forms groups before
normalization using Unicode mark categories and ZWJ continuation. Leading marks
form a leading group; ZWNJ remains a boundary. Normalized source anchors map
back to these groups. [Cluster merging](../otshape/clusters.go) includes adjacent
surviving members of the boundary groups, preserving association when a base
is consumed into a ligature. The contract remains HarfBuzz level-0 grouping,
anchored to zero-based original Unicode scalar indexes, with logical sink order.

`runBuffer.SourceAnchors` preserves per-glyph provenance separately through
normalization, mapping, edits, copies, reordering, and event-span append.
Composition and many-to-one substitution retain the minimum consumed source
anchor. Random alternates use this provenance; indexed masks use cluster starts.
The associated normalization correction copies one-rune composition output into
scratch storage, preventing a later event span from overwriting raw input.

**Range decision (2026-10-29):** select an initial base/mark group by its starting
scalar. For `x + acute + y`, clusters are `[0,0,2]`: `[1,3)` selects only `y`,
`[0,1)` selects the whole first group, and `[1,2)` selects neither. Later
substitutions retain existing mask inheritance. SHAPE-016 subsequently applies
the same rule to feature-event boundaries inside a group.

**Permanent regressions:**

- [Text baselines](../internal/hbcmp/text_baselines_test.go) include unchanged
  `latin_marks_canonical`, `latin_uncomposable_mark`, `hebrew_vav_holam_mark_on`,
  and `hebrew_shin_reordered` captures. The corpus has 15 ordinary / 9 tagged
  cases. [The streaming matrix](../internal/hbcmp/streaming_baselines_test.go)
  promotes 32 mark cells unchanged, bringing its totals to 182 ordinary / 138
  tagged; eight tiny-buffer mark cells remain under SHAPE-015.
- [Cluster tests](../otshape/clusters_test.go) cover leading marks, normalization
  provenance, range bounds, surviving marks after ligature edits, metadata
  copies/reordering/reuse, and event-input scratch isolation.
- [Public range tests](../otshape/otcore/shared_api_test.go)
  (`TestSHAPE011FeatureRangesSelectInitialClusters`) verify starts and ends
  inside groups. [Random-alternate tests](../otshape/alternate_values_test.go)
  (`TestSHAPE011GroupingPreservesRandomSourceAnchors`) verify that grouping does
  not replace per-glyph source anchors in random selection.

**Review verification (2026-10-29):** full ordinary and race suites passed.
Discovery retained ten failing top-level groups (nine shaping groups and the
no-GPOS parser case). Read-only verification against installed HarfBuzz 15.0.0
confirmed all 24 stored text captures. Independently disabling grouping, cluster
merging, source-anchor random selection, or scratch isolation through temporary
Go overlays triggered their corresponding regressions. No oracle data or
assertion fields were changed.

**Remaining scope:** the complete historical pointed-shin assertion stays tagged
for the [SHAPE-023 Hebrew mark ordering](#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).
Effective Hebrew mark support was subsequently fixed under SHAPE-010, and
canonical ordering/font-aware representation under SHAPE-012/013 within their
agreed scope. Hebrew ordering and streaming/limits (SHAPE-015) remain open;
SHAPE-016 subsequently removes feature-event barriers within a carry. General emoji and other
script-specific sequence rules are not claimed. Conservative partial compaction
still rejects repeated output clusters; separate source anchors do not prove
that a prefix is safe to emit.

### SHAPE-012 — Canonical-equivalent mark orders shape differently

**Status:** fixed; accepted 2026-11-01 jointly with SHAPE-013. **Regressions:**
`TestHarfBuzzTextBaselines/latin_marks_reordered` and
`TestSHAPE012013NormalizationBaselines`.

**Expected behavior and basis:** independent HarfBuzz captures of Gentium
`a + U+0323 + U+0301` and `a + U+0301 + U+0323` yield the same GIDs
`[277,2173]`, clusters `[0,0]`, advances `[940,0]`, and X offsets `[0,83]`.
Both inputs are Latn/en/LTR with default features and buffers `4/1/5`.

**Cause:** adjacent-pair composition ran before canonical ordering, so
noncanonical input produced different glyphs. The canonical case already passed
after SHAPE-007/011; the noncanonical case required this normalization fix.

**Fix (2026-11-01):** jointly authorized with SHAPE-013. Normalization now
chooses supported decompositions before ordering, stably sorts nonzero canonical
combining classes, then recomposes against the starter when intervening marks do
not block it. Precomposed accents participate in ordering. Decomposition copies
original source anchors, sorting moves them with runes, and composition retains
the earliest consumed anchor. Initial level-0 groups still select feature ranges.
Supported standalone characters retain their spelling in composed mode.

The original HarfBuzz capture is unchanged and promoted. Fourteen additional
captures in [normalization.json](../internal/hbcmp/testdata/text/normalization.json)
cover hidden accents, equivalent orders, blocking/equal classes, font coverage,
and original-index feature ranges.
Non-ranged captures run through both public APIs with whole-input buffers.
Unit tests also cover leading marks, CGJ barriers, no silent CGJ insertion,
normalization modes, source anchors, and unavailable custom-hook results.

**Agreed boundary:** applying Unicode CCC sorting to Hebrew regressed the
accepted shin/dot/qamats case. The user explicitly chose to preserve Hebrew's
existing order: Hebrew bypasses the new ordering pass, while sharing the
font-coverage checks. Existing Hebrew ordering failures retain their full tagged
oracles. SHAPE-016 subsequently removed event-span barriers within each buffered
carry. Script-specific modified classes and streaming safety (SHAPE-015) remain
separate; this is not full HarfBuzz normalization conformance. No sequence limits or error semantics were introduced.

**Recorded implementation verification (2026-11-01), covering both fixes:**
the full ordinary suite, focused shaping/layout/HarfBuzz race tests,
`go build ./...`, and `go vet ./...` passed. Tagged discovery retains seven expected
failing top-level groups and no unexpectedly passing known-bug groups. Read-only
preparation verified all five normalization fonts, the unchanged 35 text captures,
and the fourteen additional captures. The 35-record corpus now has 32 ordinary
and three tagged cases; the new corpus adds 26 whole-input comparisons.

```sh
go test ./otshape -run 'TestCanonical|TestNormalization|TestClusterNormalization' -count=1
go test ./internal/hbcmp -run 'TestSHAPE012013|TestHarfBuzzTextBaselines/latin_marks_' -count=1
```

### SHAPE-013 — Normalization ignores font coverage

**Status:** fixed; accepted 2026-11-01 jointly with SHAPE-012. **Regressions:**
`TestHarfBuzzTextBaselines/latin_missing_composite_nfc` and
`latin_missing_composite_nfd`, plus `TestSHAPE012013NormalizationBaselines`.

**Reproduction:** the derived `ShaperTestLatin-NoEacute.ttf` removes only
U+00E9's Unicode cmap entries, retaining `e`, U+0301, glyph outlines, and layout
features. Input is U+00E9 + `X` or `e` + U+0301 + `X`, Latn/fr/LTR, defaults,
whole-input buffers `3/1/4` or `4/1/5`. Hashes, derivation and licensing are in
[font provenance](../testdata/fonts/NORMALIZATION.md).

**Expected behavior and basis:** HarfBuzz emits GIDs `[72,2173,59]`, advances
`[946,0,1260]`, X offsets `[0,107,0]`. Clusters are `[0,0,1]` for NFC and
`[0,0,2]` for NFD, preserving the following `X`'s original input position.

**Cause:** normalization neither decomposed an unavailable input composite nor
checked cmap before composing available components. Both NFC and NFD inputs
therefore emitted a missing glyph despite available component mappings.

**Fix (2026-11-01):** normalization checks cmap before accepting either a
Unicode or custom-hook composition. A missing composite decomposes into available
components; if full decomposition would introduce a missing glyph, a supported
NFC nucleus plus supported remaining marks is used, or the original character is
retained. Composed/decomposed modes are representation preferences, not promises
of unconditional Unicode NFC/NFD. `NormalizationNone` still bypasses the work.
Cmap errors make a replacement unavailable; final mapping retains its existing
error handling.

The original NFC/NFD captures pass all six output fields unchanged and are
promoted. New derived fonts remove U+1EA1, or U+030A together with U+01FA,
respectively. Independent HarfBuzz captures verify recomposition across an
unconsumed lower-class mark and partial decomposition (`Ǻ` to `Å` + acute)
without introducing missing glyphs. All previous fonts and captures are
unchanged. Provenance and preparation commands are in
[normalization fonts](../testdata/fonts/NORMALIZATION.md) and the
[normalization report](../internal/hbcmp/testdata/text/NORMALIZATION.md).

The three-pass structure follows HarfBuzz 15's normalizer, using the existing
`x/text/unicode/norm` data and project hooks. It does not import HarfBuzz's
script-specific overrides, variation-selector/space fallbacks, or a general
font-wide representation search. Hebrew ordering and buffer/context limitations
remain as described under [SHAPE-012](#shape-012--canonical-equivalent-mark-orders-shape-differently),
which also records the combined implementation verification.

```sh
go test ./internal/hbcmp -run 'TestSHAPE012013|TestHarfBuzzTextBaselines/latin_missing_composite' -count=1
```

### SHAPE-016 — Feature scopes sever unrelated lookup context

**Status:** fixed; accepted 2026-11-01 within the buffered-carry scope.
Safe streaming cuts remain SHAPE-015. SHAPE-022 subsequently fixed the
mark-to-mark predecessor-selection defect; broader compatibility remains separate.

**Expected behavior and basis:** a `liga` scope must not disable an otherwise
enabled `kern` relationship across its boundary. Event settings are independently
checked against the equivalent HarfBuzz scalar ranges before shaping.

**Original reproduction:** Gentium `AVATAR`, Latn/en/LTR, global `kern=1,liga=1`;
push `liga=0` at scalar 1 and pop at 2. Whole-input buffers `7/1/8` rule out
streaming cuts. HarfBuzz receives `kern=1,liga=1,liga[1:2]=0`.
Previously the first A advance was 1219 instead of 1059 and V was 1331 instead
of 1130: `shapeEventCarry` normalized and shaped each plan-ID span separately.

**Agreed fix:** retain immutable scope assignments per raw scalar, convert the
current carry's scopes to original-scalar ranges, and compile a common plan and
mask layout. Normalize/map/shape the carry once, using an engine instance
initialized for that combined plan. Run hooks see the whole carry, and no
span append/rebasing is needed. Push/pop does not itself compile or shape.
Implicit root defaults are restored after pop; request random seeds survive
combined-plan rebuilding. The existing mask budget applies to the combined
plan. Empty scopes have no glyph effect. Scope-ID limits and error propagation
remain in place; combined-plan failures suppress the current chunk.

Events inside initial base/mark groups use the existing group-start rule:
settings at the initial group's first original scalar govern the whole group.
A scope beginning inside the group excludes it; one ending inside includes it.

**Authorized supporting fix:** lookup masks now constrain actual matched inputs,
not only their start. Otherwise `fi` with `liga[1:2]=0` still ligated and `AV`
with `kern[1:2]=0` still kerned. GSUB ligatures, GSUB/GPOS contextual inputs,
pair positioning, and mark attachment targets use the input mask. Backtrack
and lookahead remain readable regardless of masks; ignored glyphs still follow
lookup flags. Owned lower-level masks track nested substitutions, deletion,
and ignored ligature survivors; outer metadata is replayed once as before.
Unconfigured direct `ApplyFeature` calls retain unrestricted input matching.
Cursive partner selection and reverse traversal redesign are outside this fix.

**Regression coverage:** six original `scope_pair` event comparisons (whole/lookahead/
carry, both flush modes) are promoted unchanged. The four tiny/forced event
comparisons retain their complete oracle under SHAPE-015. The original matrix
now has 224 ordinary and 116 tagged comparisons. Eleven new independent
HarfBuzz 15 captures initially added forty passing comparisons and four tagged
SHAPE-022 comparisons. All 44 now pass after SHAPE-022; see [SCOPES.md](../internal/hbcmp/testdata/text/SCOPES.md).
Two existing normalization/range captures now also pass through `ShapeEvents`.
Lower tests cover all twelve contextual format paths, ignored marks, mask
inheritance through nested edits and clones, shared engine hooks, original
coordinates after compaction, stable random choices across combined carry rebuilds, and combined-plan validation
failure.

**Remaining limitations:** SHAPE-015 safe flushing/capacity remains open.
SHAPE-021 subsequently resolved engine/user precedence. The new mark-range capture exposed the independent
SHAPE-022 predecessor-selection defect, subsequently fixed with its complete
expectation promoted unchanged. Broader attachment compatibility, Hebrew
internal ordering and other script-specific work stay deferred.

**Recorded implementation verification (2026-11-01):** the full ordinary suite,
targeted layout/shaping/HarfBuzz race tests, build, and vet passed. Discovery
retains seven failing top-level groups: SHAPE-016 is removed
and SHAPE-022 is added; no unexpectedly passing known-bug groups remain.

```sh
go test ./internal/hbcmp -run 'TestSHAPE016|TestHarfBuzzStreamingBaselines/scope_pair/' -count=1
go test ./otlayout ./otshape -count=1
```

### SHAPE-017 — GSUB deletion is ignored

**Status:** fixed; accepted 2026-11-01. Passing comparisons run in
`TestHarfBuzzStreamingBaselines`; the former `TestKnownBugSHAPE017GSUBDeletion`
wrapper is removed. Remaining streaming comparisons belong to SHAPE-015.

**Cause and fix:** GSUB type 2 treated an empty replacement sequence as “not
applied.” Empty sequences now delete through `ReplaceGlyphs` and its edit
journal, returning successful application at the deletion index so the shifted
next glyph is visited. Contextual input maps skip deleted targets and retain the
edited continuation; positions and side arrays follow the edit. Deleting the
last glyph of a cluster merges its cluster into a survivor, backward when
available and forward at the run start, matching
[HarfBuzz 15's deletion rules](https://github.com/harfbuzz/harfbuzz/blob/15.0.0/src/hb-buffer.cc#L621).
Survivors keep their own source anchors, masks, and other metadata.

**Agreed zero-output contract (2026-11-01):** both public APIs consume an
all-deleted carry only when EOF is known. Before EOF they retain raw context,
read up to `MaxBuffer`, and reshape. If the raw buffer fills with no output and
EOF remains unknown, they return `otshape.ErrZeroOutputBufferFull`. This includes
input whose length exactly equals the cap: an additional read would be needed
to discover EOF. Previously emitted output remains; provisional empty carry is
not discarded. `ShapeEvents` still checks for unclosed feature scopes at EOF.
This fixes the missing zero-output progress path in `Shape` and premature carry
discard in `ShapeEvents`. The broader minimum capacity, context limits, and
safe-cut policy remain under SHAPE-015.

**Regression coverage:** the normal-parsed
[progress font](../testdata/fonts/STREAMING.md) contains `sub d by NULL` and both
required layout tables. Unchanged independent HarfBuzz captures expect four
`a` glyphs from `adadada`, at clusters `[0,2,4,6]` with advances
`[580,580,580,600]`, and no output from `dddddd`. Twenty-four comparisons
(whole/lookahead/carry × both APIs × both flush modes) are promoted. A new
independent `ddaadd` capture adds twelve passing comparisons for leading,
trailing, and consecutive deletion, with clusters `[0,3]` and advances
`[580,600]`. All twelve prior streaming captures, their metadata, and the font
binary remain unchanged.

Focused regressions in [otlayout/deletion_test.go](../otlayout/deletion_test.go),
[otshape/deletion_test.go](../otshape/deletion_test.go), and
[internal/hbcmp/deletion_test.go](../internal/hbcmp/deletion_test.go) cover direct,
extension, and contextual deletion, shared positions, journals and partial
errors, deleted contextual targets, continuation, survivor masks/source anchors,
cluster merging, syllable-isolated execution, retained lookahead context after
an initially empty result, bounded capacity errors, exact-cap input, preservation
of earlier sink writes, and feature-scope closure after all-deletion.

**Remaining scope:** tiny/forced mixed-deletion profiles still lose kerning
context; all-deleted profiles now fail explicitly at capacity. These full-output
comparisons, including the new edge capture, remain tagged under SHAPE-015 with
unchanged expectations. Separate ordinary tests assert the exact capacity error.
General buffer-contract work can later reclassify unsupported configurations.
SHAPE-016 subsequently removes event barriers within a carry. Ignored-mark preservation was subsequently
resolved under SHAPE-019.

**Recorded implementation verification (2026-11-01):** full ordinary and race
suites, build, and vet passed. Discovery has ten failing top-level tests, down
from eleven; the SHAPE-017 wrapper is gone. The streaming matrix has 340
comparisons: 218 ordinary and 122 tagged. Outside deletion and its
reclassification, failure assignments are unchanged. Read-only preparation
verified all thirteen streaming captures. See [TESTING.md](TESTING.md) and the
[matrix report](../internal/hbcmp/testdata/text/STREAMING.md).

```sh
go test ./otlayout ./otshape ./internal/hbcmp -run 'Deletion|SHAPE017|TestHarfBuzzStreamingBaselines/progress_' -count=1 -timeout=30s
```

### SHAPE-018 — Forward lookup scans bypass feature masks

**Fixed:** implemented and reviewed 2026-10-29, within the agreed forward-lookup
scope. Review found no blocking defects.

**Cause and fix:** `matchCoverageForward` could search past the glyph whose mask,
alternate value, and random seed the shaping loop had checked. `matchCoverageAt`
now attempts only the designated start; uncovered or lookup-flag-ignored starts
are normal non-matches. The caller advances and rechecks the next candidate.
This repairs the TTX-derived `[17,18]` range case from `[17,20]` to `[17,22]`.

Contextual/chained first inputs, nested sequence-record targets, and extension
lookups follow the same start rule. A matched contextual rule may still inspect
or modify later glyphs; subsequent inputs, lookahead, and backtrack retain
ignored-glyph skipping. Nested calls inherit the enclosing alternate value and
random stream; they do not independently recheck feature settings at every
nested target. See [FEATURE_APPLICATION.md](FEATURE_APPLICATION.md#forward-lookup-starts-shape-018)
for the changed direct-caller contract. Error and partial-progress contracts
are unchanged.

**Permanent regressions:**

- [Public alternate/range/scope tests](../otshape/otcore/alternate_values_test.go)
  check the original crossing case and disabled/value-3 ranges through both APIs
  with whole-input and streaming buffers.
- [Lookup-start tests](../otlayout/lookup_start_test.go) cover direct/extension
  GSUB/GPOS misses, explicit caller advancement, and nested targets.
- [Internal shaping tests](../otshape/alternate_values_test.go) check masked GPOS
  targets and fixed-seed choices at the same source anchor with/without a prefix.
- Eight complete [HarfBuzz matrix cells](../internal/hbcmp/streaming_baselines_test.go)
  were promoted unchanged: `scope_nested` whole/runes and `scope_accent`
  whole/lookahead/carry/runes, both flush modes. All output fields remain checked.

**Remaining scope:** eight tiny/forced scope cells retain their complete oracles
under SHAPE-015. SHAPE-016 subsequently removed scope context barriers within
each buffered carry. Reverse chaining remains separate; contextual progression
was subsequently fixed under
[SHAPE-006](#shape-006--contextual-lookup-revisits-matched-input).
An explicit ignored-start regression, particularly for format-3 contexts, would
strengthen coverage; that branch was verified by inspection during review.

**Review verification (2026-10-29):** the full ordinary suite and
`go test -race ./otlayout ./otshape/... ./internal/hbcmp -count=1` passed.
Restoring the old scanning behavior through a temporary Go overlay made all six
focused regression tests fail. Discovery retained 18 failing top-level tests,
without build failures, panics, or timeouts. The matrix has 150 passing and 170
failing cells. Independent captures and full-output assertions are unchanged.

```sh
go test ./otlayout ./otshape/... -run 'SHAPE018|ForwardLookupStartsAtCursor|NestedForwardLookupStaysAtTarget' -count=1
go test ./internal/hbcmp -run '^TestHarfBuzzStreamingBaselines/(scope_nested|scope_accent)/' -count=1
```

### SHAPE-019 — Ligature substitution deletes ignored marks

**Status:** fixed; implemented and reviewed 2026-11-01 within the agreed
mark-preservation and GPOS 5 component-association scope.
The former `TestKnownBugSHAPE005AOTSLigatureAcrossMark` is now the ordinary
`TestAOTSGPOSBaselines/gpos5.tests:2`, with its complete upstream expectation
unchanged. The obsolete tagged wrapper is removed.

**Expected behavior and basis:** unchanged upstream
[`gpos5.tests:2`](../testdata/harfbuzz/aots/tests/gpos5.tests), at HarfBuzz revision
`29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b`, requires GIDs 30 and 31 to form
ligature 18 across ignored mark 19, preserving the mark. The complete glyph/origin
oracle is `[17@0,0 18@1500,0 19@1400,-80 17@3000,0]`.

**Cause:** [gsubLookupType4Fmt1](../otlayout/gsub.go) skipped the mark while
matching components, then replaced the entire matched span with the ligature.
That replacement deleted the skipped glyph as well as the consumed components.
GPOS 5 also always selected the last component. The fixture's first-component
anchor requires mark origin `(1400,-80)`; the last gives `(1401,-79)`, confirming
that preservation alone would not resolve the full expectation.

**Fix:** GSUB records the actual matched positions and preserves skipped
glyphs in input order after the new ligature. An immutable survivor mapping in
the edit journal keeps contextual targets and side metadata aligned: consumed
components map to the ligature, while skipped marks retain their own identities,
masks, source anchors and positions. Level-0 clusters merge across the span.
An ignored mark's source anchor does not become the ligature's source anchor.

Transient per-glyph provenance identifies the ligature occurrence and original
component, independently of buffer indexes and output clusters. It travels with
position records across GSUB/GPOS stages and isolated spans, flattens nested
ligatures, and reassigns associated marks after the last nested component.
Single and multiple substitutions preserve a mark's association. GPOS 5 uses
the matching association (clamped to the available component anchors); absent
or mismatched association retains the last-component fallback. Final sink
records omit this transient provenance but retain attachment diagnostics.

**Regressions:** the upstream record passes unchanged. Ten independent
HarfBuzz 15 captures in [ligatures.json](../internal/hbcmp/testdata/text/ligatures.json)
cover intervening/trailing/multiple marks, both nested constructions, separate
ligature occurrences, an input ligature, later single/multiple substitutions,
and Hebrew RTL. Both APIs compare all six captured fields (20 comparisons).
The new font is repository-authored; provenance and exact reproduction commands
are in [LIGATURE_MARKS.md](../testdata/fonts/LIGATURE_MARKS.md).
Lower-level tests independently verify preservation without GPOS, extension
lookups, contextual targets/continuation, shared ownership, translated survivor
maps, partial errors, component clamping and source/mask/position correspondence.

**Boundaries:** this builds on accepted SHAPE-005/007/008/017 without reopening
them. SHAPE-016 subsequently removed feature-event context barriers within each
buffered carry. SHAPE-022 subsequently fixed mark-to-mark predecessor selection.
Streaming cuts (SHAPE-015), broader attachment compatibility, and general
script-specific ligature matching policy remain outside this fix.
The Hebrew synthetic case verifies logical RTL
positioning, not complete Hebrew shaping conformance. No existing oracle or
fixture bytes changed, and no remaining tagged assertion was weakened.

**Review verification (2026-11-01):** the full ordinary suite, targeted race
suite (`./otlayout ./otshape/... ./internal/hbcmp`), build, and vet passed.
The promoted upstream record and all twenty new public-API comparisons passed.
Temporary Go overlays independently restoring mark deletion or disabling
component selection caused the relevant lower-level and public-API regressions
to fail, including the unchanged upstream record. Discovery retained nine
expected failing top-level groups with no unexpected passing groups.
Read-only verification reproduced all ten new HarfBuzz captures and rebuilt the
new font byte-for-byte. Implementation verification also recorded unchanged
35-record text and 13-record streaming captures. No blocking review findings.

```sh
go test ./otlayout ./otshape -run '^TestLigature' -count=1
go test ./internal/hbcmp -run 'TestSHAPE019|TestAOTSGPOSBaselines/gpos5.tests:2' -count=1
```

### SHAPE-020 — Default features run in feature order instead of lookup order

**Fixed:** implemented and reviewed 2026-11-01. No blocking findings within the
shared stage-scheduling and alternate-selection scope.

**Cause and fix:** the compiler assigned one stage per default/user feature,
so default feature-list order overrode LookupList order. With the original
LigaFirst fixture, `calt` ran before lookup 0 (`liga`), producing `f.alt i`
instead of HarfBuzz's `fi` ligature (GID 3, cluster 0, advance 600).
[The compiler](../otshape/plan_compile.go) now sorts and deduplicates lookups
within stages defined by explicit pauses. The original independent capture is
promoted unchanged; the swapped-order control remains ordinary.

**Stage contract:** [engine registrations](../otshape/plan_hooks.go) retain their
earliest stage; unregistered defaults/user features join the final stage.
Required features default to stage zero unless explicitly registered elsewhere.
Direct GSUB pauses run at their collection boundaries, including empty and
consecutive stages; nil callbacks still close a stage. GSUB pauses do not advance
GPOS stages. Anchored Before/After callbacks surround the whole containing stage,
and all shared-feature tags remain available as anchors. There is no public GPOS
pause API.

A lookup executes once per stage, retaining all contributing tags/masks and
per-feature metadata. Any unmasked/required contributor makes it unconditional.
Automatic joiner flags combine with AND; random/per-syllable flags come from the
first contributor, collected deterministically in required-first, then tag order.
See [SHAPER_PLAN.md](SHAPER_PLAN.md) for the scheduling and flag rules.

**Shared-alternate decision (2026-11-01):** active contributors with equal values
run once; zero/default equals one and all negative values mean random.
Required/unmasked contributors supply the default value. Incompatible values
produce `otshape.ErrConflictingAlternates` only when a nonempty alternate set
matches, including contextual/extension leaves. Other lookup kinds and uncovered
glyphs remain usable. A retained random flag forces random selection after
conflict checking. Earlier internal edits and emitted output survive; both public
APIs withhold the failing chunk. This explicitly agreed contract differs from
HarfBuzz's packed-mask numeric behavior; no oracle was adjusted to conceal it.

**Permanent regressions:**

- [Text baselines](../internal/hbcmp/text_baselines_test.go) and
  [event/contract tests](../internal/hbcmp/lookup_order_test.go) cover all seven
  lookup-order/shared-alternate captures through both public APIs, plus explicit
  conflict errors. [LOOKUP_ORDER.md](../testdata/fonts/LOOKUP_ORDER.md) records
  five reproducible fonts and their hashes. The corpus has 31 records:
  22 ordinary and nine tagged.
- [Planner tests](../otshape/lookup_order_test.go) cover required-feature and
  repeated registration placement, separate GPOS scheduling, empty/consecutive/
  nil pauses, callback errors after edits, shared masks/flags, resolved metadata,
  anchors, and a shared lookup appearing in separate stages.
- [Selection tests](../otshape/lookup_selection_test.go) cover equal, default,
  random, oversized, inactive, unmasked, and required contributors, conflicts,
  uncovered glyphs, and partial progress.
  [Lower-level tests](../otlayout/alternate_selection_error_test.go) cover deferred
  errors through direct, contextual, and extension calls, unaffected single
  substitutions/nonmatches, and clearing the error on a shared clone.

**Review verification (2026-11-01):** full ordinary and race suites passed,
including Arabic regressions. Read-only preparation checks verified all 31
captures against installed HarfBuzz 15.0.0 and reproduced all five synthetic font
binaries byte-for-byte. All 26 earlier capture records and metadata, and both
original font binaries, are unchanged. Comparing discovery results before and
after the fix removed only `TestKnownBugSHAPE020LookupOrder`: eleven failing
top-level tests remain, with no other changed failing tests/subtests. Build and
vet also passed during implementation.

**Remaining scope:** SHAPE-021 subsequently resolved setting precedence;
streaming/context limitations remain under SHAPE-015. Arabic's registered pause schedule is now honored by the shared
planner, but passing existing Arabic tests does not establish Arabic/Syriac
conformance. No resolved or deferred finding was reopened.

### SHAPE-021 — Engine and user feature settings combine as "off wins"

**Status:** fixed; accepted 2026-11-02. The two formerly tagged failures now run
in `TestFeaturePrecedence` in
[feature_precedence_test.go](../internal/hbcmp/feature_precedence_test.go), with
unchanged complete HarfBuzz expectations. Range/scope and value regressions
also run in the ordinary suite.

**Cause:** lookup selection and glyph masks resolved settings independently.
An engine disabling a feature removed its lookups even when the user enabled
it; a user disabling a feature left its mask off even when `OverrideFeatures`
enabled it. Thus either source could veto the other, contrary to hook order.
The original Gentium `office` reproductions returned six glyphs instead of the
ligature output for collect-off/user-on and user-off/override-on. Existing
independent `latin_office_liga_on` and `latin_office_liga_off` captures provide
the complete expected output for each effective setting.

**Agreed contract (2026-11-01/02):** `CollectFeatures` supplies engine defaults,
user settings override them, and `OverrideFeatures` runs last. An explicit
`OverrideFeatures` assignment applies to the whole input, superseding user
globals, ranges and event scopes for that tag. Collected defaults remain
effective outside user ranges; popped event scopes restore enclosing settings
before the final engine override is applied.

**Fix:** plan compilation now applies `CollectFeatures`, user settings, then
`OverrideFeatures`. One resolved per-tag setting sequence supplies lookup
selection, mask values/ranges and related fallback decisions. A later global
assignment discards earlier settings for that tag, including shadowed ranges
and alternate values. Engine stage/flag/pause registration stays separate;
earliest-stage retention and deterministic mask allocation are preserved.
`ShapeEvents`' existing range conversion passes through the same resolver;
no separate event precedence path is needed.

**Coverage:** `TestFeaturePrecedence` runs the original five cases plus four
range/override cases through both APIs. `TestFeaturePrecedenceNestedScopes`
reuses the unchanged nested-scope capture with an engine-provided default,
checking inheritance outside ranges and after pops.
`TestFeaturePrecedenceAlternateValues` verifies user-over-collect and
override-over-user values against the existing alternate capture.
`TestFeaturePrecedencePlanValues` checks original scalar anchors, zero/default,
negative/random and large positive values, removal of shadowed values, lookup
inclusion and earliest-stage retention. No HarfBuzz captures were changed.

**Scope and limits:** custom engines and the shared planner are covered; among
the supplied engines only `otarabic` uses these planning methods. This is no new
Arabic conformance claim. Required features remain unconditional. With no
explicit collected default, existing range-only mask defaults remain unchanged.
`HasFeature` remains an intent/default query rather than a font-support query.
Runtime `SetupMasks` still runs after initial mask assignment. Buffer/context
limitations remain separate.

**Recorded implementation verification (2026-11-02):** `go test ./... -count=1`
and `go vet ./...` passed. Tagged discovery across `./otshape/... ./internal/hbcmp`
retained four expected failing groups (SHAPE-004/014/015/023), with no unexpected
known-bug passes, build errors, panics or timeouts. The SHAPE-021 promotion removes
one group; the preceding SHAPE-023 renaming only consolidated historical wrappers.
No fixture or independent expectation was regenerated. See [TESTING.md](TESTING.md).

```sh
go test ./internal/hbcmp -run '^TestFeaturePrecedence' -count=1 -v
go test ./otshape -run 'TestFeaturePrecedencePlanValues|TestEngineFeatureMaskAllocationStable|TestSHAPE020' -count=1
```

### SHAPE-022 — Mark-to-mark attachment crosses an intervening base

**Status:** fixed; accepted 2026-11-01 within the predecessor-selection scope.
SHAPE-007 and SHAPE-019 are not reopened; broader attachment compatibility
remains separate.

**Original reproduction:** Gentium `x\u0301x\u0301`, Latn/en/LTR,
`mark=1,mark[1:3]=0`, whole-input buffers `5/1/6`. Equivalent events push
`mark=0` at scalar 1 and pop at 3. The first initial group retains mark positioning;
the second starts inside the disabled range. Previously both APIs produced the
correct four GIDs and clusters, but the final accent had offsets `(-999,360)`
instead of HarfBuzz's `(0,0)`. All advances and the first accent's `(31,0)` matched.

**Cause and focused fix:** `gposLookupType6Fmt1` searched backward until it found
a glyph in Mark2Coverage, passing over nonmatching bases and marks. It now
examines the nearest eligible predecessor once. That predecessor must be a
GDEF-classified mark and belong to Mark2Coverage; failure does not continue
searching for an earlier covered mark. The target search clears the ordinary
IgnoreBaseGlyphs/IgnoreLigatures/IgnoreMarks bits in a local context copy, so
bases and ligatures remain barriers. Mark filtering, attachment-class filtering,
and input masks remain active; the lookup's original flags and start checks
are unchanged. A filtered-out mark can still be skipped, while an eligible
but disabled or uncovered predecessor blocks attachment.

This follows the predecessor-selection portion of
[HarfBuzz's MarkMarkPosFormat1 implementation](https://github.com/harfbuzz/harfbuzz/blob/main/src/OT/Layout/GPOS/MarkMarkPosFormat1.hh).
Anchor selection, arithmetic, finalization, and glyph/cluster metadata are
unchanged. Broader ligature/component compatibility checks remain separate;
this fix does not claim complete mark-to-mark conformance or Unicode fallback
classification when GDEF is missing.

**Regressions:** `TestSHAPE022MarkAttachmentCrossesBase` promotes all four
original comparisons unchanged (both APIs, both flush modes); the tagged wrapper
is removed. The eleven scope captures now yield 44 ordinary comparisons. The
separate `mkmk=0` control is retained. `TestSHAPE022MarkToMarkPredecessor` covers
adjacent/nearest marks, base and ligature barriers despite ignore flags,
unclassified and covered nonmarks, uncovered intervening marks, mark filtering,
disabled inputs, and missing predecessors, each directly and through extension
lookups. Nonmatches preserve cursor/positions; successful matches verify the
parsed TTX anchors and preserve advances, other positions and lookup flags.
The existing synthetic anchor-placement test now uses an actual GDEF mark as its
type-6 target, preserving every placement assertion and parsed anchor value.

**Recorded implementation verification (2026-11-01):** the full ordinary suite,
targeted layout/shaping/HarfBuzz race suite, build, and vet passed. Read-only
recapture verified all eleven scope oracles;
no captured output or font bytes changed. Discovery retains six expected failing
top-level groups, removing only SHAPE-022, with no unexpected passing known-bug
groups, build errors, panics, or timeouts. SHAPE-021 subsequently resolved feature
precedence. Streaming safety (SHAPE-015), Hebrew ordering (SHAPE-023), and broader
attachment compatibility remain outside this change.

```sh
go test ./otlayout ./internal/hbcmp -run '^TestSHAPE022' -count=1
```
