package harfbuzz

import (
	"fmt"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
)

// Support functions for OpenType shaping related queries.
// ported from src/hb-ot-shape.cc Copyright © 2009,2010  Red Hat, Inc. 2010,2011,2012  Google, Inc. Behdad Esfahbod

/*
 * GSUB/GPOS feature query and enumeration interface
 */

const (
	// Special value for script index indicating unsupported script.
	NoScriptIndex = 0xFFFF
	// Special value for feature index indicating unsupported feature.
	NoFeatureIndex = 0xFFFF
	// Special value for language index indicating default or unsupported language.
	DefaultLanguageIndex = 0xFFFF
	// Special value for variations index indicating unsupported variation.
	noVariationsIndex = -1
)

type otShapePlanner struct {
	shaper                        ShapingEngine
	props                         SegmentProperties
	tables                        *font.Font // also used by the map builders
	map_                          otMapBuilder
	scriptZeroMarks               bool
	scriptFallbackMarkPositioning bool
}

var _ FeaturePlanner = (*otShapePlanner)(nil)
var _ ResolvedFeaturePlanner = (*otShapePlanner)(nil)

func (planner *otShapePlanner) EnableFeature(tag tables.Tag) {
	planner.map_.EnableFeature(tag)
}

func (planner *otShapePlanner) AddFeatureExt(tag tables.Tag, flags FeatureFlags, value uint32) {
	planner.map_.AddFeatureExt(tag, flags, value)
}

func (planner *otShapePlanner) EnableFeatureExt(tag tables.Tag, flags FeatureFlags, value uint32) {
	planner.map_.EnableFeatureExt(tag, flags, value)
}

func (planner *otShapePlanner) AddGSUBPause(fn GSUBPauseFunc) {
	planner.map_.AddGSUBPause(fn)
}

func (planner *otShapePlanner) AddGSUBPauseBefore(tag tables.Tag, fn GSUBPauseFunc) bool {
	return planner.map_.AddGSUBPauseBefore(tag, fn)
}

func (planner *otShapePlanner) AddGSUBPauseAfter(tag tables.Tag, fn GSUBPauseFunc) bool {
	return planner.map_.AddGSUBPauseAfter(tag, fn)
}

func (planner *otShapePlanner) HasFeature(tag tables.Tag) bool {
	return planner.map_.HasFeature(tag)
}

func newOtShapePlanner(tables *font.Font, props SegmentProperties) *otShapePlanner {
	var out otShapePlanner
	out.props = props
	out.tables = tables
	out.map_ = newOtMapBuilder(tables, props)

	out.shaper = out.selectShaper()

	zwm, fb := shaperMarksBehavior(out.shaper)
	out.scriptZeroMarks = zwm != zeroWidthMarksNone
	out.scriptFallbackMarkPositioning = fb
	return &out
}

func (planner *otShapePlanner) compile(plan *otShapePlan, key otShapePlanKey) {
	plan.props = planner.props
	plan.shaper = planner.shaper
	planner.map_.compile(&plan.map_, key, func(view ResolvedFeatureView) {
		shaperPostResolveFeatures(planner.shaper, planner, view, planner.props.Script)
	})

	plan.fracMask = plan.map_.getMask1(ot.NewTag('f', 'r', 'a', 'c'))
	plan.numrMask = plan.map_.getMask1(ot.NewTag('n', 'u', 'm', 'r'))
	plan.dnomMask = plan.map_.getMask1(ot.NewTag('d', 'n', 'o', 'm'))
	plan.hasFrac = plan.fracMask != 0 || (plan.numrMask != 0 && plan.dnomMask != 0)

	plan.rtlmMask = plan.map_.getMask1(ot.NewTag('r', 't', 'l', 'm'))
	plan.hasVert = plan.map_.getMask1(ot.NewTag('v', 'e', 'r', 't')) != 0

	gposTag := shaperGposTag(plan.shaper)
	disableGpos := gposTag != 0 && gposTag != plan.map_.chosenScript[1]

	// Decide who provides glyph classes. GDEF or Unicode.
	if planner.tables.GDEF.GlyphClassDef == nil {
		plan.fallbackGlyphClasses = true
	}

	// Decide who does positioning. OpenType GPOS-only.
	hasGPOS := !disableGpos && planner.tables.GPOS.Lookups != nil

	if hasGPOS {
		plan.applyGpos = true
	}

	plan.zeroMarks = planner.scriptZeroMarks
	plan.hasGposMark = plan.map_.getMask1(ot.NewTag('m', 'a', 'r', 'k')) != 0

	plan.adjustMarkPositioningWhenZeroing = !plan.applyGpos

	plan.fallbackMarkPositioning = plan.adjustMarkPositioningWhenZeroing && planner.scriptFallbackMarkPositioning
}

