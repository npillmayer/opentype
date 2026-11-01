/*
Package otarabic provides the Arabic and Syriac shaping engine for package
otshape.

The engine matches left-to-right or right-to-left runs in Arabic script with
certain confidence and Syriac runs with high confidence. Use it together with
the core engine:

	shaper := otshape.NewShaper(otcore.New(), otarabic.New())

In addition to the general otshape steps, the engine:

  - determines each character's joining form (isolated, initial, medial,
    final, and the Syriac final/medial variants) and enables only the
    matching form feature (isol, init, medi, fina, fin2, fin3, med2) for it;
  - adds the Arabic GSUB features stch and mset to the plan, with explicit
    zero-width-joiner handling for the joining-related features;
  - moves modifier combining marks of combining class 220 or 230 (such as
    hamza above or below) to the start of the preceding mark sequence;
  - after GSUB, repeats tatweel for the stch feature, and for Arabic replaces
    .notdef glyphs by the font's Arabic presentation-form glyphs when the font
    lacks the form or rlig features.

Each shaping call uses its own engine instance, because the engine keeps
per-call plan state.

Status: Arabic and Syriac shaping is outside the current conformance focus
(Latin and Hebrew). Joining classification is hand-maintained rather than
derived from the Unicode joining data, and the behavior is covered by unit
tests only, without independent HarfBuzz comparisons. The Syriac script is
currently mapped to the default OpenType script tag rather than syrc. GSUB
features currently run in the general default order, so the form features are
applied after rlig, calt, and liga rather than before them.
*/
package otarabic
