/*
Package otlayout provides access to OpenType font layout features.

# Status

Work in progress.

# License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © Norbert Pillmayer <norbert@pillmayer.com>
*/
package otlayout

import (
	"fmt"

	"github.com/npillmayer/schuko/tracing"
)

// errFontFormat produces user level errors for font parsing.
func errFontFormat(message string) error {
	return fmt.Errorf("OpenType font format: %s", message)
}

// tracer writes to trace with key 'font.opentype'
func tracer() tracing.Trace {
	return tracing.Select("tyse.fonts")
}
