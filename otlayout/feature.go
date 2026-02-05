package otlayout

import (
	"errors"
	"fmt"

	"github.com/npillmayer/opentype/ot"
)

// Feature is a type for OpenType layout features.
// From the specification website
// https://docs.microsoft.com/en-us/typography/opentype/spec/featuretags :
//
// “Features provide information about how to use the glyphs in a font to render a script or
// language. For example, an Arabic font might have a feature for substituting initial glyph
// forms, and a Kanji font might have a feature for positioning glyphs vertically. All
// OpenType Layout features define data for glyph substitution, glyph positioning, or both.
//
// Each OpenType Layout feature has a feature tag that identifies its typographic function
// and effects. By examining a feature’s tag, a text-processing client can determine what a
// feature does and decide whether to implement it.”
//
// A feature uses ‘lookups’ to do operations on glyphs. GSUB and GPOS tables store lookups in a
// LookupList, into which Features link by maintaining a list of indices into the LookupList.
// The order of the lookup indices matters.
type Feature interface {
	Tag() ot.Tag          // e.g., 'liga'
	Type() LayoutTagType  // GSUB or GPOS ?
	Params() ot.Navigator // parameters for this feature
	LookupCount() int     // number of Lookups for this feature
	LookupIndex(int) int  // get index of lookup #i
}

// feature is the default implementation of Feature. Other, more spezialized Feature
// implementations will build on top of this.
type feature struct {
	typ LayoutTagType
	tag ot.Tag
	nav ot.Navigator
}

// FontFeature looks up OpenType layout features in OpenType font otf, i.e. it trys to
// find features in table GSUB as well as in table GPOS.
// In OpenType, features may be specific for script/language combinations, or DFLT.
// Also, some (few) features may have a GSUB part as well as a GPOS part.
// Setting script to 0 will look for a DFLT feature set.
//
// Returns GSUB features, GPOS features and a possible error condition.
// The features at index 0 of each slice are the mandatory features (for a script), and may
// be nil.
func FontFeatures(otf *ot.Font, script, lang ot.Tag) ([]Feature, []Feature, error) {
	lytTables, err := getLayoutTables(otf) // get GSUB and GPOS table for font otf
	if err != nil {
		return nil, nil, err
	}
	var feats = make([][]Feature, 2)
	if script == 0 {
		script = ot.DFLT
	}
	for i := range 2 { // collect features from GSUB and GPOS
		t := lytTables[i]
		m := t.ScriptList.Map()
		if !m.IsTagRecordMap() {
			return nil, nil, errors.New("script list is not a tag record map")
		}
		trm := m.AsTagRecordMap()
		scr := trm.LookupTag(script)
		if scr.IsNull() && script != ot.DFLT {
			scr = trm.LookupTag(ot.DFLT)
		}
		if scr.IsNull() {
			tracer().Infof("font %s has no feature-links from script %s", otf.F.Fontname, script)
			feats[i] = []Feature{}
			continue
		}
		tracer().Debugf("found script table for '%s'", script)
		langs := scr.Navigate()
		//tracer().Debugf("now at table %s", langs.Name())
		var dflt, lsys ot.Navigator
		dflt = langs.Link().Navigate()
		if lang != 0 {
			langMap := langs.Map()
			if langMap.IsTagRecordMap() {
				if lptr := langMap.AsTagRecordMap().LookupTag(lang); !lptr.IsNull() {
					lsys = lptr.Navigate()
				}
			}
		}
		if lsys == nil || lsys.IsVoid() {
			lsys = dflt
		}
		if lsys == nil || lsys.IsVoid() {
			return nil, nil, errFontFormat(fmt.Sprintf("font %s has empty LangSys entry for %s",
				otf.F.Fontname, script)) // I am not quite sure if this is really illegal
		}
		tracer().Debugf("lsys = %v, |lsys| = %d", lsys.Name(), lsys.List().Len())
		flocs := ListAll(lsys.List())
		feats[i] = make([]Feature, len(flocs))
		for j, loc := range flocs { // iterate over all feature records and wrap them into Go types
			inx := loc.U16(0) // inx is an index into a FeatureList
			feats[i][j] = wrapFeature(t, inx, i)
			if feats[i][j] != nil {
				tracer().Debugf("%2d: feat[%v] ", j, feats[i][j].Tag())
			}
		}
	}
	return feats[0], feats[1], nil
}

// wrapFeature creates a Feature type from a NavLocation, which should be
// an underlying feature bytes segment.
// `which` is 0 (GSUB) or 1 (GPOS).
func wrapFeature(t *ot.LayoutTable, inx uint16, which int) Feature {
	if inx == 0xffff {
		return nil // 0xffff denotes an unused mandatory feature slot (see OT spec)
	}
	tag, link := t.FeatureList.Get(int(inx))
	f := feature{
		tag: tag,
		nav: link.Navigate(),
	}
	if which == 0 {
		f.typ = GSubFeatureType
	} else {
		f.typ = GPosFeatureType
	}
	return f
}

// Tag returns the identifying tag of this feature.
func (f feature) Tag() ot.Tag {
	return f.tag
}

// Type returns wether this is a GSUB-feature or a GPOS-feature.
func (f feature) Type() LayoutTagType {
	return f.typ
}

// Params returns the parameters for this feature.
func (f feature) Params() ot.Navigator {
	return f.nav.Link().Navigate()
}

// LookupCount returns the number of lookup entries for a feature.
func (f feature) LookupCount() int {
	return f.nav.List().Len()
}

// LookupIndex gets the index-position of lookup number i.
func (f feature) LookupIndex(i int) int {
	if i < 0 || i >= f.nav.List().Len() {
		return -1
	}
	inx := f.nav.List().Get(i).U16(0)
	return int(inx)
}

// --- Feature application ---------------------------------------------------

// ApplyFeature will apply a feature to one or more glyphs of buffer buf, starting at
// position pos. It will return the position after application of the feature.
//
// If a feature is unsuited for the glyph at pos, ApplyFeature will do nothing and return pos.
//
// Attention: It is a requirement that font otf contains the appropriate layout table (either GSUB or
// GPOS) for the feature. Having the table missing may result in a crash. This should never happen, as
// extracting the feature will have required the layout table in the first place. Presence of the
// layout table is not checked again.
func ApplyFeature(otf *ot.Font, feat Feature, buf GlyphBuffer, pos, alt int) (int, bool, GlyphBuffer) {
	if feat == nil { // this is legal for unused mandatory feature slots
		return pos, false, buf
	} else if buf == nil || pos < 0 || pos >= len(buf) {
		tracer().Infof("application of font-feature requested for unusable buffer condition")
		return pos, false, buf
	}
	var lytTable *ot.LayoutTable
	if feat.Type() == GSubFeatureType {
		lytTable = &otf.Table(ot.T("GSUB")).Self().AsGSub().LayoutTable
	} else {
		lytTable = &otf.Table(ot.T("GPOS")).Self().AsGPos().LayoutTable
	}
	var applied, ok bool
	gdef := otf.Layout.GDef
	for i := 0; i < feat.LookupCount(); i++ { // lookups have to be applied in sequence
		inx := feat.LookupIndex(i)
		tracer().Debugf("feature %s lookup #%d => index %d", feat.Tag(), i, inx)
		lookup := lytTable.LookupList.Navigate(inx)
		pos, ok, buf, _ = applyLookup(&lookup, feat, buf, pos, alt, gdef, lytTable.LookupList)
		applied = applied || ok
	}
	return pos, applied, buf
}

