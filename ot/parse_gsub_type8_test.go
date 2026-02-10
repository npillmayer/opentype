package ot

import (
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestParseGSubType8(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	fonts := []string{"Calibri", "GentiumPlus-R"}
	found := false
	for _, name := range fonts {
		otf := parseFont(t, name)
		table := getTable(otf, "GSUB", t)
		gsub := table.Self().AsGSub()
		if gsub == nil {
			t.Fatalf("cannot convert GSUB table for %s", name)
		}

		graph := gsub.LookupGraph()
		if graph == nil {
			continue
		}
		for i := 0; i < graph.Len(); i++ {
			lookup := graph.Lookup(i)
			if lookup == nil || lookup.Type != GSubLookupTypeReverseChaining {
				continue
			}
			found = true
			for j := 0; j < int(lookup.SubTableCount); j++ {
				node := lookup.Subtable(j)
				if node == nil {
					t.Fatalf("%s: lookup[%d] subtable[%d] missing", name, i, j)
				}
				if node.LookupType != GSubLookupTypeReverseChaining {
					t.Errorf("%s: lookup[%d] subtable[%d] type = %d", name, i, j, node.LookupType)
				}
				payload := node.GSubPayload()
				if payload == nil || payload.ReverseChainingFmt1 == nil {
					t.Fatalf("%s: lookup[%d] subtable[%d] missing reverse chaining payload", name, i, j)
				}
				if len(payload.ReverseChainingFmt1.SubstituteGlyphIDs) == 0 {
					t.Fatalf("%s: lookup[%d] subtable[%d] reverse chaining payload has no substitutes", name, i, j)
				}
			}
		}
	}
	if !found {
		t.Skip("no GSUB type 8 lookups found in testdata fonts")
	}
}
