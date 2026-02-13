package otshape

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

type planTable uint8

const (
	planGSUB planTable = iota
	planGPOS
)

func (t planTable) String() string {
	assert(t == planGSUB || t == planGPOS, "plan table is neither GSUB nor GPOS")
	switch t {
	case planGSUB:
		return "GSUB"
	case planGPOS:
		return "GPOS"
	}
	panic("unreachable")
}

type lookupRunFlags uint8

const (
	lookupAutoZWNJ lookupRunFlags = 1 << iota
	lookupAutoZWJ
	lookupRandom
	lookupPerSyllable
)

func (f lookupRunFlags) has(wanted lookupRunFlags) bool {
	return f&wanted == wanted
}

type pauseHookID uint16

const noPauseHook pauseHookID = 0

type pauseHook func(run *runBuffer) error

type segmentProps struct {
	Direction bidi.Direction
	Script    language.Script
	Language  language.Tag
}

type maskSpec struct {
	Mask         uint32
	Shift        uint8
	DefaultValue uint32
}

type maskLayout struct {
	GlobalMask uint32
	ByFeature  map[ot.Tag]maskSpec
}

type featureBind struct {
	Tag          ot.Tag
	FeatureIndex uint16
	Stage        int
	Mask         uint32
	Required     bool
}

type lookupOp struct {
	LookupIndex uint16
	FeatureTag  ot.Tag
	Mask        uint32
	Flags       lookupRunFlags
}

type stage struct {
	FirstLookup int // inclusive
	LastLookup  int // exclusive
	Pause       pauseHookID
}

type tableProgram struct {
	FoundScript  bool
	Stages       []stage
	Lookups      []lookupOp
	FeatureBinds []featureBind
}

func (tp tableProgram) lookupCount() int {
	return len(tp.Lookups)
}

func (tp tableProgram) stageCount() int {
	return len(tp.Stages)
}

func (tp tableProgram) lookupsForStage(stageIndex int) []lookupOp {
	if stageIndex < 0 || stageIndex >= len(tp.Stages) {
		return nil
	}
	st := tp.Stages[stageIndex]
	if st.FirstLookup < 0 || st.FirstLookup > st.LastLookup || st.LastLookup > len(tp.Lookups) {
		return nil
	}
	out := make([]lookupOp, st.LastLookup-st.FirstLookup)
	copy(out, tp.Lookups[st.FirstLookup:st.LastLookup])
	return out
}

func (tp tableProgram) validate(maxPauseID pauseHookID) error {
	last := 0
	for i, st := range tp.Stages {
		if st.FirstLookup < 0 || st.LastLookup < st.FirstLookup || st.LastLookup > len(tp.Lookups) {
			return fmt.Errorf("invalid stage %d lookup bounds [%d:%d) for %d lookups",
				i, st.FirstLookup, st.LastLookup, len(tp.Lookups))
		}
		if i > 0 && st.FirstLookup < last {
			return fmt.Errorf("stage %d starts before previous stage end", i)
		}
		last = st.LastLookup
		if st.Pause > maxPauseID {
			return fmt.Errorf("stage %d references unknown pause hook id %d", i, st.Pause)
		}
	}
	return nil
}

type planPolicy struct {
	Strict          bool // fail early on unsupported/incomplete OT data
	ApplyGPOS       bool // run GPOS stage at execution time
	ZeroMarks       bool // zero mark advances if enabled by script policy
	FallbackMarkPos bool // optional fallback mark positioning
}

type planHookSet struct {
	pause []pauseHook
}

func newPlanHookSet() planHookSet {
	return planHookSet{pause: []pauseHook{nil}} // reserve id 0 (noPauseHook)
}

func (hs *planHookSet) addPause(fn pauseHook) pauseHookID {
	if hs == nil || fn == nil {
		return noPauseHook
	}
	if len(hs.pause) == 0 {
		hs.pause = append(hs.pause, nil)
	}
	hs.pause = append(hs.pause, fn)
	return pauseHookID(len(hs.pause) - 1)
}

func (hs planHookSet) pauseHook(id pauseHookID) (pauseHook, bool) {
	if id == noPauseHook || int(id) >= len(hs.pause) {
		return nil, false
	}
	fn := hs.pause[id]
	return fn, fn != nil
}

func (hs planHookSet) maxPauseID() pauseHookID {
	if len(hs.pause) == 0 {
		return noPauseHook
	}
	return pauseHookID(len(hs.pause) - 1)
}

