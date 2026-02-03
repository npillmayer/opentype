package otlayout

import "github.com/npillmayer/opentype/ot"

// GlyphBuffer is an interface for mutable sequences of glyph IDs.
type GlyphBuffer interface {
	Len() int
	At(i int) ot.GlyphIndex
	Set(i int, g ot.GlyphIndex)
	Replace(i, j int, repl []ot.GlyphIndex) GlyphBuffer
	Insert(i int, glyphs []ot.GlyphIndex) GlyphBuffer
	Delete(i, j int) GlyphBuffer
}

// GlyphSlice is the default GlyphBuffer implementation backed by a slice.
type GlyphSlice []ot.GlyphIndex

func (b GlyphSlice) Len() int {
	return len(b)
}

func (b GlyphSlice) At(i int) ot.GlyphIndex {
	return b[i]
}

func (b GlyphSlice) Set(i int, g ot.GlyphIndex) {
	b[i] = g
}

func (b GlyphSlice) Replace(i, j int, repl []ot.GlyphIndex) GlyphBuffer {
	out := append(b[:i], repl...)
	out = append(out, b[j:]...)
	return GlyphSlice(out)
}

func (b GlyphSlice) Insert(i int, glyphs []ot.GlyphIndex) GlyphBuffer {
	out := append(b[:i], glyphs...)
	out = append(out, b[i:]...)
	return GlyphSlice(out)
}

func (b GlyphSlice) Delete(i, j int) GlyphBuffer {
	out := append(b[:i], b[j:]...)
	return GlyphSlice(out)
}
