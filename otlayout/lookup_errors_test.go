package otlayout

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type errorTestFeature struct {
	kind    LayoutTagType
	lookups []int
}

func (f errorTestFeature) Tag() ot.Tag           { return ot.T("test") }
func (f errorTestFeature) Type() LayoutTagType   { return f.kind }
func (f errorTestFeature) LookupCount() int      { return len(f.lookups) }
func (f errorTestFeature) LookupIndex(i int) int { return f.lookups[i] }

// The fixture owns these bytes. Modify them before the first lazy lookup read;
// production clients must treat Table.Binary as read-only.
func firstLookupBytes(t *testing.T, font *ot.Font, tag string) []byte {
	t.Helper()
	raw := font.Table(ot.T(tag)).Binary()
	list := raw[int(binary.BigEndian.Uint16(raw[8:])):]
	return list[int(binary.BigEndian.Uint16(list[2:])):]
}

func TestLazyLookupErrorsReachApplyFeature(t *testing.T) {
	for _, tc := range []struct {
		name, file, tag string
		kind            LayoutTagType
	}{
		{"GSUB", "gsub3_1_simple_f1.otf", "GSUB", GSubFeatureType},
		{"GPOS", "gpos_chaining3_boundary_f2.otf", "GPOS", GPosFeatureType},
	} {
		for _, level := range []string{"table", "node"} {
			t.Run(tc.name+"/"+level, func(t *testing.T) {
				font := loadTestFont(t, tc.file)
				raw := firstLookupBytes(t, font, tc.tag)
				if level == "table" {
					binary.BigEndian.PutUint16(raw[6:], 0xffff)
				} else {
					sub := raw[int(binary.BigEndian.Uint16(raw[6:])):]
					binary.BigEndian.PutUint16(sub[2:], 0xffff) // invalid coverage offset
				}
				var graph *ot.LookupListGraph
				if tc.kind == GPosFeatureType {
					graph = font.Layout.GPos.LookupGraph()
				} else {
					graph = font.Layout.GSub.LookupGraph()
				}
				lookup := graph.Lookup(0)
				cause := lookup.Error()
				if level == "node" {
					cause = lookup.Subtable(0).Error()
				}
				if cause == nil {
					t.Fatal("fixture did not produce a lazy parser error")
				}
				st := NewBufferState(GlyphBuffer{10}, nil)
				pos, applied, err := ApplyFeature(font, errorTestFeature{tc.kind, []int{0}}, st, 0)
				if !errors.Is(err, cause) || applied || pos != 0 || st.Glyphs[0] != 10 {
					t.Fatalf("got %d, %v, %v, glyphs %v; cause %v", pos, applied, err, st.Glyphs, cause)
				}
				if !strings.Contains(err.Error(), "feature test lookup 0") {
					t.Fatalf("missing feature/lookup context: %v", err)
				}
				if level == "node" && !strings.Contains(err.Error(), "subtable 0") {
					t.Fatalf("missing subtable context: %v", err)
				}
			})
		}
	}
}

func TestSubtableFallbackStopsOnlyOnApplicationOrError(t *testing.T) {
	for _, mode := range []string{"miss", "error", "applied"} {
		t.Run(mode, func(t *testing.T) {
			font := loadTestFont(t, "gsub3_1_simple_f1.otf")
			raw := firstLookupBytes(t, font, "GSUB")
			words := []uint16{
				1, 0, 2, 10, 22, // single lookup, two subtables
				1, 6, 1, 1, 1, 10, // single format 1, delta 1, coverage {10}
				1, 6, 2, 1, 1, 10, // delta 2, coverage {10}
			}
			if len(raw) < len(words)*2 {
				t.Fatal("fixture too small")
			}
			for i, w := range words {
				binary.BigEndian.PutUint16(raw[2*i:], w)
			}
			switch mode {
			case "miss":
				binary.BigEndian.PutUint16(raw[20:], 99)
			case "error":
				binary.BigEndian.PutUint16(raw[12:], 0xffff)
			}
			st := NewBufferState(GlyphBuffer{10}, nil)
			_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
			switch mode {
			case "error":
				if err == nil || applied || st.Glyphs[0] != 10 {
					t.Fatalf("fallback after error: %v %v %v", applied, err, st.Glyphs)
				}
			default:
				want := ot.GlyphIndex(11)
				if mode == "miss" {
					want = 12
				}
				if err != nil || !applied || st.Glyphs[0] != want {
					t.Fatalf("got %v %v %v; want %d", applied, err, st.Glyphs, want)
				}
			}
		})
	}
}

