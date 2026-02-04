# Lookups and Lookup Subtables

This document summarizes the current code structure and the relevant OpenType
specification sections for GSUB/GPOS lookups and lookup subtables.

## Code Reading Summary

### Lookup type enumerations
- GSUB lookup types are defined in `ot/gsub.go` as `GSubLookupType*` (1..8):
  Single, Multiple, Alternate, Ligature, Context, Chaining Context, Extension,
  Reverse Chaining.
- GPOS lookup types are defined in `ot/gpos.go` as `GPosLookupType*` (1..9):
  Single, Pair, Cursive, MarkToBase, MarkToLigature, MarkToMark, Context,
  Chained Context, Extension.

### Shared layout structures
- `ot/layout.go` defines the layout table header and the shared ScriptList,
  FeatureList, and LookupList structure in `LayoutTable`.
- It also defines `LayoutTableLookupFlag` and `LayoutTableLookupType` plus
  shared data structures (Coverage, ClassDef, etc.) used by GSUB/GPOS.

### Application pipeline in `otlayout/feature.go`
- `FontFeatures` discovers features for a script/lang by walking:
  ScriptList -> LangSys -> FeatureList -> Feature -> LookupList indices.
- `Feature.LookupCount` and `Feature.LookupIndex` read the feature's lookup
  index list (the "Feature" table data).
- `ApplyFeature` gets the appropriate `LayoutTable` and applies each lookup in
  lookup list order.
- `applyLookup` is the main dispatcher:
  - It iterates over lookup subtables and switches on subtable type/format.
  - GSUB type 1..6 are partially implemented; several cases are TODO/panic.
  - Type 7 (Extension) and type 8 (Reverse Chaining) are not yet wired in.

## Spec Summary (Microsoft Learn)

### Common layout tables (GSUB/GPOS)
- GSUB and GPOS share the same high-level organization: ScriptList, FeatureList,
  LookupList, and FeatureVariations.
- ScriptList identifies scripts; Script tables and LangSys tables reference
  FeatureList entries via feature indices.
- FeatureList is an array of FeatureRecords sorted by tag; Feature tables
  contain the list of LookupList indices for that feature.
- LookupList is an array of offsets to Lookup tables; the list order defines
  the application order of lookups.
- Most lookup subtables include a Coverage table that determines which glyphs
  the lookup applies to; ClassDef tables are used to group glyphs.

### GSUB lookup types (1..8)
- GSUB lookups use 8 types: Single, Multiple, Alternate, Ligature, Contextual,
  Chaining Contextual, Extension, Reverse Chaining Contextual Single.
- Extension (type 7) provides 32-bit offsets to other subtable types without
  adding a new substitution action.
- Reverse Chaining Contextual Single (type 8) processes from end to start and
  uses coverage-based contexts; it is intended for scripts like Arabic.

### GPOS lookup types (1..9)
- GPOS lookups use 9 types: Single, Pair, Cursive, MarkToBase, MarkToLigature,
  MarkToMark, Context, Chained Context, Extension.
- As with GSUB, a lookup can include multiple subtables of the same type; the
  choice of subtable format is based on storage efficiency.

## Code-to-Spec Mapping Notes

- The GSUB/GPOS lookup type enums in `ot/gsub.go` and `ot/gpos.go` match the
  spec lookup type enumerations and are used by parsing and dispatch logic.
- Coverage and ClassDef parsing/types are shared in `ot/layout.go` and used by
  lookup-subtable application logic in `otlayout/feature.go`.
- The current application code in `otlayout/feature.go` implements:
  - GSUB type 1 (Single) formats 1 and 2
  - GSUB type 2 (Multiple) format 1
  - GSUB type 3 (Alternate) format 1
  - GSUB type 4 (Ligature) format 1
  - GSUB type 5/6 partially scaffolded (TODOs/panics)
- No explicit GPOS subtable application logic is implemented in this file yet.

## Progress Checklist and Implementation Plan

