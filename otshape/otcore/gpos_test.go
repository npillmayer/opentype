package otcore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/internal/fontload"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otquery"
	"github.com/npillmayer/opentype/otshape"
	"github.com/npillmayer/opentype/otshape/otcore"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

func TestShapeAppliesGPOSSingleAdjust(t *testing.T) {
	font := loadRootOTFont(t, "GentiumPlus-R.ttf")
	_, gposFeats, err := otlayout.FontFeatures(font, ot.T("latn"), ot.T("ENG"))
	if err != nil {
		t.Fatalf("font feature extraction failed: %v", err)
	}
	markFeat := featureByTag(gposFeats, ot.T("mark"))
	if markFeat == nil {
		t.Skip("font exposes no GPOS mark feature")
	}
	_, node := lookupNodeForFeatureType(t, font, markFeat, ot.GPosLookupTypeSingle)
	payload := node.GPosPayload().SingleFmt2
	if payload == nil {
		t.Skip("single-position lookup is not format 2")
	}
	gid, cp, covInx, ok := firstCoveredGlyphWithCodepoint(font, node.Coverage)
	if !ok || covInx < 0 || covInx >= len(payload.Values) {
		t.Skip("no cmap-backed glyph for single-position lookup coverage")
	}
	want := valueDelta(payload.Values[covInx], payload.ValueFormat)
	want.XAdvance += int32(otquery.GlyphMetrics(font, gid).Advance)

	got := shapeRunes(t, font, []rune{cp}, []otshape.FeatureRange{
		{Feature: ot.T("mark"), On: true},
		{Feature: ot.T("kern"), On: false},
		{Feature: ot.T("mkmk"), On: false},
	})
	if len(got) != 1 {
		t.Fatalf("shaped glyph count = %d, want 1", len(got))
	}
	if got[0].GID != gid {
		t.Fatalf("shaped glyph = %d, want %d", got[0].GID, gid)
	}
	assertPosDelta(t, got[0].Pos, want)
}

func TestShapeAppliesGPOSPairAdjust(t *testing.T) {
	font := loadRootOTFont(t, "GentiumPlus-R.ttf")
	_, gposFeats, err := otlayout.FontFeatures(font, ot.T("latn"), ot.T("ENG"))
	if err != nil {
		t.Fatalf("font feature extraction failed: %v", err)
	}
	kernFeat := featureByTag(gposFeats, ot.T("kern"))
	if kernFeat == nil {
		t.Skip("font exposes no GPOS kern feature")
	}
	_, node := lookupNodeForFeatureType(t, font, kernFeat, ot.GPosLookupTypePair)
	payload := node.GPosPayload().PairFmt1
	if payload == nil {
		t.Skip("pair-position lookup is not format 1")
	}
	g1, cp1, g2, cp2, rec, ok := firstPairFmt1CandidateWithCodepoints(font, node, payload)
	if !ok {
		t.Skip("no cmap-backed pair-position candidate found")
	}
	want1 := valueDelta(rec.Value1, payload.ValueFormat1)
	want2 := valueDelta(rec.Value2, payload.ValueFormat2)
	want1.XAdvance += int32(otquery.GlyphMetrics(font, g1).Advance)
	want2.XAdvance += int32(otquery.GlyphMetrics(font, g2).Advance)

	got := shapeRunes(t, font, []rune{cp1, cp2}, []otshape.FeatureRange{
		{Feature: ot.T("kern"), On: true},
		{Feature: ot.T("mark"), On: false},
		{Feature: ot.T("mkmk"), On: false},
	})
	if len(got) != 2 {
		t.Fatalf("shaped glyph count = %d, want 2", len(got))
	}
	if got[0].GID != g1 || got[1].GID != g2 {
		t.Fatalf("shaped glyphs = [%d %d], want [%d %d]", got[0].GID, got[1].GID, g1, g2)
	}
	assertPosDelta(t, got[0].Pos, want1)
	assertPosDelta(t, got[1].Pos, want2)
}

