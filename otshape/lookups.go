package otshape

import (
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otlayout"
)

type planLookupFeature struct {
	tag       ot.Tag
	typ       otlayout.LayoutTagType
	lookupInx int
}

func (f planLookupFeature) Tag() ot.Tag {
	return f.tag
}

func (f planLookupFeature) Type() otlayout.LayoutTagType {
	return f.typ
}

func (f planLookupFeature) LookupCount() int {
	return 1
}

func (f planLookupFeature) LookupIndex(i int) int {
	if i != 0 {
		return -1
	}
	return f.lookupInx
}

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
	applyFeatureRangesToMasks(e.run.Masks, pl.Masks.ByFeature, pl.featureRanges)
}

func (e *planExecutor) realignSideArrays(pl *plan, st *otlayout.BufferState) {
	assert(e != nil, "executor is nil")
	assert(e.run != nil, "run buffer is nil")
	assert(pl != nil, "plan is nil")
	assert(st != nil, "buffer state is nil")
	e.run.Glyphs = st.Glyphs
	e.run.Pos = st.Pos
	if e.run.Clusters != nil && len(e.run.Clusters) != e.run.Len() {
		e.run.Clusters = resizeUint32(e.run.Clusters, e.run.Len())
	}
	if e.run.UnsafeFlags != nil && len(e.run.UnsafeFlags) != e.run.Len() {
		e.run.UnsafeFlags = resizeUint16(e.run.UnsafeFlags, e.run.Len())
	}
	if e.run.Syllables != nil && len(e.run.Syllables) != e.run.Len() {
		e.run.Syllables = resizeUint16(e.run.Syllables, e.run.Len())
	}
	if e.run.Joiners != nil && len(e.run.Joiners) != e.run.Len() {
		e.run.Joiners = resizeUint8(e.run.Joiners, e.run.Len())
	}
	e.ensureRunMasks(pl)
}

func (e *planExecutor) applyLookups(pl *plan, table planTable, lookups []lookupOp) error {
	assert(e.owns(), "plan executor does not own run buffer")
	assert(pl != nil, "plan is nil")
	assert(e.run != nil, "run buffer is nil")
	assert(pl.font != nil, "plan has no font")
	if len(lookups) == 0 {
		return nil
	}

	fType := otlayout.GSubFeatureType
	if table == planGPOS {
		fType = otlayout.GPosFeatureType
	}

	st := otlayout.NewBufferState(e.run.Glyphs, e.run.Pos)
	for _, op := range lookups {
		alt := 0
		if op.Flags.has(lookupRandom) {
			alt = -1
		}
		feat := planLookupFeature{
			tag:       op.FeatureTag,
			typ:       fType,
			lookupInx: int(op.LookupIndex),
		}
		if op.Flags.has(lookupPerSyllable) && table == planGSUB {
			if err := e.applyLookupPerSyllable(pl, op, feat, st, alt); err != nil {
				return err
			}
			continue
		}
		if _, err := e.applyLookupSpan(pl, op, feat, st, alt, 0, st.Len(), 0); err != nil {
			return err
		}
	}

	e.run.Glyphs = st.Glyphs
	e.run.Pos = st.Pos
	return nil
}

func (e *planExecutor) applyLookupPerSyllable(
	pl *plan,
	op lookupOp,
	feat planLookupFeature,
	st *otlayout.BufferState,
	alt int,
) error {
	for start := 0; start < st.Len(); {
		end := e.lookupSpanEnd(start, st.Len())
		if end <= start {
			end = start + 1
		}
		nextStart, err := e.applyLookupIsolatedSpan(pl, op, feat, st, alt, start, end)
		if err != nil {
			return err
		}
		start = nextStart
	}
	return nil
}

func (e *planExecutor) applyLookupIsolatedSpan(
	pl *plan,
	op lookupOp,
	feat planLookupFeature,
	st *otlayout.BufferState,
	alt int,
	start int,
	end int,
) (int, error) {
	if start < 0 {
		start = 0
	}
	if end > st.Len() {
		end = st.Len()
	}
	if end <= start {
		return end, nil
	}
	subGlyphs := append(otlayout.GlyphBuffer(nil), st.Glyphs[start:end]...)
	var subPos otlayout.PosBuffer
	if st.Pos != nil {
		subPos = append(otlayout.PosBuffer(nil), st.Pos[start:end]...)
	}
	sub := otlayout.NewBufferState(subGlyphs, subPos)
	if _, err := e.applyLookupSpan(pl, op, feat, sub, alt, 0, sub.Len(), start); err != nil {
		return start, err
	}
	newLen := sub.Len()
	st.Glyphs = st.Glyphs.Replace(start, end, append([]ot.GlyphIndex(nil), sub.Glyphs...))
	if st.Pos != nil {
		edit := &otlayout.EditSpan{From: start, To: end, Len: newLen}
		st.Pos = st.Pos.ApplyEdit(edit)
		if sub.Pos != nil && len(sub.Pos) == newLen {
			copy(st.Pos[start:start+newLen], sub.Pos)
		}
	}
	if newLen != end-start {
		e.realignSideArrays(pl, st)
	}
	return start + newLen, nil
}

