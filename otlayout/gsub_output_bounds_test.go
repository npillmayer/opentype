package otlayout

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func outputBoundsNode(kind string, output ot.GlyphIndex) *ot.LookupNode {
	n := &ot.LookupNode{Format: 1, Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}, GSub: &ot.GSubLookupPayload{}}
	switch kind {
	case "single":
		n.LookupType, n.Format = ot.GSubLookupTypeSingle, 2
		n.GSub.SingleFmt2 = &ot.GSubSingleFmt2Payload{SubstituteGlyphIDs: []ot.GlyphIndex{output}}
	case "multiple":
		n.LookupType = ot.GSubLookupTypeMultiple
		n.GSub.MultipleFmt1 = &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, output, 12}}}
	case "alternate":
		n.LookupType = ot.GSubLookupTypeAlternate
		n.GSub.AlternateFmt1 = &ot.GSubAlternateFmt1Payload{Alternates: [][]ot.GlyphIndex{{output, 65535}}}
	case "ligature":
		n.LookupType = ot.GSubLookupTypeLigature
		n.GSub.LigatureFmt1 = &ot.GSubLigatureFmt1Payload{LigatureSets: [][]ot.GSubLigatureRule{{{Components: []ot.GlyphIndex{11}, Ligature: output}}}}
	case "reverse":
		n.LookupType = ot.GSubLookupTypeReverseChaining
		n.GSub.ReverseChainingFmt1 = &ot.GSubReverseChainingFmt1Payload{SubstituteGlyphIDs: []ot.GlyphIndex{output}}
	}
	return n
}

func applyOutputBoundsLeaf(ctx *applyCtx, n *ot.LookupNode, alt int) (int, bool, GlyphBuffer, *EditSpan, error) {
	switch n.LookupType {
	case ot.GSubLookupTypeSingle:
		return gsubLookupType1Fmt2(ctx, n, ctx.buf.Glyphs, 1)
	case ot.GSubLookupTypeMultiple:
		return gsubLookupType2Fmt1(ctx, n, ctx.buf.Glyphs, 1)
	case ot.GSubLookupTypeAlternate:
		return gsubLookupType3Fmt1(ctx, n, ctx.buf.Glyphs, 1, alt)
	case ot.GSubLookupTypeLigature:
		return gsubLookupType4Fmt1(ctx, n, ctx.buf.Glyphs, 1)
	default:
		return gsubLookupType8Fmt1(ctx, n, ctx.buf.Glyphs, 1)
	}
}

func TestExplicitOutputGlyphBounds(t *testing.T) {
	for _, kind := range []string{"single", "multiple", "alternate", "ligature", "reverse"} {
		for _, output := range []ot.GlyphIndex{0, 19, 20, 65535} {
			for _, match := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%d/match=%t", kind, output, match), func(t *testing.T) {
					n := outputBoundsNode(kind, output)
					if !match {
						n.Coverage = ot.Coverage{GlyphRange: testGlyphRange{glyph: 99}}
					}
					st := NewBufferState(GlyphBuffer{7, 10, 11, 8}, NewPosBuffer(4))
					st.Index = 1
					st.Pos[3].XAdvance = 123
					st.edits = []EditSpan{{From: 0, To: 1, Len: 1}}
					shared := st.CloneShared()
					before := *st
					before.Glyphs = append(GlyphBuffer(nil), st.Glyphs...)
					before.Pos = append(PosBuffer(nil), st.Pos...)
					before.edits = append([]EditSpan(nil), st.edits...)
					ctx := &applyCtx{env: applyEnv{numGlyphs: 20}, buf: st, subnode: n}
					pos, applied, out, edit, err := applyOutputBoundsLeaf(ctx, n, 0)
					wantErr := match && output >= 20
					if (err != nil) != wantErr {
						t.Fatalf("error=%v wantErr=%t", err, wantErr)
					}
					if !match || wantErr {
						if applied || edit != nil || pos != 1 || !reflect.DeepEqual(*st, before) || !reflect.DeepEqual(out, before.Glyphs) {
							t.Fatalf("failed/nonmatching leaf mutated state: %+v", st)
						}
						if wantErr && !strings.Contains(err.Error(), "glyph count 20") {
							t.Fatal(err)
						}
					} else {
						want := GlyphBuffer{7, output, 11, 8}
						if kind == "multiple" {
							want = GlyphBuffer{7, 11, output, 12, 11, 8}
						}
						if kind == "ligature" {
							want = GlyphBuffer{7, output, 8}
						}
						if !applied || edit == nil || !reflect.DeepEqual(out, want) || len(st.Pos) != len(want) {
							t.Fatalf("unexpected result %v %v %v", out, applied, edit)
						}
					}
					if !reflect.DeepEqual(shared.Glyphs, before.Glyphs) || !reflect.DeepEqual(shared.Pos, before.Pos) {
						t.Fatal("shared buffer changed")
					}
				})
			}
		}
	}
}

