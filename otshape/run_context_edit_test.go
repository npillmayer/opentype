package otshape

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestRunContextInsertGlyphsAlignsSideArrays(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 20)
	run.Pos = otlayout.NewPosBuffer(2)
	run.Pos[0].XAdvance = 11
	run.Pos[1].XAdvance = 22
	run.Codepoints = []rune{'a', 'b'}
	run.Clusters = []uint32{7, 8}
	run.Masks = []uint32{0x11, 0x22}
	run.UnsafeFlags = []uint16{1, 2}
	run.Syllables = []uint16{3, 4}
	run.Joiners = []uint8{5, 6}

	rc := newRunContext(run)
	rc.InsertGlyphs(1, []ot.GlyphIndex{30, 40})

	if got := run.Len(); got != 4 {
		t.Fatalf("run length = %d, want 4", got)
	}
	wantGlyphs := []ot.GlyphIndex{10, 30, 40, 20}
	for i, w := range wantGlyphs {
		if run.Glyphs[i] != w {
			t.Fatalf("glyph[%d]=%d, want %d", i, run.Glyphs[i], w)
		}
	}
	if len(run.Pos) != 4 || len(run.Codepoints) != 4 || len(run.Clusters) != 4 ||
		len(run.Masks) != 4 || len(run.UnsafeFlags) != 4 || len(run.Syllables) != 4 || len(run.Joiners) != 4 {
		t.Fatalf("side-array lengths not aligned after insert")
	}
	if run.Clusters[1] != 7 || run.Clusters[2] != 7 {
		t.Fatalf("inserted clusters = [%d,%d], want inherited [7,7]", run.Clusters[1], run.Clusters[2])
	}
	if run.Codepoints[1] != 0 || run.Codepoints[2] != 0 {
		t.Fatalf("inserted codepoints = [%U,%U], want zero defaults", run.Codepoints[1], run.Codepoints[2])
	}
	if run.Masks[1] != 0 || run.Masks[2] != 0 {
		t.Fatalf("inserted masks = [0x%X,0x%X], want zero defaults", run.Masks[1], run.Masks[2])
	}
	if run.Pos[1].AttachTo != -1 || run.Pos[2].AttachTo != -1 {
		t.Fatalf("inserted pos AttachTo = [%d,%d], want [-1,-1]", run.Pos[1].AttachTo, run.Pos[2].AttachTo)
	}
}

func TestRunContextInsertGlyphCopiesReplicatesSourceRecord(t *testing.T) {
	run := newRunBuffer(0)
	run.Glyphs = append(run.Glyphs, 10, 20)
	run.Pos = otlayout.NewPosBuffer(2)
	run.Pos[0] = otlayout.PosItem{XAdvance: 15, XOffset: -2, AttachTo: -1}
	run.Pos[1] = otlayout.PosItem{XAdvance: 25, XOffset: -3, AttachTo: -1}
	run.Codepoints = []rune{'x', 'y'}
	run.Clusters = []uint32{3, 9}
	run.Masks = []uint32{0xAA, 0xBB}
	run.UnsafeFlags = []uint16{8, 9}
	run.Syllables = []uint16{4, 5}
	run.Joiners = []uint8{1, 2}

	rc := newRunContext(run)
	rc.InsertGlyphCopies(1, 0, 2)

	wantGlyphs := []ot.GlyphIndex{10, 10, 10, 20}
	for i, w := range wantGlyphs {
		if run.Glyphs[i] != w {
			t.Fatalf("glyph[%d]=%d, want %d", i, run.Glyphs[i], w)
		}
	}
	for _, inx := range []int{1, 2} {
		if run.Codepoints[inx] != 'x' {
			t.Fatalf("codepoint[%d]=%U, want 'x'", inx, run.Codepoints[inx])
		}
		if run.Clusters[inx] != 3 {
			t.Fatalf("cluster[%d]=%d, want 3", inx, run.Clusters[inx])
		}
		if run.Masks[inx] != 0xAA {
			t.Fatalf("mask[%d]=0x%X, want 0xAA", inx, run.Masks[inx])
		}
		if run.UnsafeFlags[inx] != 8 {
			t.Fatalf("unsafe[%d]=%d, want 8", inx, run.UnsafeFlags[inx])
		}
		if run.Syllables[inx] != 4 {
			t.Fatalf("syllable[%d]=%d, want 4", inx, run.Syllables[inx])
		}
		if run.Joiners[inx] != 1 {
			t.Fatalf("joiner[%d]=%d, want 1", inx, run.Joiners[inx])
		}
		if run.Pos[inx].XAdvance != 15 || run.Pos[inx].XOffset != -2 {
			t.Fatalf("pos[%d]={adv=%d,off=%d}, want {15,-2}", inx, run.Pos[inx].XAdvance, run.Pos[inx].XOffset)
		}
	}
}
