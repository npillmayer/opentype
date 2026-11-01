/*
Package othebrew provides the Hebrew shaping engine for package otshape.

The engine matches runs in Hebrew script with certain confidence and reports
no match otherwise. Use it together with the core engine:

	shaper := otshape.NewShaper(otcore.New(), othebrew.New())

Shaping follows the general otshape steps, with GPOS positioning and default
normalization. The engine adds two Hebrew-specific steps:

  - Composition: when the font's plan has no GPOS mark positioning, letters and
    points without a canonical composition may be composed into Hebrew
    presentation forms (U+FB1D–U+FB4E), for example yod with hiriq or a letter
    with dagesh. Fonts that position marks with GPOS keep letters and points as
    separate glyphs.
  - Mark reordering: a patah or qamats followed by sheva or hiriq and then by
    meteg or another below mark is reordered, so that fonts position the marks
    as intended. The affected marks are merged into one cluster.

Hebrew is shaped right to left, but glyphs are emitted in logical order; see
the otshape package documentation.

Known gaps: the decision whether GPOS mark positioning is available currently
misses mark features that are enabled by default, so presentation forms may be
composed when they should not be; and the internal order of some mark
combinations (such as shin with qamats and shin dot) still differs from
HarfBuzz. doc/SHAPING_FINDINGS.md tracks both.
*/
package othebrew
