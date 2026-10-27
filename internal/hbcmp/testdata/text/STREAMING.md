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

## What is now implemented

[streaming_baselines_test.go](../../streaming_baselines_test.go) applies the same
independent whole-text expectation to five buffer profiles, `Shape` and
`ShapeEvents`, and both `FlushOnRunBoundary` and `FlushOnClusterBoundary`.
It compares exact ordered GIDs, clusters, advances, and offsets. RTL oracle
records are converted to logical order by the existing `textLogicalOutput`;
the captured data remains unchanged. Small-buffer output is never used as an
oracle, and whole-versus-streaming equality is not the primary assertion.

[streaming.json](streaming.json) adds twelve independently captured records.
Four existing text records are reused as controls/known whole-run failures.
The total matrix is **16 cases × 5 profiles × 2 entry points × 2 flush modes =
320 executions: 142 passing and 178 tagged failures**. Ten separate empty-input
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
budget exhaustion occurred with the final fixtures. All 178 remaining failures are exact
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
indexed-range input can fail separately (SHAPE-018). The unrelated-feature
pair case fails even with whole-input buffering (SHAPE-016), distinguishing
scope segmentation from the ordinary streaming-context problem (SHAPE-015).

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
| `scope_nested` | whole, tiny, forced | tiny, forced |
| `scope_pair` | tiny, forced | all |
| `scope_accent` | all | tiny, forced |
| `progress_expand` | none | none |
| `progress_delete` | all | all |
| `progress_all_deleted` | all | all |
| `progress_contract` | tiny, forced | tiny, forced |
| `latin_office_liga_off` | none | none |
| `latin_avatar_kern_off` | none | none |
| `latin_uncomposable_mark` | all | all |
| `hebrew_vav_holam_mark_on` | all | all |

The repeated text cases are `office office office`, `AVATAR AVATAR`, three
decomposed `café` words, and `שלום שלום שלום`. Enabled whole-run results match;
the reused disabled-ligature/disabled-kerning controls pass all configurations.
The two mark cases already fail whole-run SHAPE-007/011 and are explicitly
assigned to existing findings, not presented as new streaming-only defects.

The [synthetic font](../../../../testdata/fonts/STREAMING.md) gives independent
expansion (`abababa`), deletion (`adadada`), all-deletion (`dddddd`), and
contraction (`afifiafifi`) expectations. Expansion passes all configurations;
contraction passes whole/lookahead/carry. Deletion is currently a no-op
(SHAPE-017). Consequently the all-deletion oracle is present, but execution of
the actual zero-glyph output branch remains blocked until deletion works.
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
# Ordinary tests, including 142 matrix executions and ten empty-input checks:
go test ./internal/hbcmp -count=1 -timeout=30s

# Complete discovery suite: 22 failing top-level tests, including five matrix groups.
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1 -timeout=30s

# Observe consumption/emission and one whole-input scope failure:
go test -tags shaping_knownbugs ./internal/hbcmp -run 'TestKnownBugSHAPE016FeatureScopeContext/scope_pair/whole/events/run$' -count=1 -timeout=30s -v

# Preparation only:
python3 testdata/scripts/capture_harfbuzz_streaming.py
python3 testdata/scripts/build_streaming_font.py  # recorded FontTools version required
```

The ordinary full suite passes. Five top-level tagged matrix groups cover SHAPE-015
(stream context), SHAPE-016 (scope context), SHAPE-017 (deletion), SHAPE-018
(masked forward scans), and the streaming extension of SHAPE-011. Their cells
are statically assigned by `streamingFinding`; no outcome-driven skip or buggy
expectation is used. Promote cells when their full independent comparison passes.

This completes the initial runner/event/progress setup. Remaining expansion
includes scopes inside merged clusters, additional feature values, longer
combining sequences, error/cancellation scenarios beyond the existing unit
tests, and broader buffer sweeps. Actual all-deletion progress and successful
final mark placement remain blocked by recorded production defects. See
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md).
