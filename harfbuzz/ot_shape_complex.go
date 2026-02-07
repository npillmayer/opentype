package harfbuzz

import (
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
)

type ZeroWidthMarksMode uint8

const (
	ZeroWidthMarksNone ZeroWidthMarksMode = iota
	ZeroWidthMarksByGDEFEarly
	ZeroWidthMarksByGDEFLate
)

const (
	zeroWidthMarksNone        = ZeroWidthMarksNone
	zeroWidthMarksByGdefEarly = ZeroWidthMarksByGDEFEarly
	zeroWidthMarksByGdefLate  = ZeroWidthMarksByGDEFLate
)

func (planner *otShapePlanner) categorizeComplex() ShapingEngine {
	return resolveShaperForContext(SelectionContext{
		Script:       planner.props.Script,
		Direction:    planner.props.Direction,
		ChosenScript: planner.map_.chosenScript,
		FoundScript:  planner.map_.foundScript,
	})
}

// zero byte struct providing no-ops, used to reduced boilerplate
type complexShaperNil struct{}

func (complexShaperNil) GposTag() tables.Tag { return 0 }

func (complexShaperNil) CollectFeatures(plan FeaturePlanner, script language.Script) {}
func (complexShaperNil) OverrideFeatures(plan FeaturePlanner)                        {}
func (complexShaperNil) InitPlan(plan PlanContext)                                   {}
func (complexShaperNil) Decompose(c NormalizeContext, ab rune) (a, b rune, ok bool) {
	return c.DecomposeUnicode(ab)
}

func (complexShaperNil) Compose(c NormalizeContext, a, b rune) (ab rune, ok bool) {
	return c.ComposeUnicode(a, b)
}
func (complexShaperNil) PreprocessText(*Buffer, *Font) {}
func (complexShaperNil) PostprocessGlyphs(*Buffer, *Font) {
}
func (complexShaperNil) SetupMasks(*Buffer, *Font, language.Script) {
}
func (complexShaperNil) ReorderMarks(*Buffer, int, int) {}

type complexShaperDefault struct {
	complexShaperNil

	/* if true, no mark advance zeroing / fallback positioning.
	 * Dumbest shaper ever, basically. */
	dumb        bool
	disableNorm bool
}

var _ ShapingEngine = complexShaperDefault{}

func (cs complexShaperDefault) Name() string {
	return "default"
}

func (cs complexShaperDefault) Match(SelectionContext) int {
	return 0
}

func (cs complexShaperDefault) New() ShapingEngine {
	return cs
}

func (cs complexShaperDefault) MarksBehavior() (ZeroWidthMarksMode, bool) {
	if cs.dumb {
		return zeroWidthMarksNone, false
	}
	return zeroWidthMarksByGdefLate, true
}

func (cs complexShaperDefault) NormalizationPreference() NormalizationMode {
	if cs.disableNorm {
		return nmNone
	}
	return nmDefault
}

func syllabicInsertDottedCircles(font *Font, buffer *Buffer, brokenSyllableType,
	dottedcircleCategory uint8, rephaCategory, dottedCirclePosition int,
) bool {
	if (buffer.Flags & DoNotinsertDottedCircle) != 0 {
		return false
	}

	if (buffer.scratchFlags & bsfHasBrokenSyllable) == 0 {
		return false
	}

	dottedcircleGlyph, ok := font.face.NominalGlyph(0x25CC)
	if !ok {
		return false
	}

	dottedcircle := GlyphInfo{
		Glyph:           dottedcircleGlyph,
		complexCategory: dottedcircleCategory,
	}

	if dottedCirclePosition != -1 {
		dottedcircle.complexAux = uint8(dottedCirclePosition)
	}

	buffer.clearOutput()

	buffer.idx = 0
	var lastSyllable uint8
	for buffer.idx < len(buffer.Info) {
		syllable := buffer.cur(0).syllable
		if lastSyllable != syllable && (syllable&0x0F) == brokenSyllableType {
			lastSyllable = syllable

			ginfo := dottedcircle
			ginfo.Cluster = buffer.cur(0).Cluster
			ginfo.Mask = buffer.cur(0).Mask
			ginfo.syllable = buffer.cur(0).syllable

			/* Insert dottedcircle after possible Repha. */
			if rephaCategory != -1 {
				for buffer.idx < len(buffer.Info) &&
					lastSyllable == buffer.cur(0).syllable &&
					buffer.cur(0).complexCategory == uint8(rephaCategory) {
					buffer.nextGlyph()
				}
			}
			buffer.outInfo = append(buffer.outInfo, ginfo)
		} else {
			buffer.nextGlyph()
		}
	}
	buffer.swapBuffers()
	return true
}
