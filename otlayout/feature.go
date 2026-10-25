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
	Tag() ot.Tag         // e.g., 'liga'
	Type() LayoutTagType // GSUB or GPOS ?
	LookupCount() int    // number of Lookups for this feature
	LookupIndex(int) int // get index of lookup #i
}

// feature is the default implementation of Feature. Other, more spezialized Feature
// implementations will build on top of this.
type feature struct {
	typ           LayoutTagType
	tag           ot.Tag
	lookupIndices []int
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
		sg := t.ScriptGraph()
		fg := t.FeatureGraph()
		if sg == nil || fg == nil {
			return nil, nil, errors.New("layout table has no concrete script/feature graph")
		}
		scr := sg.Script(script)
		if scr == nil && script != ot.DFLT {
			scr = sg.Script(ot.DFLT)
		}
		if scr == nil {
			tracer().Infof("font has no feature-links from script %s", script)
			feats[i] = []Feature{}
			continue
		}
		tracer().Debugf("found script table for '%s'", script)
		var lsys *ot.LangSys
		if lang != 0 {
			lsys = scr.LangSys(lang)
		}
		if lsys == nil {
			lsys = scr.DefaultLangSys()
		}
		if lsys == nil {
			return nil, nil, errFontFormat(fmt.Sprintf("font has empty LangSys entry for %s",
				script)) // I am not quite sure if this is really illegal
		}
		featureByPtr := make(map[*ot.Feature]ot.Tag, fg.Len())
		for tag, cf := range fg.Range() {
			if cf != nil {
				featureByPtr[cf] = tag
			}
		}
		concreteFeatures := lsys.Features()
		feats[i] = make([]Feature, 0, 1+len(concreteFeatures))
		if reqInx, ok := lsys.RequiredFeatureIndex(); ok {
			cf, tag := featureAtConcreteIndex(fg, int(reqInx))
			feats[i] = append(feats[i], wrapConcreteFeature(cf, tag, i))
		} else {
			feats[i] = append(feats[i], nil) // mandatory feature slot
		}
		for j, cf := range concreteFeatures {
			if cf == nil {
				feats[i] = append(feats[i], nil)
				continue
			}
			tag := featureByPtr[cf]
			wrapped := wrapConcreteFeature(cf, tag, i)
			feats[i] = append(feats[i], wrapped)
			tracer().Debugf("%2d: feat[%v] ", j+1, wrapped.Tag())
		}
	}
	return feats[0], feats[1], nil
}

func wrapConcreteFeature(cf *ot.Feature, tag ot.Tag, which int) Feature {
	if cf == nil {
		return nil
	}
	lookups := make([]int, 0, cf.LookupCount())
	for i := 0; i < cf.LookupCount(); i++ {
		lookups = append(lookups, cf.LookupIndex(i))
	}
	f := feature{
		tag:           tag,
		lookupIndices: lookups,
	}
	if which == 0 {
		f.typ = GSubFeatureType
	} else {
		f.typ = GPosFeatureType
	}
	return f
}

func featureAtConcreteIndex(fg *ot.FeatureList, inx int) (*ot.Feature, ot.Tag) {
	if fg == nil || inx < 0 {
		return nil, 0
	}
	i := 0
	for tag, cf := range fg.Range() {
		if i == inx {
			return cf, tag
		}
		i++
	}
	return nil, 0
}

// Tag returns the identifying tag of this feature.
func (f feature) Tag() ot.Tag {
	return f.tag
}

// Type returns wether this is a GSUB-feature or a GPOS-feature.
func (f feature) Type() LayoutTagType {
	return f.typ
}

// LookupCount returns the number of lookup entries for a feature.
func (f feature) LookupCount() int {
	return len(f.lookupIndices)
}

// LookupIndex gets the index-position of lookup number i.
func (f feature) LookupIndex(i int) int {
	if i < 0 || i >= len(f.lookupIndices) {
		return -1
	}
	return f.lookupIndices[i]
}

// --- Feature application ---------------------------------------------------