// applyCtx bundles immutable lookup state for dispatch and helpers.
type applyCtx struct {
	feat       Feature                  // active feature for alternate selection and tracing
	lookup     *ot.Lookup               // lookup currently being applied
	lookupList lookupNavigator          // lookup list for nested lookups
	buf        GlyphBuffer              // mutable glyph buffer (GSUB), read-only for matching
	pos        int                      // current glyph position in buffer
	alt        int                      // alternate index (1..n) for substitution selection
	isGPos     bool                     // true if lookup is GPOS (non-substituting)
	flag       ot.LayoutTableLookupFlag // lookup flags for ignore/mark filtering
	gdef       *ot.GDefTable            // GDEF table for glyph classification, if present
}

// EditSpan describes a buffer mutation so contextual/chaining lookups can
// re-map lookup-record positions after a replacement/insertion.
type EditSpan struct {
	From int // start index (inclusive) of the replaced range
	To   int // end index (exclusive) of the replaced range
	Len  int // length of the replacement segment
}

// To apply a lookup, we have to iterate over the lookup's subtables and call them
// appropriately, respecting different subtable semantics and formats.
// Therefore this function more or less is a large switch to delegate to functions
// implementing a specific subtable logic.
func applyLookup(lookup *ot.Lookup, feat Feature, buf GlyphBuffer, pos, alt int, gdef *ot.GDefTable, lookupList lookupNavigator) (int, bool, GlyphBuffer, *EditSpan) {
	if lookup == nil {
		return pos, false, buf, nil
	}
	ctx := applyCtx{
		feat:       feat,
		lookup:     lookup,
		lookupList: lookupList,
		buf:        buf,
		pos:        pos,
		alt:        alt,
		isGPos:     ot.IsGPosLookupType(lookup.Type),
		flag:       lookup.Flag,
		gdef:       gdef,
	}
	return dispatchLookup(&ctx)
}

func dispatchLookup(ctx *applyCtx) (int, bool, GlyphBuffer, *EditSpan) {
	if ctx.lookup == nil {
		return ctx.pos, false, ctx.buf, nil
	}
	lookupType := ot.GSubLookupType(ctx.lookup.Type)
	if ctx.isGPos {
		lookupType = ot.GPosLookupType(ctx.lookup.Type)
	}
	tracer().Debugf("applying lookup '%s'/%d flags=0x%04x", ctx.feat.Tag(), lookupType, uint16(ctx.lookup.Flag))
	for i := 0; i < int(ctx.lookup.SubTableCount) && ctx.pos < ctx.buf.Len(); i++ {
		tracer().Debugf("-------------------- pos = %d", ctx.pos)
		sub := ctx.lookup.Subtable(i)
		if sub == nil {
			continue
		}
		tracer().Debugf("subtable #%d type %d format %d", i, sub.LookupType, sub.Format)
		var (
			pos  int
			ok   bool
			buf  GlyphBuffer
			edit *EditSpan
		)
		if ctx.isGPos {
			pos, ok, buf, edit = dispatchGPosLookup(ctx, sub)
		} else {
			pos, ok, buf, edit = dispatchGSubLookup(ctx, sub)
		}
		if ok {
			return pos, ok, buf, edit
		}
	}
	return ctx.pos, false, ctx.buf, nil
}

func dispatchGSubLookup(ctx *applyCtx, sub *ot.LookupSubtable) (int, bool, GlyphBuffer, *EditSpan) {
	switch sub.LookupType {
	case ot.GSubLookupTypeSingle: // Single Substitution Subtable
		switch sub.Format {
		case 1:
			return gsubLookupType1Fmt1(ctx, sub, ctx.buf, ctx.pos)
		case 2:
			return gsubLookupType1Fmt2(ctx, sub, ctx.buf, ctx.pos)
		}
	case ot.GSubLookupTypeMultiple: // Multiple Substitution Subtable
		return gsubLookupType2Fmt1(ctx, sub, ctx.buf, ctx.pos)
	case ot.GSubLookupTypeAlternate: // Alternate Substitution Subtable
		return gsubLookupType3Fmt1(ctx, sub, ctx.buf, ctx.pos, ctx.alt)
	case ot.GSubLookupTypeLigature: // Ligature Substitution Subtable
		return gsubLookupType4Fmt1(ctx, sub, ctx.buf, ctx.pos)
	case ot.GSubLookupTypeContext:
		switch sub.Format {
		case 1:
			return gsubLookupType5Fmt1(ctx, sub, ctx.buf, ctx.pos)
		case 2:
			return gsubLookupType5Fmt2(ctx, sub, ctx.buf, ctx.pos)
		case 3:
			return gsubLookupType5Fmt3(ctx, sub, ctx.buf, ctx.pos)
		}
	case ot.GSubLookupTypeChainingContext:
		switch sub.Format {
		case 1:
			return gsubLookupType6Fmt1(ctx, sub, ctx.buf, ctx.pos)
		case 2:
			return gsubLookupType6Fmt2(ctx, sub, ctx.buf, ctx.pos)
		case 3:
			return gsubLookupType6Fmt3(ctx, sub, ctx.buf, ctx.pos)
		}
	case ot.GSubLookupTypeExtensionSubs:
		tracer().Errorf("GSUB extension subtable reached dispatch; extension should be unwrapped during parsing")
		return ctx.pos, false, ctx.buf, nil
	case ot.GSubLookupTypeReverseChaining:
		switch sub.Format {
		case 1:
			return gsubLookupType8Fmt1(ctx, sub, ctx.buf, ctx.pos)
		}
	}
	tracer().Errorf("unknown GSUB lookup type %d/%d", sub.LookupType, sub.Format)
	return ctx.pos, false, ctx.buf, nil
}

