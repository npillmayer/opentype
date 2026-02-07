package harfbuzz

import (
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
)

// ported from harfbuzz/test/api/test-ot-tag.c Copyright © 2011  Google, Inc. Behdad Esfahbod

func assertEqualTag(t *testing.T, t1, t2 tables.Tag) {
	t.Helper()

	if t1 != t2 {
		t.Fatalf("unexpected %s != %s", t1, t2)
	}
}

/* https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags */

func testSimpleTags(t *testing.T, s string, script language.Script) {
	tag := ot.MustNewTag(s)

	tags, _ := newOTTagsFromScriptAndLanguage(script, "")

	if len(tags) != 0 {
		assertEqualTag(t, tags[0], tag)
	} else {
		assertEqualTag(t, ot.MustNewTag("DFLT"), tag)
	}
}

func testScriptTagsFromLanguage(t *testing.T, s, langS string, script language.Script) {
	var tag tables.Tag
	if s != "" {
		tag = ot.MustNewTag(s)
	}

	tags, _ := newOTTagsFromScriptAndLanguage(script, language.NewLanguage(langS))
	if len(tags) != 0 {
		assertEqualInt(t, len(tags), 1)
		assertEqualTag(t, tags[0], tag)
	}
}

func testIndicTags(t *testing.T, s1, s2, s3 string, script language.Script) {
	tag1 := ot.MustNewTag(s1)
	tag2 := ot.MustNewTag(s2)
	tag3 := ot.MustNewTag(s3)

	tags, _ := newOTTagsFromScriptAndLanguage(script, "")

	assertEqualInt(t, len(tags), 3)
	assertEqualTag(t, tags[0], tag1)
	assertEqualTag(t, tags[1], tag2)
	assertEqualTag(t, tags[2], tag3)
}

func TestOtTagScriptDegenerate(t *testing.T) {
	assertEqualTag(t, ot.MustNewTag("DFLT"), tagDefaultScript)

	/* HIRAGANA and KATAKANA both map to 'kana' */
	testSimpleTags(t, "kana", language.Katakana)

	tags, _ := newOTTagsFromScriptAndLanguage(language.Hiragana, "")

	assertEqualInt(t, len(tags), 1)
	assertEqualTag(t, tags[0], ot.MustNewTag("kana"))

	testSimpleTags(t, "DFLT", 0)

	/* Spaces are replaced */
	// assertEqualInt(t, hb_ot_tag_to_script(ot.MustNewTag("be  ")), hb_script_from_string("Beee", -1))
}

func TestOtTagScriptSimple(t *testing.T) {
	/* Arbitrary non-existent script */
	// testSimpleTags(t, "wwyz", hb_script_from_string("wWyZ", -1))

	/* These we don't really care about */
	testSimpleTags(t, "zyyy", language.Common)
	testSimpleTags(t, "zinh", language.Inherited)
	testSimpleTags(t, "zzzz", language.Unknown)

	testSimpleTags(t, "arab", language.Arabic)
	testSimpleTags(t, "copt", language.Coptic)
	testSimpleTags(t, "kana", language.Katakana)
	testSimpleTags(t, "latn", language.Latin)

	testSimpleTags(t, "math", language.Mathematical_notation)

	/* These are trickier since their OT script tags have space. */
	testSimpleTags(t, "lao ", language.Lao)
	testSimpleTags(t, "yi  ", language.Yi)
	/* Unicode-5.0 additions */
	testSimpleTags(t, "nko ", language.Nko)
	/* Unicode-5.1 additions */
	testSimpleTags(t, "vai ", language.Vai)

	/* https://docs.microsoft.com/en-us/typography/opentype/spec/scripttags */

	/* Unicode-5.2 additions */
	testSimpleTags(t, "mtei", language.Meetei_Mayek)
	/* Unicode-6.0 additions */
	testSimpleTags(t, "mand", language.Mandaic)
}

func TestOtTagScriptFromLanguage(t *testing.T) {
	testScriptTagsFromLanguage(t, "", "", 0)
	testScriptTagsFromLanguage(t, "", "en", 0)
	testScriptTagsFromLanguage(t, "copt", "en", language.Coptic)
	testScriptTagsFromLanguage(t, "", "x-hbsc", 0)
	testScriptTagsFromLanguage(t, "copt", "x-hbsc", language.Coptic)
	testScriptTagsFromLanguage(t, "", "x-hbsc-", 0)
	testScriptTagsFromLanguage(t, "", "x-hbsc-1", 0)
	testScriptTagsFromLanguage(t, "", "x-hbsc-1a", 0)
	testScriptTagsFromLanguage(t, "", "x-hbsc-1a2b3c4x", 0)
	testScriptTagsFromLanguage(t, "2lon", "x-hbsc2lon", 0)
	testScriptTagsFromLanguage(t, "abc ", "x-hbscabc", 0)
	testScriptTagsFromLanguage(t, "deva", "x-hbscdeva", 0)
	testScriptTagsFromLanguage(t, "dev2", "x-hbscdev2", 0)
	testScriptTagsFromLanguage(t, "dev3", "x-hbscdev3", 0)
	testScriptTagsFromLanguage(t, "dev3", "x-hbscdev3", 0)
	testScriptTagsFromLanguage(t, "copt", "x-hbotpap0-hbsccopt", 0)
	testScriptTagsFromLanguage(t, "", "en-x-hbsc", 0)
	testScriptTagsFromLanguage(t, "copt", "en-x-hbsc", language.Coptic)
	testScriptTagsFromLanguage(t, "abc ", "en-x-hbscabc", 0)
	testScriptTagsFromLanguage(t, "deva", "en-x-hbscdeva", 0)
	testScriptTagsFromLanguage(t, "dev2", "en-x-hbscdev2", 0)
	testScriptTagsFromLanguage(t, "dev3", "en-x-hbscdev3", 0)
	testScriptTagsFromLanguage(t, "dev3", "en-x-hbscdev3", 0)
	testScriptTagsFromLanguage(t, "copt", "en-x-hbotpap0-hbsccopt", 0)
	testScriptTagsFromLanguage(t, "", "UTF-8", 0)

	// corner cases should not panic
	testScriptTagsFromLanguage(t, "", "x", 0)
}

