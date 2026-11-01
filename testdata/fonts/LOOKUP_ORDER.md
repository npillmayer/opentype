# Synthetic lookup-order fonts

`ShaperTestLookupOrder-LigaFirst.ttf` and `ShaperTestLookupOrder-CaltFirst.ttf`
are authored in this repository, not derived from an upstream font. Their
source is [build_lookup_order_fonts.py](../scripts/build_lookup_order_fonts.py);
they are covered by the repository's [BSD 3-Clause license](../../LICENSE).
They isolate the order in which lookups of different default features run
([SHAPE-020](../../doc/SHAPING_FINDINGS.md#shape-020--default-features-run-in-feature-order-instead-of-lookup-order)).

| File | LookupList order | SHA-256 |
| --- | --- | --- |
| `ShaperTestLookupOrder-LigaFirst.ttf` | lookup 0 = `liga`, lookup 1 = `calt` | `f4365dfbce8cb7e7484909da6d4f14955b0aae9097f60155c9cee3778c6bca6a` |
| `ShaperTestLookupOrder-CaltFirst.ttf` | lookup 0 = `calt`, lookup 1 = `liga` (control) | `a862ada9ebb2aad26fc68cc7084c0350dd315100980f091f40d12af78591bcb9` |

Both fonts have 1000 units/em, rectangle outlines, and 600-unit nominal
advances. Glyph order is `.notdef, f, i, fi, f.alt` (GIDs 0–4); Unicode maps
only `f` and `i`. Both contain the same rules: GSUB `liga` contracts `f i` to
`fi`, GSUB `calt` substitutes `f` by `f.alt`, and GPOS `kern` adjusts `i i` by
−20 (present only so that the normal parser, which requires GPOS, accepts the
fonts). Only the LookupList order differs; feaLib numbers lookups in the order
the features are defined. Both DFLT and latn/default-language systems expose
the features. These are diagnostic fonts, not text-rendering typefaces.

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
