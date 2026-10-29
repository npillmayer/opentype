# AOTS baseline adapter

**Reviewed SHAPE-006 resolution (2026-10-29):** the fix is accepted; see the
[archived resolution](../../../doc/SHAPING_FINDINGS.md#shape-006--contextual-lookup-revisits-matched-input). `gsub_context1_next_glyph.tests:2` now
matches its complete unchanged upstream sequence and is an ordinary
`TestAOTSGSUBBaselines` case. All five selected GSUB records pass; the original
batch results below remain historical. No oracle or fixture data changed.

**SHAPE-005 update (2026-10-29):** `gsub_context1_lookupflag.tests:2` and
`gsub3_1_lookupflag.tests:2` now pass their complete unchanged upstream glyph
expectations and are ordinary `TestAOTSGSUBBaselines` cases. The classification
assertions moved to `TestAOTSGDEFClassification`. SHAPE-005 is reviewed and
resolved; the remaining ligature mark-loss test retains its old SHAPE005 name
but tracks [SHAPE-019](../../../doc/SHAPING_FINDINGS.md#shape-019--ligature-substitution-deletes-ignored-marks).
The dated results below retain
the original discovery evidence.

The test-only adapter in
[`internal/hbcmp/aots_gsub_test.go`](../../../internal/hbcmp/aots_gsub_test.go)
reads explicitly selected records from the recovered files. It checks the
pinned manifest revision and the source/font SHA-256 hashes before shaping.
Existing upstream files remain unchanged. The later [pair batch](PAIR.md)
adds two fonts, two complete source files, and their manifest provenance.
The shared adapter now also supports GPOS glyph-origin records through
[`aots_gpos_test.go`](../../../internal/hbcmp/aots_gpos_test.go).
The [GPOS batch report](GPOS.md) documents its coordinate contract, six records,
and results. The GSUB sections below retain the earlier batch history.

## Selection and results

The initial batch was verified on 2026-10-26 against production source at
`7b9df6582ee7154a8f9be2a2ba718689e4a7681e`, with the new test files in the
working tree. Expected glyph IDs come directly from upstream; no clusters or
position fields are fabricated.

| Manifest ID | Expected glyph IDs | Actual glyph IDs | Placement |
| --- | --- | --- | --- |
| `classdef2.tests:2` | `[17,18,19,20,21]` | Same | Ordinary `TestAOTSGSUBBaselines` |
| `gsub_context1_lookupflag.tests:2` | `[0,60,90,61,91,92,62,0]` | `[0,20,90,21,91,92,22,0]` | Tagged SHAPE-005 |
| `gsub_context1_next_glyph.tests:2` | `[0,60,20,60,20,20,0]` | `[0,60,60,60,60,20,0]` | Tagged SHAPE-006 |

The passing class-based case expects an unchanged glyph sequence. It is a useful
negative baseline, but does not establish successful class-based substitution
in general. The two failures are active upstream cases, not records that were
commented out upstream. Details and diagnostic subtests are in
[`SHAPING_FINDINGS.md`](../../../doc/SHAPING_FINDINGS.md).

### Alternate-substitution batch

Verified on 2026-10-26 at revision
`88627506dae374b163968c67cf97601a4ada18c1` plus the adapter/test changes.
Two more active records use single-index feature requests. Neither passes yet;
their expectations are unchanged and their tests are tagged.

| Manifest ID / execution | Expected glyph IDs | Actual glyph IDs | Finding |
| --- | --- | --- | --- |
| `gsub3_1_simple.tests:2`, whole input | `[17,18,17,20,17,21,17,22,17,18,17,20,17]` | `[17,18,17,20,17,20,17,20,17,18,17,20,17]` | SHAPE-003: second/third values select the first alternate |
| Same record, buffers `2/1/4` | Same upstream expectation | `[17,18,17,18,17,18,17,18,17,18,17,18,17]` | SHAPE-002: substitutions disappear; SHAPE-003 also remains open |
| `gsub3_1_lookupflag.tests:2`, whole input | `[17,18,18,18,19,22,23,19,17]` | `[17,18,18,18,19,19,19,19,17]` | SHAPE-005: unclassified GID 19 is incorrectly skipped by IgnoreBaseGlyphs |

All calls return nil errors. The simple case enables positions 3, 5, 7, and 11
with values 1, 2, 3, and 1, and explicitly disables positions 1 and 9. Whole-input
watermarks are `14/1/15`. Its first-alternate positions work in the whole-input
run but fail in the streaming run, independently of the later-alternate defect.
The lookup-flag case enables positions 5 and 6 with values 1 and 2 and disables
4 and 7; whole-input watermarks are `10/1/11`. Its filtering failure masks
alternate selection, so this is not an isolated SHAPE-003 reproduction.

At the end of the GSUB batches there were five selected upstream records: one ordinary passing baseline
and four failing records, with an additional streaming execution of the simple
alternate case. No new finding IDs are needed for this batch.

**SHAPE-002 update (2026-10-27):** the streaming simple-alternate case now
selects the same source positions as the whole-input case, including the first
alternates at positions 3 and 11. Its complete upstream expectation still fails
for values 2 and 3 (SHAPE-003), so it remains tagged as
`TestKnownBugSHAPE003AOTSAlternatesStreaming`. No expected glyphs were changed.

**SHAPE-003 update (2026-10-27):** both simple-alternate executions now match
the complete unchanged upstream glyph sequence. The whole-input record is in
`TestAOTSGSUBBaselines`; the streaming execution is
`TestSHAPE003AOTSAlternatesStreaming`. The flagged record retains its full
expectation under SHAPE-005. No upstream data or capture was regenerated.

## Explicit adapter contract

- The quoted `--features` list supports the `test` tag, an optional `-` prefix,
  an optional single index `[i]`, and an optional nonnegative decimal value
  `=n`. Negative prefix plus explicit value is rejected. Global `test` maps to
  `FeatureRange{Feature: test, On: true, Arg: 1}`; single-index requests map
  to `[Start:i, End:i+1)`. Requests retain source order. Unsupported tags,
  spans such as `[a:b]`, negative values, malformed/overflowing numbers, and
  indexes outside the input fail explicitly. This deliberately covers only
  the syntax needed by the selected records, not all HarfBuzz feature syntax.
- Indexes are zero-based Unicode scalar positions, with an exclusive end;
  they are not glyph indexes or UTF-8 byte offsets. `--utf8-clusters` is not
  supported. These records use the custom feature `test`, disabled outside
  the explicitly enabled ranges. There is no implicit global enable request.
- Bare `test[i]` has value 1. Positive values are passed unchanged as `Arg`,
  using our agreed one-based contract. `-test[i]` and `test[i]=0` map to
  `On:false, Arg:0`: HarfBuzz zero means disabled, whereas our **enabled**
  `Arg:0` means default/first. Do not translate zero into an enabled request.
- `@shaper=ot` selects the OpenType path; our tests use `otcore.New()` through
  the exported `NewShaper(...).Shape(...)` API.
- Script/language/direction are explicitly Latin/English/LTR. The upstream AOTS
  generator uses that context. The original command-line records omit these
  values, so their equivalence was checked separately as described below.
- Input is the exact Unicode scalar list, including U+0000 and control
  characters. `--single-par` is represented by supplying the complete sequence
  to one `Shape` call. Whole-input watermarks are high=`input length + 1`, low=1,
  max=`input length + 2`. The additional streaming test explicitly uses `2/1/4`
  (high/low/max) against the same upstream expectation.
- `--no-glyph-names --no-clusters --no-positions` means compare only numeric
  glyph IDs and their order. Clusters, advances, offsets, and unsafe flags are
  deliberately unchecked in glyph-only mode; the GSUB batch establishes no positioning or cluster
  parity and tests only one small-buffer configuration.
- Disabled records, unavailable fonts, unknown record IDs, unsupported
  directives, malformed scalars, and output incompatible with the selected mode are errors.
  Fixture problems do not silently skip tests. Focused adapter tests cover
  unsupported inputs and preservation of NUL/glyph zero.

These choices are specific to the selected records; they are not a general
translation of every HarfBuzz option. The existing full-position JSON harness
and its Gentium baseline remain separate.

`TestAOTSFeatureMapping` checks the adapter contract separately from shaping:
first/last scalar boundaries, multibyte text, ordered requests, zero-as-off,
and preservation of positive values. These are not additional upstream font
cases. The recovered records cover values 1/2/3 and explicit disabling, but
provide no oracle for enabled `Arg:0`, out-of-range positives, negative/random
selection, or lower-level `ApplyFeature`. Those agreed-contract tests remain
separate work; the adapter does not pretend that HarfBuzz's options establish
our random-selection policy.

The range/value mapping follows the pinned upstream
[`hb_feature_from_string` parser](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/src/hb-common.cc)
and [`shape-options.hh`](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/util/shape-options.hh),
inspected from the pinned recovery archive.

## Independent context verification

`/opt/homebrew/bin/hb-shape --version` reported **HarfBuzz 15.0.0** on the
verification machine. This was an installed binary, not a build of the pinned
upstream source commit. Each selected GSUB record was run with its recorded options
under `LC_ALL=C`, selecting the `ot` shaper and `ot` font functions, then again
with explicit `--script=Latn --language=en --direction=ltr`. All ten invocations
returned exactly the stored expectation. These checks corroborate the context
mapping; the pinned upstream records remain the test oracle.

To repeat the optional verification from the repository root (requires an
installed `hb-shape`; not invoked by Go tests):

```sh
hb-shape --version
python3 - <<'PY'
import os
from pathlib import Path
import subprocess
import shlex

for name in ("classdef2.tests", "gsub_context1_lookupflag.tests",
             "gsub_context1_next_glyph.tests", "gsub3_1_simple.tests",
             "gsub3_1_lookupflag.tests"):
    line = (Path("testdata/harfbuzz/aots/tests") / name).read_text().splitlines()[1]
    font, options, unicodes, expected = line.split(";")
    command = ["hb-shape", str(Path("testdata/fonttools") / Path(font).name),
               "--unicodes", unicodes, *shlex.split(options),
               "--shapers=ot", "--font-funcs=ot"]
    for extra in ([], ["--script=Latn", "--language=en", "--direction=ltr"]):
        actual = subprocess.check_output(command + extra, text=True,
                                         env={**os.environ, "LC_ALL": "C"}).strip()
        assert actual == expected, (name, extra, actual, expected)
        print(name, "explicit" if extra else "recorded", "PASS")
PY
```

## Running the Go tests

```sh
# Passing baseline and adapter validation; no HarfBuzz/Python required.
go test ./internal/hbcmp -run '^TestAOTS' -count=1 -v

# AOTS findings: four failing top-level tests; SHAPE005 is the legacy name for SHAPE-019.
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE(005|006|007|008)AOTS' -count=1 -v
```

The full ordinary `go test ./... -count=1` suite passed. Once a defect is fixed,
move its record ID into the ordinary baseline list and remove its tagged test;
promote useful diagnostic regressions as well. No production fixes are included
in these batches. GPOS origin comparisons are now covered in [GPOS.md](GPOS.md);
pair format 1 is covered in [PAIR.md](PAIR.md). Class-based pairs,
richer position captures, and broader feature syntax remain
later work. Promote each streaming configuration only when it too passes.
