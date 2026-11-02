package otlayout

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

// Parsed anchors provide independent placement expectations; a controlled type-6
// graph and parsed GDEF distinguish coverage matching from predecessor selection.
func TestSHAPE022MarkToMarkPredecessor(t *testing.T) {
	for _, extension := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			glyphs  GlyphBuffer
			flag    ot.LayoutTableLookupFlag
			filter  bool
			masks   []uint32
			covered ot.GlyphIndex
			target  int // -1 means no attachment
		}{
			{name: "adjacent marks", glyphs: GlyphBuffer{90, 90}, target: 0},
			{name: "nearest covered mark", glyphs: GlyphBuffer{90, 90, 90}, target: 1},
			{name: "base barrier", glyphs: GlyphBuffer{90, 80, 90}, target: -1},
			{name: "ignored base still barrier", glyphs: GlyphBuffer{90, 80, 90}, flag: ot.LOOKUP_FLAG_IGNORE_BASE_GLYPHS, target: -1},
			{name: "ignored ligature still barrier", glyphs: GlyphBuffer{90, 85, 90}, flag: ot.LOOKUP_FLAG_IGNORE_LIGATURES, target: -1},
			{name: "unclassified barrier", glyphs: GlyphBuffer{90, 10, 90}, target: -1},
			{name: "covered base is not a mark", glyphs: GlyphBuffer{80, 90}, covered: 80, target: -1},
			{name: "uncovered mark blocks earlier coverage", glyphs: GlyphBuffer{90, 91, 90}, target: -1},
			{name: "filtered mark may be skipped", glyphs: GlyphBuffer{90, 91, 90}, flag: ot.LOOKUP_FLAG_USE_MARK_FILTERING_SET, filter: true, target: 0},
			{name: "filter does not hide base", glyphs: GlyphBuffer{90, 80, 91, 90}, flag: ot.LOOKUP_FLAG_USE_MARK_FILTERING_SET | ot.LOOKUP_FLAG_IGNORE_BASE_GLYPHS, filter: true, target: -1},
			{name: "disabled predecessor blocks earlier coverage", glyphs: GlyphBuffer{90, 90, 90}, masks: []uint32{1, 0, 1}, target: -1},
			{name: "disabled filtered mark may be skipped", glyphs: GlyphBuffer{90, 91, 90}, masks: []uint32{1, 0, 1}, flag: ot.LOOKUP_FLAG_USE_MARK_FILTERING_SET, filter: true, target: 0},
			{name: "ignore marks still excludes lookup start", glyphs: GlyphBuffer{90, 90}, flag: ot.LOOKUP_FLAG_IGNORE_MARKS, target: -1},
			{name: "no predecessor", glyphs: GlyphBuffer{90}, target: -1},
		} {
			t.Run(fmt.Sprintf("%s/extension=%t", tc.name, extension), func(t *testing.T) {
				font := loadTestFont(t, "gpos4_simple_1.otf")
				font.Layout.GDef = loadTestFont(t, "gsub_context1_lookupflag_f1.otf").Layout.GDef
				lookup := font.Layout.GPos.LookupGraph().Lookup(0)
				if tc.filter {
					// The fixture originally has no filtering flag. Supply the
					// set at the retained parsed index for this controlled graph.
					index := lookup.MarkFilteringSet()
					font.Layout.GDef.MarkGlyphSets = make([]ot.GlyphRange, int(index)+1)
					font.Layout.GDef.MarkGlyphSets[index] = testGlyphRange{glyph: 90}
				}
				node := lookup.Subtable(0)
				anchors := node.GPos.MarkToBaseFmt1
				covered := tc.covered
				if covered == 0 {
					covered = 90
				}
				lookup.Type, lookup.Flag = ot.MaskGPosLookupType(ot.GPosLookupTypeMarkToMark), tc.flag
				*node = ot.LookupNode{LookupType: lookup.Type, Format: 1,
					Coverage: ot.Coverage{GlyphRange: testGlyphRange{glyph: 90}},
					GPos: &ot.GPosLookupPayload{MarkToMarkFmt1: &ot.GPosMarkToMarkFmt1Payload{
						Mark2Coverage:  ot.Coverage{GlyphRange: testGlyphRange{glyph: covered}},
						MarkClassCount: anchors.MarkClassCount, Mark1Records: anchors.MarkRecords, Mark2Records: anchors.BaseRecords,
					}},
				}
				if extension {
					leaf := *node
					lookup.Type = ot.MaskGPosLookupType(ot.GPosLookupTypeExtensionPos)
					*node = ot.LookupNode{LookupType: lookup.Type, Format: 1, GPos: &ot.GPosLookupPayload{ExtensionFmt1: &ot.GPosExtensionFmt1Payload{Resolved: &leaf}}}
				}
				st := NewBufferState(append(GlyphBuffer(nil), tc.glyphs...), NewPosBuffer(len(tc.glyphs)))
				start := len(tc.glyphs) - 1
				st.Index = start
				st.Pos[start].XOffset, st.Pos[start].YOffset, st.Pos[start].XAdvance = 99, -88, 7
				if tc.target >= 0 {
					st.Pos[tc.target].YOffset = 13
				}
				before := append(PosBuffer(nil), st.Pos...)
				st.SetInputMasks(tc.masks, 1)
				pos, applied, err := ApplyFeature(font, errorTestFeature{GPosFeatureType, []int{0}}, st, 0)
				if err != nil || applied != (tc.target >= 0) {
					t.Fatalf("applied=%t err=%v", applied, err)
				}
				if !reflect.DeepEqual(st.Glyphs, tc.glyphs) || len(st.AppliedEdits()) != 0 || lookup.Flag != tc.flag {
					t.Fatal("positioning changed glyphs, journal or lookup flags")
				}
				if tc.target < 0 {
					if pos != start || !reflect.DeepEqual(st.Pos, before) {
						t.Fatalf("nonmatch mutated state: pos=%d positions=%+v", pos, st.Pos)
					}
					return
				}
				p := st.Pos[start]
				// The existing TTX anchors are mark=(200,230), target=(100,150).
				if pos != start+1 || p.AttachTo != int32(tc.target) || p.AttachKind != AttachMarkToMark || !p.MarkAttachmentPending || p.XOffset != -100 || p.YOffset != -67 || p.XAdvance != 7 || p.AnchorRef.MarkAnchor == 0 || p.AnchorRef.BaseAnchor == 0 {
					t.Fatalf("pos=%d attachment=%+v", pos, p)
				}
				if !reflect.DeepEqual(st.Pos[:start], before[:start]) {
					t.Fatal("attachment changed predecessor positions")
				}
			})
		}
	}
}
