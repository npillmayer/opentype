package ot

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestArrayGetErrorContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    array
		i    int
		want string
	}{
		{"logical bounds", array{name: "GlyphIDs", recordSize: 2, length: 1, loc: binarySegm{0, 7, 0, 9}}, 1,
			`array "GlyphIDs": index 1 outside length 1`},
		{"physical bounds", array{name: "GlyphIDs", recordSize: 2, length: 2, loc: binarySegm{0, 7, 0}}, 1,
			`array "GlyphIDs": index 1 with record size 2 exceeds backing size 3 bytes`},
		{"invalid record size", array{name: "GlyphIDs", recordSize: 0, length: 1}, 0,
			`array "GlyphIDs": invalid record size 0 at index 0`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item, err := tc.a.Get(tc.i)
			if item != nil || !errors.Is(err, errBufferBounds) {
				t.Fatalf("Get(%d) = %v, %v; want nil and wrapped errBufferBounds", tc.i, item, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain context %q", err, tc.want)
			}
		})
	}
}

func TestArrayRangeResults(t *testing.T) {
	a := array{name: "records", recordSize: 2, length: 4, loc: binarySegm{0, 7, 0, 9, 0}}
	count := 0
	for i, record := range a.Range() {
		if i != count {
			t.Fatalf("index = %d, want %d", i, count)
		}
		count++
		if i < 2 {
			if record.Err != nil || record.Data.U16(0) != uint16(7+2*i) {
				t.Fatalf("record %d = %+v", i, record)
			}
			if &record.Data[0] != &a.loc[i*2] {
				t.Fatal("successful result must retain the backing-byte view")
			}
		} else {
			_, wantErr := a.Get(i)
			if record.Data != nil || !errors.Is(record.Err, errBufferBounds) {
				t.Fatalf("record %d = %+v; want nil data and bounds error", i, record)
			}
			if record.Err.Error() != wantErr.Error() {
				t.Fatalf("Range lost Get error context: %v", record.Err)
			}
		}
	}
	if count != a.Len() {
		t.Fatalf("yielded %d records, want %d", count, a.Len())
	}
}

func TestArrayRangeEarlyStop(t *testing.T) {
	a := array{recordSize: 2, length: 3, loc: binarySegm{0, 7}}
	for _, stopAt := range []int{0, 1} {
		count := 0
		for i := range a.Range() {
			count++
			if i == stopAt {
				break
			}
		}
		if count != stopAt+1 {
			t.Fatalf("break at %d yielded %d records", stopAt, count)
		}
	}
}

func TestArrayRangeEmpty(t *testing.T) {
	for range (array{}).Range() {
		t.Fatal("empty array must yield nothing")
	}
}

func TestArrayGetResult(t *testing.T) {
	a := array{recordSize: 2, length: 2, loc: binarySegm{0, 7, 0, 9}}
	item, err := a.Get(1)
	if err != nil || item.U16(0) != 9 {
		t.Fatalf("Get(1) = %v, %v; want record 9 without error", item, err)
	}
	for _, i := range []int{-1, 2} {
		item, err := a.Get(i)
		if item != nil || !errors.Is(err, errBufferBounds) {
			t.Errorf("Get(%d) = %v, %v; want nil and errBufferBounds", i, item, err)
		}
	}
	// A partial record must return a bounds error as well.
	partial := array{recordSize: 2, length: 1, loc: binarySegm{0}}
	if item, err := partial.Get(0); item != nil || !errors.Is(err, errBufferBounds) {
		t.Errorf("partial.Get(0) = %v, %v; want nil and errBufferBounds", item, err)
	}
}

func TestArrayGetBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    array
		i    int
	}{
		{"trailing complete record", array{recordSize: 2, length: 1, loc: binarySegm{0, 7, 0, 9}}, 1},
		{"truncated final record", array{recordSize: 2, length: 2, loc: binarySegm{0, 7, 0}}, 1},
		{"zero record size", array{length: 1, loc: binarySegm{0}}, 0},
		{"negative record size", array{recordSize: -1, length: 1, loc: binarySegm{0}}, 0},
		{"empty array", array{}, 0},
		{"zero length with backing bytes", array{recordSize: 2, loc: binarySegm{0, 7}}, 0},
		{"negative length", array{recordSize: 2, length: -1, loc: binarySegm{0, 7}}, 0},
		{"minimum index", array{recordSize: 2, length: 1}, math.MinInt},
		{"maximum index", array{recordSize: 2, length: math.MaxInt}, math.MaxInt},
		{"multiplication overflow", array{recordSize: 4, length: math.MaxInt, loc: binarySegm{0, 7}}, math.MaxInt / 2},
		{"oversized record", array{recordSize: math.MaxInt, length: 2, loc: binarySegm{0, 7}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item, err := tc.a.Get(tc.i)
			if item != nil || !errors.Is(err, errBufferBounds) {
				t.Fatalf("Get(%d) = %v, %v; want nil and errBufferBounds", tc.i, item, err)
			}
		})
	}
}

func TestBinarySegmViewBounds(t *testing.T) {
	b := binarySegm{1, 2, 3, 4}
	for _, tc := range []struct {
		name      string
		offset, n int
		want      []byte
	}{
		{"whole segment", 0, 4, []byte{1, 2, 3, 4}},
		{"last byte", 3, 1, []byte{4}},
		{"negative offset", -1, 1, nil},
		{"past end", 5, 1, nil},
		{"at end", 4, 1, nil},
		{"truncated view", 3, 2, nil},
		{"zero length", 0, 0, nil},
		{"zero length at end", 4, 0, nil},
		{"negative length", 0, -1, nil},
		{"length overflow", 1, math.MaxInt, nil},
		{"offset overflow", math.MaxInt, 1, nil},
		{"minimum offset", math.MinInt, 1, nil},
		{"minimum length", 0, math.MinInt, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := b.view(tc.offset, tc.n)
			if tc.want == nil {
				if got != nil || !errors.Is(err, errBufferBounds) {
					t.Fatalf("view = %v, %v; want nil and errBufferBounds", got, err)
				}
			} else if err != nil || !bytes.Equal(got, tc.want) {
				t.Fatalf("view = %v, %v; want %v without error", got, err, tc.want)
			}
		})
	}
	if got, err := (binarySegm(nil)).view(0, 1); got != nil || !errors.Is(err, errBufferBounds) {
		t.Fatalf("empty view = %v, %v; want nil and errBufferBounds", got, err)
	}
}
