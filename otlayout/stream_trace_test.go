package otlayout

import (
	"errors"
	"github.com/npillmayer/opentype/ot"
	"reflect"
	"testing"
)

func traceForGlyphs(n int) *StreamTrace {
	trace := NewStreamTrace(n)
	for i := 0; i < n; i++ {
		trace.Sources = append(trace.Sources, SourceSpan{i, i + 1})
	}
	return trace
}

func TestStreamTraceContextAndIncompleteEdge(t *testing.T) {
	font, graph := limitFixture(t, GSubFeatureType)
	cov := func(g ot.GlyphIndex) ot.Coverage { return ot.Coverage{GlyphRange: testGlyphRange{glyph: g}} }
	root := graph.Lookup(0)
	root.Type = ot.GSubLookupTypeChainingContext
	root.Flag = 0
	*root.Subtable(0) = ot.LookupNode{LookupType: root.Type, Format: 3, GSub: &ot.GSubLookupPayload{ChainingContextFmt3: &ot.GSubChainingContextFmt3Payload{
		BacktrackCoverages: []ot.Coverage{cov(9)}, InputCoverages: []ot.Coverage{cov(10)}, LookaheadCoverages: []ot.Coverage{cov(11)}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}},
	}}}
	nested := graph.Lookup(1)
	nested.Type = ot.GSubLookupTypeSingle
	nested.Flag = 0
	*nested.Subtable(0) = ot.LookupNode{LookupType: nested.Type, Format: 1, Coverage: cov(10), GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 2}}}
	for _, tc := range []struct {
		name    string
		glyphs  GlyphBuffer
		applied bool
		unsafe  []bool
		keep    int
	}{
		{"match", GlyphBuffer{9, 10, 11}, true, []bool{false, true, true, false}, 3},
		{"known mismatch", GlyphBuffer{9, 10, 13}, false, []bool{false, false, false, false}, 3},
		{"unfinished lookahead", GlyphBuffer{9, 10}, false, []bool{false, true, false}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := NewBufferState(append(GlyphBuffer(nil), tc.glyphs...), nil)
			st.Index = 1
			st.Stream = traceForGlyphs(st.Len())
			_, applied, err := ApplyFeature(font, feature{tag: ot.T("test"), typ: GSubFeatureType, lookupIndices: []int{0}}, st, 0)
			if err != nil || applied != tc.applied || !reflect.DeepEqual(st.Stream.Unsafe, tc.unsafe) || st.Stream.KeepFrom != tc.keep {
				t.Fatalf("err=%v applied=%v trace=%+v", err, applied, st.Stream)
			}
		})
	}
}

func TestStreamTraceLookbackHonorsIgnoredGlyphsAndNestedFlags(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	graph := font.Layout.GSub.LookupGraph()
	cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
	root := graph.Lookup(0)
	root.Type = ot.GSubLookupTypeContext
	root.Flag = 0
	*root.Subtable(0) = ot.LookupNode{LookupType: root.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}}}}}
	child := graph.Lookup(1)
	child.Type = ot.GSubLookupTypeChainingContext
	child.Flag = ot.LOOKUP_FLAG_IGNORE_MARKS
	*child.Subtable(0) = ot.LookupNode{LookupType: child.Type, Format: 3, GSub: &ot.GSubLookupPayload{ChainingContextFmt3: &ot.GSubChainingContextFmt3Payload{BacktrackCoverages: []ot.Coverage{cov, cov}, InputCoverages: []ot.Coverage{cov}}}}
	// Find a real GDEF mark, without relying on its cmap or glyph name.
	mark := ot.GlyphIndex(0)
	for g := ot.GlyphIndex(1); g < 100; g++ {
		if glyphClass(font.Layout.GDef, g) == ot.MarkGlyph {
			mark = g
			break
		}
	}
	if mark == 0 {
		t.Fatal("fixture has no mark")
	}
	st := NewBufferState(GlyphBuffer{10, 10, mark, mark, mark}, nil)
	st.Stream = traceForGlyphs(st.Len())
	st.RetainLookupContext(graph, 0, font.Layout.GDef)
	if st.Stream.Disabled || st.Stream.KeepFrom != 0 {
		t.Fatalf("trace=%+v", st.Stream)
	}
}

func TestStreamTraceEditsAndCloneOwnership(t *testing.T) {
	st := NewBufferState(GlyphBuffer{1, 2, 3}, nil)
	st.Stream = traceForGlyphs(3)
	clone := st.CloneShared()
	clone.ReplaceGlyphs(1, 2, []ot.GlyphIndex{4, 5})
	if len(clone.Stream.Sources) != 4 || len(st.Stream.Sources) != 3 {
		t.Fatal("shared trace mutation")
	}
	clone.Stream.Join(0, 3)
	if st.Stream.Unsafe[1] {
		t.Fatal("shared unsafe storage")
	}
	clone.ReplaceGlyphs(2, 3, nil)
	if len(clone.Stream.Sources) != 3 || !clone.Stream.Unsafe[2] {
		t.Fatalf("deletion trace=%+v", clone.Stream)
	}
}

func TestStreamGlyphExpansionLimitBeforeMutation(t *testing.T) {
	font, graph := limitFixture(t, GSubFeatureType)
	l := graph.Lookup(0)
	l.Type = ot.GSubLookupTypeMultiple
	l.Flag = 0
	*l.Subtable(0) = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}, GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12}}}}}
	for _, limit := range []int{1, 2} {
		st := NewBufferState(GlyphBuffer{10}, nil)
		st.MaxGlyphs = limit
		st.Stream = traceForGlyphs(1)
		_, _, err := ApplyFeature(font, feature{tag: ot.T("test"), typ: GSubFeatureType, lookupIndices: []int{0}}, st, 0)
		if limit == 1 {
			if !errors.Is(err, ErrGlyphLimit) || st.Len() != 1 || st.Glyphs[0] != 10 || len(st.AppliedEdits()) != 0 {
				t.Fatalf("err=%v state=%+v", err, st)
			}
		} else if err != nil || st.Len() != 2 {
			t.Fatalf("err=%v state=%+v", err, st)
		}
	}
}
