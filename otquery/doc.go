/*
Package otquery queries metrics and other information from OpenType fonts.

Package otquery provides functions to query layout information from a font. It knows about
the various tables contained in OpenType fonts and which ones to address for queries.
Clients of this package will, amongst other, be:

▪︎ text shapers, such as HarfBuzz (https://harfbuzz.github.io/what-does-harfbuzz-do.html)

▪︎ glyph rasterizers, such as FreeType (https://github.com/golang/freetype)

# Converting font units

[FontMetrics] and [GlyphMetrics] return design metrics in font units. [FontScale]
uses a concrete font's units per em and a requested em size to produce a reusable
conversion factor. Multiply font-unit values by this factor to obtain pixels,
points, or another chosen unit. The same factor applies to the final advances
and offsets emitted by the shaper. Rounding and coordinate transforms belong to
the caller.

# Status

Work in progress. Handling fonts is fiddly and fonts have become complex software
applications in their own right. I often need a break from the vast desert of
bytes (without any sign posts), which is what font data files are at their core. Breaks,
where I talk to myself and ask, this is what you do in your spare time? Really?

No font collections nor variable fonts are supported yet.

# License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © Norbert Pillmayer <norbert@pillmayer.com>
*/
package otquery

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer writes to trace with key 'font.opentype'
func tracer() tracing.Trace {
	return tracing.Select("font.opentype")
}
