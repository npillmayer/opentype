package ot

import (
	"encoding/binary"
	"testing"
)

func TestCMapFormat4NonzeroRangeOffsetAndDelta(t *testing.T) {
	// Two segments: A-C use glyphIdArray plus delta; 0xffff is the sentinel.
	// The first idRangeOffset word is four bytes before glyphIdArray.
	words := []uint16{
		4, 38, 0, // format, length, language
		4, 4, 1, 0, // segCountX2, searchRange, entrySelector, rangeShift
		0x0043, 0xffff, // endCode
		0, // reservedPad
		0x0041, 0xffff, // startCode
		3, 1, // idDelta
		4, 0, // idRangeOffset
		7, 0, 9, // glyphIdArray: zero must remain missing despite the delta
	}
	data := make(binarySegm, len(words)*2)
	for i, word := range words {
		binary.BigEndian.PutUint16(data[i*2:], word)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("format-4 parsing/lookup must accept nonzero range offset and delta: %v", r)
		}
	}()
	index, err := makeGlyphIndexFormat4(data, T("cmap"), 0, &errorCollector{})
	if err != nil {
		t.Fatalf("parse format-4 cmap: %v", err)
	}
	for _, tc := range []struct {
		codepoint rune
		want      GlyphIndex
	}{
		{'A', 10},
		{'B', 0},
		{'C', 12},
		{'D', 0},
		{0xffff, 0},
	} {
		if got := index.Lookup(tc.codepoint); got != tc.want {
			t.Errorf("Lookup(%U) = %d, want %d", tc.codepoint, got, tc.want)
		}
	}
}
