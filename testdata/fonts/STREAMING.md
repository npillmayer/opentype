# Synthetic progress font

`ShaperTestProgress.ttf` is authored in this repository, not derived from an
upstream font. Its source is
[build_streaming_font.py](../scripts/build_streaming_font.py); it is covered by
the repository's [BSD 3-Clause license](../../LICENSE).

SHA-256: `6b4bb3ea60b769f0b385eeed74dc79ab50e61628f458c1a1368548acd59293be`.

The font has 1000 units/em, rectangle outlines, and 600-unit nominal advances.
Glyph order is `.notdef, a, b, d, f, i, fi, space` (GIDs 0–7). Unicode maps
`a`, `b`, `d`, `f`, `i`, and space. GSUB feature `test` expands `b` to `b b`,
deletes `d`, and contracts `f i` to `fi`. GPOS `kern` changes the first advance
in `a a` by −20. Both DFLT and latn/default-language systems expose the features.
Both GSUB and GPOS are present, so ordinary `ot.Parse` is used. It is a diagnostic
font, not a text-rendering typeface.

Preparation uses FontTools **4.61.2.dev0**, available at checkout revision
`9cec77d49bdb1a1ca346ac5fefdc5e7c30929026`. Timestamps are fixed, and the default
script operation rebuilds in memory and compares bytes without writing:

```sh
# Supply the recorded FontTools version through the Python environment.
python3 testdata/scripts/build_streaming_font.py
python3 testdata/scripts/build_streaming_font.py --write
```

A checkout may be supplied with `PYTHONPATH=/path/to/fonttools/Lib`. No FontTools
installation is needed to run Go tests. The baseline output is independently
captured with HarfBuzz, including output after deletion and its effect on GPOS;
it is not calculated from the Go shaper or merely assumed from the feature source.
See the [streaming report](../../internal/hbcmp/testdata/text/STREAMING.md).

All glyphs have nonempty outlines to keep this fixture focused on shaping and
progress. An initial empty-outline prototype encountered `GlyphMetrics`' unchecked
glyf bounding-box read; empty-outline metrics are outside this batch's coverage.
