# First AOTS GSUB baseline batch

The test-only adapter in
[`internal/hbcmp/aots_gsub_test.go`](../../../internal/hbcmp/aots_gsub_test.go)
reads explicitly selected records from the recovered files. It checks the
pinned manifest revision and the source/font SHA-256 hashes before shaping.
Upstream files and the recovery manifest remain unchanged.

## Selection and results

Verified on 2026-10-26 against production source at
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

## Explicit adapter contract

- Only global `--features="test"` is supported in this batch. It maps to
  `FeatureRange{Feature: test, On: true, Arg: 0}`. Ranged features, other options,
  and richer expected-output formats fail explicitly instead of being ignored.
- `@shaper=ot` selects the OpenType path; our tests use `otcore.New()` through
  the exported `NewShaper(...).Shape(...)` API.
- Script/language/direction are explicitly Latin/English/LTR. The upstream AOTS
  generator uses that context. The original command-line records omit these
  values, so their equivalence was checked separately as described below.
- Input is the exact Unicode scalar list, including U+0000 and control
  characters. `--single-par` is represented by supplying the complete sequence
  to one `Shape` call. Watermarks are high=`input length + 1`, low=1,
  max=`input length + 2`, keeping the entire input in one shaping window.
- `--no-glyph-names --no-clusters --no-positions` means compare only numeric
  glyph IDs and their order. Clusters, advances, offsets, and unsafe flags are
  deliberately unchecked; this batch establishes no streaming or positioning
  parity.
- Disabled records, unavailable fonts, unknown record IDs, unsupported
  directives, malformed scalars, and invalid glyph-only output are errors.
  Fixture problems do not silently skip tests. Focused adapter tests cover
  unsupported inputs and preservation of NUL/glyph zero.

These choices are specific to the selected records; they are not a general
translation of every HarfBuzz option. The existing full-position JSON harness
and its Gentium baseline remain separate.

## Independent context verification

`/opt/homebrew/bin/hb-shape --version` reported **HarfBuzz 15.0.0** on the
verification machine. This was an installed binary, not a build of the pinned
upstream source commit. Each selected record was run with its recorded options
under `LC_ALL=C`, selecting the `ot` shaper and `ot` font functions, then again
with explicit `--script=Latn --language=en --direction=ltr`. All six invocations
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

for name in ("classdef2.tests", "gsub_context1_lookupflag.tests",
             "gsub_context1_next_glyph.tests"):
    line = (Path("testdata/harfbuzz/aots/tests") / name).read_text().splitlines()[1]
    font, options, unicodes, expected = line.split(";")
    command = ["hb-shape", str(Path("testdata/fonttools") / Path(font).name),
               "--unicodes", unicodes, *options.split(),
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

# Two reproduced failures; expected nonzero exit until fixed.
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE00[56]' -count=1 -v
```

The full ordinary `go test ./... -count=1` suite passed. Once a defect is fixed,
move its record ID into the ordinary baseline list and remove its tagged test;
promote useful diagnostic regressions as well. No production fixes are included
in this batch. Ranged alternate cases and GPOS origin comparisons remain later
adapter work.