func (hs planHookSet) clone() planHookSet {
	out := planHookSet{pause: make([]pauseHook, len(hs.pause))}
	copy(out.pause, hs.pause)
	return out
}

type planNoteLevel uint8

const (
	planNoteInfo planNoteLevel = iota
	planNoteWarning
)

type planNote struct {
	Level   planNoteLevel
	Message string
}

type plan struct {
	font *ot.Font

	Props     segmentProps
	ScriptTag ot.Tag
	LangTag   ot.Tag
	VarIndex  [2]int // GSUB, GPOS variation selection (-1 if none)

	Masks maskLayout
	GSUB  tableProgram
	GPOS  tableProgram

	Policy planPolicy
	Hooks  planHookSet
	Notes  []planNote

	featureRanges    []FeatureRange          // preserved for runtime mask setup
	joinerGlyphClass map[ot.GlyphIndex]uint8 // GSUB-time joiner annotation by glyph
}

func (p *plan) table(t planTable) *tableProgram {
	assert(t == planGPOS || t == planGSUB, "invalid table type")
	if t == planGPOS {
		return &p.GPOS
	}
	return &p.GSUB
}

func (p *plan) maskForFeature(tag ot.Tag) (maskSpec, bool) {
	if p == nil || p.Masks.ByFeature == nil {
		return maskSpec{}, false
	}
	ms, ok := p.Masks.ByFeature[tag]
	return ms, ok
}

func (p *plan) validate() error {
	if p == nil {
		return errors.New("plan is nil")
	}
	if p.Masks.ByFeature == nil {
		return errors.New("plan masks are uninitialized")
	}
	maxPauseID := p.Hooks.maxPauseID()
	if err := p.GSUB.validate(maxPauseID); err != nil {
		return fmt.Errorf("GSUB program invalid: %w", err)
	}
	if err := p.GPOS.validate(maxPauseID); err != nil {
		return fmt.Errorf("GPOS program invalid: %w", err)
	}
	return nil
}

// --- Compiling Plans --------------------------------------------------

type planRequest struct {
	Font         *ot.Font
	Props        segmentProps
	ScriptTag    ot.Tag
	LangTag      ot.Tag
	UserFeatures []FeatureRange
	VarIndex     [2]int
	Policy       planPolicy
	Hooks        planHookSet
}

var defaultGSUBFeatures = []ot.Tag{
	ot.T("locl"),
	ot.T("ccmp"),
	ot.T("rlig"),
	ot.T("rclt"),
	ot.T("calt"),
	ot.T("clig"),
	ot.T("liga"),
}

var defaultGPOSFeatures = []ot.Tag{
	ot.T("abvm"),
	ot.T("blwm"),
	ot.T("mark"),
	ot.T("mkmk"),
	ot.T("curs"),
	ot.T("dist"),
	ot.T("kern"),
}

var manualJoinerBothFeatures = map[ot.Tag]struct{}{
	ot.T("mark"): {},
	ot.T("mkmk"): {},
}

// Arabic shaping lookups usually need explicit ZWJ handling by the script shaper.
var manualZWJFeatures = map[ot.Tag]struct{}{
	ot.T("ccmp"): {},
	ot.T("locl"): {},
	ot.T("isol"): {},
	ot.T("fina"): {},
	ot.T("fin2"): {},
	ot.T("fin3"): {},
	ot.T("medi"): {},
	ot.T("med2"): {},
	ot.T("init"): {},
	ot.T("rlig"): {},
	ot.T("calt"): {},
	ot.T("rclt"): {},
	ot.T("liga"): {},
	ot.T("clig"): {},
	ot.T("mset"): {},
}

// Indic and similar shaping features should be contained to one syllable.
var perSyllableFeatures = map[ot.Tag]struct{}{
	ot.T("rphf"): {},
	ot.T("pref"): {},
	ot.T("blwf"): {},
	ot.T("half"): {},
	ot.T("abvf"): {},
	ot.T("pstf"): {},
	ot.T("vatu"): {},
	ot.T("cjct"): {},
	ot.T("pres"): {},
	ot.T("abvs"): {},
	ot.T("blws"): {},
	ot.T("psts"): {},
	ot.T("haln"): {},
}