func TestShapeAppliesGPOSMarkToBaseAttachment(t *testing.T) {
	font := loadRootOTFont(t, "GentiumPlus-R.ttf")
	_, gposFeats, err := otlayout.FontFeatures(font, ot.T("latn"), ot.T("ENG"))
	if err != nil {
		t.Fatalf("font feature extraction failed: %v", err)
	}
	markFeat := featureByTag(gposFeats, ot.T("mark"))
	if markFeat == nil {
		t.Skip("font exposes no GPOS mark feature")
	}
	_, node := lookupNodeForFeatureType(t, font, markFeat, ot.GPosLookupTypeMarkToBase)
	payload := node.GPosPayload().MarkToBaseFmt1
	if payload == nil {
		t.Skip("mark-to-base lookup is not format 1")
	}
	base, baseCP, baseInx, ok := firstCoveredGlyphWithCodepoint(font, payload.BaseCoverage)
	if !ok {
		t.Skip("no cmap-backed base glyph for mark-to-base lookup")
	}
	mark, markCP, markInx, ok := firstCoveredGlyphWithCodepoint(font, node.Coverage)
	if !ok || markInx < 0 || markInx >= len(payload.MarkRecords) {
		t.Skip("no cmap-backed mark glyph for mark-to-base lookup")
	}
	class := int(payload.MarkRecords[markInx].Class)
	wantMarkOff, wantBaseOff, ok := payload.AnchorOffsets(markInx, baseInx, class)
	if !ok {
		t.Skip("invalid mark/base anchor offsets for selected candidate")
	}

	got := shapeRunes(t, font, []rune{baseCP, markCP}, []otshape.FeatureRange{
		{Feature: ot.T("mark"), On: true},
		{Feature: ot.T("kern"), On: false},
		{Feature: ot.T("mkmk"), On: false},
	})
	if len(got) != 2 {
		t.Fatalf("shaped glyph count = %d, want 2", len(got))
	}
	if got[0].GID != base || got[1].GID != mark {
		t.Fatalf("shaped glyphs = [%d %d], want [%d %d]", got[0].GID, got[1].GID, base, mark)
	}
	markPos := got[1].Pos
	if markPos.AttachKind != otlayout.AttachMarkToBase {
		t.Fatalf("mark attach kind = %d, want %d", markPos.AttachKind, otlayout.AttachMarkToBase)
	}
	if markPos.AttachTo != 0 {
		t.Fatalf("mark AttachTo = %d, want 0", markPos.AttachTo)
	}
	if markPos.AttachClass != payload.MarkRecords[markInx].Class {
		t.Fatalf("mark AttachClass = %d, want %d", markPos.AttachClass, payload.MarkRecords[markInx].Class)
	}
	if markPos.AnchorRef.MarkAnchor != wantMarkOff || markPos.AnchorRef.BaseAnchor != wantBaseOff {
		t.Fatalf("unexpected mark anchor refs: got mark=%d base=%d, want mark=%d base=%d",
			markPos.AnchorRef.MarkAnchor, markPos.AnchorRef.BaseAnchor, wantMarkOff, wantBaseOff)
	}
}

