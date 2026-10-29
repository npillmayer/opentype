package otshape

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/npillmayer/opentype/ot"
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
