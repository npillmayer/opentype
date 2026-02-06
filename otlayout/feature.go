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
func ApplyFeature(otf *ot.Font, feat Feature, st *BufferState, alt int) (int, bool) {
	if feat == nil { // this is legal for unused mandatory feature slots
		return st.Index, false
	} else if st == nil || st.Glyphs == nil || st.Index < 0 || st.Index >= len(st.Glyphs) {
		tracer().Infof("application of font-feature requested for unusable buffer condition")
		if st != nil {
			return st.Index, false
		}
		return 0, false
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
		_, ok, _ = applyLookup(&lookup, feat, st, alt, gdef, lytTable.LookupList)
		applied = applied || ok
	}
	return st.Index, applied
}

// applyCtx bundles immutable lookup state for dispatch and helpers.
type applyCtx struct {
	feat       Feature                  // active feature for alternate selection and tracing
	lookup     *ot.Lookup               // lookup currently being applied
	lookupList lookupNavigator          // lookup list for nested lookups
	buf        *BufferState             // buffer state (glyphs + positions)
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

// BufferState bundles glyph and position buffers with a current index.
// Position buffer may be nil when only GSUB is applied.
// Copy-on-write is implemented via shared flags; mutating methods will clone
// backing slices when necessary.
type BufferState struct {
	Glyphs       GlyphBuffer
	Pos          PosBuffer
	Index        int
	glyphsShared bool
	posShared    bool
}

// NewBufferState constructs a buffer state with index 0.
func NewBufferState(g GlyphBuffer, p PosBuffer) *BufferState {
	b := &BufferState{
		Glyphs: g,
		Pos:    p,
		Index:  0,
	}
	if p != nil && len(p) != len(g) {
		b.Pos = p.ResizeLike(g)
	}
	return b
}

// CloneShared returns a logically shared copy of the buffer state.
// Both the original and the clone will clone on the next mutation.
func (b *BufferState) CloneShared() *BufferState {
	if b == nil {
		return nil
	}
	b.glyphsShared = true
	b.posShared = true
	return &BufferState{
		Glyphs:       b.Glyphs,
		Pos:          b.Pos,
		Index:        b.Index,
		glyphsShared: true,
		posShared:    true,
	}
}

func (b *BufferState) ensureUniqueGlyphs() {
	if b == nil {
		return
	}
	if b.glyphsShared {
		b.Glyphs = append(GlyphBuffer(nil), b.Glyphs...)
		b.glyphsShared = false
	}
}

func (b *BufferState) ensureUniquePos() {
	if b == nil {
		return
	}
	if b.Pos == nil {
		b.Pos = NewPosBuffer(len(b.Glyphs))
		b.posShared = false
		return
	}
	if b.posShared {
		b.Pos = append(PosBuffer(nil), b.Pos...)
		b.posShared = false
	}
}

func (b *BufferState) Len() int {
	if b == nil {
		return 0
	}
	return b.Glyphs.Len()
}

func (b *BufferState) At(i int) ot.GlyphIndex {
	return b.Glyphs.At(i)
}

func (b *BufferState) Set(i int, g ot.GlyphIndex) {
	b.ensureUniqueGlyphs()
	b.Glyphs.Set(i, g)
}

// ApplyEdit mirrors a GSUB edit onto the position buffer to keep alignment.
func (b *BufferState) ApplyEdit(edit *EditSpan) {
	if b == nil || edit == nil {
		return
	}
	if b.Pos == nil {
		return
	}
	b.ensureUniquePos()
	b.Pos = b.Pos.ApplyEdit(edit)
}

// EnsurePos allocates a position buffer if missing and keeps it aligned with glyphs.
func (b *BufferState) EnsurePos() {
	if b == nil {
		return
	}
	if b.Pos == nil {
		b.Pos = NewPosBuffer(len(b.Glyphs))
		b.posShared = false
		return
	}
	if len(b.Pos) != len(b.Glyphs) {
		b.ensureUniquePos()
		b.Pos = b.Pos.ResizeLike(b.Glyphs)
	}
}

// ReplaceGlyphs replaces the range [i:j) with repl and mirrors the edit into Pos when present.
func (b *BufferState) ReplaceGlyphs(i, j int, repl []ot.GlyphIndex) *EditSpan {
	if b == nil {
		return nil
	}
	if i < 0 || j < i || j > len(b.Glyphs) {
		panic("BufferState.ReplaceGlyphs: invalid range")
	}
	b.ensureUniqueGlyphs()
	b.Glyphs = b.Glyphs.Replace(i, j, repl)
	edit := &EditSpan{From: i, To: j, Len: len(repl)}
	if b.Pos != nil {
		b.ensureUniquePos()
		b.Pos = b.Pos.ApplyEdit(edit)
	}
	return edit
}

// InsertGlyphs inserts glyphs before index i.
func (b *BufferState) InsertGlyphs(i int, glyphs []ot.GlyphIndex) *EditSpan {
	return b.ReplaceGlyphs(i, i, glyphs)
}

// DeleteGlyphs removes the range [i:j).
func (b *BufferState) DeleteGlyphs(i, j int) *EditSpan {
	return b.ReplaceGlyphs(i, j, nil)
}

// PosBuffer holds per-glyph positioning information for GPOS.
// It is kept in sync with the glyph buffer by index.
type PosBuffer []PosItem

// AttachKind describes how a glyph is attached to another glyph.
type AttachKind uint8

const (
	AttachNone AttachKind = iota
	AttachMarkToBase
	AttachMarkToLigature
	AttachMarkToMark
	AttachCursive
)

// AnchorRef carries unresolved anchor references for later resolution.
// Indices are format-specific references into GPOS mark/base/ligature tables.
type AnchorRef struct {
	MarkAnchor   uint16 // index into MarkArray for mark attachments (GPOS 4/5/6)
	BaseAnchor   uint16 // index into BaseArray / Mark2Array for mark attachments
	LigatureComp uint16 // ligature component index (GPOS 5)
	CursiveEntry uint16 // entry anchor index (GPOS 3)
	CursiveExit  uint16 // exit anchor index (GPOS 3)
}

// PosItem stores positioning deltas and optional attachment metadata.
// Advances/offsets are in font units and are relative, not absolute.
type PosItem struct {
	XAdvance int32
	YAdvance int32
	XOffset  int32
	YOffset  int32

	AttachTo    int32
	AttachKind  AttachKind
	AttachClass uint16
	AnchorRef   AnchorRef

	Cluster uint32 // potentially used for shaping
	Flags   uint16 // TODO
}

// NewPosBuffer allocates a position buffer of length n.
// All items start with AttachTo = -1 (no attachment).
func NewPosBuffer(n int) PosBuffer {
	if n <= 0 {
		return PosBuffer{}
	}
	pb := make(PosBuffer, n)
	for i := range pb {
		pb[i].AttachTo = -1
	}
	return pb
}

// ResizeLike ensures the position buffer length matches the glyph buffer length.
// New items (if any) are initialized with AttachTo = -1.
func (pb PosBuffer) ResizeLike(buf GlyphBuffer) PosBuffer {
	n := buf.Len()
	if n == len(pb) {
		return pb
	}
	if n < len(pb) {
		return pb[:n]
	}
	out := make(PosBuffer, n)
	copy(out, pb)
	for i := len(pb); i < n; i++ {
		out[i].AttachTo = -1
	}
	return out
}

// ApplyEdit mirrors a GSUB edit to keep positional data aligned with glyph indices.
func (pb PosBuffer) ApplyEdit(edit *EditSpan) PosBuffer {
	if edit == nil {
		return pb
	}
	if edit.From < 0 || edit.To < edit.From || edit.To > len(pb) || edit.Len < 0 {
		panic("PosBuffer.ApplyEdit: invalid edit span")
	}
	repl := make(PosBuffer, edit.Len)
	for i := range repl {
		repl[i].AttachTo = -1
	}
	out := append(pb[:edit.From], repl...)
	out = append(out, pb[edit.To:]...)
	return out
}

// To apply a lookup, we have to iterate over the lookup's subtables and call them
// appropriately, respecting different subtable semantics and formats.
// Therefore this function more or less is a large switch to delegate to functions
// implementing a specific subtable logic.
func applyLookup(lookup *ot.Lookup, feat Feature, st *BufferState, alt int, gdef *ot.GDefTable, lookupList lookupNavigator) (int, bool, *EditSpan) {
	if lookup == nil {
		if st != nil {
			return st.Index, false, nil
		}
		return 0, false, nil
	}
	ctx := applyCtx{
		feat:       feat,
		lookup:     lookup,
		lookupList: lookupList,
		buf:        st,
		pos:        st.Index,
		alt:        alt,
		isGPos:     ot.IsGPosLookupType(lookup.Type),
		flag:       lookup.Flag,
		gdef:       gdef,
	}
	pos, ok, buf, pbuf, edit := dispatchLookup(&ctx)
	if st != nil {
		if buf != nil {
			st.Glyphs = buf
		}
		if pbuf != nil {
			st.Pos = pbuf
		}
		if edit != nil {
			st.ApplyEdit(edit)
		}
		st.Index = pos
	}
	return pos, ok, edit
}

func dispatchLookup(ctx *applyCtx) (int, bool, GlyphBuffer, PosBuffer, *EditSpan) {
	if ctx.lookup == nil {
		return ctx.pos, false, ctx.buf.Glyphs, ctx.buf.Pos, nil
	}
	lookupType := ot.GSubLookupType(ctx.lookup.Type)
	if ctx.isGPos {
		lookupType = ot.GPosLookupType(ctx.lookup.Type)
	}
	tracer().Debugf("applying lookup '%s'/%d flags=0x%04x", ctx.feat.Tag(), lookupType, uint16(ctx.lookup.Flag))
	for i := 0; i < int(ctx.lookup.SubTableCount) && ctx.pos < ctx.buf.Glyphs.Len(); i++ {
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
			pbuf PosBuffer
			edit *EditSpan
		)
		if ctx.isGPos {
			pos, ok, buf, pbuf, edit = dispatchGPosLookup(ctx, sub)
		} else {
			pos, ok, buf, pbuf, edit = dispatchGSubLookup(ctx, sub)
		}
		if ok {
			return pos, ok, buf, pbuf, edit
		}
	}
	return ctx.pos, false, ctx.buf.Glyphs, ctx.buf.Pos, nil
}