type otShapePlan struct {
	shaper ShapingEngine
	props  SegmentProperties

	map_ otMap

	fracMask GlyphMask
	numrMask GlyphMask
	dnomMask GlyphMask
	rtlmMask GlyphMask

	hasFrac                          bool
	hasVert                          bool
	hasGposMark                      bool
	zeroMarks                        bool
	fallbackGlyphClasses             bool
	fallbackMarkPositioning          bool
	adjustMarkPositioningWhenZeroing bool

	applyGpos bool
}

var _ PlanContext = (*otShapePlan)(nil)

func (sp *otShapePlan) Script() language.Script {
	return sp.props.Script
}

func (sp *otShapePlan) Direction() Direction {
	return sp.props.Direction
}

func (sp *otShapePlan) FeatureMask1(tag tables.Tag) GlyphMask {
	return sp.map_.getMask1(tag)
}

func (sp *otShapePlan) FeatureNeedsFallback(tag tables.Tag) bool {
	return sp.map_.needsFallback(tag)
}

func (sp *otShapePlan) init0(tables *font.Font, props SegmentProperties, userFeatures []Feature, otKey otShapePlanKey) {
	planner := newOtShapePlanner(tables, props)

	planner.CollectFeatures(userFeatures)

	planner.compile(sp, otKey)

	shaperInitPlan(sp.shaper, sp)
}

func (sp *otShapePlan) substitute(font *Font, buffer *Buffer) {
	sp.map_.substitute(sp, font, buffer)
}

func (sp *otShapePlan) position(font *Font, buffer *Buffer) {
	if sp.applyGpos {
		sp.map_.position(sp, font, buffer)
	}
}

var (
	commonFeatures = [...]otMapFeature{
		{ot.NewTag('a', 'b', 'v', 'm'), ffGLOBAL},
		{ot.NewTag('b', 'l', 'w', 'm'), ffGLOBAL},
		{ot.NewTag('c', 'c', 'm', 'p'), ffGLOBAL},
		{ot.NewTag('l', 'o', 'c', 'l'), ffGLOBAL},
		{ot.NewTag('m', 'a', 'r', 'k'), ffGlobalManualJoiners},
		{ot.NewTag('m', 'k', 'm', 'k'), ffGlobalManualJoiners},
		{ot.NewTag('r', 'l', 'i', 'g'), ffGLOBAL},
	}

	horizontalFeatures = [...]otMapFeature{
		{ot.NewTag('c', 'a', 'l', 't'), ffGLOBAL},
		{ot.NewTag('c', 'l', 'i', 'g'), ffGLOBAL},
		{ot.NewTag('c', 'u', 'r', 's'), ffGLOBAL},
		{ot.NewTag('d', 'i', 's', 't'), ffGLOBAL},
		{ot.NewTag('k', 'e', 'r', 'n'), ffGLOBAL},
		{ot.NewTag('l', 'i', 'g', 'a'), ffGLOBAL},
		{ot.NewTag('r', 'c', 'l', 't'), ffGLOBAL},
	}
)

