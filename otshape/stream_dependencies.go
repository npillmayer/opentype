package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

// ShapingEngineStreamingNormalization is an optional promise for normalization
// hooks: they are stateless and depend only on the pair/mark sequence supplied,
// font, selection, and (when reported) HasGposMark. Other run/plan hooks still
// require whole-input buffering. Engines without normalization hooks need not
// implement it. An incorrect promise can invalidate emitted output.
type ShapingEngineStreamingNormalization interface {
	StreamingNormalization() (local bool, dependsOnGposMark bool)
}

func localStreamingEngine(engine ShapingEngine, pl *plan, events, rootMark bool) bool {
	if _, ok := engine.(ShapingEnginePreprocessHook); ok {
		return false
	}
	if _, ok := engine.(ShapingEnginePreGSUBHook); ok {
		return false
	}
	if _, ok := engine.(ShapingEnginePostprocessHook); ok {
		return false
	}
	if _, ok := engine.(ShapingEngineMaskHook); ok {
		return false
	}
	if _, ok := engine.(ShapingEnginePlanHooks); ok {
		return false
	}
	if _, ok := engine.(ShapingEnginePostResolveHook); ok {
		return false
	}
	if _, ok := engine.(ShapingEnginePlanValidateHook); ok {
		return false
	}
	_, early := engine.(ShapingEngineNormalizationReorderHook)
	if _, ok := engine.(ShapingEngineReorderHook); ok && !early {
		return false
	}
	_, compose := engine.(ShapingEngineComposeHook)
	if early || compose {
		policy, ok := engine.(ShapingEngineStreamingNormalization)
		if !ok {
			return false
		}
		local, mark := policy.StreamingNormalization()
		if !local || (events && mark && !rootMark) {
			return false
		}
	}
	if pl != nil {
		for _, prog := range []tableProgram{pl.GSUB, pl.GPOS} {
			for _, stage := range prog.Stages {
				if stage.Pause != noPauseHook {
					return false
				}
			}
			for _, op := range prog.Lookups {
				if op.Flags.has(lookupPerSyllable) {
					return false
				}
			}
		}
	}
	return true
}

type normalizationWork struct {
	trace *otlayout.StreamTrace
	base  uint32
	err   error
}

func (w *normalizationWork) check(size int) bool {
	if w == nil {
		return true
	}
	if size > MaxIntermediateEntries {
		w.err = &LimitError{"normalized rune buffer", MaxIntermediateEntries, w.base}
		return false
	}
	return true
}
func (w *normalizationWork) compose(a, b uint32) {
	if w != nil {
		w.trace.Join(int(min(a, b)-w.base), int(max(a, b)-w.base)+1)
	}
}

// Retain possible backward dependencies at every stage snapshot, including
// lookups that a future event can enable. Glyph counts alone are insufficient:
// IgnoreMarks/filtering flags can make a short context span many raw runes.
func (e *planExecutor) retainStreamContext(pl *plan) {
	trace := e.run.stream
	if trace == nil || trace.Disabled {
		return
	}
	st := otlayout.NewBufferState(e.run.Glyphs, e.run.Pos)
	st.Stream = trace
	for _, table := range []planTable{planGSUB, planGPOS} {
		if table == planGPOS && !pl.Policy.ApplyGPOS {
			continue
		}
		layout := layoutTableFor(pl.font, table)
		if layout == nil {
			continue
		}
		graph := layout.LookupGraph()
		if graph == nil || graph.Error() != nil {
			trace.Disabled = true
			return
		}
		indices := map[int]bool{}
		for _, op := range pl.table(table).Lookups {
			indices[int(op.LookupIndex)] = true
		}
		if e.run.streamEvents {
			features, err := fontFeaturesForTable(pl.font, table, pl.ScriptTag, pl.LangTag)
			if err != nil {
				trace.Disabled = true
				return
			}
			if unstableEventLookupPolicy(features, table, hasAnyJoiner(e.run.Joiners)) {
				trace.Disabled = true
				return
			}
			for _, feat := range features {
				if feat == nil {
					continue
				}
				if lookupFlagsForFeature(table, feat.Tag()).has(lookupPerSyllable) {
					trace.Disabled = true
					return
				}
				for i := 0; i < feat.LookupCount(); i++ {
					indices[feat.LookupIndex(i)] = true
				}
			}
		}
		for index := range indices {
			st.RetainLookupContext(graph, index, pl.font.Layout.GDef)
		}
	}
}

func layoutTableFor(font *ot.Font, table planTable) *ot.LayoutTable {
	if table == planGSUB {
		if t := font.Table(ot.T("GSUB")); t != nil {
			if g := t.Self().AsGSub(); g != nil {
				return &g.LayoutTable
			}
		}
	}
	if table == planGPOS {
		if t := font.Table(ot.T("GPOS")); t != nil {
			if g := t.Self().AsGPos(); g != nil {
				return &g.LayoutTable
			}
		}
	}
	return nil
}

// Contributor selection can change a shared operation's plan-wide flags even
// where the newly selected feature is masked off. Preserve that existing policy
// by buffering the whole request when it can affect current output.
func unstableEventLookupPolicy(features []otlayout.Feature, table planTable, hasJoiners bool) bool {
	flagsByLookup := map[int]lookupRunFlags{}
	relevant := lookupRandom
	if hasJoiners {
		relevant |= lookupAutoZWJ | lookupAutoZWNJ
	}
	for _, feature := range features {
		if feature == nil {
			continue
		}
		flags := lookupFlagsForFeature(table, feature.Tag()) & relevant
		for i := 0; i < feature.LookupCount(); i++ {
			index := feature.LookupIndex(i)
			if prev, ok := flagsByLookup[index]; ok && prev != flags {
				return true
			}
			flagsByLookup[index] = flags
		}
	}
	return false
}