func TestShapeAppliesGPOSCursiveAttachment(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	node := lookupNodeAt(t, font, 0)
	if got := ot.GPosLookupType(node.LookupType); got != ot.GPosLookupTypeCursive {
		t.Fatalf("lookup[0] type = %d, want cursive", got)
	}
	payload := node.GPosPayload().CursiveFmt1
	if payload == nil {
		t.Skip("cursive lookup is not format 1")
	}
	g1 := ot.GlyphIndex(18)
	g2 := ot.GlyphIndex(19)
	cp1 := otquery.CodePointForGlyph(font, g1)
	cp2 := otquery.CodePointForGlyph(font, g2)
	if cp1 == 0 || cp2 == 0 {
		t.Fatalf("expected cmap mapping for glyphs g18/g19, got cp1=%#U cp2=%#U", cp1, cp2)
	}
	inx, ok := node.Coverage.Match(g1)
	if !ok {
		t.Fatalf("glyph %d not covered by cursive lookup", g1)
	}
	entryOff, exitOff, ok := payload.EntryExitOffsets(inx)
	if !ok {
		t.Fatalf("no entry/exit offsets for coverage index %d", inx)
	}

	got := shapeRunes(t, font, []rune{cp1, cp2}, []otshape.FeatureRange{
		{Feature: ot.T("test"), On: true},
	})
	if len(got) != 2 {
		t.Fatalf("shaped glyph count = %d, want 2", len(got))
	}
	if got[0].GID != g1 || got[1].GID != g2 {
		t.Fatalf("shaped glyphs = [%d %d], want [%d %d]", got[0].GID, got[1].GID, g1, g2)
	}
	follower := got[1].Pos
	if follower.AttachKind != otlayout.AttachCursive {
		t.Fatalf("cursive attach kind = %d, want %d", follower.AttachKind, otlayout.AttachCursive)
	}
	if follower.AttachTo != 0 {
		t.Fatalf("cursive AttachTo = %d, want 0", follower.AttachTo)
	}
	if follower.AnchorRef.CursiveEntry != entryOff || follower.AnchorRef.CursiveExit != exitOff {
		t.Fatalf("unexpected cursive anchor refs for index %d: got entry=%d exit=%d, want entry=%d exit=%d",
			inx, follower.AnchorRef.CursiveEntry, follower.AnchorRef.CursiveExit, entryOff, exitOff)
	}
}

func TestShapeSkipsGPOSCursiveWithIgnoreBaseGlyphs(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font2.otf")
	node := lookupNodeAt(t, font, 0)
	if got := ot.GPosLookupType(node.LookupType); got != ot.GPosLookupTypeCursive {
		t.Fatalf("lookup[0] type = %d, want cursive", got)
	}
	g1 := ot.GlyphIndex(18)
	g2 := ot.GlyphIndex(19)
	cp1 := otquery.CodePointForGlyph(font, g1)
	cp2 := otquery.CodePointForGlyph(font, g2)
	if cp1 == 0 || cp2 == 0 {
		t.Fatalf("expected cmap mapping for glyphs g18/g19, got cp1=%#U cp2=%#U", cp1, cp2)
	}

	got := shapeRunes(t, font, []rune{cp1, cp2}, []otshape.FeatureRange{
		{Feature: ot.T("test"), On: true},
	})
	if len(got) != 2 {
		t.Fatalf("shaped glyph count = %d, want 2", len(got))
	}
	if got[0].GID != g1 || got[1].GID != g2 {
		t.Fatalf("shaped glyphs = [%d %d], want [%d %d]", got[0].GID, got[1].GID, g1, g2)
	}
	if got[1].Pos.AttachKind != otlayout.AttachNone || got[1].Pos.AttachTo != -1 {
		t.Fatalf("expected no attachment for ignore-base-flag fixture, got kind=%d to=%d",
			got[1].Pos.AttachKind, got[1].Pos.AttachTo)
	}
}

