package otquery

import (
	"errors"
	"testing"

	"github.com/npillmayer/opentype/ot"
)

type failingCMap struct{ err error }

func (m failingCMap) Lookup(rune) (ot.GlyphIndex, error) { return 0, m.err }
func (m failingCMap) ReverseLookup(ot.GlyphIndex) rune   { return 0 }

func TestGlyphIndexPreservesError(t *testing.T) {
	want := errors.New("malformed cmap record")
	font := &ot.Font{CMap: &ot.CMapTable{GlyphIndexMap: failingCMap{want}}}
	if gid, err := GlyphIndex(font, 'A'); gid != 0 || !errors.Is(err, want) {
		t.Fatalf("GlyphIndex = %d, %v; want zero and original error", gid, err)
	}
}

func TestGlyphIndexUnavailableCMap(t *testing.T) {
	for _, font := range []*ot.Font{nil, {}, {CMap: &ot.CMapTable{}}} {
		if gid, err := GlyphIndex(font, 'A'); gid != 0 || err == nil {
			t.Fatalf("GlyphIndex = %d, %v; want zero and error", gid, err)
		}
	}
}
