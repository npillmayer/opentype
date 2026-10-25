package otshape

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

type successCoverage ot.GlyphIndex

func (c successCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == ot.GlyphIndex(c) }
func (successCoverage) ByteSize() int                       { return 0 }

func installSuccessLookup(graph *ot.LookupListGraph, index int, typ ot.LayoutTableLookupType, glyph ot.GlyphIndex, payload *ot.GSubLookupPayload) {
	l := graph.Lookup(index)
	node := l.Subtable(0)
	l.Type, l.Flag = typ, 0
	*node = ot.LookupNode{LookupType: typ, Format: 1, Coverage: ot.Coverage{GlyphRange: successCoverage(glyph)}, GSub: payload}
}

// Feature ranges are assigned to original input slots. Later lookups must follow
// their inherited masks after expansion, not reinterpret ranges on shifted slots.
func TestSuccessfulEditsPreserveFeatureRanges(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		t.Run(fmt.Sprintf("per-syllable=%t", isolated), func(t *testing.T) {
			font := loadMiniOTFont(t, "gsub_context1_lookupflag_f1.otf")
			graph := font.Layout.GSub.LookupGraph()
			installSuccessLookup(graph, 0, ot.GSubLookupTypeMultiple, 10, &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12}}}})
			installSuccessLookup(graph, 1, ot.GSubLookupTypeSingle, 20, &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}})
			installSuccessLookup(graph, 2, ot.GSubLookupTypeLigature, 11, &ot.GSubLookupPayload{LigatureFmt1: &ot.GSubLigatureFmt1Payload{LigatureSets: [][]ot.GSubLigatureRule{{{Components: []ot.GlyphIndex{12}, Ligature: 13}}}}})
			a, b := ot.T("salt"), ot.T("ss01")
			pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{a: {Mask: 1}, b: {Mask: 2, Shift: 1}}},
				featureRanges: []FeatureRange{{Feature: a, On: true, Start: 0, End: 1}, {Feature: b, On: true, Start: 1, End: 2}}}
			run := newRunBuffer(0)
			run.Glyphs = otlayout.GlyphBuffer{10, 20, 10}
			run.Pos = otlayout.NewPosBuffer(3)
			run.Pos[1].XAdvance = 17
			run.Pos[2].XAdvance = 29
			run.Clusters = []uint32{0, 1, 2}
			run.PlanIDs = []uint16{4, 5, 6}
			run.Syllables = []uint16{1, 2, 3}
			run.Joiners = []uint8{0, joinerClassZWJ, joinerClassZWNJ}
			run.UnsafeFlags = []uint16{1, 2, 4}
			run.Codepoints = []rune{'a', 'b', 'c'}
			e := &planExecutor{}
			e.acquireBuffer(run)
			defer e.releaseBuffer()
			e.ensureRunMasks(pl)
			flags := lookupRunFlags(0)
			if isolated {
				flags = lookupPerSyllable
			}
			stages := []struct {
				index                    uint16
				tag                      ot.Tag
				mask                     uint32
				glyphs                   otlayout.GlyphBuffer
				masks, clusters          []uint32
				plans, syllables, unsafe []uint16
				joiners                  []uint8
				codepoints               []rune
			}{
				{0, a, 1, otlayout.GlyphBuffer{11, 12, 20, 10}, []uint32{1, 1, 2, 0}, []uint32{0, 0, 1, 2}, []uint16{4, 4, 5, 6}, []uint16{1, 1, 2, 3}, []uint16{1, 1, 2, 4}, []uint8{0, 0, joinerClassZWJ, joinerClassZWNJ}, []rune{'a', 'a', 'b', 'c'}},
				{1, b, 2, otlayout.GlyphBuffer{11, 12, 21, 10}, []uint32{1, 1, 2, 0}, []uint32{0, 0, 1, 2}, []uint16{4, 4, 5, 6}, []uint16{1, 1, 2, 3}, []uint16{1, 1, 2, 4}, []uint8{0, 0, joinerClassZWJ, joinerClassZWNJ}, []rune{'a', 'a', 'b', 'c'}},
				{2, a, 1, otlayout.GlyphBuffer{13, 21, 10}, []uint32{1, 2, 0}, []uint32{0, 1, 2}, []uint16{4, 5, 6}, []uint16{1, 2, 3}, []uint16{1, 2, 4}, []uint8{0, joinerClassZWJ, joinerClassZWNJ}, []rune{'a', 'b', 'c'}},
			}
			for _, s := range stages {
				if err := e.applyLookups(pl, planGSUB, []lookupOp{{LookupIndex: s.index, FeatureTag: s.tag, Mask: s.mask, Flags: flags}}); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(run.Glyphs, s.glyphs) || !reflect.DeepEqual(run.Masks, s.masks) || !reflect.DeepEqual(run.Clusters, s.clusters) || !reflect.DeepEqual(run.PlanIDs, s.plans) || !reflect.DeepEqual(run.Syllables, s.syllables) || !reflect.DeepEqual(run.UnsafeFlags, s.unsafe) || !reflect.DeepEqual(run.Joiners, s.joiners) || !reflect.DeepEqual(run.Codepoints, s.codepoints) {
					t.Fatalf("lookup %d lost correspondence: %+v", s.index, run)
				}
				n := run.Len()
				if len(run.Pos) != n || run.Pos[n-2].XAdvance != 17 || run.Pos[n-1].XAdvance != 29 {
					t.Fatalf("lookup %d lost suffix positions: %v", s.index, run.Pos)
				}
			}
		})
	}
}