func TestShapeHandlesGPOSCursiveMixedEntryExitRecords(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font3.otf")
	node := lookupNodeAt(t, font, 0)
	if got := ot.GPosLookupType(node.LookupType); got != ot.GPosLookupTypeCursive {
		t.Fatalf("lookup[0] type = %d, want cursive", got)
	}
	g18 := ot.GlyphIndex(18)
	g19 := ot.GlyphIndex(19)
	g20 := ot.GlyphIndex(20)
	cp18 := otquery.CodePointForGlyph(font, g18)
	cp19 := otquery.CodePointForGlyph(font, g19)
	cp20 := otquery.CodePointForGlyph(font, g20)
	if cp18 == 0 || cp19 == 0 || cp20 == 0 {
		t.Fatalf("expected cmap mapping for glyphs g18/g19/g20, got cp18=%#U cp19=%#U cp20=%#U", cp18, cp19, cp20)
	}

	got := shapeRunes(t, font, []rune{cp18, cp19, cp20}, []otshape.FeatureRange{
		{Feature: ot.T("test"), On: true},
	})
	if len(got) != 3 {
		t.Fatalf("shaped glyph count = %d, want 3", len(got))
	}
	if got[0].GID != g18 || got[1].GID != g19 || got[2].GID != g20 {
		t.Fatalf("shaped glyphs = [%d %d %d], want [%d %d %d]",
			got[0].GID, got[1].GID, got[2].GID, g18, g19, g20)
	}
	for i := range got {
		if got[i].Pos.AttachKind != otlayout.AttachNone || got[i].Pos.AttachTo != -1 {
			t.Fatalf("expected no attachment on glyph[%d], got kind=%d to=%d",
				i, got[i].Pos.AttachKind, got[i].Pos.AttachTo)
		}
	}
}

func TestShapeAppliesGPOSCursiveWithRangeMasks(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	g18 := ot.GlyphIndex(18)
	g19 := ot.GlyphIndex(19)
	cp18 := otquery.CodePointForGlyph(font, g18)
	cp19 := otquery.CodePointForGlyph(font, g19)
	if cp18 == 0 || cp19 == 0 {
		t.Fatalf("expected cmap mapping for glyphs g18/g19, got cp18=%#U cp19=%#U", cp18, cp19)
	}
	input := []rune{cp18, cp19}

	t.Run("range-on-first-glyph-applies", func(t *testing.T) {
		got := shapeRunes(t, font, input, []otshape.FeatureRange{
			{Feature: ot.T("test"), On: true, Start: 0, End: 1},
		})
		if len(got) != 2 {
			t.Fatalf("shaped glyph count = %d, want 2", len(got))
		}
		if got[1].Pos.AttachKind != otlayout.AttachCursive || got[1].Pos.AttachTo != 0 {
			t.Fatalf("expected cursive attachment from range-on on first glyph, got kind=%d to=%d",
				got[1].Pos.AttachKind, got[1].Pos.AttachTo)
		}
	})

	t.Run("range-on-second-glyph-does-not-apply", func(t *testing.T) {
		got := shapeRunes(t, font, input, []otshape.FeatureRange{
			{Feature: ot.T("test"), On: true, Start: 1, End: 2},
		})
		if len(got) != 2 {
			t.Fatalf("shaped glyph count = %d, want 2", len(got))
		}
		if got[1].Pos.AttachKind != otlayout.AttachNone || got[1].Pos.AttachTo != -1 {
			t.Fatalf("expected no cursive attachment when first glyph is masked off, got kind=%d to=%d",
				got[1].Pos.AttachKind, got[1].Pos.AttachTo)
		}
	})
}

func TestShapeGPOSRangeOverlapLastWins(t *testing.T) {
	font := loadMiniOTFont(t, "gpos3_font1.otf")
	g18 := ot.GlyphIndex(18)
	g19 := ot.GlyphIndex(19)
	cp18 := otquery.CodePointForGlyph(font, g18)
	cp19 := otquery.CodePointForGlyph(font, g19)
	if cp18 == 0 || cp19 == 0 {
		t.Fatalf("expected cmap mapping for glyphs g18/g19, got cp18=%#U cp19=%#U", cp18, cp19)
	}
	input := []rune{cp18, cp19}

	t.Run("last-range-off-disables", func(t *testing.T) {
		got := shapeRunes(t, font, input, []otshape.FeatureRange{
			{Feature: ot.T("test"), On: true, Start: 0, End: 1},
			{Feature: ot.T("test"), On: false, Start: 0, End: 1},
		})
		if got[1].Pos.AttachKind != otlayout.AttachNone || got[1].Pos.AttachTo != -1 {
			t.Fatalf("expected no attachment after overlapping range-off override, got kind=%d to=%d",
				got[1].Pos.AttachKind, got[1].Pos.AttachTo)
		}
	})

	t.Run("last-range-on-enables", func(t *testing.T) {
		got := shapeRunes(t, font, input, []otshape.FeatureRange{
			{Feature: ot.T("test"), On: false, Start: 0, End: 1},
			{Feature: ot.T("test"), On: true, Start: 0, End: 1},
		})
		if got[1].Pos.AttachKind != otlayout.AttachCursive || got[1].Pos.AttachTo != 0 {
			t.Fatalf("expected attachment after overlapping range-on override, got kind=%d to=%d",
				got[1].Pos.AttachKind, got[1].Pos.AttachTo)
		}
	})
}

