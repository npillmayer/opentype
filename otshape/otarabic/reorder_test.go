package otarabic

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

type reorderRun struct {
	glyphs   []ot.GlyphIndex
	cps      []rune
	clusters []uint32
	masks    []uint32
}

func (r *reorderRun) Len() int { return len(r.cps) }
func (r *reorderRun) Glyph(i int) ot.GlyphIndex {
	return r.glyphs[i]
}
func (r *reorderRun) SetGlyph(i int, gid ot.GlyphIndex) {
	r.glyphs[i] = gid
}
func (r *reorderRun) Codepoint(i int) rune {
	return r.cps[i]
}
func (r *reorderRun) SetCodepoint(i int, cp rune) {
	r.cps[i] = cp
}
func (r *reorderRun) Cluster(i int) uint32 {
	return r.clusters[i]
}
func (r *reorderRun) SetCluster(i int, cluster uint32) {
	r.clusters[i] = cluster
}
func (r *reorderRun) MergeClusters(start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(r.clusters) {
		end = len(r.clusters)
	}
	if start >= end {
		return
	}
	min := r.clusters[start]
	for i := start + 1; i < end; i++ {
		if r.clusters[i] < min {
			min = r.clusters[i]
		}
	}
	for i := start; i < end; i++ {
		r.clusters[i] = min
	}
}
func (r *reorderRun) Pos(i int) otlayout.PosItem {
	_ = i
	return otlayout.PosItem{AttachTo: -1}
}
func (r *reorderRun) SetPos(i int, pos otlayout.PosItem) {
	_, _ = i, pos
}
func (r *reorderRun) Mask(i int) uint32 {
	return r.masks[i]
}
func (r *reorderRun) SetMask(i int, mask uint32) {
	r.masks[i] = mask
}
func (r *reorderRun) InsertGlyphs(index int, glyphs []ot.GlyphIndex) {
	_, _ = index, glyphs
}
func (r *reorderRun) InsertGlyphCopies(index int, source int, count int) {
	_, _, _ = index, source, count
}
func (r *reorderRun) Swap(i, j int) {
	r.glyphs[i], r.glyphs[j] = r.glyphs[j], r.glyphs[i]
	r.cps[i], r.cps[j] = r.cps[j], r.cps[i]
	r.clusters[i], r.clusters[j] = r.clusters[j], r.clusters[i]
	r.masks[i], r.masks[j] = r.masks[j], r.masks[i]
}

func TestReorderMarksMovesMCMToFront(t *testing.T) {
	// class 30 non-mark-bucket, then MCM(class 230), then non-MCM(class 230)
	run := &reorderRun{
		glyphs:   []ot.GlyphIndex{1, 2, 3},
		cps:      []rune{0x064E, 0x0654, 0x06E1}, // FATHA, HAMZA ABOVE(MCM), SMALL HIGH DOTLESS HEAD OF KHAH(non-MCM)
		clusters: []uint32{3, 4, 5},
		masks:    []uint32{0, 0, 0},
	}
	var s Shaper
	s.ReorderMarks(run, 0, run.Len())

	want := []rune{0x0654, 0x064E, 0x06E1}
	for i, w := range want {
		if run.cps[i] != w {
			t.Fatalf("codepoint[%d]=%U, want %U", i, run.cps[i], w)
		}
	}
	// merge range [start:j) was [0:2), therefore first two entries share cluster.
	if run.clusters[0] != run.clusters[1] {
		t.Fatalf("clusters[0]=%d clusters[1]=%d, expected merged cluster", run.clusters[0], run.clusters[1])
	}
}

func TestReorderMarksNoopWhenBucketStartsNonMCM(t *testing.T) {
	run := &reorderRun{
		glyphs:   []ot.GlyphIndex{1, 2, 3},
		cps:      []rune{0x064E, 0x06E1, 0x0654}, // class30, non-MCM230, MCM230
		clusters: []uint32{1, 2, 3},
		masks:    []uint32{0, 0, 0},
	}
	var s Shaper
	s.ReorderMarks(run, 0, run.Len())

	want := []rune{0x064E, 0x06E1, 0x0654}
	for i, w := range want {
		if run.cps[i] != w {
			t.Fatalf("unexpected reorder at %d: got %U want %U", i, run.cps[i], w)
		}
	}
}
