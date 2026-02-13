package othebrew_test

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/othebrew"
	"golang.org/x/text/language"
)

func TestShaperMatchHebrew(t *testing.T) {
	var s = othebrew.Shaper{}

	if got := s.Match(otshape.SelectionContext{Script: language.MustParseScript("Hebr")}); got <= otshape.ShaperConfidenceNone {
		t.Fatalf("expected Hebrew match, got %d", got)
	}
	if got := s.Match(otshape.SelectionContext{Script: language.MustParseScript("Arab")}); got != otshape.ShaperConfidenceNone {
		t.Fatalf("expected Arabic non-match, got %d", got)
	}
}

func TestShaperHookSurface(t *testing.T) {
	engine := othebrew.New()

	if _, ok := engine.(otshape.ShapingEnginePolicy); !ok {
		t.Fatal("hebrew shaper must implement policy hooks")
	}
	if _, ok := engine.(otshape.ShapingEngineComposeHook); !ok {
		t.Fatal("hebrew shaper must implement compose hook")
	}
	if _, ok := engine.(otshape.ShapingEngineReorderHook); !ok {
		t.Fatal("hebrew shaper must implement reorder hook")
	}
}

func TestNewName(t *testing.T) {
	if got := othebrew.New().Name(); got != "hebrew" {
		t.Fatalf("New().Name() = %q, want %q", got, "hebrew")
	}
}

type normalizeProbe struct {
	hasGposMark bool
	composed    rune
	ok          bool
}

func (p normalizeProbe) Font() *ot.Font { return nil }
func (p normalizeProbe) Selection() otshape.SelectionContext {
	return otshape.SelectionContext{}
}
func (p normalizeProbe) ComposeUnicode(a, b rune) (rune, bool) {
	if p.ok {
		return p.composed, true
	}
	return 0, false
}
func (p normalizeProbe) HasGposMark() bool { return p.hasGposMark }

func TestComposeUsesUnicodeWhenAvailable(t *testing.T) {
	s := othebrew.Shaper{}
	c := normalizeProbe{composed: 'X', ok: true}
	if got, ok := s.Compose(c, 'a', 'b'); !ok || got != 'X' {
		t.Fatalf("compose unicode = (%U,%t), want (%U,true)", got, ok, 'X')
	}
}

func TestComposeHebrewPresentationFallback(t *testing.T) {
	s := othebrew.Shaper{}
	c := normalizeProbe{}
	got, ok := s.Compose(c, 0x05D9, 0x05B4) // YOD + HIRIQ
	if !ok || got != 0xFB1D {
		t.Fatalf("compose fallback = (%U,%t), want (%U,true)", got, ok, rune(0xFB1D))
	}
}

func TestComposeFallbackDisabledWhenGposMarkPresent(t *testing.T) {
	s := othebrew.Shaper{}
	c := normalizeProbe{hasGposMark: true}
	if got, ok := s.Compose(c, 0x05D9, 0x05B4); ok || got != 0 {
		t.Fatalf("compose fallback with GPOS mark = (%U,%t), want (0,false)", got, ok)
	}
}

type runProbe struct {
	codepoints []rune
	clusters   []uint32
}

func (p *runProbe) Len() int { return len(p.codepoints) }
func (p *runProbe) Glyph(i int) ot.GlyphIndex {
	_ = i
	return 0
}
func (p *runProbe) SetGlyph(i int, gid ot.GlyphIndex) {
	_, _ = i, gid
}
func (p *runProbe) Codepoint(i int) rune { return p.codepoints[i] }
func (p *runProbe) Cluster(i int) uint32 { return p.clusters[i] }
func (p *runProbe) MergeClusters(start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(p.clusters) {
		end = len(p.clusters)
	}
	if start >= end {
		return
	}
	min := p.clusters[start]
	for i := start + 1; i < end; i++ {
		if p.clusters[i] < min {
			min = p.clusters[i]
		}
	}
	for i := start; i < end; i++ {
		p.clusters[i] = min
	}
}
func (p *runProbe) Mask(i int) uint32 {
	_ = i
	return 0
}
func (p *runProbe) SetMask(i int, mask uint32) {
	_, _ = i, mask
}
func (p *runProbe) Swap(i, j int) {
	p.codepoints[i], p.codepoints[j] = p.codepoints[j], p.codepoints[i]
	p.clusters[i], p.clusters[j] = p.clusters[j], p.clusters[i]
}

func TestReorderMarksSwapsMetegAfterPattern(t *testing.T) {
	s := othebrew.Shaper{}
	run := &runProbe{
		codepoints: []rune{0x05B7, 0x05B0, 0x05BD}, // PATAH, SHEVA, METEG
		clusters:   []uint32{0, 1, 2},
	}
	s.ReorderMarks(run, 0, run.Len())
	if run.codepoints[1] != 0x05BD || run.codepoints[2] != 0x05B0 {
		t.Fatalf("reordered codepoints = [%U,%U,%U], want [U+05B7,U+05BD,U+05B0]",
			run.codepoints[0], run.codepoints[1], run.codepoints[2])
	}
	if run.clusters[1] != run.clusters[2] {
		t.Fatalf("expected merged clusters at reordered pair, got [%d,%d]", run.clusters[1], run.clusters[2])
	}
}

func TestReorderMarksNoopWithoutPattern(t *testing.T) {
	s := othebrew.Shaper{}
	run := &runProbe{
		codepoints: []rune{0x05B0, 0x05B7, 0x05BD}, // SHEVA, PATAH, METEG
		clusters:   []uint32{0, 1, 2},
	}
	s.ReorderMarks(run, 0, run.Len())
	if run.codepoints[0] != 0x05B0 || run.codepoints[1] != 0x05B7 || run.codepoints[2] != 0x05BD {
		t.Fatalf("unexpected reorder for non-matching pattern: [%U,%U,%U]",
			run.codepoints[0], run.codepoints[1], run.codepoints[2])
	}
}
