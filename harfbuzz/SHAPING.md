# HarfBuzz Shaping Summary

## Core idea
HarfBuzz is a shaping engine: it turns Unicode text into positioned glyphs for a specific font, script, language, and direction. Shaping output is not just glyph IDs, but also per-glyph advances and offsets needed for rendering.

In the shaping flow described by the docs, `hb_shape()` takes a font, a buffer, and optional user features. The same buffer is returned with glyph data (`codepoint` becomes glyph ID, `cluster` tracks text-to-glyph mapping) and positions (`x_advance`, `y_advance`, `x_offset`, `y_offset`).

## What HarfBuzz does and does not do
HarfBuzz handles shaping for one text run with uniform properties. It does not do full layout tasks such as:
- bidi reordering across mixed-direction text
- segmentation into runs when font/script/language/direction changes
- line breaking, hyphenation, or justification

Those responsibilities must be handled before/after shaping.

## Shape plans and internal decisions
The docs explain that HarfBuzz builds a shape plan from segment properties plus font tables, then uses that plan to shape. Key table-selection behavior:
- glyph classes: `GDEF`, else Unicode fallback
- substitutions: `morx` (AAT), else `GSUB`
- positioning (upstream HarfBuzz): `kerx` (AAT), else `GPOS`, else `kern`, else fallback mark positioning
- positioning (this Go fork/subset): OpenType `GPOS` plus fallback mark positioning; legacy `kern` execution is intentionally omitted

Shape plans can be cached because building them includes script/language lookup decisions and compatibility workarounds.

## OpenType features
HarfBuzz enables a default set of OpenType features automatically (for example `abvm`, `blwm`, `ccmp`, `locl`, `mark`, `mkmk`, `rlig`, plus horizontal defaults such as `kern`/`liga`, or `vert` in vertical text).

User features can override behavior using tag/value/range (`start`, `end`): `1` enables, `0` disables, and some features accept other numeric values. The docs also call out automatic fraction handling around `U+2044` using `numr`, `dnom`, and `frac`.

## Shaper selection
`hb_shape()` auto-selects shapers from font capabilities (for example OpenType vs AAT). `hb_shape_full()` lets callers provide an explicit ordered shaper preference list. `hb_shape_list_shapers()` reports compiled-in shapers.

## How this maps to this Go port
- Entry point: `(*Buffer).Shape(font, features)` in `shape.go`
- Segment context: `Buffer.Props` (`Script`, `Language`, `Direction`) in `harfbuzz.go`/`buffer.go`
- Output storage: `Buffer.Info` and `Buffer.Pos`
- Feature control: `Feature{Tag, Value, Start, End}` and `ParseFeature(...)`
- Internal caching: per-buffer shape-plan cache (`newShapePlanCached`)
- Internal shaper/planner behavior in `ot_shaper.go` follows the project scope above (OpenType-oriented shaping with `GSUB`/`GPOS` and no legacy `kern` pass)

## References
- https://harfbuzz.github.io/what-is-harfbuzz.html
- https://harfbuzz.github.io/shaping-and-shape-plans.html
- https://harfbuzz.github.io/shaping-opentype-features.html
- https://harfbuzz.github.io/shaping-shaper-selection.html
