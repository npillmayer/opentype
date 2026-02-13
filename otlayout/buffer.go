package otlayout

import "github.com/npillmayer/opentype/ot"

// GlyphBuffer is a mutable sequence of glyph IDs used by GSUB/GPOS application.
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
// GlyphBuffer is a concrete, slice-backed buffer.
type GlyphBuffer []ot.GlyphIndex

func (b GlyphBuffer) Len() int {
	return len(b)
}

func (b GlyphBuffer) At(i int) ot.GlyphIndex {
	return b[i]
}

func (b GlyphBuffer) Set(i int, g ot.GlyphIndex) {
	b[i] = g
}

func (b GlyphBuffer) Replace(i, j int, repl []ot.GlyphIndex) GlyphBuffer {
	out := append(b[:i:i], repl...)
	out = append(out, b[j:]...)
	return GlyphBuffer(out)
}

func (b GlyphBuffer) Insert(i int, glyphs []ot.GlyphIndex) GlyphBuffer {
	out := append(b[:i:i], glyphs...)
	out = append(out, b[i:]...)
	return GlyphBuffer(out)
}

func (b GlyphBuffer) Delete(i, j int) GlyphBuffer {
	out := append(b[:i:i], b[j:]...)
	return GlyphBuffer(out)
}
