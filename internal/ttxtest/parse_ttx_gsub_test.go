package ttxtest

import (
	"path/filepath"
	"testing"
)

func TestParseTTXGSUB_AlternateSubstFmt1(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "fonttools-tests", "gsub3_1_simple_f1.ttx.GSUB")
	exp, err := ParseTTXGSUB(path)
	if err != nil {
		t.Fatalf("ParseTTXGSUB: %v", err)
	}
	if len(exp.Lookups) != 1 {
		t.Fatalf("expected 1 lookup, got %d", len(exp.Lookups))
	}
	lk := exp.Lookups[0]
	if lk.Type != 3 {
		t.Fatalf("expected lookup type 3, got %d", lk.Type)
	}
	if lk.Flag != 0 {
		t.Fatalf("expected lookup flag 0, got %d", lk.Flag)
	}
	if len(lk.Subtables) != 1 {
		t.Fatalf("expected 1 subtable, got %d", len(lk.Subtables))
	}
	st := lk.Subtables[0]
	if st.Format != 1 {
		t.Fatalf("expected format 1, got %d", st.Format)
	}
	if len(st.Coverage) != 1 || st.Coverage[0] != "g18" {
		t.Fatalf("unexpected coverage: %#v", st.Coverage)
	}
	alts := st.Alternates["g18"]
	if len(alts) != 3 || alts[0] != "g20" || alts[1] != "g21" || alts[2] != "g22" {
		t.Fatalf("unexpected alternates for g18: %#v", alts)
	}
}