func lookupFlagsForFeature(table planTable, tag ot.Tag) lookupRunFlags {
	flags := lookupAutoZWJ | lookupAutoZWNJ
	if _, ok := manualJoinerBothFeatures[tag]; ok {
		flags &^= lookupAutoZWJ | lookupAutoZWNJ
	}
	if _, ok := manualZWJFeatures[tag]; ok {
		flags &^= lookupAutoZWJ
	}
	if table == planGSUB {
		if _, ok := perSyllableFeatures[tag]; ok {
			flags |= lookupPerSyllable
		}
	}
	if tag == ot.T("rand") {
		flags |= lookupRandom
	}
	return flags
}

type userFeatureToggle struct {
	on        bool
	arg       int
	hasRange  bool
	mentioned bool
}

func collectUserFeatureToggles(features []FeatureRange) map[ot.Tag]userFeatureToggle {
	toggles := make(map[ot.Tag]userFeatureToggle)
	for _, f := range features {
		if f.Feature == 0 {
			continue
		}
		toggles[f.Feature] = userFeatureToggle{
			on:        f.On,
			arg:       f.Arg,
			hasRange:  f.Start != 0 || f.End != 0,
			mentioned: true,
		}
	}
	return toggles
}

type compiledFeature struct {
	tag     ot.Tag
	typ     otlayout.LayoutTagType
	lookups []int
}

func (f compiledFeature) Tag() ot.Tag                  { return f.tag }
func (f compiledFeature) Type() otlayout.LayoutTagType { return f.typ }
func (f compiledFeature) LookupCount() int             { return len(f.lookups) }
func (f compiledFeature) LookupIndex(i int) int {
	if i < 0 || i >= len(f.lookups) {
		return -1
	}
	return f.lookups[i]
}

func fontFeaturesForTable(font *ot.Font, table planTable, scriptTag ot.Tag, langTag ot.Tag) ([]otlayout.Feature, error) {
	if font == nil {
		return nil, errShaper("font is nil")
	}
	var (
		tag ot.Tag
		typ otlayout.LayoutTagType
		lyt *ot.LayoutTable
	)
	switch table {
	case planGSUB:
		tag = ot.T("GSUB")
		typ = otlayout.GSubFeatureType
		if t := font.Table(tag); t != nil {
			if gsub := t.Self().AsGSub(); gsub != nil {
				lyt = &gsub.LayoutTable
			}
		}
	case planGPOS:
		tag = ot.T("GPOS")
		typ = otlayout.GPosFeatureType
		if t := font.Table(tag); t != nil {
			if gpos := t.Self().AsGPos(); gpos != nil {
				lyt = &gpos.LayoutTable
			}
		}
	default:
		return nil, errShaper("invalid plan table")
	}
	if lyt == nil {
		return nil, errShaper(fmt.Sprintf("font has no %s table", tag))
	}
	sg := lyt.ScriptGraph()
	fg := lyt.FeatureGraph()
	if sg == nil || fg == nil {
		return nil, errShaper(fmt.Sprintf("%s has no script or feature graph", tag))
	}
	if scriptTag == 0 {
		scriptTag = ot.DFLT
	}
	scr := sg.Script(scriptTag)
	if scr == nil && scriptTag != ot.DFLT {
		scr = sg.Script(ot.DFLT)
	}
	if scr == nil {
		return []otlayout.Feature{}, nil
	}
	var lsys *ot.LangSys
	if langTag != 0 {
		lsys = scr.LangSys(langTag)
	}
	if lsys == nil {
		lsys = scr.DefaultLangSys()
	}
	if lsys == nil {
		return nil, errShaper(fmt.Sprintf("%s has no language system for script %s", tag, scriptTag))
	}
	featureByPtr := make(map[*ot.Feature]ot.Tag, fg.Len())
	for featureTag, cf := range fg.Range() {
		if cf != nil {
			featureByPtr[cf] = featureTag
		}
	}
	features := lsys.Features()
	out := make([]otlayout.Feature, 0, 1+len(features))
	if reqInx, ok := lsys.RequiredFeatureIndex(); ok {
		cf, reqTag := featureAtConcreteIndex(fg, int(reqInx))
		if cf != nil && reqTag != 0 {
			out = append(out, wrapCompiledFeature(cf, reqTag, typ))
		} else {
			out = append(out, nil)
		}
	} else {
		out = append(out, nil)
	}
	for _, cf := range features {
		if cf == nil {
			out = append(out, nil)
			continue
		}
		featureTag := featureByPtr[cf]
		if featureTag == 0 {
			out = append(out, nil)
			continue
		}
		out = append(out, wrapCompiledFeature(cf, featureTag, typ))
	}
	return out, nil
}

