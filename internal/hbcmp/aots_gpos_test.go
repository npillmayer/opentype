package hbcmp

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otshape"
)

// --ned suppresses advances in HB's text format, making @x,y the origin
// (preceding advances + current offset). Omitted @ means origin (0,0).
// It does not supply independently assertable advances or offsets.
type aotsGlyphOrigin struct {
	GID  ot.GlyphIndex
	X, Y int64
}

func (g aotsGlyphOrigin) String() string {
	return fmt.Sprintf("%d@%d,%d", g.GID, g.X, g.Y)
}

func parseAOTSGlyphOrigin(token string) (aotsGlyphOrigin, error) {
	var g aotsGlyphOrigin
	id, coords, hasCoords := strings.Cut(token, "@")
	n, err := strconv.ParseUint(id, 10, 16)
	if err != nil {
		return g, fmt.Errorf("invalid numeric GID in %q", token)
	}
	g.GID = ot.GlyphIndex(n)
	if hasCoords {
		x, y, ok := strings.Cut(coords, ",")
		if !ok {
			return g, fmt.Errorf("expected origin x,y in %q", token)
		}
		g.X, err = strconv.ParseInt(x, 10, 32)
		if err != nil {
			return g, fmt.Errorf("invalid origin X in %q", token)
		}
		g.Y, err = strconv.ParseInt(y, 10, 32)
		if err != nil {
			return g, fmt.Errorf("invalid origin Y in %q", token)
		}
	}
	return g, nil
}

func aotsGlyphOrigins(records []otshape.GlyphRecord) []aotsGlyphOrigin {
	origins := make([]aotsGlyphOrigin, len(records))
	var x, y int64
	for i, record := range records {
		origins[i] = aotsGlyphOrigin{record.GID, x + int64(record.Pos.XOffset), y + int64(record.Pos.YOffset)}
		x += int64(record.Pos.XAdvance)
		y += int64(record.Pos.YAdvance)
	}
	return origins
}

func runAOTSGPOSCase(t *testing.T, id string) {
	t.Helper()
	runAOTSCaseWithBuffers(t, id, "glyph_ids_and_origins", otshape.BufferOptions{})
}

func TestAOTSGPOSBaselines(t *testing.T) {
	for _, id := range []string{
		"gpos_chaining3_boundary.tests:3",
		"gpos4_simple.tests:3", // unattached GDEF mark has zero final advance
		"gpos4_simple.tests:4",
		"gpos2_1_lookupflag.tests:3", // IgnoreBaseGlyphs and pair advances
		"gpos2_1_simple.tests:2",     // matching pair and nonmatching second glyph
		"gpos2_1_simple.tests:3",     // covered first glyph at end of input
	} {
		t.Run(id, func(t *testing.T) { runAOTSGPOSCase(t, id) })
	}
}

func TestAOTSOriginAdapter(t *testing.T) {
	const record = `../fonts/example.otf;--features="test" --single-par --no-clusters --no-glyph-names --ned;U+0000,U+0012,U+0013;[0|18@1500,0|19@1400,-80]`
	c, err := parseAOTSRecord(record, "glyph_ids_and_origins")
	want := []aotsGlyphOrigin{{0, 0, 0}, {18, 1500, 0}, {19, 1400, -80}}
	if err != nil || !slices.Equal(c.wantOrigins, want) {
		t.Fatalf("origins=%v error=%v; want %v", c.wantOrigins, err, want)
	}
	c, err = parseAOTSRecord(strings.Replace(record, "19@1400,-80", "19", 1), "glyph_ids_and_origins")
	if err != nil || len(c.wantOrigins) != 3 || c.wantOrigins[2] != (aotsGlyphOrigin{GID: 19}) {
		t.Fatalf("omitted origin after nonzero origin must mean zero: %v, %v", c.wantOrigins, err)
	}
	// Offsets affect the current origin, not the following pen position. Include
	// negative coordinates, a zero advance, vertical advances, and an origin at
	// zero after a nonzero pen position; this is adapter arithmetic, not a font case.
	records := []otshape.GlyphRecord{
		{GID: 1, Pos: otlayout.PosItem{XAdvance: 100, YAdvance: 10, XOffset: -5, YOffset: 3}},
		{GID: 2, Pos: otlayout.PosItem{XAdvance: 0, YAdvance: -20, XOffset: -100, YOffset: -10}},
		{GID: 3, Pos: otlayout.PosItem{XOffset: 7, YOffset: -2}},
	}
	want = []aotsGlyphOrigin{{1, -5, 3}, {2, 0, 0}, {3, 107, -12}}
	if got := aotsGlyphOrigins(records); !slices.Equal(got, want) {
		t.Fatalf("origins=%v; want %v", got, want)
	}
	for _, token := range []string{"18@", "18@1", "18@1,2,3", "18@1,", "18@,2", "18=0", "18+1500", "18@1,2+1500", "18@2147483648,0", "65536", "g18", "*"} {
		t.Run(token, func(t *testing.T) {
			if _, err := parseAOTSGlyphOrigin(token); err == nil {
				t.Fatalf("unsupported origin accepted: %q", token)
			}
		})
	}
	if _, err := parseAOTSRecord(strings.Replace(record, "--ned", "--no-positions", 1), "glyph_ids_and_origins"); err == nil {
		t.Fatal("glyph-only mode accepted as an origin oracle")
	}
	if _, err := loadAOTSCase("classdef2.tests:2", "glyph_ids_and_origins"); err == nil {
		t.Fatal("glyph-only manifest entry accepted as an origin oracle")
	}
}