This checklist is intended to track GSUB/GPOS lookup coverage end-to-end:
parsing, data model access, and application in `otlayout/feature.go`. Statuses
reflect what is visible in the current codebase.

Legend:
- Status: DONE / PARTIAL / TODO
- “Plan” focuses on application logic (parsing is already present for many types,
  but should be validated per lookup type as part of the tasks).

### GSUB lookup types (1..8)

- GSUB-1 Single Substitution
  - Nature: Replace one glyph with one glyph.
  - Status: DONE (Format 1 and Format 2 implemented in `gsubLookupType1Fmt1/2`).
  - Plan: Add bounds checking against max glyph ID; add tests for both formats,
    including Coverage index ordering and delta application.

- GSUB-2 Multiple Substitution
  - Nature: Replace one glyph with a sequence of glyphs.
  - Status: DONE (Format 1 implemented in `gsubLookupType2Fmt1`).
  - Plan: Validate Sequence table parsing, add tests for glyph sequence length
    and buffer expansion.

- GSUB-3 Alternate Substitution
  - Nature: Replace one glyph with one of several alternates (selected by `alt`).
  - Status: DONE (Format 1 implemented in `gsubLookupType3Fmt1`).
  - Plan: Define behavior for out-of-range `alt` (current behavior uses last if
    alt<0, else ignore); add tests for alt selection and empty alternate sets.

- GSUB-4 Ligature Substitution
  - Nature: Replace multiple glyphs with one glyph (ligatures).
  - Status: DONE (Format 1 implemented in `gsubLookupType4Fmt1`).
  - Plan: Add tests for multiple ligature records and component matching; ensure
    correct handling of overlapping ligature candidates and buffer bounds.

- GSUB-5 Contextual Substitution
  - Nature: Substitute based on context (glyph sequences) using SequenceRule sets.
  - Status: PARTIAL (format skeletons exist; TODO/panic in 5/1, 5/2, 5/3).
  - Plan:
    1) Implement format 1 (glyph-based): parse SequenceRuleSet and SequenceRule,
       match input sequence, then apply SequenceLookupRecords.
    2) Implement format 2 (class-based): use ClassDef to map glyphs to classes,
       match class sequences, then apply SequenceLookupRecords.
    3) Implement format 3 (coverage-based): check per-position Coverage tables,
       then apply SequenceLookupRecords.
    4) Add reusable helpers: matchInputSequence, matchClassSequence,
       applySequenceLookupRecords (reuses lookup application with position
       offsets).
    5) Add tests for each format with minimal fonts exercising each rule kind.

- GSUB-6 Chaining Contextual Substitution
  - Nature: Like GSUB-5 but with backtrack and lookahead sequences.
  - Status: PARTIAL (skeletons exist; TODO/panic in 6/1, 6/2, 6/3).
  - Plan:
    1) Implement format 1 (glyph-based): match backtrack, input, lookahead
       sequences, then apply SequenceLookupRecords.
    2) Implement format 2 (class-based): map glyphs via ClassDef and match
       backtrack/input/lookahead class sequences.
    3) Implement format 3 (coverage-based): check coverage arrays for backtrack,
       input, lookahead positions.
    4) Reuse helpers from GSUB-5 with added backtrack/lookahead matching.
    5) Add tests for each format; ensure buffer boundaries are respected.

- GSUB-7 Extension Substitution
  - Nature: Indirection wrapper that points to another GSUB subtable type
    using 32-bit offsets.
  - Status: TODO (not handled in `applyLookup`).
  - Plan:
    1) Extend parsing to recognize extension subtables (if not already),
       capturing the “extensionLookupType” and the referenced subtable.
    2) In `applyLookup`, detect type 7 and dispatch to the referenced subtable
       type/format (same code paths as types 1–6 or 8).
    3) Add tests using a font with extension-based lookups.

- GSUB-8 Reverse Chaining Contextual Single
  - Nature: Contextual substitution applied right-to-left; uses coverage for
    backtrack/input/lookahead and replaces input glyphs.
  - Status: TODO (not handled in `applyLookup`).
  - Plan:
    1) Parse ReverseChainSingleSubst format (coverage arrays + substitute glyphs).
    2) In `applyLookup`, process from end to start; match backtrack/lookahead
       coverage; substitute glyphs at input positions.
    3) Add tests with Arabic-like contexts and verify right-to-left behavior.

