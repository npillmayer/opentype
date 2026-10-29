package otlayout

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

// Both direct and extension dispatch must leave iteration to the caller.
func TestForwardLookupStartsAtCursor(t *testing.T) {
	for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
		for _, extension := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/extension=%t", kind, extension), func(t *testing.T) {
				font, graph := limitFixture(t, kind)
				limitLeaf(graph, kind, 0) // Covers GID 10, not the preceding GID 9.
				lookup := graph.Lookup(0)
				node := lookup.Subtable(0)
				if kind == GSubFeatureType {
					node.GSub.SingleFmt1.DeltaGlyphID = 1
				}
				if extension {
					leaf := *node
					if kind == GSubFeatureType {
						lookup.Type = ot.GSubLookupTypeExtensionSubs
						*node = ot.LookupNode{LookupType: lookup.Type, Format: 1, GSub: &ot.GSubLookupPayload{ExtensionFmt1: &ot.GSubExtensionFmt1Payload{Resolved: &leaf}}}
					} else {
						lookup.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeExtensionPos)
						*node = ot.LookupNode{LookupType: lookup.Type, Format: 1, GPos: &ot.GPosLookupPayload{ExtensionFmt1: &ot.GPosExtensionFmt1Payload{Resolved: &leaf}}}
					}
				}
				st := NewBufferState(GlyphBuffer{9, 10}, NewPosBuffer(2))
				feat := errorTestFeature{kind, []int{0}}
				pos, applied, err := ApplyFeature(font, feat, st, 0)
				if err != nil || applied || pos != 0 || st.Index != 0 || !reflect.DeepEqual(st.Glyphs, GlyphBuffer{9, 10}) || !reflect.DeepEqual(st.Pos, NewPosBuffer(2)) || len(st.AppliedEdits()) != 0 {
					t.Fatalf("non-match scanned/mutated: pos=%d applied=%t err=%v state=%+v", pos, applied, err, st)
				}
				st.Index = 1 // The caller explicitly advances to the covered glyph.
				pos, applied, err = ApplyFeature(font, feat, st, 0)
				if err != nil || !applied || pos != 2 || st.Index != 2 {
					t.Fatalf("covered start: pos=%d applied=%t err=%v", pos, applied, err)
				}
				if kind == GSubFeatureType {
					if !reflect.DeepEqual(st.Glyphs, GlyphBuffer{9, 11}) {
						t.Fatalf("glyphs=%v", st.Glyphs)
					}
				} else if st.Pos[0].XAdvance != 0 || st.Pos[1].XAdvance != 1 {
					t.Fatalf("positions=%v", st.Pos)
				}
			})
		}
	}
}

func TestNestedForwardLookupStaysAtTarget(t *testing.T) {
	for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
		for _, target := range []uint16{0, 1} {
			t.Run(fmt.Sprintf("%d/target=%d", kind, target), func(t *testing.T) {
				font, graph := limitFixture(t, kind)
				limitContext(graph, kind, 0, 1)
				limitLeaf(graph, kind, 1)
				parent, child := graph.Lookup(0).Subtable(0), graph.Lookup(1).Subtable(0)
				// Parent matches [10,11]; child only covers 11. A record targeting
				// input zero must not drift to input one and apply there instead.
				covs := []ot.Coverage{{GlyphRange: testGlyphRange{10}}, {GlyphRange: testGlyphRange{11}}}
				child.Coverage = covs[1]
				if kind == GSubFeatureType {
					parent.GSub.ContextFmt3.InputCoverages = covs
					parent.GSub.ContextFmt3.Records[0].SequenceIndex = target
					child.GSub.SingleFmt1.DeltaGlyphID = 1
				} else {
					parent.GPos.ContextFmt3.InputCoverages = covs
					parent.GPos.ContextFmt3.Records[0].SequenceIndex = target
				}
				st := NewBufferState(GlyphBuffer{10, 11}, NewPosBuffer(2))
				_, applied, err := ApplyFeature(font, errorTestFeature{kind, []int{0}}, st, 0)
				if err != nil || applied != (target == 1) {
					t.Fatalf("applied=%t err=%v", applied, err)
				}
				want := GlyphBuffer{10, 11}
				if kind == GSubFeatureType && target == 1 {
					want[1] = 12
				}
				if !reflect.DeepEqual(st.Glyphs, want) {
					t.Fatalf("glyphs=%v want=%v", st.Glyphs, want)
				}
				if target == 0 && (len(st.AppliedEdits()) != 0 || !reflect.DeepEqual(st.Pos, NewPosBuffer(2))) {
					t.Fatal("missed nested target mutated state")
				}
				if kind == GPosFeatureType && target == 1 && st.Pos[1].XAdvance != 1 {
					t.Fatalf("positions=%v", st.Pos)
				}
			})
		}
	}
}