func (planner *otShapePlanner) CollectFeatures(userFeatures []Feature) {
	map_ := &planner.map_

	map_.EnableFeature(ot.NewTag('r', 'v', 'r', 'n'))
	map_.AddGSUBPause(nil)

	switch planner.props.Direction {
	case LeftToRight:
		map_.EnableFeature(ot.NewTag('l', 't', 'r', 'a'))
		map_.EnableFeature(ot.NewTag('l', 't', 'r', 'm'))
	case RightToLeft:
		map_.EnableFeature(ot.NewTag('r', 't', 'l', 'a'))
		map_.addFeature(ot.NewTag('r', 't', 'l', 'm'))
	}

	/* Automatic fractions. */
	map_.addFeature(ot.NewTag('f', 'r', 'a', 'c'))
	map_.addFeature(ot.NewTag('n', 'u', 'm', 'r'))
	map_.addFeature(ot.NewTag('d', 'n', 'o', 'm'))

	/* Random! */
	map_.EnableFeatureExt(ot.NewTag('r', 'a', 'n', 'd'), ffRandom, otMapMaxValue)

	map_.EnableFeature(ot.NewTag('H', 'a', 'r', 'f')) /* Considered required. */
	map_.EnableFeature(ot.NewTag('H', 'A', 'R', 'F')) /* Considered discretionary. */

	shaperCollectFeatures(planner.shaper, planner, planner.props.Script)

	map_.EnableFeature(ot.NewTag('B', 'u', 'z', 'z')) /* Considered required. */
	map_.EnableFeature(ot.NewTag('B', 'U', 'Z', 'Z')) /* Considered discretionary. */

	for _, feat := range commonFeatures {
		map_.AddFeatureExt(feat.tag, feat.flags, 1)
	}

	if planner.props.Direction.isHorizontal() {
		for _, feat := range horizontalFeatures {
			map_.AddFeatureExt(feat.tag, feat.flags, 1)
		}
	} else {
		/* We really want to find a 'vert' feature if there's any in the font, no
		 * matter which script/langsys it is listed (or not) under.
		 * See various bugs referenced from:
		 * https://github.com/harfbuzz/harfbuzz/issues/63 */
		map_.EnableFeatureExt(ot.NewTag('v', 'e', 'r', 't'), ffGlobalSearch, 1)
	}

	for _, f := range userFeatures {
		ftag := ffNone
		if f.Start == FeatureGlobalStart && f.End == FeatureGlobalEnd {
			ftag = ffGLOBAL
		}
		map_.AddFeatureExt(f.Tag, ftag, f.Value)
	}

	shaperOverrideFeatures(planner.shaper, planner)
}

/*
 * shaper
 */

type otContext struct {
	plan         *otShapePlan
	font         *Font
	buffer       *Buffer
	userFeatures []Feature

	// transient stuff
	targetDirection Direction
}

/* Main shaper */

/*
 * Substitute
 */

