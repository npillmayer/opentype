package othebrew

import (
	"testing"

	"github.com/go-text/typesetting/language"
	"github.com/npillmayer/opentype/harfbuzz"
)

func TestShaperMatchHebrew(t *testing.T) {
	var s Shaper

	if got := s.Match(harfbuzz.SelectionContext{Script: language.Hebrew}); got < 0 {
		t.Fatalf("expected Hebrew match, got %d", got)
	}
	if got := s.Match(harfbuzz.SelectionContext{Script: language.Arabic}); got >= 0 {
		t.Fatalf("expected Arabic non-match, got %d", got)
	}
}

func TestNewName(t *testing.T) {
	if got := New().Name(); got != "hebrew" {
		t.Fatalf("New().Name() = %q, want %q", got, "hebrew")
	}
}
