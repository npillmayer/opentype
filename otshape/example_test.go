package otshape_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// Expected output in these examples comes from the independent HarfBuzz
// captures latin_office_liga_on and latin_office_liga_off in
// internal/hbcmp/testdata/text/baselines.json, with cluster offsets added for
// the channel example.

// A producer shapes successive runs while the caller consumes one continuous
// glyph channel. Finish closes the channel after the last run. Cancel ctx if
// the consumer stops early; cancellation releases a producer blocked on output.
func ExampleStreamingShaper() {
	params := otshape.Params{
		Font:      loadGentium(),
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
	}
	stream, err := otshape.NewStreamingShaper(2, otcore.New())
	if err != nil {
		panic(err)
	}
	if err := stream.SetInitialCluster(10); err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for _, text := range []string{"office", "office"} {
			if err := stream.Shape(ctx, params, strings.NewReader(text), otshape.BufferOptions{}); err != nil {
				return // Shape stores the error and closes the channel.
			}
		}
		stream.Finish()
	}()
	for glyph := range stream.Glyphs() {
		glyphPrinter{}.WriteGlyph(glyph)
	}
	if err := stream.Err(); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// gid=82 cluster=10 advance=1030
	// gid=741 cluster=11 advance=1829
	// gid=70 cluster=14 advance=895
	// gid=72 cluster=15 advance=946
	// gid=82 cluster=16 advance=1030
	// gid=741 cluster=17 advance=1829
	// gid=70 cluster=20 advance=895
	// gid=72 cluster=21 advance=946
}

// glyphPrinter is a minimal GlyphSink that prints each glyph as it arrives.
type glyphPrinter struct{}

func (glyphPrinter) WriteGlyph(g otshape.GlyphRecord) error {
	fmt.Printf("gid=%d cluster=%d advance=%d\n", g.GID, g.Cluster, g.Pos.XAdvance)
	return nil
}

// loadGentium loads a checked-in test font. Fonts must stay unmodified while
// in use, because the parsed font refers to the original bytes.
func loadGentium() *ot.Font {
	data, err := os.ReadFile("../testdata/fonts/GentiumPlus-R.ttf")
	if err != nil {
		panic(err)
	}
	otf, err := ot.Parse(data)
	if err != nil {
		panic(err)
	}
	return otf
}

// Shaping Latin text with the default features. Gentium substitutes the
// "ffi" ligature; the ligature glyph carries the cluster of its first input
// character, and the following clusters keep their original scalar indexes.
func ExampleShaper_Shape() {
	params := otshape.Params{
		Font:      loadGentium(),
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
	}
	shaper := otshape.NewShaper(otcore.New())
	err := shaper.Shape(params, strings.NewReader("office"), glyphPrinter{}, otshape.BufferOptions{})
	if err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// gid=82 cluster=0 advance=1030
	// gid=741 cluster=1 advance=1829
	// gid=70 cluster=4 advance=895
	// gid=72 cluster=5 advance=946
}

// Short runs can be shaped synchronously into reusable caller-owned storage.
func ExampleShaper_ShapeInto() {
	params := otshape.Params{
		Font:      loadGentium(),
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
	}
	shaper := otshape.NewShaper(otcore.New())
	var storage [16]otshape.GlyphRecord
	n, err := shaper.ShapeInto(params, strings.NewReader("office"), storage[:])
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, glyph := range storage[:n] {
		glyphPrinter{}.WriteGlyph(glyph)
	}
	// Output:
	// gid=82 cluster=0 advance=1030
	// gid=741 cluster=1 advance=1829
	// gid=70 cluster=4 advance=895
	// gid=72 cluster=5 advance=946
}

// eventList is a minimal InputEventSource that replays a fixed event list.
type eventList []otshape.InputEvent

func (l *eventList) ReadEvent() (otshape.InputEvent, error) {
	if len(*l) == 0 {
		return otshape.InputEvent{}, io.EOF
	}
	ev := (*l)[0]
	*l = (*l)[1:]
	return ev, nil
}

// Scoping features with input events. A push event disables "liga" for the
// runes up to the matching pop event, so the "ffi" ligature is not formed.
// Every push must be closed by a pop before the end of input.
func ExampleShaper_ShapeEvents() {
	params := otshape.Params{
		Font:      loadGentium(),
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
	}
	events := eventList{{
		Kind: otshape.InputEventPushFeatures,
		Push: []otshape.FeatureSetting{{Tag: ot.T("liga"), Enabled: false}},
	}}
	for _, r := range "office" {
		events = append(events, otshape.InputEvent{Kind: otshape.InputEventRune, Rune: r})
	}
	events = append(events, otshape.InputEvent{Kind: otshape.InputEventPopFeatures})

	shaper := otshape.NewShaper(otcore.New())
	err := shaper.ShapeEvents(params, &events, glyphPrinter{}, otshape.BufferOptions{})
	if err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// gid=82 cluster=0 advance=1030
	// gid=73 cluster=1 advance=637
	// gid=73 cluster=2 advance=637
	// gid=76 cluster=3 advance=555
	// gid=70 cluster=4 advance=895
	// gid=72 cluster=5 advance=946
}