func vertCharFor(u rune) rune {
	switch u >> 8 {
	case 0x20:
		switch u {
		case 0x2013:
			return 0xfe32 // EN DASH
		case 0x2014:
			return 0xfe31 // EM DASH
		case 0x2025:
			return 0xfe30 // TWO DOT LEADER
		case 0x2026:
			return 0xfe19 // HORIZONTAL ELLIPSIS
		}
	case 0x30:
		switch u {
		case 0x3001:
			return 0xfe11 // IDEOGRAPHIC COMMA
		case 0x3002:
			return 0xfe12 // IDEOGRAPHIC FULL STOP
		case 0x3008:
			return 0xfe3f // LEFT ANGLE BRACKET
		case 0x3009:
			return 0xfe40 // RIGHT ANGLE BRACKET
		case 0x300a:
			return 0xfe3d // LEFT DOUBLE ANGLE BRACKET
		case 0x300b:
			return 0xfe3e // RIGHT DOUBLE ANGLE BRACKET
		case 0x300c:
			return 0xfe41 // LEFT CORNER BRACKET
		case 0x300d:
			return 0xfe42 // RIGHT CORNER BRACKET
		case 0x300e:
			return 0xfe43 // LEFT WHITE CORNER BRACKET
		case 0x300f:
			return 0xfe44 // RIGHT WHITE CORNER BRACKET
		case 0x3010:
			return 0xfe3b // LEFT BLACK LENTICULAR BRACKET
		case 0x3011:
			return 0xfe3c // RIGHT BLACK LENTICULAR BRACKET
		case 0x3014:
			return 0xfe39 // LEFT TORTOISE SHELL BRACKET
		case 0x3015:
			return 0xfe3a // RIGHT TORTOISE SHELL BRACKET
		case 0x3016:
			return 0xfe17 // LEFT WHITE LENTICULAR BRACKET
		case 0x3017:
			return 0xfe18 // RIGHT WHITE LENTICULAR BRACKET
		}
	case 0xfe:
		switch u {
		case 0xfe4f:
			return 0xfe34 // WAVY LOW LINE
		}
	case 0xff:
		switch u {
		case 0xff01:
			return 0xfe15 // FULLWIDTH EXCLAMATION MARK
		case 0xff08:
			return 0xfe35 // FULLWIDTH LEFT PARENTHESIS
		case 0xff09:
			return 0xfe36 // FULLWIDTH RIGHT PARENTHESIS
		case 0xff0c:
			return 0xfe10 // FULLWIDTH COMMA
		case 0xff1a:
			return 0xfe13 // FULLWIDTH COLON
		case 0xff1b:
			return 0xfe14 // FULLWIDTH SEMICOLON
		case 0xff1f:
			return 0xfe16 // FULLWIDTH QUESTION MARK
		case 0xff3b:
			return 0xfe47 // FULLWIDTH LEFT SQUARE BRACKET
		case 0xff3d:
			return 0xfe48 // FULLWIDTH RIGHT SQUARE BRACKET
		case 0xff3f:
			return 0xfe33 // FULLWIDTH LOW LINE
		case 0xff5b:
			return 0xfe37 // FULLWIDTH LEFT CURLY BRACKET
		case 0xff5d:
			return 0xfe38 // FULLWIDTH RIGHT CURLY BRACKET
		}
	}

	return u
}

func (c *otContext) otRotateChars() {
	info := c.buffer.Info

	if c.targetDirection.isBackward() {
		rtlmMask := c.plan.rtlmMask

		for i := range info {
			codepoint := uni.mirroring(info[i].codepoint)
			if codepoint != info[i].codepoint && c.font.hasGlyph(codepoint) {
				info[i].codepoint = codepoint
			} else {
				info[i].Mask |= rtlmMask
			}
		}
	}

	if c.targetDirection.isVertical() && !c.plan.hasVert {
		for i := range info {
			codepoint := vertCharFor(info[i].codepoint)
			if codepoint != info[i].codepoint && c.font.hasGlyph(codepoint) {
				info[i].codepoint = codepoint
			}
		}
	}
}

func (c *otContext) setupMasksFraction() {
	if c.buffer.scratchFlags&bsfHasNonASCII == 0 || !c.plan.hasFrac {
		return
	}

	buffer := c.buffer

	var preMask, postMask GlyphMask
	if buffer.Props.Direction.isForward() {
		preMask = c.plan.numrMask | c.plan.fracMask
		postMask = c.plan.fracMask | c.plan.dnomMask
	} else {
		preMask = c.plan.fracMask | c.plan.dnomMask
		postMask = c.plan.numrMask | c.plan.fracMask
	}

	count := len(buffer.Info)
	info := buffer.Info
	for i := 0; i < count; i++ {
		if info[i].codepoint == 0x2044 /* FRACTION SLASH */ {
			start, end := i, i+1
			for start != 0 && info[start-1].unicode.generalCategory() == decimalNumber {
				start--
			}
			for end < count && info[end].unicode.generalCategory() == decimalNumber {
				end++
			}

			buffer.unsafeToBreak(start, end)

			for j := start; j < i; j++ {
				info[j].Mask |= preMask
			}
			info[i].Mask |= c.plan.fracMask
			for j := i + 1; j < end; j++ {
				info[j].Mask |= postMask
			}

			i = end - 1
		}
	}
}

func (c *otContext) initializeMasks() {
	c.buffer.resetMasks(c.plan.map_.globalMask)
}