func dispatchGPosLookup(ctx *applyCtx, sub *ot.LookupSubtable) (int, bool, GlyphBuffer, *EditSpan) {
	switch sub.LookupType {
	case ot.GPosLookupTypeSingle,
		ot.GPosLookupTypePair,
		ot.GPosLookupTypeCursive,
		ot.GPosLookupTypeMarkToBase,
		ot.GPosLookupTypeMarkToLigature,
		ot.GPosLookupTypeMarkToMark,
		ot.GPosLookupTypeContextPos,
		ot.GPosLookupTypeChainedContextPos:
		tracer().Errorf("GPOS lookup type %d/%d not implemented", sub.LookupType, sub.Format)
	case ot.GPosLookupTypeExtensionPos:
		tracer().Errorf("GPOS extension subtable reached dispatch; extension should be unwrapped during parsing")
	default:
		tracer().Errorf("unknown GPOS lookup type %d/%d", sub.LookupType, sub.Format)
	}
	return ctx.pos, false, ctx.buf, nil
}

// GSUB LookupType 1: Single Substitution Subtable
//
// Single substitution (SingleSubst) subtables tell a client to replace a single glyph
// with another glyph. The subtables can be either of two formats. Both formats require
// two distinct sets of glyph indices: one that defines input glyphs (specified in the
// Coverage table), and one that defines the output glyphs.

// GSUB LookupSubtable Type 1 Format 1 calculates the indices of the output glyphs, which
// are not explicitly defined in the subtable. To calculate an output glyph index,
// Format 1 adds a constant delta value to the input glyph index. For the substitutions to
// occur properly, the glyph indices in the input and output ranges must be in the same order.
// This format does not use the Coverage index that is returned from the Coverage table.
func gsubLookupType1Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, _, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage) // format 1 does not use the Coverage index
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d", buf.At(mpos), ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	// support is deltaGlyphID: add to original glyph ID to get substitute glyph ID
	var delta int
	switch v := lksub.Support.(type) {
	case int16:
		delta = int(v)
	case ot.GlyphIndex:
		delta = int(v)
	case int:
		delta = v
	default:
		tracer().Errorf("GSUB 1/1: unexpected delta type %T", lksub.Support)
		return pos, false, buf, nil
	}
	newGlyph := int(buf.At(mpos)) + delta
	tracer().Debugf("OT lookup GSUB 1/1: subst %d for %d", newGlyph, buf.At(mpos))
	// TODO: check bounds against max glyph ID
	buf.Set(mpos, ot.GlyphIndex(newGlyph))
	return mpos + 1, true, buf, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
}

// GSUB LookupSubtable Type 1 Format 2 provides an array of output glyph indices
// (substituteGlyphIDs) explicitly matched to the input glyph indices specified in the
// Coverage table.
// The substituteGlyphIDs array must contain the same number of glyph indices as the
// Coverage table. To locate the corresponding output glyph index in the substituteGlyphIDs
// array, this format uses the Coverage index returned from the Coverage table.
func gsubLookupType1Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	if glyph := lookupGlyph(lksub.Index, inx, false); glyph != 0 {
		tracer().Debugf("OT lookup GSUB 1/2: subst %d for %d", glyph, buf.At(mpos))
		buf.Set(mpos, glyph)
		return mpos + 1, true, buf, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
	}
	return pos, false, buf, nil
}

// LookupType 2: Multiple Substitution Subtable
//
// A Multiple Substitution (MultipleSubst) subtable replaces a single glyph with more
// than one glyph, as when multiple glyphs replace a single ligature.

// GSUB LookupSubtable Type 2 Format 1 defines a count of offsets in the sequenceOffsets
// array (sequenceCount), and an array of offsets to Sequence tables that define the output
// glyph indices (sequenceOffsets). The Sequence table offsets are ordered by the Coverage
// index of the input glyphs.
// For each input glyph listed in the Coverage table, a Sequence table defines the output
// glyphs. Each Sequence table contains a count of the glyphs in the output glyph sequence
// (glyphCount) and an array of output glyph indices (substituteGlyphIDs).
func gsubLookupType2Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	if glyphs := lookupGlyphs(lksub.Index, inx, true); len(glyphs) != 0 {
		tracer().Debugf("OT lookup GSUB 2/1: subst %v for %d", glyphs, buf.At(mpos))
		buf = buf.Replace(mpos, mpos+1, glyphs)
		return mpos + len(glyphs), true, buf, &EditSpan{From: mpos, To: mpos + 1, Len: len(glyphs)}
	}
	return pos, false, buf, nil
}

// LookupType 3: Alternate Substitution Subtable
//
// An Alternate Substitution (AlternateSubst) subtable identifies any number of aesthetic
// alternatives from which a user can choose a glyph variant to replace the input glyph.
// For example, if a font contains four variants of the ampersand symbol, the 'cmap' table
// will specify the index of one of the four glyphs as the default glyph index, and an
// AlternateSubst subtable will list the indices of the other three glyphs as alternatives.
// A text-processing client would then have the option of replacing the default glyph with
// any of the three alternatives.

// GSUB LookupSubtable Type 3 Format 1: For each glyph, an AlternateSet subtable contains a
// count of the alternative glyphs (glyphCount) and an array of their glyph indices
// (alternateGlyphIDs). Parameter `alt` selects an alternative glyph from this array.
// Having `alt` set to -1 will selected the last alternative glyph from the array.
func gsubLookupType3Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos, alt int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	if glyphs := lookupGlyphs(lksub.Index, inx, true); len(glyphs) != 0 {
		if alt < 0 {
			alt = len(glyphs) - 1
		}
		if alt < len(glyphs) {
			tracer().Debugf("OT lookup GSUB 3/1: subst %v for %d", glyphs[alt], buf.At(mpos))
			buf.Set(mpos, glyphs[alt])
			return mpos + 1, true, buf, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
		}
	}
	return pos, false, buf, nil
}

// LookupType 4: Ligature Substitution Subtable
//
// A Ligature Substitution (LigatureSubst) subtable identifies ligature substitutions where
// a single glyph replaces multiple glyphs. One LigatureSubst subtable can specify any number
// of ligature substitutions.