func (e *planExecutor) applyLookupSpan(
	pl *plan,
	op lookupOp,
	feat planLookupFeature,
	st *otlayout.BufferState,
	alt int,
	start int,
	end int,
	indexBase int,
) (int, error) {
	if start < 0 {
		start = 0
	}
	if end > st.Len() {
		end = st.Len()
	}
	if end <= start {
		return end, nil
	}
	st.Index = start
	for st.Index < end && st.Index < st.Len() {
		if !e.lookupIndexEnabled(pl, op, st, st.Index, indexBase) {
			st.Index++
			continue
		}
		prevIndex := st.Index
		prevLen := st.Len()
		_, applied := otlayout.ApplyFeature(pl.font, feat, st, alt)
		if !applied && st.Index == prevIndex {
			st.Index++
			continue
		}
		if st.Len() == prevLen && st.Index == prevIndex {
			st.Index++
		}
		if st.Len() != prevLen {
			delta := st.Len() - prevLen
			end += delta
			if end < st.Index {
				end = st.Index
			}
			e.realignSideArrays(pl, st)
			if end > st.Len() {
				end = st.Len()
			}
		}
	}
	return end, nil
}

func (e *planExecutor) lookupIndexEnabled(pl *plan, op lookupOp, st *otlayout.BufferState, inx int, indexBase int) bool {
	if inx < 0 || inx >= st.Len() {
		return false
	}
	absInx := indexBase + inx
	if op.Mask != 0 {
		if absInx >= len(e.run.Masks) {
			e.realignSideArrays(pl, st)
		}
		if absInx >= len(e.run.Masks) || e.run.Masks[absInx]&op.Mask == 0 {
			return false
		}
	}
	if e.lookupShouldSkipJoiner(op, absInx) {
		return false
	}
	return true
}

func (e *planExecutor) lookupShouldSkipJoiner(op lookupOp, inx int) bool {
	if e == nil || e.run == nil || len(e.run.Joiners) == 0 || inx < 0 || inx >= len(e.run.Joiners) {
		return false
	}
	j := e.run.Joiners[inx]
	if op.Flags.has(lookupAutoZWNJ) && (j&joinerClassZWNJ) != 0 {
		return true
	}
	if op.Flags.has(lookupAutoZWJ) && (j&joinerClassZWJ) != 0 {
		return true
	}
	return false
}

func (e *planExecutor) lookupSpanEnd(start, n int) int {
	if start < 0 {
		start = 0
	}
	if start >= n {
		return n
	}
	if len(e.run.Syllables) == n {
		syl := e.run.Syllables[start]
		i := start + 1
		for i < n && e.run.Syllables[i] == syl {
			i++
		}
		return i
	}
	if len(e.run.Clusters) == n {
		cl := e.run.Clusters[start]
		i := start + 1
		for i < n && e.run.Clusters[i] == cl {
			i++
		}
		return i
	}
	return n
}

func applyFeatureRangesToMasks(dst []uint32, specs map[ot.Tag]maskSpec, ranges []FeatureRange) {
	n := len(dst)
	if n == 0 || len(ranges) == 0 || len(specs) == 0 {
		return
	}
	for _, r := range ranges {
		spec, ok := specs[r.Feature]
		if !ok {
			continue
		}
		start, end := normalizeMaskRange(r.Start, r.End, n)
		if end <= start {
			continue
		}
		val := uint32(0)
		if r.On {
			if r.Arg > 0 {
				val = uint32(r.Arg)
			} else {
				val = 1
			}
		}
		valueBits := (val << spec.Shift) & spec.Mask
		for i := start; i < end; i++ {
			dst[i] &^= spec.Mask
			dst[i] |= valueBits
		}
	}
}

func normalizeMaskRange(start, end, n int) (int, int) {
	if start < 0 {
		start = 0
	}
	if start > n {
		start = n
	}
	if end <= 0 || end > n {
		end = n
	}
	if end < start {
		end = start
	}
	return start, end
}