func (c *otContext) SetupMasks() {
	map_ := &c.plan.map_
	buffer := c.buffer

	c.setupMasksFraction()

	shaperPrepareGSUB(c.plan.shaper, buffer, c.font, c.plan.props.Script)

	shaperSetupMasks(c.plan.shaper, buffer, c.font, c.plan.props.Script)

	for _, feature := range c.userFeatures {
		if !(feature.Start == FeatureGlobalStart && feature.End == FeatureGlobalEnd) {
			mask, shift := map_.getMask(feature.Tag)
			buffer.setMasks(feature.Value<<shift, mask, feature.Start, feature.End)
		}
	}
}

func zeroWidthDefaultIgnorables(buffer *Buffer) {
	if buffer.scratchFlags&bsfHasDefaultIgnorables == 0 ||
		buffer.Flags&PreserveDefaultIgnorables != 0 ||
		buffer.Flags&RemoveDefaultIgnorables != 0 {
		return
	}

	pos := buffer.Pos
	for i, info := range buffer.Info {
		if info.isDefaultIgnorable() {
			pos[i].XAdvance, pos[i].YAdvance, pos[i].XOffset, pos[i].YOffset = 0, 0, 0, 0
		}
	}
}

func hideDefaultIgnorables(buffer *Buffer, font *Font) {
	if buffer.scratchFlags&bsfHasDefaultIgnorables == 0 ||
		buffer.Flags&PreserveDefaultIgnorables != 0 {
		return
	}

	info := buffer.Info

	var (
		invisible = buffer.Invisible
		ok        bool
	)
	if invisible == 0 {
		invisible, ok = font.face.NominalGlyph(' ')
	}
	if buffer.Flags&RemoveDefaultIgnorables == 0 && ok {
		// replace default-ignorables with a zero-advance invisible glyph.
		for i := range info {
			if info[i].isDefaultIgnorable() {
				info[i].Glyph = invisible
			}
		}
	} else {
		otLayoutDeleteGlyphsInplace(buffer, (*GlyphInfo).isDefaultIgnorable)
	}
}

// use unicodeProp to assign a class
func synthesizeGlyphClasses(buffer *Buffer) {
	info := buffer.Info
	for i := range info {
		/* Never mark default-ignorables as marks.
		 * They won't get in the way of lookups anyway,
		 * but having them as mark will cause them to be skipped
		 * over if the lookup-flag says so, but at least for the
		 * Mongolian variation selectors, looks like Uniscribe
		 * marks them as non-mark.  Some Mongolian fonts without
		 * GDEF rely on this.  Another notable character that
		 * this applies to is COMBINING GRAPHEME JOINER. */
		class := tables.GPMark
		if info[i].unicode.generalCategory() != nonSpacingMark || info[i].isDefaultIgnorable() {
			class = tables.GPBaseGlyph
		}

		info[i].glyphProps = class
	}
}

func (c *otContext) substituteBeforePosition() {
	buffer := c.buffer

	// substituteDefault : normalize and sets Glyph
	c.otRotateChars()

	otShapeNormalize(c.plan, buffer, c.font)

	c.SetupMasks()

	// this is unfortunate to go here, but necessary...
	if c.plan.fallbackMarkPositioning {
		fallbackMarkPositionRecategorizeMarks(buffer)
	}

	if debugMode {
		fmt.Println("BEFORE SUBSTITUTE:", c.buffer.Info)
	}

	// substitutePan : glyph fields are now set up ...
	// ... apply complex substitution from font

	layoutSubstituteStart(c.font, buffer)

	if c.plan.fallbackGlyphClasses {
		synthesizeGlyphClasses(c.buffer)
	}

	c.plan.substitute(c.font, buffer)
}

func (c *otContext) substituteAfterPosition() {
	hideDefaultIgnorables(c.buffer, c.font)

	if debugMode {
		fmt.Println("POSTPROCESS glyphs start")
	}
	shaperPostprocessGlyphs(c.plan.shaper, c.buffer, c.font)
	if debugMode {
		fmt.Println("POSTPROCESS glyphs end ")
	}
}

