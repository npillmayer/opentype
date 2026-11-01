package otshape

import (
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type eventPlanObservation struct {
	hooks     []string
	lengths   []int
	masks     []uint32
	extraMask uint32
	instances int
}
type eventPlanProbe struct {
	observed       *eventPlanObservation
	extraMask      uint32
	rejectCombined bool
}

func (e *eventPlanProbe) Name() string                            { return "event-plan-probe" }
func (e *eventPlanProbe) Match(SelectionContext) ShaperConfidence { return ShaperConfidenceCertain }
func (e *eventPlanProbe) New() ShapingEngine {
	e.observed.instances++
	return &eventPlanProbe{observed: e.observed, rejectCombined: e.rejectCombined}
}
func (*eventPlanProbe) NormalizationPreference() NormalizationMode       { return NormalizationNone }
func (*eventPlanProbe) ApplyGPOS() bool                                  { return true }
func (*eventPlanProbe) CollectFeatures(FeaturePlanner, SelectionContext) {}
func (*eventPlanProbe) OverrideFeatures(FeaturePlanner)                  {}
func (e *eventPlanProbe) InitPlan(p PlanContext)                         { e.extraMask = p.FeatureMask1(ot.T("zzzz")) }
func (e *eventPlanProbe) ValidatePlan(PlanContext) error {
	if e.rejectCombined && e.extraMask != 0 {
		return errors.New("reject combined plan")
	}
	return nil
}
func (e *eventPlanProbe) note(hook string, run RunContext) {
	e.observed.hooks = append(e.observed.hooks, hook)
	e.observed.lengths = append(e.observed.lengths, run.Len())
}
func (e *eventPlanProbe) PreprocessRun(run RunContext) { e.note("pre", run) }
func (e *eventPlanProbe) PrepareGSUB(run RunContext)   { e.note("prepare", run) }
func (e *eventPlanProbe) SetupMasks(run RunContext) {
	e.note("masks", run)
	e.observed.extraMask = e.extraMask
	for i := 0; i < run.Len(); i++ {
		e.observed.masks = append(e.observed.masks, run.Mask(i)&e.extraMask)
	}
}
func (e *eventPlanProbe) PostprocessRun(run RunContext) { e.note("post", run) }

func TestSHAPE016SharedPlanAndEngineHooks(t *testing.T) {
	for _, reject := range []bool{false, true} {
		font := loadMiniOTFont(t, "gpos3_font1.otf")
		observed := &eventPlanObservation{}
		engine := &eventPlanProbe{observed: observed, rejectCombined: reject}
		src := &sliceEventSource{events: []InputEvent{
			{Kind: InputEventRune, Rune: 0x12, Size: 1},
			{Kind: InputEventPushFeatures, Push: []FeatureSetting{{Tag: ot.T("zzzz"), Enabled: true, Value: 1}}},
			{Kind: InputEventRune, Rune: 0x12, Size: 1},
			{Kind: InputEventPopFeatures},
			{Kind: InputEventRune, Rune: 0x12, Size: 1},
		}}
		sink := &collectSink{}
		err := NewShaper(engine).ShapeEvents(standardParams(font), src, sink,
			BufferOptions{HighWatermark: 4, LowWatermark: 1, MaxBuffer: 5})
		if reject {
			if err == nil || err.Error() != "reject combined plan" || len(sink.glyphs) != 0 || len(observed.hooks) != 0 {
				t.Fatalf("combined validation: err=%v sink=%v hooks=%v", err, sink.glyphs, observed.hooks)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if observed.instances != 2 || engine.extraMask != 0 {
			t.Fatalf("engine isolation: %+v", observed)
		}
		if !reflect.DeepEqual(observed.hooks, []string{"pre", "prepare", "masks", "post"}) || !reflect.DeepEqual(observed.lengths, []int{3, 3, 3, 3}) {
			t.Fatalf("hooks=%v lengths=%v", observed.hooks, observed.lengths)
		}
		if observed.extraMask == 0 || !reflect.DeepEqual(observed.masks, []uint32{0, observed.extraMask, 0}) {
			t.Fatalf("combined plan mask=%x run masks=%v", observed.extraMask, observed.masks)
		}
	}
}

func TestEventRangesRestoreImplicitDefaultsAfterCompaction(t *testing.T) {
	// A resolved, unmasked default must survive a temporary off scope.
	root := &plan{GPOS: tableProgram{Lookups: []lookupOp{{Features: []lookupFeature{{Tag: ot.T("curs")}}}}}}
	base := newFeatureSet(nil)
	scoped := base.applyPush([]FeatureSetting{{Tag: ot.T("curs"), Enabled: false}, {Tag: ot.T("zzzz"), Enabled: true, Value: 2}})
	st := &streamingState{rawRunes: []rune{'a', 'b', 'c'}, rawClusters: []uint32{20, 21, 22}, rawPlanIDs: []uint16{1, 2, 1}}
	features, err := eventFeatureRanges(st, nil, root, map[uint16]featureSet{1: base, 2: scoped})
	if err != nil {
		t.Fatal(err)
	}
	want := []FeatureRange{
		{Feature: ot.T("curs"), On: true, Arg: 1},
		{Feature: ot.T("zzzz")},
		{Feature: ot.T("curs"), Start: 21, End: 22},
		{Feature: ot.T("zzzz"), On: true, Arg: 2, Start: 21, End: 22},
	}
	if !reflect.DeepEqual(features, want) {
		t.Fatalf("ranges=%+v want=%+v", features, want)
	}
}

func TestSHAPE016CombinedCarryKeepsRandomSeed(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	params := standardParams(font)
	params.Features = []FeatureRange{{Feature: ot.T("test"), On: true, Arg: -1}}
	compiler := newPlanCompiler(params, selectionContextFromParams(params), &hookProbeShaper{})
	compiler.randomSeed = 42
	root, err := compiler.compileDefault()
	if err != nil {
		t.Fatal(err)
	}
	base := newFeatureSet(params.Features)
	scopes := map[uint16]featureSet{
		1: base,
		2: base.applyPush([]FeatureSetting{{Tag: ot.T("kern"), Enabled: false}}),
		3: base.applyPush([]FeatureSetting{{Tag: ot.T("test"), Enabled: true, Value: math.MinInt}}),
	}
	ws := newShapeWorkspace(64)
	for _, span := range [][2]int{{0, 32}, {7, 19}, {16, 32}, {7, 19}} {
		st := newStreamingState(streamingConfig{highWatermark: 32, lowWatermark: 1, maxBuffer: 64})
		for i := span[0]; i < span[1]; i++ {
			st.rawRunes = append(st.rawRunes, 18)
			st.rawClusters = append(st.rawClusters, uint32(i))
			st.rawPlanIDs = append(st.rawPlanIDs, uint16(1+i%3))
		}
		st.nextCluster = uint32(span[1])
		run, err := shapeEventCarry(ws, st, compiler, root, scopes)
		if err != nil {
			t.Fatal(err)
		}
		if run.Len() != span[1]-span[0] {
			t.Fatalf("run length %d", run.Len())
		}
		for i, gid := range run.Glyphs {
			source := uint32(span[0] + i)
			want := []ot.GlyphIndex{20, 21, 22}[rand.New(rand.NewPCG(42, uint64(source)<<32)).IntN(3)]
			if gid != want || run.Clusters[i] != source {
				t.Fatalf("source %d: glyph=%d cluster=%d want glyph=%d", source, gid, run.Clusters[i], want)
			}
		}
	}
}
