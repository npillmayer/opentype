# Code Review: ot/ and otlayout/

## Reassessment: 2026-10-25

Reviewed checkout: `4fb7aad` (2026-02-16, "Further cleanup work"). The working tree was clean before this reassessment. The February review below is historical: its locations, APIs, priorities, and helper sketches must not be used as a current implementation checklist.

### Current findings and priorities

1. **High: cmap parsing still contains a debug panic.** `ot/cmap.go:290` panics whenever a format-4 entry has both nonzero offset and delta. The lookup implementation itself handles this combination. Replace this diagnostic trap and add a regression fixture exercising that path. The separate `panic("unreachable")` at `ot/cmap.go:116` remains, but is behind format selection; it should not be presented as equally reachable from font input without demonstrating a bypass. The three old map/list subset panics have disappeared with those APIs.
2. **Medium: array access still silently substitutes record zero.** `ot/bytes.go:452` resets invalid indices to zero. It also checks backing bytes rather than the declared `a.length`, so arrays retaining trailing bytes can expose records outside their logical extent. Return an explicit empty/error result consistently and test negative indices, index equal to length, and trailing bytes. This remains relevant to cmap and internal parsing even though public navigation has changed.
3. **Medium: runtime nil handling remains inconsistent.** `otlayout/feature.go:183` reads `st.Index` when `feat == nil` before testing `st == nil`; passing both nil panics. Table extraction at `otlayout/feature.go:197` still assumes a suitable font/table, as documented. `otlayout/layout.go:139` also dereferences `table` in a helper described as safe. Decide and document the preconditions, or validate them consistently. These are caller-input/API robustness issues, distinct from font-data parser failures.
4. **Low: duplicated parsing orchestration remains.** `ot/parse_gsubgpos.go:4` and `ot/parse_gsubgpos.go:24` repeat the layout-header/list parsing pipeline. Chained glyph/class rule parsing at `ot/parse_lookup_gsub.go:652` and `ot/parse_lookup_gsub.go:685` repeats sequencing, but already delegates bounds-sensitive reads to shared helpers. Treat further extraction as maintenance work, not a demonstrated correctness failure.

### Status of the original findings

IDs include findings removed from this document since the original review, to reconcile the earlier discussion.

