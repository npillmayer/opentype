package otlayout

import (
	"github.com/npillmayer/opentype/ot"
	"reflect"
	"testing"
)

type partialProgressLaterCoverage struct{}

func (partialProgressLaterCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == 12 || g == 20 }
func (partialProgressLaterCoverage) ByteSize() int                     { return 0 }

// Stage-5 contract tests: deliberately red until edit ownership is repaired.
func TestPartialProgressExpansionEditsPositionsOnce(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	lookup := font.Layout.GSub.LookupGraph().Lookup(0)
	node := lookup.Subtable(0)
	lookup.Type, lookup.Flag = ot.GSubLookupTypeMultiple, 0
	*node = ot.LookupNode{LookupType: lookup.Type, Format: 1,
		Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}},
		GSub:     &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12, 13}}}}}
	st := NewBufferState(GlyphBuffer{10, 20}, NewPosBuffer(2))
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0, 65535}}, st, 0)
	if err == nil || !applied {
		t.Fatalf("expected partial success and error: %v %v", applied, err)
	}
	if !reflect.DeepEqual(st.Glyphs, GlyphBuffer{11, 12, 13, 20}) {
		t.Fatalf("lost completed expansion: %v", st.Glyphs)
	}
	if len(st.Pos) != len(st.Glyphs) {
		t.Fatalf("position edit applied twice: glyphs=%d positions=%d", len(st.Glyphs), len(st.Pos))
	}
}

func TestPartialProgressNestedEditRemapsOuterInput(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	graph := font.Layout.GSub.LookupGraph()
	cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
	installContext := func(index int, covs []ot.Coverage, records []ot.SequenceLookupRecord) {
		l := graph.Lookup(index)
		n := l.Subtable(0)
		l.Type, l.Flag = ot.GSubLookupTypeContext, 0
		*n = ot.LookupNode{LookupType: l.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: covs, Records: records}}}
	}
	installContext(0, []ot.Coverage{cov, {GlyphRange: testGlyphRange{glyph: 20}}},
		[]ot.SequenceLookupRecord{{LookupListIndex: 1}, {SequenceIndex: 1, LookupListIndex: 3}, {LookupListIndex: 65535}})
	installContext(1, []ot.Coverage{cov}, []ot.SequenceLookupRecord{{LookupListIndex: 2}})
	l := graph.Lookup(2)
	n := l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeMultiple, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12, 13}}}}}
	l = graph.Lookup(3)
	n = l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeSingle, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: partialProgressLaterCoverage{}}, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
	st := NewBufferState(GlyphBuffer{10, 20}, nil)
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || !applied {
		t.Fatalf("expected partial success: %v %v", applied, err)
	}
	if want := (GlyphBuffer{11, 12, 13, 21}); !reflect.DeepEqual(st.Glyphs, want) {
		t.Fatalf("outer input mapping lost nested edit: got %v want %v", st.Glyphs, want)
	}
}
