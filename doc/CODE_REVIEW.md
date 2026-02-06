# Code Review: ot/ and otlayout/

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
