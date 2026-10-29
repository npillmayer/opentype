package ot

import "testing"

func TestClassDefinitionFormat2InclusiveRanges(t *testing.T) {
	// Independent class values and inclusive endpoints, including singleton
	// ranges, adjacent ranges, gaps, and the maximum representable glyph ID.
	defs := classDefinitionsFormat2{
		count: 5,
		classRanges: array{recordSize: 6, length: 5, loc: binarySegm{
			0, 0, 0, 0, 0, 4,
			0, 10, 0, 12, 0, 1,
			0, 13, 0, 13, 0, 2,
			0, 20, 0, 22, 0, 3,
			255, 255, 255, 255, 0, 4,
		}},
	}
	for _, tc := range []struct {
		gid  GlyphIndex
		want int
	}{
		{0, 4}, {1, 0}, {9, 0}, {10, 1}, {11, 1}, {12, 1},
		{13, 2}, {14, 0}, {19, 0}, {20, 3}, {21, 3}, {22, 3},
		{23, 0}, {65534, 0}, {65535, 4},
	} {
		if got := defs.Lookup(tc.gid); got != tc.want {
			t.Errorf("Lookup(%d) = %d, want %d", tc.gid, got, tc.want)
		}
	}
}