### GPOS lookup types (1..9)

- GPOS-1 Single Adjustment
  - Nature: Adjust positioning for single glyph via ValueRecord.
  - Status: TODO (no GPOS application logic in `otlayout/feature.go`).
  - Plan:
    1) Implement parsing of ValueRecords and anchor/device tables as needed.
    2) Apply ValueRecord to glyph advances/placements in buffer model.
    3) Add tests for simple X/Y placement and advance adjustments.

- GPOS-2 Pair Adjustment
  - Nature: Adjust positions for glyph pairs (kerning).
  - Status: TODO.
  - Plan:
    1) Implement format 1 (pair sets) and format 2 (class-based pairs).
    2) Add pair matching logic based on coverage/class defs.
    3) Apply ValueRecords to both glyphs; add tests for pair kern adjustments.

- GPOS-3 Cursive Attachment
  - Nature: Attach cursive glyphs by aligning entry/exit anchors.
  - Status: TODO.
  - Plan:
    1) Parse entry/exit anchors for each glyph in coverage.
    2) Adjust positions for glyph sequences using anchor alignment.
    3) Add tests for cursive scripts.

- GPOS-4 MarkToBase
  - Nature: Attach marks to base glyphs using anchor classes.
  - Status: TODO.
  - Plan:
    1) Parse MarkArray and BaseArray, class counts and anchors.
    2) Match mark glyph to base glyph; compute offsets via anchors.
    3) Add tests for combining marks.

- GPOS-5 MarkToLigature
  - Nature: Attach marks to a specific component of ligature glyphs.
  - Status: TODO.
  - Plan:
    1) Parse LigatureArray and component anchors.
    2) Select component based on mark attachment type/context.
    3) Add tests using fonts with ligature marks.

- GPOS-6 MarkToMark
  - Nature: Attach one mark glyph to another mark glyph.
  - Status: TODO.
  - Plan:
    1) Parse MarkArray and Mark2Array; apply anchor-based alignment.
    2) Ensure mark class matching and lookup flags are respected.
    3) Add tests with stacked marks.

- GPOS-7 Contextual Positioning
  - Nature: Position adjustments based on context (glyph sequences).
  - Status: TODO.
  - Plan:
    1) Implement formats analogous to GSUB-5 (glyph/class/coverage based).
    2) Apply SequenceLookupRecords with positioning lookups.
    3) Add tests for contextual positioning rules.

- GPOS-8 Chained Contextual Positioning
  - Nature: Contextual positioning with backtrack/lookahead.
  - Status: TODO.
  - Plan:
    1) Implement formats analogous to GSUB-6 (glyph/class/coverage based).
    2) Apply SequenceLookupRecords with backtrack/lookahead matching.
    3) Add tests for chained contexts.

- GPOS-9 Extension Positioning
  - Nature: Indirection wrapper for other GPOS lookup types (32-bit offsets).
  - Status: TODO.
  - Plan:
    1) Parse Extension positioning subtables.
    2) Dispatch to referenced lookup type/format in `applyLookup`.
    3) Add tests using a font with extension-based GPOS.

## Cross-Cutting Implementation Notes

- Lookup flags (ignore base/ligatures/marks, mark filtering set, attachment type)
  should be enforced during matching for both GSUB and GPOS. See “Lookup flags”.
- Contextual and chaining lookups (GSUB 5/6, GPOS 7/8) should reuse shared helpers
  for matching glyph sequences, class sequences, and coverage sequences.
- Extension lookups require a uniform “unwrap and dispatch” path to avoid
  duplicating logic across GSUB/GPOS.
- Glyph buffers should support replacement, insertion, and positioning adjustments
  to simplify GPOS code.
- An edit tracking mechanism is needed so contextual/chaining logic can keep
  lookup-record positions stable across buffer mutations.

