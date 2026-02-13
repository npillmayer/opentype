package otshape

import (
	"sort"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

type featureTarget uint8

const (
	targetGSUB featureTarget = 1 << iota
	targetGPOS
)

type planFeaturePlanner struct {
	font           *ot.Font
	selection      SelectionContext
	hooks          *planHookSet
	gsubDefaults   []ot.Tag
	gposDefaults   []ot.Tag
	togglesByTag   map[ot.Tag]userFeatureToggle
	flagsByTable   map[planTable]map[ot.Tag]FeatureFlags
	maskValues     map[ot.Tag]uint32
	baseMaskValues map[ot.Tag]struct{}
	gsubPauseHooks []pauseHookID
}

func newPlanFeaturePlanner(
	font *ot.Font,
	selection SelectionContext,
	hooks *planHookSet,
	userFeatures []FeatureRange,
) *planFeaturePlanner {
	baseMaskValues := make(map[ot.Tag]struct{}, len(userFeatures))
	for _, f := range userFeatures {
		if f.Feature != 0 {
			baseMaskValues[f.Feature] = struct{}{}
		}
	}
	return &planFeaturePlanner{
		font:           font,
		selection:      selection,
		hooks:          hooks,
		gsubDefaults:   append([]ot.Tag(nil), defaultGSUBFeatures...),
		gposDefaults:   append([]ot.Tag(nil), defaultGPOSFeatures...),
		togglesByTag:   collectUserFeatureToggles(userFeatures),
		flagsByTable:   map[planTable]map[ot.Tag]FeatureFlags{planGSUB: {}, planGPOS: {}},
		maskValues:     make(map[ot.Tag]uint32),
		baseMaskValues: baseMaskValues,
	}
}

func (p *planFeaturePlanner) EnableFeature(tag ot.Tag) {
	p.AddFeature(tag, FeatureNone, 1)
}

func (p *planFeaturePlanner) AddFeature(tag ot.Tag, flags FeatureFlags, value uint32) {
	if p == nil || tag == 0 {
		return
	}
	targets := inferFeatureTargets(tag)
	if targets&targetGSUB != 0 {
		p.gsubDefaults = appendUniqueTag(p.gsubDefaults, tag)
		p.flagsByTable[planGSUB][tag] |= flags
	}
	if targets&targetGPOS != 0 {
		p.gposDefaults = appendUniqueTag(p.gposDefaults, tag)
		p.flagsByTable[planGPOS][tag] |= flags
	}
	arg := 1
	if value > 0 {
		arg = int(value)
		p.maskValues[tag] = value
	}
	p.togglesByTag[tag] = userFeatureToggle{
		on:        true,
		arg:       arg,
		hasGlobal: true,
		hasAnyOn:  true,
		mentioned: true,
	}
}

func (p *planFeaturePlanner) DisableFeature(tag ot.Tag) {
	if p == nil || tag == 0 {
		return
	}
	p.togglesByTag[tag] = userFeatureToggle{
		on:        false,
		arg:       0,
		hasGlobal: true,
		mentioned: true,
	}
}

func (p *planFeaturePlanner) AddGSUBPause(fn PauseHook) {
	if p == nil || fn == nil || p.hooks == nil {
		return
	}
	id := p.hooks.addPause(wrapPauseHook(p.font, fn))
	if id != noPauseHook {
		p.gsubPauseHooks = append(p.gsubPauseHooks, id)
	}
}

func (p *planFeaturePlanner) HasFeature(tag ot.Tag) bool {
	if p == nil || tag == 0 {
		return false
	}
	if t, ok := p.togglesByTag[tag]; ok && t.on {
		return true
	}
	return tagInSlice(p.gsubDefaults, tag) || tagInSlice(p.gposDefaults, tag)
}

func (p *planFeaturePlanner) defaultTags(table planTable) []ot.Tag {
	if p == nil {
		return nil
	}
	if table == planGSUB {
		return append([]ot.Tag(nil), p.gsubDefaults...)
	}
	return append([]ot.Tag(nil), p.gposDefaults...)
}

func (p *planFeaturePlanner) toggles() map[ot.Tag]userFeatureToggle {
	if p == nil {
		return map[ot.Tag]userFeatureToggle{}
	}
	out := make(map[ot.Tag]userFeatureToggle, len(p.togglesByTag))
	for tag, t := range p.togglesByTag {
		out[tag] = t
	}
	return out
}

func (p *planFeaturePlanner) featureFlags(table planTable) map[ot.Tag]FeatureFlags {
	if p == nil || p.flagsByTable == nil {
		return map[ot.Tag]FeatureFlags{}
	}
	src := p.flagsByTable[table]
	out := make(map[ot.Tag]FeatureFlags, len(src))
	for tag, flags := range src {
		out[tag] = flags
	}
	return out
}

func (p *planFeaturePlanner) maskFeatures() []FeatureRange {
	if p == nil || len(p.maskValues) == 0 {
		return nil
	}
	features := make([]FeatureRange, 0, len(p.maskValues))
	for tag, v := range p.maskValues {
		if _, exists := p.baseMaskValues[tag]; exists {
			continue
		}
		toggle, hasToggle := p.togglesByTag[tag]
		if hasToggle && !toggle.on {
			continue
		}
		features = append(features, FeatureRange{
			Feature: tag,
			On:      true,
			Arg:     int(v),
		})
	}
	return features
}

func (p *planFeaturePlanner) applyDirectGSUBPauses(prog *tableProgram) {
	if p == nil || prog == nil || len(p.gsubPauseHooks) == 0 {
		return
	}
	pos := len(prog.Lookups)
	for _, id := range p.gsubPauseHooks {
		if id == noPauseHook {
			continue
		}
		prog.Stages = append(prog.Stages, stage{
			FirstLookup: pos,
			LastLookup:  pos,
			Pause:       id,
		})
	}
}

type resolvedFeatureView struct {
	features map[LayoutTable][]ResolvedFeature
	byTag    map[LayoutTable]map[ot.Tag]ResolvedFeature
}

func newResolvedFeatureView(
	gsub tableProgram,
	gpos tableProgram,
	gsubFlags map[ot.Tag]FeatureFlags,
	gposFlags map[ot.Tag]FeatureFlags,
) *resolvedFeatureView {
	v := &resolvedFeatureView{
		features: map[LayoutTable][]ResolvedFeature{},
		byTag: map[LayoutTable]map[ot.Tag]ResolvedFeature{
			LayoutGSUB: {},
			LayoutGPOS: {},
		},
	}
	v.features[LayoutGSUB] = collectResolvedFeatures(gsub, planGSUB, gsubFlags)
	v.features[LayoutGPOS] = collectResolvedFeatures(gpos, planGPOS, gposFlags)
	for _, table := range []LayoutTable{LayoutGSUB, LayoutGPOS} {
		for _, rf := range v.features[table] {
			v.byTag[table][rf.Tag] = rf
		}
	}
	return v
}

func (v *resolvedFeatureView) SelectedFeatures(table LayoutTable) []ResolvedFeature {
	if v == nil {
		return nil
	}
	list := v.features[table]
	out := make([]ResolvedFeature, len(list))
	copy(out, list)
	return out
}

func (v *resolvedFeatureView) HasSelectedFeature(table LayoutTable, tag ot.Tag) bool {
	if v == nil {
		return false
	}
	_, ok := v.byTag[table][tag]
	return ok
}

type resolvedPausePlanner struct {
	font       *ot.Font
	hooks      *planHookSet
	gsubProg   *tableProgram
	stageByTag map[ot.Tag]int
}

func newResolvedPausePlanner(font *ot.Font, hooks *planHookSet, gsubProg *tableProgram) *resolvedPausePlanner {
	stageByTag := map[ot.Tag]int{}
	if gsubProg != nil {
		stageByTag = stageIndexByTag(*gsubProg)
	}
	return &resolvedPausePlanner{
		font:       font,
		hooks:      hooks,
		gsubProg:   gsubProg,
		stageByTag: stageByTag,
	}
}

func (p *resolvedPausePlanner) AddGSUBPauseBefore(tag ot.Tag, fn PauseHook) bool {
	if p == nil || p.gsubProg == nil || p.hooks == nil || fn == nil {
		return false
	}
	stageInx, ok := p.stageByTag[tag]
	if !ok {
		return false
	}
	id := p.hooks.addPause(wrapPauseHook(p.font, fn))
	if id == noPauseHook {
		return false
	}
	if stageInx <= 0 {
		p.prependStagePause(id)
		return true
	}
	return attachPauseToStage(p.gsubProg, p.hooks, stageInx-1, id)
}

func (p *resolvedPausePlanner) AddGSUBPauseAfter(tag ot.Tag, fn PauseHook) bool {
	if p == nil || p.gsubProg == nil || p.hooks == nil || fn == nil {
		return false
	}
	stageInx, ok := p.stageByTag[tag]
	if !ok {
		return false
	}
	id := p.hooks.addPause(wrapPauseHook(p.font, fn))
	if id == noPauseHook {
		return false
	}
	return attachPauseToStage(p.gsubProg, p.hooks, stageInx, id)
}

func (p *resolvedPausePlanner) prependStagePause(id pauseHookID) {
	if p == nil || p.gsubProg == nil || id == noPauseHook {
		return
	}
	firstLookup := 0
	if len(p.gsubProg.Stages) > 0 {
		firstLookup = p.gsubProg.Stages[0].FirstLookup
	}
	newStage := stage{
		FirstLookup: firstLookup,
		LastLookup:  firstLookup,
		Pause:       id,
	}
	p.gsubProg.Stages = append([]stage{newStage}, p.gsubProg.Stages...)
	for tag, inx := range p.stageByTag {
		p.stageByTag[tag] = inx + 1
	}
}

func attachPauseToStage(prog *tableProgram, hooks *planHookSet, stageInx int, id pauseHookID) bool {
	if prog == nil || hooks == nil || id == noPauseHook || stageInx < 0 || stageInx >= len(prog.Stages) {
		return false
	}
	st := &prog.Stages[stageInx]
	if st.Pause == noPauseHook {
		st.Pause = id
		return true
	}
	oldFn, oldOK := hooks.pauseHook(st.Pause)
	newFn, newOK := hooks.pauseHook(id)
	if !oldOK || !newOK {
		return false
	}
	st.Pause = hooks.addPause(func(run *runBuffer) error {
		if oldFn != nil {
			if err := oldFn(run); err != nil {
				return err
			}
		}
		if newFn != nil {
			return newFn(run)
		}
		return nil
	})
	return true
}

func collectResolvedFeatures(
	prog tableProgram,
	table planTable,
	featureFlags map[ot.Tag]FeatureFlags,
) []ResolvedFeature {
	stageByTag := stageIndexByTag(prog)
	byTag := map[ot.Tag]ResolvedFeature{}
	for _, op := range prog.Lookups {
		rf := byTag[op.FeatureTag]
		rf.Tag = op.FeatureTag
		rf.Stage = stageByTag[op.FeatureTag]
		rf.AutoZWNJ = rf.AutoZWNJ || op.Flags.has(lookupAutoZWNJ)
		rf.AutoZWJ = rf.AutoZWJ || op.Flags.has(lookupAutoZWJ)
		rf.PerSyllable = rf.PerSyllable || op.Flags.has(lookupPerSyllable)
		rf.SupportsRandom = rf.SupportsRandom || op.Flags.has(lookupRandom)
		if featureFlags[op.FeatureTag]&FeatureHasFallback != 0 {
			rf.NeedsFallback = true
		}
		byTag[op.FeatureTag] = rf
	}
	out := make([]ResolvedFeature, 0, len(byTag))
	for _, rf := range byTag {
		out = append(out, rf)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Stage == out[j].Stage {
			return out[i].Tag < out[j].Tag
		}
		return out[i].Stage < out[j].Stage
	})
	return out
}