func TestOtTagScriptIndic(t *testing.T) {
	testIndicTags(t, "bng3", "bng2", "beng", language.Bengali)
	testIndicTags(t, "dev3", "dev2", "deva", language.Devanagari)
	testIndicTags(t, "gjr3", "gjr2", "gujr", language.Gujarati)
	testIndicTags(t, "gur3", "gur2", "guru", language.Gurmukhi)
	testIndicTags(t, "knd3", "knd2", "knda", language.Kannada)
	testIndicTags(t, "mlm3", "mlm2", "mlym", language.Malayalam)
	testIndicTags(t, "ory3", "ory2", "orya", language.Oriya)
	testIndicTags(t, "tml3", "tml2", "taml", language.Tamil)
	testIndicTags(t, "tel3", "tel2", "telu", language.Telugu)
}

/* https://docs.microsoft.com/en-us/typography/opentype/spec/languagetags */

func testTagFromLanguage(t *testing.T, tagS, langS string) {
	t.Helper()

	lang := language.NewLanguage(langS)
	tag := ot.MustNewTag(tagS)

	_, tags := newOTTagsFromScriptAndLanguage(0, lang)

	if len(tags) != 0 {
		assertEqualTag(t, tag, tags[0])
	} else {
		assertEqualTag(t, tag, ot.MustNewTag("dflt"))
	}
}

func testTags(t *testing.T, script language.Script, langS string, expectedScriptCount, expectedLanguageCount int, expected ...string) {
	lang := language.NewLanguage(langS)

	scriptTags, languageTags := newOTTagsFromScriptAndLanguage(script, lang)

	assertEqualInt(t, len(scriptTags), expectedScriptCount)
	assertEqualInt(t, len(languageTags), expectedLanguageCount)

	for i, s := range expected {
		expectedTag := ot.MustNewTag(s)
		var actualTag tables.Tag
		if i < expectedScriptCount {
			actualTag = scriptTags[i]
		} else {
			actualTag = languageTags[i-expectedScriptCount]
		}
		assertEqualTag(t, actualTag, expectedTag)
	}
}

func TestOtTagLanguageStrict(t *testing.T) {
	assertEqualInt(t, int(ot.MustNewTag("dflt")), int(tagDefaultLanguage))

	testTagFromLanguage(t, "dflt", "")
	testTagFromLanguage(t, "ENG ", "en")
	testTagFromLanguage(t, "ENG ", "en_US")
	testTagFromLanguage(t, "ARA ", "ar")
	testTagFromLanguage(t, "dflt", "xyz")

	// x/hbot override should still be honored with strict parsing.
	testTagFromLanguage(t, "ABC ", "fa-x-hbotabc")
	testTagFromLanguage(t, "ABCD", "en-x-hbotABCD")

	// No complex-language fallback mapping in strict mode.
	testTagFromLanguage(t, "ENG ", "en-fonnapa")
	testTagFromLanguage(t, "ENG ", "und-fonnapa")
}

func TestOtTagFullStrict(t *testing.T) {
	testTags(t, 0, "en", 0, 1, "ENG ")
	testTags(t, 0, "en-x-hbscdflt", 1, 1, "DFLT", "ENG ")
	testTags(t, language.Latin, "en", 1, 1, "latn", "ENG ")

	// With strict mode we map by canonical base language only.
	testTags(t, 0, "en-fonnapa", 0, 1, "ENG ")
	testTags(t, 0, "und-fonnapa", 0, 1, "ENG ")

	testTags(t, 0, "x-hbot1234-hbsc5678", 1, 1, "5678", "1234")
	testTags(t, 0, "x-hbsc5678-hbot1234", 1, 1, "5678", "1234")
	testTags(t, 0, "xyz", 0, 0)
}

func TestOtTagFromLanguage(t *testing.T) {
	scs, _ := newOTTagsFromScriptAndLanguage(language.Tai_Tham, "")
	if len(scs) != 1 && scs[0] != 1818324577 {
		t.Fatalf("exected [lana], got %v", scs)
	}
}
