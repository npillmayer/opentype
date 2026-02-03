package otlayout

import "github.com/npillmayer/opentype/ot"

// GlyphBuffer is a mutable sequence of glyph IDs used by GSUB/GPOS application.
//
// Implementations may be simple slices or more complex structures (e.g. a glyph
// buffer with parallel positioning data). The interface is intentionally small
// to let clients provide their own storage while still enabling substitutions.
//
// Contract:
//   - Indices are zero-based in the range [0, Len()).
//   - At/Set operate on the current buffer.
//   - Replace/Insert/Delete return the resulting buffer. They may return the
//     same receiver or a new buffer. Callers must always use the returned value.
//   - Arguments follow slice semantics: Replace(i, j, repl) replaces the range
//     [i:j) with repl; Insert(i, glyphs) inserts before i; Delete(i, j) removes
//     [i:j).
//   - Out-of-range indices are programmer errors and may panic.
//
// Implementations should not retain references to input slices unless that is a
// deliberate part of their design.
type GlyphBuffer interface {
	// Len returns the number of glyphs in the buffer.
	Len() int
	// At returns the glyph at index i.
	At(i int) ot.GlyphIndex
	// Set overwrites the glyph at index i.
	Set(i int, g ot.GlyphIndex)
	// Replace replaces the range [i:j) with repl and returns the resulting buffer.
	Replace(i, j int, repl []ot.GlyphIndex) GlyphBuffer
	// Insert inserts glyphs before index i and returns the resulting buffer.
	Insert(i int, glyphs []ot.GlyphIndex) GlyphBuffer
	// Delete removes the range [i:j) and returns the resulting buffer.
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