/*
 * Position
 */

func zeroMarkWidthsByGdef(buffer *Buffer, adjustOffsets bool) {
	for i, inf := range buffer.Info {
		if inf.isMark() {
			pos := &buffer.Pos[i]
			if adjustOffsets { // adjustMarkOffsets
				pos.XOffset -= pos.XAdvance
				pos.YOffset -= pos.YAdvance
			}
			// zeroMarkWidth
			pos.XAdvance = 0
			pos.YAdvance = 0
		}
	}
}

// override Pos array with default values
func (c *otContext) positionDefault() {
	direction := c.buffer.Props.Direction
	info := c.buffer.Info
	pos := c.buffer.Pos

	if direction.isHorizontal() {
		for i, inf := range info {
			pos[i].XAdvance, pos[i].YAdvance = c.font.GlyphHAdvance(inf.Glyph), 0
			pos[i].XOffset, pos[i].YOffset = c.font.subtractGlyphHOrigin(inf.Glyph, 0, 0)
		}
	} else {
		for i, inf := range info {
			pos[i].XAdvance, pos[i].YAdvance = 0, c.font.getGlyphVAdvance(inf.Glyph)
			pos[i].XOffset, pos[i].YOffset = c.font.subtractGlyphVOrigin(inf.Glyph, 0, 0)
		}
	}
	if c.buffer.scratchFlags&bsfHasSpaceFallback != 0 {
		fallbackSpaces(c.font, c.buffer)
	}
}

func (c *otContext) positionComplex() {
	info := c.buffer.Info
	pos := c.buffer.Pos

	/* If the font has no GPOS and direction is forward, then when
	* zeroing mark widths, we shift the mark with it, such that the
	* mark is positioned hanging over the previous glyph.  When
	* direction is backward we don't shift and it will end up
	* hanging over the next glyph after the final reordering.
	*
	* Note: If fallback positioning happens, we don't care about
	* this as it will be overridden. */
	adjustOffsetsWhenZeroing := c.plan.adjustMarkPositioningWhenZeroing && c.buffer.Props.Direction.isForward()

	// we change glyph origin to what GPOS expects (horizontal), apply GPOS, change it back.

	for i, inf := range info {
		pos[i].XOffset, pos[i].YOffset = c.font.addGlyphHOrigin(inf.Glyph, pos[i].XOffset, pos[i].YOffset)
	}

	otLayoutPositionStart(c.font, c.buffer)
	markBehavior, _ := shaperMarksBehavior(c.plan.shaper)

	if c.plan.zeroMarks {
		if markBehavior == zeroWidthMarksByGdefEarly {
			zeroMarkWidthsByGdef(c.buffer, adjustOffsetsWhenZeroing)
		}
	}

	c.plan.position(c.font, c.buffer) // apply GPOS

	if c.plan.zeroMarks {
		if markBehavior == zeroWidthMarksByGdefLate {
			zeroMarkWidthsByGdef(c.buffer, adjustOffsetsWhenZeroing)
		}
	}

	// finish off. Has to follow a certain order.
	zeroWidthDefaultIgnorables(c.buffer)
	otLayoutPositionFinishOffsets(c.font, c.buffer)

	for i, inf := range info {
		pos[i].XOffset, pos[i].YOffset = c.font.subtractGlyphHOrigin(inf.Glyph, pos[i].XOffset, pos[i].YOffset)
	}

	if c.plan.fallbackMarkPositioning {
		fallbackMarkPosition(c.plan, c.font, c.buffer, adjustOffsetsWhenZeroing)
	}
}

func (c *otContext) position() {
	c.buffer.clearPositions()

	c.positionDefault()

	if debugMode {
		fmt.Println("AFTER DEFAULT POSITION", c.buffer.Pos)
	}

	c.positionComplex()

	if c.buffer.Props.Direction.isBackward() {
		c.buffer.Reverse()
	}
}

/* Propagate cluster-level glyph flags to be the same on all cluster glyphs.
 * Simplifies using them. */
