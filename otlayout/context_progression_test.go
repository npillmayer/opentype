package otlayout

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

// All twelve dispatch paths must publish the input end. Classes deliberately
// use the real fixture's default class zero for the unclassified input glyphs.
func TestContextProgressionFormats(t *testing.T) {
	for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
		for _, chained := range []bool{false, true} {
			for format := uint16(1); format <= 3; format++ {
				for _, ignored := range []bool{false, true} {
					t.Run(fmt.Sprintf("kind=%d/chained=%t/format=%d/marks=%t", kind, chained, format, ignored), func(t *testing.T) {
						font, graph := limitFixture(t, kind)
						font.Layout.GDef = loadTestFont(t, "gsub_context1_lookupflag_f1.otf").Layout.GDef
						defs := font.Layout.GDef.GlyphClassDef
						cov := func(g ot.GlyphIndex) ot.Coverage { return ot.Coverage{GlyphRange: testGlyphRange{glyph: g}} }
						records := []ot.SequenceLookupRecord{{LookupListIndex: 1}}
						l := graph.Lookup(0)
						n := l.Subtable(0)
						l.Flag = 0
						if ignored {
							l.Flag = ot.LOOKUP_FLAG_IGNORE_MARKS
						}
						*n = ot.LookupNode{Format: format, Coverage: cov(10)}
						if kind == GSubFeatureType {
							l.Type = ot.GSubLookupTypeContext
							p := &ot.GSubLookupPayload{}
							n.GSub = p
							if chained {
								l.Type = ot.GSubLookupTypeChainingContext
								switch format {
								case 1:
									p.ChainingContextFmt1 = &ot.GSubChainingContextFmt1Payload{RuleSets: [][]ot.GSubChainedSequenceRule{{{Backtrack: []ot.GlyphIndex{9}, Input: []ot.GlyphIndex{11}, Lookahead: []ot.GlyphIndex{12}, Records: records}}}}
								case 2:
									p.ChainingContextFmt2 = &ot.GSubChainingContextFmt2Payload{BacktrackClassDef: defs, InputClassDef: defs, LookaheadClassDef: defs, RuleSets: [][]ot.GSubChainedClassRule{{{Backtrack: []uint16{0}, Input: []uint16{0}, Lookahead: []uint16{0}, Records: records}}}}
								case 3:
									p.ChainingContextFmt3 = &ot.GSubChainingContextFmt3Payload{BacktrackCoverages: []ot.Coverage{cov(9)}, InputCoverages: []ot.Coverage{cov(10), cov(11)}, LookaheadCoverages: []ot.Coverage{cov(12)}, Records: records}
								}
							} else {
								switch format {
								case 1:
									p.ContextFmt1 = &ot.GSubContextFmt1Payload{RuleSets: [][]ot.GSubSequenceRule{{{InputGlyphs: []ot.GlyphIndex{11}, Records: records}}}}
								case 2:
									p.ContextFmt2 = &ot.GSubContextFmt2Payload{ClassDef: defs, RuleSets: [][]ot.GSubClassSequenceRule{{{InputClasses: []uint16{0}, Records: records}}}}
								case 3:
									p.ContextFmt3 = &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov(10), cov(11)}, Records: records}
								}
							}
						} else {
							l.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeContextPos)
							p := &ot.GPosLookupPayload{}
							n.GPos = p
							if chained {
								l.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeChainedContextPos)
								switch format {
								case 1:
									p.ChainingContextFmt1 = &ot.GPosChainingContextFmt1Payload{RuleSets: [][]ot.GPosChainedSequenceRule{{{Backtrack: []ot.GlyphIndex{9}, Input: []ot.GlyphIndex{11}, Lookahead: []ot.GlyphIndex{12}, Records: records}}}}
								case 2:
									p.ChainingContextFmt2 = &ot.GPosChainingContextFmt2Payload{BacktrackClassDef: defs, InputClassDef: defs, LookaheadClassDef: defs, RuleSets: [][]ot.GPosChainedClassRule{{{Backtrack: []uint16{0}, Input: []uint16{0}, Lookahead: []uint16{0}, Records: records}}}}
								case 3:
									p.ChainingContextFmt3 = &ot.GPosChainingContextFmt3Payload{BacktrackCoverages: []ot.Coverage{cov(9)}, InputCoverages: []ot.Coverage{cov(10), cov(11)}, LookaheadCoverages: []ot.Coverage{cov(12)}, Records: records}
								}
							} else {
								switch format {
								case 1:
									p.ContextFmt1 = &ot.GPosContextFmt1Payload{RuleSets: [][]ot.GPosSequenceRule{{{InputGlyphs: []ot.GlyphIndex{11}, Records: records}}}}
								case 2:
									p.ContextFmt2 = &ot.GPosContextFmt2Payload{ClassDef: defs, RuleSets: [][]ot.GPosClassSequenceRule{{{InputClasses: []uint16{0}, Records: records}}}}
								case 3:
									p.ContextFmt3 = &ot.GPosContextFmt3Payload{InputCoverages: []ot.Coverage{cov(10), cov(11)}, Records: records}
								}
							}
						}
						n.LookupType = l.Type
						limitLeaf(graph, kind, 1) // GSUB zero delta still counts as application.
						input := GlyphBuffer{9, 10, 11, 12, 13}
						end := 3
						if ignored {
							input = GlyphBuffer{9, 10, 90, 11, 91, 12, 13}
							end = 4
						}
						for _, miss := range []bool{false, true} {
							child := graph.Lookup(1).Subtable(0)
							child.Coverage = cov(10)
							if miss {
								child.Coverage = cov(99)
							}
							st := NewBufferState(append(GlyphBuffer(nil), input...), NewPosBuffer(len(input)))
							st.Index = 1
							pos, applied, err := ApplyFeature(font, errorTestFeature{kind, []int{0}}, st, 0)
							wantEnd := end
							if miss {
								wantEnd = 1
							}
							if err != nil || applied == miss || pos != wantEnd || st.Index != wantEnd || !reflect.DeepEqual(st.Glyphs, input) {
								t.Fatalf("miss=%t: pos=%d index=%d applied=%t err=%v glyphs=%v; want end %d", miss, pos, st.Index, applied, err, st.Glyphs, wantEnd)
							}
							wantPos := NewPosBuffer(len(input))
							if kind == GPosFeatureType && !miss {
								wantPos[1].XAdvance = 1
							}
							if !reflect.DeepEqual(st.Pos, wantPos) {
								t.Fatalf("positions=%v want=%v", st.Pos, wantPos)
							}
						}
					})
				}
			}
		}
	}
}

