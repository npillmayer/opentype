package otshape

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func TestSHAPE003RandomSelectionSurvivesCarryAndScopes(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	params := standardParams(font)
	params.Features = []FeatureRange{{Feature: ot.T("test"), On: true, Arg: -1}}
	compiler := newPlanCompiler(params, selectionContextFromParams(params), nil)
	compiler.randomSeed = 42
	root, err := compiler.compileDefault()
	if err != nil {
		t.Fatal(err)
	}
	stack := newPlanStack(params.Features, root)
	// A push of an unrelated feature must preserve the root's negative value.
	if _, err := stack.push([]FeatureSetting{{Tag: ot.T("kern"), Enabled: false}}, compiler.compile); err != nil {
		t.Fatal(err)
	}
	plans := []*plan{root, stack.currentPlan()}
	if _, err := stack.push([]FeatureSetting{{Tag: ot.T("test"), Enabled: true, Value: math.MinInt}}, compiler.compile); err != nil {
		t.Fatal(err)
	}
	plans = append(plans, stack.currentPlan())
	seen := map[ot.GlyphIndex]bool{}
	for _, pl := range plans {
		if pl.randomSeed != 42 {
			t.Fatal("event plan lost request seed")
		}
		// Re-shape overlapping pieces of the original input, as carry and event
		// spans do. Selection must use the original anchor, not its local index.
		for _, span := range [][2]int{{0, 32}, {7, 19}, {16, 32}, {7, 19}} {
			runes := make([]rune, span[1]-span[0])
			clusters := make([]uint32, len(runes))
			for i := range runes {
				runes[i] = 18
				clusters[i] = uint32(span[0] + i)
			}
			run := mapRunesToRunBufferInto(newRunBuffer(len(runes)), runes, clusters, nil, font)
			if err := shapeMappedRun(run, nil, pl); err != nil {
				t.Fatal(err)
			}
			for i, g := range run.Glyphs {
				// Independently draw from the TTX alternate list with the documented
				// seed derivation (source anchor in high bits, lookup 0 in low bits).
				oracle := rand.New(rand.NewPCG(42, uint64(clusters[i])<<32))
				want := []ot.GlyphIndex{20, 21, 22}[oracle.IntN(3)]
				if g != want || run.Clusters[i] != clusters[i] {
					t.Fatalf("source %d: got %d cluster %d; want %d", clusters[i], g, run.Clusters[i], want)
				}
				seen[want] = true
			}
		}
	}
	if len(seen) != 3 {
		t.Fatal("fixed seed must exercise all alternatives")
	}
}

func TestSHAPE003MasksPreserveLargeValues(t *testing.T) {
	features := []FeatureRange{{Feature: ot.T("test"), On: true, Arg: math.MaxInt}}
	// More than 255 distinct values must not wrap the mask's value code.
	for value := 1; value <= 300; value++ {
		features = append(features, FeatureRange{Feature: ot.T("test"), On: true, Arg: value, Start: value, End: value + 1})
	}
	layout, err := compileUserFeatureMasks(features)
	if err != nil {
		t.Fatal(err)
	}
	clusters := make([]uint32, 302)
	masks := make([]uint32, len(clusters))
	for i := range clusters {
		clusters[i] = uint32(i)
		masks[i] = layout.GlobalMask
	}
	applyFeatureRangesToMasks(masks, clusters, layout.ByFeature, features)
	spec := layout.ByFeature[ot.T("test")]
	for i, mask := range masks {
		want := i
		if i == 0 || i == 301 {
			want = math.MaxInt
		}
		if got := spec.decode(mask); got != want {
			t.Fatalf("source %d: decoded %d, want %d", i, got, want)
		}
	}
}

