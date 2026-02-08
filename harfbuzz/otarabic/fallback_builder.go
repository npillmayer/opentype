package otarabic

import (
	"sort"

	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/npillmayer/opentype/harfbuzz"
)

// This matches OpenType lookup flag bit 3 ("ignore marks").
const lookupFlagIgnoreMarks uint16 = 1 << 3

// used to sort both arrays at the same time
type jointGlyphs struct {
	glyphs, substitutes []tables.GlyphID
}

func (a jointGlyphs) Len() int { return len(a.glyphs) }
func (a jointGlyphs) Swap(i, j int) {
	a.glyphs[i], a.glyphs[j] = a.glyphs[j], a.glyphs[i]
	a.substitutes[i], a.substitutes[j] = a.substitutes[j], a.substitutes[i]
}
func (a jointGlyphs) Less(i, j int) bool { return a.glyphs[i] < a.glyphs[j] }

func arabicFallbackSynthesizeLookupSingle(ft *harfbuzz.Font, featureIndex int) *harfbuzz.SyntheticGSUBLookup {
	var glyphs, substitutes []tables.GlyphID

	for u := rune(firstArabicShape); u <= lastArabicShape; u++ {
		s := rune(arabicShaping[u-firstArabicShape][featureIndex])
		uGlyph, hasU := ft.NominalGlyph(u)
		sGlyph, hasS := ft.NominalGlyph(s)

		if s == 0 || !hasU || !hasS || uGlyph == sGlyph {
			continue
		}

		glyphs = append(glyphs, tables.GlyphID(uGlyph))
		substitutes = append(substitutes, tables.GlyphID(sGlyph))
	}

	if len(glyphs) == 0 {
		return nil
	}

	sort.Stable(jointGlyphs{glyphs: glyphs, substitutes: substitutes})

	return &harfbuzz.SyntheticGSUBLookup{
		LookupFlags: lookupFlagIgnoreMarks,
		Subtables: []tables.GSUBLookup{
			tables.SingleSubs{Data: tables.SingleSubstData2{
				Coverage:           tables.Coverage1{Glyphs: glyphs},
				SubstituteGlyphIDs: substitutes,
			}},
		},
	}
}

// used to sort both arrays at the same time
type glyphsIndirections struct {
	glyphs       []tables.GlyphID
	indirections []int
}

func (a glyphsIndirections) Len() int { return len(a.glyphs) }
func (a glyphsIndirections) Swap(i, j int) {
	a.glyphs[i], a.glyphs[j] = a.glyphs[j], a.glyphs[i]
	a.indirections[i], a.indirections[j] = a.indirections[j], a.indirections[i]
}
func (a glyphsIndirections) Less(i, j int) bool { return a.glyphs[i] < a.glyphs[j] }

func arabicFallbackSynthesizeLookupLigature(ft *harfbuzz.Font, ligatureTable []arabicTableEntry, lookupFlags uint16) *harfbuzz.SyntheticGSUBLookup {
	var (
		firstGlyphs            []tables.GlyphID
		firstGlyphsIndirection []int // original index into ligature table
	)

	for firstGlyphIdx, lig := range ligatureTable {
		firstGlyph, ok := ft.NominalGlyph(lig.First)
		if !ok {
			continue
		}
		firstGlyphs = append(firstGlyphs, tables.GlyphID(firstGlyph))
		firstGlyphsIndirection = append(firstGlyphsIndirection, firstGlyphIdx)
	}

	if len(firstGlyphs) == 0 {
		return nil
	}

	sort.Stable(glyphsIndirections{glyphs: firstGlyphs, indirections: firstGlyphsIndirection})

	var out tables.LigatureSubs
	out.Coverage = tables.Coverage1{Glyphs: firstGlyphs}

	for _, firstGlyphIdx := range firstGlyphsIndirection {
		ligs := ligatureTable[firstGlyphIdx].Ligatures
		var ligatureSet tables.LigatureSet
		for _, v := range ligs {
			ligatureU := v.ligature
			ligatureGlyph, hasLigature := ft.NominalGlyph(ligatureU)
			if !hasLigature {
				continue
			}

			components := v.components
			var componentGIDs []tables.GlyphID
			for _, componentU := range components {
				componentGlyph, hasComponent := ft.NominalGlyph(componentU)
				if !hasComponent {
					break
				}
				componentGIDs = append(componentGIDs, tables.GlyphID(componentGlyph))
			}

			if len(components) != len(componentGIDs) {
				continue
			}

			ligatureSet.Ligatures = append(ligatureSet.Ligatures, tables.Ligature{
				LigatureGlyph:     tables.GlyphID(ligatureGlyph),
				ComponentGlyphIDs: componentGIDs,
			})
		}
		out.LigatureSets = append(out.LigatureSets, ligatureSet)
	}

	return &harfbuzz.SyntheticGSUBLookup{
		LookupFlags: lookupFlags,
		Subtables: []tables.GSUBLookup{
			out,
		},
	}
}

func arabicFallbackSynthesizeLookup(font *harfbuzz.Font, featureIndex int) *harfbuzz.SyntheticGSUBLookup {
	switch featureIndex {
	case 0, 1, 2, 3:
		return arabicFallbackSynthesizeLookupSingle(font, featureIndex)
	case 4:
		return arabicFallbackSynthesizeLookupLigature(font, arabicLigature3Table[:], lookupFlagIgnoreMarks)
	case 5:
		return arabicFallbackSynthesizeLookupLigature(font, arabicLigatureTable[:], lookupFlagIgnoreMarks)
	case 6:
		return arabicFallbackSynthesizeLookupLigature(font, arabicLigatureMarkTable[:], 0)
	default:
		panic("unexpected arabic fallback feature index")
	}
}

func newArabicFallbackProgram(featureMasks [arabicFallbackMaxLookups]harfbuzz.GlyphMask, font *harfbuzz.Font) *harfbuzz.SyntheticGSUBProgram {
	specs := make([]harfbuzz.SyntheticGSUBLookup, 0, arabicFallbackMaxLookups)
	for i := range arabicFallbackFeatures {
		mask := featureMasks[i]
		if mask == 0 {
			continue
		}
		spec := arabicFallbackSynthesizeLookup(font, i)
		if spec == nil {
			continue
		}
		spec.Mask = mask
		specs = append(specs, *spec)
	}
	return harfbuzz.CompileSyntheticGSUBProgram(specs)
}