func shapeRunes(t *testing.T, font *ot.Font, runes []rune, features []otshape.FeatureRange) []otshape.GlyphRecord {
	return shapeRunesWithBoundary(t, font, runes, features, otshape.FlushOnRunBoundary)
}

func shapeRunesWithBoundary(
	t *testing.T,
	font *ot.Font,
	runes []rune,
	features []otshape.FeatureRange,
	boundary otshape.FlushBoundary,
) []otshape.GlyphRecord {
	t.Helper()
	source := strings.NewReader(string(runes))
	sink := &glyphCollector{}
	params := otshape.Params{
		Font:      font,
		Direction: bidi.LeftToRight,
		Script:    language.MustParseScript("Latn"),
		Language:  language.English,
		Features:  features,
	}
	options := otshape.BufferOptions{
		FlushBoundary: boundary,
	}
	engines := []otshape.ShapingEngine{otcore.New()}
	shaper := otshape.NewShaper(engines...)
	err := shaper.Shape(params, source, sink, options)
	if err != nil {
		t.Fatalf("shape failed: %v", err)
	}
	return sink.glyphs
}

func featureByTag(features []otlayout.Feature, tag ot.Tag) otlayout.Feature {
	for _, feat := range features {
		if feat != nil && feat.Tag() == tag {
			return feat
		}
	}
	return nil
}

func lookupNodeForFeatureType(
	t *testing.T,
	font *ot.Font,
	feat otlayout.Feature,
	wantType ot.LayoutTableLookupType,
) (int, *ot.LookupNode) {
	t.Helper()
	graph := font.Layout.GPos.LookupGraph()
	if graph == nil {
		t.Fatalf("font has no concrete GPOS lookup graph")
	}
	for i := 0; i < feat.LookupCount(); i++ {
		lookupInx := feat.LookupIndex(i)
		lookup := graph.Lookup(lookupInx)
		if lookup == nil {
			continue
		}
		for j := 0; j < int(lookup.SubTableCount); j++ {
			node := lookup.Subtable(j)
			if node == nil {
				continue
			}
			if ot.GPosLookupType(node.LookupType) == wantType {
				return lookupInx, node
			}
		}
	}
	t.Fatalf("feature %s has no lookup of type %d", feat.Tag(), wantType)
	return -1, nil
}

func lookupNodeAt(t *testing.T, font *ot.Font, lookupInx int) *ot.LookupNode {
	t.Helper()
	if font == nil || font.Layout.GPos == nil {
		t.Fatalf("font has no GPOS table")
	}
	graph := font.Layout.GPos.LookupGraph()
	if graph == nil {
		t.Fatalf("font has no concrete GPOS lookup graph")
	}
	lookup := graph.Lookup(lookupInx)
	if lookup == nil || lookup.SubTableCount == 0 {
		t.Fatalf("lookup[%d] missing", lookupInx)
	}
	node := lookup.Subtable(0)
	if node == nil {
		t.Fatalf("lookup[%d] subtable[0] missing", lookupInx)
	}
	return node
}

