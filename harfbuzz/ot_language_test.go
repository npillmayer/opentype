package harfbuzz

import (
	"strings"
	"testing"
)

func TestOTLanguageIndexCoverage(t *testing.T) {
	for _, l := range otLanguages {
		if l.tag == 0 {
			continue
		}
		tags := otLanguageTagsForPrimary(l.language)
		if len(tags) == 0 {
			t.Fatalf("can't find mapped tags for language %q", l.language)
		}

		found := false
		for _, tag := range tags {
			if tag == l.tag {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("tag %s for language %q missing from index entry", l.tag, l.language)
		}
	}
}

func TestOTLanguageIndexUnknown(t *testing.T) {
	if tags := otLanguageTagsForPrimary(strings.ToLower("zzzz")); len(tags) != 0 {
		t.Fatalf("expected no tags for unknown language, got %v", tags)
	}
}