func TestNestedLookupErrorsPreserveAppliedFlag(t *testing.T) {
	for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
		for _, prior := range []bool{false, true} {
			t.Run(fmt.Sprintf("kind=%d/prior=%t", kind, prior), func(t *testing.T) {
				file := "gsub_context1_lookupflag_f1.otf"
				if kind == GPosFeatureType {
					file = "gpos_chaining3_boundary_f2.otf"
				}
				font := loadTestFont(t, file)
				var graph *ot.LookupListGraph
				if kind == GPosFeatureType {
					graph = font.Layout.GPos.LookupGraph()
				} else {
					graph = font.Layout.GSub.LookupGraph()
				}
				cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
				records := []ot.SequenceLookupRecord{{SequenceIndex: 0, LookupListIndex: 65535}}
				if prior {
					records = append([]ot.SequenceLookupRecord{{SequenceIndex: 0, LookupListIndex: 1}}, records...)
				}
				parent := graph.Lookup(0)
				parent.Flag = 0
				child := graph.Lookup(1)
				child.Flag = 0
				if kind == GSubFeatureType {
					parent.Type = ot.GSubLookupTypeContext
					child.Type = ot.GSubLookupTypeSingle
					*parent.Subtable(0) = ot.LookupNode{LookupType: parent.Type, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: records}}}
					*child.Subtable(0) = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
				} else {
					parent.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeContextPos)
					child.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeSingle)
					*parent.Subtable(0) = ot.LookupNode{LookupType: parent.Type, Format: 3, GPos: &ot.GPosLookupPayload{ContextFmt3: &ot.GPosContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: records}}}
					*child.Subtable(0) = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov, GPos: &ot.GPosLookupPayload{SingleFmt1: &ot.GPosSingleFmt1Payload{}}}
				}
				st := NewBufferState(GlyphBuffer{10}, nil)
				_, applied, err := ApplyFeature(font, errorTestFeature{kind, []int{0}}, st, 0)
				if err == nil || applied != prior || !strings.Contains(err.Error(), "lookup 65535") || !strings.Contains(err.Error(), "sequence record") {
					t.Fatalf("applied=%v error=%v; want prior=%v", applied, err, prior)
				}
				if prior && kind == GSubFeatureType && st.Glyphs[0] != 11 {
					t.Fatal("earlier substitution lost")
				}
			})
		}
	}
}

func TestSequenceRecordInvalidInputIndex(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	_, _, applied, err := applySequenceLookupRecords(NewBufferState(GlyphBuffer{10}, nil), []int{0},
		[]ot.SequenceLookupRecord{{SequenceIndex: 1}}, font.Layout.GSub.LookupGraph(),
		errorTestFeature{GSubFeatureType, []int{0}}, 0, nil, applyEnv{numGlyphs: 100})
	if err == nil || applied || !strings.Contains(err.Error(), "input index 1") {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
}

func TestDispatchRejectsMissingPayloadAndUnsupportedFormat(t *testing.T) {
	for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
		for _, mode := range []string{"missing payload", "unsupported format", "extension", "unknown type"} {
			t.Run(fmt.Sprintf("%d/%s", kind, mode), func(t *testing.T) {
				sub := &ot.LookupNode{LookupType: 1, Format: 1, Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}}
				switch mode {
				case "unsupported format":
					sub.Format = 99
				case "extension":
					sub.LookupType = ot.GSubLookupTypeExtensionSubs
					if kind == GPosFeatureType {
						sub.LookupType = ot.GPosLookupTypeExtensionPos
					}
				case "unknown type":
					sub.LookupType = 99
				}
				ctx := &applyCtx{buf: NewBufferState(GlyphBuffer{10}, nil), subnode: sub}
				var applied bool
				var err error
				if kind == GSubFeatureType {
					_, applied, _, _, _, err = dispatchGSubLookup(ctx, sub)
				} else {
					_, applied, _, _, _, err = dispatchGPosLookup(ctx, sub)
				}
				if err == nil || applied || ctx.buf.Glyphs[0] != 10 {
					t.Fatalf("applied=%v err=%v", applied, err)
				}
			})
		}
	}
}

