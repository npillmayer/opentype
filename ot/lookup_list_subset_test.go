package ot

import (
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestLookupListSubset(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "font.opentype")
	defer teardown()

	offsets := []uint16{4, 10, 20, 30}
	buf := make([]byte, 2*len(offsets))
	for i, off := range offsets {
		writeU16(buf, i*2, off)
	}
	ll := LookupList{
		array: array{
			recordSize: 2,
			length:     len(offsets),
			loc:        binarySegm(buf),
		},
		base: binarySegm(make([]byte, 64)),
		name: "LookupList",
	}

	indices := []int{2, 0, 2}
	var sub RootList = ll.Subset(indices)
	if sub.Len() != 3 {
		t.Fatalf("expected subset length 3, got %d", sub.Len())
	}
	if got := sub.Get(0).U16(0); got != offsets[2] {
		t.Fatalf("expected subset[0]=%d, got %d", offsets[2], got)
	}
	if got := sub.Get(1).U16(0); got != offsets[0] {
		t.Fatalf("expected subset[1]=%d, got %d", offsets[0], got)
	}
	if got := sub.Get(2).U16(0); got != offsets[2] {
		t.Fatalf("expected subset[2]=%d, got %d", offsets[2], got)
	}

	indices[0] = 1
	if got := sub.Get(0).U16(0); got != offsets[2] {
		t.Fatalf("subset should not depend on indices slice, got %d", got)
	}

	sub2 := sub.Subset([]int{1})
	if sub2.Len() != 1 {
		t.Fatalf("expected second subset length 1, got %d", sub2.Len())
	}
	if got := sub2.Get(0).U16(0); got != offsets[0] {
		t.Fatalf("expected sub2[0]=%d, got %d", offsets[0], got)
	}
}