func wrapCompiledFeature(cf *ot.Feature, tag ot.Tag, typ otlayout.LayoutTagType) otlayout.Feature {
	lookups := make([]int, 0, cf.LookupCount())
	for i := 0; i < cf.LookupCount(); i++ {
		lookups = append(lookups, cf.LookupIndex(i))
	}
	return compiledFeature{
		tag:     tag,
		typ:     typ,
		lookups: lookups,
	}
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

func compileUserFeatureMasks(features []FeatureRange) (maskLayout, error) {
	layout := maskLayout{
		GlobalMask: 0,
		ByFeature:  make(map[ot.Tag]maskSpec),
	}
	if len(features) == 0 {
		return layout, nil
	}
	var nextBit uint8
	for _, f := range features {
		if f.Feature == 0 {
			continue
		}
		if _, exists := layout.ByFeature[f.Feature]; exists {
			continue // first occurrence wins in this scaffold
		}
		if nextBit >= 31 {
			return maskLayout{}, errShaper("too many user features for uint32 mask layout")
		}
		maxValue := uint32(1)
		if f.Arg > 0 {
			maxValue = uint32(f.Arg)
		}
		bitsNeeded := bitStorage32(maxValue)
		if bitsNeeded == 0 {
			bitsNeeded = 1
		}
		if bitsNeeded > 8 {
			bitsNeeded = 8
		}
		if int(nextBit)+bitsNeeded > 31 {
			return maskLayout{}, errShaper("mask bit budget exhausted")
		}
		mask := uint32((1<<bitsNeeded)-1) << nextBit
		def := uint32(0)
		if f.On {
			if f.Arg > 0 {
				def = uint32(f.Arg)
			} else {
				def = 1
			}
			layout.GlobalMask |= (def << nextBit) & mask
		}
		layout.ByFeature[f.Feature] = maskSpec{
			Mask:         mask,
			Shift:        nextBit,
			DefaultValue: def,
		}
		nextBit += uint8(bitsNeeded)
	}
	return layout, nil
}

func compileTableProgram(
	features []otlayout.Feature,
	table planTable,
	defaultTags []ot.Tag,
	toggles map[ot.Tag]userFeatureToggle,
	masks maskLayout,
	policy planPolicy,
) (tableProgram, []planNote, error) {
	prog := tableProgram{
		FoundScript: len(features) > 0,
		Stages:      []stage{},
		Lookups:     []lookupOp{},
	}
	notes := make([]planNote, 0)
	if len(features) == 0 {
		return prog, notes, nil
	}

	available := make(map[ot.Tag]otlayout.Feature, len(features))
	required := make(map[ot.Tag]bool)
	for i, feat := range features {
		if feat == nil {
			continue
		}
		tag := feat.Tag()
		available[tag] = feat
		if i == 0 { // otlayout reserves slot 0 for required feature when present
			required[tag] = true
		}
	}
	active := make(map[ot.Tag]bool, len(available))
	for tag := range required {
		active[tag] = true
	}
	for _, tag := range defaultTags {
		if _, ok := available[tag]; ok {
			active[tag] = true
		}
	}
	for tag, t := range toggles {
		_, ok := available[tag]
		if !ok {
			if t.on && policy.Strict {
				return prog, notes, errShaper(fmt.Sprintf("feature %s requested but not available in %s", tag, table))
			}
			if t.mentioned {
				notes = append(notes, planNote{
					Level:   planNoteWarning,
					Message: fmt.Sprintf("feature %s ignored in %s (not available)", tag, table),
				})
			}
			continue
		}
		if required[tag] && !t.on {
			notes = append(notes, planNote{
				Level:   planNoteWarning,
				Message: fmt.Sprintf("required feature %s in %s cannot be disabled", tag, table),
			})
			continue
		}
		active[tag] = t.on
	}

	stageByTag := make(map[ot.Tag]int, len(available))
	stageNo := 0
	for tag := range required {
		if active[tag] {
			stageByTag[tag] = stageNo
		}
	}
	if len(required) > 0 {
		stageNo++
	}
	for _, tag := range defaultTags {
		if !active[tag] || required[tag] {
			continue
		}
		if _, exists := stageByTag[tag]; exists {
			continue
		}
		stageByTag[tag] = stageNo
		stageNo++
	}
	remaining := make([]ot.Tag, 0, len(active))
	for tag, on := range active {
		if !on {
			continue
		}
		if _, exists := stageByTag[tag]; exists {
			continue
		}
		remaining = append(remaining, tag)
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i] < remaining[j] })
	for _, tag := range remaining {
		stageByTag[tag] = stageNo
		stageNo++
	}

	type stageLookupBucket map[uint16]lookupOp
	stageBuckets := make(map[int]stageLookupBucket)
	for i, feat := range features {
		if feat == nil {
			continue
		}
		tag := feat.Tag()
		if !active[tag] {
			continue
		}
		mask := uint32(0)
		if ms, ok := masks.ByFeature[tag]; ok {
			mask = ms.Mask
		}
		fstage, ok := stageByTag[tag]
		if !ok {
			continue
		}
		prog.FeatureBinds = append(prog.FeatureBinds, featureBind{
			Tag:          tag,
			FeatureIndex: uint16(i),
			Stage:        fstage,
			Mask:         mask,
			Required:     required[tag],
		})
		flags := lookupFlagsForFeature(table, tag)
		for j := 0; j < feat.LookupCount(); j++ {
			inx := feat.LookupIndex(j)
			if inx < 0 {
				continue
			}
			uinx := uint16(inx)
			bucket, ok := stageBuckets[fstage]
			if !ok {
				bucket = make(stageLookupBucket)
				stageBuckets[fstage] = bucket
			}
			if op, exists := bucket[uinx]; exists {
				op.Mask |= mask
				op.Flags |= flags
				bucket[uinx] = op
				continue
			}
			bucket[uinx] = lookupOp{
				LookupIndex: uint16(inx),
				FeatureTag:  tag,
				Mask:        mask,
				Flags:       flags,
			}
		}
	}
	if len(stageBuckets) == 0 {
		return prog, notes, nil
	}
	stageKeys := make([]int, 0, len(stageBuckets))
	for st := range stageBuckets {
		stageKeys = append(stageKeys, st)
	}
	sort.Ints(stageKeys)
	for _, st := range stageKeys {
		bucket := stageBuckets[st]
		if len(bucket) == 0 {
			continue
		}
		start := len(prog.Lookups)
		indices := make([]uint16, 0, len(bucket))
		for inx := range bucket {
			indices = append(indices, inx)
		}
		sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
		for _, inx := range indices {
			prog.Lookups = append(prog.Lookups, bucket[inx])
		}
		end := len(prog.Lookups)
		prog.Stages = append(prog.Stages, stage{
			FirstLookup: start,
			LastLookup:  end,
			Pause:       noPauseHook,
		})
	}
	return prog, notes, nil
}

