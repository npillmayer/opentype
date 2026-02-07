package harfbuzz

import (
	"sync"

	"github.com/go-text/typesetting/font/opentype/tables"
)

var (
	otLanguageIndexOnce sync.Once
	otLanguageIndex     map[string][]tables.Tag
)

func initOTLanguageIndex() {
	otLanguageIndex = make(map[string][]tables.Tag, len(otLanguages))
	for _, entry := range otLanguages {
		if entry.tag == 0 {
			continue
		}
		otLanguageIndex[entry.language] = append(otLanguageIndex[entry.language], entry.tag)
	}
}

func otLanguageTagsForPrimary(primary string) []tables.Tag {
	otLanguageIndexOnce.Do(initOTLanguageIndex)
	tags := otLanguageIndex[primary]
	if len(tags) == 0 {
		return nil
	}
	out := make([]tables.Tag, len(tags))
	copy(out, tags)
	return out
}