func TestSuccessfulContractionRespectsSyllableBoundary(t *testing.T) {
	for _, split := range []bool{false, true} {
		for _, isolated := range []bool{false, true} {
			t.Run(fmt.Sprintf("split=%t/isolated=%t", split, isolated), func(t *testing.T) {
				font := loadMiniOTFont(t, "gsub_context1_lookupflag_f1.otf")
				installSuccessLookup(font.Layout.GSub.LookupGraph(), 0, ot.GSubLookupTypeLigature, 10, &ot.GSubLookupPayload{LigatureFmt1: &ot.GSubLigatureFmt1Payload{LigatureSets: [][]ot.GSubLigatureRule{{{Components: []ot.GlyphIndex{11}, Ligature: 12}}}}})
				run := newRunBuffer(0)
				run.Glyphs = otlayout.GlyphBuffer{10, 11, 20}
				run.Clusters = []uint32{0, 1, 2}
				run.Syllables = []uint16{1, 1, 2}
				if split {
					run.Syllables[1] = 2
				}
				pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
				e := &planExecutor{}
				e.acquireBuffer(run)
				defer e.releaseBuffer()
				flags := lookupRunFlags(0)
				if isolated {
					flags = lookupPerSyllable
				}
				if err := e.applyLookups(pl, planGSUB, []lookupOp{{LookupIndex: 0, FeatureTag: ot.T("test"), Flags: flags}}); err != nil {
					t.Fatal(err)
				}
				want := otlayout.GlyphBuffer{12, 20}
				if split && isolated {
					want = otlayout.GlyphBuffer{10, 11, 20}
				}
				if !reflect.DeepEqual(run.Glyphs, want) {
					t.Fatalf("got %v want %v", run.Glyphs, want)
				}
				if len(run.Clusters) != len(want) || len(run.Syllables) != len(want) {
					t.Fatal("annotation lengths changed incorrectly")
				}
			})
		}
	}
}

func TestSuccessfulShapeFeatureRangeExpansion(t *testing.T) {
	font := loadMiniOTFont(t, "gsub3_1_simple_f1.otf")
	installSuccessLookup(font.Layout.GSub.LookupGraph(), 0, ot.GSubLookupTypeMultiple, 18, &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{19, 20}}}})
	params := standardParams(font)
	params.Features = []FeatureRange{{Feature: ot.T("test"), On: false}, {Feature: ot.T("test"), On: true, Start: 1, End: 2}}
	sink := &collectSink{}
	shaper := NewShaper(&hookProbeShaper{})
	err := shaper.Shape(params, strings.NewReader(string([]rune{18, 18, 18})), sink, BufferOptions{FlushBoundary: FlushOnRunBoundary, HighWatermark: 64, LowWatermark: 32, MaxBuffer: 64})
	if err != nil {
		t.Fatal(err)
	}
	want := []ot.GlyphIndex{18, 19, 20, 18}
	clusters := []uint32{0, 1, 1, 2}
	if len(sink.glyphs) != len(want) {
		t.Fatalf("output=%v", sink.glyphs)
	}
	for i, g := range sink.glyphs {
		if g.GID != want[i] || g.Cluster != clusters[i] {
			t.Fatalf("glyph %d = %+v; want %d cluster %d", i, g, want[i], clusters[i])
		}
	}
}
