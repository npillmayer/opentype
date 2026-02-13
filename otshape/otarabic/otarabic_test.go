package otarabic_test

import (
	"testing"

	"github.com/npillmayer/opentype/ot"
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
