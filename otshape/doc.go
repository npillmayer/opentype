/*
Package otshape provides a streaming OpenType shaping pipeline.

The package API is centered around [Shape] and [NewShaper]:
  - callers provide shaping parameters ([ShapeOptions]),
  - runes are consumed from a [RuneSource],
  - shaped glyph records are emitted to a [GlyphSink].

Advanced clients may use [ShapeEvents] with an [InputEventSource] to provide
explicit feature push/pop boundaries in-band with rune input.
In this mode, [ShapeOptions.Features] is restricted to global defaults
(FeatureRange with Start==0 and End==0 only).

The pipeline compiles a per-request plan, applies GSUB/GPOS lookups, and supports
script-specific shaper engines through hook interfaces defined in this package.
*/
package otshape

import (
	"fmt"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/schuko/tracing"
)

// NOTDEF is the glyph index for OpenType ".notdef".
const NOTDEF = ot.GlyphIndex(0)

// tracer returns a trace sink for the otshape package namespace.
func tracer() tracing.Trace {
	return tracing.Select("opentype.shaper")
}

// errShaper wraps a message as a user-facing shaping error.
func errShaper(x string) error {
	return fmt.Errorf("OpenType text shaping: %s", x)
}

// assert panics when condition is false.
func assert(condition bool, msg string) {
	if !condition {
		panic(msg)
	}
}
