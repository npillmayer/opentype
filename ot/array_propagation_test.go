package ot

import (
	"errors"
	"strings"
	"testing"
)

func TestGraphRetainsArrayAccessError(t *testing.T) {
	// The first record and its target are valid. The array claims a second
	// record but its backing view ends after the first, exercising Get itself.
	raw := make(binarySegm, 16)
	copy(raw, "latn")
	writeU16(raw, 4, 12)
	records := array{name: "test records", recordSize: 6, length: 2, loc: raw[:6]}
	scripts := parseConcreteScriptList(raw, records, nil)
	if !errors.Is(scripts.Error(), errBufferBounds) || !strings.Contains(scripts.Error().Error(), "index 1") {
		t.Fatalf("script error = %v; want contextual bounds error", scripts.Error())
	}
	if scripts.Script(T("latn")) == nil || scripts.Script(0) != nil {
		t.Fatal("valid script must survive without inventing a script for the failed record")
	}

	copy(raw, "liga")
	features := parseConcreteFeatureList(raw, records)
	if !errors.Is(features.Error(), errBufferBounds) || !strings.Contains(features.Error().Error(), "index 1") {
		t.Fatalf("feature error = %v; want contextual bounds error", features.Error())
	}
	if features.Len() != 2 || features.featureAtIndex(0) == nil || features.featureAtIndex(0).Error() != nil {
		t.Fatal("feature indices and the valid feature must be preserved")
	}
	failed := features.featureAtIndex(1)
	if failed == nil || !errors.Is(failed.Error(), errBufferBounds) || failed.LookupCount() != 0 {
		t.Fatal("failed record must resolve to an error-bearing feature without lookup references")
	}
	if len(features.Indices(0)) != 0 {
		t.Fatal("failed record must not register a fabricated tag")
	}
}

func TestGraphPreservesFirstError(t *testing.T) {
	raw := binarySegm{'l', 'i', 'g', 'a', 0, 0}
	records := array{name: "test records", recordSize: 6, length: 2, loc: raw}
	features := parseConcreteFeatureList(raw, records)
	if features.Error() == nil || !strings.Contains(features.Error().Error(), "invalid offset 0") {
		t.Fatalf("first error was lost: %v", features.Error())
	}
	for i := 0; i < 2; i++ {
		failed := features.featureAtIndex(i)
		if failed == nil || failed.Error() == nil || failed.LookupCount() != 0 {
			t.Fatal("invalid records must resolve to error-bearing features without lookup references")
		}
	}
}

func TestRuleSetErrorsDiscardPartialResults(t *testing.T) {
	// One valid empty-action rule, followed by an out-of-bounds rule offset.
	sequence := binarySegm{0, 2, 0, 6, 0, 255, 0, 1, 0, 0}
	chained := binarySegm{0, 2, 0, 6, 0, 255, 0, 0, 0, 1, 0, 0, 0, 0}
	if rules, err := parseGSubSequenceRuleSet(sequence); rules != nil || !errors.Is(err, errBufferBounds) {
		t.Fatalf("sequence rules = %v, %v; want nil and bounds error", rules, err)
	}
	if rules, err := parseGSubClassSequenceRuleSet(sequence); rules != nil || !errors.Is(err, errBufferBounds) {
		t.Fatalf("class rules = %v, %v; want nil and bounds error", rules, err)
	}
	if rules, err := parseGSubChainedSequenceRuleSet(chained); rules != nil || !errors.Is(err, errBufferBounds) {
		t.Fatalf("chained rules = %v, %v; want nil and bounds error", rules, err)
	}
	if rules, err := parseGSubChainedClassRuleSet(chained); rules != nil || !errors.Is(err, errBufferBounds) {
		t.Fatalf("chained class rules = %v, %v; want nil and bounds error", rules, err)
	}
}

func TestLazyLookupRetainsMalformedSubtableError(t *testing.T) {
	// LookupList -> GSUB single-substitution format 2, with a truncated array.
	raw := binarySegm{
		0, 1, 0, 4,
		0, 1, 0, 0, 0, 1, 0, 8,
		0, 2, 0, 0, 0, 2, 0, 7,
	}
	graph := parseConcreteLookupListGraph(raw, false)
	lookup := graph.Lookup(0)
	if lookup == nil {
		t.Fatal("missing lookup")
	}
	node := lookup.Subtable(0)
	if node == nil || node.Error() == nil {
		t.Fatal("malformed subtable must retain an error")
	}
	if node.GSub.SingleFmt2.SubstituteGlyphIDs != nil {
		t.Fatal("truncated substitution array must not expose fabricated glyphs")
	}
	if lookup.Subtable(0) != node {
		t.Fatal("repeated lazy access must retain the same node and error")
	}
}
