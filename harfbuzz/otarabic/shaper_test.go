package otarabic

import (
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/npillmayer/opentype/harfbuzz"
)

func TestShaperMatchArabicAndSyriac(t *testing.T) {
	var s Shaper

	if got := s.Match(harfbuzz.SelectionContext{Script: language.Arabic, Direction: harfbuzz.RightToLeft}); got < 0 {
		t.Fatalf("expected Arabic match, got %d", got)
	}

	syriacCtx := harfbuzz.SelectionContext{
		Script:       language.Syriac,
		Direction:    harfbuzz.RightToLeft,
		ChosenScript: [2]ot.Tag{ot.NewTag('s', 'y', 'r', 'c'), 0},
	}
	if got := s.Match(syriacCtx); got < 0 {
		t.Fatalf("expected Syriac non-DFLT match, got %d", got)
	}

	syriacDefaultCtx := harfbuzz.SelectionContext{
		Script:       language.Syriac,
		Direction:    harfbuzz.RightToLeft,
		ChosenScript: [2]ot.Tag{ot.NewTag('D', 'F', 'L', 'T'), 0},
	}
	if got := s.Match(syriacDefaultCtx); got >= 0 {
		t.Fatalf("expected Syriac DFLT non-match, got %d", got)
	}
}

func TestNewName(t *testing.T) {
	if got := New().Name(); got != "arabic" {
		t.Fatalf("New().Name() = %q, want %q", got, "arabic")
	}
}
