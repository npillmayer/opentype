/*
Package ot provides access to OpenType font tables and features.
Intended audience for this package are:

▪︎ text shapers, such as HarfBuzz (https://harfbuzz.github.io/what-does-harfbuzz-do.html)

▪︎ glyph rasterizers, such as FreeType (https://github.com/golang/freetype)

▪︎ any application needing to have the internal structure of an OpenType font file available,
and possibly extending the methods of package `ot` by handling additional font tables

Package `ot` will not provide functions to interpret any table of a font, but rather
just expose the tables to the client. For example, it is not possible to ask
package `ot` for a kerning distance between two glyphs. Clients have to check
for the availability of kerning information and consult the appropriate table(s)
themselves. From this point of view, `ot` is a low-level package.
Functions for getting kerning values and other layout directives from a font
are homed in a sister package.
Likewise, this package is not intended for font manipulation applications (you may check out
https://pkg.go.dev/github.com/ConradIrwin/font for that).

OpenType fonts contain a whole lot of different tables and sub-tables. This package
strives to make the semantics of the tables accessible, thus has a lot of different
types for the different kinds of OT tables. This makes `ot` a shallow API,
but it will nevertheless abstract away some implementation details of fonts:

▪︎ Format versions: many OT tables may occur in a variety of formats. Tables in `ot` will
hide the concrete format and structure of underlying OT tables.

▪︎ Word size: offsets in OT may either be 2-byte or 4-byte values. Package `ot` will
hide offset-related details (see section below).

▪︎ Bugs in fonts: many fonts in the wild contain entries that—strictly speaking—infringe
upon the OT specification (for example, Calibri has an overflow in a 'kern' table variable),
but an application using it should not fail because of recoverable errors.
Package `ot` will try to circumvent known bugs in common fonts.

# Status

Work in progress. Handling fonts is fiddly and fonts have become complex software
applications in their own right. I often need a break from the vast desert of
bytes (without any sign posts), which is what font data files are at their core. A break
where I talk to myself and ask, this is what you do in your spare time? Really?

No font collections nor variable fonts are supported yet, but will be in time.

# License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © Norbert Pillmayer <norbert@pillmayer.com>

Some code has originally been copied over from golang.org/x/image/font/sfnt/cmap.go,
as the cmap-routines are not accessible through the sfnt package's API.
I understand this to be legally okay as long as the Go license information
stays intact.

	Copyright 2017 The Go Authors. All rights reserved.
	Use of this source code is governed by a BSD-style
	license that can be found in the LICENSE file.

The license file mentioned can be found in file GO-LICENSE at the root folder
of this module.
*/
package ot

/*
There are (at least) two Go packages around for parsing SFNT fonts:

▪ https://pkg.go.dev/golang.org/x/image/font/sfnt

▪ https://pkg.go.dev/github.com/ConradIrwin/font/sfnt

It's always a good idea to prefer packages from the Go core team, and the
x/image/font/sfnt package is certainly well suited for rasterizing applications
(as proven by the test cases). However, it is less well suited as a basis for
the task of text-shaping. This task requires access to the tables contained in
a font and means of navigating them, cross-checking entries, applying different
shaping algorithms, etc. Moreover, the API is not intended to be extended by
other packages, but has been programmed with a concrete target in mind.

ConradIrwin/font allows access to the font tables it has parsed. However, its
focus is on font file manipulation (read in ⇒ manipulate ⇒ export), thus
access to tables means more or less access to the tables binaries and
doing much of the interpretation on the client side. I started out pursuing this
approach, but at the end abondened it. The main reason for this is that I
prefer the approach of the Go core team of keeping the initial font binary
in memory, and not copying out too much into separate buffers or data structures.
I need to have the binary data in memory anyway, as for complex-script shaping
we will rely on HarfBuzz for a long time to come (HarfBuzz receives a font
as a byte-blob and does its own font parsing).

A better suited blueprint of what we're trying to accomplish is this implementation
in Rust:

▪︎ https://github.com/bodoni/opentype

// Valuable resource:
// http://opentypecookbook.com/


*/

import (
	"fmt"

	"github.com/npillmayer/schuko/tracing"
)

// tracer writes to trace with key 'font.opentype'
func tracer() tracing.Trace {
	return tracing.Select("font.opentype")
}

func assertEqualInt(name string, a, b int) {
	if a != b {
		panic(fmt.Sprintf("assertion [%s] failed: %d != %d", name, a, b))
	}
}

func assertEqualUint16(name string, a, b uint16) {
	if a != b {
		panic(fmt.Sprintf("assertion [%s] failed: %d != %d", name, a, b))
	}
}

func assertIsType[T any](name string, x any) {
	if _, ok := x.(T); !ok {
		panic(fmt.Sprintf("assertion [%s] failed: wrong type for %v: %T", name, x, x))
	}
}
