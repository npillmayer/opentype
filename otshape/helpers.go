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
