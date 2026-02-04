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

		ll := gsub.LookupList
		for i := 0; i < ll.Len(); i++ {
			lookup := ll.Navigate(i)
			if lookup.Type != GSubLookupTypeReverseChaining {
				continue
			}
			found = true
			for j := 0; j < int(lookup.SubTableCount); j++ {
				sub := lookup.Subtable(j)
				if sub.LookupType != GSubLookupTypeReverseChaining {
					t.Errorf("%s: lookup[%d] subtable[%d] type = %d", name, i, j, sub.LookupType)
				}
				if sub.Support == nil {
					t.Fatalf("%s: lookup[%d] subtable[%d] missing reverse chaining support", name, i, j)
				}
				if _, ok := sub.Support.(*ReverseChainingSubst); !ok {
					t.Fatalf("%s: lookup[%d] subtable[%d] Support is %T", name, i, j, sub.Support)
				}
			}
		}
	}
	if !found {
		t.Skip("no GSUB type 8 lookups found in testdata fonts")
	}
}