// compile builds a structural plan from request inputs.
//
// It compiles script/language feature programs for GSUB/GPOS into staged
// lookup schedules and prepares mask layout and policy for execution.
func compile(req planRequest) (*plan, error) {
	if req.Font == nil {
		return nil, errShaper("plan compile needs a font")
	}
	//
	scriptTag := req.ScriptTag
	if scriptTag == 0 {
		scriptTag = ScriptTagForScript(req.Props.Script)
	}
	langTag := req.LangTag
	if langTag == 0 {
		langTag = LanguageTagForLanguage(req.Props.Language, language.Low)
	}
	masks, err := compileUserFeatureMasks(req.UserFeatures)
	if err != nil {
		return nil, err
	}
	policy := req.Policy
	if policy == (planPolicy{}) {
		policy.ApplyGPOS = true
	}
	hooks := req.Hooks.clone()
	if len(hooks.pause) == 0 {
		hooks = newPlanHookSet()
	}

	toggles := collectUserFeatureToggles(req.UserFeatures)
	var (
		gsubFeats []otlayout.Feature
		gposFeats []otlayout.Feature
		notes     []planNote
	)
	gsubFeats, err = fontFeaturesForTable(req.Font, planGSUB, scriptTag, langTag)
	if err != nil {
		if policy.Strict {
			return nil, errShaper(err.Error())
		}
		notes = append(notes, planNote{
			Level:   planNoteWarning,
			Message: fmt.Sprintf("GSUB feature extraction failed: %s", err),
		})
	}
	gposFeats, err = fontFeaturesForTable(req.Font, planGPOS, scriptTag, langTag)
	if err != nil {
		if policy.Strict && policy.ApplyGPOS {
			return nil, errShaper(err.Error())
		}
		notes = append(notes, planNote{
			Level:   planNoteWarning,
			Message: fmt.Sprintf("GPOS feature extraction failed: %s", err),
		})
	}

	gsubProg, gsubNotes, err := compileTableProgram(gsubFeats, planGSUB, defaultGSUBFeatures, toggles, masks, policy)
	if err != nil {
		return nil, err
	}
	notes = append(notes, gsubNotes...)
	gposProg, gposNotes, err := compileTableProgram(gposFeats, planGPOS, defaultGPOSFeatures, toggles, masks, policy)
	if err != nil {
		return nil, err
	}
	notes = append(notes, gposNotes...)

	p := &plan{
		font:             req.Font,
		Props:            req.Props,
		ScriptTag:        scriptTag,
		LangTag:          langTag,
		VarIndex:         req.VarIndex,
		Masks:            masks,
		GSUB:             gsubProg,
		GPOS:             gposProg,
		Policy:           policy,
		Hooks:            hooks,
		Notes:            notes,
		featureRanges:    append([]FeatureRange(nil), req.UserFeatures...),
		joinerGlyphClass: compileJoinerGlyphClass(req.Font),
	}
	err = p.validate()
	assert(err == nil, "newly created plan does not validate")
	return p, nil
}

