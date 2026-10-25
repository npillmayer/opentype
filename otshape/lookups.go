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
	if e.run.Codepoints != nil && len(e.run.Codepoints) != e.run.Len() {
		e.run.Codepoints = resizeRunes(e.run.Codepoints, e.run.Len())
	}
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
	defer func() { e.run.Glyphs, e.run.Pos = st.Glyphs, st.Pos }()
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
		for i := range subPos {
			target := int(subPos[i].AttachTo)
			if target < 0 {
				continue
			}
			if target >= start && target < end {
				subPos[i].AttachTo -= int32(start)
			} else {
				// Preserve outside-span references separately from local attachment indices.
				subPos[i].AttachTo = -int32(target) - 2
			}
		}
	}
	sub := otlayout.NewBufferState(subGlyphs, subPos)
	var edits []otlayout.EditSpan
	_, err := e.applyLookupSpanInternal(pl, op, feat, sub, alt, 0, sub.Len(), start, false, &edits)
	newLen := sub.Len()
	st.Glyphs = st.Glyphs.Replace(start, end, append([]ot.GlyphIndex(nil), sub.Glyphs...))
	if st.Pos != nil || sub.Pos != nil {
		if st.Pos == nil {
			st.Pos = otlayout.NewPosBuffer(len(st.Glyphs) - newLen + end - start)
		}
		for _, edit := range edits {
			if !edit.ResetsPositions {
				continue
			}
			st.Pos = st.Pos.ApplyEdit(&edit)
		}
		if sub.Pos != nil {
			for i := range sub.Pos {
				target := sub.Pos[i].AttachTo
				if target >= 0 {
					sub.Pos[i].AttachTo += int32(start)
				} else if target < -1 {
					target = -target - 2
					for _, edit := range edits {
						if !edit.ResetsPositions {
							continue
						}
						if int(target) >= edit.From && int(target) < edit.To {
							target = -1
							sub.Pos[i].AttachKind = otlayout.AttachNone
							sub.Pos[i].AttachClass = 0
							sub.Pos[i].AnchorRef = otlayout.AnchorRef{}
							break
						}
						if int(target) >= edit.To {
							target += int32(edit.Len - (edit.To - edit.From))
						}
					}
					sub.Pos[i].AttachTo = target
				}
			}
			copy(st.Pos[start:start+newLen], sub.Pos)
		}
	}
	st.Index = start + sub.Index
	e.run.Glyphs, e.run.Pos = st.Glyphs, st.Pos
	return start + newLen, err
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
	return e.applyLookupSpanInternal(pl, op, feat, st, alt, start, end, indexBase, true, nil)
}

func (e *planExecutor) applyLookupSpanInternal(
	pl *plan,
	op lookupOp,
	feat planLookupFeature,
	st *otlayout.BufferState,
	alt int,
	start int,
	end int,
	indexBase int,
	publish bool,
	spanEdits *[]otlayout.EditSpan,
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
		_, applied, err := otlayout.ApplyFeature(pl.font, feat, st, alt)
		for _, edit := range st.AppliedEdits() {
			edit.From += indexBase
			edit.To += indexBase
			e.run.applyLayoutEdit(edit)
			if spanEdits != nil {
				*spanEdits = append(*spanEdits, edit)
			}
		}
		if publish {
			e.run.Glyphs, e.run.Pos = st.Glyphs, st.Pos
		}
		end += st.Len() - prevLen
		if end > st.Len() {
			end = st.Len()
		}
		if err != nil {
			return end, err
		}
		if !applied && st.Index == prevIndex {
			st.Index++
			continue
		}
		if st.Len() == prevLen && st.Index == prevIndex {
			st.Index++
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

// applyLayoutEdit updates metadata only. Layout already owns glyph/position edits.
// Replacement slots inherit the first input's annotations; clusters merge to the
// minimum input cluster and unsafe flags accumulate across the consumed range.
func (rb *runBuffer) applyLayoutEdit(edit otlayout.EditSpan) {
	if edit.From+1 == edit.To && edit.Len == 1 {
		return
	}
	var cluster uint32
	var unsafe uint16
	if rb.Clusters != nil && edit.From < edit.To {
		cluster = rb.Clusters[edit.From]
		for _, c := range rb.Clusters[edit.From:edit.To] {
			if c < cluster {
				cluster = c
			}
		}
	}
	if rb.UnsafeFlags != nil {
		for _, f := range rb.UnsafeFlags[edit.From:edit.To] {
			unsafe |= f
		}
	}
	rb.Codepoints = inheritLayoutEdit(rb.Codepoints, edit)
	rb.Clusters = inheritLayoutEdit(rb.Clusters, edit)
	rb.PlanIDs = inheritLayoutEdit(rb.PlanIDs, edit)
	rb.Masks = inheritLayoutEdit(rb.Masks, edit)
	rb.UnsafeFlags = inheritLayoutEdit(rb.UnsafeFlags, edit)
	rb.Syllables = inheritLayoutEdit(rb.Syllables, edit)
	rb.Joiners = inheritLayoutEdit(rb.Joiners, edit)
	if edit.From < edit.To {
		for i := edit.From; i < edit.From+edit.Len; i++ {
			if rb.Clusters != nil {
				rb.Clusters[i] = cluster
			}
			if rb.UnsafeFlags != nil {
				rb.UnsafeFlags[i] = unsafe
			}
		}
	}
}

func inheritLayoutEdit[T any](s []T, edit otlayout.EditSpan) []T {
	if s == nil {
		return nil
	}
	var seed T
	if edit.From < edit.To {
		seed = s[edit.From]
	}
	replacement := make([]T, edit.Len)
	for i := range replacement {
		replacement[i] = seed
	}
	out := append(s[:edit.From:edit.From], replacement...)
	return append(out, s[edit.To:]...)
}
