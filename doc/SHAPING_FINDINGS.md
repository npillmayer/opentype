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

# Discovery suite: ten failing top-level tests across the two packages.
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
| [SHAPE-010](#shape-010--default-mark-positioning-is-missed-during-normalization) | reproduced | Hebrew presentation-form fallback ignores default-enabled GPOS mark support |
| [SHAPE-012](#shape-012--canonical-equivalent-mark-orders-shape-differently) | reproduced | Canonical ordering is missing before composition |
| [SHAPE-013](#shape-013--normalization-ignores-font-coverage) | reproduced | Composition/decomposition ignores available cmap mappings |
| [SHAPE-014](#shape-014--fonts-without-gpos-stop-at-the-parser) | deferred | Existing parser scope excludes fonts without GPOS |
| [SHAPE-015](#shape-015--streaming-cuts-lose-shaping-context) | reproduced | Buffer cuts change ligatures, pair positioning, and normalization |
| [SHAPE-016](#shape-016--feature-scopes-sever-unrelated-lookup-context) | reproduced | Feature-event spans break kerning across scope boundaries |
| [SHAPE-017](#shape-017--gsub-deletion-is-ignored) | reproduced | Empty GSUB multiple-substitution sequences leave glyphs unchanged |
| [SHAPE-019](#shape-019--ligature-substitution-deletes-ignored-marks) | reproduced | Ligature span replacement deletes skipped marks; component association remains to be verified |

Archived fixes in [Resolved findings](#resolved-findings):
[SHAPE-001 — mask preservation](#shape-001--setupmasks-changes-are-overwritten),
[SHAPE-002 — source ranges](#shape-002--feature-ranges-use-chunk-local-coordinates),
[SHAPE-003 — alternate selection](#shape-003--feature-values-do-not-select-alternates),
[SHAPE-005 — GDEF classification](#shape-005--gdef-classification-prevents-ignoremarks),
[SHAPE-006 — contextual continuation](#shape-006--contextual-lookup-revisits-matched-input),
[SHAPE-007 — final mark positioning](#shape-007--mark-attachments-do-not-resolve-to-origins),
[SHAPE-008 — GDEF mark advances](#shape-008--marks-retain-nominal-advances),
[SHAPE-011 — base/mark clusters](#shape-011--combining-mark-clusters-differ-from-harfbuzz-level-0),
and [SHAPE-018 — forward lookup starts](#shape-018--forward-lookup-scans-bypass-feature-masks).

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
`mark=1,mkmk=1`, the complete output now matches after SHAPE-007/011. With
`mark=0,mkmk=0`, HarfBuzz uses fallback GID 126 while our output stays
`[124,46]`: allocating masks is incorrectly treated as enabled support.
The NoMark fixture removes those features while retaining GPOS. Its default
fallback passes; explicitly requesting the absent features again incorrectly
suppresses fallback and emits `[124,46]` instead of `[126]`. The last two
cases extend `TestKnownBugSHAPE010HebrewComposition`. Exact fixtures and output
are in the [normalization report](../internal/hbcmp/testdata/text/NORMALIZATION.md).

**Fix scope:** derive effective mark support from resolved font/plan state,
including enablement, rather than mask allocation alone. Resolving composition
will not by itself fix internal mark order; the tagged tests retain all
position and cluster expectations. SHAPE-007/011 now resolve mark offsets and
base/mark grouping within their agreed scopes. Absence of the entire GPOS table is
separately blocked at the parser and deferred under SHAPE-014.

```sh
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE010' -count=1 -v
```

## SHAPE-012 — Canonical-equivalent mark orders shape differently

**Status:** reproduced. **Test:** `TestKnownBugSHAPE012CanonicalOrdering`.
**Verified:** 2026-10-26, source `3acce4440f5a215e9752cd2f7c25430b3279994a`
plus this batch's tests. This verification baseline also applies to SHAPE-013/014.

**Expected behavior and basis:** independent HarfBuzz captures of Gentium
`a + U+0323 + U+0301` and `a + U+0301 + U+0323` yield the same GIDs
`[277,2173]`, clusters `[0,0]`, advances `[940,0]`, and X offsets `[0,83]`.
Both inputs are Latn/en/LTR with default features and buffers `4/1/5`.

**Observed:** canonical input gets the expected GIDs, but noncanonical input
gets `[105,2289]`. Originally both retained clusters `[0,2]` and zero offsets.
After SHAPE-007/011 the canonical case passes its complete oracle and runs in
the ordinary suite; the noncanonical case remains tagged with full expectations.

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

## SHAPE-019 — Ligature substitution deletes ignored marks

**Status:** reproduced 2026-10-29 after the SHAPE-005 classification fix.
Split from that finding so the completed classification work can be archived.
**Test:** `TestKnownBugSHAPE005AOTSLigatureAcrossMark` in
[aots_knownbugs_test.go](../internal/hbcmp/aots_knownbugs_test.go). The test retains
its historical name in this documentation-only update; it now tracks SHAPE-019.

**Expected behavior and basis:** unchanged upstream
[`gpos5.tests:2`](../testdata/harfbuzz/aots/tests/gpos5.tests), at HarfBuzz revision
`29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b`, requires GIDs 30 and 31 to form
ligature 18 across ignored mark 19, preserving the mark. The complete glyph/origin
oracle is `[17@0,0 18@1500,0 19@1400,-80 17@3000,0]`.

**Reproduction:** `gpos5_font1.otf`, input
`U+0011 U+001E U+0013 U+001F U+0011`, Latin/English/LTR, core engine,
global `test` enabled, watermarks high/low/max `6/1/7`.
`Shape` returns nil and emits `[17@0,0 18@1500,0 17@3000,0]`: the ligature
forms, but the mark is absent. Source/font hashes and the full upstream output
remain checked by the existing test; no capture was changed.

**Cause:** [gsubLookupType4Fmt1](../otlayout/gsub.go) skips the mark while
matching components, then replaces the entire matched span with the ligature.
That replacement deletes the skipped glyph as well as the consumed components.

**Fix scope and dependencies:** preserve skipped marks and their source/cluster
correspondence during ligature substitution. Component association needs
verification once the mark survives: current GPOS defaults to the last ligature
component, but this failing output alone does not establish the required repair.
SHAPE-007/008 now resolve anchor offsets and cancel advances for surviving
GDEF marks in the agreed horizontal Latin/Hebrew scope. Keep the full
oracle tagged until all its blockers are resolved; a future mark-preservation
fix should also have an isolated regression that can pass independently of GPOS.
This differs from SHAPE-017, where an intentional empty substitution is ignored.

**Verification:** reproduced during the SHAPE-005 review on 2026-10-29 with the
classification fix in the working tree. This reassignment adds no test and does
not change the fifteen failing top-level discovery tests.

```sh
# Historical test name; current finding is SHAPE-019.
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE005AOTSLigatureAcrossMark$' -count=1 -v
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

### Hebrew internal mark ordering follow-up

This remains open independently of the resolved SHAPE-011 grouping fix.
`TestKnownBugSHAPE007HebrewMarkOffsets` retains the complete pointed-shin oracle:
Noto Sans Hebrew, U+05E9 U+05B8 U+05C1, RTL with default features. All three glyphs
now have cluster 0 and correct per-glyph offsets, but glyph order remains
`[96,79,100]` instead of `[96,100,79]`. The historical SHAPE-011 test attribution
identifies this remaining follow-up, not a failure of the accepted grouping fix.
Choose its implementation scope separately; retain the full tagged assertion.
Tiny-buffer attachment-context failures are tracked under SHAPE-015.

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
policy for all merged clusters, range ends, or reordering. SHAPE-011 subsequently
introduced base/mark grouping while preserving separate source correspondence.

**Subsequent SHAPE-011 decision (2026-10-29):** initial base/mark groups now
use their starting scalar for both bounds: starts inside exclude the group;
ends inside include it. Source anchors remain separate from output grouping.
This supersedes the provisional preference above for those groups only;
feature-event barriers and Hebrew reordering remain separate.

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

**Remaining work:** classification now permits the interleaved-mark ligature,
but span replacement deletes the mark. That distinct failure is
[SHAPE-019](#shape-019--ligature-substitution-deletes-ignored-marks), retaining the
full oracle under the historical `TestKnownBugSHAPE005AOTSLigatureAcrossMark`
name. Component association remains separate; SHAPE-007/008 subsequently
resolved positioning for surviving marks within their agreed scope.

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
Intentional deletion (SHAPE-017), ignored-mark preservation (SHAPE-019), reverse
traversal, and streaming context safety remain separate. This resolution does
not establish complete contextual shaping conformance.

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

**Remaining scope:** the full historical `TestKnownBugSHAPE007HebrewMarkOffsets`
remains tagged for the [Hebrew mark-ordering follow-up](#hebrew-internal-mark-ordering-follow-up).
Noto Sans Hebrew mark offsets are exactly 227 and 539. SHAPE-011 subsequently
fixed grouping: output is now `[96/0,79/0,100/0]` instead of
`[96/0,100/0,79/0]`. Its four complete combining-mark text cases now pass unchanged
in the ordinary suite. Normalization blockers remain
SHAPE-010/012. Cursive resolution, hinted anchor points, device/variation
adjustments, ignored-mark preservation/component association (SHAPE-019), and
streaming/event context (SHAPE-015/016) are outside this fix. Adjacent marks on
ligatures retain the existing last-component choice.

### SHAPE-008 — Marks retain nominal advances

**Fixed:** implemented and reviewed 2026-10-29 within the agreed Latin/Hebrew
scope. No blocking defects found.

**Cause and fix:** runs store positioning adjustments; output materialization
adds nominal horizontal metrics. Zeroing an internal adjustment alone therefore
leaves the nominal mark advance intact. Public Latin/Hebrew plans whose engine
permits GPOS now enable `ZeroGDEFMarks`. After GPOS,
[the position policy](../otshape/plan.go) sets each GDEF mark's horizontal
adjustment to minus its nominal metric and its vertical adjustment to zero.
Materialization then yields zero final advances, including for unattached marks.
The current output path adds no nominal vertical metric; this is not vertical
shaping support.

The policy replaces existing advance adjustments, preserves offsets and all
attachment metadata, and is idempotent. It requires a GDEF glyph-class table;
Unicode category or attachment metadata alone does not opt a glyph in.
Other scripts and GPOS-disabled requests retain their previous policy. The older
internal `ZeroMarks`/fallback-positioning mechanism is unchanged.

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
suite. SHAPE-019 still deletes an intervening mark before positioning. Missing usable
GDEF, Unicode fallback, other scripts, normalization/cluster differences, and
safe streaming boundaries remain separate.

```sh
go test ./internal/hbcmp -run '^TestAOTSGPOSBaselines/gpos4_simple' -count=1 -v
go test ./otshape ./otshape/otcore -run '^TestGDEFMarkAdvance' -count=1
```

### SHAPE-011 — Combining-mark clusters differ from HarfBuzz level 0

**Fixed:** implemented and reviewed 2026-10-29 within the agreed Latin/Hebrew
base/mark grouping scope. No blocking findings; Hebrew internal mark ordering
and context-safe streaming remain separate.

**Cause and fix:** base letters and combining marks retained separate scalar
clusters. [Initial grouping](../otshape/shape_runtime.go) now forms groups before
normalization using Unicode mark categories and ZWJ continuation. Leading marks
form a leading group; ZWNJ remains a boundary. Normalized source anchors map
back to these groups. [Cluster merging](../otshape/lookups.go) includes adjacent
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
substitutions retain existing mask inheritance. Feature-event boundaries inside
a group remain separate from indexed `Shape` ranges (SHAPE-016).

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
for the [Hebrew mark-ordering follow-up](#hebrew-internal-mark-ordering-follow-up).
Canonical normalization (SHAPE-012), cmap-aware normalization (SHAPE-013),
effective Hebrew mark support (SHAPE-010), streaming/limits (SHAPE-015), and
feature-event barriers (SHAPE-016) remain open. General emoji and other
script-specific sequence rules are not claimed. Conservative partial compaction
still rejects repeated output clusters; separate source anchors do not prove
that a prefix is safe to emit.

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
under SHAPE-015. Reverse chaining and scope context barriers (SHAPE-016) remain
separate; contextual progression was subsequently fixed under
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
