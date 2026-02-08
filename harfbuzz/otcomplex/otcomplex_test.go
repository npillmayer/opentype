package otcomplex

import "testing"

func TestCompatibilityConstructors(t *testing.T) {
	if got := NewHebrew().Name(); got != "hebrew" {
		t.Fatalf("NewHebrew().Name() = %q, want %q", got, "hebrew")
	}
	if got := NewArabic().Name(); got != "arabic" {
		t.Fatalf("NewArabic().Name() = %q, want %q", got, "arabic")
	}
}
