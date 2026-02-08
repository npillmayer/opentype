package harfbuzz

import (
	"testing"

	"github.com/go-text/typesetting/font/opentype/tables"
)

func TestCompileSyntheticGSUBProgramFiltersInvalidSpecs(t *testing.T) {
	specs := []SyntheticGSUBLookup{
		{
			Mask:      0,
			Subtables: []tables.GSUBLookup{tables.SingleSubs{}},
		},
		{
			Mask:      1,
			Subtables: nil,
		},
		{
			Mask:      1,
			Subtables: []tables.GSUBLookup{tables.SingleSubs{}},
		},
	}

	program := CompileSyntheticGSUBProgram(specs)
	if program.Empty() {
		t.Fatal("compiled program should not be empty")
	}
	if got, want := program.NumLookups(), 1; got != want {
		t.Fatalf("NumLookups() = %d, want %d", got, want)
	}
}

func TestCompileSyntheticGSUBProgramEmpty(t *testing.T) {
	program := CompileSyntheticGSUBProgram(nil)
	if !program.Empty() {
		t.Fatal("nil specs should compile to empty program")
	}
	if got := program.NumLookups(); got != 0 {
		t.Fatalf("NumLookups() = %d, want 0", got)
	}
}
