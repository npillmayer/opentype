package ot

import (
	"errors"
	"testing"
)

func TestCMapFormat4ArrayReadFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset uint16
		ids    array
	}{
		{"negative index", 2, array{recordSize: 2, length: 1, loc: binarySegm{0, 7}}},
		{"past logical end", 6, array{recordSize: 2, length: 1, loc: binarySegm{0, 7, 0, 9}}},
		{"truncated record", 6, array{recordSize: 2, length: 2, loc: binarySegm{0, 7, 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := format4GlyphIndex{
				entries: []cmapEntry16{
					{start: 'A', end: 'A', delta: 3, offset: tc.offset},
					{start: 0xffff, end: 0xffff, delta: 1},
				},
				glyphIds: tc.ids,
			}
			if got, err := index.Lookup('A'); got != 0 || !errors.Is(err, errBufferBounds) {
				t.Fatalf("Lookup(A) = %d, %v; want missing glyph and bounds error", got, err)
			}
		})
	}
}

func TestClassLookupArrayReadFallback(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		length := 1
		values := binarySegm{0, 7, 0, 9}
		ranges := binarySegm{0, 10, 0, 12, 0, 7, 0, 20, 0, 22, 0, 9}
		if truncated {
			length = 2
			values = values[:3]
			ranges = ranges[:11]
		}
		format1 := classDefinitionsFormat1{
			start: 10, count: 2,
			valueArray: array{recordSize: 2, length: length, loc: values},
		}
		format2 := classDefinitionsFormat2{
			count:       2,
			classRanges: array{recordSize: 6, length: length, loc: ranges},
		}
		if got := format1.Lookup(10); got != 7 {
			t.Errorf("format 1 valid record = %d, want 7", got)
		}
		if got := format1.Lookup(11); got != 0 {
			t.Errorf("format 1 failed read (truncated=%v) = %d, want default class", truncated, got)
		}
		if got := format2.Lookup(10); got != 7 {
			t.Errorf("format 2 valid record = %d, want 7", got)
		}
		if got := format2.Lookup(20); got != 0 {
			t.Errorf("format 2 failed read (truncated=%v) = %d, want default class", truncated, got)
		}
	}
}
