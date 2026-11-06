package otshape

import "errors"

// ErrGlyphBufferFull means the caller's output slice cannot hold another glyph.
// It is distinct from ErrBufferFull, which describes an internal shaping limit.
var ErrGlyphBufferFull = errors.New("otshape: output glyph buffer full")

// ShapeInto shapes one uniform run into dst and returns the number of records
// written. It uses Shape with default BufferOptions. Output is in logical order,
// including RTL, with cluster and feature indexes relative to this call's input.
//
// Available output space is len(dst), not cap(dst). ShapeInto never grows or
// retains dst, and leaves dst[n:] unchanged. An exact fit succeeds; an additional
// glyph returns ErrGlyphBufferFull. A nil or zero-length dst succeeds only when
// shaping succeeds without producing glyphs. Internal shaping workspace is still
// allocated, and the ordinary shaping limits apply independently of len(dst).
//
// On any error, n counts the records already written. Input consumption and
// destination writes are not rolled back. The result is incomplete and may end
// inside a cluster; use the fragment only after success. To retry after overflow,
// supply a larger destination and a fresh or reset source. The error does not
// report the total required output size.
func (s *Shaper) ShapeInto(params Params, src RuneSource, dst []GlyphRecord) (n int, err error) {
	sink := sliceGlyphSink{dst: dst}
	err = s.Shape(params, src, &sink, BufferOptions{})
	return sink.n, err
}

type sliceGlyphSink struct {
	dst []GlyphRecord
	n   int
}

func (s *sliceGlyphSink) WriteGlyph(g GlyphRecord) error {
	if s.n == len(s.dst) {
		return ErrGlyphBufferFull
	}
	s.dst[s.n] = g
	s.n++
	return nil
}
