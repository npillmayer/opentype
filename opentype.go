/*
Package opentype handles OpenType fonts.

# License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © Norbert Pillmayer <norbert@pillmayer.com>
*/
package opentype

import (
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// FromBinary parses raw OpenType bytes and returns a decoded font.
//
// The input is expected to contain a complete single-font SFNT stream.
// It must not change after parsing for the font to be usable for the font to be usa
func FromBinary(data []byte) (*ot.Font, error) {
	return ot.Parse(data)
}

// FamilyName extracts family and subfamily names from a font's `name` table.
//
// Returned values are empty if no matching records exist or if records cannot be
// decoded by the current name-table reader.
func FamilyName(f *ot.Font) (family, subfamily string) {
	for nameId, stringValue := range otquery.NamesRange(f) {
		switch nameId {
		case sfnt.NameIDFamily:
			family = stringValue
		case sfnt.NameIDSubfamily:
			subfamily = stringValue
		}
	}
	return
}

type glyphCollector struct {
	glyphs []otshape.GlyphRecord
}

// WriteGlyph appends one shaped glyph record to the collector.
func (c *glyphCollector) WriteGlyph(g otshape.GlyphRecord) error {
	c.glyphs = append(c.glyphs, g)
	return nil
}

// ShapeLatinText shapes UTF-8 text as one left-to-right run in “Latin” (i.e.,
// Western) script.
//
// It uses the core OpenType shaper with script `Latn` and language `en`, and
// returns glyph records in output order. If `otf` is nil or `text` is empty, it
// does nothing.
//
// This is a convenience API for a very common use-case of short pieces of Western
// test. Clients who need more control over shaping, such as shaping multiple runs or
// using different scripts and languages, need to use the `otshape` package
// directly. Package `otshape` employs a streaming API that allows clients to
// manage memory allocation more efficiently.
func ShapeLatinText(otf *ot.Font, text string) ([]otshape.GlyphRecord, error) {
	if otf == nil || text == "" {
		return nil, nil
	}
	params := otshape.Params{
		Font:      otf,
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
	}
	options := otshape.BufferOptions{
		FlushBoundary: otshape.FlushOnRunBoundary,
	}
	src := strings.NewReader(string(text))
	sink := &glyphCollector{
		glyphs: make([]otshape.GlyphRecord, 0, len(text)+16),
	}
	coreEngine := otcore.New()
	shaper := otshape.NewShaper(coreEngine)
	err := shaper.Shape(params, src, sink, options)
	return sink.glyphs, err
}
