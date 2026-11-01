/*
Package otcore provides the general shaping engine for package otshape.

The core engine applies OpenType shaping without script-specific steps. It uses
the default normalization (composed for most scripts), the common default GSUB
and GPOS features of the font's script and language system, and GPOS
positioning. It adds no hooks of its own.

Use it as the engine for Latin text and as a fallback next to script-specific
engines:

	shaper := otshape.NewShaper(otcore.New(), othebrew.New())

When asked by [otshape.Shaper] to match a run, the engine reports high
confidence for Latin script, low confidence for other left-to-right runs, and
no match for other directions. A script-specific engine with a better match
therefore wins.

Latin is the focus of the shared conformance work; other scripts that need
their own shaping logic (such as Indic scripts) are not covered by this engine.
The otshape package documentation lists current limitations.
*/
package otcore
