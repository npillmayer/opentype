package otarabic

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"
)

type postRun struct {
	glyphs []ot.GlyphIndex
	cps    []rune
	masks  []uint32
}

func (r *postRun) Len() int { return len(r.glyphs) }
func (r *postRun) Glyph(i int) ot.GlyphIndex {
	return r.glyphs[i]
}
func (r *postRun) SetGlyph(i int, gid ot.GlyphIndex) {
	r.glyphs[i] = gid
}
func (r *postRun) Codepoint(i int) rune {
	return r.cps[i]
}
func (r *postRun) SetCodepoint(i int, cp rune) {
	r.cps[i] = cp
}
func (r *postRun) Cluster(i int) uint32 {
	return uint32(i)
}
func (r *postRun) SetCluster(i int, cluster uint32) {
	_, _ = i, cluster
}
func (r *postRun) MergeClusters(start, end int) {
	_, _ = start, end
}
func (r *postRun) Pos(i int) otlayout.PosItem {
	_ = i
	return otlayout.PosItem{AttachTo: -1}
}
func (r *postRun) SetPos(i int, pos otlayout.PosItem) {
	_, _ = i, pos
}
func (r *postRun) Mask(i int) uint32 {
	return r.masks[i]
}
func (r *postRun) SetMask(i int, mask uint32) {
	r.masks[i] = mask
}
func (r *postRun) InsertGlyphs(index int, glyphs []ot.GlyphIndex) {
	if len(glyphs) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index > len(r.glyphs) {
		index = len(r.glyphs)
	}
	repl := append([]ot.GlyphIndex(nil), glyphs...)
	r.glyphs = append(r.glyphs[:index:index], append(repl, r.glyphs[index:]...)...)
	r.cps = append(r.cps[:index:index], append(make([]rune, len(glyphs)), r.cps[index:]...)...)
	r.masks = append(r.masks[:index:index], append(make([]uint32, len(glyphs)), r.masks[index:]...)...)
}
func (r *postRun) InsertGlyphCopies(index int, source int, count int) {
	if count <= 0 || source < 0 || source >= len(r.glyphs) {
		return
	}
	if index < 0 {
		index = 0
	}
	if index > len(r.glyphs) {
		index = len(r.glyphs)
	}
	gid := r.glyphs[source]
	cp := r.cps[source]
	mask := r.masks[source]
	glyphs := make([]ot.GlyphIndex, count)
	cps := make([]rune, count)
	masks := make([]uint32, count)
	for i := 0; i < count; i++ {
		glyphs[i] = gid
		cps[i] = cp
		masks[i] = mask
	}
	r.glyphs = append(r.glyphs[:index:index], append(glyphs, r.glyphs[index:]...)...)
	r.cps = append(r.cps[:index:index], append(cps, r.cps[index:]...)...)
	r.masks = append(r.masks[:index:index], append(masks, r.masks[index:]...)...)
}
func (r *postRun) Swap(i, j int) {
	r.glyphs[i], r.glyphs[j] = r.glyphs[j], r.glyphs[i]
	r.cps[i], r.cps[j] = r.cps[j], r.cps[i]
	r.masks[i], r.masks[j] = r.masks[j], r.masks[i]
}

func TestFallbackGlyphForMapsExtendedForms(t *testing.T) {
	table := map[rune]glyphForms{
		'\u0628': {formIsol: 11, formFina: 12, formMedi: 13, formInit: 14},
	}
	if gid, ok := fallbackGlyphFor(table, '\u0628', formFin2); !ok || gid != 12 {
		t.Fatalf("fin2 fallback = (%d,%t), want (12,true)", gid, ok)
	}
	if gid, ok := fallbackGlyphFor(table, '\u0628', formMed2); !ok || gid != 13 {
		t.Fatalf("med2 fallback = (%d,%t), want (13,true)", gid, ok)
	}
}

func TestPostprocessRunAppliesFallbackGlyphsForNotdef(t *testing.T) {
	s := &Shaper{
		plan: shaperPlanState{
			hasNotdefFallback: true,
			fallbackGlyph: map[rune]glyphForms{
				'\u0628': {formInit: 41, formMedi: 42, formFina: 43, formIsol: 44},
			},
		},
		preparedForm: []int{formInit, formMedi, formFina},
	}
	run := &postRun{
		glyphs: []ot.GlyphIndex{otshape.NOTDEF, otshape.NOTDEF, otshape.NOTDEF},
		cps:    []rune{'\u0628', '\u0628', '\u0628'},
		masks:  []uint32{0, 0, 0},
	}
	s.PostprocessRun(run)
	want := []ot.GlyphIndex{41, 42, 43}
	for i, w := range want {
		if run.glyphs[i] != w {
			t.Fatalf("glyph[%d]=%d, want %d", i, run.glyphs[i], w)
		}
	}
	if len(s.preparedForm) != 0 {
		t.Fatalf("prepared forms should be cleared after postprocess, got len=%d", len(s.preparedForm))
	}
}

