# Synthetic ligature-component fixture

`ShaperTestLigatureMarks.ttf` is authored in this repository and covered by its
[BSD 3-Clause license](../../LICENSE). It is not derived from an upstream font.
The source is [build_ligature_mark_font.py](../scripts/build_ligature_mark_font.py).
SHA-256: `7c404970c9be91facf44a5e5533f00ad2cb11455b8ddb863a05fd00d7b9d7e52`.

The font uses 1000 units/em and rectangle outlines. Every nominal advance is
600, including marks, so public mark-advance zeroing is exercised. Glyph order
is `.notdef, f, i, l, fi, il, fil, overline, vertical, extra, space` (GIDs 0–10).
Unicode maps `f/i/l`, space, U+FB01, U+0305 and U+030D; Hebrew alef/bet/gimel
map to `f/i/l`, and sheva maps to `overline`, solely to exercise logical RTL
output through the Hebrew engine. This is not a Hebrew design or a conformance
fixture for script-specific ligature rules.

GSUB defines `liga: f i -> fi`, `clig: i l -> il`, then `rlig: fi l -> fil`
and `f il -> fil`, all ignoring marks. Disabling `liga` selects the second
nested construction. Later `ss01` replaces overline with vertical; `ss02`
expands overline to vertical+extra. GDEF explicitly classifies bases, ligatures
and marks. GPOS `mark` provides two or three distinct component anchors:
`(100,400)`, `(300,500)`, `(500,600)`. Mark anchors are `(0,0)`, `(0,10)` and
`(0,20)`, so wrong component selection is visible in both offsets.

Preparation uses FontTools **4.61.2.dev0**, checkout
`9cec77d49bdb1a1ca346ac5fefdc5e7c30929026`. Timestamps are fixed; the default
operation builds in memory and compares bytes without writing:

```sh
PYTHONPATH=/path/to/fonttools/Lib python3 testdata/scripts/build_ligature_mark_font.py
PYTHONPATH=/path/to/fonttools/Lib python3 testdata/scripts/build_ligature_mark_font.py --write
python3 testdata/scripts/capture_harfbuzz_ligatures.py
python3 testdata/scripts/capture_harfbuzz_ligatures.py --write
```

The independent expectations were captured on 2026-11-01 with **HarfBuzz
15.0.0**, never with the repository shaper. The ten records in
[ligatures.json](../../internal/hbcmp/testdata/text/ligatures.json) include exact
commands, Unicode inputs, font and executable hashes, clusters, glyph IDs,
advances and offsets. The default capture command checks all provenance and
output without writing. The new data is separate from all earlier snapshots.
Go tests require neither Python nor FontTools nor HarfBuzz.

| Capture ID | Purpose |
| --- | --- |
| `ligature_intervening` | Preserved mark on the first component |
| `ligature_trailing` | Unassociated trailing mark uses the last component |
| `ligature_many_marks` | Two intervening marks and one trailing mark |
| `ligature_nested_left` | Form `fi`, then `fi+l`; flatten component indices |
| `ligature_nested_right` | Form `il`, then `f+il`; reassign marks following the consumed `il` |
| `ligature_distinct_instances` | Two occurrences with different mark origins |
| `ligature_input_glyph` | Input U+FB01 has no GSUB provenance; retain last-component fallback |
| `ligature_later_single` | Preserve association across single substitution |
| `ligature_later_multiple` | Preserve association on both outputs of mark expansion |
| `ligature_hebrew_rtl` | Full logical RTL glyph, cluster and position comparison |

`TestSHAPE019LigatureBaselines` compares all six fields for every record through
both `Shape` and `ShapeEvents`, using whole-input buffers and global settings.
RTL uses the existing whole-sequence reversal of the raw HarfBuzz output into
logical order. No fields or marks are dropped from the comparison. Indexed
feature ranges, context across event boundaries and streaming cuts are not
established by these twenty comparisons.
