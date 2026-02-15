package otshape

import (
	"errors"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/otquery"
)

type hookProbeSink struct {
	glyphs []GlyphRecord
}

func (s *hookProbeSink) WriteGlyph(g GlyphRecord) error {
	s.glyphs = append(s.glyphs, g)
	return nil
}

type hookProbeShaper struct {
	useCompose bool
	useReorder bool

	composeCalls int
	reorderCalls int
}

func (s *hookProbeShaper) Name() string { return "hook-probe" }

func (s *hookProbeShaper) Match(SelectionContext) ShaperConfidence {
	return ShaperConfidenceCertain
}

func (s *hookProbeShaper) New() ShapingEngine { return s }

func (s *hookProbeShaper) NormalizationPreference() NormalizationMode {
	return NormalizationComposed
}

func (s *hookProbeShaper) ApplyGPOS() bool {
	return true
}

func (s *hookProbeShaper) Compose(_ NormalizeContext, a, b rune) (rune, bool) {
	s.composeCalls++
	if !s.useCompose {
		return 0, false
	}
	if a == 0x12 && b == 0x13 {
		return 0x12, true
	}
	return 0, false
}

func (s *hookProbeShaper) ReorderMarks(run RunContext, start, end int) {
	s.reorderCalls++
	if !s.useReorder {
		return
	}
	if end-start >= 2 {
		run.Swap(start, start+1)
		run.MergeClusters(start, start+2)
	}
}

func TestShapeComposeHookCanCollapseRunePair(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	source := strings.NewReader(string([]rune{0x12, 0x13}))
	sink := &hookProbeSink{}
	engine := &hookProbeShaper{useCompose: true}
	shaper := NewShaper([]ShapingEngine{engine}...)
	bufOpts := BufferOptions{FlushBoundary: FlushOnRunBoundary}

	err := shaper.Shape(params, source, sink, bufOpts)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if engine.composeCalls == 0 {
		t.Fatalf("compose hook was not called")
	}
	if len(sink.glyphs) != 1 {
		t.Fatalf("glyph count = %d, want 1", len(sink.glyphs))
	}
	want := otquery.GlyphIndex(font, 0x12)
	if sink.glyphs[0].GID != want {
		t.Fatalf("composed glyph = %d, want %d", sink.glyphs[0].GID, want)
	}
}

func TestShapeReorderHookCanSwapRunItems(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	source := strings.NewReader(string([]rune{0x12, 0x13}))
	sink := &hookProbeSink{}
	engine := &hookProbeShaper{useReorder: true}
	shaper := NewShaper([]ShapingEngine{engine}...)
	bufOpts := BufferOptions{FlushBoundary: FlushOnRunBoundary}

	err := shaper.Shape(params, source, sink, bufOpts)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if engine.reorderCalls == 0 {
		t.Fatalf("reorder hook was not called")
	}
	if len(sink.glyphs) != 2 {
		t.Fatalf("glyph count = %d, want 2", len(sink.glyphs))
	}
	want0 := otquery.GlyphIndex(font, 0x13)
	want1 := otquery.GlyphIndex(font, 0x12)
	if sink.glyphs[0].GID != want0 || sink.glyphs[1].GID != want1 {
		t.Fatalf("reordered glyphs = [%d %d], want [%d %d]",
			sink.glyphs[0].GID, sink.glyphs[1].GID, want0, want1)
	}
	if sink.glyphs[0].Cluster != sink.glyphs[1].Cluster {
		t.Fatalf("cluster merge not applied: clusters = [%d %d]",
			sink.glyphs[0].Cluster, sink.glyphs[1].Cluster)
	}
}

type planValidateProbeShaper struct {
	validateErr   error
	initCalls     int
	validateCalls int
}

func (s *planValidateProbeShaper) Name() string { return "plan-validate-probe" }

func (s *planValidateProbeShaper) Match(SelectionContext) ShaperConfidence {
	return ShaperConfidenceCertain
}

func (s *planValidateProbeShaper) New() ShapingEngine { return s }

func (s *planValidateProbeShaper) NormalizationPreference() NormalizationMode {
	return NormalizationComposed
}

func (s *planValidateProbeShaper) ApplyGPOS() bool {
	return true
}

func (s *planValidateProbeShaper) CollectFeatures(FeaturePlanner, SelectionContext) {}

func (s *planValidateProbeShaper) OverrideFeatures(FeaturePlanner) {}

func (s *planValidateProbeShaper) InitPlan(PlanContext) {
	s.initCalls++
}

func (s *planValidateProbeShaper) ValidatePlan(PlanContext) error {
	s.validateCalls++
	return s.validateErr
}

var _ ShapingEngine = (*planValidateProbeShaper)(nil)

func TestShapePlanValidateHookErrorStopsPipeline(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	engine := &planValidateProbeShaper{
		validateErr: errors.New("plan validation failed"),
	}
	source := strings.NewReader(string([]rune{0x12}))
	sink := &hookProbeSink{}
	shaper := NewShaper([]ShapingEngine{engine}...)

	err := shaper.Shape(params, source, sink, singleBufOpts)
	if err == nil || err.Error() != "plan validation failed" {
		t.Fatalf("shape error = %v, want plan validation failure", err)
	}
	if engine.initCalls != 1 || engine.validateCalls != 1 {
		t.Fatalf("unexpected plan hook counts init=%d validate=%d, want 1/1",
			engine.initCalls, engine.validateCalls)
	}
	if len(sink.glyphs) != 0 {
		t.Fatalf("pipeline should stop before output write, got %d glyphs", len(sink.glyphs))
	}
}

func TestShapePlanValidateHookSuccessContinues(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	engine := &planValidateProbeShaper{}
	source := strings.NewReader(string([]rune{0x12}))
	sink := &hookProbeSink{}
	shaper := NewShaper([]ShapingEngine{engine}...)

	err := shaper.Shape(params, source, sink, singleBufOpts)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if engine.initCalls != 1 || engine.validateCalls != 1 {
		t.Fatalf("unexpected plan hook counts init=%d validate=%d, want 1/1",
			engine.initCalls, engine.validateCalls)
	}
	if len(sink.glyphs) != 1 {
		t.Fatalf("glyph count = %d, want 1", len(sink.glyphs))
	}
}

func TestShapeOutputIncludesNominalAdvance(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	params := standardParams(font)
	source := strings.NewReader(string([]rune{0x12}))
	sink := &hookProbeSink{}
	engine := &hookProbeShaper{}
	shaper := NewShaper([]ShapingEngine{engine}...)

	err := shaper.Shape(params, source, sink, singleBufOpts)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	if len(sink.glyphs) != 1 {
		t.Fatalf("glyph count = %d, want 1", len(sink.glyphs))
	}
	gid := otquery.GlyphIndex(font, 0x12)
	wantAdv := int32(otquery.GlyphMetrics(font, gid).Advance)
	if sink.glyphs[0].Pos.XAdvance != wantAdv {
		t.Fatalf("xAdvance = %d, want %d", sink.glyphs[0].Pos.XAdvance, wantAdv)
	}
}