// ApplyFeature applies a feature starting at st.Index, updating the buffer state.
// It returns the resulting position and whether any lookup was applied. Application
// does not necessarily change a glyph or position value: a zero-delta substitution
// can still be applied. The boolean is not an error/success indicator.
//
// A normal non-match leaves the buffer and position unchanged and returns false.
// A nil feature represents an unused mandatory-feature slot and is a no-op when
// st is non-nil.
//
// A non-no-op application requires a valid maxp glyph count. Missing or invalid
// count information returns an error before changing st. Lookup errors stop
// application and are wrapped with feature, lookup, and subtable context.
// Earlier applications are not rolled back: applied may be true alongside an
// error. See doc/FEATURE_APPLICATION.md for the remaining staged guarantees.
func ApplyFeature(otf *ot.Font, feat Feature, st *BufferState, alt int) (int, bool, error) {
	if feat == nil { // this is legal for unused mandatory feature slots
		if st == nil {
			return 0, false, nil
		}
		return st.Index, false, nil
	} else if st == nil || st.Glyphs == nil || st.Index < 0 || st.Index >= len(st.Glyphs) {
		tracer().Infof("application of font-feature requested for unusable buffer condition")
		if st != nil {
			return st.Index, false, errors.New("feature application: unusable buffer state")
		}
		return 0, false, errors.New("feature application: missing buffer state")
	}
	env, err := newApplyEnv(otf)
	if err != nil {
		return st.Index, false, err
	}
	var lytTable *ot.LayoutTable
	if feat.Type() == GSubFeatureType {
		if table := otf.Table(ot.T("GSUB")); table != nil && table.Self().AsGSub() != nil {
			lytTable = &table.Self().AsGSub().LayoutTable
		}
	} else if feat.Type() == GPosFeatureType {
		if table := otf.Table(ot.T("GPOS")); table != nil && table.Self().AsGPos() != nil {
			lytTable = &table.Self().AsGPos().LayoutTable
		}
	}
	if lytTable == nil {
		return st.Index, false, fmt.Errorf("feature %s: missing layout table or invalid feature type %d", feat.Tag(), feat.Type())
	}
	st.edits = nil
	var applied, ok bool
	gdef := otf.Layout.GDef
	lookupGraph := lytTable.LookupGraph()
	if lookupGraph == nil {
		return st.Index, false, fmt.Errorf("feature %s: missing lookup graph", feat.Tag())
	}
	if err := lookupGraph.Error(); err != nil {
		return st.Index, false, fmt.Errorf("feature %s: lookup graph: %w", feat.Tag(), err)
	}
	for i := 0; i < feat.LookupCount(); i++ { // lookups have to be applied in sequence
		inx := feat.LookupIndex(i)
		tracer().Debugf("feature %s lookup #%d => index %d", feat.Tag(), i, inx)
		clookup := lookupGraph.Lookup(inx)
		_, ok, _, err = applyLookupConcrete(clookup, lookupGraph, feat, st, alt, gdef, env)
		applied = applied || ok
		if err != nil {
			return st.Index, applied, fmt.Errorf("feature %s lookup %d: %w", feat.Tag(), inx, err)
		}
	}
	return st.Index, applied, nil
}

// applyEnv is a read-only snapshot of font validation data, passed by value to
// every lookup, including nested lookups. It does not belong to BufferState.
type applyEnv struct {
	numGlyphs int
}

func newApplyEnv(font *ot.Font) (applyEnv, error) {
	if font == nil {
		return applyEnv{}, errors.New("feature application: font is nil; maxp glyph count unavailable")
	}
	table := font.Table(ot.T("maxp"))
	if table == nil {
		return applyEnv{}, errors.New("feature application: missing maxp glyph count")
	}
	maxp := table.Self().AsMaxP()
	if maxp == nil {
		return applyEnv{}, errors.New("feature application: invalid maxp table")
	}
	if maxp.NumGlyphs <= 0 || maxp.NumGlyphs > 0xffff {
		return applyEnv{}, fmt.Errorf("feature application: invalid maxp.NumGlyphs %d", maxp.NumGlyphs)
	}
	return applyEnv{numGlyphs: maxp.NumGlyphs}, nil
}

