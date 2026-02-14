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

	Codepoints  []rune   // optional codepoint alignment for normalization/reorder hooks
	Clusters    []uint32 // optional rune->glyph mapping
	PlanIDs     []uint16 // optional plan-boundary marker: active plan id per glyph
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
	if rb.Codepoints != nil {
		rb.Codepoints = rb.Codepoints[:0]
	}
	if rb.Clusters != nil {
		rb.Clusters = rb.Clusters[:0]
	}
	if rb.PlanIDs != nil {
		rb.PlanIDs = rb.PlanIDs[:0]
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

// PrepareForMappedRun resets rb for rune->glyph mapping.
//
// It clears existing content, disables shaping-stage side arrays, and enables
// mapping-stage side arrays (codepoints/clusters and optional plan IDs).
func (rb *runBuffer) PrepareForMappedRun(withPlanIDs bool, reserve int) {
	if rb == nil {
		return
	}
	if reserve < 0 {
		reserve = 0
	}
	rb.Reset()

	// Mapping starts with rune-derived metadata only; shaped-state arrays are
	// lazily enabled by later pipeline stages.
	rb.Pos = nil
	rb.Masks = nil
	rb.UnsafeFlags = nil
	rb.Syllables = nil
	rb.Joiners = nil

	rb.UseCodepoints()
	rb.UseClusters()
	if withPlanIDs {
		rb.UsePlanIDs()
	} else {
		rb.PlanIDs = nil
	}
	if reserve > 0 {
		rb.ReserveGlyphs(reserve)
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

// EnsureCodepoints allocates/aligns codepoint storage.
func (rb *runBuffer) EnsureCodepoints() {
	if rb == nil {
		return
	}
	if rb.Codepoints == nil {
		rb.Codepoints = make([]rune, rb.Len())
		return
	}
	if len(rb.Codepoints) != rb.Len() {
		rb.Codepoints = resizeRunes(rb.Codepoints, rb.Len())
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

// EnsurePlanIDs allocates/aligns per-glyph plan ids.
func (rb *runBuffer) EnsurePlanIDs() {
	if rb == nil {
		return
	}
	if rb.PlanIDs == nil {
		rb.PlanIDs = make([]uint16, rb.Len())
		return
	}
	if len(rb.PlanIDs) != rb.Len() {
		rb.PlanIDs = resizeUint16(rb.PlanIDs, rb.Len())
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

// ReserveGlyphs ensures capacity for at least extra additional glyph records.
//
// If side arrays are active, their capacities are grown to match as well.
func (rb *runBuffer) ReserveGlyphs(extra int) {
	if rb == nil || extra <= 0 {
		return
	}
	n := rb.Len()
	need := n + extra
	rb.Glyphs = reserveGlyphBuffer(rb.Glyphs, need)
	if rb.Pos != nil {
		assert(len(rb.Pos) == n, "run buffer alignment violated for Pos")
		rb.Pos = reservePosBuffer(rb.Pos, need)
	}
	if rb.Codepoints != nil {
		assert(len(rb.Codepoints) == n, "run buffer alignment violated for Codepoints")
		rb.Codepoints = reserveRunes(rb.Codepoints, need)
	}
	if rb.Clusters != nil {
		assert(len(rb.Clusters) == n, "run buffer alignment violated for Clusters")
		rb.Clusters = reserveUint32(rb.Clusters, need)
	}
	if rb.PlanIDs != nil {
		assert(len(rb.PlanIDs) == n, "run buffer alignment violated for PlanIDs")
		rb.PlanIDs = reserveUint16(rb.PlanIDs, need)
	}
	if rb.Masks != nil {
		assert(len(rb.Masks) == n, "run buffer alignment violated for Masks")
		rb.Masks = reserveUint32(rb.Masks, need)
	}
	if rb.UnsafeFlags != nil {
		assert(len(rb.UnsafeFlags) == n, "run buffer alignment violated for UnsafeFlags")
		rb.UnsafeFlags = reserveUint16(rb.UnsafeFlags, need)
	}
	if rb.Syllables != nil {
		assert(len(rb.Syllables) == n, "run buffer alignment violated for Syllables")
		rb.Syllables = reserveUint16(rb.Syllables, need)
	}
	if rb.Joiners != nil {
		assert(len(rb.Joiners) == n, "run buffer alignment violated for Joiners")
		rb.Joiners = reserveUint8(rb.Joiners, need)
	}
}

// UsePos activates per-glyph positioning storage.
func (rb *runBuffer) UsePos() {
	if rb == nil {
		return
	}
	if rb.Pos != nil {
		rb.EnsurePos()
		return
	}
	if rb.Len() == 0 {
		rb.Pos = make(otlayout.PosBuffer, 0, cap(rb.Glyphs))
		return
	}
	rb.Pos = otlayout.NewPosBuffer(rb.Len())
}

// UseCodepoints activates per-glyph codepoint storage.
func (rb *runBuffer) UseCodepoints() {
	if rb == nil {
		return
	}
	if rb.Codepoints != nil {
		rb.EnsureCodepoints()
		return
	}
	n := rb.Len()
	rb.Codepoints = make([]rune, n, maxInt(cap(rb.Glyphs), n))
}

// UseClusters activates per-glyph cluster storage.
func (rb *runBuffer) UseClusters() {
	if rb == nil {
		return
	}
	if rb.Clusters != nil {
		rb.EnsureClusters()
		return
	}
	n := rb.Len()
	rb.Clusters = make([]uint32, n, maxInt(cap(rb.Glyphs), n))
}

// UsePlanIDs activates per-glyph plan id storage.
func (rb *runBuffer) UsePlanIDs() {
	if rb == nil {
		return
	}
	if rb.PlanIDs != nil {
		rb.EnsurePlanIDs()
		return
	}
	n := rb.Len()
	rb.PlanIDs = make([]uint16, n, maxInt(cap(rb.Glyphs), n))
}

// UseMasks activates per-glyph mask storage.
func (rb *runBuffer) UseMasks() {
	if rb == nil {
		return
	}
	if rb.Masks != nil {
		rb.EnsureMasks()
		return
	}
	n := rb.Len()
	rb.Masks = make([]uint32, n, maxInt(cap(rb.Glyphs), n))
}

// UseUnsafeFlags activates per-glyph unsafe-flag storage.
func (rb *runBuffer) UseUnsafeFlags() {
	if rb == nil {
		return
	}
	if rb.UnsafeFlags != nil {
		rb.EnsureUnsafeFlags()
		return
	}
	n := rb.Len()
	rb.UnsafeFlags = make([]uint16, n, maxInt(cap(rb.Glyphs), n))
}

// UseSyllables activates per-glyph syllable-id storage.
func (rb *runBuffer) UseSyllables() {
	if rb == nil {
		return
	}
	if rb.Syllables != nil {
		rb.EnsureSyllables()
		return
	}
	n := rb.Len()
	rb.Syllables = make([]uint16, n, maxInt(cap(rb.Glyphs), n))
}

// UseJoiners activates per-glyph joiner-class storage.
func (rb *runBuffer) UseJoiners() {
	if rb == nil {
		return
	}
	if rb.Joiners != nil {
		rb.EnsureJoiners()
		return
	}
	n := rb.Len()
	rb.Joiners = make([]uint8, n, maxInt(cap(rb.Glyphs), n))
}

// AppendGlyph appends one glyph record and default values for active side arrays.
func (rb *runBuffer) AppendGlyph(gid ot.GlyphIndex) int {
	assert(rb != nil, "run buffer is nil")
	n := rb.Len()
	rb.Glyphs = append(rb.Glyphs, gid)
	if rb.Pos != nil {
		assert(len(rb.Pos) == n, "run buffer alignment violated for Pos")
		rb.Pos = append(rb.Pos, otlayout.PosItem{AttachTo: -1})
	}
	if rb.Codepoints != nil {
		assert(len(rb.Codepoints) == n, "run buffer alignment violated for Codepoints")
		rb.Codepoints = append(rb.Codepoints, 0)
	}
	if rb.Clusters != nil {
		assert(len(rb.Clusters) == n, "run buffer alignment violated for Clusters")
		rb.Clusters = append(rb.Clusters, 0)
	}
	if rb.PlanIDs != nil {
		assert(len(rb.PlanIDs) == n, "run buffer alignment violated for PlanIDs")
		rb.PlanIDs = append(rb.PlanIDs, 0)
	}
	if rb.Masks != nil {
		assert(len(rb.Masks) == n, "run buffer alignment violated for Masks")
		rb.Masks = append(rb.Masks, 0)
	}
	if rb.UnsafeFlags != nil {
		assert(len(rb.UnsafeFlags) == n, "run buffer alignment violated for UnsafeFlags")
		rb.UnsafeFlags = append(rb.UnsafeFlags, 0)
	}
	if rb.Syllables != nil {
		assert(len(rb.Syllables) == n, "run buffer alignment violated for Syllables")
		rb.Syllables = append(rb.Syllables, 0)
	}
	if rb.Joiners != nil {
		assert(len(rb.Joiners) == n, "run buffer alignment violated for Joiners")
		rb.Joiners = append(rb.Joiners, 0)
	}
	return n
}

// AppendMappedGlyph appends one mapped glyph with codepoint/cluster metadata.
//
// If withPlanID is true, plan id storage is activated lazily when needed.
func (rb *runBuffer) AppendMappedGlyph(gid ot.GlyphIndex, cp rune, cluster uint32, planID uint16, withPlanID bool) int {
	assert(rb != nil, "run buffer is nil")
	if rb.Codepoints == nil {
		rb.UseCodepoints()
	}
	if rb.Clusters == nil {
		rb.UseClusters()
	}
	if withPlanID && rb.PlanIDs == nil {
		rb.UsePlanIDs()
	}
	i := rb.AppendGlyph(gid)
	rb.Codepoints[i] = cp
	rb.Clusters[i] = cluster
	if withPlanID {
		rb.PlanIDs[i] = planID
	}
	return i
}

// AppendRun appends all glyph records from src to rb while preserving alignment.
func (rb *runBuffer) AppendRun(src *runBuffer) {
	if rb == nil || src == nil {
		return
	}
	srcLen := src.Len()
	if srcLen == 0 {
		return
	}
	if len(src.Pos) == srcLen {
		rb.UsePos()
	}
	if len(src.Codepoints) == srcLen {
		rb.UseCodepoints()
	}
	if len(src.Clusters) == srcLen {
		rb.UseClusters()
	}
	if len(src.PlanIDs) == srcLen {
		rb.UsePlanIDs()
	}
	if len(src.Masks) == srcLen {
		rb.UseMasks()
	}
	if len(src.UnsafeFlags) == srcLen {
		rb.UseUnsafeFlags()
	}
	if len(src.Syllables) == srcLen {
		rb.UseSyllables()
	}
	if len(src.Joiners) == srcLen {
		rb.UseJoiners()
	}
	rb.ReserveGlyphs(srcLen)
	for i := 0; i < srcLen; i++ {
		j := rb.AppendGlyph(src.Glyphs[i])
		if len(src.Pos) == srcLen && len(rb.Pos) == rb.Len() {
			rb.Pos[j] = src.Pos[i]
		}
		if len(src.Codepoints) == srcLen && len(rb.Codepoints) == rb.Len() {
			rb.Codepoints[j] = src.Codepoints[i]
		}
		if len(src.Clusters) == srcLen && len(rb.Clusters) == rb.Len() {
			rb.Clusters[j] = src.Clusters[i]
		}
		if len(src.PlanIDs) == srcLen && len(rb.PlanIDs) == rb.Len() {
			rb.PlanIDs[j] = src.PlanIDs[i]
		}
		if len(src.Masks) == srcLen && len(rb.Masks) == rb.Len() {
			rb.Masks[j] = src.Masks[i]
		}
		if len(src.UnsafeFlags) == srcLen && len(rb.UnsafeFlags) == rb.Len() {
			rb.UnsafeFlags[j] = src.UnsafeFlags[i]
		}
		if len(src.Syllables) == srcLen && len(rb.Syllables) == rb.Len() {
			rb.Syllables[j] = src.Syllables[i]
		}
		if len(src.Joiners) == srcLen && len(rb.Joiners) == rb.Len() {
			rb.Joiners[j] = src.Joiners[i]
		}
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
	if rb.Codepoints != nil {
		rb.Codepoints = applyEditRunes(rb.Codepoints, edit)
	}
	if rb.Clusters != nil {
		rb.Clusters = applyEditUint32(rb.Clusters, edit)
	}
	if rb.PlanIDs != nil {
		rb.PlanIDs = applyEditUint16(rb.PlanIDs, edit)
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

// InsertGlyphs inserts glyphs at index and keeps all active side arrays aligned.
// Inserted side-array slots are initialized to defaults (or inherited cluster id).
func (rb *runBuffer) InsertGlyphs(index int, glyphs []ot.GlyphIndex) (int, int) {
	if rb == nil || len(glyphs) == 0 {
		return 0, 0
	}
	n := rb.Len()
	if index < 0 {
		index = 0
	}
	if index > n {
		index = n
	}
	insertLen := len(glyphs)

	clusterSeed := uint32(0)
	if len(rb.Clusters) == n {
		switch {
		case index > 0:
			clusterSeed = rb.Clusters[index-1]
		case n > 0:
			clusterSeed = rb.Clusters[0]
		}
	}
	planSeed := uint16(0)
	if len(rb.PlanIDs) == n {
		switch {
		case index > 0:
			planSeed = rb.PlanIDs[index-1]
		case n > 0:
			planSeed = rb.PlanIDs[0]
		}
	}

	edit := &otlayout.EditSpan{From: index, To: index, Len: insertLen}
	rb.ApplyEdit(edit)
	copy(rb.Glyphs[index:index+insertLen], glyphs)

	if len(rb.Clusters) == rb.Len() {
		for i := index; i < index+insertLen; i++ {
			rb.Clusters[i] = clusterSeed
		}
	}
	if len(rb.PlanIDs) == rb.Len() {
		for i := index; i < index+insertLen; i++ {
			rb.PlanIDs[i] = planSeed
		}
	}
	return index, index + insertLen
}

// InsertGlyphCopies inserts `count` copies of a source index at `index`.
// All active side arrays are copied from the source record for inserted slots.
func (rb *runBuffer) InsertGlyphCopies(index int, source int, count int) (int, int) {
	if rb == nil || count <= 0 {
		return 0, 0
	}
	n := rb.Len()
	if source < 0 || source >= n {
		return 0, 0
	}
	if index < 0 {
		index = 0
	}
	if index > n {
		index = n
	}

	gid := rb.Glyphs[source]
	insertGlyphs := make([]ot.GlyphIndex, count)
	for i := range insertGlyphs {
		insertGlyphs[i] = gid
	}

	hasPos := len(rb.Pos) == n
	var pos otlayout.PosItem
	if hasPos {
		pos = rb.Pos[source]
	}
	hasCodepoints := len(rb.Codepoints) == n
	var cp rune
	if hasCodepoints {
		cp = rb.Codepoints[source]
	}
	hasClusters := len(rb.Clusters) == n
	var cluster uint32
	if hasClusters {
		cluster = rb.Clusters[source]
	}
	hasMasks := len(rb.Masks) == n
	var mask uint32
	if hasMasks {
		mask = rb.Masks[source]
	}
	hasPlanIDs := len(rb.PlanIDs) == n
	var planID uint16
	if hasPlanIDs {
		planID = rb.PlanIDs[source]
	}
	hasUnsafe := len(rb.UnsafeFlags) == n
	var unsafe uint16
	if hasUnsafe {
		unsafe = rb.UnsafeFlags[source]
	}
	hasSyllables := len(rb.Syllables) == n
	var syllable uint16
	if hasSyllables {
		syllable = rb.Syllables[source]
	}
	hasJoiners := len(rb.Joiners) == n
	var joiner uint8
	if hasJoiners {
		joiner = rb.Joiners[source]
	}

	start, end := rb.InsertGlyphs(index, insertGlyphs)
	for i := start; i < end; i++ {
		if hasPos {
			rb.Pos[i] = pos
		}
		if hasCodepoints {
			rb.Codepoints[i] = cp
		}
		if hasClusters {
			rb.Clusters[i] = cluster
		}
		if hasMasks {
			rb.Masks[i] = mask
		}
		if hasPlanIDs {
			rb.PlanIDs[i] = planID
		}
		if hasUnsafe {
			rb.UnsafeFlags[i] = unsafe
		}
		if hasSyllables {
			rb.Syllables[i] = syllable
		}
		if hasJoiners {
			rb.Joiners[i] = joiner
		}
	}
	return start, end
}

func applyEditUint32(s []uint32, edit *otlayout.EditSpan) []uint32 {
	repl := make([]uint32, edit.Len)
	out := append(s[:edit.From:edit.From], repl...)
	out = append(out, s[edit.To:]...)
	return out
}

func applyEditUint16(s []uint16, edit *otlayout.EditSpan) []uint16 {
	repl := make([]uint16, edit.Len)
	out := append(s[:edit.From:edit.From], repl...)
	out = append(out, s[edit.To:]...)
	return out
}

func applyEditUint8(s []uint8, edit *otlayout.EditSpan) []uint8 {
	repl := make([]uint8, edit.Len)
	out := append(s[:edit.From:edit.From], repl...)
	out = append(out, s[edit.To:]...)
	return out
}

func applyEditRunes(s []rune, edit *otlayout.EditSpan) []rune {
	repl := make([]rune, edit.Len)
	out := append(s[:edit.From:edit.From], repl...)
	out = append(out, s[edit.To:]...)
	return out
}

func reserveGlyphBuffer(s otlayout.GlyphBuffer, n int) otlayout.GlyphBuffer {
	if n <= cap(s) {
		return s
	}
	out := make(otlayout.GlyphBuffer, len(s), n)
	copy(out, s)
	return out
}

func reservePosBuffer(s otlayout.PosBuffer, n int) otlayout.PosBuffer {
	if n <= cap(s) {
		return s
	}
	out := make(otlayout.PosBuffer, len(s), n)
	copy(out, s)
	return out
}

func reserveUint32(s []uint32, n int) []uint32 {
	if n <= cap(s) {
		return s
	}
	out := make([]uint32, len(s), n)
	copy(out, s)
	return out
}

func reserveUint16(s []uint16, n int) []uint16 {
	if n <= cap(s) {
		return s
	}
	out := make([]uint16, len(s), n)
	copy(out, s)
	return out
}

func reserveUint8(s []uint8, n int) []uint8 {
	if n <= cap(s) {
		return s
	}
	out := make([]uint8, len(s), n)
	copy(out, s)
	return out
}

func reserveRunes(s []rune, n int) []rune {
	if n <= cap(s) {
		return s
	}
	out := make([]rune, len(s), n)
	copy(out, s)
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

func resizeRunes(s []rune, n int) []rune {
	if n <= len(s) {
		return s[:n]
	}
	out := make([]rune, n)
	copy(out, s)
	return out
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}