func firstPairFmt1CandidateWithCodepoints(
	font *ot.Font,
	node *ot.LookupNode,
	payload *ot.GPosPairFmt1Payload,
) (g1 ot.GlyphIndex, cp1 rune, g2 ot.GlyphIndex, cp2 rune, rec ot.PairValueRecord, ok bool) {
	limit := maxGlyphCount(font)
	for gid := 1; gid < limit; gid++ {
		first := ot.GlyphIndex(gid)
		row, match := node.Coverage.Match(first)
		if !match || row < 0 || row >= len(payload.PairSets) {
			continue
		}
		cpFirst := otquery.CodePointForGlyph(font, first)
		if cpFirst == 0 {
			continue
		}
		for _, pairRec := range payload.PairSets[row] {
			second := ot.GlyphIndex(pairRec.SecondGlyph)
			cpSecond := otquery.CodePointForGlyph(font, second)
			if cpSecond == 0 {
				continue
			}
			return first, cpFirst, second, cpSecond, pairRec, true
		}
	}
	return 0, 0, 0, 0, ot.PairValueRecord{}, false
}

func firstCoveredGlyphWithCodepoint(font *ot.Font, cov ot.Coverage) (ot.GlyphIndex, rune, int, bool) {
	limit := maxGlyphCount(font)
	for gid := 1; gid < limit; gid++ {
		g := ot.GlyphIndex(gid)
		inx, ok := cov.Match(g)
		if !ok {
			continue
		}
		cp := otquery.CodePointForGlyph(font, g)
		if cp == 0 {
			continue
		}
		return g, cp, inx, true
	}
	return 0, 0, -1, false
}

func maxGlyphCount(font *ot.Font) int {
	if font == nil {
		return ot.MaxGlyphCount
	}
	if maxp := font.Table(ot.T("maxp")); maxp != nil {
		if t := maxp.Self().AsMaxP(); t != nil && t.NumGlyphs > 0 {
			return t.NumGlyphs
		}
	}
	return ot.MaxGlyphCount
}

func valueDelta(vr ot.ValueRecord, format ot.ValueFormat) otlayout.PosItem {
	var p otlayout.PosItem
	if format&ot.ValueFormatXPlacement != 0 {
		p.XOffset = int32(vr.XPlacement)
	}
	if format&ot.ValueFormatYPlacement != 0 {
		p.YOffset = int32(vr.YPlacement)
	}
	if format&ot.ValueFormatXAdvance != 0 {
		p.XAdvance = int32(vr.XAdvance)
	}
	if format&ot.ValueFormatYAdvance != 0 {
		p.YAdvance = int32(vr.YAdvance)
	}
	return p
}

func assertPosDelta(t *testing.T, got otlayout.PosItem, want otlayout.PosItem) {
	t.Helper()
	if got.XAdvance != want.XAdvance || got.YAdvance != want.YAdvance || got.XOffset != want.XOffset || got.YOffset != want.YOffset {
		t.Fatalf("unexpected pos delta: got {xa=%d ya=%d xo=%d yo=%d}, want {xa=%d ya=%d xo=%d yo=%d}",
			got.XAdvance, got.YAdvance, got.XOffset, got.YOffset,
			want.XAdvance, want.YAdvance, want.XOffset, want.YOffset)
	}
}

func loadRootOTFont(t *testing.T, filename string) *ot.Font {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", filename)
	sf, err := fontload.LoadOpenTypeFont(path)
	if err != nil {
		t.Fatalf("load test font %s: %v", path, err)
	}
	otf, err := ot.Parse(sf.Binary)
	if err != nil {
		t.Fatalf("parse test font %s: %v", path, err)
	}
	return otf
}

func loadMiniOTFont(t *testing.T, filename string) *ot.Font {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fonttools-tests", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mini font %s: %v", path, err)
	}
	otf, err := ot.Parse(data, ot.IsTestfont)
	if err != nil {
		t.Fatalf("parse mini font %s: %v", path, err)
	}
	return otf
}