| ID | Original concern | Current assessment and evidence |
| --- | --- | --- |
| 1 | ApplyFeature returns original buffer | Superseded. `ApplyFeature` now mutates a `*BufferState` and returns position/success, not a slice (`otlayout/feature.go:183`). The proposed return-value fix no longer applies. |
| 2 | Missing cmap under relaxed completeness panics | Resolved with a stricter policy. `ot/parse.go:238` explicitly rejects missing cmap even for test fonts. The old suggestion to allow nil cmap is not the implemented policy; remove it from active priorities. |
| 3 | Incorrect 32-bit range stride | Removed. `glyphRangeRecords` now only has the 16-bit record path (`ot/bytes.go:186`). Do not implement the old 10-byte stride proposal. |
| 4 | Parsing/subsetting panics | Partly valid. Both cmap panics remain; old tag-map and lookup-list subset implementations are gone. Distinguish reachable debug traps from invariant checks and documented programmer-error panics. |
| 5 | LookupList.Navigate unchecked index | Resolved by replacement. `LookupListGraph.Lookup` checks nil, negative/out-of-range indices and offset bounds, and initializes entries with `sync.Once` (`ot/lookup.go:57`). |
| 6 | mapWrapper.Range yields empty entries | Removed. `mapWrapper` no longer exists; concrete script/feature graphs provide iteration. |
| 7 | array.Get masks errors | Still valid; see current priority 2. |
| 8 | GSUB/GPOS parse pipeline duplication | Still valid at new locations; low maintenance priority. |
| 9 | Duplicate subtable dispatcher skeleton | Substantially superseded. `parseConcreteLookupNodeWithDepth` shares header validation and family dispatch (`ot/parse_lookup.go:93`). Separate typed payload parsers remain appropriate; the old callback skeleton targets deleted APIs. |
| 10 | Chained rule parsing duplication | Partially valid, reduced. Typed rule parsers share list/record readers; GPOS uses conversions from shared GSUB rule representations (`ot/parse_lookup_gpos.go:767`). Remaining orchestration can be simplified if useful. |
| 11 | Forward/backward matching duplication | Addressed. Directional wrappers delegate to `matchCoverageSequence`, `matchGlyphSequence`, and `matchClassSequence` (`otlayout/feature.go:817`, `:953`, `:1037`). Commented-out predecessor bodies remain cleanup candidates. |
| 12 | Interface-to-concrete coupling | Partially valid, originally imprecise. RootTagMap/RootList were interface-to-interface capability assertions, not concrete casts, and are now gone. The cmap concrete type switch remains (`ot/parse.go:260`), as do unchecked AsGSub/AsGPos assumptions in ApplyFeature. |
| 13 | Linear tag-map search | Superseded for the cited implementation. Script/feature graphs use tag-index maps (`ot/layout_scripts_features.go:13`, `:59`). Other linear searches should be profiled before assigning performance severity. |
| 14 | GPOS application unimplemented | Obsolete. `otlayout/gpos.go` implements positioning operations and `otlayout/feature.go:600` dispatches them; runtime golden tests exist. This does not establish complete shaping/spec conformance. |
| 15 | Miscellaneous TODOs | Mixed. VarArray and the old navigation stub are gone. Glyph bounds TODO (`otlayout/gsub.go:36`) and JSTF checks TODO (`ot/parse.go:312`) remain. The old bounded varArray loop was not evidence of an infinite loop merely because its comment asked about one. |
| 16 | Empty structs hide parse errors | Cited helpers removed. New graphs/nodes expose Error methods and typed parsers return errors. End-to-end lazy diagnostic propagation still merits separate review, but the original examples no longer substantiate this finding. |

### Shared-helper proposals: revised disposition

- **Layout-table pipeline:** still useful. Keep concrete GSUB/GPOS construction in the wrappers and pass their existing embedded `*LayoutTable` into a shared internal orchestration helper. Preserve parse order and error short-circuiting. Avoid constructing a second layout object solely to copy it back.
- **Subtable dispatcher:** retire the old proposal; extend the existing concrete dispatcher only when a demonstrated common operation warrants it.
- **Chained rules:** retain typed return values. If extracting the remaining sequence, use an internal typed parts structure or a narrowly constrained generic helper, preserving glyph/class semantics and input-count-minus-one handling. Do not introduce the old `interface{}` callback proposal, which would recreate the coupling the review criticized.
- **Directional matching:** already implemented. Remove dead commented-out bodies in a later cleanup; preserve the different forward/backward starting offsets.

### Project status and verification

The latest commit in this checkout is dated 2026-02-16. Since the original review, the project has moved toward concrete, lazy script/feature/lookup graphs, typed GSUB/GPOS payloads, and BufferState-based application. It also includes the `otshape` pipeline, Arabic/Hebrew shaping packages, `ot-tools`, and expanded synthetic, font-golden, TTX, and HarfBuzz-fixture tests. This reassessment focuses on the old findings rather than certifying all of those newer components.

`go test ./...` passed on 2026-10-25 across all packages with tests. This is a useful baseline, not proof that malformed-font paths are safe: some font-golden tests explicitly skip subtables reporting parse errors (`ot/parse_concrete_golden_fonts_test.go:317`, `:369`). No race run, fuzz campaign, or fresh external reference-engine comparison was performed in this reassessment. `go.mod` also retains an absolute local replacement for `github.com/go-text/typesetting`; it did not prevent this test run, but should be revisited for portable development if that dependency is used again.

Only this review document was changed. No production fixes were implemented.

---

## Historical review (2026-02-05; superseded where noted above)

Date: 2026-02-05

Scope: `ot/` and `otlayout/` packages only. This is a source scan focused on code smells, design issues, duplication, unsafe casts, long/complex flows, and weak documentation.