### Lookup flags

Lookup flags are parsed and stored on `ot.Lookup.Flag` and copied into `applyCtx.flag`
in `otlayout/feature.go`. The matching/apply logic does not yet enforce these flags.
Current matching helpers (`matchCoverageForward`, `matchCoverageSequenceForward`, etc.)
already route through a `skipGlyph` hook, which currently returns false for all glyphs.
This is the intended insertion point for the ignore/mark-filtering behavior.

Status quo:
- Flags are parsed and available on each lookup (`ot.Lookup.Flag`).
- `otlayout` carries the flag in `applyCtx`, but ignores it during matching.
- No GDEF-based filtering is applied yet (mark classes / mark attachment sets).

### GDEF requirement and parsing contract

The parsing contract of package `ot` is that a successful parse yields a
workable font for later phases (`otlayout`, `otquery`, `otshape`) with a
consistent minimum of layout data. In the OpenType spec, GDEF is not universally
required, but becomes mandatory when lookup flags or lookup data require GDEF
subtables (glyph class definitions, mark attachment classes, mark filtering
sets). This is complicated further by the JSTF table, which can include lookups
that also depend on GDEF.

Status quo:
- Parsing collects GDEF requirements during the first and only pass of GSUB/GPOS
  lookup list parsing and stores them in `Layout.Requirements`.
- `extractLayoutInfo` now requires GDEF only when needed by lookup flags; if
  required subtables are missing, it raises a critical error.
- If GDEF is present, its version is still validated; GDEF is no longer required
  unconditionally.

TODO (parsing phase, actionable):
- Extend the requirement collection and cross-checking to JSTF lookups once JSTF
  parsing is implemented (treat JSTF lookups like GSUB/GPOS for these checks).

Later stages (planned):
- Implement `skipGlyph` using GDEF glyph class definitions:
  - `IGNORE_BASE_GLYPHS`, `IGNORE_LIGATURES`, `IGNORE_MARKS`.
- Support `LOOKUP_FLAG_MARK_ATTACHMENT_TYPE_MASK` using GDEF mark attachment classes.
- Support `LOOKUP_FLAG_USE_MARK_FILTERING_SET` using GDEF mark glyph sets.
- Ensure backtrack/lookahead matching uses the same skip logic.

### EditSpan tracking

`EditSpan` describes a single buffer mutation so that contextual/chaining helpers
can re-map lookup-record positions after a replacement or insertion.

- Shape: `From` (start index), `To` (exclusive end index), `Len` (length of
  replacement segment).
- Semantics: the original range `[From:To)` is replaced with `Len` glyphs.
- Used by: GSUB 1..4 handlers return an `EditSpan` to describe the mutation;
  GSUB 5/6 helpers can then update lookup-record indices using this information.
- Not used by: lookups that do not mutate glyph arrays (e.g. GPOS), and GSUB
  lookups that fail to apply should return a nil edit span.

Example (two lookup records):

```go
// Starting glyphs: [10 20 30 40]
// Record A: sequenceIndex=1 (gid 20)
// Record B: sequenceIndex=3 (gid 40)
// Apply record A first: replace [20] -> [200 201]
edit := EditSpan{From: 1, To: 2, Len: 2} // delta = +1

// After edit, glyphs: [10 200 201 30 40]
// Record B was originally index 3, but index shifts by +1 for positions >= To.
// Updated sequenceIndex for B becomes 4 (points to gid 40).
```

Coverage helpers:

- Prefer `Coverage.Match(gid)` over accessing `Coverage.GlyphRange` directly.
- Use `Coverage.Contains(gid)` when only membership matters.

## Implementation Task Breakdown (by file)

This section maps the plan to concrete files and likely code touchpoints.

### Core dispatcher
- `otlayout/feature.go`
  - Add GSUB type 7 (Extension) dispatch path in `applyLookup`.
  - Add GSUB type 8 (Reverse Chaining) dispatch path in `applyLookup`.
  - ✅ Add GPOS dispatch paths (types 1..9) to `applyLookup` or split into
    `applyLookupGSUB` / `applyLookupGPOS` for clarity.

