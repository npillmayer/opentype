package harfbuzz

import (
	"testing"

	"github.com/go-text/typesetting/language"
)

type testRegistryShaper struct {
	complexShaperDefault
	name  string
	score int
}

func (s testRegistryShaper) Name() string { return s.name }
func (s testRegistryShaper) Match(SelectionContext) int {
	return s.score
}
func (s testRegistryShaper) New() ShapingEngine { return s }

func TestShaperRegistryResolveTieBreakByName(t *testing.T) {
	reg := newShaperRegistry()
	if err := reg.registerShaper(testRegistryShaper{name: "zzz", score: 7}); err != nil {
		t.Fatal(err)
	}
	if err := reg.registerShaper(testRegistryShaper{name: "aaa", score: 7}); err != nil {
		t.Fatal(err)
	}

	got := reg.resolve(SelectionContext{})
	if got == nil {
		t.Fatal("resolve returned nil")
	}
	if got.Name() != "aaa" {
		t.Fatalf("expected tie-break to pick aaa, got %q", got.Name())
	}
}

func TestShaperRegistryNoMatchFallsBackToDefault(t *testing.T) {
	reg := newShaperRegistry()
	if err := reg.registerShaper(testRegistryShaper{name: "never", score: -1}); err != nil {
		t.Fatal(err)
	}

	got := reg.resolve(SelectionContext{})
	if got == nil {
		t.Fatal("resolve returned nil")
	}
	if got.Name() != "default" {
		t.Fatalf("expected default fallback, got %q", got.Name())
	}
}

func TestShaperRegistryClearAndReregister(t *testing.T) {
	reg := newShaperRegistry()
	if err := reg.registerShaper(testRegistryShaper{name: "first", score: 1}); err != nil {
		t.Fatal(err)
	}
	if got := reg.resolve(SelectionContext{}); got.Name() != "first" {
		t.Fatalf("expected first before clear, got %q", got.Name())
	}

	reg.clear()

	if err := reg.registerShaper(testRegistryShaper{name: "first", score: 2}); err != nil {
		t.Fatal(err)
	}
	if got := reg.resolve(SelectionContext{}); got.Name() != "first" {
		t.Fatalf("expected first after clear+reregister, got %q", got.Name())
	}
}

func TestShaperRegistryBuiltinsDefaultOnly(t *testing.T) {
	reg := newShaperRegistry()
	for _, shaper := range builtInShapers() {
		if err := reg.registerShaper(shaper); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name string
		ctx  SelectionContext
		want string
	}{
		{
			name: "arabic falls back to default",
			ctx: SelectionContext{
				Script:    language.Arabic,
				Direction: RightToLeft,
			},
			want: "default",
		},
		{
			name: "hebrew falls back to default",
			ctx: SelectionContext{
				Script:    language.Hebrew,
				Direction: RightToLeft,
			},
			want: "default",
		},
		{
			name: "latin",
			ctx: SelectionContext{
				Script:    language.Latin,
				Direction: LeftToRight,
			},
			want: "default",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := reg.resolve(tc.ctx)
			if got == nil {
				t.Fatal("resolve returned nil")
			}
			if got.Name() != tc.want {
				t.Fatalf("resolve(%s): got %q want %q", tc.name, got.Name(), tc.want)
			}
		})
	}
}