---

## Findings (ordered by severity)

### High


4) **Panics on malformed or unexpected input in core parsing paths**
   - Locations:
     - `ot/cmap.go:108-116` (`panic("unreachable")`)
     - `ot/cmap.go:283-292` (debug panic)
     - `ot/bytes.go:752-756` (count mismatch)
     - `ot/bytes.go:905-908` (subset bounds)
     - `ot/layout.go:611-614` (subset bounds)
   - Impact: Parsing untrusted fonts can crash the process rather than return structured errors. This conflicts with the stated error-tolerant design.
   - Evidence:
     ```go
     panic("unreachable")
     panic("Hurray! Font with cmap format 4, offset > 0 and delta > 0, detected!")
     panic("record count n not equal to given count N")
     panic("subset of tag record map: cannot apply link > |record array|")
     panic("subset of lookup list: index out of range")
     ```

5) **Lookup list navigation lacks bounds checks**
   - Location: `ot/layout.go:640-653`
   - Impact: `Navigate(i)` can panic on out-of-range `i`, which may originate from malformed fonts or incorrect caller indices.
   - Evidence:
     ```go
     if ll.lookupsCache == nil {
         ll.lookupsCache = make([]Lookup, ll.length)
     } else if ll.lookupsCache[i].Type != 0 { ... }
     lookupPtr := ll.Get(i)
     lookup := ll.base[lookupPtr.U16(0):]
     ```

---

### Medium

6) **`mapWrapper.Range` does not iterate map entries**
   - Location: `ot/bytes.go:966-989`
   - Impact: `Range()` always yields zero values because `Get` is a stub; callers expecting tag iteration will silently receive empty/invalid data.
   - Evidence:
     ```go
     func (mw mapWrapper) Get(int) (Tag, NavLink) { return 0, link16{} }
     func (mw mapWrapper) Range() iter.Seq2[Tag, NavLink] {
         for i := range mw.Len() { tag, link := mw.Get(i); yield(tag, link) }
     }
     ```

7) **Silent error masking in `array.Get`**
   - Location: `ot/bytes.go:582-588`
   - Impact: Out-of-range access resets `i` to 0 and returns the first element, masking input corruption and making downstream errors hard to diagnose.
   - Evidence:
     ```go
     if i < 0 || (i+1)*a.recordSize > len(a.loc.Bytes()) { i = 0 }
     b, _ := a.loc.view(i*a.recordSize, a.recordSize)
     return b
     ```

8) **Duplicate parsing flow for GSUB/GPOS tables**
   - Locations:
     - `ot/parse_gsub.go:4-18`
     - `ot/parse_gpos.go:4-18`
   - Impact: Two near-identical functions increase maintenance cost and risk divergence.
   - Evidence:
     ```go
     err = parseLayoutHeader(...)
     err = parseLookupList(...)
     err = parseFeatureList(...)
     err = parseScriptList(...)
     ```

9) **Duplicate subtable dispatch logic for GSUB/GPOS**
   - Locations:
     - `ot/parse_gsub.go:27-63`
     - `ot/parse_gpos.go:135-178`
   - Impact: Similar coverage parsing and switch logic re-implemented twice; harder to keep validation consistent.
   - Evidence (GSUB/GPOS both do format/coverage parse then switch):
     ```go
     format := b.U16(0)
     covlink, err := parseLink16(b, 2, b, "Coverage")
     switch lookupType { ... }
     ```

10) **Repeated, near-identical rule parsing for chained context**
    - Locations:
      - `ot/parse.go:1602-1704` (`ChainedSequenceRule`)
      - `ot/parse.go:1708-1819` (`ChainedClassSequenceRule`)
    - Impact: Duplicated bounds-check-heavy parsing makes it easy to fix one path and forget the other.
    - Evidence (same structure repeated for backtrack/input/lookahead/lookup):
      ```go
      backtrackCount := int(b.U16(0))
      ...
      inputCount := int(b.U16(offset))
      ...
      lookaheadCount := int(b.U16(offset))
      ...
      seqLookupCount := int(b.U16(offset))
      ```

