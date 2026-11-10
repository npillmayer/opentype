package otshape_test

import (
	"fmt"
	"testing"

	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"
)

func TestGlyphRecordPositionedGlyph(t *testing.T) {
	record := otshape.GlyphRecord{
		GID: 42, Cluster: 7, Mask: 123, UnsafeFlags: 3,
		Pos: otlayout.PosItem{
			XAdvance: 123456, YAdvance: -234567, XOffset: -345678, YOffset: 456789,
			Cluster: 99, Flags: 5, AttachTo: 2, AttachKind: otlayout.AttachMarkToBase,
			AttachClass: 4, AnchorRef: otlayout.AnchorRef{MarkAnchor: 10, BaseAnchor: 20},
		},
	}
	original := record
	want := otshape.PositionedGlyph{GID: 42, Cluster: 7,
		XAdvance: 123456, YAdvance: -234567, XOffset: -345678, YOffset: 456789}
	got := record.PositionedGlyph()
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	got.GID, got.Cluster, got.XOffset = 1, 2, 3
	if record != original {
		t.Fatal("conversion or mutation of compact value changed detailed record")
	}
	if got := (otshape.GlyphRecord{}).PositionedGlyph(); got != (otshape.PositionedGlyph{}) {
		t.Fatalf("zero record converted to %+v", got)
	}
}

func ExampleGlyphRecord_PositionedGlyph() {
	record := otshape.GlyphRecord{
		GID: 42, Cluster: 3,
		Pos: otlayout.PosItem{XAdvance: 600, XOffset: -25, YOffset: 80},
	}
	glyph := record.PositionedGlyph()
	fmt.Printf("glyph %d, cluster %d: advance (%d, %d), offset (%d, %d)\n",
		glyph.GID, glyph.Cluster, glyph.XAdvance, glyph.YAdvance, glyph.XOffset, glyph.YOffset)
	// Output: glyph 42, cluster 3: advance (600, 0), offset (-25, 80)
}
