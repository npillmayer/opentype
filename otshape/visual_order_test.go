package otshape_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"
	"golang.org/x/text/unicode/bidi"
)

func TestVisualOrder(t *testing.T) {
	// Repeated clusters and distinct positions catch cluster-only reversal and
	// accidentally detaching positioning/metadata from the glyph record.
	a := otshape.GlyphRecord{GID: 96, Cluster: 0, Mask: 3, Pos: otlayout.PosItem{XAdvance: 730}}
	b := otshape.GlyphRecord{GID: 100, Cluster: 0, UnsafeFlags: 1, Pos: otlayout.PosItem{XOffset: 539, YOffset: -20, AttachTo: 0, AttachKind: otlayout.AttachMarkToBase}}
	c := otshape.GlyphRecord{GID: 79, Cluster: 0, Pos: otlayout.PosItem{XOffset: 227, YAdvance: -5}}
	d := otshape.GlyphRecord{GID: 42, Cluster: 3, Pos: otlayout.PosItem{XAdvance: 500}}
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		for _, inPlace := range []bool{false, true} {
			t.Run(fmt.Sprintf("direction=%d/inPlace=%t", dir, inPlace), func(t *testing.T) {
				input := []otshape.GlyphRecord{a, b, c, d}
				original := slices.Clone(input)
				want := []otshape.GlyphRecord{a, b, c, d}
				if dir == bidi.RightToLeft {
					want = []otshape.GlyphRecord{d, c, b, a}
				}
				got, err := otshape.VisualOrder(input, dir, inPlace)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("got %+v, %v; want %+v", got, err, want)
				}
				if (&got[0] == &input[0]) != inPlace {
					t.Fatal("unexpected backing array aliasing")
				}
				if !inPlace {
					got[0].GID = 999
					if !reflect.DeepEqual(input, original) {
						t.Fatal("copy mode modified input")
					}
				}
			})
		}
	}
}

func TestVisualOrderSmallInputs(t *testing.T) {
	for _, input := range [][]otshape.GlyphRecord{nil, {}, {{GID: 1}}} {
		for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
			for _, inPlace := range []bool{false, true} {
				got, err := otshape.VisualOrder(input, dir, inPlace)
				if err != nil || !reflect.DeepEqual(got, input) {
					t.Fatalf("small input: got %v, %v; want %v", got, err, input)
				}
				if len(input) > 0 && (&got[0] == &input[0]) != inPlace {
					t.Fatal("single-glyph backing array aliasing")
				}
			}
		}
	}
}

func TestVisualOrderInvalidDirection(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.Mixed, bidi.Neutral, bidi.Direction(99)} {
		for _, inPlace := range []bool{false, true} {
			input := []otshape.GlyphRecord{{GID: 1}, {GID: 2}}
			original := slices.Clone(input)
			got, err := otshape.VisualOrder(input, dir, inPlace)
			if err == nil || got != nil || !reflect.DeepEqual(input, original) {
				t.Fatalf("invalid direction %d: got %v, %v; input %v", dir, got, err, input)
			}
		}
	}
}

func ExampleVisualOrder() {
	logical := []otshape.GlyphRecord{{GID: 10, Cluster: 0}, {GID: 11, Cluster: 0}, {GID: 12, Cluster: 1}}
	visual, err := otshape.VisualOrder(logical, bidi.RightToLeft, false)
	if err != nil {
		panic(err)
	}
	for _, g := range visual {
		fmt.Printf("gid=%d cluster=%d\n", g.GID, g.Cluster)
	}
	fmt.Println("logical first glyph:", logical[0].GID)
	// Output:
	// gid=12 cluster=1
	// gid=11 cluster=0
	// gid=10 cluster=0
	// logical first glyph: 10
}

// A client record need not embed an OpenType type or even be comparable.
func TestVisualOrderClientGlyphs(t *testing.T) {
	type clientGlyph struct {
		ID              uint32
		Cluster         uint32
		Advance, Offset int32
		Metadata        []string
	}
	a := clientGlyph{1, 0, 730, 0, []string{"base"}}
	b := clientGlyph{2, 0, 0, 539, []string{"mark"}}
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		for _, inPlace := range []bool{false, true} {
			t.Run(fmt.Sprintf("direction=%d/inPlace=%t", dir, inPlace), func(t *testing.T) {
				input := []clientGlyph{a, b}
				want := []clientGlyph{a, b}
				if dir == bidi.RightToLeft {
					want = []clientGlyph{b, a}
				}
				got, err := otshape.VisualOrder(input, dir, inPlace)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("got %+v, %v; want %+v", got, err, want)
				}
				if (&got[0] == &input[0]) != inPlace {
					t.Fatal("unexpected backing array aliasing")
				}
				if !inPlace {
					got[0].ID = 999
					if !reflect.DeepEqual(input, []clientGlyph{a, b}) {
						t.Fatal("copy mode modified input")
					}
				}
			})
		}
	}
	// An untyped nil requires an explicit type argument with the generic API.
	got, err := otshape.VisualOrder[clientGlyph](nil, bidi.RightToLeft, false)
	if err != nil || got != nil {
		t.Fatalf("nil input: %v, %v", got, err)
	}
}