12) **Interface-to-concrete type assertions create brittle coupling**
    - Locations:
      - `otlayout/layout.go:85-118` (asserts `ot.RootTagMap`, `ot.RootList`)
      - `otlayout/feature.go:177-182` (assumes `Table(...).Self().AsGSub/AsGPos()` non-nil)
      - `ot/parse.go:245-252` (type-switches concrete cmap glyph index formats)
    - Impact: Adding new implementations (e.g., new glyph index format) or using alternate `NavList`/`TagRecordMap` implementations may silently fail or return empty results.

13) **Linear searches in potentially large maps without fallback optimization**
    - Location: `ot/bytes.go:815-846` (`tagRecordMap16.LookupTag`)
    - Impact: O(N) lookup for script/feature maps can be costly; comment hints at a missing binary search.
    - Evidence:
      ```go
      for i := 0; i < m.records.length; i++ { ... }
      // TODO binary search with |N| > ?
      ```

---

### Low

15) **Miscellaneous TODOs hint at unfinished behavior/unsafe edge cases**
    - Locations:
      - `ot/bytes.go:675` (possible infinite loop)
      - `otlayout/feature.go:375` (missing glyph bounds check)
      - `ot/parse.go:298` (missing JSTF requirement checks)
      - `ot/layout.go:781` (stub returns `binarySegm{}`)
    - Impact: Known gaps that could turn into latent bugs.

16) **Ad-hoc helpers with inconsistent error semantics**
    - Example: `parseVarArray16` and `parseTagRecordMap16` return empty structs on error, while other parsers return errors. This inconsistent strategy makes it hard to reason about failure paths.
    - Locations: `ot/bytes.go:635-662`, `ot/bytes.go:762-799`

---

## Suggested next steps (optional)

- Prioritize fixing  **nil-deref risk in extractLayoutInfo**.
- Replace panics in parsing with structured `errorCollector` entries or explicit errors.
- Introduce shared helpers for duplicated GSUB/GPOS parse flow and chained-context parsing.
- Add bounds checks in `LookupList.Navigate` and remove silent fallback in `array.Get`.



---

## Proposed shared helpers for duplication hotspots (design only)

These are design proposals to reduce duplication without changing behavior.

1) **Shared GSUB/GPOS layout-table parse helper**
   - Candidates: `parseGSub` and `parseGPos`.
   - Proposed signature:
     ```go
     func parseLayoutTable(tag Tag, b binarySegm, offset, size uint32, isGPos bool, ec *errorCollector) (*LayoutTable, error)
     ```
   - Each table-specific parser would wrap this and construct the concrete table type.

2) **Shared lookup subtable dispatcher skeleton**
   - Candidates: `parseGSubLookupSubtableWithDepth` and `parseGPosLookupSubtableWithDepth`.
   - Common logic:
     - Validate minimum length
     - Read `format`
     - Conditionally parse coverage link
     - Switch on lookup type to dispatch to concrete parsers
   - Proposed helper:
     ```go
     func parseLookupSubtableBase(
         b binarySegm,
         lookupType LayoutTableLookupType,
         depth int,
         coverageOffsetOk func(lookupType LayoutTableLookupType, format uint16) bool,
         dispatch func(b binarySegm, sub LookupSubtable, depth int) LookupSubtable,
     ) LookupSubtable
     ```

3) **Shared chained rule parsing helper**
   - Candidates: `ChainedSequenceRule` and `ChainedClassSequenceRule`.
   - Both parse: backtrack count/array, input count/array, lookahead count/array, sequence lookup records.
   - Proposed helper that accepts a callback to interpret the arrays (glyph vs class) but reuses bounds checks:
     ```go
     func parseChainedRule(
         loc NavLocation,
         elemSize int,
         newRule func(back, in, look array, rec array) interface{},
     ) interface{}
     ```
