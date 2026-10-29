package otshape

import (
	"slices"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestInitialCombiningClusters(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []uint32
	}{
		{"x\u0301y", []uint32{10, 10, 12}},
		{"\u05e9\u05b8\u05c1\u05dc", []uint32{10, 10, 10, 13}},
		{"\u0301\u0323x\u0301", []uint32{10, 10, 12, 12}},
		{"x\u200dy\ufe0f", []uint32{10, 10, 12, 12}},
		{"x\u200cy", []uint32{10, 11, 12}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			runes := []rune(tc.input)
			sources := make([]uint32, len(runes))
			for i := range sources {
				sources[i] = uint32(10 + i)
			}
			if got := formInputClusters(nil, runes, sources); !slices.Equal(got, tc.want) {
				t.Fatalf("groups %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClusterNormalizationPreservesSources(t *testing.T) {
	for _, tc := range []struct {
		input           string
		mode            NormalizationMode
		wantRunes       string
		groups, sources []uint32
	}{
		{"a\u0323\u0301x", NormalizationComposed, "\u1ea1\u0301x", []uint32{10, 10, 13}, []uint32{10, 12, 13}},
		{"\u00e9\u0323x", NormalizationDecomposed, "e\u0301\u0323x", []uint32{10, 10, 10, 12}, []uint32{10, 10, 11, 12}},
		{"x\u0301y", NormalizationNone, "x\u0301y", []uint32{10, 10, 12}, []uint32{10, 11, 12}},
	} {
		t.Run(tc.wantRunes, func(t *testing.T) {
			ws := newShapeWorkspace(16)
			runes := []rune(tc.input)
			sources := make([]uint32, len(runes))
			for i := range sources {
				sources[i] = uint32(10 + i)
			}
			r, c, s := ws.normalize(runes, sources, nil, SelectionContext{}, clusterPolicyEngine{mode: tc.mode}, nil)
			if string(r) != tc.wantRunes || !slices.Equal(c, tc.groups) || !slices.Equal(s, tc.sources) {
				t.Fatalf("got %q groups=%v sources=%v; want %q groups=%v sources=%v", string(r), c, s, tc.wantRunes, tc.groups, tc.sources)
			}
		})
	}
}

type clusterPolicyEngine struct{ mode NormalizationMode }

func (e clusterPolicyEngine) Name() string                               { return "cluster-policy-test" }
func (e clusterPolicyEngine) Match(SelectionContext) ShaperConfidence    { return ShaperConfidenceCertain }
func (e clusterPolicyEngine) New() ShapingEngine                         { return e }
func (e clusterPolicyEngine) NormalizationPreference() NormalizationMode { return e.mode }
func (e clusterPolicyEngine) ApplyGPOS() bool                            { return false }

func TestNormalizationScratchDoesNotAliasEventInput(t *testing.T) {
	ws := newShapeWorkspace(16)
	runes := []rune("abc")
	sources := []uint32{10, 11, 12}
	engine := clusterPolicyEngine{NormalizationComposed}
	ws.normalize(runes[:1], sources[:1], nil, SelectionContext{}, engine, nil)
	r, c, s := ws.normalize(runes[1:], sources[1:], nil, SelectionContext{}, engine, nil)
	if string(runes) != "abc" || !slices.Equal(sources, []uint32{10, 11, 12}) {
		t.Fatalf("raw input overwritten: %q %v", string(runes), sources)
	}
	if string(r) != "bc" || !slices.Equal(c, []uint32{11, 12}) || !slices.Equal(s, []uint32{11, 12}) {
		t.Fatalf("second span: %q groups=%v sources=%v", string(r), c, s)
	}
}

func TestClusterRangeUsesGroupStart(t *testing.T) {
	for _, tc := range []struct {
		start, end int
		want       []uint32
	}{
		{1, 3, []uint32{1, 1, 3}}, // Start inside the group excludes its base and mark.
		{0, 1, []uint32{3, 3, 1}}, // End inside the group includes its base and mark.
		{1, 2, []uint32{1, 1, 1}}, // A range containing only the mark selects nothing.
	} {
		run := newRunBuffer(3)
		for i, cp := range []rune("x\u0301y") {
			run.AppendMappedGlyph(ot.GlyphIndex(i), cp, uint32(i), 0, false)
		}
		run.Clusters = formInputClusters(nil, run.Codepoints, run.SourceAnchors)
		pl := &plan{Masks: maskLayout{GlobalMask: 1, ByFeature: map[ot.Tag]maskSpec{ot.T("test"): {Mask: 2, Shift: 1}}}, featureRanges: []FeatureRange{{Feature: ot.T("test"), On: true, Start: tc.start, End: tc.end}}}
		(&planExecutor{run: run}).ensureRunMasks(pl)
		if !slices.Equal(run.Masks, tc.want) {
			t.Errorf("[%d,%d): masks=%v, want %v", tc.start, tc.end, run.Masks, tc.want)
		}
	}
}

func TestClusterMetadataSurvivesEdits(t *testing.T) {
	run := newRunBuffer(4)
	for i, cp := range []rune("fi\u0301x") {
		run.AppendMappedGlyph(ot.GlyphIndex(10+i), cp, uint32(10+i), 0, false)
	}
	run.Clusters = formInputClusters(nil, run.Codepoints, run.SourceAnchors)
	// A ligature consumes f and i, leaving the acute as a separate glyph.
	run.Glyphs = otlayout.GlyphBuffer{20, 12, 13}
	run.applyLayoutEdit(otlayout.EditSpan{From: 0, To: 2, Len: 1})
	if !slices.Equal(run.Clusters, []uint32{10, 10, 13}) || !slices.Equal(run.SourceAnchors, []uint32{10, 12, 13}) {
		t.Fatalf("ligature lost association: groups=%v sources=%v", run.Clusters, run.SourceAnchors)
	}
	run.InsertGlyphCopies(2, 1, 1)
	rc := runContext{run: run}
	rc.Swap(1, 3)
	rc.MergeClusters(1, 3) // Include the adjacent member at index 0 as well.
	if !slices.Equal(run.Clusters, []uint32{10, 10, 10, 10}) || !slices.Equal(run.SourceAnchors, []uint32{10, 13, 12, 12}) {
		t.Fatalf("reorder/copy lost association: groups=%v sources=%v", run.Clusters, run.SourceAnchors)
	}
	dst := newRunBuffer(0)
	dst.AppendRun(run)
	if !slices.Equal(dst.SourceAnchors, run.SourceAnchors) {
		t.Fatal("event append lost source anchors")
	}
	dst.PrepareForMappedRun(false, 2)
	dst.AppendMappedGlyph(30, 'y', 20, 0, false)
	if !slices.Equal(dst.SourceAnchors, []uint32{20}) {
		t.Fatal("reused buffer retained stale source anchors")
	}
}