func dispatchGSubLookup(ctx *applyCtx, sub *ot.LookupSubtable) (int, bool, GlyphBuffer, PosBuffer, *EditSpan) {
	pos := ctx.pos
	ok := false
	buf := ctx.buf.Glyphs
	var edit *EditSpan
	switch sub.LookupType {
	case ot.GSubLookupTypeSingle: // Single Substitution Subtable
		switch sub.Format {
		case 1:
			pos, ok, buf, edit = gsubLookupType1Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 2:
			pos, ok, buf, edit = gsubLookupType1Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeMultiple: // Multiple Substitution Subtable
		pos, ok, buf, edit = gsubLookupType2Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
	case ot.GSubLookupTypeAlternate: // Alternate Substitution Subtable
		pos, ok, buf, edit = gsubLookupType3Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos, ctx.alt)
	case ot.GSubLookupTypeLigature: // Ligature Substitution Subtable
		pos, ok, buf, edit = gsubLookupType4Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
	case ot.GSubLookupTypeContext:
		switch sub.Format {
		case 1:
			pos, ok, buf, edit = gsubLookupType5Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 2:
			pos, ok, buf, edit = gsubLookupType5Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 3:
			pos, ok, buf, edit = gsubLookupType5Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeChainingContext:
		switch sub.Format {
		case 1:
			pos, ok, buf, edit = gsubLookupType6Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 2:
			pos, ok, buf, edit = gsubLookupType6Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 3:
			pos, ok, buf, edit = gsubLookupType6Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeExtensionSubs:
		tracer().Errorf("GSUB extension subtable reached dispatch; extension should be unwrapped during parsing")
	case ot.GSubLookupTypeReverseChaining:
		switch sub.Format {
		case 1:
			pos, ok, buf, edit = gsubLookupType8Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	default:
		tracer().Errorf("unknown GSUB lookup type %d/%d", sub.LookupType, sub.Format)
	}
	return pos, ok, buf, ctx.buf.Pos, edit
}

func dispatchGPosLookup(ctx *applyCtx, sub *ot.LookupSubtable) (int, bool, GlyphBuffer, PosBuffer, *EditSpan) {
	pos := ctx.pos
	ok := false
	buf := ctx.buf.Glyphs
	var edit *EditSpan
	switch sub.LookupType {
	case ot.GPosLookupTypeSingle,
		ot.GPosLookupTypePair,
		ot.GPosLookupTypeCursive,
		ot.GPosLookupTypeMarkToBase,
		ot.GPosLookupTypeMarkToLigature,
		ot.GPosLookupTypeMarkToMark,
		ot.GPosLookupTypeContextPos,
		ot.GPosLookupTypeChainedContextPos:
		switch sub.LookupType {
		case ot.GPosLookupTypeSingle:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType1Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit = gposLookupType1Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypePair:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType2Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit = gposLookupType2Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeCursive:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType3Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeMarkToBase:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType4Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeMarkToLigature:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType5Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeMarkToMark:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType6Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeContextPos:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType7Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit = gposLookupType7Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 3:
				pos, ok, buf, edit = gposLookupType7Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeChainedContextPos:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit = gposLookupType8Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit = gposLookupType8Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 3:
				pos, ok, buf, edit = gposLookupType8Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		}
		tracer().Errorf("GPOS lookup type %d/%d not implemented", sub.LookupType, sub.Format)
	case ot.GPosLookupTypeExtensionPos:
		tracer().Errorf("GPOS extension subtable reached dispatch; extension should be unwrapped during parsing")
	default:
		tracer().Errorf("unknown GPOS lookup type %d/%d", sub.LookupType, sub.Format)
	}
	return pos, ok, buf, ctx.buf.Pos, edit
}

// --- Helpers ---------------------------------------------------------------

// skipGlyph applies lookup-flags to decide whether to skip a glyph while
// matching with a coverage rule.
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

type singleMatchFn func(ctx *applyCtx, buf GlyphBuffer, pos int) (int, bool)
type matchSeqFn func(ctx *applyCtx, buf GlyphBuffer, pos int) ([]int, bool)

func matchCoverageForward(ctx *applyCtx, buf GlyphBuffer, pos int, cov ot.Coverage) (mpos, inx int, ok bool) {
	for i := pos; i < buf.Len(); {
		mpos, ok = nextMatchable(ctx, buf, i)
		if !ok {
			return 0, 0, false
		}
		if inx, ok = cov.Match(buf.At(mpos)); ok {
			return
		}
		i = mpos + 1
	}
	return 0, 0, false
}

type matchingCoveraveCtx struct {
	covs    []ot.Coverage
	pos     int
	dir     int
	offset  int
	matcher singleMatchFn
}

func matchCoverageSequenceForward(ctx *applyCtx, buf GlyphBuffer, pos int, covs []ot.Coverage) ([]int, bool) {
	mctx := matchingCoveraveCtx{
		covs:    covs,
		pos:     pos,
		dir:     1,
		offset:  0,
		matcher: nextMatchable,
	}
	return matchCoverageSequence(ctx, buf, mctx)
	// return matchCoverageSequence(ctx, buf, mctx)
	// if len(covs) == 0 {
	// 	return nil, false
	// }
	// out := make([]int, len(covs))
	// cur := pos
	// for i, cov := range covs {
	// 	mpos, ok := nextMatchable(ctx, buf, cur)
	// 	if !ok {
	// 		return nil, false
	// 	}
	// 	if _, ok := cov.Match(buf.At(mpos)); !ok {
	// 		return nil, false
	// 	}
	// 	out[i] = mpos
	// 	cur = mpos + 1
	// }
	// return out, true
}

func matchCoverageSequenceBackward(ctx *applyCtx, buf GlyphBuffer, pos int, covs []ot.Coverage) ([]int, bool) {
	mctx := matchingCoveraveCtx{
		covs:    covs,
		pos:     pos,
		dir:     -1,
		offset:  -1,
		matcher: prevMatchable,
	}
	return matchCoverageSequence(ctx, buf, mctx)
	// if len(covs) == 0 {
	// 	return nil, false
	// }
	// out := make([]int, len(covs))
	// cur := pos - 1
	// for i, cov := range covs {
	// 	mpos, ok := prevMatchable(ctx, buf, cur)
	// 	if !ok {
	// 		return nil, false
	// 	}
	// 	if _, ok := cov.Match(buf.At(mpos)); !ok {
	// 		return nil, false
	// 	}
	// 	out[i] = mpos
	// 	cur = mpos - 1
	// }
	// return out, true
}

func matchCoverageSequence(ctx *applyCtx, buf GlyphBuffer, matchCtx matchingCoveraveCtx) ([]int, bool) {
	if len(matchCtx.covs) == 0 {
		return nil, false
	}
	out := make([]int, len(matchCtx.covs))
	cur := matchCtx.pos + matchCtx.offset
	for i, cov := range matchCtx.covs {
		mpos, ok := matchCtx.matcher(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if _, ok := cov.Match(buf.At(mpos)); !ok {
			return nil, false
		}
		out[i] = mpos
		cur = mpos + matchCtx.dir
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

type matchingGlyphCtx struct {
	glyphs  []ot.GlyphIndex
	pos     int
	dir     int
	offset  int
	matcher singleMatchFn
}

func matchGlyphSequenceForward(ctx *applyCtx, buf GlyphBuffer, pos int, glyphs []ot.GlyphIndex) ([]int, bool) {
	mctx := matchingGlyphCtx{
		glyphs:  glyphs,
		pos:     pos,
		dir:     1,
		offset:  0,
		matcher: nextMatchable,
	}
	return matchGlyphSequence(ctx, buf, mctx)
	// if len(glyphs) == 0 {
	// 	return nil, false
	// }
	// out := make([]int, len(glyphs))
	// cur := pos
	// for i, gid := range glyphs {
	// 	mpos, ok := nextMatchable(ctx, buf, cur)
	// 	if !ok {
	// 		return nil, false
	// 	}
	// 	if buf.At(mpos) != gid {
	// 		return nil, false
	// 	}
	// 	out[i] = mpos
	// 	cur = mpos + 1
	// }
	// return out, true
}

func matchGlyphSequenceBackward(ctx *applyCtx, buf GlyphBuffer, pos int, glyphs []ot.GlyphIndex) ([]int, bool) {
	mctx := matchingGlyphCtx{
		glyphs:  glyphs,
		pos:     pos,
		dir:     -1,
		offset:  -1,
		matcher: prevMatchable,
	}
	return matchGlyphSequence(ctx, buf, mctx)
	// if len(glyphs) == 0 {
	// 	return nil, false
	// }
	// out := make([]int, len(glyphs))
	// cur := pos - 1
	// for i, gid := range glyphs {
	// 	mpos, ok := prevMatchable(ctx, buf, cur)
	// 	if !ok {
	// 		return nil, false
	// 	}
	// 	if buf.At(mpos) != gid {
	// 		return nil, false
	// 	}
	// 	out[i] = mpos
	// 	cur = mpos - 1
	// }
	// return out, true
}

func matchGlyphSequence(ctx *applyCtx, buf GlyphBuffer, matchCtx matchingGlyphCtx) ([]int, bool) {
	if len(matchCtx.glyphs) == 0 {
		return nil, false
	}
	out := make([]int, len(matchCtx.glyphs))
	cur := matchCtx.pos + matchCtx.offset
	for i, gid := range matchCtx.glyphs {
		mpos, ok := matchCtx.matcher(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if buf.At(mpos) != gid {
			return nil, false
		}
		out[i] = mpos
		cur = mpos + matchCtx.dir
	}
	return out, true
}

type matchingClassCtx struct {
	classDef ot.ClassDefinitions
	classes  []uint16
	pos      int
	dir      int
	offset   int
	matcher  singleMatchFn
}

func matchClassSequenceForward(ctx *applyCtx, buf GlyphBuffer, pos int, classDef ot.ClassDefinitions, classes []uint16) ([]int, bool) {
	mctx := matchingClassCtx{
		classDef: classDef,
		classes:  classes,
		pos:      pos,
		dir:      1,
		offset:   0,
		matcher:  nextMatchable,
	}
	return matchClassSequence(ctx, buf, mctx)
	// out := make([]int, len(classes))
	// cur := pos
	// for i, clz := range classes {
	// 	mpos, ok := nextMatchable(ctx, buf, cur)
	// 	if !ok {
	// 		return nil, false
	// 	}
	// 	if uint16(classDef.Lookup(buf.At(mpos))) != clz {
	// 		return nil, false
	// 	}
	// 	out[i] = mpos
	// 	cur = mpos + 1
	// }
	// return out, true
}

func matchClassSequenceBackward(ctx *applyCtx, buf GlyphBuffer, pos int, classDef ot.ClassDefinitions, classes []uint16) ([]int, bool) {
	mctx := matchingClassCtx{
		classDef: classDef,
		classes:  classes,
		pos:      pos,
		dir:      -1,
		offset:   -1,
		matcher:  prevMatchable,
	}
	return matchClassSequence(ctx, buf, mctx)
	// if len(classes) == 0 {
	// 	return nil, false
	// }
	// out := make([]int, len(classes))
	// cur := pos - 1
	// for i, clz := range classes {
	// 	mpos, ok := prevMatchable(ctx, buf, cur)
	// 	if !ok {
	// 		return nil, false
	// 	}
	// 	if uint16(classDef.Lookup(buf.At(mpos))) != clz {
	// 		return nil, false
	// 	}
	// 	out[i] = mpos
	// 	cur = mpos - 1
	// }
	// return out, true
}

func matchClassSequence(ctx *applyCtx, buf GlyphBuffer, matchCtx matchingClassCtx) ([]int, bool) {
	if len(matchCtx.classes) == 0 {
		return nil, false
	}
	out := make([]int, len(matchCtx.classes))
	cur := matchCtx.pos + matchCtx.dir
	for i, clz := range matchCtx.classes {
		mpos, ok := matchCtx.matcher(ctx, buf, cur)
		if !ok {
			return nil, false
		}
		if uint16(matchCtx.classDef.Lookup(buf.At(mpos))) != clz {
			return nil, false
		}
		out[i] = mpos
		cur = mpos + matchCtx.dir // advance to next potential matching position
	}
	return out, true
}

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
	posBuf PosBuffer,
	matchPositions []int,
	records []ot.SequenceLookupRecord,
	lookupList lookupNavigator,
	feat Feature,
	alt int,
	gdef *ot.GDefTable,
) (GlyphBuffer, PosBuffer, bool) {
	mapIdx := buildInputMap(matchPositions)
	if lookupList == nil || len(mapIdx) == 0 {
		return buf, posBuf, false
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
		st := NewBufferState(buf, posBuf)
		st.Index = targetPos
		_, ok, edit := applyLookup(&lookup, feat, st, alt, gdef, lookupList)
		if !ok {
			continue
		}
		applied = true
		buf = st.Glyphs
		posBuf = st.Pos
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
	return buf, posBuf, applied
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