func TestResolvedExtensionErrorPreservesCause(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	raw := firstLookupBytes(t, font, "GSUB")
	sub := raw[int(binary.BigEndian.Uint16(raw[6:])):]
	binary.BigEndian.PutUint16(sub[2:], 0xffff)
	badNode := font.Layout.GSub.LookupGraph().Lookup(0).Subtable(0)
	cause := badNode.Error()
	if cause == nil {
		t.Fatal("expected malformed fixture")
	}
	for _, wrapper := range []*ot.LookupNode{
		{GSub: &ot.GSubLookupPayload{ExtensionFmt1: &ot.GSubExtensionFmt1Payload{Resolved: badNode}}},
		{GPos: &ot.GPosLookupPayload{ExtensionFmt1: &ot.GPosExtensionFmt1Payload{Resolved: badNode}}},
	} {
		_, err := effectiveLookupNode(wrapper)
		if !errors.Is(err, cause) {
			t.Fatalf("resolved cause lost: %v", err)
		}
	}
}

func TestFeatureErrorAfterEarlierLookup(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	st := NewBufferState(GlyphBuffer{18, 18}, nil)
	pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0, 65535}}, st, 0)
	if err == nil || !applied || pos != st.Index || st.Glyphs[0] == 18 {
		t.Fatalf("pos=%d applied=%v err=%v state=%+v", pos, applied, err, st)
	}
}

func TestLookupGraphErrorReachesApplication(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	binary.BigEndian.PutUint16(firstLookupBytes(t, font, "GSUB")[4:], 65535)
	font, err := ot.Parse(font.Binary(), ot.IsTestfont)
	if err != nil {
		t.Fatal(err)
	}
	cause := font.Layout.GSub.LookupGraph().Error()
	if cause == nil {
		t.Fatal("expected malformed lookup graph")
	}
	st := NewBufferState(GlyphBuffer{18}, nil)
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if applied || !errors.Is(err, cause) {
		t.Fatalf("applied=%v err=%v cause=%v", applied, err, cause)
	}
}

func TestZeroDeltaIsAppliedAndCoverageMissIsNot(t *testing.T) {
	for _, glyph := range []ot.GlyphIndex{10, 11} {
		sub := &ot.LookupNode{
			LookupType: ot.GSubLookupTypeSingle, Format: 1,
			Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}},
			GSub:     &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{}},
		}
		ctx := &applyCtx{env: applyEnv{numGlyphs: 100}, buf: NewBufferState(GlyphBuffer{glyph}, nil), subnode: sub}
		_, applied, out, _, _, err := dispatchGSubLookup(ctx, sub)
		if err != nil || applied != (glyph == 10) || out[0] != glyph {
			t.Fatalf("glyph=%d applied=%v err=%v out=%v", glyph, applied, err, out)
		}
	}
}

func TestContextRequiresGraphForNonemptyRecords(t *testing.T) {
	_, _, applied, err := applySequenceLookupRecords(NewBufferState(GlyphBuffer{10}, nil), []int{0},
		[]ot.SequenceLookupRecord{{}}, nil, errorTestFeature{GSubFeatureType, []int{0}},
		0, nil, applyEnv{numGlyphs: 100})
	if err == nil || applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
}

func TestApplyFeatureInvalidRequiredArguments(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	feat := errorTestFeature{GSubFeatureType, []int{0}}
	for _, st := range []*BufferState{nil, NewBufferState(nil, nil), {Glyphs: GlyphBuffer{10}, Index: -1}} {
		if _, applied, err := ApplyFeature(font, feat, st, 0); err == nil || applied {
			t.Fatalf("invalid state %+v: applied=%v err=%v", st, applied, err)
		}
	}
	if _, applied, err := ApplyFeature(font, errorTestFeature{kind: 99}, NewBufferState(GlyphBuffer{10}, nil), 0); err == nil || applied {
		t.Fatalf("invalid feature type: applied=%v err=%v", applied, err)
	}
}