// applyCtx bundles immutable lookup state for dispatch and helpers.
type applyCtx struct {
	env         applyEnv                 // immutable font validation data
	feat        Feature                  // active feature for alternate selection and tracing
	clookup     *ot.LookupTable          // concrete lookup currently being applied
	lookupGraph *ot.LookupListGraph      // concrete lookup graph for nested lookups
	buf         *BufferState             // buffer state (glyphs + positions)
	pos         int                      // current glyph position in buffer
	alt         int                      // alternate index (1..n) for substitution selection
	flag        ot.LayoutTableLookupFlag // lookup flags for ignore/mark filtering
	gdef        *ot.GDefTable            // GDEF table for glyph classification, if present
	subnode     *ot.LookupNode           // effective concrete node for current subtable dispatch
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
	edits        []EditSpan // ordered mutations, in coordinates immediately before each edit
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
	b.edits = append(b.edits, EditSpan{From: i, To: i + 1, Len: 1})
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

// EnsurePos prepares a uniquely owned position buffer for GPOS writes.
func (b *BufferState) EnsurePos() {
	if b == nil {
		return
	}
	if b.Pos == nil {
		b.Pos = NewPosBuffer(len(b.Glyphs))
		b.posShared = false
		return
	}
	b.ensureUniquePos()
	if len(b.Pos) != len(b.Glyphs) {
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
	b.edits = append(b.edits, *edit)
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
	out := append(pb[:edit.From:edit.From], repl...)
	out = append(out, pb[edit.To:]...)
	return out
}

// applyLookupConcrete retains returned buffers and edits even on error.
// The applied flag describes completed work, not absence of errors.
func applyLookupConcrete(
	clookup *ot.LookupTable,
	lookupGraph *ot.LookupListGraph,
	feat Feature,
	st *BufferState,
	alt int,
	gdef *ot.GDefTable,
	env applyEnv,
) (int, bool, *EditSpan, error) {
	if st == nil {
		return 0, false, nil, errors.New("lookup: missing buffer state")
	}
	if st.Index < 0 || st.Index > st.Len() || feat == nil {
		return st.Index, false, nil, errors.New("lookup: invalid cursor or missing feature")
	}
	if clookup == nil {
		return st.Index, false, nil, errors.New("lookup: missing or out-of-range lookup")
	}
	if err := clookup.Error(); err != nil {
		return st.Index, false, nil, fmt.Errorf("lookup table: %w", err)
	}
	ctx := applyCtx{
		env:         env,
		feat:        feat,
		clookup:     clookup,
		lookupGraph: lookupGraph,
		buf:         st,
		pos:         st.Index,
		alt:         alt,
		flag:        clookup.Flag,
		gdef:        gdef,
	}
	pos, ok, buf, pbuf, edit, err := dispatchLookup(&ctx)
	if st != nil {
		if buf != nil {
			st.Glyphs = buf
		}
		if pbuf != nil {
			st.Pos = pbuf
		}
		st.Index = pos
	}
	return pos, ok, edit, err
}

func dispatchLookup(ctx *applyCtx) (int, bool, GlyphBuffer, PosBuffer, *EditSpan, error) {
	if ctx.clookup == nil {
		return ctx.pos, false, ctx.buf.Glyphs, ctx.buf.Pos, nil, errors.New("missing lookup table")
	}
	isGPos := ot.IsGPosLookupType(ctx.clookup.Type)
	lookupType := ot.GSubLookupType(ctx.clookup.Type)
	if isGPos {
		lookupType = ot.GPosLookupType(ctx.clookup.Type)
	}
	tracer().Debugf("applying lookup '%s'/%d flags=0x%04x", ctx.feat.Tag(), lookupType, uint16(ctx.clookup.Flag))
	for i := 0; i < int(ctx.clookup.SubTableCount) && ctx.pos < ctx.buf.Glyphs.Len(); i++ {
		tracer().Debugf("-------------------- pos = %d", ctx.pos)
		subnode, err := effectiveLookupNode(ctx.clookup.Subtable(i))
		ctx.subnode = subnode
		if err != nil {
			return ctx.pos, false, ctx.buf.Glyphs, ctx.buf.Pos, nil, fmt.Errorf("subtable %d: %w", i, err)
		}
		subType := subnode.LookupType
		if isGPos {
			subType = ot.GPosLookupType(subType)
		} else {
			subType = ot.GSubLookupType(subType)
		}
		tracer().Debugf("subtable #%d type %d format %d", i, subType, subnode.Format)
		var (
			pos  int
			ok   bool
			buf  GlyphBuffer
			pbuf PosBuffer
			edit *EditSpan
		)
		if isGPos {
			pos, ok, buf, pbuf, edit, err = dispatchGPosLookup(ctx, subnode)
		} else {
			pos, ok, buf, pbuf, edit, err = dispatchGSubLookup(ctx, subnode)
		}
		if err != nil {
			return pos, ok, buf, pbuf, edit, fmt.Errorf("subtable %d type %d format %d: %w", i, subType, subnode.Format, err)
		}
		if ok {
			return pos, ok, buf, pbuf, edit, err
		}
	}
	return ctx.pos, false, ctx.buf.Glyphs, ctx.buf.Pos, nil, nil
}

func dispatchGSubLookup(ctx *applyCtx, sub *ot.LookupNode) (int, bool, GlyphBuffer, PosBuffer, *EditSpan, error) {
	pos := ctx.pos
	ok := false
	buf := ctx.buf.Glyphs
	var edit *EditSpan
	err := fmt.Errorf("unsupported lookup type %d format %d", sub.LookupType, sub.Format)
	subType := ot.GSubLookupType(sub.LookupType)
	switch subType {
	case ot.GSubLookupTypeSingle: // Single Substitution Subtable
		switch sub.Format {
		case 1:
			pos, ok, buf, edit, err = gsubLookupType1Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 2:
			pos, ok, buf, edit, err = gsubLookupType1Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeMultiple: // Multiple Substitution Subtable
		if sub.Format == 1 {
			pos, ok, buf, edit, err = gsubLookupType2Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeAlternate: // Alternate Substitution Subtable
		if sub.Format == 1 {
			pos, ok, buf, edit, err = gsubLookupType3Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos, ctx.alt)
		}
	case ot.GSubLookupTypeLigature: // Ligature Substitution Subtable
		if sub.Format == 1 {
			pos, ok, buf, edit, err = gsubLookupType4Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeContext:
		switch sub.Format {
		case 1:
			pos, ok, buf, edit, err = gsubLookupType5Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 2:
			pos, ok, buf, edit, err = gsubLookupType5Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 3:
			pos, ok, buf, edit, err = gsubLookupType5Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeChainingContext:
		switch sub.Format {
		case 1:
			pos, ok, buf, edit, err = gsubLookupType6Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 2:
			pos, ok, buf, edit, err = gsubLookupType6Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		case 3:
			pos, ok, buf, edit, err = gsubLookupType6Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	case ot.GSubLookupTypeReverseChaining:
		switch sub.Format {
		case 1:
			pos, ok, buf, edit, err = gsubLookupType8Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
		}
	}
	return pos, ok, buf, ctx.buf.Pos, edit, err
}

func dispatchGPosLookup(ctx *applyCtx, sub *ot.LookupNode) (int, bool, GlyphBuffer, PosBuffer, *EditSpan, error) {
	pos := ctx.pos
	ok := false
	buf := ctx.buf.Glyphs
	var edit *EditSpan
	err := fmt.Errorf("unsupported lookup type %d format %d", sub.LookupType, sub.Format)
	subType := ot.GPosLookupType(sub.LookupType)
	switch subType {
	case ot.GPosLookupTypeSingle,
		ot.GPosLookupTypePair,
		ot.GPosLookupTypeCursive,
		ot.GPosLookupTypeMarkToBase,
		ot.GPosLookupTypeMarkToLigature,
		ot.GPosLookupTypeMarkToMark,
		ot.GPosLookupTypeContextPos,
		ot.GPosLookupTypeChainedContextPos:
		switch subType {
		case ot.GPosLookupTypeSingle:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType1Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit, err = gposLookupType1Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypePair:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType2Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit, err = gposLookupType2Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeCursive:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType3Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeMarkToBase:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType4Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeMarkToLigature:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType5Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeMarkToMark:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType6Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeContextPos:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType7Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit, err = gposLookupType7Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 3:
				pos, ok, buf, edit, err = gposLookupType7Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		case ot.GPosLookupTypeChainedContextPos:
			switch sub.Format {
			case 1:
				pos, ok, buf, edit, err = gposLookupType8Fmt1(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 2:
				pos, ok, buf, edit, err = gposLookupType8Fmt2(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			case 3:
				pos, ok, buf, edit, err = gposLookupType8Fmt3(ctx, sub, ctx.buf.Glyphs, ctx.pos)
			}
		}
	}
	return pos, ok, buf, ctx.buf.Pos, edit, err
}

// --- Helpers ---------------------------------------------------------------

// skipGlyph applies lookup-flags to decide whether to skip a glyph while
// matching with a coverage rule.
func skipGlyph(ctx *applyCtx, g ot.GlyphIndex) bool {
	if ctx == nil || ctx.gdef == nil {
		return false
	}
	if ctx.clookup == nil {
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
			setIndex := ctx.clookup.MarkFilteringSet()
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

func effectiveLookupNode(n *ot.LookupNode) (*ot.LookupNode, error) {
	if n == nil {
		return nil, errors.New("missing lookup subtable")
	}
	if err := n.Error(); err != nil {
		return nil, fmt.Errorf("lookup node: %w", err)
	}
	resolved := n
	if p := n.GSubPayload(); p != nil && p.ExtensionFmt1 != nil && p.ExtensionFmt1.Resolved != nil {
		resolved = p.ExtensionFmt1.Resolved
	}
	if p := n.GPosPayload(); p != nil && p.ExtensionFmt1 != nil && p.ExtensionFmt1.Resolved != nil {
		resolved = p.ExtensionFmt1.Resolved
	}
	if err := resolved.Error(); err != nil {
		return nil, fmt.Errorf("resolved extension node: %w", err)
	}
	return resolved, nil
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

// applySequenceLookupRecords applies nested lookups in record order. Its boolean
// accumulates whether any nested lookup applied. Nested operations can mutate the
// buffer incrementally; callers must not assume rollback. Errors retain completed
// edits in st's journal and return its current buffers. Each enclosing frame
// consumes those edits only to remap its own matched inputs, not to mutate buffers.
func applySequenceLookupRecords(
	st *BufferState,
	matchPositions []int,
	records []ot.SequenceLookupRecord,
	lookupGraph *ot.LookupListGraph,
	feat Feature,
	alt int,
	gdef *ot.GDefTable,
	env applyEnv,
) (GlyphBuffer, PosBuffer, bool, error) {
	// Share the actual owner, not just its slices: nested writes must retain
	// copy-on-write flags and append to the same edit journal.
	savedIndex := st.Index
	defer func() { st.Index = savedIndex }()
	mapIdx := buildInputMap(matchPositions)
	if len(records) == 0 {
		return st.Glyphs, st.Pos, false, nil
	}
	if lookupGraph == nil {
		return st.Glyphs, st.Pos, false, errors.New("context lookup: missing lookup graph")
	}
	if err := lookupGraph.Error(); err != nil {
		return st.Glyphs, st.Pos, false, fmt.Errorf("context lookup graph: %w", err)
	}

	applied := false
	for recordIndex, rec := range records {
		tracer().Debugf("sequence lookup record: seq=%d lookup=%d", rec.SequenceIndex, rec.LookupListIndex)
		seqIndex := int(rec.SequenceIndex)
		if seqIndex < 0 || seqIndex >= len(mapIdx) {
			return st.Glyphs, st.Pos, applied, fmt.Errorf("sequence record %d: input index %d outside %d matched positions", recordIndex, seqIndex, len(mapIdx))
		}
		targetPos := mapIdx[seqIndex]
		if targetPos < 0 { // an earlier record deleted this matched input
			continue
		}
		if targetPos >= st.Len() {
			return st.Glyphs, st.Pos, applied, fmt.Errorf("sequence record %d: target position %d outside buffer", recordIndex, targetPos)
		}
		tracer().Debugf("sequence lookup record: target position %d", targetPos)
		clookup := lookupGraph.Lookup(int(rec.LookupListIndex))
		checkpoint := len(st.edits)
		st.Index = targetPos
		_, ok, _, err := applyLookupConcrete(clookup, lookupGraph, feat, st, alt, gdef, env)
		applied = applied || ok
		if err != nil {
			return st.Glyphs, st.Pos, applied, fmt.Errorf("sequence record %d input %d lookup %d: %w", recordIndex, seqIndex, rec.LookupListIndex, err)
		}
		for _, edit := range st.edits[checkpoint:] {
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
	}
	return st.Glyphs, st.Pos, applied, nil
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
