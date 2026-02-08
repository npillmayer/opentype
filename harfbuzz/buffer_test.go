package harfbuzz

// ported from harfbuzz/test/api/test-buffer.c Copyright © 2011  Google, Inc. Behdad Esfahbod

var utf32 = [7]rune{'a', 'b', 0x20000, 'd', 'e', 'f', 'g'}

const (
	bufferEmpty = iota
	bufferOneByOne
	bufferUtf32
	bufferNumTypes
)

func newTestBuffer(kind int) *Buffer {
	b := NewBuffer()

	switch kind {
	case bufferEmpty:

	case bufferOneByOne:
		for i := 1; i < len(utf32)-1; i++ {
			b.AddRune(utf32[i])
		}

	case bufferUtf32:
		b.AddRunes(utf32[:], 1, len(utf32)-2)

	}
	return b
}

/*
 * Comparing buffers.
 */

// Flags from comparing two buffers.
//
// For buffers with differing length, the per-glyph comparison is not
// attempted, though we do still scan reference buffer for dotted circle and
// `.notdef` glyphs.
//
// If the buffers have the same length, we compare them glyph-by-glyph and
// report which aspect(s) of the glyph info/position are different.
const (

	/* For buffers with differing length, the per-glyph comparison is not
	 * attempted, though we do still scan reference for dottedcircle / .notdef
	 * glyphs. */
	bdfLengthMismatch = 1 << iota

	/* We want to know if dottedcircle / .notdef glyphs are present in the
	 * reference, as we may not care so much about other differences in this
	 * case. */
	bdfNotdefPresent
	bdfDottedCirclePresent

	/* If the buffers have the same length, we compare them glyph-by-glyph
	 * and report which aspect(s) of the glyph info/position are different. */
	bdfCodepointMismatch
	bdfClusterMismatch
	bdfGlyphFlagsMismatch
	bdfPositionMismatch

	bufferDiffFlagEqual = 0x0000
)

/**
 * hb_buffer_diff:
 * @buffer: a buffer.
 * @reference: other buffer to compare to.
 * @dottedcircleGlyph: glyph id of U+25CC DOTTED CIRCLE, or (hb_codepont_t) -1.
 * @positionFuzz: allowed absolute difference in position values.
 *
 * If dottedcircleGlyph is (hb_codepoint_t) -1 then #bdfDottedCirclePresent
 * and #bdfNotdefPresent are never returned.  This should be used by most
 * callers if just comparing two buffers is needed.
 *
 * Since: 1.5.0
 **/

func bufferDiff(buffer, reference *Buffer, dottedcircleGlyph GID, positionFuzz int32) int {
	result := bufferDiffFlagEqual
	contains := dottedcircleGlyph != ^GID(0)

	count := len(reference.Info)

	if len(buffer.Info) != count {
		/*
		 * we can't compare glyph-by-glyph, but we do want to know if there
		 * are .notdef or dottedcircle glyphs present in the reference buffer
		 */
		info := reference.Info
		for i := 0; i < count; i++ {
			if contains && info[i].Glyph == dottedcircleGlyph {
				result |= bdfDottedCirclePresent
			}
			if contains && info[i].Glyph == 0 {
				result |= bdfNotdefPresent
			}
		}
		result |= bdfLengthMismatch
		return result
	}

	if count == 0 {
		return result
	}

	bufInfo := buffer.Info
	refInfo := reference.Info
	for i := 0; i < count; i++ {
		if bufInfo[i].codepoint != refInfo[i].codepoint {
			result |= bdfCodepointMismatch
		}
		if bufInfo[i].Cluster != refInfo[i].Cluster {
			result |= bdfClusterMismatch
		}
		if (bufInfo[i].Mask^refInfo[i].Mask)&glyphFlagDefined != 0 {
			result |= bdfGlyphFlagsMismatch
		}
		if contains && refInfo[i].Glyph == dottedcircleGlyph {
			result |= bdfDottedCirclePresent
		}
		if contains && refInfo[i].Glyph == 0 {
			result |= bdfNotdefPresent
		}
	}

	isDifferent := func(a, b int32) bool {
		d := a - b
		if d < 0 {
			d = -d
		}
		return d > positionFuzz
	}

	bufPos := buffer.Pos
	refPos := reference.Pos
	for i := 0; i < count; i++ {
		if isDifferent(bufPos[i].XAdvance, refPos[i].XAdvance) ||
			isDifferent(bufPos[i].YAdvance, refPos[i].YAdvance) ||
			isDifferent(bufPos[i].XOffset, refPos[i].XOffset) ||
			isDifferent(bufPos[i].YOffset, refPos[i].YOffset) {
			result |= bdfPositionMismatch
			break
		}
	}

	return result
}