// GSUB LookupSubtable Type 4 Format 1 receives a sequence of glyphs and outputs a
// single glyph replacing the sequence. The Coverage table specifies only the index of the
// first glyph component of each ligature set.
//
// As this is a multi-lookup algorithm, calling gsubLookupType4Fmt1 will return a
// NavLocation which is a LigatureSet, i.e. a list of records of unequal lengths.
func gsubLookupType4Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	ligatureSet, err := lksub.Index.Get(inx, false)
	if err != nil || ligatureSet.Size() < 2 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 4|1 ligature set size = %d", ligatureSet.Size())
	ligCount := ligatureSet.U16(0)
	if ligatureSet.Size() < int(2+ligCount*2) { // must have room for count and u16offset[count]
		return pos, false, buf, nil
	}
	for i := 0; i < int(ligCount); i++ { // iterate over every ligature record in a ligature table
		ligpos := int(ligatureSet.U16(2 + i*2)) // jump to start of ligature record
		if ligatureSet.Size() < ligpos+6 {
			return pos, false, buf, nil
		}
		// Ligature table (glyph components for one ligature):
		// uint16 |  ligatureGlyph                       |  glyph ID of ligature to substitute
		// uint16 |  componentCount                      |  Number of components in the ligature
		// uint16 |  componentGlyphIDs[componentCount-1] |  Array of component glyph IDs
		componentCount := int(ligatureSet.U16(ligpos + 2))
		if componentCount == 0 || componentCount > 10 { // 10 is arbitrary, just to be careful
			continue
		}
		componentGlyphs := ligatureSet.Slice(ligpos+4, ligpos+4+(componentCount-1)*2).Glyphs()
		tracer().Debugf("%d component glyphs of ligature: %d %v", componentCount, buf.At(mpos), componentGlyphs)
		// now we know that buf[mpos] has matched the first glyph of the component pattern and
		// we will have to match following glyphs to the remaining componentGlyphs
		match := true
		cur := mpos
		for _, g := range componentGlyphs {
			next, ok := nextMatchable(ctx, buf, cur+1)
			if !ok || g != buf.At(next) {
				match = false
				break
			}
			cur = next
		}
		if match {
			ligatureGlyph := ot.GlyphIndex(ligatureSet.U16(ligpos))
			buf = buf.Replace(mpos, cur+1, []ot.GlyphIndex{ligatureGlyph})
			tracer().Debugf("after application of ligature, glyph = %d", buf.At(mpos))
			return mpos + 1, true, buf, &EditSpan{From: mpos, To: cur + 1, Len: 1}
		}
	}
	return pos, false, buf, nil
}

