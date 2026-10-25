package ot

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestCMapLookupMalformedMapping(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index CMapGlyphIndex
	}{
		{"format4 glyph limit", format4GlyphIndex{entries: []cmapEntry16{{start: 'A', end: 'A', delta: 1}}, numGlyphs: 10}},
		{"format4 unaligned offset", format4GlyphIndex{entries: []cmapEntry16{{start: 'A', end: 'A', offset: 3}}}},
		{"format12 glyph limit", format12GlyphIndex{entries: []cmapEntry32{{start: 'A', end: 'A', delta: 10}}, numGlyphs: 10}},
		{"format12 narrowing overflow", format12GlyphIndex{entries: []cmapEntry32{{start: 'A', end: 'A', delta: 65536}}}},
		{"format12 addition overflow", format12GlyphIndex{entries: []cmapEntry32{{start: 'A' - 1, end: 'A', delta: 0xffffffff}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gid, err := tc.index.Lookup('A')
			if gid != 0 || err == nil || !strings.Contains(err.Error(), "U+0041") {
				t.Fatalf("Lookup(A) = %d, %v; want zero and contextual error", gid, err)
			}
			if gid, err := tc.index.Lookup('Z'); gid != 0 || err != nil {
				t.Fatalf("absent character = %d, %v; want 0, nil", gid, err)
			}
		})
	}
}

func TestCMapFormat12LookupValidAndMissing(t *testing.T) {
	index := format12GlyphIndex{entries: []cmapEntry32{{start: 'A', end: 'C', delta: 7}}}
	for _, tc := range []struct {
		r    rune
		want GlyphIndex
	}{{'A', 7}, {'C', 9}, {'D', 0}} {
		if got, err := index.Lookup(tc.r); got != tc.want || err != nil {
			t.Errorf("Lookup(%U) = %d, %v; want %d, nil", tc.r, got, err, tc.want)
		}
	}
}

func TestCMapFormat4NonzeroRangeOffsetAndDelta(t *testing.T) {
	// Two segments: A-C use glyphIdArray plus delta; 0xffff is the sentinel.
	// The first idRangeOffset word is four bytes before glyphIdArray.
	words := []uint16{
		4, 38, 0, // format, length, language
		4, 4, 1, 0, // segCountX2, searchRange, entrySelector, rangeShift
		0x0043, 0xffff, // endCode
		0,              // reservedPad
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
		if got, err := index.Lookup(tc.codepoint); err != nil || got != tc.want {
			t.Errorf("Lookup(%U) = %d, %v; want %d, nil", tc.codepoint, got, err, tc.want)
		}
	}
}
