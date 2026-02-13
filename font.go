/*
Package opentype is for typeface and font handling.

There is a certain confusion with the nomenclature of typesetting. We will
stick to the following definitions:

▪︎ A "typeface" is a family of fonts. An example is "Helvetica".
This corresponds to a TrueType "collection" (*.ttc).

▪︎ A "scalable font" is a font, i.e. a variant of a typeface with a
certain weight, slant, etc.  An example is "Helvetica regular".

▪︎ A "typecase" is a scaled font, i.e. a font in a certain size for
a certain script and language. The name is reminiscend on the wooden
boxes of typesetters in the era of metal type.
An example is "Helvetica regular 11pt, Latin, en_US".

Please note that Go (Golang) does use the terms "font" and "face"
differently–actually more or less in an opposite manner.

# Status

Does not yet contain methods for font collections (*.ttc), e.g.,
/System/Library/Fonts/Helvetica.ttc on Mac OS.

# Links

OpenType explained:
https://docs.microsoft.com/en-us/typography/opentype/

______________________________________________________________________

# License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © Norbert Pillmayer <norbert@pillmayer.com>
*/
package opentype

import (
	"os"

	"github.com/npillmayer/schuko/tracing"
	"golang.org/x/image/font/sfnt"
)

// tracer writes to trace with key 'tyse.font'
func tracer() tracing.Trace {
	return tracing.Select("opentype")
}

// ScalableFont is an internal representation of an outline-font of type
// TTF of OTF.
type ScalableFont struct {
	Fontname string
	Filepath string     // file path
	Binary   []byte     // raw data
	SFNT     *sfnt.Font // the font's container // TODO: not threadsafe???
}

// LoadOpenTypeFont loads an OpenType font (TTF or OTF) from a file.
func LoadOpenTypeFont(fontfile string) (*ScalableFont, error) {
	bytez, err := os.ReadFile(fontfile)
	if err != nil {
		return nil, err
	}
	return ParseOpenTypeFont(bytez)
}

// ParseOpenTypeFont loads an OpenType font (TTF or OTF) from memory.
func ParseOpenTypeFont(fbytes []byte) (f *ScalableFont, err error) {
	f = &ScalableFont{Binary: fbytes}
	f.SFNT, err = sfnt.Parse(f.Binary)
	if err != nil {
		return nil, err
	}
	if f.Fontname, err = f.SFNT.Name(nil, sfnt.NameIDFull); err == nil {
		tracer().Debugf("loaded and parsed SFNT %s", f.Fontname)
	}
	return
}