// LookupType 5: Contextual Substitution
//
// GSUB type 5 format 1 subtables (and GPOS type 7 format 1 subtables) define input sequences in terms of
// specific glyph IDs. Several sequences may be specified, but each is specified using glyph IDs.
//
// The first glyphs for the sequences are specified in a Coverage table. The remaining glyphs in each
// sequence are defined in SequenceRule tables—one for each sequence. If multiple sequences start with
// the same glyph, that glyph ID must be listed once in the Coverage table, and the corresponding sequence
// rules are aggregated using a SequenceRuleSet table—one for each initial glyph specified in the
// Coverage table.
//
// When evaluating a SequenceContextFormat1 subtable for a given position in a glyph sequence, the client
// searches for the current glyph in the Coverage table. If found, the corresponding SequenceRuleSet
// table is retrieved, and the SequenceRule tables for that set are examined to see if the current glyph
// sequence matches any of the sequence rules. The first matching rule subtable is used.
func gsubLookupType5Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	ruleSetLoc, err := lksub.Index.Get(inx, false)
	if err != nil || ruleSetLoc.Size() < 2 { // extra coverage glyphs or extra sequence rule sets are ignored
		return pos, false, buf, nil
	}
	// SequenceRuleSet table – all contexts beginning with the same glyph:
	// uint16   | seqRuleCount                 | Number of SequenceRule tables
	// Offset16 | seqRuleOffsets[seqRuleCount] | Array of offsets to SequenceRule tables, from
	//                                           beginning of the SequenceRuleSet table
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "SequenceRuleSet")
	tracer().Debugf("GSUB 5|1 rule set has %d rules", ruleSet.Size())
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 4 {
			continue
		}
		// SequenceRule table:
		// uint16 | glyphCount                  | Number of glyphs in the input glyph sequence
		// uint16 | seqLookupCount              | Number of SequenceLookupRecords
		// uint16 | inputSequence[glyphCount-1] | Array of input glyph IDs—starting with the second glyph
		// SequenceLookupRecord | seqLookupRecords[seqLookupCount] | Array of Sequence lookup records
		glyphCount := int(ruleLoc.U16(0))
		seqLookupCount := int(ruleLoc.U16(2))
		if glyphCount < 1 {
			continue
		}
		inputCount := glyphCount - 1
		inputBytes := inputCount * 2
		recBytes := seqLookupCount * 4
		minSize := 4 + inputBytes + recBytes
		if ruleLoc.Size() < minSize {
			continue
		}
		inputGlyphs := make([]ot.GlyphIndex, inputCount)
		for j := 0; j < inputCount; j++ {
			inputGlyphs[j] = ot.GlyphIndex(ruleLoc.U16(4 + j*2))
		}
		tracer().Debugf("GSUB 5|1 rule #%d input glyphs = %v", i, inputGlyphs)
		restPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, inputGlyphs)
		if !ok {
			continue
		}
		tracer().Debugf("GSUB 5|1 rule #%d matched at positions %v", i, append([]int{mpos}, restPos...))
		matchPositions := make([]int, 0, glyphCount)
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, restPos...)
		records := make([]ot.SequenceLookupRecord, seqLookupCount)
		recStart := 4 + inputBytes
		for r := 0; r < seqLookupCount; r++ {
			off := recStart + r*4
			records[r] = ot.SequenceLookupRecord{
				SequenceIndex:   ruleLoc.U16(off),
				LookupListIndex: ruleLoc.U16(off + 2),
			}
		}
		if ctx.lookupList == nil {
			return pos, false, buf, nil
		}
		out, applied := applySequenceLookupRecords(buf, matchPositions, records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		tracer().Debugf("GSUB 5|1 rule #%d applied = %v", i, applied)
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType5Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	if lksub.Support == nil {
		tracer().Errorf("expected SequenceContext|ClassDefs in field 'Support', is nil")
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok {
		tracer().Errorf("expected SequenceContext|ClassDefs in field 'Support', type error")
		return pos, false, buf, nil
	}
	if len(seqctx.ClassDefs) == 0 {
		tracer().Errorf("SequenceContext has no ClassDefs for GSUB 5|2")
		return pos, false, buf, nil
	}
	firstClass := seqctx.ClassDefs[0].Lookup(buf.At(mpos))
	tracer().Debugf("GSUB 5|2 first glyph class = %d", firstClass)
	ruleSetLoc, err := lksub.Index.Get(int(firstClass), false)
	if err != nil || ruleSetLoc.Size() < 2 {
		return pos, false, buf, nil
	}
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "SequenceRuleSet")
	tracer().Debugf("GSUB 5|2 rule set has %d rules", ruleSet.Size())
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 4 {
			continue
		}
		glyphCount := int(ruleLoc.U16(0))
		seqLookupCount := int(ruleLoc.U16(2))
		if glyphCount < 1 {
			continue
		}
		classCount := glyphCount - 1
		classBytes := classCount * 2
		recBytes := seqLookupCount * 4
		minSize := 4 + classBytes + recBytes
		if ruleLoc.Size() < minSize {
			continue
		}
		classes := make([]uint16, classCount)
		for j := 0; j < classCount; j++ {
			classes[j] = ruleLoc.U16(4 + j*2)
		}
		tracer().Debugf("GSUB 5|2 rule #%d classes = %v", i, classes)
		restPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, seqctx.ClassDefs[0], classes)
		if !ok {
			continue
		}
		tracer().Debugf("GSUB 5|2 rule #%d matched at positions %v", i, append([]int{mpos}, restPos...))
		matchPositions := make([]int, 0, glyphCount)
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, restPos...)
		records := make([]ot.SequenceLookupRecord, seqLookupCount)
		recStart := 4 + classBytes
		for r := 0; r < seqLookupCount; r++ {
			off := recStart + r*4
			records[r] = ot.SequenceLookupRecord{
				SequenceIndex:   ruleLoc.U16(off),
				LookupListIndex: ruleLoc.U16(off + 2),
			}
		}
		if ctx.lookupList == nil {
			return pos, false, buf, nil
		}
		out, applied := applySequenceLookupRecords(buf, matchPositions, records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		tracer().Debugf("GSUB 5|2 rule #%d applied = %v", i, applied)
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType5Fmt3(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	if lksub.Support == nil {
		tracer().Errorf("expected SequenceContext in field 'Support', is nil")
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok {
		tracer().Errorf("expected SequenceContext in field 'Support', type error")
		return pos, false, buf, nil
	}
	if len(seqctx.InputCoverage) == 0 {
		tracer().Errorf("SequenceContext has no InputCoverage for GSUB 5|3")
		return pos, false, buf, nil
	}
	inputPos, ok := matchCoverageSequenceForward(ctx, buf, pos, seqctx.InputCoverage)
	if !ok {
		return pos, false, buf, nil
	}
	if len(lksub.LookupRecords) == 0 {
		return pos, false, buf, nil
	}
	if ctx.lookupList == nil {
		return pos, false, buf, nil
	}
	out, applied := applySequenceLookupRecords(buf, inputPos, lksub.LookupRecords, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
	if applied {
		return pos, true, out, nil
	}
	return pos, false, buf, nil
}

func gsubLookupType6Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	rules, err := parseChainedSequenceRules(lksub, inx)
	if err != nil || len(rules) == 0 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|1 rule set for coverage %d: %d rules", inx, len(rules))
	for _, rule := range rules {
		tracer().Debugf("GSUB 6|1 rule: backtrack=%d input=%d lookahead=%d records=%d",
			len(rule.Backtrack), len(rule.Input), len(rule.Lookahead), len(rule.Records))
		inputPos, ok := matchGlyphSequenceForward(ctx, buf, mpos+1, rule.Input)
		if !ok {
			tracer().Debugf("GSUB 6|1 input sequence did not match at pos %d", mpos+1)
			continue
		}
		matchPositions := make([]int, 0, 1+len(inputPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, inputPos...)
		if len(rule.Backtrack) > 0 {
			if _, ok := matchGlyphSequenceBackward(ctx, buf, mpos, rule.Backtrack); !ok {
				tracer().Debugf("GSUB 6|1 backtrack did not match at pos %d", mpos)
				continue
			}
		}
		if len(rule.Lookahead) > 0 {
			last := mpos
			if len(inputPos) > 0 {
				last = inputPos[len(inputPos)-1]
			}
			if _, ok := matchGlyphSequenceForward(ctx, buf, last+1, rule.Lookahead); !ok {
				tracer().Debugf("GSUB 6|1 lookahead did not match at pos %d", last+1)
				continue
			}
		}
		if len(rule.Records) == 0 || ctx.lookupList == nil {
			continue
		}
		tracer().Debugf("GSUB 6|1 matched at positions %v", matchPositions)
		out, applied := applySequenceLookupRecords(buf, matchPositions, rule.Records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

func gsubLookupType6Fmt2(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	mpos, inx, ok := matchCoverageForward(ctx, buf, pos, lksub.Coverage)
	if ok {
		tracer().Debugf("coverage of glyph ID %d is %d/%v", buf.At(mpos), inx, ok)
	}
	if !ok {
		return pos, false, buf, nil
	}
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok || seqctx == nil || len(seqctx.ClassDefs) < 3 {
		tracer().Debugf("GSUB 6|2 missing class definitions")
		return pos, false, buf, nil
	}
	rules, err := parseChainedClassSequenceRules(lksub, inx)
	if err != nil || len(rules) == 0 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|2 rule set for coverage %d: %d rules", inx, len(rules))
	for _, rule := range rules {
		tracer().Debugf("GSUB 6|2 rule: backtrack=%d input=%d lookahead=%d records=%d",
			len(rule.Backtrack), len(rule.Input), len(rule.Lookahead), len(rule.Records))
		inputPos, ok := matchClassSequenceForward(ctx, buf, mpos+1, seqctx.ClassDefs[1], rule.Input)
		if !ok {
			tracer().Debugf("GSUB 6|2 input classes did not match at pos %d", mpos+1)
			continue
		}
		matchPositions := make([]int, 0, 1+len(inputPos))
		matchPositions = append(matchPositions, mpos)
		matchPositions = append(matchPositions, inputPos...)
		if len(rule.Backtrack) > 0 {
			if _, ok := matchClassSequenceBackward(ctx, buf, mpos, seqctx.ClassDefs[0], rule.Backtrack); !ok {
				tracer().Debugf("GSUB 6|2 backtrack did not match at pos %d", mpos)
				continue
			}
		}
		if len(rule.Lookahead) > 0 {
			last := mpos
			if len(inputPos) > 0 {
				last = inputPos[len(inputPos)-1]
			}
			if _, ok := matchClassSequenceForward(ctx, buf, last+1, seqctx.ClassDefs[2], rule.Lookahead); !ok {
				tracer().Debugf("GSUB 6|2 lookahead did not match at pos %d", last+1)
				continue
			}
		}
		if len(rule.Records) == 0 || ctx.lookupList == nil {
			continue
		}
		tracer().Debugf("GSUB 6|2 matched at positions %v", matchPositions)
		out, applied := applySequenceLookupRecords(buf, matchPositions, rule.Records, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
		if applied {
			return pos, true, out, nil
		}
	}
	return pos, false, buf, nil
}

// Chained Sequence Context Format 3: coverage-based glyph contexts
// GSUB type 6 format 3 subtables and GPOS type 6 format 3 subtables define input sequences patterns, as
// well as chained backtrack and lookahead sequence patterns, in terms of sets of glyph defined using
// Coverage tables.
// The ChainedSequenceContextFormat3 table specifies exactly one input sequence pattern. It has three
// arrays of offsets to coverage tables: one for the input sequence pattern, one for the backtrack
// sequence pattern, and one for the lookahead sequence pattern. For each array, the offsets correspond,
// in order, to the positions in the sequence pattern.
func gsubLookupType6Fmt3(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	seqctx, ok := lksub.Support.(*ot.SequenceContext)
	if !ok || len(seqctx.InputCoverage) == 0 {
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|3 coverages: backtrack=%d input=%d lookahead=%d records=%d",
		len(seqctx.BacktrackCoverage), len(seqctx.InputCoverage), len(seqctx.LookaheadCoverage), len(lksub.LookupRecords))
	tracer().Debugf("GSUB 6|3 pos=%d glyph=%d", pos, buf.At(pos))
	if len(seqctx.InputCoverage) > 0 {
		tracer().Debugf("GSUB 6|3 input[0] contains glyph %d = %v", buf.At(pos), seqctx.InputCoverage[0].Contains(buf.At(pos)))
		if pos+1 < buf.Len() {
			tracer().Debugf("GSUB 6|3 input[0] contains glyph %d (pos+1) = %v", buf.At(pos+1), seqctx.InputCoverage[0].Contains(buf.At(pos+1)))
		}
		tracer().Debugf("GSUB 6|3 input[0] contains glyph 76 = %v", seqctx.InputCoverage[0].Contains(76))
		tracer().Debugf("GSUB 6|3 input[0] contains glyph 2195 = %v", seqctx.InputCoverage[0].Contains(2195))
		tracer().Debugf("GSUB 6|3 input[0] contains glyph 18944 = %v", seqctx.InputCoverage[0].Contains(18944))
	}
	inputFn := func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
		if len(seqctx.InputCoverage) > 0 {
			if _, ok := seqctx.InputCoverage[0].Match(buf.At(pos)); !ok {
				tracer().Debugf("GSUB 6|3 first input coverage did not match glyph %d at pos %d", buf.At(pos), pos)
			}
		}
		return matchCoverageSequenceForward(ctx, buf, pos, seqctx.InputCoverage)
	}
	var backtrackFn matchSeqFn
	if len(seqctx.BacktrackCoverage) > 0 {
		backtrackFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceBackward(ctx, buf, pos, seqctx.BacktrackCoverage)
		}
	}
	var lookaheadFn matchSeqFn
	if len(seqctx.LookaheadCoverage) > 0 {
		lookaheadFn = func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool) {
			return matchCoverageSequenceForward(ctx, buf, pos+1, seqctx.LookaheadCoverage)
		}
	}
	inputPos, ok := matchChainedForward(ctx, buf, pos, backtrackFn, inputFn, lookaheadFn)
	if !ok {
		tracer().Debugf("GSUB 6|3 no match at pos %d", pos)
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 6|3 matched at positions %v", inputPos)
	if len(lksub.LookupRecords) == 0 {
		tracer().Debugf("GSUB 6|3 has no lookup records")
		return pos, false, buf, nil
	}
	if ctx.lookupList == nil {
		tracer().Debugf("GSUB 6|3 missing lookup list")
		return pos, false, buf, nil
	}
	out, applied := applySequenceLookupRecords(buf, inputPos, lksub.LookupRecords, ctx.lookupList, ctx.feat, ctx.alt, ctx.gdef)
	tracer().Debugf("GSUB 6|3 applied = %v", applied)
	if applied {
		return pos, true, out, nil
	}
	return pos, false, buf, nil
}

// GSUB LookupType 8: Reverse Chaining Single Substitution Subtable
func gsubLookupType8Fmt1(ctx *applyCtx, lksub *ot.LookupSubtable, buf GlyphBuffer, pos int) (
	int, bool, GlyphBuffer, *EditSpan) {
	//
	rc, ok := lksub.Support.(*ot.ReverseChainingSubst)
	if !ok || rc == nil {
		tracer().Debugf("GSUB 8|1 missing ReverseChainingSubst support")
		return pos, false, buf, nil
	}
	tracer().Debugf("GSUB 8|1 pos=%d backtrack=%d lookahead=%d subst=%d",
		pos, len(rc.BacktrackCoverage), len(rc.LookaheadCoverage), len(rc.SubstituteGlyphIDs))
	minPos := pos
	if minPos < 0 {
		minPos = 0
	}
	for i := buf.Len() - 1; i >= minPos; {
		mpos, ok := prevMatchable(ctx, buf, i)
		if !ok || mpos < minPos {
			break
		}
		tracer().Debugf("GSUB 8|1 candidate pos=%d glyph=%d", mpos, buf.At(mpos))
		inx, ok := lksub.Coverage.Match(buf.At(mpos))
		if !ok {
			tracer().Debugf("GSUB 8|1 coverage did not match at pos %d", mpos)
			i = mpos - 1
			continue
		}
		if len(rc.BacktrackCoverage) > 0 {
			if _, ok := matchCoverageSequenceBackward(ctx, buf, mpos, rc.BacktrackCoverage); !ok {
				tracer().Debugf("GSUB 8|1 backtrack did not match at pos %d", mpos)
				i = mpos - 1
				continue
			}
		}
		if len(rc.LookaheadCoverage) > 0 {
			if _, ok := matchCoverageSequenceForward(ctx, buf, mpos+1, rc.LookaheadCoverage); !ok {
				tracer().Debugf("GSUB 8|1 lookahead did not match at pos %d", mpos)
				i = mpos - 1
				continue
			}
		}
		if inx < 0 || inx >= len(rc.SubstituteGlyphIDs) {
			tracer().Debugf("GSUB 8|1 substitute index %d out of range", inx)
			i = mpos - 1
			continue
		}
		subst := rc.SubstituteGlyphIDs[inx]
		tracer().Debugf("GSUB 8|1 subst %d for %d at pos %d", subst, buf.At(mpos), mpos)
		buf.Set(mpos, subst)
		return mpos + 1, true, buf, &EditSpan{From: mpos, To: mpos + 1, Len: 1}
	}
	return pos, false, buf, nil
}

// --- Helpers ---------------------------------------------------------------

func skipGlyph(ctx *applyCtx, g ot.GlyphIndex) bool {
	if ctx == nil || ctx.gdef == nil {
		return false
	}
	if ctx.lookup == nil {
		return false
	}
	class := glyphClass(ctx.gdef, g)
	if ctx.flag&ot.LOOKUP_FLAG_IGNORE_BASE_GLYPHS != 0 && class == ot.BaseGlyph {
		return true
	}
	if ctx.flag&ot.LOOKUP_FLAG_IGNORE_LIGATURES != 0 && class == ot.LigatureGlyph {
		return true
	}
	if ctx.flag&ot.LOOKUP_FLAG_IGNORE_MARKS != 0 && class == ot.MarkGlyph {
		return true
	}
	if class == ot.MarkGlyph {
		if ctx.flag&ot.LOOKUP_FLAG_USE_MARK_FILTERING_SET != 0 {
			setIndex := ctx.lookup.MarkFilteringSet()
			if !inMarkFilteringSet(ctx.gdef, setIndex, g) {
				return true
			}
		}
		if matype := markAttachmentType(ctx.flag); matype != 0 {
			if markAttachClass(ctx.gdef, g) != matype {
				return true
			}
		}
	}
	return false
}

func nextMatchable(ctx *applyCtx, buf GlyphBuffer, pos int) (int, bool) {
	for i := pos; i < buf.Len(); i++ {
		if !skipGlyph(ctx, buf.At(i)) {
			return i, true
		}
	}
	return 0, false
}

func prevMatchable(ctx *applyCtx, buf GlyphBuffer, pos int) (int, bool) {
	for i := pos; i >= 0; i-- {
		if !skipGlyph(ctx, buf.At(i)) {
			return i, true
		}
	}
	return 0, false
}

func matchCoverageForward(ctx *applyCtx, buf GlyphBuffer, pos int, cov ot.Coverage) (int, int, bool) {
	for i := pos; i < buf.Len(); {
		mpos, ok := nextMatchable(ctx, buf, i)
		if !ok {
			return 0, 0, false
		}
		if inx, ok := cov.Match(buf.At(mpos)); ok {
			return mpos, inx, true
		}
		i = mpos + 1
	}
	return 0, 0, false
}

func matchCoverageSequenceForward(ctx *applyCtx, buf GlyphBuffer, pos int, covs []ot.Coverage) ([]int, bool) {
	if len(covs) == 0 {
		return nil, false
	}
	out := make([]int, len(covs))
	cur := pos
	for i, cov := range covs {
		mpos, ok := nextMatchable(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if _, ok := cov.Match(buf.At(mpos)); !ok {
			return nil, false
		}
		out[i] = mpos
		cur = mpos + 1
	}
	return out, true
}

func matchCoverageSequenceBackward(ctx *applyCtx, buf GlyphBuffer, pos int, covs []ot.Coverage) ([]int, bool) {
	if len(covs) == 0 {
		return nil, false
	}
	out := make([]int, len(covs))
	cur := pos - 1
	for i, cov := range covs {
		mpos, ok := prevMatchable(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if _, ok := cov.Match(buf.At(mpos)); !ok {
			return nil, false
		}
		out[i] = mpos
		cur = mpos - 1
	}
	return out, true
}

func buildInputMap(matchPositions []int) []int {
	out := make([]int, len(matchPositions))
	copy(out, matchPositions)
	return out
}

func glyphClass(gdef *ot.GDefTable, gid ot.GlyphIndex) ot.GlyphClassDefEnum {
	if gdef == nil {
		return 0
	}
	return ot.GlyphClassDefEnum(gdef.GlyphClassDef.Lookup(gid))
}

func markAttachClass(gdef *ot.GDefTable, gid ot.GlyphIndex) uint16 {
	if gdef == nil {
		return 0
	}
	return uint16(gdef.MarkAttachmentClassDef.Lookup(gid))
}

func inMarkFilteringSet(gdef *ot.GDefTable, setIndex uint16, gid ot.GlyphIndex) bool {
	if gdef == nil {
		return false
	}
	if int(setIndex) >= len(gdef.MarkGlyphSets) {
		return false
	}
	set := gdef.MarkGlyphSets[setIndex]
	if set == nil {
		return false
	}
	_, ok := set.Match(gid)
	return ok
}

func markAttachmentType(flag ot.LayoutTableLookupFlag) uint16 {
	return uint16((flag & ot.LOOKUP_FLAG_MARK_ATTACHMENT_TYPE_MASK) >> 8)
}

type lookupNavigator interface {
	Navigate(int) ot.Lookup
}

func matchGlyphSequenceForward(ctx *applyCtx, buf GlyphBuffer, pos int, glyphs []ot.GlyphIndex) ([]int, bool) {
	if len(glyphs) == 0 {
		return nil, false
	}
	out := make([]int, len(glyphs))
	cur := pos
	for i, gid := range glyphs {
		mpos, ok := nextMatchable(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if buf.At(mpos) != gid {
			return nil, false
		}
		out[i] = mpos
		cur = mpos + 1
	}
	return out, true
}

func matchGlyphSequenceBackward(ctx *applyCtx, buf GlyphBuffer, pos int, glyphs []ot.GlyphIndex) ([]int, bool) {
	if len(glyphs) == 0 {
		return nil, false
	}
	out := make([]int, len(glyphs))
	cur := pos - 1
	for i, gid := range glyphs {
		mpos, ok := prevMatchable(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if buf.At(mpos) != gid {
			return nil, false
		}
		out[i] = mpos
		cur = mpos - 1
	}
	return out, true
}

func matchClassSequenceForward(ctx *applyCtx, buf GlyphBuffer, pos int, classDef ot.ClassDefinitions, classes []uint16) ([]int, bool) {
	if len(classes) == 0 {
		return nil, false
	}
	out := make([]int, len(classes))
	cur := pos
	for i, clz := range classes {
		mpos, ok := nextMatchable(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if uint16(classDef.Lookup(buf.At(mpos))) != clz {
			return nil, false
		}
		out[i] = mpos
		cur = mpos + 1
	}
	return out, true
}

func matchClassSequenceBackward(ctx *applyCtx, buf GlyphBuffer, pos int, classDef ot.ClassDefinitions, classes []uint16) ([]int, bool) {
	if len(classes) == 0 {
		return nil, false
	}
	out := make([]int, len(classes))
	cur := pos - 1
	for i, clz := range classes {
		mpos, ok := prevMatchable(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if uint16(classDef.Lookup(buf.At(mpos))) != clz {
			return nil, false
		}
		out[i] = mpos
		cur = mpos - 1
	}
	return out, true
}

type matchSeqFn func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool)

func matchChainedForward(ctx *applyCtx, buf GlyphBuffer, pos int, backtrack, input, lookahead matchSeqFn) ([]int, bool) {
	inputPos, ok := input(ctx, buf, pos)
	if !ok || len(inputPos) == 0 {
		return nil, false
	}
	if backtrack != nil {
		if _, ok := backtrack(ctx, buf, inputPos[0]); !ok {
			return nil, false
		}
	}
	if lookahead != nil {
		last := inputPos[len(inputPos)-1]
		if _, ok := lookahead(ctx, buf, last); !ok {
			return nil, false
		}
	}
	return inputPos, true
}

// Chained-context rule parsing helpers. These are used by GSUB-6 (chained substitution)
// and will also be useful for GPOS-8 (chained positioning).
type parsedChainedRule struct {
	Backtrack []ot.GlyphIndex
	Input     []ot.GlyphIndex
	Lookahead []ot.GlyphIndex
	Records   []ot.SequenceLookupRecord
}

func parseChainedSequenceRules(lksub *ot.LookupSubtable, coverageIndex int) ([]parsedChainedRule, error) {
	if lksub.Index.Size() == 0 {
		return nil, nil
	}
	ruleSetLoc, err := lksub.Index.Get(coverageIndex, false)
	if err != nil {
		return nil, err
	}
	if ruleSetLoc.Size() < 2 {
		return nil, nil
	}
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "ChainSubRuleSet")
	out := make([]parsedChainedRule, 0, ruleSet.Size())
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 2 {
			continue
		}
		rule := lksub.ChainedSequenceRule(ruleLoc)
		out = append(out, parsedChainedRule{
			Backtrack: rule.BacktrackGlyphs(),
			Input:     rule.InputGlyphs(),
			Lookahead: rule.LookaheadGlyphs(),
			Records:   rule.LookupRecords(),
		})
	}
	return out, nil
}

type parsedChainedClassRule struct {
	Backtrack []uint16
	Input     []uint16
	Lookahead []uint16
	Records   []ot.SequenceLookupRecord
}

func parseChainedClassSequenceRules(lksub *ot.LookupSubtable, coverageIndex int) ([]parsedChainedClassRule, error) {
	if lksub.Index.Size() == 0 {
		return nil, nil
	}
	ruleSetLoc, err := lksub.Index.Get(coverageIndex, false)
	if err != nil {
		return nil, err
	}
	if ruleSetLoc.Size() < 2 {
		return nil, nil
	}
	ruleSet := ot.ParseVarArray(ruleSetLoc, 0, 2, "ChainSubClassSet")
	out := make([]parsedChainedClassRule, 0, ruleSet.Size())
	for i := 0; i < ruleSet.Size(); i++ {
		ruleLoc, err := ruleSet.Get(i, false)
		if err != nil || ruleLoc.Size() < 2 {
			continue
		}
		rule := lksub.ChainedClassSequenceRule(ruleLoc)
		out = append(out, parsedChainedClassRule{
			Backtrack: rule.BacktrackClasses(),
			Input:     rule.InputClasses(),
			Lookahead: rule.LookaheadClasses(),
			Records:   rule.LookupRecords(),
		})
	}
	return out, nil
}

func applySequenceLookupRecords(
	buf GlyphBuffer,
	matchPositions []int,
	records []ot.SequenceLookupRecord,
	lookupList lookupNavigator,
	feat Feature,
	alt int,
	gdef *ot.GDefTable,
) (GlyphBuffer, bool) {
	mapIdx := buildInputMap(matchPositions)
	if lookupList == nil || len(mapIdx) == 0 {
		return buf, false
	}

	applied := false
	for _, rec := range records {
		tracer().Debugf("sequence lookup record: seq=%d lookup=%d", rec.SequenceIndex, rec.LookupListIndex)
		seqIndex := int(rec.SequenceIndex)
		if seqIndex < 0 || seqIndex >= len(mapIdx) {
			continue
		}
		targetPos := mapIdx[seqIndex]
		if targetPos < 0 || targetPos >= buf.Len() {
			continue
		}
		tracer().Debugf("sequence lookup record: target position %d", targetPos)
		lookup := lookupList.Navigate(int(rec.LookupListIndex))
		_, ok, out, edit := applyLookup(&lookup, feat, buf, targetPos, alt, gdef, lookupList)
		if !ok {
			continue
		}
		applied = true
		buf = out
		if edit == nil {
			continue
		}
		delta := edit.Len - (edit.To - edit.From)
		for i := range mapIdx {
			if mapIdx[i] < 0 {
				continue
			}
			if mapIdx[i] >= edit.To {
				mapIdx[i] += delta
			} else if mapIdx[i] >= edit.From {
				if edit.Len == 0 {
					mapIdx[i] = -1
				} else {
					mapIdx[i] = edit.From
				}
			}
		}
	}
	return buf, applied
}

// lookupGlyph is a small helper which looks up an index for a glyph (previously
// returned from a coverage table), checks for errors, and returns the resulting glyph index.
func lookupGlyph(index ot.VarArray, ginx int, deep bool) ot.GlyphIndex {
	outglyph, err := index.Get(ginx, deep)
	if err != nil {
		return 0
	}
	return ot.GlyphIndex(outglyph.U16(0))
}

// lookupGlyphs is a small helper which looks up an index for a glyph (previously
// returned from a coverage table), checks for errors, and returns the resulting glyphs.
func lookupGlyphs(index ot.VarArray, ginx int, deep bool) []ot.GlyphIndex {
	outglyphs, err := index.Get(ginx, deep)
	if err != nil {
		return []ot.GlyphIndex{}
	}
	if outglyphs.Size() < 2 {
		return []ot.GlyphIndex{}
	}
	cnt := int(outglyphs.U16(0))
	if cnt == 0 {
		return []ot.GlyphIndex{}
	}
	b := outglyphs.Bytes()
	if len(b) < 2 {
		return []ot.GlyphIndex{}
	}
	if len(b) < 2+cnt*2 {
		cnt = (len(b) - 2) / 2
	}
	glyphs := make([]ot.GlyphIndex, 0, cnt)
	for i := 0; i < cnt; i++ {
		off := 2 + i*2
		glyphs = append(glyphs, ot.GlyphIndex(b[off])<<8|ot.GlyphIndex(b[off+1]))
	}
	return glyphs
}

// get GSUB and GPOS from a font safely
func getLayoutTables(otf *ot.Font) ([]*ot.LayoutTable, error) {
	var table ot.Table
	var lytt = make([]*ot.LayoutTable, 2)
	if table = otf.Table(ot.T("GSUB")); table == nil {
		return nil, errFontFormat(fmt.Sprintf("font %s has no GSUB table", otf.F.Fontname))
	}
	lytt[0] = &table.Self().AsGSub().LayoutTable
	if table = otf.Table(ot.T("GPOS")); table == nil {
		return nil, errFontFormat(fmt.Sprintf("font %s has no GPOS table", otf.F.Fontname))
	}
	lytt[1] = &table.Self().AsGPos().LayoutTable
	return lytt, nil
}

// check if we recognize a feature tag
func identifyFeatureTag(tag ot.Tag) (LayoutTagType, error) {
	if tag&0xffff0000 == ot.T("cv__")&0xffff0000 { // cv00 - cv99
		return GSubFeatureType, nil
	}
	if tag&0xffff0000 == ot.T("ss__")&0xffff0000 { // ss00 - ss20
		return GSubFeatureType, nil
	}
	typ, ok := RegisteredFeatureTags[tag]
	if !ok {
		return 0, errFontFormat(fmt.Sprintf("feature '%s' seems not to be registered", tag))
	}
	return typ, nil
}