func stageIndexByTag(prog tableProgram) map[ot.Tag]int {
	stageByTag := make(map[ot.Tag]int)
	for i, st := range prog.Stages {
		if st.FirstLookup < 0 || st.LastLookup > len(prog.Lookups) || st.LastLookup < st.FirstLookup {
			continue
		}
		for _, op := range prog.Lookups[st.FirstLookup:st.LastLookup] {
			if _, exists := stageByTag[op.FeatureTag]; !exists {
				stageByTag[op.FeatureTag] = i
			}
		}
	}
	return stageByTag
}

func applyFeatureFlags(base lookupRunFlags, flags FeatureFlags, table planTable) lookupRunFlags {
	if flags&FeatureManualZWNJ != 0 {
		base &^= lookupAutoZWNJ
	}
	if flags&FeatureManualZWJ != 0 {
		base &^= lookupAutoZWJ
	}
	if flags&FeatureRandom != 0 {
		base |= lookupRandom
	}
	if table == planGSUB && flags&FeaturePerSyllable != 0 {
		base |= lookupPerSyllable
	}
	return base
}

func inferFeatureTargets(tag ot.Tag) featureTarget {
	if typ, ok := otlayout.RegisteredFeatureTags[tag]; ok {
		switch typ {
		case otlayout.GSubFeatureType:
			return targetGSUB
		case otlayout.GPosFeatureType:
			return targetGPOS
		}
	}
	return targetGSUB | targetGPOS
}

func appendUniqueTag(tags []ot.Tag, tag ot.Tag) []ot.Tag {
	if tagInSlice(tags, tag) {
		return tags
	}
	return append(tags, tag)
}

func tagInSlice(tags []ot.Tag, tag ot.Tag) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func wrapPauseHook(font *ot.Font, fn PauseHook) pauseHook {
	if fn == nil {
		return nil
	}
	return func(run *runBuffer) error {
		return fn(newPauseContext(font, run))
	}
}
