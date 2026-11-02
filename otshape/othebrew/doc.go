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
  - Mark reordering: normalization sorts with the Hebrew modified combining
    classes, then adjusts patah/qamats followed by sheva/hiriq and meteg/below.
    This runs before recomposition, once per mark sequence. Original scalar
    sources follow their runes; initial level-0 groups govern feature ranges.

Hebrew is shaped right to left, but glyphs are emitted in logical order; see
the otshape package documentation.

The normalizer uses the same modified classes for sorting and composition
blocking. Composition still requires font coverage. GPOS mark support reflects
the resolved plan, including defaults and ranged enables. This does not establish
safe context retention across streaming cuts; see doc/SHAPING_FINDINGS.md.
*/
package othebrew
