package harfbuzz

import (
	"sort"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// Features ordered the same as the entries in [arabicShaping] rows,
// followed by rlig. Don't change.
var arabicFallbackFeatures = [...]ot.Tag{
	ot.NewTag('i', 'n', 'i', 't'),
	ot.NewTag('m', 'e', 'd', 'i'),
	ot.NewTag('f', 'i', 'n', 'a'),
	ot.NewTag('i', 's', 'o', 'l'),
	ot.NewTag('r', 'l', 'i', 'g'),
	ot.NewTag('r', 'l', 'i', 'g'),
	ot.NewTag('r', 'l', 'i', 'g'),
}

// used to sort both array at the same time
type jointGlyphs struct {
	glyphs, substitutes []gID
}

func (a jointGlyphs) Len() int { return len(a.glyphs) }
func (a jointGlyphs) Swap(i, j int) {
	a.glyphs[i], a.glyphs[j] = a.glyphs[j], a.glyphs[i]
	a.substitutes[i], a.substitutes[j] = a.substitutes[j], a.substitutes[i]
}
func (a jointGlyphs) Less(i, j int) bool { return a.glyphs[i] < a.glyphs[j] }

func arabicFallbackSynthesizeLookupSingle(ft *Font, featureIndex int) *lookupGSUB {
	var glyphs, substitutes []gID

	for u := rune(firstArabicShape); u <= lastArabicShape; u++ {
		s := rune(arabicShaping[u-firstArabicShape][featureIndex])
		uGlyph, hasU := ft.face.NominalGlyph(u)
		sGlyph, hasS := ft.face.NominalGlyph(s)

		if s == 0 || !hasU || !hasS || uGlyph == sGlyph || uGlyph > 0xFFFF || sGlyph > 0xFFFF {
			continue
		}

		glyphs = append(glyphs, gID(uGlyph))
		substitutes = append(substitutes, gID(sGlyph))
	}

	if len(glyphs) == 0 {
		return nil
	}

	sort.Stable(jointGlyphs{glyphs: glyphs, substitutes: substitutes})

	return &lookupGSUB{
		LookupOptions: font.LookupOptions{Flag: otIgnoreMarks},
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
	glyphs       []gID
	indirections []int
}

func (a glyphsIndirections) Len() int { return len(a.glyphs) }
func (a glyphsIndirections) Swap(i, j int) {
	a.glyphs[i], a.glyphs[j] = a.glyphs[j], a.glyphs[i]
	a.indirections[i], a.indirections[j] = a.indirections[j], a.indirections[i]
}
func (a glyphsIndirections) Less(i, j int) bool { return a.glyphs[i] < a.glyphs[j] }

func arabicFallbackSynthesizeLookupLigature(ft *Font, ligatureTable []arabicTableEntry, lookupFlags uint16) *lookupGSUB {
	var (
		firstGlyphs            []gID
		firstGlyphsIndirection []int // original index into ligature table
	)

	for firstGlyphIdx, lig := range ligatureTable {
		firstGlyph, ok := ft.face.NominalGlyph(lig.First)
		if !ok {
			continue
		}
		firstGlyphs = append(firstGlyphs, gID(firstGlyph))
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
			ligatureGlyph, hasLigature := ft.face.NominalGlyph(ligatureU)
			if !hasLigature {
				continue
			}

			components := v.components
			var componentGIDs []gID
			for _, componentU := range components {
				componentGlyph, hasComponent := ft.face.NominalGlyph(componentU)
				if !hasComponent {
					break
				}
				componentGIDs = append(componentGIDs, gID(componentGlyph))
			}

			if len(components) != len(componentGIDs) {
				continue
			}

			ligatureSet.Ligatures = append(ligatureSet.Ligatures, tables.Ligature{
				LigatureGlyph:     gID(ligatureGlyph),
				ComponentGlyphIDs: componentGIDs,
			})
		}
		out.LigatureSets = append(out.LigatureSets, ligatureSet)
	}

	return &lookupGSUB{
		LookupOptions: font.LookupOptions{Flag: lookupFlags},
		Subtables: []tables.GSUBLookup{
			out,
		},
	}
}

func arabicFallbackSynthesizeLookup(font *Font, featureIndex int) *lookupGSUB {
	switch featureIndex {
	case 0, 1, 2, 3:
		return arabicFallbackSynthesizeLookupSingle(font, featureIndex)
	case 4:
		return arabicFallbackSynthesizeLookupLigature(font, arabicLigature3Table[:], otIgnoreMarks)
	case 5:
		return arabicFallbackSynthesizeLookupLigature(font, arabicLigatureTable[:], otIgnoreMarks)
	case 6:
		return arabicFallbackSynthesizeLookupLigature(font, arabicLigatureMarkTable[:], 0)
	default:
		panic("unexpected arabic fallback feature index")
	}
}

const arabicFallbackMaxLookups = len(arabicFallbackFeatures)

type arabicFallbackPlan struct {
	accelArray [arabicFallbackMaxLookups]otLayoutLookupAccelerator
	numLookups int
	maskArray  [arabicFallbackMaxLookups]GlyphMask
}

func (fbPlan *arabicFallbackPlan) initUnicode(featureMasks [arabicFallbackMaxLookups]GlyphMask, font *Font) bool {
	var j int
	for i := range arabicFallbackFeatures {
		mask := featureMasks[i]
		if mask != 0 {
			lk := arabicFallbackSynthesizeLookup(font, i)
			if lk != nil {
				fbPlan.maskArray[j] = mask
				fbPlan.accelArray[j].init(*lk)
				j++
			}
		}
	}

	fbPlan.numLookups = j
	return j > 0
}

func newArabicFallbackPlan(featureMasks [arabicFallbackMaxLookups]GlyphMask, font *Font) *arabicFallbackPlan {
	var fbPlan arabicFallbackPlan

	// Try synthesizing GSUB table using Unicode Arabic Presentation Forms.
	if fbPlan.initUnicode(featureMasks, font) {
		return &fbPlan
	}

	return &arabicFallbackPlan{}
}

func (fbPlan *arabicFallbackPlan) shape(font *Font, buffer *Buffer) {
	var c otApplyContext
	c.reset(0, font, buffer)
	for i := 0; i < fbPlan.numLookups; i++ {
		if fbPlan.accelArray[i].lookup != nil {
			c.setLookupMask(fbPlan.maskArray[i])
			c.substituteLookup(&fbPlan.accelArray[i])
		}
	}
}
