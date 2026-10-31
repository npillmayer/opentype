/*
Package otshape provides a streaming OpenType shaping pipeline.

The package API is centered around [Shape] and [NewShaper]:
  - callers provide shaping metadata ([Params]) and buffering policy ([BufferOptions]),
  - runes are consumed from a [RuneSource],
  - shaped glyph records are emitted to a [GlyphSink].

Advanced clients may use [ShapeEvents] with an [InputEventSource] to provide
explicit feature push/pop boundaries in-band with rune input.
In this mode, [Params.Features] is restricted to global defaults
(FeatureRange with Start==0 and End==0 only).

The pipeline compiles a per-request plan, applies GSUB/GPOS lookups, and supports
script-specific shaper engines through hook interfaces defined in this package.
*/
package otshape