### GSUB contextual/chaining helpers
- `otlayout/feature.go` (new helper section)
  - `matchInputGlyphSequence(...)`
  - `matchInputClassSequence(...)`
  - `matchInputCoverageSequence(...)`
  - `matchBacktrackLookaheadGlyphs(...)`
  - ✅ `applySequenceLookupRecords(...)`
  - ✅ `applySequenceLookupRecords(...)` should accept edit tracking and update
    record positions when earlier lookups change the buffer.
  - These helpers should be used by:
    - `gsubLookupType5Fmt1/2/3`
    - `gsubLookupType6Fmt1/2/3`
    - GPOS type 7/8 (shared with GSUB, but for positioning lookups)

### New helper responsibilities

- ✅ `applySequenceLookupRecords` applies nested lookups in record order and
  re-maps each record position based on earlier edits (using `EditSpan`).
- Matching helpers should operate on `GlyphBuffer` rather than raw slices to
  keep mutation semantics centralized and consistent.

### GSUB Extension and Reverse Chaining
- `ot/layout.go` (if needed)
  - Ensure extension subtable parsing captures:
    - `extensionLookupType`
    - referenced subtable bytes
  - If parsing already exists, add accessors on `LookupSubtable` to expose
    referenced subtable.
- `otlayout/feature.go`
  - `gsubLookupType7Ext(...)` (unwrap and dispatch)
  - `gsubLookupType8Reverse(...)` (right-to-left application)
 - `ot/parse_gsub.go`
   - ✅ Parse GSUB type 8 (reverse chaining) into support data
   - ✅ Extract backtrack/lookahead coverage arrays and substitute glyph IDs

### GPOS application
- `otlayout/feature.go`
  - Add GPOS type 1..6 handlers:
    - `gposLookupType1` (Single adjustment)
    - `gposLookupType2` (Pair adjustment)
    - `gposLookupType3` (Cursive)
    - `gposLookupType4/5/6` (Mark attachments)
  - Add GPOS type 7/8 handlers (contextual/chained positioning)
  - Add GPOS type 9 (Extension) handler

### Shared data models and parsing
- `ot/layout.go`
  - ✅ Confirm Coverage and ClassDef accessors provide:
    - ✅ `GlyphRange.Match(gid)` (already used by GSUB)
    - ✅ Any class lookup helpers required for GPOS/GSUB context logic
- `ot/parse.go`
  - ✅ Ensure all GSUB/GPOS subtable formats are parsed into `LookupSubtable`
    structures (or equivalent), including extension subtables.

### Tests
- `otlayout/gsub_test.go`
  - Add unit tests for GSUB types 5/6/7/8 using small fonts that exercise
    contextual, chaining, and extension behavior.
- `otlayout/gpos_test.go` (new)
  - Create tests for GPOS types 1..6 (basic) and 7/8/9 (contextual/extension).
  - Prefer minimal test fonts for deterministic results.

## Quick Table: Type → Primary Code Targets

GSUB:
- 1..4: `otlayout/feature.go` (already implemented)
- 5/6: `otlayout/feature.go` + new helpers
- 7: `otlayout/feature.go` + `ot/layout.go` (extension parsing/access)
- 8: `otlayout/feature.go` (reverse chaining logic)

GPOS:
- 1..6: `otlayout/feature.go` + `ot/layout.go` (anchors, value records)
- 7/8: `otlayout/feature.go` + new helpers (shared with GSUB)
- 9: `otlayout/feature.go` + `ot/layout.go` (extension parsing/access)

## Spec Reference URLs

- Common layout table formats:
  https://learn.microsoft.com/en-us/typography/opentype/spec/chapter2#features-and-lookups
- GSUB:
  https://learn.microsoft.com/en-us/typography/opentype/spec/gsub#gsub-table-structures
- GPOS:
  https://learn.microsoft.com/en-us/typography/opentype/spec/gpos#gpos-table-structures
