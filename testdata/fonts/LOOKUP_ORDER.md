# Synthetic lookup-order fonts

The five `ShaperTestLookupOrder-*.ttf` fonts are authored in this repository, not derived from an upstream font. Their
source is [build_lookup_order_fonts.py](../scripts/build_lookup_order_fonts.py);
they are covered by the repository's [BSD 3-Clause license](../../LICENSE).
They isolate the order in which lookups of different default features run
([SHAPE-020](../../doc/SHAPING_FINDINGS.md#shape-020--default-features-run-in-feature-order-instead-of-lookup-order)).

| File | LookupList order | SHA-256 |
| --- | --- | --- |
| `ShaperTestLookupOrder-LigaFirst.ttf` | lookup 0 = `liga`, lookup 1 = `calt` | `f4365dfbce8cb7e7484909da6d4f14955b0aae9097f60155c9cee3778c6bca6a` |
| `ShaperTestLookupOrder-CaltFirst.ttf` | lookup 0 = `calt`, lookup 1 = `liga` (control) | `a862ada9ebb2aad26fc68cc7084c0350dd315100980f091f40d12af78591bcb9` |
| `ShaperTestLookupOrder-KernFirst.ttf` | GPOS 0 = `kern`, 1 = `mark` | `a9558d933d42b650636bc77327e123efdaeb6d09c0417921f9e6e2e429e68f8d` |
| `ShaperTestLookupOrder-MarkFirst.ttf` | GPOS 0 = `mark`, 1 = `kern` | `d54120a3d7ad6cc2136fdbe3bd1e732d924720bed3f71be86b1d3bd2f7cee57c` |
| `ShaperTestLookupOrder-SharedAlternates.ttf` | GSUB 0 shared by `salt` and `ss01` | `f437e51b0a503c55f2753be513cb55b5369bb7f50c7e8f123a81883ff09905aa` |

The original GSUB pair has 1000 units/em, rectangle outlines, and 600-unit nominal
advances. Glyph order is `.notdef, f, i, fi, f.alt` (GIDs 0–4); Unicode maps
only `f` and `i`. Both contain the same rules: GSUB `liga` contracts `f i` to
`fi`, GSUB `calt` substitutes `f` by `f.alt`, and GPOS `kern` adjusts `i i` by
−20 (present only so that the normal parser, which requires GPOS, accepts the
fonts). Only the LookupList order differs; feaLib numbers lookups in the order
the features are defined. Both DFLT and latn/default-language systems expose
the features. These are diagnostic fonts, not text-rendering typefaces.

The GPOS pair adds GID 5 `acute` mapped from U+0301, with a GDEF mark class.
It retains the `liga` GSUB table for normal parsing. `kern` moves `f` up by 50;
`mark` attaches `acute` at base anchor (100,200), mark anchor (0,0). Attachment
captures the base's Y placement at lookup time, so `f` + U+0301 has mark Y offset
250 with KernFirst and 200 with MarkFirst. In both, the mark's X offset is -500
and its advances are zero; the base advances by 600 with Y offset 50.

SharedAlternates uses the original five-glyph/cmap layout, one GSUB alternate
lookup `f -> [i, fi, f.alt]`, referenced by both `salt` and `ss01`, plus the
original dummy GPOS `kern`. Independent captures cover each feature alone and
both requesting alternate 1. Contract tests separately exercise incompatible
values; the agreed explicit error is intentionally different from HarfBuzz's
packed-mask result. All five fonts have the same provenance/license. The three
new variants were added 2026-11-01 without changing either original font's bytes.

Preparation uses FontTools **4.61.2.dev0**, from the available checkout at
commit `9cec77d49bdb1a1ca346ac5fefdc5e7c30929026`. Timestamps are fixed, and the
default script operation rebuilds in memory and compares bytes without writing:

```sh
# Supply the recorded FontTools version through the Python environment.
PYTHONPATH=/path/to/fonttools/Lib python3 testdata/scripts/build_lookup_order_fonts.py
PYTHONPATH=/path/to/fonttools/Lib python3 testdata/scripts/build_lookup_order_fonts.py --write
```

No FontTools installation is needed to run Go tests. The expected output is
independently captured with HarfBuzz as the text baselines
`latin_lookup_order_liga_first` and `latin_lookup_order_calt_first`; see the
[text baseline report](../../internal/hbcmp/testdata/text/README.md).
