package otarabic_test

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otarabic"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func TestShaperMatchArabicAndSyriac(t *testing.T) {
	var s = otarabic.Shaper{}

	if got := s.Match(otshape.SelectionContext{
		Script:    language.MustParseScript("Arab"),
		Direction: bidi.RightToLeft,
	}); got <= otshape.ShaperConfidenceNone {
		t.Fatalf("expected Arabic match, got %d", got)
	}

	if got := s.Match(otshape.SelectionContext{
		Script:    language.MustParseScript("Syrc"),
		Direction: bidi.RightToLeft,
	}); got <= otshape.ShaperConfidenceNone {
		t.Fatalf("expected Syriac match, got %d", got)
	}

	if got := s.Match(otshape.SelectionContext{
		Script:    language.MustParseScript("Arab"),
		Direction: bidi.Mixed,
	}); got != otshape.ShaperConfidenceNone {
		t.Fatalf("expected non-match for unsupported direction, got %d", got)
	}
}

func TestShaperHookSurface(t *testing.T) {
	engine := otarabic.New()

	if _, ok := engine.(otshape.ShapingEnginePolicy); !ok {
		t.Fatal("arabic shaper must implement policy hooks")
	}
	if _, ok := engine.(otshape.ShapingEnginePlanHooks); !ok {
		t.Fatal("arabic shaper must implement plan hooks")
	}
	if _, ok := engine.(otshape.ShapingEnginePostResolveHook); !ok {
		t.Fatal("arabic shaper must implement post-resolve hooks")
	}
	if _, ok := engine.(otshape.ShapingEnginePreGSUBHook); !ok {
		t.Fatal("arabic shaper must implement pre-GSUB hooks")
	}
	if _, ok := engine.(otshape.ShapingEngineReorderHook); !ok {
		t.Fatal("arabic shaper must implement reorder hooks")
	}
	if _, ok := engine.(otshape.ShapingEngineMaskHook); !ok {
		t.Fatal("arabic shaper must implement mask hooks")
	}
	if _, ok := engine.(otshape.ShapingEnginePostprocessHook); !ok {
		t.Fatal("arabic shaper must implement postprocess hooks")
	}
}

func TestNewName(t *testing.T) {
	if got := otarabic.New().Name(); got != "arabic" {
		t.Fatalf("New().Name() = %q, want %q", got, "arabic")
	}
}

type plannerProbe struct {
	added     []ot.Tag
	pauses    int
	hasByTag  map[ot.Tag]bool
	disabled  map[ot.Tag]bool
	lastValue map[ot.Tag]uint32
}

func (p *plannerProbe) EnableFeature(tag ot.Tag) {
	p.AddFeature(tag, otshape.FeatureNone, 1)
}

func (p *plannerProbe) AddFeature(tag ot.Tag, _ otshape.FeatureFlags, value uint32) {
	if p.hasByTag == nil {
		p.hasByTag = map[ot.Tag]bool{}
	}
	if p.lastValue == nil {
		p.lastValue = map[ot.Tag]uint32{}
	}
	if !p.hasByTag[tag] {
		p.added = append(p.added, tag)
	}
	p.hasByTag[tag] = true
	p.lastValue[tag] = value
}

func (p *plannerProbe) DisableFeature(tag ot.Tag) {
	if p.disabled == nil {
		p.disabled = map[ot.Tag]bool{}
	}
	p.disabled[tag] = true
}

func (p *plannerProbe) AddGSUBPause(fn otshape.PauseHook) {
	if fn != nil {
		p.pauses++
	}
}

func (p *plannerProbe) HasFeature(tag ot.Tag) bool {
	return p.hasByTag[tag]
}

func TestCollectFeaturesAddsArabicPipelineFeatures(t *testing.T) {
	engine := otarabic.New().(*otarabic.Shaper)
	probe := &plannerProbe{}
	engine.CollectFeatures(probe, otshape.SelectionContext{
		Script:    language.MustParseScript("Arab"),
		Direction: bidi.RightToLeft,
	})

	mustHave := []ot.Tag{
		ot.T("stch"), ot.T("ccmp"), ot.T("locl"),
		ot.T("isol"), ot.T("fina"), ot.T("fin2"), ot.T("fin3"), ot.T("medi"), ot.T("med2"), ot.T("init"),
		ot.T("rlig"), ot.T("calt"), ot.T("rclt"), ot.T("liga"), ot.T("clig"), ot.T("mset"),
	}
	for _, tag := range mustHave {
		if !probe.hasByTag[tag] {
			t.Fatalf("CollectFeatures did not enable %s", tag)
		}
	}
	if probe.pauses == 0 {
		t.Fatalf("CollectFeatures should register pause boundaries")
	}
}

