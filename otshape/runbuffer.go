package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

// runBuffer is the internal mutable shaping state (SoA-by-concern).
//
// Slice alignment rule:
// If a side-array is non-nil, its length must equal len(Glyphs).
type runBuffer struct {
	owner  any // only one mutating owner allowed at any time
	front  int // index of the first glyph in the buffer
	end    int // index pointing just behind the last glyph in the buffer
	Glyphs otlayout.GlyphBuffer
	Pos    otlayout.PosBuffer // optional until positioning becomes necessary

	Clusters    []uint32 // optional rune->glyph mapping
	Masks       []uint32 // optional feature/shaping flags
	UnsafeFlags []uint16 // optional line-break/concat safety flags
	Syllables   []uint16 // optional pre-segmented syllable ids (contiguous runs)
	Joiners     []uint8  // optional joiner classes aligned to glyph indices
}

const (
	joinerClassNone uint8 = 0
	joinerClassZWNJ uint8 = 1 << 0
	joinerClassZWJ  uint8 = 1 << 1
)

// newRunBuffer creates an empty run buffer with optional reserved capacity.
func newRunBuffer(capacity int) *runBuffer {
	if capacity < 0 {
		capacity = 0
	}
	return &runBuffer{
		Glyphs: make(otlayout.GlyphBuffer, 0, capacity),
	}
}

// Len returns the glyph length of the run.
func (rb *runBuffer) Len() int {
	if rb == nil {
		return 0
	}
	return rb.Glyphs.Len()
}

// Reset clears the run while retaining allocated capacity.
func (rb *runBuffer) Reset() {
	if rb == nil {
		return
	}
	rb.Glyphs = rb.Glyphs[:0]
	if rb.Pos != nil {
		rb.Pos = rb.Pos[:0]
	}
	if rb.Clusters != nil {
		rb.Clusters = rb.Clusters[:0]
	}
	if rb.Masks != nil {
		rb.Masks = rb.Masks[:0]
	}
	if rb.UnsafeFlags != nil {
		rb.UnsafeFlags = rb.UnsafeFlags[:0]
	}
	if rb.Syllables != nil {
		rb.Syllables = rb.Syllables[:0]
	}
	if rb.Joiners != nil {
		rb.Joiners = rb.Joiners[:0]
	}
}

// EnsurePos allocates/aligns position storage.
func (rb *runBuffer) EnsurePos() {
	if rb == nil {
		return
	}
	if rb.Pos == nil {
		rb.Pos = otlayout.NewPosBuffer(rb.Len())
		return
	}
	if len(rb.Pos) != rb.Len() {
		rb.Pos = rb.Pos.ResizeLike(rb.Glyphs)
	}
}

// EnsureClusters allocates/aligns cluster storage.
func (rb *runBuffer) EnsureClusters() {
	if rb == nil {
		return
	}
	if rb.Clusters == nil {
		rb.Clusters = make([]uint32, rb.Len())
		return
	}
	if len(rb.Clusters) != rb.Len() {
		rb.Clusters = resizeUint32(rb.Clusters, rb.Len())
	}
}

// EnsureMasks allocates/aligns glyph mask storage.
func (rb *runBuffer) EnsureMasks() {
	if rb == nil {
		return
	}
	if rb.Masks == nil {
		rb.Masks = make([]uint32, rb.Len())
		return
	}
	if len(rb.Masks) != rb.Len() {
		rb.Masks = resizeUint32(rb.Masks, rb.Len())
	}
}

// EnsureUnsafeFlags allocates/aligns unsafe flag storage.
func (rb *runBuffer) EnsureUnsafeFlags() {
	if rb == nil {
		return
	}
	if rb.UnsafeFlags == nil {
		rb.UnsafeFlags = make([]uint16, rb.Len())
		return
	}
	if len(rb.UnsafeFlags) != rb.Len() {
		rb.UnsafeFlags = resizeUint16(rb.UnsafeFlags, rb.Len())
	}
}

// EnsureSyllables allocates/aligns syllable ids.
func (rb *runBuffer) EnsureSyllables() {
	if rb == nil {
		return
	}
	if rb.Syllables == nil {
		rb.Syllables = make([]uint16, rb.Len())
		return
	}
	if len(rb.Syllables) != rb.Len() {
		rb.Syllables = resizeUint16(rb.Syllables, rb.Len())
	}
}

// EnsureJoiners allocates/aligns joiner markers.
func (rb *runBuffer) EnsureJoiners() {
	if rb == nil {
		return
	}
	if rb.Joiners == nil {
		rb.Joiners = make([]uint8, rb.Len())
		return
	}
	if len(rb.Joiners) != rb.Len() {
		rb.Joiners = resizeUint8(rb.Joiners, rb.Len())
	}
}

// ApplyEdit mirrors a GSUB edit over all active aligned side arrays.
func (rb *runBuffer) ApplyEdit(edit *otlayout.EditSpan) {
	if rb == nil || edit == nil {
		return
	}
	if edit.From < 0 || edit.To < edit.From || edit.To > rb.Len() || edit.Len < 0 {
		panic("RunBuffer.ApplyEdit: invalid edit span")
	}
	repl := make([]ot.GlyphIndex, edit.Len)
	rb.Glyphs = rb.Glyphs.Replace(edit.From, edit.To, repl)
	if rb.Pos != nil {
		rb.Pos = rb.Pos.ApplyEdit(edit)
	}
	if rb.Clusters != nil {
		rb.Clusters = applyEditUint32(rb.Clusters, edit)
	}
	if rb.Masks != nil {
		rb.Masks = applyEditUint32(rb.Masks, edit)
	}
	if rb.UnsafeFlags != nil {
		rb.UnsafeFlags = applyEditUint16(rb.UnsafeFlags, edit)
	}
	if rb.Syllables != nil {
		rb.Syllables = applyEditUint16(rb.Syllables, edit)
	}
	if rb.Joiners != nil {
		rb.Joiners = applyEditUint8(rb.Joiners, edit)
	}
}

func applyEditUint32(s []uint32, edit *otlayout.EditSpan) []uint32 {
	repl := make([]uint32, edit.Len)
	out := append(s[:edit.From], repl...)
	out = append(out, s[edit.To:]...)
	return out
}

func applyEditUint16(s []uint16, edit *otlayout.EditSpan) []uint16 {
	repl := make([]uint16, edit.Len)
	out := append(s[:edit.From], repl...)
	out = append(out, s[edit.To:]...)
	return out
}

func applyEditUint8(s []uint8, edit *otlayout.EditSpan) []uint8 {
	repl := make([]uint8, edit.Len)
	out := append(s[:edit.From], repl...)
	out = append(out, s[edit.To:]...)
	return out
}

func resizeUint32(s []uint32, n int) []uint32 {
	if n <= len(s) {
		return s[:n]
	}
	out := make([]uint32, n)
	copy(out, s)
	return out
}

func resizeUint16(s []uint16, n int) []uint16 {
	if n <= len(s) {
		return s[:n]
	}
	out := make([]uint16, n)
	copy(out, s)
	return out
}

func resizeUint8(s []uint8, n int) []uint8 {
	if n <= len(s) {
		return s[:n]
	}
	out := make([]uint8, n)
	copy(out, s)
	return out
}
