# Shaping Engine and Test Strategy

Current coverage and open findings are described below; resolved issues have
compact reference entries in
[SHAPING_FINDINGS.md](SHAPING_FINDINGS.md#resolved-findings). Dated assessments
and batch counts describe their original discovery snapshots.

## SHAPE-023 resolution (2026-11-02)

Hebrew now sorts with modified combining classes and uses the same classes for
composition blocking. Its meteg adjustment runs per mark sequence before
composition, with no later replay. Original scalar sources and initial level-0
groups retain their separate roles. The new optional normalization reorder hook
is documented in [SHAPER_INTERFACES.md](SHAPER_INTERFACES.md).

Both original pointed-Hebrew failures are promoted unchanged. The focused
`TestSHAPE023HebrewOrdering` compares twelve cases through both APIs (24 full
six-field comparisons): the two original cases, the existing reordered-shin
control and nine new independent captures. See
[Hebrew ordering corpus](../internal/hbcmp/testdata/text/HEBREW_ORDER.md).
Lower tests cover source correspondence, leading marks, stable equal classes,
starters and CGJ barriers, and early-hook timing including `NormalizationNone`.

Recorded implementation verification: `go test ./... -count=1` and
`go vet ./...` passed. Tagged discovery
across `./otshape/... ./internal/hbcmp` fails only in SHAPE-004/014/015, with no
unexpected known-bug passes or infrastructure failures. Earlier counts and
Hebrew exclusions below describe their dated snapshots. Safe streaming cuts
remain separate; no existing capture was changed. The accepted fix is archived
in [Resolved findings](SHAPING_FINDINGS.md#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).

## SHAPE-021 resolution (2026-11-02)

Plan-time settings now resolve in `CollectFeatures` → user → `OverrideFeatures`
order. Lookup selection and masks share the resolved settings. Explicit engine
overrides replace user globals/ranges/scopes for the entire input; collected
defaults remain effective outside user ranges. Stage registration, required
features and the runtime `SetupMasks` hook retain their contracts.

Both formerly tagged failures are promoted into `TestFeaturePrecedence` with
unchanged complete HarfBuzz expectations. The five original cases and four
range/override cases run through both public APIs. Additional tests cover nested
scope restoration, alternate-value precedence, signed/default/large values,
shadowed-value removal, deterministic masks and earliest-stage retention.
No fixtures or independent expectations were regenerated.

Recorded implementation verification: `go test ./... -count=1` and
`go vet ./...` passed. Tagged discovery across
`./otshape/... ./internal/hbcmp` fails only in SHAPE-004/014/015/023, with no
unexpected known-bug passes, build errors, panics or timeouts. The count drops
from five to four because SHAPE-021 is promoted; the earlier SHAPE-023 count
change below was attribution only. The accepted fix is archived in
[Resolved findings](SHAPING_FINDINGS.md#shape-021--engine-and-user-feature-settings-combine-as-off-wins).

## Historical SHAPE-023 attribution (2026-11-02)

The existing Hebrew internal mark-ordering follow-up now has its own
[SHAPE-023 entry](SHAPING_FINDINGS.md#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).
`TestKnownBugSHAPE023HebrewMarkOrdering` contains the unchanged pointed-shin and
pointed-shalom regressions formerly named under SHAPE-007 and SHAPE-010; both
baseline assignments now identify SHAPE-023. No production code, fixture or
assertion changed. Composition, clusters and per-glyph positions remain fixed;
internal mark order still differs. Merging two historical wrappers into one
reduces the expected failing top-level group count from six to five without
resolving either case. Historical verification counts below remain dated snapshots.

Verification: `go test ./internal/hbcmp -count=1` passes. Tagged discovery across
`./otshape/... ./internal/hbcmp` fails only in the five expected top-level groups
(SHAPE-004/014/015/021/023), including both SHAPE-023 subtests, with no unexpected
known-bug passes or infrastructure failures.

## SHAPE-022 resolution (2026-11-01)

GPOS mark-to-mark attachment now considers only the nearest eligible predecessor,
which must be a GDEF mark in Mark2Coverage. Ordinary ignore flags cannot hide an
intervening base/ligature; mark filtering and feature masks still apply. Anchor
arithmetic and finalization remain unchanged. Broader ligature/component
compatibility and missing-GDEF Unicode classification are outside this fix.

Four full HarfBuzz comparisons are promoted unchanged into
`TestSHAPE022MarkAttachmentCrossesBase`: both public APIs and both flush modes.
The eleven scope captures now yield 44 ordinary comparisons, including the
separate `mkmk=0` control. `TestSHAPE022MarkToMarkPredecessor` covers positive and
negative predecessor selection, filtering, masks, unclassified/covered nonmarks,
state preservation and extensions. The existing anchor-placement unit fixture
now supplies a GDEF mark as the synthetic type-6 target; its parsed-anchor and
placement assertions are unchanged.

Recorded implementation verification: the full ordinary suite, targeted
layout/shaping/HarfBuzz race tests, build, and vet passed. Read-only recapture
reproduced all eleven scope captures.
Discovery retains six expected failing top-level groups, removing only SHAPE-022,
with no unexpected passing known-bug groups or infrastructure failures.
The accepted fix is archived in
[Resolved findings](SHAPING_FINDINGS.md#shape-022--mark-to-mark-attachment-crosses-an-intervening-base)
with its reproduction, scope and regression evidence. The historical SHAPE-016 counts below
describe its original verification, before this promotion.

## SHAPE-016 resolution (2026-11-01)

Feature events now share normalization, a combined plan/mask layout, and lookup
context for each buffered carry. Six original `scope_pair` event comparisons
(whole/lookahead/carry, both flush modes) are promoted unchanged. Four tiny/forced
event comparisons move to SHAPE-015 with their full expectations retained. The
original 340-cell streaming matrix now has 224 ordinary and 116 tagged cases.

[SCOPES.md](../internal/hbcmp/testdata/text/SCOPES.md) describes eleven new
independent HarfBuzz 15 captures: ten initially passed through both APIs and both
flush modes (forty comparisons). The eleventh exposed SHAPE-022; its four
comparisons subsequently passed unchanged, bringing the total to 44 ordinary
comparisons. The two existing normalization captures
with indexed ranges now also have equivalent `ShapeEvents` recipes; the original
fourteen-record normalization corpus has 28 ordinary comparisons. No old capture
or assertion changed.

Lower-level tests cover masks on all twelve contextual GSUB/GPOS format paths,
unmasked backtrack/lookahead, ignored marks, and inheritance through nested
edits, deletion, clones and ligature survivors. Event tests verify combined-plan
engine state, shared hooks and masks, implicit defaults after pop/compaction,
stable random choices across combined carry rebuilds, and validation failures
before output. Existing scope/error, random-value and
shared-alternate conflict tests remain ordinary. This adds no safe-cut guarantee
and does not establish cursive finalization or Hebrew ordering. SHAPE-021
subsequently resolved engine/user precedence.

Recorded implementation verification: `go test ./... -count=1`, targeted race
tests for `otlayout`, `otshape/...` and `internal/hbcmp`, `go build ./...`, and
`go vet ./...` passed.
Tagged discovery has seven expected failing top-level groups (SHAPE-016 removed,
SHAPE-022 added), no unexpected passing known-bug groups, and no build/panic/
timeout failures. The accepted fix is archived in
[Resolved findings](SHAPING_FINDINGS.md#shape-016--feature-scopes-sever-unrelated-lookup-context).

## SHAPE-012/013 resolution (2026-11-01)

The jointly authorized normalization fix chooses cmap-supported decompositions,
orders combining marks stably, and recomposes against the starter with canonical
blocking and coverage checks. Original source anchors and level-0 group-based
feature ranges survive each pass. Hebrew explicitly retains its previous order
until its separate script-specific ordering fix; shared coverage checks apply.

Three original regressions are promoted with unchanged complete captures. The
35-record text corpus now has 32 ordinary and three tagged cases (two Hebrew
ordering cases and the deferred no-GPOS parser case). Fourteen new independent
HarfBuzz 15 captures in `text/normalization.json` add 26 whole-input comparisons:
twelve cases through both APIs and two indexed-range cases through `Shape`.
They cover precomposed accents, equivalent mark orders, composition blocking,
missing composites, unavailable components, partial decomposition and ranges
after expansion/reordering. Four original normalization cases also run through
`ShapeEvents`. No event-span or streaming-boundary guarantee is added.

`otshape/normalize_test.go` verifies original anchors, equal-class stability,
leading marks, CGJ barriers, no silent CGJ insertion, modes, partial font coverage,
custom-hook coverage checks, and the agreed Hebrew ordering exclusion. The
existing decomposition/source test now expects canonical mark order while
retaining the original source association of every resulting rune.

Recorded implementation verification: the full ordinary suite, focused
shaping/layout/HarfBuzz race suite, build, and vet passed. Tagged discovery retains seven expected failing
top-level groups, removing SHAPE-012/013, with no unexpectedly passing known-bug
groups. Read-only preparation checks reproduced all five normalization fonts,
all 35 original text captures, and all fourteen new captures. Original fixtures
and oracles are unchanged. Both findings are accepted and archived with their
stable IDs: [SHAPE-012](SHAPING_FINDINGS.md#shape-012--canonical-equivalent-mark-orders-shape-differently)
and [SHAPE-013](SHAPING_FINDINGS.md#shape-013--normalization-ignores-font-coverage).

## Reviewed SHAPE-019 resolution (2026-11-01)

GSUB ligatures preserve skipped glyphs with an edit-journal survivor mapping.
Component provenance survives stages, nested ligatures and later substitutions;
GPOS 5 selects the associated component. The unchanged upstream
`gpos5.tests:2` now passes in `TestAOTSGPOSBaselines`; its historical SHAPE005
tagged wrapper is removed. All fourteen selected AOTS records pass.

Ten new independent HarfBuzz 15 captures in `text/ligatures.json` add twenty
whole-input comparisons through both APIs. They cover intervening, trailing and
multiple marks, both nested constructions, distinct ligatures, input ligatures,
later single/multiple substitutions and Hebrew RTL. All six output fields are
asserted. The existing 35 text and thirteen streaming capture records are
unchanged. The synthetic font and reproducible builder are documented in
[LIGATURE_MARKS.md](../testdata/fonts/LIGATURE_MARKS.md).

Focused lower-level regressions cover isolated preservation, extension lookups,
contextual targets and continuation, retained positions and attachments,
source anchors, masks, cluster merging, shared buffers, partial errors and
component clamping. They do not depend on GPOS to demonstrate mark survival.
Review verification: the full ordinary suite, targeted race tests for
layout/shaping/HarfBuzz, build and vet passed. The upstream regression and all
twenty new public-API comparisons passed. Temporary overlays restoring mark
deletion or disabling component selection independently triggered the relevant
regressions. Discovery retained nine expected failing groups with no unexpected
passing groups. Read-only checks reproduced all ten new captures and the font
byte-for-byte; implementation verification also checked the 35 text and thirteen
streaming records. SHAPE-016 subsequently removed event barriers within a carry.
Streaming boundaries and general mark-to-mark compatibility remain separate. The fix is accepted and archived in
[Resolved findings](SHAPING_FINDINGS.md#shape-019--ligature-substitution-deletes-ignored-marks).

## SHAPE-010 resolution (2026-11-01)

Normalization now derives mark support from resolved GPOS lookup contributors
and their enablement, including shared lookups, unmasked defaults and required
features. Focused plan tests cover empty/absent features, inactive contributors,
disabled GPOS, and ranges. The agreed signal is `mark`-only and plan-wide;
`mkmk` alone does not suppress Hebrew presentation-form fallback.

Three existing vav/holam regressions are promoted with unchanged expectations.
Four independent HarfBuzz captures add mark-only, mkmk-only, ranged-on and
ranged-off coverage. All 31 previous records and provenance are unchanged.
The text corpus now has 35 cases: 29 ordinary and six tagged. Seven global
mark-setting cases also run through `ShapeEvents`; indexed ranges use `Shape`
in that batch. SHAPE-016 subsequently removed event-plan normalization barriers
within each buffered carry and added focused range/event equivalence coverage.

Pointed shalom now has the correct seven glyphs, clusters and per-glyph positions;
its full comparison subsequently passed unchanged after SHAPE-023 resolved
internal mark order. SHAPE-007/011 are not reopened.

Recorded implementation verification: the full ordinary suite passed;
tagged discovery retained ten failing top-level tests, removing exactly the three
promoted subtests, with no new failures, panics, or timeouts.
`go build ./...` and `go vet ./...` passed. Read-only recapture verified all 35
independent HarfBuzz records. SHAPE-010 is accepted and archived in
[Resolved findings](SHAPING_FINDINGS.md#shape-010--default-mark-positioning-is-missed-during-normalization).

## SHAPE-017 resolution (2026-11-01)

GSUB 2 empty sequences delete their input and journal the edit. Regression tests
cover contextual targets/continuation, partial errors, shared positions, and
survivor metadata. Leading deletion carries the disappearing cluster start
forward, with surviving source anchors and masks unchanged. Both public APIs
retain zero-output raw carry until EOF; capacity before known EOF returns the
agreed `ErrZeroOutputBufferFull`, including exact-cap input. Separate tests cover
lookahead changing an initially empty result, scope closure, and prior sink output.

Twenty-four existing whole/lookahead/carry deletion matrix cells are promoted
with unchanged HarfBuzz expectations. One new `ddaadd` capture verifies clusters
`[0,3]`, surviving glyphs `[1,1]`, and advances `[580,600]`, adding twelve passing
cells. The original twelve streaming captures, their capture metadata, and the
progress font remain unchanged. The matrix now has thirteen captures plus four
reused text cases: 340 comparisons, 218 ordinary and 122 tagged.

Sixteen old tiny/forced deletion comparisons and eight new edge-deletion
comparisons remain under SHAPE-015 with complete full-output expectations.
They expose cut-related context loss or the deliberate zero-output capacity
limit. Ordinary contract tests separately assert the specific error; the
independent oracle is not replaced with an error expectation. The former
SHAPE-017 tagged wrapper is removed. General buffer limits remain separate.

Recorded implementation verification: `go test ./... -count=1`,
`go test -race ./... -count=1`,
`go build ./...`, and `go vet ./...` passed. Discovery with `shaping_knownbugs`
and a 30-second timeout has ten failing top-level tests (previously eleven),
without new non-deletion failures, fixture errors, panics, or timeouts. Read-only
`capture_harfbuzz_streaming.py` verifies all thirteen independent captures.
SHAPE-017 is accepted and archived in
[Resolved findings](SHAPING_FINDINGS.md#shape-017--gsub-deletion-is-ignored).

## Reviewed SHAPE-020 resolution (2026-11-01)

The shared GSUB/GPOS compiler now groups features by explicit pause boundaries
and sorts/deduplicates lookups within each stage. The original LigaFirst
regression is promoted unchanged. Five additional independent captures cover
swapped GPOS order and shared alternate lookup values; `text/baselines.json` now
has 31 records (22 ordinary, nine tagged). All 26 previous capture records and
metadata, and both original lookup-order font binaries, remain unchanged.
Both public APIs compare the seven lookup-order/shared-alternate cases against
all captured fields. Fixture definitions and hashes are in
[LOOKUP_ORDER.md](../testdata/fonts/LOOKUP_ORDER.md).

Focused regressions cover explicit/empty/nil pause boundaries, repeated feature
registration, required-feature placement, independent GPOS stages, shared masks
and flags, retained feature metadata and anchors, and alternate conflicts at
matching leaves (including nested/extension lookups). Conflicts follow the
user-approved `ErrConflictingAlternates` contract; incompatible shared values
are not claimed to match HarfBuzz's packed-mask numeric behavior.

Review verification: `go test ./... -count=1` and
`go test -race ./... -count=1` passed, including Arabic regression tests.
Build and vet also passed during implementation.
The tagged discovery command below still fails as expected: 11 top-level tests,
down from 12 before the fix. The only removed failure is
`TestKnownBugSHAPE020LookupOrder`; every other failing test/subtest is unchanged.
No new failures, unexpected passes, fixture failures, panics, or timeouts occurred.
SHAPE-021 subsequently resolved setting precedence; no broad script-conformance
claim is made.

```sh
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1 -timeout=30s
python3 testdata/scripts/capture_harfbuzz_text.py
PYTHONPATH=/path/to/fonttools/Lib python3 testdata/scripts/build_lookup_order_fonts.py
```

Read-only review checks verified all 31 captures against HarfBuzz 15.0.0 and
reproduced all five synthetic binaries byte-for-byte. The fix passed review and
is archived as [SHAPE-020](SHAPING_FINDINGS.md#shape-020--default-features-run-in-feature-order-instead-of-lookup-order),
retaining the stage and shared-alternate contracts and permanent regression links.

## Reviewed SHAPE-011 resolution (2026-10-29)

Latin/Hebrew base/mark groups now share original-scalar cluster starts. Indexed
ranges select these initial groups by start; source anchors remain separate for
random selection and edit provenance. Four complete text captures and 32 matrix
cells are promoted unchanged. The corpus has 15 passing / 9 tagged text cases;
the streaming matrix has 182 passing / 138 tagged cells. Eight tiny-buffer mark
cells remain under SHAPE-015, and the historical pointed-shin reproduction
needed SHAPE-023 at that stage. Its complete oracle now passes unchanged after
[the ordering fix](SHAPING_FINDINGS.md#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).
The [reviewed SHAPE-011 resolution](SHAPING_FINDINGS.md#shape-011--combining-mark-clusters-differ-from-harfbuzz-level-0)
is archived with its stable ID, range decision, and regression links.
The full ordinary and race suites pass; discovery has ten remaining top-level
failure groups. Read-only verification confirmed all 24 stored captures against
HarfBuzz 15.0.0. Temporary overlays independently disabling grouping, cluster
merging, source-anchor random selection, and scratch isolation triggered their
regressions. Build and vet checks passed during implementation.

## Reviewed SHAPE-007 resolution (2026-10-29)

The two attached-mark origin records (`gpos4_simple.tests:2`, `gpos5.tests:3`)
now pass unchanged and are ordinary `TestAOTSGPOSBaselines` cases. Of fourteen
selected AOTS records, thirteen passed at this review stage; SHAPE-019
subsequently resolved the final tagged record, so all fourteen now pass.
The Latin/Hebrew shaper finalizes design-unit mark-to-base/ligature/mark
attachments after GPOS and advance zeroing. Focused regressions cover chains,
lookup ordering, LTR/RTL pen compensation, both APIs, and idempotence.
The review's attachment-time Y-offset overflow is corrected with widened,
saturating arithmetic. `TestMarkAttachmentYOffsetSaturation` covers both
overflow directions, exact limits, nearby values, and full anchor differences.
The fix is accepted within the agreed horizontal Latin/Hebrew scope and
archived under resolved findings.
Complete text assertions retain their cluster, normalization, and mark-order
blockers. The discovery suite has twelve failing top-level groups; the ordinary
suite passes after the overflow correction. Race, build, and vet checks passed
during the preceding review. See the archived
[SHAPE-007 resolution](SHAPING_FINDINGS.md#shape-007--mark-attachments-do-not-resolve-to-origins)
for scope and remaining limitations. Earlier dated counts below are historical.

## Reviewed SHAPE-008 resolution (2026-10-29)

The two unattached-mark origin records (`gpos4_simple.tests:3` and `:4`) pass
unchanged and are ordinary `TestAOTSGPOSBaselines` cases. At this review stage,
fourteen selected AOTS records had eleven passing and three tagged records,
plus the passing simple-alternate streaming execution. SHAPE-007 subsequently
resolved the two attached-mark records; SHAPE-019 subsequently resolved
ignored-mark preservation and component association. All fourteen now pass.

The [reviewed SHAPE-008 policy](SHAPING_FINDINGS.md#shape-008--marks-retain-nominal-advances)
cancels final advances of GDEF-classified marks after
GPOS for Latin/Hebrew requests. It preserves placement and attachment metadata;
missing GDEF/class tables, Unicode fallback, other scripts, and GPOS-disabled
requests are outside this policy. Focused tests cover positive/negative advance
adjustments, repeated execution, both APIs, Latin attached/unattached marks,
and Hebrew policy selection. The synthetic Hebrew fixture does not establish
script-specific attachment or full Hebrew conformance.
The older internal fallback policy remains unchanged. A public GPOS-single test
now accounts for final mark zeroing while preserving its placement assertions.

## Reviewed SHAPE-006 resolution (2026-10-29)

The [reviewed SHAPE-006 fix](SHAPING_FINDINGS.md#shape-006--contextual-lookup-revisits-matched-input)
keeps forward contextual matches from revisiting their input. The upstream case
passes unchanged in
`TestAOTSGSUBBaselines`. Fourteen selected AOTS records now have nine passing
and five tagged records, plus the passing simple-alternate streaming execution.
The shared endpoint fix covers GSUB 5/6 and GPOS 7/8, all three formats. Focused
checks cover ignored marks, lookahead boundaries, nested expansion/contraction,
partial failures, and whole/isolated-run progression. The approved forward
class-matching offset correction has a separate forward/backward regression.
Empty-record/no-applying-child behavior remains separate. Intentional deletion
was subsequently resolved under SHAPE-017, and ignored-mark preservation and
component association under SHAPE-019.

## Reviewed SHAPE-005 resolution (2026-10-29)

The [reviewed SHAPE-005 fix](SHAPING_FINDINGS.md#shape-005--gdef-classification-prevents-ignoremarks)
corrects inclusive ClassDef format-2 endpoints and GDEF class numbers.
Three unchanged AOTS records were promoted: interleaved IgnoreMarks and flagged
alternates in `TestAOTSGSUBBaselines`, and the flagged pair-advance case in
`TestAOTSGPOSBaselines`. The fourteen selected AOTS records now have eight
passing and six tagged records, plus the passing simple-alternate streaming
execution. `TestAOTSGDEFClassification` retains the promoted classification
assertions; parser boundary tests cover singleton/adjacent ranges and glyph 65535.

Two cursive metadata tests previously treated unclassified glyphs as base glyphs.
Their expectations now follow the fonts' GDEF and checked-in GPOS TTX; an added
case verifies skipping the actual base glyph. No HarfBuzz expectations changed.
The full interleaved-mark ligature record subsequently passed after
[SHAPE-019](SHAPING_FINDINGS.md#shape-019--ligature-substitution-deletes-ignored-marks)
resolved mark preservation and component association. It now runs in
`TestAOTSGPOSBaselines/gpos5.tests:2`; its historical SHAPE005 wrapper was removed.
SHAPE-007/008 resolved positioning for surviving marks within their agreed scope.

## Assessment as of 2026-10-26

The shaping pipeline is substantially implemented and easy to exercise through
tests, but it is not yet a complete, reliable general-purpose shaper. Feature
planning, sequenced GSUB/GPOS execution, streaming output, script-specific
engines, and an offline HarfBuzz comparison harness exist. The earlier milestone
plan in this document predated these implementations; its restrictions on
multiple lookups and its proposal to create `internal/ttxtest` are obsolete.

The review included source inspection, existing tests, and temporary public-API
probes. The full `go test ./... -count=1` run passed. Four probes now have
reproducible, opt-in regression tests under the `shaping_knownbugs` build tag.
The first three recovered AOTS baseline cases add one passing ordinary test
and two further tagged findings (GDEF mark classification and contextual
lookup progression). Two subsequent alternate records add independent evidence
for existing range/value and GDEF findings; both remain tagged. The full
ordinary suite still passes. The GPOS batch adds one passing chained single-
positioning case and five tagged records, exposing unresolved mark origins
(SHAPE-007), retained mark advances (SHAPE-008), and another SHAPE-005 blocker.
The pair batch adds two passing pair-positioning cases and a flagged
advance-changing case blocked by SHAPE-005, using two newly recovered fonts.
The real-text corpus now has 24 complete HarfBuzz captures: 10 pass and 14
remain tagged. Logical sink order and HarfBuzz level-0 cluster grouping are
agreed contracts. Plain Hebrew passes with the explicit logical-order oracle
conversion (SHAPE-009 resolved). The normalization extension exposes canonical
ordering and font-coverage gaps (SHAPE-012/013), strengthens SHAPE-002/010/011,
and records the parser's no-GPOS scope restriction separately (SHAPE-014).
The streaming/scope extension adds twelve separate captures and a 320-execution
matrix: 134 pass, 186 remain tagged; ten empty-input/event checks also pass.
It exposes lost streaming context, feature-scope context barriers, ignored GSUB
deletion, and masked forward-scan errors (SHAPE-015–018). The initial test setup
for priority 5 is implemented; production fixes remain separate.
[SHAPING_FINDINGS.md](SHAPING_FINDINGS.md) records their workflow, evidence, and
open contract questions. Failing discovery tests expose gaps without fixing
the implementation and are excluded from the ordinary suite; passing baselines
remain ordinary regressions.

## Modules, directories, and testing responsibilities

Paths are relative to the repository root. These are packages and directories
within the same Go module.

| Location | Role in shaping and testing |
| --- | --- |
| [`ot/`](../ot/) | Parses font tables and concrete lookup graphs. Structural, synthetic, negative, and TTX tests check whether GSUB/GPOS data is read correctly, independently of shaping output. |
| [`otquery/`](../otquery/) | Supplies cmap lookup, glyph metrics, and font/script queries. Tests cover mapping, metrics, and query errors; failures can appear as incorrect glyphs or advances downstream. |
| [`otlayout/`](../otlayout/) | Executes GSUB/GPOS features and lookups on glyph/position buffers. Tests isolate substitution and positioning, nested lookups, edits, attachment remapping, errors, glyph bounds, and execution limits. |
| [`otshape/`](../otshape/) | Owns the public API, engine selection, normalization, feature plans/masks, streaming, and output. Tests cover orchestration, hooks, ranges, events, buffer correspondence, partial progress, and flushing. The godoc examples in `example_test.go` print output that must match the HarfBuzz captures `latin_office_liga_on` and `latin_office_liga_off` from `internal/hbcmp/testdata/text/baselines.json`. |
| [`otshape/otcore/`](../otshape/otcore/) | Generic engine. External-package tests exercise the public API with GSUB/GPOS fixtures, including single/pair adjustments and attachment metadata. `shared_api_knownbugs_test.go` adds opt-in reproductions for shared API findings. |
| [`otshape/otarabic/`](../otshape/otarabic/) | Arabic/Syriac staging, joining forms, masks, mark reordering, fallback repair, and postprocessing. Unit tests exercise these components; its streaming test uses non-joining digits. |
| [`otshape/othebrew/`](../otshape/othebrew/) | Hebrew composition and mark-reordering hooks. Tests cover selected script behaviors and streaming parity. |
| [`internal/ttxtest/`](../internal/ttxtest/) | Parses stored GSUB/GPOS TTX XML into expected models. Comparisons against parsed fonts live primarily in `ot/`; this helper is not a shaping oracle. |
| [`internal/hbcmp/`](../internal/hbcmp/) | Shapes Unicode through the public API. JSON comparisons check glyph IDs, clusters, advances, and offsets; the AOTS adapter checks glyph IDs or glyph IDs plus origins according to the stored format. All selected AOTS cases pass; the pointed Hebrew cases are promoted unchanged (SHAPE-023), with nine new ordering/composition captures checked through both APIs. `feature_precedence_test.go` reuses independent text, scope and alternate captures with a probe engine to check engine/user feature precedence through both APIs; all SHAPE-021 cases are ordinary. The streaming matrix adds buffer profiles, actual feature events, and progress instrumentation. `ligature_marks_test.go` compares ten component cases through both APIs. No runner invokes HarfBuzz during tests. |
| [`internal/hbcmp/testdata/`](../internal/hbcmp/testdata/) | End-to-end expectations: the original `Gentium_AB.json`, 35 Latin/Hebrew cases in `text/baselines.json`, fourteen normalization cases in `text/normalization.json`, eleven scope cases in `text/scopes.json`, ten component cases in `text/ligatures.json`, and thirteen streaming/scope/progress records in `text/streaming.json`. The text reports and `testdata/fonts/LIGATURE_MARKS.md` record semantics, commands, provenance and results. |
| [`testdata/harfbuzz/aots/`](../testdata/harfbuzz/aots/) | Pinned upstream AOTS shaping records, font hashes, and provenance. All fourteen selected records pass (five GSUB, nine GPOS, plus a GSUB streaming execution). `ADAPTER.md`, `GPOS.md`, and `PAIR.md` record semantics and results. |
| [`testdata/fonttools/`](../testdata/fonttools/) | Twelve original mini-fonts with paired `*.ttx.GSUB` / `*.ttx.GPOS` dumps, plus two subsequently recovered pair fonts without TTX dumps. Used for parsing, lookup, and public shaping tests. |
| [`testdata/fonts/`](../testdata/fonts/) | Larger fonts for mapping, metrics, parser, shaping, and diagnostics. Gentium Plus supplies Latin text baselines; pinned Noto Sans Hebrew supplies Hebrew cases, with provenance and OFL license alongside it. Five derived normalization fonts remove selected cmap mappings, mark features, or GPOS; `fonts/NORMALIZATION.md` records their generation and licenses. The repository-authored `ShaperTestProgress.ttf` isolates expansion, deletion, and contraction; `fonts/STREAMING.md` records its reproducible construction. The five repository-authored `ShaperTestLookupOrder-*.ttf` fonts isolate GSUB/GPOS lookup order and shared alternate selection; `fonts/LOOKUP_ORDER.md` records their construction. Also contains Calibri and Go fonts. |
| [`testdata/scripts/`](../testdata/scripts/) | Fixture discovery/preparation helpers. `find_test_fonts.py` indexes fonts by lookup type; `fix-maxp.py` prepares fonts; `recover_harfbuzz_aots.py` recovers upstream records; `capture_harfbuzz_text.py` and `capture_harfbuzz_normalization.py` capture/check complete text output with HarfBuzz 15.0.0. `build_normalization_fonts.py` reproducibly derives capability-reduced fixtures with FontTools. `build_streaming_font.py` builds the progress fixture; `build_lookup_order_fonts.py` builds the lookup-order fixtures; `capture_harfbuzz_streaming.py` captures/checks the thirteen additional oracles. These are not Go test drivers. |
| [`ot-tools/`](../ot-tools/) | Manual diagnostics: `shape` prints glyph output, `view` renders glyph proofs, and `font` inspects tables. Useful for investigation; visual inspection does not replace output assertions. |
| [`internal/fontload/`](../internal/fontload/) | Shared font-byte/SFNT loading support. It does not select or execute shaping features. |

The main dependency path is `otshape` → `otlayout` → `ot`, with `otquery`
providing mapping and metrics. Script engines plug into `otshape`. Localize
failures at the lowest relevant layer, then cover them through the public
shaping API when integration behavior matters.

## Current API and functionality

The entry points are methods on an explicitly constructed shaper:

```go
shaper := otshape.NewShaper(otcore.New(), otarabic.New(), othebrew.New())
err := shaper.Shape(params, source, sink, options)
// Advanced input with nested feature scopes:
err = shaper.ShapeEvents(params, eventSource, sink, options)
```

See [shape_api.go](../otshape/shape_api.go),
[shape_events.go](../otshape/shape_events.go),
[params.go](../otshape/params.go), and [pipeline.go](../otshape/pipeline.go).

- `NewShaper` accepts candidate engines; there is no global registration step.
  Selection uses confidence and name tie-breaking, then calls the engine's `New`.
- `Params` supplies one font, script, language, direction, and feature ranges.
  The caller supplies run metadata. Automatic script itemization, font fallback,
  and paragraph bidi processing are absent. The sink contract is logical shaping
  order; final visual ordering belongs to the caller.
- `RuneSource` has `ReadRune`; `strings.Reader` and `bufio.Reader` can supply
  input. `GlyphSink.WriteGlyph` receives IDs, positions, clusters, masks, and
  optional unsafe flags. Clusters originate as codepoint indexes, not UTF-8 byte
  offsets. Position values are in font units.
- `Shape` compiles a plan, repeatedly reads input, normalizes/maps it, runs
  script hooks and GSUB/GPOS stages, emits a prefix, and reshapes raw carry on the
  next cycle. Feature sequencing and stage pauses are implemented.
- `ShapeEvents` supports nested feature push/pop scopes. `Params.Features` must
  be global in this mode. Contiguous spans with different plan IDs are shaped
  separately.
- `BufferOptions` controls watermarks and the raw input buffer cap. Current
  high/low/max defaults are 3500 / 1024 / 4096 codepoints. `FlushExplicit` is
  unsupported. Despite its name, `FlushOnRunBoundary` can emit streaming prefixes
  before EOF; it does not provide atomic whole-request delivery.
- A shaping error prevents emission of the current in-progress chunk, but
  previously emitted prefixes remain. A sink error can leave part of a prefix
  written. Atomic consumers must collect output and commit only after success.

[engine_api.go](../otshape/engine_api.go) exposes hooks for feature planning,
validation, composition, reordering, pre-GSUB processing, masks, stage pauses,
and postprocessing. These interfaces make small test engines straightforward.

The core engine supplies common features. Arabic/Syriac adds joining and staged
features; Hebrew adds composition and reordering. No dedicated Indic or other
complex-script engines exist. Default per-syllable annotations derive from
clusters rather than a script syllable recognizer.

## Confirmed gaps and regression targets

### Failures reproduced through the public API

These began as temporary review probes. Engine selection remains tagged;
range-crossing forward starts now have ordinary regressions;
see [SHAPING_FINDINGS.md](SHAPING_FINDINGS.md) for stable IDs, exact commands,
and current status. Streaming-context and script-tag observations below remain
review probes outside that first regression batch.

| Finding | Reproduction / result | Implementation location |
| --- | --- | --- |
| Streaming loses context | With `gpos3_font1.otf`, input U+0012 U+0013, and `test` enabled, default buffering records a cursive attachment on the second glyph. High/low/max `1/1/2` loses it. | [shape_streaming.go](../otshape/shape_streaming.go) |
| A nonmatching engine can be selected | Supply only an engine returning `ShaperConfidenceNone`. Shaping succeeds instead of returning `ErrNoMatchingShaper`; selection skips negative scores but accepts zero. | `selectShapingEngine` in [shape_api.go](../otshape/shape_api.go) |
| Script tags are incomplete | `ScriptTagForScript` returns `DFLT` for `Syrc`, `Taml`, and `Mlym`. Much of the map uses full names instead of ISO script codes. Syriac engine matching does not repair the layout script tag. | [language.go](../otshape/language.go) |

### Additional limitations found by inspection

- **Attachment positioning has remaining limits.** Horizontal Latin/Hebrew
  mark-to-base, mark-to-ligature, and mark-to-mark offsets are finalized from
  design-unit anchors and final advances (SHAPE-007). Cursive resolution,
  hint/device/variation adjustments, and ligature component association remain
  incomplete. Context lost at streaming/event boundaries is separate. See
  [mark_attachments.go](../otshape/mark_attachments.go) and
  [gpos.go](../otlayout/gpos.go).
- **Streaming safety is incomplete.** Cuts consult clusters and optional unsafe
  flags, but the production pipeline does not populate sufficient contextual
  safety information. When glyph count changes, partial flushing is deferred
  until EOF or a forced full-buffer flush. The raw buffer cap guarantees progress,
  not equivalence to whole-run shaping. See `findFlushCut` and
  `rawFlushForGlyphPrefix` in [shape_streaming.go](../otshape/shape_streaming.go).
- **Normalization has bounded scope.** SHAPE-012/013 add font-supported
  decomposition, stable canonical ordering and starter-based recomposition with
  cmap checks. Hebrew uses modified combining classes under SHAPE-023. This is
  not full script-specific HarfBuzz normalization conformance; safe normalization
  across streaming cuts remains separate. Event scopes share normalization. See
  [normalize.go](../otshape/normalize.go).
- **Script support is selective.** Arabic joining uses a hand-maintained
  classification and selected fallback operations. Hebrew implements selected
  composition/reordering cases. Their presence and unit coverage do not establish
  full script conformance.

## What the existing tests establish

### Font discovery, provenance, and actual fixture use

[`tests-found.txt`](../testdata/scripts/tests-found.txt) is a saved discovery
report from [`find_test_fonts.py`](../testdata/scripts/find_test_fonts.py), not
a test manifest or a list of executed tests. The script opens fonts with
FontTools, indexes their GSUB/GPOS lookup types, and prints counts and example
filenames. Its "Coverage" heading describes available lookup data, not this
repository's test coverage. No Go test reads this report to discover cases;
the tests select fixtures explicitly.

The original local FontTools checkout was found at
`/Users/npi/prg/python/fonttools`, with fixtures under
`Tests/ttLib/tables/data/aots`. On 2026-10-26, its HEAD was
`9cec77d49bdb1a1ca346ac5fefdc5e7c30929026` and its origin was
<https://github.com/fonttools/fonttools>. The original 24 files in our
[`testdata/fonttools/`](../testdata/fonttools/) directory were byte-for-byte
identical to their counterparts there: 12 fonts and 12 TTX files. This is a
verified local provenance reference, not a runtime dependency or proof of the
exact historical download command.
The later [pair batch](../testdata/harfbuzz/aots/PAIR.md) imports two additional
fonts directly from the pinned HarfBuzz archive. Neither is listed in the
historical `tests-found.txt`; neither adds a TTX comparison file.

The upstream fixture README identifies
[Adobe's Annotated OpenType Specification (AOTS)](https://github.com/adobe-type-tools/aots)
revision `1c41fd20d2b020177625541a228c4c7c934879ef` as the source of the `.otf`
fonts. It says they were built with `make` and copied from `tests/*.otf`;
the original AOTS XML was not copied. The FontTools checkout contains 206 AOTS
fonts, with TTX references including 94 GSUB, 93 GPOS, and 148 GDEF files.

The Python driver is FontTools' `Tests/ttLib/tables/tables_test.py`, using
pytest. It checks binary-to-TTX output and XML round-tripping against stored
TTX. Those are font-table reading/serialization tests, not Unicode-to-glyph
shaping tests. No HarfBuzz-style `*.tests` shaping case files were found under
that checkout's `Tests/` directory.

The original TTX inventory from 2026-10-26 (before the pair-font additions) is:

| Item | Count |
| --- | ---: |
| Distinct font filenames in `tests-found.txt` | 31 |
| Listed fonts copied into `testdata/fonttools/` | 12 |
| Copied fonts referenced by Go tests | 12 |
| Copied TTX files consumed by structural comparisons | 9 of 12 |
| Listed fonts not copied into this repository | 19 |

The following map accounts for the 12 original copied fonts. Names below omit `.otf`;
each has a paired `.ttx.GSUB` or `.ttx.GPOS` file. The consumers listed are
representative; additional tests reuse some fonts for errors, bounds, masks,
and streaming.

| Font(s) | Structural TTX comparisons | Lookup execution / public shaping examples |
| --- | --- | --- |
| `gsub3_1_simple_f1` | GSUB structural and concrete comparisons | Alternate selection in `otlayout/feature_functional_test.go`; public GSUB in `otshape/otcore/otcore_test.go`; ordinary shared-API and AOTS range/value regressions; ordinary SHAPE-018 crossing case |
| `gsub3_1_lookupflag_f1` | GSUB structural and concrete comparisons | Ordinary AOTS IgnoreBaseGlyphs regression (SHAPE-005); no corresponding case in `feature_functional_test.go` |
| `gsub_chaining2_next_glyph_f1` | Selected single/ligature lookups in both GSUB comparison suites | No corresponding execution case in `feature_functional_test.go` |
| `gsub_context1_lookupflag_f1` | Selected contextual lookup in both GSUB comparison suites | `feature_functional_test.go` exercises the IgnoreMarks flag without interleaved marks; pipeline/edit tests; ordinary AOTS SHAPE-005 regression adds interleaved marks |
| `gsub_context1_next_glyph_f1`, `classdef2_font4` | Selected contextual lookups in both GSUB comparison suites | AOTS adapter: ordinary SHAPE-006 progression regression and passing no-change class baseline, respectively |
| `gpos_chaining3_boundary_f2` | Selected single, pair, and chained-context lookups in both GPOS comparison suites | Single/pair execution in `otlayout/gpos_runtime_golden_test.go`; passing AOTS chained single-positioning baseline (not pair positioning) |
| `gpos4_simple_1`, `gpos5_font1` | Mark-to-base and mark-to-ligature comparisons | Attachment execution in `otlayout/gpos_runtime_golden_test.go`; all five AOTS mark-origin/advance records pass after SHAPE-019 |
| `gpos3_font1`, `gpos3_font2`, `gpos3_font3` | Their TTX files are currently unused by the comparison tests | Cursive attachment cases in `otshape/otcore/gpos_test.go` |

The later additions `gpos2_1_simple_f1` and `gpos2_1_lookupflag_f2` bring the
current mini-font total to 14. They have no structural TTX comparisons: the
first supplies two passing AOTS pair cases, and the second a tagged pair-advance
case for SHAPE-005 (promoted on 2026-10-29). The missing-type/format TTX inventory below is unchanged.

The test entry points are:

| File | Responsibility |
| --- | --- |
| [`internal/ttxtest/parse_ttx_gsub_test.go`](../internal/ttxtest/parse_ttx_gsub_test.go) | One self-test of the TTX-reading helper. It reads the alternate fixture's XML; it does not load its binary font or shape text. |
| [`ot/ttx_gsub_compare_test.go`](../ot/ttx_gsub_compare_test.go) | Seven structural GSUB tests comparing binary parsing with TTX-derived expectations. |
| [`ot/ttx_gpos_compare_test.go`](../ot/ttx_gpos_compare_test.go) | Five structural GPOS tests comparing binary parsing with TTX-derived expectations. |
| [`ot/parse_concrete_golden_ttx_test.go`](../ot/parse_concrete_golden_ttx_test.go) | Two table-driven tests with seven GSUB and five GPOS subcases, covering the concrete lookup representation. These overlap the structural cases, rather than adding twelve distinct scenarios. |
| [`otlayout/feature_functional_test.go`](../otlayout/feature_functional_test.go) | Applies GSUB lookups to glyph buffers; tests alternate selection and contextual substitution with the IgnoreMarks flag, but without interleaved marks. |
| [`internal/hbcmp/aots_gsub_test.go`](../internal/hbcmp/aots_gsub_test.go) | Adapts five upstream GSUB records through public shaping; verifies source/font hashes, maps global/single-index features, and compares recorded glyph IDs. All five and the additional streaming execution are ordinary regressions. |
| [`internal/hbcmp/aots_gpos_test.go`](../internal/hbcmp/aots_gpos_test.go) | Adds origin parsing/comparison to the shared AOTS loader/runner. All nine selected GPOS records pass, including the SHAPE-019 promotion; the obsolete tagged wrapper was removed. Includes matching and nonmatching pair cases. Arithmetic/schema tests validate the adapter separately. |
| [`otlayout/gpos_runtime_golden_test.go`](../otlayout/gpos_runtime_golden_test.go) | Applies positioning lookups and checks single/pair adjustments and attachment metadata. `mark_attachments_test.go` additionally checks anchor placement and its lifecycle for GPOS 4/5/6. |
| [`otshape/otcore/otcore_test.go`](../otshape/otcore/otcore_test.go), [`gpos_test.go`](../otshape/otcore/gpos_test.go) | Exercises the public shaping API, including the alternate and cursive mini-fonts; other cases use larger fonts. |

For example, `gsub3_1_simple_f1` is used to test the XML helper, binary GSUB
parsing, lookup execution on glyph IDs, and public shaping from Unicode input.
The helper's single test therefore does not represent the extent of fixture
testing elsewhere in the repository.

The 19 listed but uncopied fonts include `gsub3_1_multiple_f1`,
`gsub_context1_lookupflag_f2`, `gsub7_font1/2`, `gpos6_font1`, `gpos9_font1`,
and all five listed `gpos_context*` fonts. The remaining entries are
`gpos4_multiple_anchors_1`, `gpos4_lookupflag_f1/f2`, `Lobster.subset.otf`,
`TestVGID-Regular.otf`, `I.otf`, `NotoSans-VF-cubic.subset.ttf`, and
`Amstelvar-avar2.subset.ttf`. Absence of these particular fixtures does not
establish absence of lookup-type coverage: other tests construct synthetic
lookup data.

We have therefore adopted a selected subset of the fonts and written Go tests
around them, rather than imported the complete upstream test suite. The
corresponding HarfBuzz/AOTS source cases have now been recovered and their font
versions verified; see [the recovery record](../testdata/harfbuzz/aots/README.md).
The [adapter](../testdata/harfbuzz/aots/ADAPTER.md) now executes five GSUB and
nine GPOS records through `internal/hbcmp`; the remaining records await adaptation.
Keep their upstream provenance and distinguish those behavioral expectations
from the TTX structure checks.

### Structural parsing

The TTX path loads a mini-font with `ot.Parse(data, ot.IsTestfont)`, parses its
paired XML with `internal/ttxtest`, and compares selected structures.
`IsTestfont` relaxes font completeness requirements for reduced fixtures while
retaining validation of present data. Assertions include lookup types/formats,
coverage, substitution records, classes, and selected positioning/anchor data.

See `ot/ttx_*_compare_test.go` and `ot/parse_concrete_golden_ttx_test.go`.
Synthetic matrices in `ot/parse_concrete_golden_synth_test.go` and real-font
checks in `ot/parse_concrete_golden_fonts_test.go` extend structural coverage
beyond the TTX subset. Structural coverage is distinct from lookup-runtime or
end-to-end shaping coverage.

#### TTX coverage by lookup type and format

This inventory was checked against the selected lookup indexes, comparison
functions, and binary fixture subtable formats on **2026-10-26**. "Covered"
means an active test compares that lookup's type-specific data with a TTX-derived
expectation. It does not mean every variation or field is covered. Lookup types
are distinct from feature tags such as `liga`, `kern`, or the fixtures' `test`;
these are structural tests of lookup data, not feature-planning tests.

Both the structural and concrete comparison suites exercise the same type/format
subset. Merely containing a lookup in a loaded font, or having a comparison
helper capable of handling it, does not count as a TTX assertion.

| GSUB type | Operation | Formats with active TTX assertions | Formats still missing | Selected fixture and lookup index (zero-based) |
| --- | --- | --- | --- | --- |
| 1 | Single substitution | 1 | 2 | `gsub_chaining2_next_glyph_f1`, lookup 0 |
| 2 | Multiple substitution | None | 1 | Lookup 3 exists in several copied GSUB fonts, but is not selected for comparison |
| 3 | Alternate substitution | 1 | — | `gsub3_1_simple_f1` and `gsub3_1_lookupflag_f1`, lookup 0 |
| 4 | Ligature substitution | 1 | — | `gsub_chaining2_next_glyph_f1`, lookups 1 and 2 |
| 5 | Contextual substitution | 1, 2 | 3 | Format 1: `gsub_context1_lookupflag_f1` / `gsub_context1_next_glyph_f1`, lookup 4; format 2: `classdef2_font4`, lookup 3 |
| 6 | Chained contextual substitution | None | 1, 2, 3 | Format 2 is present at lookup 4 of `gsub_chaining2_next_glyph_f1`, but is not selected |
| 7 | Extension substitution | None | 1, including wrapped lookup data | No selected fixture |
| 8 | Reverse chained single substitution | None | 1 | No selected fixture |

An earlier summary said single substitution covered both delta and explicit-map
formats. That overstated the TTX coverage: the selected binary lookup is format
1 only. The helper has a format-2 comparison branch, but no active TTX case
exercises it. TTX `SingleSubst` represents mappings without necessarily retaining
the binary format; `normalizeSingleSubst` currently infers format from the
mapping. A new format-2 case must verify the intended binary encoding as well.

| GPOS type | Operation | Formats with active TTX assertions | Formats still missing | Selected fixture and lookup index (zero-based) |
| --- | --- | --- | --- | --- |
| 1 | Single adjustment | 1 | 2 | `gpos_chaining3_boundary_f2`, lookup 0 |
| 2 | Pair adjustment | 1 | 2 (class pairs) | `gpos_chaining3_boundary_f2`, lookup 1 |
| 3 | Cursive attachment | None | 1 | `gpos3_font1/2/3` and paired TTX are already copied, but the TTX files are unused |
| 4 | Mark-to-base attachment | 1 | — | `gpos4_simple_1`, lookup 0 |
| 5 | Mark-to-ligature attachment | 1 | — | `gpos5_font1`, lookup 0 |
| 6 | Mark-to-mark attachment | None | 1 | No selected fixture |
| 7 | Contextual positioning | None | 1, 2, 3 | No selected fixture |
| 8 | Chained contextual positioning | 3 | 1, 2 | `gpos_chaining3_boundary_f2`, lookup 4 |
| 9 | Extension positioning | None | 1, including wrapped lookup data | No selected fixture |

Thus **4 of 8 GSUB lookup types** and **5 of 9 GPOS lookup types** have some
TTX-backed assertions. These counts are not measures of overall parser or
shaper correctness. In particular, runtime cursive tests and synthetic tests
for additional formats do not fill the TTX-specific gaps above.

#### Work left open in the TTX test infrastructure

Adding font files alone will not close these gaps. The XML expectation readers
and the Go comparison dispatchers also need extension:

- [`ParseTTXGSUB`](../internal/ttxtest/parse_ttx_gsub.go) currently models single,
  alternate, ligature, and contextual formats 1/2. Multiple, chained contextual,
  extension, reverse chained, and contextual format 3 need expectation support
  and matching assertions. Its introductory comment claiming only alternate
  support is stale; the implementation is broader.
- [`ParseTTXGPOS`](../internal/ttxtest/parse_ttx_gpos.go) currently models single
  format 1, pair format 1, mark-to-base, mark-to-ligature, and chained-context
  format 3. Each missing type/format in the GPOS table requires reader and
  comparison support. Its introductory support list also omits implemented
  mark-to-base and mark-to-ligature handling.
- Make unsupported selected cases fail explicitly. The XML readers can omit
  unmodeled subtable elements, and the comparison switches have no default
  rejection for unsupported lookup types. A newly added case must assert its
  expected subtable count and payload, not merely pass through a dispatcher.
- Audit common assertions while extending the suite: lookup flags are currently
  compared only when the expected flag is nonzero. A zero expectation should
  also be checked. Existing tests do not constitute an exhaustive matrix of
  coverage/class encodings, lookup flags, anchor formats, or device data.

For Latin/Hebrew needs, useful next TTX additions are GPOS single format 2,
class-pair format 2, and mark-to-mark; GSUB single format 2, multiple substitution,
and missing contextual/chained formats; and extension wrappers for both tables.
The local AOTS collection has candidate pairs such as `gsub1_2_simple_f1`,
`gsub2_1_simple_f1`, `gpos1_2_font1`, `gpos2_2_font1`, `gpos6_font1`,
`gsub7_font1`, and `gpos9_font1`. Confirm each candidate's actual lookup format
before importing it. No GSUB-8 TTX case was found in that local AOTS directory;
it needs another fixture source or a deliberately prepared font and TTX pair.
Cursive TTX comparisons can reuse the three already-copied pairs, but remain
lower priority under the current script focus.

Run the existing TTX comparisons and helper self-test with:

```sh
go test ./ot ./internal/ttxtest -run 'Test(TTX|ParseTTX)' -count=1 -v
```

This command passed at the inventory date. It validates the selected cases,
not the missing rows. The structural inventory itself added no tests or fixes;
the later AOTS adapter adds behavioral tests, not structural TTX comparisons.

### Lookup behavior and pipeline mechanics

`otlayout/feature_functional_test.go` supplies `loadTestFont` and
`applyGSUBLookup`. The latter selects a concrete lookup by index, seeds a glyph
and position buffer, and runs the concrete dispatcher. Embedded vectors cover
alternate selection and contextual substitution with the IgnoreMarks flag,
but those vectors contain no interleaved marks. The upstream baseline in
SHAPE-005 supplies that missing behavior and now passes. The reviewed
[SHAPE-006 regressions](SHAPING_FINDINGS.md#shape-006--contextual-lookup-revisits-matched-input)
verify non-overlapping contextual continuation, including nested glyph edits.
Other
`otlayout` tests cover synthetic substitutions, positioning, extensions, nested
errors, bounded recursion/work, and partial edits.

In `otshape`, `plan_test.go` covers selection, ordering, stages, masks, joiners,
and syllable isolation. The `shape_*_test.go` files cover entry points, hooks,
events, normalization probes, flushing, and source/sink behavior.
`successful_edits_test.go`, `partial_progress_test.go`,
`output_bounds_test.go`, and run-buffer tests cover metadata correspondence and
failure semantics.

Ordinary regressions in
[`otcore/shared_api_test.go`](../otshape/otcore/shared_api_test.go) verify that
engine-customized masks survive execution through both public APIs, checking
GIDs, clusters, and disabled/enabled masks. `plan_test.go` also verifies stable
engine feature-bit allocation with user precedence and disabled features.

Ordinary [alternate-selection regressions](../otshape/otcore/alternate_values_test.go)
cover both public APIs, overlapping ranges, nested scopes, default/one-based/
random/oversized values, and cluster preservation. The
[lower-level tests](../otlayout/feature_functional_test.go) verify the same
selection contract with fixed-seed random draws; internal
[carry and mask tests](../otshape/alternate_values_test.go) check repeatability
across rebuilt spans/event plans and retention of large values. The unchanged
AOTS alternate oracle passes in whole-input and streaming configurations.
See the [reviewed SHAPE-003 resolution](SHAPING_FINDINGS.md#shape-003--feature-values-do-not-select-alternates).
The [reviewed SHAPE-018 fix](SHAPING_FINDINGS.md#shape-018--forward-lookup-scans-bypass-feature-masks)
anchors forward starts at the checked glyph: the range-crossing
regression is ordinary, and fixed-seed tests check the matched source anchor
with and without an uncovered prefix. Direct/extension/nested GSUB and GPOS
checks live in [lookup_start_test.go](../otlayout/lookup_start_test.go).
This guarantee concerns lookup starts; matched contextual rules may still touch
later glyphs, and nested calls inherit the enclosing value and random stream.
An explicit ignored-start case, especially for format-3 contexts, remains an
optional coverage improvement; review verified that branch by inspection.

The engine subpackages add public-API and script unit tests. However, Arabic
streaming parity deliberately uses non-joining digits; Hebrew streaming parity
uses plain letters. Neither establishes joining/mark behavior across boundaries.
Comparing small buffers with the same implementation at default buffering also
allows shared bugs to pass. A whole-input baseline must use thresholds larger
than the input; default buffering is not always one-shot.

### Independent expected output

The upstream recovery step covers the original 12 mini-fonts and two added pair fonts.
[`testdata/harfbuzz/aots/`](../testdata/harfbuzz/aots/README.md) preserves 13
unchanged test files from HarfBuzz revision
`29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b`; every local font is byte-identical
to its upstream counterpart. Of the 35 records, 21 are active cases using our
fonts, 9 are upstream-disabled cases using our fonts, and 5 reference fonts
outside our subset. The manifest preserves file/line identities, hashes, and
these distinctions. Recovery did not execute HarfBuzz or our shaper on these
records, and disabled upstream records are not findings about our implementation.

The recovered expectations omit clusters; GSUB records also omit positions,
while GPOS records contain glyph origins with advances suppressed. They cannot
be treated as complete glyph/cluster/advance/offset JSON. The current adapter
supports glyph-only GSUB and glyph-origin GPOS expectations with global and single-index feature
requests, preserving one-based positive values and mapping HarfBuzz zero to
disabled. Broader feature syntax remains unimplemented in this adapter;
richer expectations require a separate
capture with recorded HarfBuzz provenance (as in the text batch below). Missing expected fields must not be filled
from our own shaper. See the recovery README for option semantics, feature ranges, licensing,
and reproducible recovery commands.

The [first AOTS batch](../testdata/harfbuzz/aots/ADAPTER.md) selected three active
records and ran public `Shaper.Shape` with the core engine in a single shaping
window. `classdef2.tests:2` passes with an unchanged glyph sequence, and
`gsub_context1_lookupflag.tests:2` now passes after SHAPE-005.
`gsub_context1_next_glyph.tests:2` now passes after SHAPE-006. Script/language/
direction are explicitly Latin/English/LTR. Both recorded and explicit-context
commands reproduced the expected output using installed HarfBuzz 15.0.0;
the pinned upstream records remain the oracle. No external executable is
required by the Go tests. That initial batch supplied no positioning, cluster,
Hebrew, or streaming baseline. The alternate extension adds two
active records: `gsub3_1_simple.tests:2` now passes whole-input and small-buffer
execution after SHAPE-002/003, promoted with unchanged expectations;
`gsub3_1_lookupflag.tests:2` now passes after SHAPE-005, exercising IgnoreBaseGlyphs
and alternate values. Both retain exact upstream expectations.
Adapter mapping tests separately check scalar range boundaries and value
translation; they do not extend the upstream oracle to random or out-of-range
alternate selection.

The [GPOS batch](../testdata/harfbuzz/aots/GPOS.md) adds six active records.
The chained single-positioning case passes; mark-to-base/mark-to-ligature
cases now pass after SHAPE-007/008, as do the two unattached-mark controls.
The formerly tagged SHAPE-019 record now also passes: its ignored mark survives
and attaches to the first ligature component. Origins are compared as preceding emitted advances plus the
current offset, in font units; the adapter does not resolve attachments itself.
These assertions do not independently validate every advance/offset or the
final run advance. The fixture's pair lookups are not reached by its public
feature. The later [pair batch](../testdata/harfbuzz/aots/PAIR.md) uses two new
fonts with `test` selecting their sole pair lookup (type 2 format 1). Matching
18/19, nonmatching 18/20, and incomplete end-of-input cases pass. The separate
advance-changing pair record now passes after SHAPE-005. It retains the upstream
−200 advance effect on trailing origins and checks the pair across an ignored
base glyph. The later Latin `AVATAR` captures also pass with adjusted advances.
Cursive baselines remain deferred.

The original JSON path in `internal/hbcmp` runs `Shaper.Shape` with all three
engines and API-default buffering. It still loads only the top-level
`Gentium_AB.json`; it does not recurse into the new text corpus.

Each fixture has `context` (font, ISO script, language, direction, optional
feature strings and `testfont`), Unicode scalar `input`, and expected `output`
records with `g`, `cl`, `dx`, `dy`, `ax`, and `ay`. Use the existing JSON
as the schema example. The declared `normalization` field is not applied by the
original runner. There is no watermark/event configuration in that format, and internal masks, unsafe
flags, and unresolved attachments are not compared.

The [real-text corpus](../internal/hbcmp/testdata/text/README.md) contains 35
independent captures from installed HarfBuzz 15.0.0, with exact commands,
executable and font hashes, script/language/direction, scalar cluster indexes
at level 0, and font-unit positions. The test-only runner validates every font
and complete output record, uses core/Hebrew engines and explicit whole-input
watermarks `n+1/1/n+2`. It copies and reverses RTL oracle records to undo HB's
final visual-order reversal, preserving positions and clusters. It never sorts
by cluster, resolves attachments, or modifies raw captures. Logical sink order
and HarfBuzz level-0 grouping are the agreed contracts. See the
[normalization report](../internal/hbcmp/testdata/text/NORMALIZATION.md) for the
conversion's rendering convention and exact cases.

Thirty-four text cases pass, including both original pointed-Hebrew regressions
promoted unchanged under SHAPE-023. Only the explicit no-GPOS parser restriction
remains tagged in this corpus. The separate Hebrew ordering corpus adds nine
captures; the focused test runs those plus three original/control cases through
both APIs. The additional normalization corpus and coverage are described above.

`otshape/clusters_test.go` covers leading marks, adjacent groups, original scalar
anchors through composition/decomposition, range starts/ends inside groups,
ligature edits with surviving marks, copy/reorder/append/reuse, and normalization
scratch isolation across event spans. Public `Shape` tests check the range rule;
both APIs exercise unchanged HarfBuzz text in the streaming matrix. Random
alternate regressions verify that grouping does not replace source anchors in
the seed. No capture or expected output field is weakened.

Focused attachment regressions in `otlayout/mark_attachments_test.go` and
`otshape/mark_attachments_test.go` verify anchor replacement, subsequent value
placement, attachment-time Y-offset saturation, partial failure, edit remapping,
mark-to-mark chains, lookup ordering,
LTR/RTL pen compensation, final advances, long chains, invalid targets, and
idempotence. `otshape/otcore/mark_attachments_test.go` checks full positions
through `Shape` and `ShapeEvents` in both directions. These synthetic cases do
not establish arbitrary ligature component association or safe streaming.

SHAPE-002's ordinary public-API regression verifies source ranges after streaming
compaction; `latin_range_after_nfd` verifies them after composition against the
complete HarfBuzz capture. Unit coverage also checks decomposition correspondence,
reordered anchors, range bounds, and existing mask inheritance through glyph edits.
SHAPE-011 subsequently settles indexed ranges inside initial base/mark groups:
select by the group start; starts inside exclude it and ends inside include it.
SHAPE-016 now applies the same initial-group rule to feature events within a
buffered carry, with independent Latin/Hebrew regressions.

The [streaming/scope matrix](../internal/hbcmp/testdata/text/STREAMING.md) contains
thirteen independent captures and reuses four original text cases. Five buffer
profiles × both public entry points × both flush modes yield 340 executions:
224 pass, 116 remain tagged. The runner validates actual push/pop schedules
against the equivalent HB ranges at every scalar and checks complete output,
EOF consumption, early emission, and bounded source/sink activity. Ten additional
empty-input and balanced empty-scope checks pass. Remaining matrix failures
cover streaming context/capacity (SHAPE-015); SHAPE-016 scope context is resolved.
SHAPE-011 promoted 32 mark cells unchanged; eight tiny-buffer mark cells remain
under SHAPE-015 with complete cluster/position expectations.
SHAPE-002 promoted eight matrix cells; remaining tiny/forced range failures
retain full expectations under SHAPE-015. The AOTS streaming range case now
passes after SHAPE-003 and is an ordinary regression. The reviewed SHAPE-018 fix
promoted eight more complete matrix cells: whole nested scopes and whole/lookahead/carry
accented scopes, rune input with both flush modes. Eight remaining tiny/forced
scope cells retain their full expectations under SHAPE-015.
Run with `-timeout=30s` to bound loops without source/sink activity. Actual
zero-output progress and explicit capacity errors are covered by SHAPE-017;
whole/lookahead/carry deletion captures pass unchanged.

Go tests require neither HarfBuzz nor Python/FontTools. HarfBuzz is needed only
for capture/check preparation; FontTools is needed only to derive/check the
three modified fonts and the synthetic progress font. Tool versions and font
hashes are recorded.
The capture executable hash identifies the installed tool, not a reproducible
build. The combined corpora cover selected streaming cuts and feature events; they
do not establish full Latin/Hebrew conformance or paragraph bidi processing.

### Measured snapshot

The focused review command was:

```sh
go test ./otshape/... ./otlayout ./internal/ttxtest -count=1 -coverprofile=/tmp/otshape-review.cover
```

| Package | Passing top-level tests | Skipped | Statement coverage |
| --- | ---: | ---: | ---: |
| `otshape` | 83 | 1 | 75.0% |
| `otshape/otcore` | 11 | 0 | 60.0% |
| `otshape/otarabic` | 23 | 0 | 87.5% |
| `otshape/othebrew` | 9 | 0 | 53.6% |
| `otlayout` | 53 | 0 | 56.5% |
| `internal/ttxtest` | 1 | 0 | 9.5% |

Counts exclude subtests; the skipped shaping test is `TestBufferFunctions`,
which was later removed together with the legacy `buffer.go`.
Coverage is per package in this invocation, without `-coverpkg`. In particular,
the helper's own coverage does not reflect its use by tests in `ot/`. The active
decomposition routine had 0% coverage in this run. Executed statements do not
establish correctness or conformance. The separate full repository run passed,
including the HarfBuzz fixture test. Race and benchmark results are not part of
this review snapshot.

## Running and extending tests

Run from the repository root:

```sh
# Shaping, lookup execution, and stored reference output
go test ./otshape/... ./otlayout ./internal/hbcmp -count=1

# Parsing, TTX helpers, cmap, and metrics
go test ./ot ./otquery ./internal/ttxtest -count=1

# Coverage and full repository checks
go test ./otshape/... -cover
go test ./...

# Known-bug discovery suite: expected to fail while recorded findings are open
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1

# Optional concurrency and allocation checks
go test -race ./otshape/... ./otlayout ./internal/hbcmp
go test ./otshape -run '^$' -bench 'Benchmark(LookupApplication|ShapeOrdinary|PositionReplacement)$' -benchmem
```

Go tests consume checked-in fonts, TTX dumps, JSON, and AOTS records; they require no Python,
FontTools, or HarfBuzz executables at runtime. Those tools are useful for fixture
preparation. FontTools and HarfBuzz test-font collections are sources of small
fixtures. `testdata/scripts/find_test_fonts.py` requires Python FontTools and
currently calls `main("..")`; adjust its root for the collection being inspected
rather than assuming it accepts a CLI path argument.

Prefer small fonts with few glyphs and a deliberately limited lookup set. For a
new regression:

1. Add or reuse a font in `testdata/fonttools/`, retaining provenance/licensing.
   Keep paired TTX dumps beside it when structural assertions are needed.
2. Add an `ot` or `otlayout` test isolating parsing or lookup behavior. Specify
   lookup indexes and input/output glyph IDs explicitly.
3. Add a public-API `otshape` test for planning, masks, normalization, hooks,
   streaming, or output materialization. Vary watermarks and assert clusters and
   positions as well as glyph IDs.
4. Capture independent output with HarfBuzz and store JSON under
   `internal/hbcmp/testdata/`. Record the generating command/version and ensure
   numeric glyph IDs, cluster units, font scale, features, and direction agree
   with the runner. Keep expected output offline so tests need no new external
   dependency.

Embedded vectors remain appropriate for isolated lookups. The earlier proposal
for functional sidecars next to every mini-font is not implemented as a general
loader: the existing end-to-end loader reads `internal/hbcmp/testdata/`.
Extend that convention deliberately rather than placing undiscovered JSON beside
fonts.

For a known failure, follow the discovery and promotion process in
[SHAPING_FINDINGS.md](SHAPING_FINDINGS.md#process-and-scope): assert desired
behavior in the tagged suite, record evidence, and move the regression into the
ordinary suite once fixed. Keep unresolved API choices explicit. The broader
roadmap belongs in [TO_DO.md](TO_DO.md); detailed reproductions belong in the
findings file.

For manual inspection:

```sh
go run ./ot-tools shape testdata/fonts/GentiumPlus-R.ttf AB
```

The CLI also accepts feature, codepoint, and watermark flags, but its feature
parser does not accept indexed range syntax. See [OT_TOOLS.md](OT_TOOLS.md) for
`shape`, `view`, and `font`, and [SHAPING_BENCHMARKS.md](SHAPING_BENCHMARKS.md)
for benchmark scope and historical results. The ordinary-shaping benchmark is a
small fixture, not a representative complex-script workload.

## Next testing priorities

The current focus is Latin-script text and Hebrew. Arabic/Syriac and other
scripts are deferred to a later phase. Shared pipeline defects remain in scope
where they affect Latin or Hebrew; use fixtures from those scripts or small
synthetic cases to isolate them. Existing tests for deferred scripts should
continue to pass, but expanding their coverage is not a current priority.

1. **Shared API correctness.** Engine selection remains tagged; exact forward
   lookup starts and alternate selection now have ordinary regressions for
   one-based/default/random/oversized values, ranges, and event scopes. Track
   review status and dependencies in [SHAPING_FINDINGS.md](SHAPING_FINDINGS.md).
   Continue discovery with
   Latin/Hebrew script and language selection; defer expanding the script map
   and regressions specific to other scripts.
2. **Independent Latin/Hebrew baselines.** The first AOTS GSUB adapter batch
   contributes a passing no-change case and interleaved IgnoreMarks case;
   contextual progression now also passes after SHAPE-006. Both alternate records
   now pass, including the simple record's additional streaming execution.
   All six GPOS origin records pass: one chained single-positioning baseline
   and five mark-origin/advance records, including SHAPE-019 preservation and
   component association.
   All three pair records pass, including
   the flagged advance-changing case. The 35 text captures and ten additional
   ligature-component captures exercise Latin ligatures,
   kerning, accents, source ranges after normalization, and Hebrew mark/fallback
   behavior. Logical output and level-0 grouping contracts are settled.
   Fix documented blockers in separate sessions and promote passing baselines.
   Add isolated class-based pair coverage; the AOTS origins omit individual advances.
   Extend the adapter to further recovered cases, explicitly handling their
   feature options and available expected fields. Expand the corpus to
   additional Latin/Hebrew feature values/ranges, languages, and combining
   sequences beyond the initial words. Compare glyph
   IDs and order for glyph-only records. For richer independently captured
   baselines, also compare clusters, advances, and offsets.
3. **Normalization and cluster preservation.** The focused discovery batch now
   covers canonical-equivalent mark orders (SHAPE-012), missing composed cmap
   mappings (SHAPE-013), source ranges after composition (SHAPE-002), and
   default/enabled/disabled/absent Hebrew mark features. SHAPE-010 now resolves
   plan-wide mark support, including mixed settings and ranged enablement; its
   complete passing baselines run in the ordinary suite. SHAPE-011 implements
   Latin/Hebrew base/mark grouping and initial-group range bounds.
   SHAPE-012/013 now cover canonical ordering and font-supported representation
   choices; SHAPE-023 adds Hebrew mark ordering and composition blocking. Absence of
   GPOS itself remains outside normal parser scope (SHAPE-014, deferred).
4. **Remaining mark positioning.** SHAPE-007/008 now resolve design-unit
   mark-to-base/ligature/mark positioning and final mark advances for horizontal
   Latin/Hebrew. SHAPE-019 also resolves ignored-mark preservation and ligature
   component association; SHAPE-023 resolves the recorded Hebrew mark-ordering
   blockers in complete text baselines. Cursive and hint/device/variation work
   remain outside this scope.
5. **Streaming and feature-scope boundaries.** The initial
   [matrix and event/progress driver](../internal/hbcmp/testdata/text/STREAMING.md)
   are implemented: independent Latin/Hebrew and synthetic expectations cover
   ligatures, pairs, normalization, mark failures, nested push/pop restoration,
   compaction, EOF tails, expansion, contraction, and deletion. Five buffer
   profiles, both APIs, and both flush modes distinguish passing cells from
   remaining matrix failures (SHAPE-015). SHAPE-017 now covers deletion,
   zero-output completion at EOF, and explicit capacity errors before known EOF.
   The [agreed buffer contract](STREAMING_API.md#agreed-buffer-contract-2026-10-27-not-yet-implemented)
   permits a generous required minimum and practical sequence/context limits.
   When implemented, test explicit rejection below the minimum, specific errors
   on limit exhaustion, and exact output across boundaries in long inputs at
   supported buffer sizes. Tiny-buffer acceptance is not a future requirement.
   SHAPE-016 adds scoped events inside initial groups and equivalent indexed
   ranges; SHAPE-022 now prevents mark-to-mark attachment across a base. Further
   coverage can add scopes inside substitution-merged clusters, additional
   feature values, longer combining sequences, and broader buffer sweeps.

Arabic joining, Syriac shaping, script-specific cursive behavior, and broader
script coverage remain follow-up work after these Latin/Hebrew priorities.

Related design notes: [SHAPER_PLAN.md](SHAPER_PLAN.md),
[SHAPER_INTERFACES.md](SHAPER_INTERFACES.md),
[STREAMING_API.md](STREAMING_API.md), and
[PARTIAL_PROGRESS.md](PARTIAL_PROGRESS.md). Some describe intended behavior or
historical milestones; use implementation and tests to establish current
guarantees.
