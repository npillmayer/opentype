# AOTS positioning baseline batch

**SHAPE-005 update (2026-10-29):** classification now permits the ligature in
`gpos5.tests:2`, but replacement deletes the ignored mark. Actual origins are
`[17|18@1500,0|17@3000,0]`; the complete upstream expectation remains unchanged
and tagged under the historical `TestKnownBugSHAPE005AOTSLigatureAcrossMark`
name, now tracking [SHAPE-019](../../../doc/SHAPING_FINDINGS.md#shape-019--ligature-substitution-deletes-ignored-marks).
Mark preservation/component association and final SHAPE-007/008 positioning
require separate work. The flagged pair case in [PAIR.md](PAIR.md)
now passes and has been promoted. The dated results below are historical.

Verified on 2026-10-26 at repository revision
`f560698c2ecf09ed0c317a78327862707e5d8f46` plus the new tests and adapter changes.
The six active records below use existing, hash-verified fonts and unchanged
upstream output at revision `29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b`.
No fonts, source records, or manifest entries were added or changed in that
initial positioning batch. The subsequent [pair batch](PAIR.md) imports two
fonts and two test files, with two passing pair cases and one tagged failure.

## Comparison contract

The shared loader/runner remains in
[`aots_gsub_test.go`](../../../internal/hbcmp/aots_gsub_test.go); origin parsing,
comparison, and adapter tests are in
[`aots_gpos_test.go`](../../../internal/hbcmp/aots_gpos_test.go).
The manifest's `glyph_ids_and_origins` field selects the origin mode, which
requires `--ned` and rejects `--no-positions`. The glyph-only mode still rejects
positioned output. Neither mode fabricates missing expected fields.

With `--ned`, an expected token `19@1400,-80` specifies GID 19 and its origin
in font units. A token without `@` means origin `(0,0)`, even after a nonzero
origin; it does not mean an unknown position or a zero offset from the pen.
For each emitted glyph, compare:

```
originX[i] = sum(XAdvance[0:i]) + XOffset[i]
originY[i] = sum(YAdvance[0:i]) + YOffset[i]
```

The sums use the public `GlyphRecord.Pos` values directly, with 64-bit adapter
arithmetic. The adapter does not resolve attachment references or repair mark
advances. Thus it measures the output a consumer actually receives. This
matches the pinned upstream [text serializer](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/src/hb-buffer-serialize.cc)
for these small coordinates; overflow/saturation is outside this batch's scope.
See also [`--ned` format options](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/util/shape-format.hh).
Both source files were inspected in the pinned recovery archive.

All cases use Latin/English/LTR, the core engine, global `test` enabled with
`Arg:1`, exact scalar input including control characters, and a single shaping
window: high=`input length + 1`, low=1, max=`input length + 2`.
Only glyph IDs/order and origins are compared. These records cannot establish
individual advances and offsets independently, the final glyph's advance,
total run advance, clusters, RTL positioning, or streaming correctness.

Adapter tests check omitted origins, signed X/Y coordinates, cumulative advances,
zero advances, offsets that must not move the pen, and rejection of richer or
malformed output. They are arithmetic/schema tests, not extra upstream cases.

## Results

All six `Shape` calls return nil errors. Origins use upstream text notation;
an omitted `@` below means `(0,0)`.

| Upstream record | Expected | Actual | Placement |
| --- | --- | --- | --- |
| `gpos_chaining3_boundary.tests:3` | `[0\|20@1500,0\|21@3020,0\|22@4500,0\|23@6000,0\|0@7500,0]` | Same | Ordinary `TestAOTSGPOSBaselines` |
| `gpos4_simple.tests:2` | `[17\|18@1500,0\|19@1400,-80\|17@3000,0]` | `[17\|18@1500,0\|19@3000,0\|17@4500,0]` | Tagged SHAPE-007; also SHAPE-008 |
| `gpos4_simple.tests:3` | `[17\|17@1500,0\|19@3000,0\|17@3000,0]` | `[17\|17@1500,0\|19@3000,0\|17@4500,0]` | Tagged SHAPE-008 |
| `gpos4_simple.tests:4` | `[25\|25@1500,0\|19@3000,0\|17@3000,0]` | `[25\|25@1500,0\|19@3000,0\|17@4500,0]` | Tagged SHAPE-008 |
| `gpos5.tests:2` | `[17\|18@1500,0\|19@1400,-80\|17@3000,0]` | `[17\|30@1500,0\|19@3000,0\|31@4500,0\|17@6000,0]` | Tagged SHAPE-005; positioning blocked by GSUB failure |
| `gpos5.tests:3` | `[17\|18@1500,0\|19@1401,-79\|17@3000,0]` | `[17\|18@1500,0\|19@3000,0\|17@4500,0]` | Tagged SHAPE-007; also SHAPE-008 |

The passing case exercises chained contextual GPOS type 8 format 3 invoking
single positioning type 1 format 1: GID 21 moves +20 horizontally. Although
the font also contains pair lookups 1 and 2, its `test` feature reaches lookup 4
and then lookup 0. This record is **not a pair-positioning baseline**. Pair
execution already had local tests; the subsequent [pair batch](PAIR.md) now
supplies genuine independent public-API pair baselines using different fonts.

The mark-to-base case attaches GID 19 to output index 1; the second
mark-to-ligature case first forms GID 18 and also records an attachment to index
1. Both retain zero X/Y offsets despite having anchor references. The two
unattached-mark controls have the correct mark origin but advance the following
glyph by an extra 1500. The interleaved-mark ligature case fails before GPOS:
GIDs 30 and 31 do not combine across GID 19 under IgnoreMarks. See
[SHAPING_FINDINGS.md](../../../doc/SHAPING_FINDINGS.md) for implementation evidence
and the distinction between the classification, anchor, and advance defects.

## Independent verification

Installed HarfBuzz 15.0.0 reproduced all six stored expectations both with the
recorded options and with explicit Latin/English/LTR and font-unit scale
(12 successful invocations). This is an installed binary, not a build of the
pinned source revision; the pinned records remain the test oracle.

Optional verification from the repository root, requiring `hb-shape`:

```sh
hb-shape --version
python3 - <<'PY'
from pathlib import Path
import os
import shlex
import subprocess

for name, lines in (("gpos_chaining3_boundary.tests", (3,)),
                    ("gpos4_simple.tests", (2, 3, 4)),
                    ("gpos5.tests", (2, 3))):
    source = (Path("testdata/harfbuzz/aots/tests") / name).read_text().splitlines()
    for number in lines:
        font, options, unicodes, expected = source[number - 1].split(";")
        command = ["hb-shape", str(Path("testdata/fonttools") / Path(font).name),
                   "--unicodes", unicodes, *shlex.split(options),
                   "--shapers=ot", "--font-funcs=ot"]
        for extra in ([], ["--script=Latn", "--language=en", "--direction=ltr",
                           "--font-size=upem"]):
            actual = subprocess.check_output(command + extra, text=True,
                                             env={**os.environ, "LC_ALL": "C"}).strip()
            assert actual == expected, (name, number, extra, actual, expected)
            print(name, number, "explicit" if extra else "recorded", "PASS")
PY
```

For diagnosis only, repeating `gpos4_simple.tests:3` and `gpos5.tests:3` with
`--ned` removed and `--output-format=json` added confirmed mark advance zero
in HarfBuzz. The ligature case has mark offset `(-1599,-79)`; the unattached
case has `(0,0)`. These extra fields are not silently added to the stored oracle.

## Running and remaining scope

```sh
# Passing positioning baseline and origin-adapter checks; no external runtime.
go test ./internal/hbcmp -run '^TestAOTS(GPOS|Origin)' -count=1 -v

# Recorded GPOS/GSUB-blocked failures: three failing top-level tests.
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE(007|008|005AOTSLigature)' -count=1 -v
```

At the end of this initial GPOS batch, the full ordinary suite passed and the
combined tagged suite had twelve failing
top-level tests for eight findings, with no production fixes in this batch.
Across the adapter batches at that point, eleven upstream records were selected: two passed,
nine failed, and one failing GSUB record also had a streaming execution.
Cursive cases remain deferred. Upstream-disabled records remain excluded.
Pair format 1 is now covered in [PAIR.md](PAIR.md). Class-based pair positioning,
mark-to-mark, complete per-glyph position captures, and Latin/Hebrew text and
streaming baselines remain follow-up work.
