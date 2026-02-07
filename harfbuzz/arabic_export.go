package harfbuzz

// UnicodeGeneralCategory returns the internal numeric general category code for a rune.
func UnicodeGeneralCategory(u rune) uint8 {
	return uint8(uni.generalCategory(u))
}

// ArabicJoiningType returns Arabic joining behavior class for a rune/category pair.
func ArabicJoiningType(u rune, genCat uint8) uint8 {
	return getJoiningType(u, generalCategory(genCat))
}

// ArabicIsWord reports whether a general category is considered a "word" item
// by Arabic stretch heuristics.
func ArabicIsWord(genCat uint8) bool {
	return isWord(generalCategory(genCat))
}

// ArabicFallbackPlan is a public wrapper over the internal Arabic fallback GSUB plan.
type ArabicFallbackPlan struct {
	impl *arabicFallbackPlan
}

// NewArabicFallbackPlan creates a fallback GSUB plan from per-feature masks.
// Extra entries are ignored, and missing entries default to zero masks.
func NewArabicFallbackPlan(featureMasks []GlyphMask, font *Font) *ArabicFallbackPlan {
	var normalized [arabicFallbackMaxLookups]GlyphMask
	copy(normalized[:], featureMasks)
	return &ArabicFallbackPlan{impl: newArabicFallbackPlan(normalized, font)}
}

// Shape applies fallback Arabic substitutions on the current buffer.
func (p *ArabicFallbackPlan) Shape(font *Font, buffer *Buffer) {
	if p == nil || p.impl == nil {
		return
	}
	p.impl.shape(font, buffer)
}