func TestSHAPE018RandomSelectionUsesMatchedAnchor(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	params := standardParams(font)
	params.Features = []FeatureRange{{Feature: ot.T("test"), On: true, Arg: -1}}
	compiler := newPlanCompiler(params, selectionContextFromParams(params), nil)
	compiler.randomSeed = 42
	pl, err := compiler.compileDefault()
	if err != nil {
		t.Fatal(err)
	}
	for source := uint32(1); source < 32; source += 2 {
		want := []ot.GlyphIndex{20, 21, 22}[rand.New(rand.NewPCG(42, uint64(source)<<32)).IntN(3)]
		// The same covered source may be preceded by an uncovered glyph or be
		// first in rebuilt carry. Both must use its own anchor for the choice.
		for _, prefix := range []bool{true, false} {
			runes, clusters := []rune{18}, []uint32{source}
			if prefix {
				runes, clusters = []rune{17, 18}, []uint32{source - 1, source}
			}
			run := mapRunesToRunBufferInto(newRunBuffer(2), runes, clusters, nil, font)
			if err := shapeMappedRun(run, nil, pl); err != nil {
				t.Fatal(err)
			}
			last := len(runes) - 1
			if run.Len() != len(runes) || run.Glyphs[last] != want || run.Clusters[last] != source {
				t.Fatalf("source=%d prefix=%t: glyphs=%v clusters=%v want=%d", source, prefix, run.Glyphs, run.Clusters, want)
			}
			if prefix && (run.Glyphs[0] != 17 || run.Clusters[0] != source-1) {
				t.Fatal("uncovered prefix changed")
			}
		}
	}
}

func TestSHAPE018PositioningRespectsStartMask(t *testing.T) {
	font := loadMiniOTFont(t, "gpos_chaining3_boundary_f2.otf")
	lookup := font.Layout.GPos.LookupGraph().Lookup(0)
	lookup.Type, lookup.Flag = ot.MaskGPosLookupType(ot.GPosLookupTypeSingle), 0
	*lookup.Subtable(0) = ot.LookupNode{LookupType: lookup.Type, Format: 1,
		Coverage: ot.Coverage{GlyphRange: partialProgressCoverage{}},
		GPos: &ot.GPosLookupPayload{SingleFmt1: &ot.GPosSingleFmt1Payload{
			ValueFormat: ot.ValueFormatXAdvance, Value: ot.ValueRecord{XAdvance: 25},
		}},
	}
	run := newRunBuffer(3)
	run.Glyphs = otlayout.GlyphBuffer{9, 10, 10}
	run.Clusters, run.Masks = []uint32{0, 1, 2}, []uint32{1, 0, 1}
	run.Pos = otlayout.NewPosBuffer(3)
	st := otlayout.NewBufferState(run.Glyphs, run.Pos)
	exec := &planExecutor{run: run}
	pl := &plan{font: font}
	op := lookupOp{FeatureTag: ot.T("test"), Mask: 1}
	feat := planLookupFeature{tag: op.FeatureTag, typ: otlayout.GPosFeatureType, lookupInx: 0}
	if _, err := exec.applyLookupSpan(pl, op, feat, st, 0, 0, st.Len(), 0); err != nil {
		t.Fatal(err)
	}
	for i, want := range []int32{0, 0, 25} {
		if run.Pos[i].XAdvance != want || run.Clusters[i] != uint32(i) {
			t.Fatalf("glyph %d: pos=%+v cluster=%d, want advance=%d", i, run.Pos[i], run.Clusters[i], want)
		}
	}
}

func TestSHAPE011GroupingPreservesRandomSourceAnchors(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	params := standardParams(font)
	params.Features = []FeatureRange{{Feature: ot.T("test"), On: true, Arg: -1}}
	compiler := newPlanCompiler(params, selectionContextFromParams(params), nil)
	compiler.randomSeed = 42
	pl, err := compiler.compileDefault()
	if err != nil {
		t.Fatal(err)
	}
	run := newRunBuffer(32)
	for source := uint32(0); source < 32; source++ {
		run.AppendMappedGlyph(18, 18, source, 0, false)
		run.Clusters[source] = source / 2 * 2
	}
	if err := shapeMappedRun(run, nil, pl); err != nil {
		t.Fatal(err)
	}
	for source, g := range run.Glyphs {
		want := []ot.GlyphIndex{20, 21, 22}[rand.New(rand.NewPCG(42, uint64(source)<<32)).IntN(3)]
		if g != want || run.SourceAnchors[source] != uint32(source) || run.Clusters[source] != uint32(source/2*2) {
			t.Fatalf("source %d: got %d group=%d source=%d, want %d", source, g, run.Clusters[source], run.SourceAnchors[source], want)
		}
	}
}
