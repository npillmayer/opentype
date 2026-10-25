package otshape

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

func benchmarkFont(b *testing.B, file string) *ot.Font {
	b.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "fonttools", file))
	if err != nil {
		b.Fatal(err)
	}
	font, err := ot.Parse(data, ot.IsTestfont)
	if err != nil {
		b.Fatal(err)
	}
	return font
}

type benchmarkCoverage ot.GlyphIndex

func (c benchmarkCoverage) Match(g ot.GlyphIndex) (int, bool) { return 0, g == ot.GlyphIndex(c) }
func (benchmarkCoverage) ByteSize() int                       { return 0 }

// Fonts/plans and lazy lookup nodes are prepared outside the timed region.
// Each iteration includes fresh buffers and the complete lookup application.
func BenchmarkLookupApplication(b *testing.B) {
	for _, mode := range []string{"miss", "single", "multiple", "ligature", "context"} {
		for _, isolated := range []bool{false, true} {
			name := mode + "/whole"
			if isolated {
				name = mode + "/syllables"
			}
			b.Run(name, func(b *testing.B) {
				font := benchmarkFont(b, "gsub_context1_lookupflag_f1.otf")
				graph := font.Layout.GSub.LookupGraph()
				install := func(index int, typ ot.LayoutTableLookupType, glyph ot.GlyphIndex, payload *ot.GSubLookupPayload) {
					l := graph.Lookup(index)
					n := l.Subtable(0)
					l.Type, l.Flag = typ, 0
					*n = ot.LookupNode{LookupType: typ, Format: 1, Coverage: ot.Coverage{GlyphRange: benchmarkCoverage(glyph)}, GSub: payload}
				}
				input := make(otlayout.GlyphBuffer, 128)
				for i := range input {
					input[i] = 10
				}
				wantLen := len(input)
				switch mode {
				case "miss", "single":
					glyph := ot.GlyphIndex(10)
					if mode == "miss" {
						glyph = 99
					}
					install(0, ot.GSubLookupTypeSingle, glyph, &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}})
				case "multiple":
					install(0, ot.GSubLookupTypeMultiple, 10, &ot.GSubLookupPayload{MultipleFmt1: &ot.GSubMultipleFmt1Payload{Sequences: [][]ot.GlyphIndex{{11, 12}}}})
					wantLen *= 2
				case "ligature":
					install(0, ot.GSubLookupTypeLigature, 10, &ot.GSubLookupPayload{LigatureFmt1: &ot.GSubLigatureFmt1Payload{LigatureSets: [][]ot.GSubLigatureRule{{{Components: []ot.GlyphIndex{10}, Ligature: 11}}}}})
					wantLen /= 2
				case "context":
					install(1, ot.GSubLookupTypeSingle, 10, &ot.GSubLookupPayload{SingleFmt1: &ot.GSubSingleFmt1Payload{DeltaGlyphID: 1}})
					install(0, ot.GSubLookupTypeContext, 10, &ot.GSubLookupPayload{ContextFmt3: &ot.GSubContextFmt3Payload{InputCoverages: []ot.Coverage{{GlyphRange: benchmarkCoverage(10)}}, Records: []ot.SequenceLookupRecord{{LookupListIndex: 1}}}})
					graph.Lookup(0).Subtable(0).Format = 3
				}
				flags := lookupRunFlags(0)
				if isolated {
					flags = lookupPerSyllable
				}
				ops := []lookupOp{{LookupIndex: 0, FeatureTag: ot.T("test"), Flags: flags}}
				pl := &plan{font: font, Masks: maskLayout{ByFeature: map[ot.Tag]maskSpec{}}}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					run := newRunBuffer(0)
					run.Glyphs = append(otlayout.GlyphBuffer(nil), input...)
					run.Clusters = make([]uint32, len(input))
					run.Syllables = make([]uint16, len(input))
					run.Masks = make([]uint32, len(input))
					for j := range input {
						run.Clusters[j] = uint32(j / 8)
						run.Syllables[j] = uint16(j/8 + 1)
					}
					exec := &planExecutor{}
					exec.acquireBuffer(run)
					err := exec.applyLookups(pl, planGSUB, ops)
					exec.releaseBuffer()
					if err != nil {
						b.Fatal(err)
					}
					if run.Len() != wantLen {
						b.Fatalf("got %d glyphs want %d", run.Len(), wantLen)
					}
				}
			})
		}
	}
}

type benchmarkSink struct{ count int }

func (s *benchmarkSink) WriteGlyph(GlyphRecord) error { s.count++; return nil }

func BenchmarkShapeOrdinary(b *testing.B) {
	font := benchmarkFont(b, "gpos3_font1.otf")
	params := standardParams(font)
	shaper := NewShaper(&hookProbeShaper{})
	input := strings.Repeat(string([]rune{18, 19}), 64)
	opts := BufferOptions{FlushBoundary: FlushOnRunBoundary, HighWatermark: 256, LowWatermark: 128, MaxBuffer: 256}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink := &benchmarkSink{}
		if err := shaper.Shape(params, strings.NewReader(input), sink, opts); err != nil {
			b.Fatal(err)
		}
		if sink.count != 128 {
			b.Fatalf("got %d glyphs", sink.count)
		}
	}
}

// Isolates positional replacement cost without the old dispatcher's duplicate
// edit bug, so both revisions perform the same single structural replacement.
func BenchmarkPositionReplacement(b *testing.B) {
	for _, n := range []int{128, 1024} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			pos := otlayout.NewPosBuffer(n)
			edit := &otlayout.EditSpan{From: n / 2, To: n/2 + 1, Len: 2}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out := pos.ApplyEdit(edit)
				if len(out) != n+1 {
					b.Fatal("wrong position count")
				}
			}
		})
	}
}
