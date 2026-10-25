package otlayout

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

func TestApplyEnvGlyphCountSnapshot(t *testing.T) {
	font := loadTestFont(t, "gsub3_1_simple_f1.otf")
	maxp := font.Table(ot.T("maxp")).Self().AsMaxP()
	original := maxp.NumGlyphs
	env, err := newApplyEnv(font)
	if err != nil || env.numGlyphs != original {
		t.Fatalf("newApplyEnv = %+v, %v; want count %d", env, err, original)
	}
	maxp.NumGlyphs = original + 1
	if env.numGlyphs != original {
		t.Fatal("environment did not retain its glyph-count snapshot")
	}
	for _, count := range []int{1, 65535} {
		maxp.NumGlyphs = count
		env, err = newApplyEnv(font)
		if err != nil || env.numGlyphs != count {
			t.Fatalf("count %d: environment %+v, error %v", count, env, err)
		}
	}
}

func TestApplyFeatureRejectsUnavailableGlyphCount(t *testing.T) {
	type fontCase struct {
		name string
		font *ot.Font
	}
	cases := []fontCase{
		{"nil font", nil},
		{"missing maxp", &ot.Font{}},
	}
	for _, count := range []int{-1, 0, 65536} {
		font := loadTestFont(t, "gsub3_1_simple_f1.otf")
		font.Table(ot.T("maxp")).Self().AsMaxP().NumGlyphs = count
		cases = append(cases, fontCase{fmt.Sprintf("count %d", count), font})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newApplyEnv(tc.font); err == nil || !strings.Contains(err.Error(), "maxp") {
				t.Fatalf("expected descriptive maxp error, got %v", err)
			}
			for _, kind := range []LayoutTagType{GSubFeatureType, GPosFeatureType} {
				st := NewBufferState(GlyphBuffer{10, 11}, NewPosBuffer(2))
				st.Index = 1
				before := NewBufferState(GlyphBuffer{10, 11}, NewPosBuffer(2))
				before.Index = 1
				pos, applied, err := ApplyFeature(tc.font, testFeature{typ: kind}, st, 0)
				if err == nil || applied || pos != 1 {
					t.Fatalf("ApplyFeature = %d, %v, %v", pos, applied, err)
				}
				if !reflect.DeepEqual(st, before) {
					t.Fatalf("validation failure changed buffer state: %+v", st)
				}
			}
		})
	}
}

func TestApplyFeatureNilFeatureNeedsNoEnvironment(t *testing.T) {
	st := NewBufferState(GlyphBuffer{10}, nil)
	pos, applied, err := ApplyFeature(nil, nil, st, 0)
	if pos != 0 || applied || err != nil || st.Glyphs[0] != 10 {
		t.Fatalf("nil feature = %d, %v, %v; state %+v", pos, applied, err, st)
	}
}