func compileJoinerGlyphClass(font *ot.Font) map[ot.GlyphIndex]uint8 {
	classes := make(map[ot.GlyphIndex]uint8, 2)
	if font == nil || font.CMap == nil {
		return classes
	}
	if gid := otquery.GlyphIndex(font, '\u200C'); gid != NOTDEF {
		classes[gid] |= joinerClassZWNJ
	}
	if gid := otquery.GlyphIndex(font, '\u200D'); gid != NOTDEF {
		classes[gid] |= joinerClassZWJ
	}
	return classes
}

// --- Executing Plans --------------------------------------------------

type planExecutor struct {
	run *runBuffer
}

func (e *planExecutor) acquireBuffer(run *runBuffer) {
	assert(e != nil, "executor is nil")
	assert(run != nil, "run buffer is nil")
	if run.owner == e {
		return
	}
	assert(run.owner == nil, "run buffer already owned")
	e.run = run
	run.owner = e
}

func (e *planExecutor) releaseBuffer() {
	assert(e != nil, "executor is nil")
	assert(e.run != nil, "run buffer is nil")
	assert(e.run.owner != nil, "run buffer not owned")
	assert(e.run.owner == e, "run buffer not owned")
	e.run.owner = nil
}

func (e *planExecutor) owns() bool {
	assert(e != nil, "executor is nil")
	return e.run != nil && e.run.owner == e
}

func (e *planExecutor) apply(pl *plan) error {
	assert(e.owns(), "plan executor does not own run buffer")
	e.ensureRunMasks(pl)
	if err := e.applyGSUB(pl); err != nil {
		return err
	}
	if pl != nil && pl.Policy.ApplyGPOS {
		return e.applyGPOS(pl)
	}
	return nil
}

func (e *planExecutor) applyGSUB(pl *plan) error {
	e.prepareGSUBAnnotations(pl)
	return e.applyTable(pl, planGSUB)
}

func (e *planExecutor) applyGPOS(pl *plan) error {
	return e.applyTable(pl, planGPOS)
}

func (e *planExecutor) applyTable(pl *plan, table planTable) error {
	assert(pl != nil, "plan executor received nil plan")
	assert(e.run != nil, "plan executor received nil run buffer")
	err := pl.validate()
	assert(err == nil, "plan for executor is invalid")
	//
	prog := pl.table(table)
	assert(prog != nil, "plan returns nil program, cannot happen")
	if table == planGPOS {
		e.run.EnsurePos()
	}
	for _, st := range prog.Stages {
		if st.FirstLookup < 0 || st.LastLookup < st.FirstLookup || st.LastLookup > len(prog.Lookups) {
			return errShaper("plan stage has invalid lookup bounds")
		}
		if st.LastLookup > st.FirstLookup {
			if err := e.applyLookups(pl, table, prog.Lookups[st.FirstLookup:st.LastLookup]); err != nil {
				return err
			}
		}
		if st.Pause == noPauseHook {
			continue
		}
		if fn, ok := pl.Hooks.pauseHook(st.Pause); ok {
			if err := fn(e.run); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Helpers ----------------------------------------------------------

func bitStorage32(v uint32) int {
	if v == 0 {
		return 0
	}
	return bits.Len32(v)
}
