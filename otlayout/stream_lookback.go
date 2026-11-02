package otlayout

import "github.com/npillmayer/opentype/ot"

// RetainLookupContext retains enough of this stage's input for a future lookup
// start. Counts refer to eligible glyphs, not a guessed number of raw runes.
// Nested lookups have their own flags and are visited independently. Types with
// unmodelled traversal disable partial flushing rather than assuming locality.
func (b *BufferState) RetainLookupContext(graph *ot.LookupListGraph, index int, gdef *ot.GDefTable) {
	if b.Stream == nil || b.Stream.Disabled {
		return
	}
	if len(b.Stream.Sources) != b.Len() {
		b.Stream.Disabled = true
		return
	}
	seen := map[int]bool{}
	var visit func(int, int)
	visit = func(index, depth int) {
		if depth >= maxLookupDepth || len(seen) >= maxLookupWork {
			b.Stream.Disabled = true
			return
		}
		if seen[index] {
			return
		}
		seen[index] = true
		lookup := graph.Lookup(index)
		if lookup == nil || lookup.Error() != nil {
			b.Stream.Disabled = true
			return
		}
		ctx := &applyCtx{clookup: lookup, flag: lookup.Flag, gdef: gdef, buf: b}
		for _, raw := range lookup.Range() {
			node, err := effectiveLookupNode(raw)
			if err != nil {
				b.Stream.Disabled = true
				continue
			}
			back := 0
			var records []ot.SequenceLookupRecord
			var target *ot.Coverage
			markPredecessor := false
			if p := node.GSubPayload(); p != nil {
				switch {
				case p.SingleFmt1 != nil, p.SingleFmt2 != nil, p.MultipleFmt1 != nil, p.AlternateFmt1 != nil, p.LigatureFmt1 != nil:
				case p.ReverseChainingFmt1 != nil:
					back = len(p.ReverseChainingFmt1.BacktrackCoverages)
					// Reverse traversal is a whole-run fallback when a current glyph can
					// participate. If none can, only future starts need backward context.
					for _, g := range b.Glyphs {
						if _, ok := node.Coverage.Match(g); ok {
							b.Stream.Disabled = true
							break
						}
					}
				case p.ContextFmt1 != nil:
					for _, set := range p.ContextFmt1.RuleSets {
						for _, r := range set {
							records = append(records, r.Records...)
						}
					}
				case p.ContextFmt2 != nil:
					for _, set := range p.ContextFmt2.RuleSets {
						for _, r := range set {
							records = append(records, r.Records...)
						}
					}
				case p.ContextFmt3 != nil:
					records = p.ContextFmt3.Records
				case p.ChainingContextFmt1 != nil:
					for _, set := range p.ChainingContextFmt1.RuleSets {
						for _, r := range set {
							back = max(back, len(r.Backtrack))
							records = append(records, r.Records...)
						}
					}
				case p.ChainingContextFmt2 != nil:
					for _, set := range p.ChainingContextFmt2.RuleSets {
						for _, r := range set {
							back = max(back, len(r.Backtrack))
							records = append(records, r.Records...)
						}
					}
				case p.ChainingContextFmt3 != nil:
					back = len(p.ChainingContextFmt3.BacktrackCoverages)
					records = p.ChainingContextFmt3.Records
				default:
					b.Stream.Disabled = true
				}
			} else if p := node.GPosPayload(); p != nil {
				switch {
				case p.SingleFmt1 != nil, p.SingleFmt2 != nil, p.PairFmt1 != nil, p.PairFmt2 != nil:
				case p.CursiveFmt1 != nil:
					back = 1 // The current executor reads the nearest eligible predecessor.
				case p.MarkToBaseFmt1 != nil:
					target = &p.MarkToBaseFmt1.BaseCoverage
				case p.MarkToLigatureFmt1 != nil:
					target = &p.MarkToLigatureFmt1.LigatureCoverage
				case p.MarkToMarkFmt1 != nil:
					markPredecessor = true
					back = 1
				case p.ContextFmt1 != nil:
					for _, set := range p.ContextFmt1.RuleSets {
						for _, r := range set {
							records = append(records, r.Records...)
						}
					}
				case p.ContextFmt2 != nil:
					for _, set := range p.ContextFmt2.RuleSets {
						for _, r := range set {
							records = append(records, r.Records...)
						}
					}
				case p.ContextFmt3 != nil:
					records = p.ContextFmt3.Records
				case p.ChainingContextFmt1 != nil:
					for _, set := range p.ChainingContextFmt1.RuleSets {
						for _, r := range set {
							back = max(back, len(r.Backtrack))
							records = append(records, r.Records...)
						}
					}
				case p.ChainingContextFmt2 != nil:
					for _, set := range p.ChainingContextFmt2.RuleSets {
						for _, r := range set {
							back = max(back, len(r.Backtrack))
							records = append(records, r.Records...)
						}
					}
				case p.ChainingContextFmt3 != nil:
					back = len(p.ChainingContextFmt3.BacktrackCoverages)
					records = p.ChainingContextFmt3.Records
				default:
					b.Stream.Disabled = true
				}
			} else {
				b.Stream.Disabled = true
			}
			if markPredecessor {
				ctx.flag &^= ot.LOOKUP_FLAG_IGNORE_BASE_GLYPHS | ot.LOOKUP_FLAG_IGNORE_LIGATURES | ot.LOOKUP_FLAG_IGNORE_MARKS
			}
			for i := b.Len() - 1; i >= 0 && (back > 0 || target != nil); i-- {
				if skipGlyph(ctx, b.Glyphs[i]) {
					continue
				}
				if target != nil {
					if _, ok := target.Match(b.Glyphs[i]); !ok {
						continue
					}
					b.Stream.KeepFrom = min(b.Stream.KeepFrom, b.Stream.Sources[i].Start)
					break
				}
				b.Stream.KeepFrom = min(b.Stream.KeepFrom, b.Stream.Sources[i].Start)
				back--
			}
			for _, r := range records {
				visit(int(r.LookupListIndex), depth+1)
			}
			ctx.flag = lookup.Flag
		}
	}
	visit(index, 0)
}
