package otshape

import (
	"fmt"
	"math/bits"
	"sort"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
	"github.com/npillmayer/opentype/otquery"
	"golang.org/x/text/language"
)

type planRequest struct {
	Font         *ot.Font
	Props        segmentProps
	ScriptTag    ot.Tag
	LangTag      ot.Tag
	Selection    SelectionContext
	Engine       ShapingEngine
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
	on          bool // global on/off if hasGlobal=true
	arg         int  // global arg if hasGlobal=true
	hasGlobal   bool
	hasRange    bool
	hasRangeOn  bool
	hasRangeOff bool
	hasAnyOn    bool
	mentioned   bool
}

func collectUserFeatureToggles(features []FeatureRange) map[ot.Tag]userFeatureToggle {
	toggles := make(map[ot.Tag]userFeatureToggle)
	for _, f := range features {
		if f.Feature == 0 {
			continue
		}
		t := toggles[f.Feature]
		isGlobal := f.Start == 0 && f.End == 0
		t.mentioned = true
		if isGlobal {
			t.hasGlobal = true
			t.on = f.On
			t.arg = f.Arg
		} else {
			t.hasRange = true
			if f.On {
				t.hasRangeOn = true
			} else {
				t.hasRangeOff = true
			}
		}
		if f.On {
			t.hasAnyOn = true
		}
		toggles[f.Feature] = t
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
	layout := maskLayout{ByFeature: make(map[ot.Tag]maskSpec)}
	type state struct {
		values                                       []int
		hasGlobal, globalOn, hasRangeOn, hasRangeOff bool
		globalArg                                    int
	}
	states := make(map[ot.Tag]*state)
	var order []ot.Tag
	for _, f := range features {
		if f.Feature == 0 {
			continue
		}
		st := states[f.Feature]
		if st == nil {
			st = &state{values: []int{0, 1}}
			states[f.Feature] = st
			order = append(order, f.Feature)
		}
		if f.On {
			v := featureValue(f.Arg)
			found := false
			for _, existing := range st.values {
				if existing == v {
					found = true
					break
				}
			}
			if !found {
				st.values = append(st.values, v)
			}
		}
		if f.Start == 0 && f.End == 0 {
			st.hasGlobal, st.globalOn, st.globalArg = true, f.On, f.Arg
		} else if f.On {
			st.hasRangeOn = true
		} else {
			st.hasRangeOff = true
		}
	}
	var nextBit uint8
	for _, tag := range order {
		st := states[tag]
		bitsNeeded := bitStorage32(uint32(len(st.values) - 1))
		if int(nextBit)+bitsNeeded > 31 {
			return maskLayout{}, errShaper("mask bit budget exhausted")
		}
		spec := maskSpec{Mask: uint32((1<<bitsNeeded)-1) << nextBit, Shift: nextBit, Values: st.values}
		if st.hasGlobal && st.globalOn {
			spec.DefaultValue = spec.encode(st.globalArg)
		} else if !st.hasGlobal && st.hasRangeOff && !st.hasRangeOn {
			spec.DefaultValue = 1
		}
		layout.GlobalMask |= spec.DefaultValue << nextBit
		layout.ByFeature[tag] = spec
		nextBit += uint8(bitsNeeded)
	}
	return layout, nil
}

func compileTableProgram(
	features []otlayout.Feature,
	table planTable,
	defaultTags []ot.Tag,
	toggles map[ot.Tag]userFeatureToggle,
	featureFlags map[ot.Tag]FeatureFlags,
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
			if t.hasAnyOn && policy.Strict {
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
		if required[tag] && t.hasGlobal && !t.on {
			notes = append(notes, planNote{
				Level:   planNoteWarning,
				Message: fmt.Sprintf("required feature %s in %s cannot be disabled", tag, table),
			})
			continue
		}
		if t.hasGlobal {
			active[tag] = t.on
		}
		if t.hasRangeOn {
			active[tag] = true
		}
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
		flags = applyFeatureFlags(flags, featureFlags[tag], table)
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
	policy := req.Policy
	if policy == (planPolicy{}) {
		policy.ApplyGPOS = true
	}
	hooks := req.Hooks.clone()
	if len(hooks.pause) == 0 {
		hooks = newPlanHookSet()
	}
	selection := req.Selection
	if selection.ScriptTag == 0 {
		selection = SelectionContext{
			Direction: req.Props.Direction,
			Script:    req.Props.Script,
			Language:  req.Props.Language,
			ScriptTag: scriptTag,
			LangTag:   langTag,
		}
	}
	planner := newPlanFeaturePlanner(req.Font, selection, &hooks, req.UserFeatures)
	if engineHooks, ok := req.Engine.(ShapingEnginePlanHooks); ok {
		engineHooks.CollectFeatures(planner, selection)
		engineHooks.OverrideFeatures(planner)
	}

	maskFeatures := append([]FeatureRange(nil), req.UserFeatures...)
	maskFeatures = append(maskFeatures, planner.maskFeatures()...)
	masks, err := compileUserFeatureMasks(maskFeatures)
	if err != nil {
		return nil, err
	}

	toggles := planner.toggles()
	gsubFeatureFlags := planner.featureFlags(planGSUB)
	gposFeatureFlags := planner.featureFlags(planGPOS)
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

	gsubProg, gsubNotes, err := compileTableProgram(
		gsubFeats,
		planGSUB,
		planner.defaultTags(planGSUB),
		toggles,
		gsubFeatureFlags,
		masks,
		policy,
	)
	if err != nil {
		return nil, err
	}
	notes = append(notes, gsubNotes...)
	gposProg, gposNotes, err := compileTableProgram(
		gposFeats,
		planGPOS,
		planner.defaultTags(planGPOS),
		toggles,
		gposFeatureFlags,
		masks,
		policy,
	)
	if err != nil {
		return nil, err
	}
	notes = append(notes, gposNotes...)
	planner.applyDirectGSUBPauses(&gsubProg)

	gsubFallbackNeeds := collectFallbackNeeds(gsubProg, planner.defaultTags(planGSUB), toggles, gsubFeatureFlags)
	gposFallbackNeeds := collectFallbackNeeds(gposProg, planner.defaultTags(planGPOS), toggles, gposFeatureFlags)
	planFallbackNeeds := mergeFallbackNeeds(gsubProg, gposProg, gsubFallbackNeeds, gposFallbackNeeds)
	resolvedView := newResolvedFeatureView(gsubProg, gposProg, gsubFallbackNeeds, gposFallbackNeeds)
	if postResolve, ok := req.Engine.(ShapingEnginePostResolveHook); ok {
		postPlanner := newResolvedPausePlanner(req.Font, &hooks, &gsubProg)
		postResolve.PostResolveFeatures(postPlanner, resolvedView, selection)
	}

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
		featureRanges:    maskFeatures,
		joinerGlyphClass: compileJoinerGlyphClass(req.Font),
	}
	if planHooks, ok := req.Engine.(ShapingEnginePlanHooks); ok {
		pc := newPlanContext(req.Font, selection)
		for tag, ms := range p.Masks.ByFeature {
			pc.mask1[tag] = ms.Mask & -ms.Mask
		}
		for tag, needed := range planFallbackNeeds {
			if needed {
				pc.fallback[tag] = true
			}
		}
		planHooks.InitPlan(pc)
		if planValidate, ok := req.Engine.(ShapingEnginePlanValidateHook); ok {
			if err := planValidate.ValidatePlan(pc); err != nil {
				return nil, err
			}
		}
	}
	err = p.validate()
	assert(err == nil, "newly created plan does not validate")
	return p, nil
}

// Malformed joiner mappings are treated as unavailable by this optional classification.
func compileJoinerGlyphClass(font *ot.Font) map[ot.GlyphIndex]uint8 {
	classes := make(map[ot.GlyphIndex]uint8, 2)
	if font == nil || font.CMap == nil {
		return classes
	}
	if gid, err := otquery.GlyphIndex(font, '\u200C'); err == nil && gid != NOTDEF {
		classes[gid] |= joinerClassZWNJ
	}
	if gid, err := otquery.GlyphIndex(font, '\u200D'); err == nil && gid != NOTDEF {
		classes[gid] |= joinerClassZWJ
	}
	return classes
}

func bitStorage32(v uint32) int {
	if v == 0 {
		return 0
	}
	return bits.Len32(v)
}
