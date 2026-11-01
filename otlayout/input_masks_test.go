package otlayout

import (
	"reflect"
	"testing"
)

func TestInputMasksFollowEditsAndClones(t *testing.T) {
	original := NewBufferState(GlyphBuffer{10, 20, 30}, nil)
	masks := []uint32{1, 2, 4}
	original.SetInputMasks(masks, 1)
	masks[0] = 0 // State must own its mask snapshot.
	st := original.CloneShared()
	st.ReplaceGlyphs(1, 2, GlyphBuffer{21, 22})
	st.Set(0, 11)
	st.ReplaceGlyphs(3, 4, nil)
	if !reflect.DeepEqual(st.inputMasks, []uint32{1, 2, 2}) || !reflect.DeepEqual(original.inputMasks, []uint32{1, 2, 4}) {
		t.Fatalf("edited=%v original=%v", st.inputMasks, original.inputMasks)
	}
	if !st.inputEnabled(0) || st.inputEnabled(1) {
		t.Fatal("mask intersection lost")
	}
	st.SetInputMasks(nil, 0)
	if !st.inputEnabled(1) {
		t.Fatal("clearing masks should restore unrestricted input")
	}
}

func TestLigatureInputMasksAndIgnoredSurvivors(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		font := loadTestFont(t, "gpos5_font1.otf")
		st := NewBufferState(GlyphBuffer{17, 30, 19, 19, 31, 17}, NewPosBuffer(6))
		masks := []uint32{0, 1, 2, 4, 0, 0}
		if enabled {
			masks[4] = 1
		}
		st.SetInputMasks(masks, 1)
		st.Index = 1
		_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 0)
		if err != nil || applied != enabled {
			t.Fatalf("enabled=%t applied=%t err=%v", enabled, applied, err)
		}
		wantGlyphs, wantMasks := GlyphBuffer{17, 30, 19, 19, 31, 17}, masks
		if enabled {
			wantGlyphs, wantMasks = GlyphBuffer{17, 18, 19, 19, 17}, []uint32{0, 1, 2, 4, 0}
		}
		if !reflect.DeepEqual(st.Glyphs, wantGlyphs) || !reflect.DeepEqual(st.inputMasks, wantMasks) {
			t.Fatalf("glyphs=%v masks=%v", st.Glyphs, st.inputMasks)
		}
	}
}