func propagateFlags(buffer *Buffer) {
	if buffer.scratchFlags&bsfHasGlyphFlags == 0 {
		return
	}

	/* If we are producing SAFE_TO_INSERT_TATWEEL, then do two things:
	 *
	 * - If the places that the Arabic shaper marked as SAFE_TO_INSERT_TATWEEL,
	 *   are UNSAFE_TO_BREAK, then clear the SAFE_TO_INSERT_TATWEEL,
	 * - Any place that is SAFE_TO_INSERT_TATWEEL, is also now UNSAFE_TO_BREAK.
	 *
	 * We couldn't make this interaction earlier. It has to be done here.
	 */
	flipTatweel := buffer.Flags&ProduceSafeToInsertTatweel != 0

	clearConcat := (buffer.Flags & ProduceUnsafeToConcat) == 0

	info := buffer.Info

	iter, count := buffer.clusterIterator()
	for start, end := iter.next(); start < count; start, end = iter.next() {
		var mask uint32
		for i := start; i < end; i++ {
			mask |= info[i].Mask & glyphFlagDefined
		}

		if flipTatweel {
			if mask&GlyphUnsafeToBreak != 0 {
				mask &= ^GlyphSafeToInsertTatweel
			}
			if mask&GlyphSafeToInsertTatweel != 0 {
				mask |= GlyphUnsafeToBreak | GlyphUnsafeToConcat
			}

		}

		if clearConcat {
			mask &= ^GlyphUnsafeToConcat
		}

		for i := start; i < end; i++ {
			info[i].Mask = mask
		}
	}
}

// shaperOpentype is the main shaper of this library.
// It handles complex language and Opentype layout features found in fonts.
type shaperOpentype struct {
	tables *font.Font
	plan   otShapePlan
	key    otShapePlanKey
}

type otShapePlanKey = [2]int // -1 for not found

func (sp *shaperOpentype) init(tables *font.Font, coords []tables.Coord) {
	sp.plan = otShapePlan{}
	sp.key = otShapePlanKey{
		0: tables.GSUB.FindVariationIndex(coords),
		1: tables.GPOS.FindVariationIndex(coords),
	}
	sp.tables = tables
}

func (sp *shaperOpentype) compile(props SegmentProperties, userFeatures []Feature) {
	sp.plan.init0(sp.tables, props, userFeatures, sp.key)
}

// pull it all together!
func (sp *shaperOpentype) shape(font *Font, buffer *Buffer, features []Feature) {
	c := otContext{plan: &sp.plan, font: font, buffer: buffer, userFeatures: features}
	c.buffer.scratchFlags = bsfDefault

	const maxLenFactor = 64
	const maxLenMin = 16384
	const maxOpsFactor = 1024
	const maxOpsMin = 16384
	c.buffer.maxOps = max(len(c.buffer.Info)*maxOpsFactor, maxOpsMin)
	c.buffer.maxLen = max(len(c.buffer.Info)*maxLenFactor, maxLenMin)

	// save the original direction, we use it later.
	c.targetDirection = c.buffer.Props.Direction

	c.initializeMasks()
	c.buffer.setUnicodeProps()
	c.buffer.insertDottedCircle(c.font)

	c.buffer.formClusters()

	if debugMode {
		fmt.Println("FORMING CLUSTER :", c.buffer.Info)
	}

	c.buffer.ensureNativeDirection()

	if debugMode {
		fmt.Printf("PREPROCESS text start\n")
	}
	shaperPreprocessText(c.plan.shaper, c.buffer, c.font)
	if debugMode {
		fmt.Println("PREPROCESS text end:", c.buffer.Info)
	}

	c.substituteBeforePosition() // apply GSUB

	if debugMode {
		fmt.Println("AFTER SUBSTITUTE", c.buffer.Info)
	}

	c.position()

	if debugMode {
		fmt.Println("AFTER POSITION", c.buffer.Pos)
	}

	c.substituteAfterPosition()

	propagateFlags(c.buffer)

	c.buffer.Props.Direction = c.targetDirection

	c.buffer.maxOps = maxOpsDefault
}
