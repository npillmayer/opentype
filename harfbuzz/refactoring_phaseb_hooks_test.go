package harfbuzz

import (
	"testing"

	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
)

type postResolveProbe struct {
	complexShaperDefault
	called    bool
	gotScript language.Script
}

func (p *postResolveProbe) Name() string               { return "post-resolve-probe" }
func (p *postResolveProbe) Match(SelectionContext) int { return 0 }
func (p *postResolveProbe) New() ShapingEngine         { return p }

func (p *postResolveProbe) PostResolveFeatures(_ ResolvedFeaturePlanner, _ ResolvedFeatureView, script language.Script) {
	p.called = true
	p.gotScript = script
}

type preGSUBProbe struct {
	complexShaperDefault
	called    bool
	gotScript language.Script
}

func (p *preGSUBProbe) Name() string               { return "pre-gsub-probe" }
func (p *preGSUBProbe) Match(SelectionContext) int { return 0 }
func (p *preGSUBProbe) New() ShapingEngine         { return p }

func (p *preGSUBProbe) PrepareGSUB(_ *Buffer, _ *Font, script language.Script) {
	p.called = true
	p.gotScript = script
}

type testResolvedPlanner struct{}

func (testResolvedPlanner) AddGSUBPauseBefore(tag tables.Tag, _ GSUBPauseFunc) bool { return tag != 0 }
func (testResolvedPlanner) AddGSUBPauseAfter(tag tables.Tag, _ GSUBPauseFunc) bool  { return tag != 0 }

type testResolvedView struct{}

func (testResolvedView) SelectedFeatures(LayoutTable) []ResolvedFeature { return nil }
func (testResolvedView) HasSelectedFeature(LayoutTable, tables.Tag) bool {
	return false
}
func (testResolvedView) ChosenScript(LayoutTable) tables.Tag { return 0 }
func (testResolvedView) FoundScript(LayoutTable) bool        { return false }

func TestPhaseB_DispatchPostResolveHook(t *testing.T) {
	probe := &postResolveProbe{}
	shaperPostResolveFeatures(probe, testResolvedPlanner{}, testResolvedView{}, language.Arabic)
	if !probe.called {
		t.Fatal("post-resolve hook was not called")
	}
	if probe.gotScript != language.Arabic {
		t.Fatalf("post-resolve script = %v, want %v", probe.gotScript, language.Arabic)
	}
}

func TestPhaseB_DispatchPreGSUBHook(t *testing.T) {
	probe := &preGSUBProbe{}
	shaperPrepareGSUB(probe, nil, nil, language.Arabic)
	if !probe.called {
		t.Fatal("pre-GSUB hook was not called")
	}
	if probe.gotScript != language.Arabic {
		t.Fatalf("pre-GSUB script = %v, want %v", probe.gotScript, language.Arabic)
	}
}