func TestContextProgressionThroughEdits(t *testing.T) {
	for _, scenario := range []string{"expand-first", "expand-last", "nested-expand", "contract-input", "contract-across-end", "zero-net-length"} {
		t.Run(scenario, func(t *testing.T) {
			font, graph := limitFixture(t, GSubFeatureType)
			cov := func(g ot.GlyphIndex) ot.Coverage { return ot.Coverage{GlyphRange: testGlyphRange{glyph: g}} }
			context := func(index int, inputs []ot.GlyphIndex, records []ot.SequenceLookupRecord) {
				limitContext(graph, GSubFeatureType, index)
				p := graph.Lookup(index).Subtable(0).GSub.ContextFmt3
				p.InputCoverages = nil
				for _, g := range inputs {
					p.InputCoverages = append(p.InputCoverages, cov(g))
				}
				p.Records = records
			}
			multiple := func(index int, g ot.GlyphIndex, out []ot.GlyphIndex) {
				l := graph.Lookup(index)
				n := l.Subtable(0)
				l.Type, l.Flag = ot.GSubLookupTypeMultiple, 0
				*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov(g), GSub: &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{out}}}}
			}
			ligature := func(index int, g ot.GlyphIndex, components []ot.GlyphIndex, out ot.GlyphIndex) {
				l := graph.Lookup(index)
				n := l.Subtable(0)
				l.Type, l.Flag = ot.GSubLookupTypeLigature, 0
				*n = ot.LookupNode{LookupType: l.Type, Format: 1, Coverage: cov(g), GSub: &ot.GSubLookupPayload{LigatureFmt1: &ot.GSubLigatureFmt1Payload{LigatureSets: [][]ot.GSubLigatureRule{{{Components: components, Ligature: out}}}}}}
			}
			records := []ot.SequenceLookupRecord{{LookupListIndex: 1}}
			input := GlyphBuffer{9, 10, 20, 30, 40}
			want := GlyphBuffer{9, 11, 12, 20, 30, 40}
			end := 4
			switch scenario {
			case "expand-first":
				multiple(1, 10, []ot.GlyphIndex{11, 12})
			case "expand-last":
				records[0].SequenceIndex = 1
				multiple(1, 20, []ot.GlyphIndex{21, 22})
				want = GlyphBuffer{9, 10, 21, 22, 30, 40}
			case "nested-expand":
				context(1, []ot.GlyphIndex{10}, []ot.SequenceLookupRecord{{LookupListIndex: 2}})
				multiple(2, 10, []ot.GlyphIndex{11, 12})
			case "contract-input":
				ligature(1, 10, []ot.GlyphIndex{20}, 11)
				want = GlyphBuffer{9, 11, 30, 40}
				end = 2
			case "contract-across-end":
				records[0].SequenceIndex = 1
				ligature(1, 20, []ot.GlyphIndex{30}, 21)
				want = GlyphBuffer{9, 10, 21, 40}
				end = 3
			case "zero-net-length":
				multiple(1, 10, []ot.GlyphIndex{11, 12})
				ligature(2, 11, []ot.GlyphIndex{12}, 13)
				records = append(records, ot.SequenceLookupRecord{LookupListIndex: 2})
				want = GlyphBuffer{9, 13, 20, 30, 40}
				end = 3
			}
			context(0, []ot.GlyphIndex{10, 20}, records)
			// A later record still targets the original second input, even after a
			// child restored its own cursor or changed the outer continuation.
			limitLeaf(graph, GSubFeatureType, 3)
			leaf := graph.Lookup(3).Subtable(0)
			leaf.Coverage = cov(20)
			leaf.GSub.SingleFmt1.DeltaGlyphID = 1
			if scenario == "expand-first" || scenario == "nested-expand" {
				graph.Lookup(0).Subtable(0).GSub.ContextFmt3.Records = append(records, ot.SequenceLookupRecord{SequenceIndex: 1, LookupListIndex: 3})
				want[3] = 21
			}
			for _, fail := range []bool{false, true} {
				p := graph.Lookup(0).Subtable(0).GSub.ContextFmt3
				if fail {
					p.Records = append(p.Records, ot.SequenceLookupRecord{LookupListIndex: 65535})
				}
				st := NewBufferState(append(GlyphBuffer(nil), input...), NewPosBuffer(len(input)))
				st.Index = 1
				pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
				wantEnd := end
				if fail {
					wantEnd = 1
				}
				if (err != nil) != fail || !applied || pos != wantEnd || st.Index != wantEnd || !reflect.DeepEqual(st.Glyphs, want) || len(st.Pos) != len(want) || len(st.AppliedEdits()) == 0 {
					t.Fatalf("failure=%t: pos=%d index=%d applied=%t err=%v glyphs=%v; want end=%d glyphs=%v", fail, pos, st.Index, applied, err, st.Glyphs, wantEnd, want)
				}
			}
		})
	}
}

func TestClassSequenceStartsAtRequestedPosition(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	defs := font.Layout.GDef.GlyphClassDef // GIDs 90/91 are marks, class 3.
	buf := GlyphBuffer{10, 90, 91, 20}
	forward, ok := matchClassSequenceForward(nil, buf, 1, defs, []uint16{3, 3})
	if !ok || !reflect.DeepEqual(forward, []int{1, 2}) {
		t.Fatalf("forward=%v matched=%t; want [1 2]", forward, ok)
	}
	backward, ok := matchClassSequenceBackward(nil, buf, 3, defs, []uint16{3, 3})
	if !ok || !reflect.DeepEqual(backward, []int{2, 1}) {
		t.Fatalf("backward=%v matched=%t; want [2 1]", backward, ok)
	}
	if positions, ok := matchClassSequenceForward(nil, buf, 0, defs, []uint16{3}); ok {
		t.Fatalf("class mismatch must not scan ahead: %v", positions)
	}
}
