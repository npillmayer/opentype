package otlayout

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestSingleFmt1GlyphBounds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   ot.GlyphIndex
		delta   int16
		count   int
		want    ot.GlyphIndex
		wantErr bool
	}{
		{"positive", 10, 2, 20, 12, false},
		{"negative", 10, -2, 20, 8, false},
		{"zero delta", 10, 0, 20, 10, false},
		{"notdef", 10, -10, 20, 0, false},
		{"last glyph", 10, 9, 20, 19, false},
		{"at count", 10, 10, 20, 0, true},
		{"above count", 10, 11, 20, 0, true},
		{"wrap negative valid", 0, -2, 65535, 65534, false},
		{"wrap negative invalid", 0, -1, 65535, 0, true},
		{"wrap positive valid", 65534, 2, 65535, 0, false},
		{"wrap positive invalid", 65534, 102, 100, 0, true},
		{"minimum delta", 0, -32768, 65535, 32768, false},
		{"maximum delta", 32768, 32767, 65535, 0, true},
		{"missing count", 10, 1, 0, 0, true},
		{"invalid count", 10, 1, 65536, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sub := &ot.LookupNode{
				LookupType: ot.GSubLookupTypeSingle, Format: 1,
				Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: tc.input}},
				GSub:     &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: tc.delta}},
			}
			st := NewBufferState(GlyphBuffer{42, tc.input}, NewPosBuffer(2))
			st.Index = 1
			shared := st.CloneShared()
			before := *st
			before.Glyphs = append(GlyphBuffer(nil), st.Glyphs...)
			before.Pos = append(PosBuffer(nil), st.Pos...)
			ctx := &applyCtx{env: applyEnv{numGlyphs: tc.count}, buf: st, subnode: sub, pos: 1}
			pos, applied, out, edit, err := gsubLookupType1Fmt1(ctx, sub, st.Glyphs, 1)
			if tc.wantErr {
				if err == nil || applied || edit != nil || pos != 1 || !reflect.DeepEqual(*st, before) || !reflect.DeepEqual(out, before.Glyphs) {
					t.Fatalf("failure mutated state: pos=%d applied=%v edit=%v err=%v state=%+v", pos, applied, edit, err, st)
				}
				if !strings.Contains(err.Error(), "glyph count") {
					t.Fatalf("unhelpful error: %v", err)
				}
			} else if err != nil || !applied || pos != 2 || out[1] != tc.want || edit == nil || *edit != (EditSpan{From: 1, To: 2, Len: 1}) {
				t.Fatalf("pos=%d applied=%v out=%v edit=%v err=%v; want glyph %d", pos, applied, out, edit, err, tc.want)
			}
			if shared.Glyphs[1] != tc.input {
				t.Fatal("changed shared original buffer")
			}
		})
	}
}

func TestSingleFmt1InvalidGlyphStopsSubtableFallback(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs = 20
	raw := firstLookupBytes(t, font, "GSUB")
	words := []uint16{
		1, 0, 2, 10, 22,
		1, 6, 10, 1, 1, 10, // result 20 is invalid
		1, 6, 1, 1, 1, 10, // valid fallback must not run
	}
	if len(raw) < 2*len(words) {
		t.Fatal("fixture too small")
	}
	for i, w := range words {
		binary.BigEndian.PutUint16(raw[2*i:], w)
	}
	st := NewBufferState(GlyphBuffer{10}, NewPosBuffer(1))
	pos, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || applied || pos != 0 || st.Index != 0 || st.Glyphs[0] != 10 {
		t.Fatalf("pos=%d applied=%v err=%v state=%+v", pos, applied, err, st)
	}
	for _, context := range []string{"feature test lookup 0", "subtable 0", "input glyph 10", "delta 10", "glyph 20", "glyph count 20"} {
		if !strings.Contains(err.Error(), context) {
			t.Fatalf("missing %q in %v", context, err)
		}
	}
}

func TestNestedSingleFmt1UsesFontGlyphCount(t *testing.T) {
	font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
	font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs = 20
	graph := font.Layout.GSub.LookupGraph()
	cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
	parent, child := graph.Lookup(0), graph.Lookup(1)
	parent.Flag, child.Flag = 0, 0
	parent.Type, child.Type = ot.GSubLookupTypeContext, ot.GSubLookupTypeSingle
	*parent.Subtable(0) = ot.LookupNode{LookupType: parent.Type, Format: 3, GSub: &ot.GSubLookupPayload{
		ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}}},
	}}
	*child.Subtable(0) = ot.LookupNode{LookupType: child.Type, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{
		SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 10},
	}}
	st := NewBufferState(GlyphBuffer{10}, nil)
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || applied || st.Glyphs[0] != 10 || !strings.Contains(err.Error(), "sequence record 0") || !strings.Contains(err.Error(), "glyph count 20") {
		t.Fatalf("applied=%v err=%v glyphs=%v", applied, err, st.Glyphs)
	}
}

func TestExtensionSingleFmt1UsesFontGlyphCount(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs = 20
	lookup := font.Layout.GSub.LookupGraph().Lookup(0)
	node := lookup.Subtable(0)
	lookup.Type = ot.GSubLookupTypeExtensionSubs
	resolved := &ot.LookupNode{
		LookupType: ot.GSubLookupTypeSingle, Format: 1,
		Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}},
		GSub:     &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 10}},
	}
	*node = ot.LookupNode{LookupType: lookup.Type, Format: 1,
		GSub: &ot.GSubLookupPayload{ExtensionFmt1: &ot.GSubExtensionFmt1Payload{Resolved: resolved}},
	}
	st := NewBufferState(GlyphBuffer{10}, nil)
	_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
	if err == nil || applied || st.Glyphs[0] != 10 || !strings.Contains(err.Error(), "glyph count 20") {
		t.Fatalf("applied=%v err=%v glyphs=%v", applied, err, st.Glyphs)
	}
}
