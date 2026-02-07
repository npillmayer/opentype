package harfbuzz

import (
	"strings"

	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
	xlanguage "golang.org/x/text/language"
)

// ported from harfbuzz/src/hb-ot-tag.cc Copyright © 2009  Red Hat, Inc. 2011  Google, Inc. Behdad Esfahbod, Roozbeh Pournader

var (
	// OpenType script tag, `DFLT`, for features that are not script-specific.
	tagDefaultScript = ot.NewTag('D', 'F', 'L', 'T')
	// OpenType language tag, `dflt`. Not a valid language tag, but some fonts
	// mistakenly use it.
	tagDefaultLanguage = ot.NewTag('d', 'f', 'l', 't')
)

func oldTagFromScript(script language.Script) tables.Tag {
	/* This seems to be accurate as of end of 2012. */

	switch script {
	case 0:
		return tagDefaultScript
	case language.Mathematical_notation:
		return ot.NewTag('m', 'a', 't', 'h')

	/* KATAKANA and HIRAGANA both map to 'kana' */
	case language.Hiragana:
		return ot.NewTag('k', 'a', 'n', 'a')

	/* Spaces at the end are preserved, unlike ISO 15924 */
	case language.Lao:
		return ot.NewTag('l', 'a', 'o', ' ')
	case language.Yi:
		return ot.NewTag('y', 'i', ' ', ' ')
	/* Unicode-5.0 additions */
	case language.Nko:
		return ot.NewTag('n', 'k', 'o', ' ')
	/* Unicode-5.1 additions */
	case language.Vai:
		return ot.NewTag('v', 'a', 'i', ' ')
	}

	/* Else, just change first char to lowercase and return */
	return tables.Tag(script | 0x20000000)
}

func newTagFromScript(script language.Script) tables.Tag {
	switch script {
	case language.Bengali:
		return ot.NewTag('b', 'n', 'g', '2')
	case language.Devanagari:
		return ot.NewTag('d', 'e', 'v', '2')
	case language.Gujarati:
		return ot.NewTag('g', 'j', 'r', '2')
	case language.Gurmukhi:
		return ot.NewTag('g', 'u', 'r', '2')
	case language.Kannada:
		return ot.NewTag('k', 'n', 'd', '2')
	case language.Malayalam:
		return ot.NewTag('m', 'l', 'm', '2')
	case language.Oriya:
		return ot.NewTag('o', 'r', 'y', '2')
	case language.Tamil:
		return ot.NewTag('t', 'm', 'l', '2')
	case language.Telugu:
		return ot.NewTag('t', 'e', 'l', '2')
	case language.Myanmar:
		return ot.NewTag('m', 'y', 'm', '2')
	}

	return tagDefaultScript
}

// Complete list at:
// https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags
//
// Most of the script tags are the same as the ISO 15924 tag but lowercased.
// So we just do that, and handle the exceptional cases in a switch.
func allTagsFromScript(script language.Script) []tables.Tag {
	var tags []tables.Tag

	tag := newTagFromScript(script)
	if tag != tagDefaultScript {
		// HB_SCRIPT_MYANMAR maps to 'mym2', but there is no 'mym3'.
		if tag != ot.NewTag('m', 'y', 'm', '2') {
			tags = append(tags, tag|'3')
		}
		tags = append(tags, tag)
	}

	oldTag := oldTagFromScript(script)
	if oldTag != tagDefaultScript {
		tags = append(tags, oldTag)
	}
	return tags
}

func parseLanguageTagStrict(langStr string) (xlanguage.Tag, bool) {
	if langStr == "" {
		return xlanguage.Tag{}, false
	}
	tag, err := xlanguage.Parse(langStr)
	if err != nil {
		return xlanguage.Tag{}, false
	}
	return tag, true
}

func primarySubtag(langStr string) (string, bool) {
	tag, ok := parseLanguageTagStrict(langStr)
	if !ok {
		return "", false
	}

	base, _ := tag.Base()
	primary := strings.ToLower(base.String())
	if primary == "" {
		return "", false
	}
	return primary, true
}

func isISO639_3(tag string) bool {
	if len(tag) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if !isAlpha(tag[i]) {
			return false
		}
	}
	return true
}

func otTagsFromLanguage(langStr string) []tables.Tag {
	primary, ok := primarySubtag(langStr)
	if !ok {
		return nil
	}

	if tags := otLanguageTagsForPrimary(primary); len(tags) != 0 {
		return tags
	}

	if isISO639_3(primary) {
		// assume it's ISO-639-3 and upper-case and use it.
		return []tables.Tag{ot.NewTag(toUpper(primary[0]), toUpper(primary[1]), toUpper(primary[2]), ' ')}
	}

	return nil
}

// return 0 if no tag
func parsePrivateUseSubtag(privateUseSubtag string, prefix string, normalize func(byte) byte) (tables.Tag, bool) {
	s := strings.Index(privateUseSubtag, prefix)
	if s == -1 {
		return 0, false
	}

	var tag [4]byte
	L := len(privateUseSubtag)
	s += len(prefix)
	var i int
	for ; i < 4 && s+i < L && isAlnum(privateUseSubtag[s+i]); i++ {
		tag[i] = normalize(privateUseSubtag[s+i])
	}
	if i == 0 {
		return 0, false
	}

	for ; i < 4; i++ {
		tag[i] = ' '
	}
	out := ot.NewTag(tag[0], tag[1], tag[2], tag[3])
	if (out & 0xDFDFDFDF) == tagDefaultScript {
		out ^= ^tables.Tag(0xDFDFDFDF)
	}
	return out, true
}

func privateUseExtension(tag xlanguage.Tag) string {
	for _, ext := range tag.Extensions() {
		if ext.Type() == 'x' {
			return ext.String()
		}
	}
	return ""
}

// newOTTagsFromScriptAndLanguage converts a `Script` and a `Language`
// to script and language tags.
func newOTTagsFromScriptAndLanguage(script language.Script, lang language.Language) (scriptTags, languageTags []tables.Tag) {
	if parsed, ok := parseLanguageTagStrict(string(lang)); ok {
		privateUseSubtag := privateUseExtension(parsed)

		s, hasScript := parsePrivateUseSubtag(privateUseSubtag, "-hbsc", toLower)
		if hasScript {
			scriptTags = []tables.Tag{s}
		}

		l, hasLanguage := parsePrivateUseSubtag(privateUseSubtag, "-hbot", toUpper)
		if hasLanguage {
			languageTags = append(languageTags, l)
		} else {
			languageTags = otTagsFromLanguage(parsed.String())
		}
	}

	if len(scriptTags) == 0 {
		scriptTags = allTagsFromScript(script)
	}
	return
}