type planCtxProbe struct {
	selection otshape.SelectionContext
	mask1     map[ot.Tag]uint32
	fallback  map[ot.Tag]bool
}

func (p planCtxProbe) Font() *ot.Font { return nil }
func (p planCtxProbe) Selection() otshape.SelectionContext {
	return p.selection
}
func (p planCtxProbe) FeatureMask1(tag ot.Tag) uint32 {
	return p.mask1[tag]
}
func (p planCtxProbe) FeatureNeedsFallback(tag ot.Tag) bool {
	return p.fallback[tag]
}

type runProbe struct {
	codepoints []rune
	masks      []uint32
}

func (r *runProbe) Len() int { return len(r.codepoints) }
func (r *runProbe) Glyph(i int) ot.GlyphIndex {
	_ = i
	return 0
}
func (r *runProbe) SetGlyph(i int, gid ot.GlyphIndex) {
	_, _ = i, gid
}
func (r *runProbe) Codepoint(i int) rune {
	return r.codepoints[i]
}
func (r *runProbe) SetCodepoint(i int, cp rune) {
	r.codepoints[i] = cp
}
func (r *runProbe) Cluster(i int) uint32 {
	return uint32(i)
}
func (r *runProbe) SetCluster(i int, cluster uint32) {
	_, _ = i, cluster
}
func (r *runProbe) MergeClusters(start, end int) {
	_, _ = start, end
}
func (r *runProbe) Pos(i int) otlayout.PosItem {
	_ = i
	return otlayout.PosItem{AttachTo: -1}
}
func (r *runProbe) SetPos(i int, pos otlayout.PosItem) {
	_, _ = i, pos
}
func (r *runProbe) Mask(i int) uint32 {
	return r.masks[i]
}
func (r *runProbe) SetMask(i int, mask uint32) {
	r.masks[i] = mask
}
func (r *runProbe) InsertGlyphs(index int, glyphs []ot.GlyphIndex) {
	_, _ = index, glyphs
}
func (r *runProbe) InsertGlyphCopies(index int, source int, count int) {
	_, _, _ = index, source, count
}
func (r *runProbe) Swap(i, j int) {
	r.codepoints[i], r.codepoints[j] = r.codepoints[j], r.codepoints[i]
	r.masks[i], r.masks[j] = r.masks[j], r.masks[i]
}

func TestPrepareGSUBPrecomputesFormsUsedBySetupMasks(t *testing.T) {
	s := otarabic.New().(*otarabic.Shaper)
	ctx := planCtxProbe{
		selection: otshape.SelectionContext{
			Script: language.MustParseScript("Arab"),
		},
		mask1: map[ot.Tag]uint32{
			ot.T("isol"): 0x0001,
			ot.T("fina"): 0x0002,
			ot.T("fin2"): 0x0004,
			ot.T("fin3"): 0x0008,
			ot.T("medi"): 0x0010,
			ot.T("med2"): 0x0020,
			ot.T("init"): 0x0040,
		},
	}
	s.InitPlan(ctx)
	run := &runProbe{
		codepoints: []rune{'\u0628', '\u0628', '\u0628'}, // beh beh beh
		masks:      []uint32{0x8000, 0x8000, 0x8000},
	}

	s.PrepareGSUB(run)
	s.SetupMasks(run)

	if run.masks[0] != 0x8040 {
		t.Fatalf("mask[0]=0x%X, want init 0x8040", run.masks[0])
	}
	if run.masks[1] != 0x8010 {
		t.Fatalf("mask[1]=0x%X, want medi 0x8010", run.masks[1])
	}
	if run.masks[2] != 0x8002 {
		t.Fatalf("mask[2]=0x%X, want fina 0x8002", run.masks[2])
	}
}
