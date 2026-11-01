package otlayout

import (
	"errors"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

// A deferred selection error belongs to the first matched alternate leaf,
// including through a context or extension. Other substitutions still work.
func TestDeferredAlternateSelectionError(t *testing.T) {
	for _, mode := range []string{"direct", "extension", "context", "single", "miss"} {
		t.Run(mode, func(t *testing.T) {
			font := loadTestFont(t, "gsub_context1_lookupflag_f1.otf")
			graph := font.Layout.GSub.LookupGraph()
			cov := ot.Coverage{GlyphRange: testGlyphRange{glyph: 10}}
			alt := ot.LookupNode{LookupType: ot.GSubLookupTypeAlternate, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{AlternateFmt1: &ot.GSubAlternateFmt1Payload{Alternates: [][]ot.GlyphIndex{{11, 12}}}}}
			lookup := graph.Lookup(0)
			lookup.Flag = 0
			*lookup.Subtable(0) = alt
			switch mode {
			case "extension":
				*lookup.Subtable(0) = ot.LookupNode{LookupType: ot.GSubLookupTypeExtensionSubs, Format: 1, GSub: &ot.GSubLookupPayload{ExtensionFmt1: &ot.GSubExtensionFmt1Payload{ResolvedType: alt.LookupType, Resolved: &alt}}}
			case "context":
				child := graph.Lookup(1)
				child.Flag = 0
				child.Type = alt.LookupType
				*child.Subtable(0) = alt
				*lookup.Subtable(0) = ot.LookupNode{LookupType: ot.GSubLookupTypeContext, Format: 3, GSub: &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{cov}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}}}}}
			case "single":
				*lookup.Subtable(0) = ot.LookupNode{LookupType: ot.GSubLookupTypeSingle, Format: 1, Coverage: cov, GSub: &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}}}
			}
			lookup.Type = lookup.Subtable(0).LookupType
			st := NewBufferState(GlyphBuffer{10}, nil)
			if mode == "miss" {
				st.Glyphs[0] = 20
			}
			cause := errors.New("alternate selection conflict")
			st.SetAlternateSelectionError(cause)
			st = st.CloneShared()
			_, applied, err := ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 1)
			switch mode {
			case "single":
				if err != nil || !applied || st.Glyphs[0] != 11 {
					t.Fatalf("single affected: %v %v %v", err, applied, st.Glyphs)
				}
			case "miss":
				if err != nil || applied || st.Glyphs[0] != 20 {
					t.Fatalf("miss affected: %v %v %v", err, applied, st.Glyphs)
				}
			default:
				if !errors.Is(err, cause) || applied || st.Glyphs[0] != 10 {
					t.Fatalf("conflict: %v %v %v", err, applied, st.Glyphs)
				}
				st.SetAlternateSelectionError(nil)
				st.Index = 0
				_, applied, err = ApplyFeature(font, errorTestFeature{GSubFeatureType, []int{0}}, st, 1)
				if err != nil || !applied || st.Glyphs[0] != 11 {
					t.Fatalf("reset: %v %v %v", err, applied, st.Glyphs)
				}
			}
		})
	}
}
