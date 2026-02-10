package otquery

// nameKey identifies a NameRecord entry in OpenType table 'name'.
// The key follows the OpenType NameRecord fields directly.
type nameKey struct {
	PlatformID uint16
	EncodingID uint16
	LanguageID uint16
	NameID     uint16
}