func TestOutputBoundsSelection(t *testing.T) {
	for _, tc := range []struct {
		name, kind           string
		alt                  int
		wantErr, wantApplied bool
	}{
		{"unused alternate", "alternate", 0, false, true},
		{"selected last alternate", "alternate", 2, true, false},
		{"unavailable alternate", "alternate", 3, false, false},
		{"unmatched ligature", "ligature", 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := outputBoundsNode(tc.kind, 12)
			if tc.kind == "ligature" {
				n.GSub.LigatureFmt1.LigatureSets[0] = append([]ot.GSubLigatureRule{{Components: []ot.GlyphIndex{99}, Ligature: 65535}}, n.GSub.LigatureFmt1.LigatureSets[0]...)
			}
			st := NewBufferState(GlyphBuffer{7, 10, 11}, nil)
			ctx := &applyCtx{env: applyEnv{numGlyphs: 20}, buf: st, subnode: n}
			_, applied, _, _, err := applyOutputBoundsLeaf(ctx, n, tc.alt)
			if (err != nil) != tc.wantErr || applied != tc.wantApplied {
				t.Fatalf("applied=%v err=%v", applied, err)
			}
		})
	}
}

func TestExplicitOutputBoundsExtension(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs = 20
	l := font.Layout.GSub.LookupGraph().Lookup(0)
	n := l.Subtable(0)
	l.Type, l.Flag = ot.GSubLookupTypeExtensionSubs, 0
	*n = ot.LookupNode{LookupType: l.Type, Format: 1, GSub: &ot.GSubLookupPayload{ExtensionFmt1: &ot.GSubExtensionFmt1Payload{Resolved: outputBoundsNode("multiple", 20)}}}
	st := NewBufferState(GlyphBuffer{10}, nil)
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || applied || len(st.AppliedEdits()) != 0 || !reflect.DeepEqual(st.Glyphs, GlyphBuffer{10}) {
		t.Fatalf("%v %v %+v", applied, err, st)
	}
}

func TestExplicitOutputStopsSubtableFallback(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs = 20
	raw := firstLookupBytes(t, font, "GSUB")
	words := []uint16{1, 0, 2, 10, 24, 2, 8, 1, 20, 1, 1, 10, 2, 8, 1, 11, 1, 1, 10}
	if len(raw) < len(words)*2 {
		t.Fatal("fixture too small")
	}
	for i, w := range words {
		binary.BigEndian.PutUint16(raw[2*i:], w)
	}
	st := NewBufferState(GlyphBuffer{10}, nil)
	pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || applied || pos != 0 || st.Glyphs[0] != 10 || len(st.AppliedEdits()) != 0 || !strings.Contains(err.Error(), "subtable 0") {
		t.Fatalf("%d %v %v %+v", pos, applied, err, st)
	}
}

func TestExplicitOutputNestedPartialProgress(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs = 20
	graph := font.Layout.GSub.LookupGraph()
	parent := graph.Lookup(0)
	node := parent.Subtable(0)
	parent.Type, parent.Flag = ot.GSubLookupTypeContext, 0
	*node = ot.LookupNode{LookupType: parent.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{{GlyphRange: testGlyphRange{glyph: 10}}}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}, {LookupListIndex: 2}}}}}
	for i, output := range []ot.GlyphIndex{11, 20} {
		l := graph.Lookup(i + 1)
		n := l.Subtable(0)
		replacement := outputBoundsNode("single", output)
		replacement.Coverage = ot.Coverage{GlyphRange: testGlyphRange{glyph: ot.GlyphIndex(10 + i)}}
		l.Type, l.Flag = replacement.LookupType, 0
		*n = *replacement
	}
	st := NewBufferState(GlyphBuffer{10}, NewPosBuffer(1))
	pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || !applied || pos != st.Index || !strings.Contains(err.Error(), "sequence record 1") || !strings.Contains(err.Error(), "glyph count 20") {
		t.Fatalf("%d %v %v", pos, applied, err)
	}
	if !reflect.DeepEqual(st.Glyphs, GlyphBuffer{11}) || len(st.Pos) != 1 || !reflect.DeepEqual(st.AppliedEdits(), []EditSpan{{From: 0, To: 1, Len: 1}}) {
		t.Fatalf("lost completed progress: %+v", st)
	}
}
