package otshape

import "testing"

func TestRunBufferAppendMappedGlyphKeepsAlignment(t *testing.T) {
	rb := newRunBuffer(2)
	rb.UseCodepoints()
	rb.UseClusters()
	rb.UsePlanIDs()
	rb.ReserveGlyphs(3)

	rb.AppendMappedGlyph(10, 'a', 1, 7, true)
	rb.AppendMappedGlyph(11, 'b', 2, 7, true)
	rb.AppendMappedGlyph(12, 'c', 3, 9, true)

	if rb.Len() != 3 {
		t.Fatalf("len=%d, want 3", rb.Len())
	}
	if len(rb.Codepoints) != 3 || len(rb.Clusters) != 3 || len(rb.PlanIDs) != 3 {
		t.Fatalf("aligned lengths not preserved: cp=%d cl=%d pid=%d",
			len(rb.Codepoints), len(rb.Clusters), len(rb.PlanIDs))
	}
	if got := rb.Codepoints[2]; got != 'c' {
		t.Fatalf("codepoint[2]=%q, want %q", got, 'c')
	}
	if got := rb.Clusters[0]; got != 1 {
		t.Fatalf("cluster[0]=%d, want 1", got)
	}
	if got := rb.PlanIDs[2]; got != 9 {
		t.Fatalf("planID[2]=%d, want 9", got)
	}
}

func TestRunBufferAppendRunActivatesSourceArrays(t *testing.T) {
	src := newRunBuffer(2)
	src.UseCodepoints()
	src.UseClusters()
	src.UsePlanIDs()
	src.AppendMappedGlyph(20, 'x', 4, 2, true)
	src.AppendMappedGlyph(21, 'y', 5, 3, true)

	dst := newRunBuffer(1)
	dst.AppendGlyph(10)
	dst.AppendRun(src)

	if dst.Len() != 3 {
		t.Fatalf("len=%d, want 3", dst.Len())
	}
	if len(dst.Codepoints) != 3 || len(dst.Clusters) != 3 || len(dst.PlanIDs) != 3 {
		t.Fatalf("destination alignment missing after append: cp=%d cl=%d pid=%d",
			len(dst.Codepoints), len(dst.Clusters), len(dst.PlanIDs))
	}
	if got := dst.Glyphs[1]; got != 20 {
		t.Fatalf("glyph[1]=%d, want 20", got)
	}
	if got := dst.Codepoints[1]; got != 'x' {
		t.Fatalf("codepoint[1]=%q, want %q", got, 'x')
	}
	if got := dst.PlanIDs[2]; got != 3 {
		t.Fatalf("planID[2]=%d, want 3", got)
	}
}

func TestRunBufferPrepareForMappedRunResetsLifecycleState(t *testing.T) {
	rb := newRunBuffer(4)
	rb.UsePos()
	rb.UseCodepoints()
	rb.UseClusters()
	rb.UsePlanIDs()
	rb.UseMasks()
	rb.UseUnsafeFlags()
	rb.UseSyllables()
	rb.UseJoiners()
	rb.AppendMappedGlyph(40, 'p', 10, 5, true)
	rb.Pos[0].XAdvance = 100
	rb.Masks[0] = 7
	rb.UnsafeFlags[0] = 2
	rb.Syllables[0] = 9
	rb.Joiners[0] = 1

	rb.PrepareForMappedRun(false, 3)

	if rb.Len() != 0 {
		t.Fatalf("len=%d, want 0", rb.Len())
	}
	if rb.Pos != nil || rb.Masks != nil || rb.UnsafeFlags != nil || rb.Syllables != nil || rb.Joiners != nil {
		t.Fatalf("shaping arrays should be disabled after prepare")
	}
	if rb.Codepoints == nil || rb.Clusters == nil {
		t.Fatalf("mapping arrays should be enabled after prepare")
	}
	if rb.PlanIDs != nil {
		t.Fatalf("plan IDs should be disabled for withPlanIDs=false")
	}
	if cap(rb.Glyphs) < 3 {
		t.Fatalf("glyph capacity=%d, want at least 3", cap(rb.Glyphs))
	}

	rb.PrepareForMappedRun(true, 2)
	if rb.PlanIDs == nil {
		t.Fatalf("plan IDs should be enabled for withPlanIDs=true")
	}
}
