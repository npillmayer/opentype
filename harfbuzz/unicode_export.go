package harfbuzz

// UnicodeGeneralCategory returns the internal numeric general category code for a rune.
func UnicodeGeneralCategory(u rune) uint8 {
	return uint8(uni.generalCategory(u))
}
