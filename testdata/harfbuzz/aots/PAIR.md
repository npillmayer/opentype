# Independent pair-positioning baselines

**SHAPE-005 update (2026-10-29):** correcting inclusive GDEF range endpoints
and class numbers makes `gpos2_1_lookupflag.tests:3` pass its complete unchanged
upstream origin expectation. All three pair records are now ordinary
`TestAOTSGPOSBaselines` cases. The results below retain the original discovery
snapshot; no fixture or oracle data was regenerated.

Verified on 2026-10-26 at repository revision
`a16f51a27adce48fcd0346f96de0bb8b0231faa6` plus these fixtures and tests.
All expected origins come from unchanged active records in HarfBuzz revision
`29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b`, using the [GPOS adapter](GPOS.md).

## Fixtures and coverage

Two small fonts were imported from the pinned archive into `testdata/fonttools/`.
Their bytes and both complete `.tests` files were also checked against the
previously recovered GitHub tree's blob hashes. The manifest records SHA-256
hashes and the usual upstream provenance. Existing fonts and test records are
unchanged. These fonts have no new TTX dumps and add no structural TTX coverage.
The existing [AOTS license notice](COPYING) covers these upstream fixtures.

| Font | Bytes | Selected feature and lookup | Pair adjustment |
| --- | ---: | --- | --- |
| `gpos2_1_simple_f1.otf` | 5148 | `test` → lookup 0, the only lookup; GPOS type 2 format 1, flags 0 | Pair 18/19: first XPlacement −200, second YPlacement −100 |
| `gpos2_1_lookupflag_f2.otf` | 5220 | `test` → lookup 0, the only lookup; GPOS type 2 format 1, IgnoreBaseGlyphs | Pair 19/20: first XAdvance −200, second YPlacement −100 |

The lookup paths, coverage, pair records, and value formats were inspected in
the original binary fonts. Thus the passing case actually executes pair
positioning, unlike the earlier chained single-positioning baseline.

The simple font's `gpos2_1_simple.tests:2` includes both a matching 18/19 pair
and a nonmatching 18/20 pair. Its `:3` record also checks that a covered first
glyph at end-of-input does not trigger a pair adjustment. No expectations were
derived from our shaper or substituted for upstream output.

## Results

All cases use the core engine, Latin/English/LTR, global `test`, font units,
and whole-input buffers (high/low/max `7/1/8`, `3/1/4`, and `9/1/10`, respectively).
All calls return nil errors. Origins use upstream notation; omitted `@` means zero.

| Record | Expected origins | Actual origins | Placement |
| --- | --- | --- | --- |
| `gpos2_1_simple.tests:2` | `[17\|18@1300,0\|19@3000,-100\|17@4500,0\|18@6000,0\|20@7500,0]` | Same | Ordinary `TestAOTSGPOSBaselines` |
| `gpos2_1_simple.tests:3` | `[17\|18@1500,0]` | Same | Ordinary `TestAOTSGPOSBaselines` |
| `gpos2_1_lookupflag.tests:3` | `[17\|19@1500,0\|20@2800,-100\|17@4300,0\|19@5800,0\|18@7100,0\|20@8600,-100\|17@10100,0]` | `[17\|19@1500,0\|20@3000,0\|17@4500,0\|19@6000,0\|18@7500,0\|20@9000,0\|17@10500,0]` | Tagged `TestKnownBugSHAPE005AOTSPairAdvance` |

The advance-changing case places a trailing GID 17 at X=4300 after the first
match, making the −200 advance visible independently of the second glyph's
vertical placement. It later repeats the pair across base glyph 18, which should
be ignored. The font assigns only GID 18 a GDEF base class (singleton `[18,18]`,
class 1); GIDs 19/20 are unclassified. Because `ot.BaseGlyph` currently equals
zero, `skipGlyph` filters those unclassified glyphs and no pair executes.
The singleton-range defect also affects the base glyph. This extends
[SHAPE-005](../../../doc/SHAPING_FINDINGS.md#shape-005--gdef-classification-prevents-ignoremarks),
not a newly isolated pair-arithmetic bug. A passing advance-changing baseline
still depends on resolving that blocker or adding another suitable case.

There are now fourteen selected upstream records: four pass and ten fail,
plus the existing additional GSUB streaming execution. The combined tagged
suite has thirteen failing top-level tests for eight findings. The ordinary
`go test ./... -count=1` suite passes; no production fixes are included.

## Recovery and verification

The expanded snapshot contains 14 fonts, 13 complete `.tests` files, and 35
records: 21 active local candidates, 9 disabled local records, and 5 absent-font
records. `gpos2_1_lookupflag.tests:2` is retained for provenance but its sibling
font is not imported. Cursive and upstream-disabled records remain excluded
from this testing batch.

To restore the two added fonts from the pinned archive, run from the repository
root (the manifest must be from this expanded snapshot):

```sh
python3 - <<'PY'
import hashlib
import json
from pathlib import Path
import tarfile

manifest = json.loads(Path("testdata/harfbuzz/aots/manifest.json").read_text())
names = {"gpos2_1_simple_f1.otf", "gpos2_1_lookupflag_f2.otf"}
with tarfile.open("/tmp/opentype-harfbuzz-upstream.tar.gz") as archive:
    for entry in manifest["fonts"]:
        path = Path(entry["local_path"])
        if path.name not in names:
            continue
        member = archive.getmember("harfbuzz-" + manifest["upstream_revision"]
                                   + "/" + entry["upstream_path"])
        assert member.isfile()
        data = archive.extractfile(member).read()
        assert hashlib.sha256(data).hexdigest() == entry["sha256"]
        if path.exists():
            assert path.read_bytes() == data
        else:
            path.write_bytes(data)
PY
python3 testdata/scripts/recover_harfbuzz_aots.py /tmp/opentype-harfbuzz-upstream.tar.gz
python3 testdata/scripts/recover_harfbuzz_aots.py /tmp/opentype-harfbuzz-upstream.tar.gz --check
```

Installed HarfBuzz 15.0.0 reproduced all three records with their original
options and with explicit `--script=Latn --language=en --direction=ltr
--font-size=upem` (six matches). This binary is not a build of the pinned source;
the original records remain the oracle. To repeat, use the verification script
in [GPOS.md](GPOS.md#independent-verification), replacing its case list with:

```python
(("gpos2_1_simple.tests", (2, 3)), ("gpos2_1_lookupflag.tests", (3,)))
```

```sh
# Includes the passing matching and nonmatching cases; no HarfBuzz required.
go test ./internal/hbcmp -run '^TestAOTSGPOSBaselines' -count=1 -v

# Promoted flagged pair case after SHAPE-005.
go test ./internal/hbcmp -run '^TestAOTSGPOSBaselines/gpos2_1_lookupflag' -count=1 -v
```

This batch covers pair format 1, not class-based pair format 2. It does not
establish streaming, RTL/Hebrew, complete per-glyph advances/offsets, or final
run-advance correctness. Those remain separate baseline dimensions.
