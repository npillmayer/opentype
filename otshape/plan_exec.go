package otshape

import "github.com/npillmayer/opentype/ot"

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

// apply executes a run whose masks have already been initialized and customized
// by SetupMasks, if present. Execution must preserve those mask decisions.
func (e *planExecutor) apply(pl *plan) error {
	assert(e.owns(), "plan executor does not own run buffer")
	assert(len(e.run.Masks) == e.run.Len(), "plan executor requires prepared masks")
	e.retainStreamContext(pl)
	if err := e.applyGSUB(pl); err != nil {
		return err
	}
	appliedGPOS := false
	if pl != nil && pl.Policy.ApplyGPOS {
		if err := e.applyGPOS(pl); err != nil {
			return err
		}
		appliedGPOS = true
	}
	e.retainStreamContext(pl)
	e.applyPositionPolicies(pl, appliedGPOS)
	return e.finishMarkAttachments(pl)
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
			if e.run.limitErr != nil {
				return e.run.limitErr
			}
		}
	}
	return nil
}

// ensureRunMasks initializes every mask from the plan, replacing existing values.
// Prepare masks before SetupMasks; do not call this again during execution.
func (e *planExecutor) ensureRunMasks(pl *plan) {
	assert(e != nil, "executor is nil")
	assert(e.run != nil, "run buffer is nil")
	assert(pl != nil, "plan is nil")
	n := e.run.Len()
	if e.run.Masks == nil {
		e.run.Masks = make([]uint32, n)
	}
	if len(e.run.Masks) != n {
		e.run.Masks = resizeUint32(e.run.Masks, n)
	}
	for i := range e.run.Masks {
		e.run.Masks[i] = pl.Masks.GlobalMask
	}
	applyFeatureRangesToMasks(e.run.Masks, e.run.Clusters, pl.Masks.ByFeature, pl.featureRanges)
}

// applyFeatureRangesToMasks resolves ranges against original scalar anchors,
// before lookup edits inherit masks. Buffer slots can move during compaction,
// normalization, and reordering; range endpoints must not move with them.
func applyFeatureRangesToMasks(dst, clusters []uint32, specs map[ot.Tag]maskSpec, ranges []FeatureRange) {
	n := len(dst)
	if n == 0 || len(ranges) == 0 || len(specs) == 0 {
		return
	}
	assert(len(clusters) == n, "feature ranges require source cluster anchors")
	for _, r := range ranges {
		spec, ok := specs[r.Feature]
		if !ok {
			continue
		}
		val := uint32(0)
		if r.On {
			val = spec.encode(r.Arg)
		}
		valueBits := (val << spec.Shift) & spec.Mask
		for i, cluster := range clusters {
			// Nonpositive bounds mean the start/end of the original input.
			// Widen both operands so large indexes cannot wrap on conversion.
			if r.Start > 0 && uint64(cluster) < uint64(r.Start) ||
				r.End > 0 && uint64(cluster) >= uint64(r.End) {
				continue
			}
			dst[i] &^= spec.Mask
			dst[i] |= valueBits
		}
	}
}