func TestPostprocessRunKeepsResolvedGlyphs(t *testing.T) {
	s := &Shaper{
		plan: shaperPlanState{
			hasNotdefFallback: true,
			fallbackGlyph: map[rune]glyphForms{
				'\u0628': {formInit: 41},
			},
		},
		preparedForm: []int{formInit},
	}
	run := &postRun{
		glyphs: []ot.GlyphIndex{99},
		cps:    []rune{'\u0628'},
		masks:  []uint32{0},
	}
	s.PostprocessRun(run)
	if run.glyphs[0] != 99 {
		t.Fatalf("resolved glyph overwritten by fallback: got %d", run.glyphs[0])
	}
}

func TestPostprocessRunDoesNotRepairWhenFallbackNotRequested(t *testing.T) {
	s := &Shaper{
		plan: shaperPlanState{
			hasNotdefFallback: false,
			fallbackGlyph: map[rune]glyphForms{
				'\u0628': {formInit: 41},
			},
		},
		preparedForm: []int{formInit},
	}
	run := &postRun{
		glyphs: []ot.GlyphIndex{otshape.NOTDEF},
		cps:    []rune{'\u0628'},
		masks:  []uint32{0},
	}
	s.PostprocessRun(run)
	if run.glyphs[0] != otshape.NOTDEF {
		t.Fatalf("glyph should stay .notdef when fallback is not requested, got %d", run.glyphs[0])
	}
}

func TestPostprocessRunKeepsNotdefWhenNoMappingExists(t *testing.T) {
	s := &Shaper{
		plan: shaperPlanState{
			hasNotdefFallback: true,
			fallbackGlyph: map[rune]glyphForms{
				'\u0628': {formInit: 41},
			},
		},
		preparedForm: []int{formFina},
	}
	run := &postRun{
		glyphs: []ot.GlyphIndex{otshape.NOTDEF},
		cps:    []rune{'\u062C'},
		masks:  []uint32{0},
	}
	s.PostprocessRun(run)
	if run.glyphs[0] != otshape.NOTDEF {
		t.Fatalf("glyph should stay .notdef when no fallback mapping exists, got %d", run.glyphs[0])
	}
}

type fallbackPlanProbe struct {
	fallback map[ot.Tag]bool
}

func (p fallbackPlanProbe) Font() *ot.Font                      { return nil }
func (p fallbackPlanProbe) Selection() otshape.SelectionContext { return otshape.SelectionContext{} }
func (p fallbackPlanProbe) FeatureMask1(tag ot.Tag) uint32      { _ = tag; return 0 }
func (p fallbackPlanProbe) FeatureNeedsFallback(tag ot.Tag) bool {
	return p.fallback[tag]
}

func TestPlanNeedsArabicFallback(t *testing.T) {
	cases := []struct {
		name string
		fb   map[ot.Tag]bool
		want bool
	}{
		{name: "none", fb: map[ot.Tag]bool{}, want: false},
		{name: "rlig", fb: map[ot.Tag]bool{tagRlig: true}, want: true},
		{name: "form", fb: map[ot.Tag]bool{tagInit: true}, want: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := planNeedsArabicFallback(fallbackPlanProbe{fallback: tc.fb})
			if got != tc.want {
				t.Fatalf("planNeedsArabicFallback() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestExpandTatweelForStchDuplicatesGlyph(t *testing.T) {
	run := &postRun{
		glyphs: []ot.GlyphIndex{10, 42, 20},
		cps:    []rune{'a', '\u0640', 'b'},
		masks:  []uint32{0, 0x4, 0},
	}
	n := expandTatweelForStch(run, 42, 0x4)
	if n != 1 {
		t.Fatalf("inserted=%d, want 1", n)
	}
	want := []ot.GlyphIndex{10, 42, 42, 20}
	for i, w := range want {
		if run.glyphs[i] != w {
			t.Fatalf("glyph[%d]=%d, want %d", i, run.glyphs[i], w)
		}
	}
	if run.masks[2] != 0x4 {
		t.Fatalf("inserted mask=0x%X, want 0x4", run.masks[2])
	}
}

func TestExpandTatweelForStchHonorsMaskGate(t *testing.T) {
	run := &postRun{
		glyphs: []ot.GlyphIndex{42},
		cps:    []rune{'\u0640'},
		masks:  []uint32{0},
	}
	n := expandTatweelForStch(run, 42, 0x4)
	if n != 0 {
		t.Fatalf("inserted=%d, want 0", n)
	}
	if len(run.glyphs) != 1 {
		t.Fatalf("glyph length=%d, want 1", len(run.glyphs))
	}
}
