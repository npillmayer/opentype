# Streaming and feature-scope baseline matrix

Verified on 2026-10-26 against source revision
`f36813fa22e1b0795a66a4860975612f4c7ce9cc` plus this batch's tests, fixtures,
tools, and documentation. No production behavior changed.

**Later contract decision (2026-10-27):** a generous required minimum buffer
and practical sequence/context limits are now agreed; see the
[buffer contract](../../../../doc/STREAMING_API.md#agreed-buffer-contract-2026-10-27-not-yet-implemented).
The results below include the SHAPE-002 promotions described next. Once enforced, configurations below
the minimum will require explicit rejection tests, and boundary coverage must
also use long inputs at supported sizes. No assertions have been relaxed here.

**SHAPE-002 update (2026-10-27):** source-coordinate mask assignment promotes
`stream_range` and `scope_nested` rune-input executions with `lookahead`/`carry`
profiles in both flush modes: eight complete comparisons now pass. Tiny/forced
`stream_range` failures remain under SHAPE-015; SHAPE-018 remains open for the
other nested-scope profiles. No captured output or assertion changed.

**SHAPE-018 update (2026-10-29):** exact forward lookup starts promote eight
more complete rune-input comparisons: `scope_nested/whole` and
`scope_accent/whole`, `lookahead`, and `carry`, all in both flush modes.
The eight tiny/forced scope comparisons now reside under SHAPE-015, retaining
all expected fields. Their disabled ranges are respected, but enabled ligatures
are lost at cuts; the tiny accented case also splits normalization. Nested
output counts are 34/31 instead of 28, accented counts 16/14 instead of 13.
No captures, feature schedules, or assertions changed.

**SHAPE-017 resolution (2026-11-01):** empty GSUB 2 sequences
now delete glyphs. Twenty-four existing deletion comparisons are promoted
unchanged (whole/lookahead/carry, both APIs, both flush modes). One new `ddaadd`
capture adds twelve passing comparisons and verifies leading/trailing deletion,
clusters `[0,3]`, and kerning advances `[580,600]`. It uses the existing progress
font, same HarfBuzz 15.0.0 executable and capture options; all previous twelve
records and capture metadata are unchanged. The exact command is stored in JSON.

Tiny/forced full-output expectations remain under SHAPE-015: mixed deletion loses
context at cuts; all-deleted buffers now return `ErrZeroOutputBufferFull` when
capacity is reached before EOF is known. This conservative error, including
exact-cap input, has separate ordinary contract tests. The historical full-output
oracles are retained, without asserting that these small profiles are supported.
Both APIs retain provisional empty carry for more context and finalize it only
at EOF. Additional tests cover lookahead changing the empty result, scope closure,
and preservation of earlier output. No general streaming safety fix is claimed.

**SHAPE-016 resolution (2026-11-01):** feature events now share one
normalization and lookup run per buffered carry. Six `scope_pair` event cells
(whole/lookahead/carry, both flush modes) pass and are promoted unchanged. The
remaining four tiny/forced event cells retain the complete oracle under
SHAPE-015. New focused captures and the independent SHAPE-022 discovery are
recorded separately in [SCOPES.md](SCOPES.md). Existing captures are unchanged.
The fix is accepted and archived in
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md#shape-016--feature-scopes-sever-unrelated-lookup-context).

## What is now implemented

[streaming_baselines_test.go](../../streaming_baselines_test.go) applies the same
independent whole-text expectation to five buffer profiles, `Shape` and
`ShapeEvents`, and both `FlushOnRunBoundary` and `FlushOnClusterBoundary`.
It compares exact ordered GIDs, clusters, advances, and offsets. RTL oracle
records are converted to logical order by the existing `textLogicalOutput`;
the captured data remains unchanged. Small-buffer output is never used as an
oracle, and whole-versus-streaming equality is not the primary assertion.

[streaming.json](streaming.json) contains thirteen independently captured records.
Four existing text records are reused as controls and mark-context cases.
The total matrix is **17 cases × 5 profiles × 2 entry points × 2 flush modes =
340 executions: 224 passing and 116 tagged comparisons**. Ten separate empty-input
checks pass, including balanced nested scopes with no rune events. Ordinary
tests validate every captured record/font, including tagged cases.

| Profile | High / low / max | Purpose |
| --- | --- | --- |
| `whole` | n+1 / 1 / n+2 | Independent whole-input control, EOF before any output |
| `tiny` | 1 / 1 / 2 | Minimal context; boundaries inside ligatures, pairs, and normalization sequences |
| `lookahead` | 3 / 1 / 8 | Rolling compaction with room to grow |
| `forced` | 3 / 3 / 3 | Reach the hard cap, including length-changing runs |
| `carry` | 5 / 3 / 8 | Different alignment, retained suffix, and EOF tail |

`n` is the original scalar count. Profiles select limits, not guaranteed internal
cuts. Source/sink instrumentation logs scalar consumption at every emission and
whether EOF has been seen. When input exceeds the cap and output is nonempty,
the runner requires emission before all text has been consumed. It verifies
complete input consumption and EOF on every successful call. Exact output detects
duplication, loss, cluster renumbering, and unflushed tails. It does not equate
an emission with a particular internal safe-cut decision.

Source read and output budgets abort runaway I/O; recovered panics fail the
individual case. Use the documented Go timeout to catch loops that perform no
I/O—these callbacks cannot interrupt such a loop themselves. No hang, panic, or
budget exhaustion occurred with the final fixtures. The original 138 remaining failures were exact
output mismatches. These checks establish progress for tested inputs, not a
general termination proof.

## Feature-event schedules

The test driver constructs real rune/push/pop events. Parameters supply only
global defaults; indexed ranges never enter `ShapeEvents`. Before shaping, a
separate test-side stack checks the effective assignments against the recorded
HarfBuzz ranges **at every original scalar**. Push/pop events do not increment
source coordinates. This validates the schedule without using the production
plan stack or shaper output to construct expectations.

| Case | Text and equivalent HarfBuzz settings | Event schedule |
| --- | --- | --- |
| `stream_range` | Three `office` words; `liga=0,liga[8:11]=1` | Enable at 8, pop at 11; forms the middle `ffi` |
| `scope_nested` | Five `office` words; `liga=1,liga[0:21]=0,liga[7:14]=1` | Outer disable at 0; nested enable at 7; pop at 14 restores disable; pop at 21 restores root enable; empty disable/pop at 28 leaves root unchanged |
| `scope_pair` | `AVATAR`; `kern=1,liga=1,liga[1:2]=0` | Disable `liga` around V only; kern stays enabled across both boundaries |
| `scope_accent` | `e` + U+0301 + ` office office`; `liga=1,liga[3:10]=0` | Disable first office at scalar 3, pop at 10; normalization before the scope must not shift its source coordinates |

All scopes begin/end between complete clusters in their captured text. The
pair case deliberately places a scope boundary inside a kerning dependency;
changing an unrelated feature must not sever that dependency. Other cases use
plain rune events with the same global features as `Shape`.

The nested and accented schedules pass as event input with whole/lookahead/carry
profiles, including correct restoration and global cluster indexes. Equivalent
indexed-range input previously failed separately (SHAPE-018, now repaired for
forward starts). The unrelated-feature pair case now passes with whole/lookahead/
carry buffering after SHAPE-016. Its tiny/forced failures remain under SHAPE-015.

## Coverage and current results

Only failing profiles are listed below. Each listed result applies to **both**
flush modes; all unlisted profiles pass. All means whole/tiny/lookahead/forced/carry.
Changing flush mode alone does not repair any of these mismatches.

| Case | `Shape` failing profiles | `ShapeEvents` failing profiles |
| --- | --- | --- |
| `stream_ligatures` | tiny, lookahead, forced, carry | tiny, lookahead, forced, carry |
| `stream_kerning` | tiny, forced | tiny, forced |
| `stream_accents` | tiny, lookahead, carry | tiny, lookahead, carry |
| `stream_hebrew` | tiny, forced | tiny, forced |
| `stream_range` | tiny, forced | tiny, forced |
| `scope_nested` | tiny, forced | tiny, forced |
| `scope_pair` | tiny, forced | tiny, forced |
| `scope_accent` | tiny, forced | tiny, forced |
| `progress_expand` | none | none |
| `progress_delete` | tiny, forced | tiny, forced |
| `progress_all_deleted` | tiny, forced (capacity error) | tiny, forced (capacity error) |
| `progress_delete_edges` | tiny, forced | tiny, forced |
| `progress_contract` | tiny, forced | tiny, forced |
| `latin_office_liga_off` | none | none |
| `latin_avatar_kern_off` | none | none |
| `latin_uncomposable_mark` | tiny | tiny |
| `hebrew_vav_holam_mark_on` | tiny | tiny |

The repeated text cases are `office office office`, `AVATAR AVATAR`, three
decomposed `café` words, and `שלום שלום שלום`. Enabled whole-run results match;
the reused disabled-ligature/disabled-kerning controls pass all configurations.
After SHAPE-007/011, the two mark cases pass 32 complete matrix cells. Their
eight tiny-buffer cells still lose attachment context and remain tagged under
SHAPE-015; the original HarfBuzz expectations are unchanged.

The [synthetic font](../../../../testdata/fonts/STREAMING.md) gives independent
expansion (`abababa`), deletion (`adadada`), all-deletion (`dddddd`), and
contraction (`afifiafifi`) expectations. Expansion passes all configurations;
contraction and deletion pass whole/lookahead/carry. All-deletion now exercises
actual zero-output completion at EOF (SHAPE-017). The new `ddaadd` case covers
cluster inheritance after leading deletion and unchanged kerning after deletion.
Empty input and empty balanced scopes do pass; they are direct API identity
tests, not mislabelled HarfBuzz captures.

## Provenance and execution

[capture_harfbuzz_streaming.py](../../../../testdata/scripts/capture_harfbuzz_streaming.py)
reuses the text capture implementation, selecting HB 15.0.0, OpenType shaping
and font functions, font-unit scale, explicit script/language/direction,
scalar cluster indexes, and cluster level 0. Commands, tool hash, and font hashes
are recorded per the existing schema. Default invocation compares without writing;
`--write` explicitly refreshes the snapshot. Go tests need no external tool.

**Empty oracle serialization:** HB 15.0.0 prints a lone newline for an entirely
deleted run, even with JSON output selected. The capture tool recognizes exactly
that output only for `progress_all_deleted`, records the original newline as
`empty_output_serialization`, and stores an explicit `output: []`. Missing/null
output is rejected; other cases cannot silently acquire empty expectations.

```sh
# Ordinary tests, including 224 matrix executions and ten empty-input checks:
go test ./internal/hbcmp -count=1 -timeout=30s

# Complete discovery suite: four failing top-level groups (SHAPE-004/014/015/023).
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1 -timeout=30s

# Observe consumption/emission for the promoted whole-input scope regression:
go test ./internal/hbcmp -run 'TestHarfBuzzStreamingBaselines/scope_pair/whole/events/run$' -count=1 -timeout=30s -v

# Preparation only:
python3 testdata/scripts/capture_harfbuzz_streaming.py
python3 testdata/scripts/build_streaming_font.py  # recorded FontTools version required
```

The ordinary full suite passes. One tagged matrix group covers SHAPE-015
(stream context/capacity). The separate [scope corpus](SCOPES.md) now has all
44 comparisons passing after SHAPE-022.
The obsolete SHAPE-016 wrapper is removed. SHAPE-018 forward-scan cells were promoted
or retained under SHAPE-015 as described above. The remaining cells
are statically assigned by `streamingFinding`; no outcome-driven skip or buggy
expectation is used. Promote cells when their full independent comparison passes.

This completes the initial runner/event/progress setup. Remaining expansion
includes scopes inside substitution-merged clusters, additional feature values, longer
combining sequences, error/cancellation scenarios beyond the existing unit
tests, and broader buffer sweeps. All-deletion completion at EOF and explicit
zero-output capacity errors are covered by the resolved SHAPE-017. Context-safe
streaming mark placement remains blocked by recorded production defects. See
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md).
