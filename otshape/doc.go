/*
Package otshape shapes text with OpenType fonts: it turns a run of Unicode
text into positioned glyphs, using the font's GSUB and GPOS layout tables.

A shaping call handles one run with a fixed font, script, language, and
direction. Splitting text into such runs (itemization), bidirectional
reordering, font fallback, line breaking, vertical layout, and variable fonts
are outside this package.

# Quick start

Create a [Shaper] with the candidate shaping engines, describe the run with
[Params], and call [Shaper.Shape] with a [RuneSource] for the input and a
[GlyphSink] for the output:

	shaper := otshape.NewShaper(otcore.New(), othebrew.New())
	err := shaper.Shape(params, strings.NewReader(text), sink, otshape.BufferOptions{})

A zero [BufferOptions] selects default buffering. See the examples for complete
programs.

# Input

[Params] names the font, direction, ISO 15924 script, and BCP 47 language.
Script and language are mapped to OpenType tags; unknown or unmapped values
fall back to the font's default (DFLT) script or language system.

The Features field of [Params] turns OpenType features on or off, optionally
for a span of the input (see [FeatureRange]). Span indexes count Unicode scalar values
of the whole input, starting at zero; they are not UTF-8 byte offsets.
Feature values select alternates: positive values are one-based, zero selects
the default (first) alternate, and negative values request random selection.
Common features such as ccmp, liga, kern, and mark are enabled by default.

# Output

The sink receives one [GlyphRecord] per glyph, in logical order: the order of
the text after normalization and reordering, not the visual order. This also
holds for right-to-left text; visual ordering is the caller's task.

Each record carries a cluster value. Clusters follow HarfBuzz cluster level 0:
a base character and its combining marks form one cluster, and its value is the
index of the cluster's first Unicode scalar value in the input. When glyphs
are combined, as in ligatures, they take the smallest cluster value involved.

Positions are in font design units. XAdvance already contains the glyph's
nominal advance width; offsets move the glyph without moving the pen. The
fields Mask and UnsafeFlags and the attachment fields of the position are
internal shaping state; they are not meant for layout decisions.

# Shaping engines

Script-specific behavior comes from shaping engines (see [ShapingEngine]).
Callers pass candidate engines to [NewShaper]; there is no global registry.
For each call, the engine whose Match method reports the highest confidence for
the run is selected (ties are broken by Name), and its New method provides
a fresh instance for that call. The module supplies these engines:

  - package otcore: the general engine, preferred for Latin;
  - package othebrew: Hebrew composition and mark reordering;
  - package otarabic: Arabic and Syriac joining.

Engines customize shaping through optional hook interfaces, such as
[ShapingEngineComposeHook] or [ShapingEngineMaskHook].

# Shaping steps

For each piece of buffered input, shaping proceeds as follows:

 1. Group characters into initial clusters and normalize them (composed or
    decomposed, as chosen by the engine and script).
 2. Map characters to glyphs through the font's cmap table.
 3. Run the engine's preprocessing, mark reordering, and pre-GSUB hooks.
 4. Set up the feature masks, then let the engine adjust them.
 5. Apply GSUB lookups, then GPOS lookups.
 6. Adjust mark advances and resolve mark attachment offsets (currently for
    Latin and Hebrew).
 7. Run the engine's postprocessing hook and emit the glyphs.

A feature plan is compiled from the font's features for the script and
language, the user's feature settings, and the engine's requirements: once per
call, and once per feature push in event mode. Details of the hooks and their order are in
doc/SHAPER_INTERFACES.md.

# Streaming and errors

Input is read incrementally and output is emitted in chunks. [BufferOptions]
sets the buffer limits, counted in input characters. Input shorter than the
high watermark (by default 3500 characters) is shaped as a whole. Longer input
is cut into pieces, and a cut can currently change ligatures, kerning, or
normalization across it; small buffers make this more likely. The intended
buffering contract is described in doc/STREAMING_API.md.

If shaping fails, glyphs already written to the sink remain there, and consumed
input is not restored. Callers who need all-or-nothing results should collect
the glyphs and use them only after a nil error.

# Feature scopes

[Shaper.Shape] scopes features by input spans. [Shaper.ShapeEvents] instead
reads an [InputEventSource], whose events interleave the runes with pushes and
pops of feature settings. In that mode, the Features field of [Params] may only
hold settings for the whole input, and every push must be matched by a pop.

# Fonts, randomness, and concurrency

A parsed font refers to its original bytes; they must not change while the font
is in use. Random alternate selection is seeded per call, so identical calls
may choose different alternates. Concurrent use of a [Shaper] or a font is not
yet guaranteed to be safe.

# Current limitations

Shaping is not yet fully conformant. Latin and Hebrew are the focus; there are
no engines for Indic or other complex scripts. Known gaps include:

  - Normalization does not consider which characters the font supports, and
    canonically equivalent mark orders may shape differently.
  - Buffer cuts and feature scope boundaries can break ligatures and
    kerning across them.
  - An engine reporting [ShaperConfidenceNone] can still be selected when no
    candidate matches better.
  - Fonts without a GPOS table are rejected by the parser.

doc/SHAPING_FINDINGS.md lists the reproduced findings and their status.

# Further reading

The repository's doc directory describes the implementation in more depth:
doc/SHAPING_OVERVIEW.md (architecture and data flow), doc/SHAPER_INTERFACES.md
(engine hooks), doc/STREAMING_API.md (buffering), and doc/TESTING.md (tests and
fixtures).
*/
package otshape
