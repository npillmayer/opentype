package otlayout

import "errors"

// ErrGlyphLimit reports expansion beyond BufferState.MaxGlyphs.
var ErrGlyphLimit = errors.New("otlayout: glyph buffer limit exceeded")

// SourceSpan is a half-open interval in the current raw input window. It is
// deliberately separate from output clusters, which keep their public meaning.
type SourceSpan struct{ Start, End int }

// StreamTrace conservatively records boundaries crossed while executing a
// buffered run. A clear boundary alone is NOT permission to flush: the caller
// must also retain normalization and future lookup context (KeepFrom), and
// reject engines whose dependencies are not observable.
type StreamTrace struct {
	// Sources must initially align with Glyphs; layout edits maintain alignment.
	Sources []SourceSpan
	// Unsafe[i] forbids the raw boundary immediately before scalar i.
	Unsafe []bool
	// KeepFrom is the earliest raw scalar that must remain un-emitted.
	KeepFrom int
	// Disabled requests whole-run buffering when locality cannot be certified.
	Disabled bool
	frames   []traceFrame
}

// NewStreamTrace starts a trace for a raw window; populate Sources before use.
func NewStreamTrace(rawLen int) *StreamTrace {
	return &StreamTrace{Unsafe: make([]bool, rawLen+1), KeepFrom: rawLen}
}

// Join prevents a cut through a dependency. End is exclusive.
func (s *StreamTrace) Join(start, end int) {
	if s == nil {
		return
	}
	start = max(0, start)
	end = min(end, len(s.Unsafe)-1)
	for i := start + 1; i < end; i++ {
		s.Unsafe[i] = true
	}
}

func (s *StreamTrace) begin(index int) {
	if s == nil {
		return
	}
	span := SourceSpan{0, len(s.Unsafe) - 1}
	if index >= 0 && index < len(s.Sources) {
		span = s.Sources[index]
	}
	s.frames = append(s.frames, traceFrame{span: span, origin: span})
}

func (s *StreamTrace) end() {
	if s != nil {
		s.frames = s.frames[:len(s.frames)-1]
	}
}

// Reads remain provisional until an attempt succeeds. Removing missing or
// mismatching context cannot turn that failed attempt into a match; reaching
// the unfinished right edge is different and must retain context immediately.
func (s *StreamTrace) read(index int) {
	if s == nil {
		return
	}
	span := SourceSpan{0, 0}
	if index >= len(s.Sources) {
		span = SourceSpan{len(s.Unsafe) - 1, len(s.Unsafe) - 1}
	} else if index >= 0 {
		span = s.Sources[index]
	}
	for i := range s.frames {
		f := &s.frames[i].span
		f.Start, f.End = min(f.Start, span.Start), max(f.End, span.End)
		if index >= len(s.Sources) {
			s.KeepFrom = min(s.KeepFrom, f.Start)
			s.Join(f.Start, f.End)
		}
	}
}

func (s *StreamTrace) edit(e EditSpan) {
	if s == nil {
		return
	}
	if e.From < 0 || e.To > len(s.Sources) || e.From >= e.To {
		s.Disabled = true
		return
	}
	span := s.Sources[e.From]
	for _, p := range s.Sources[e.From:e.To] {
		span.Start, span.End = min(span.Start, p.Start), max(span.End, p.End)
	}
	s.Join(span.Start, span.End)
	if e.Len == 0 {
		// Deleting an entire cluster can merge into either surviving neighbor.
		// Keep those source groups together, including a provisional right edge.
		lo, hi := span.Start, span.End
		if e.From > 0 {
			lo = min(lo, s.Sources[e.From-1].Start)
		}
		if e.To < len(s.Sources) {
			hi = max(hi, s.Sources[e.To].End)
		} else {
			s.KeepFrom = min(s.KeepFrom, lo)
		}
		s.Join(lo, hi)
	}
	if e.To-e.From == 1 && e.Len == 1 {
		return
	}
	repl := make([]SourceSpan, e.Len)
	for i := range repl {
		repl[i] = span
		if source, ok := e.SurvivorSource(i); ok {
			repl[i] = s.Sources[source]
		}
	}
	s.Sources = append(append(s.Sources[:e.From:e.From], repl...), s.Sources[e.To:]...)
}

func (s *StreamTrace) clone() *StreamTrace {
	if s == nil {
		return nil
	}
	out := *s
	out.Sources = append([]SourceSpan(nil), s.Sources...)
	out.Unsafe = append([]bool(nil), s.Unsafe...)
	out.frames = append([]traceFrame(nil), s.frames...)
	return &out
}

type traceFrame struct{ span, origin SourceSpan }

func (s *StreamTrace) resetAttempt() {
	if s != nil && len(s.frames) > 0 {
		f := &s.frames[len(s.frames)-1]
		f.span = f.origin
	}
}
func (s *StreamTrace) commitAttempt() {
	if s != nil && len(s.frames) > 0 {
		f := s.frames[len(s.frames)-1].span
		s.Join(f.Start, f.End)
	}
}
