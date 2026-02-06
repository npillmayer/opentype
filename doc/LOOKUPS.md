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
- Status: ✅DONE / PARTIAL / TODO
- “Plan” focuses on application logic (parsing is already present for many types, but should be validated per lookup type as part of the tasks).

### GSUB lookup types (1..8)

- GSUB-1 Single Substitution
  - Nature: Replace one glyph with one glyph.
  - Status: ✅DONE (Format 1 and Format 2 implemented in `gsubLookupType1Fmt1/2`).
  - Plan: Add bounds checking against max glyph ID; add tests for both formats, including Coverage index ordering and delta application.

- GSUB-2 Multiple Substitution
  - Nature: Replace one glyph with a sequence of glyphs.
  - Status: ✅DONE (Format 1 implemented in `gsubLookupType2Fmt1`).
  - Plan: Validate Sequence table parsing, add tests for glyph sequence length and buffer expansion.

- GSUB-3 Alternate Substitution
  - Nature: Replace one glyph with one of several alternates (selected by `alt`).
  - Status: ✅DONE (Format 1 implemented in `gsubLookupType3Fmt1`).
  - Plan: Define behavior for out-of-range `alt` (current behavior uses last if alt<0, else ignore); add tests for alt selection and empty alternate sets.

- GSUB-4 Ligature Substitution
  - Nature: Replace multiple glyphs with one glyph (ligatures).
  - Status: ✅DONE (Format 1 implemented in `gsubLookupType4Fmt1`).
  - Plan: Add tests for multiple ligature records and component matching; ensure correct handling of overlapping ligature candidates and buffer bounds.

- GSUB-5 Contextual Substitution
  - Nature: Substitute based on context (glyph sequences) using SequenceRule sets.
  - Status: Mostly ✅DONE (formats 1–3 implemented; needs more tests).
  - Plan:
    1) Done functional tests for format 1 (contextual substitution with lookup records).
    2) Add functional tests for format 2 (class-based sequences).
    3) Add functional tests for format 3 (coverage-based sequences).
    4) Expand mini-font coverage to include multiple rules per rule set and varied match lengths.

- GSUB-6 Chaining Contextual Substitution
  - Nature: Like GSUB-5 but with backtrack and lookahead sequences.
  - Status: ✅DONE (formats 1–3 implemented in `gsubLookupType6Fmt1/2/3`; needs tests).
  - Plan:
    1) Add tests for each format when a suitable mini-font is available.
    2) Ensure buffer boundaries are respected under lookup-flag skipping.

- GSUB-7 Extension Substitution
  - Nature: Indirection wrapper that points to another GSUB subtable type using 32-bit offsets.
  - Status: ✅DONE for parsing; apply-time handling is unnecessary because parsing unwraps the referenced subtable.
  - Plan:
    1) Add tests using a font with extension-based lookups.
    2) Keep the defensive no-op log if an extension subtable ever reaches dispatch (should not happen).

- GSUB-8 Reverse Chaining Contextual Single
  - Nature: Contextual substitution applied right-to-left; uses coverage for backtrack/input/lookahead and replaces input glyphs.
  - Status: ✅DONE (format 1 implemented in `gsubLookupType8Fmt1`; needs tests).
  - Plan:
    1) Add tests with Arabic-like contexts and verify right-to-left behavior.
    2) Ensure lookup-flag skipping behaves correctly under reverse matching.

### GPOS lookup types (1..9)

- GPOS-1 Single Adjustment
  - Nature: Adjust positioning for single glyph via ValueRecord.
  - Status: ✅DONE (apply logic in `otlayout/gpos.go`; ValueRecord parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add functional tests for simple X/Y placement and advance adjustments.

- GPOS-2 Pair Adjustment
  - Nature: Adjust positions for glyph pairs (kerning).
  - Status: ✅DONE (formats 1/2 apply in `otlayout/gpos.go`; parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add functional tests for pair kern adjustments (format 1/2).

- GPOS-3 Cursive Attachment
  - Nature: Attach cursive glyphs by aligning entry/exit anchors.
  - Status: ✅DONE (format 1 apply in `otlayout/gpos.go`; parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add tests for cursive scripts when a mini-font is available.

- GPOS-4 MarkToBase
  - Nature: Attach marks to base glyphs using anchor classes.
  - Status: ✅DONE (format 1 apply in `otlayout/gpos.go`; parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add functional tests for combining marks.

- GPOS-5 MarkToLigature
  - Nature: Attach marks to a specific component of ligature glyphs.
  - Status: ✅DONE (format 1 apply in `otlayout/gpos.go`; parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add functional tests using fonts with ligature marks.

- GPOS-6 MarkToMark
  - Nature: Attach one mark glyph to another mark glyph.
  - Status: ✅DONE (format 1 apply in `otlayout/gpos.go`; parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add tests with stacked marks.

- GPOS-7 Contextual Positioning
  - Nature: Position adjustments based on context (glyph sequences).
  - Status: ✅DONE (formats 1/2/3 apply in `otlayout/gpos.go`; parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add tests for contextual positioning rules.

- GPOS-8 Chained Contextual Positioning
  - Nature: Contextual positioning with backtrack/lookahead.
  - Status: ✅DONE (formats 1/2/3 apply in `otlayout/gpos.go`; parsing in `ot/parse_gpos.go`).
  - Plan:
    1) Add tests for chained contexts.

- GPOS-9 Extension Positioning
  - Nature: Indirection wrapper for other GPOS lookup types (32-bit offsets).
  - Status: ✅DONE for parsing; apply-time handling is unnecessary because parsing unwraps the referenced subtable.
  - Plan:
    1) Add tests using a font with extension-based GPOS.
    2) Keep the defensive no-op log if an extension subtable ever reaches dispatch (should not happen).

## Cross-Cutting Implementation Notes

- ✅ Lookup flags (ignore base/ligatures/marks, mark filtering set, attachment type) are now enforced during matching for both GSUB and GPOS via `skipGlyph`. See “Lookup flags”.
- Contextual and chaining lookups (GSUB 5/6, GPOS 7/8) should reuse shared helpers for matching glyph sequences, class sequences, and coverage sequences.
  - ✅ Coverage-based helpers are implemented (`matchCoverageForward`, `matchCoverageSequenceForward`,
    `matchCoverageSequenceBackward`).
  - ✅ Glyph- and class-sequence helpers are implemented (`matchGlyphSequenceForward`, `matchClassSequenceForward`).
- ✅ Extension lookups are unwrapped during parsing (GSUB 7 / GPOS 9), so dispatch sees the referenced
  subtable type directly. No special apply-time handling exists; if an extension subtable ever reaches
  dispatch, otlayout logs and returns without applying it (defensive no-op).
- ✅ Glyph buffers should support replacement, insertion, and positioning adjustments to simplify GPOS code.
- ✅ An edit tracking mechanism is needed so contextual/chaining logic can keep lookup-record positions stable across buffer mutations.

### Positioning buffer design (GPOS groundwork)

The GSUB pipeline works purely on glyph IDs, but GPOS needs advances and offsets. To keep GSUB usable in
isolation (and avoid forcing clients to carry positioning state), GPOS should operate on a *parallel*
position buffer rather than fusing glyph ID and positioning into one struct. This matches the plan for
future `otshape` streaming: glyph IDs flow through GSUB; positioning accumulates later and can be emitted
at stream-out.

Design choice:
- Use an array-of-structs (AoS) position buffer, not SoA.
- Do not eagerly resolve anchor coordinates into absolute values; keep offsets and attachment references
  relative to anchors until final stream-out.

Proposed data model (sketch):
- `GlyphBuffer` stays as `[]GlyphIndex` (already in `otlayout/buffer.go`).
- Add `PosBuffer []PosItem` with one entry per glyph (same length as glyph buffer).
- `PosItem` should carry (tightened proposal):
  - `XAdvance`, `YAdvance` (int32): advance deltas (font units).
  - `XOffset`, `YOffset` (int32): placement offsets (relative, not absolute).
  - `AttachTo` (int32): index of the glyph this glyph is attached to; `-1` means none.
  - `AttachKind` (enum): `None`, `MarkToBase`, `MarkToLigature`, `MarkToMark`, `Cursive`.
  - `AttachClass` (uint16): mark class / attachment type, for later resolution or debugging.
  - `AnchorRef` (struct, optional): unresolved anchor references used to compute offsets at stream-out:
    - `MarkAnchor` (uint16): index into MarkArray for mark attachments (GPOS 4/5/6).
    - `BaseAnchor` (uint16): index into BaseArray / Mark2Array for mark attachments.
    - `LigatureComp` (uint16): ligature component index (GPOS 5).
    - `CursiveEntry` / `CursiveExit` (uint16): anchor indices for cursive (GPOS 3).
  - Optional shaping metadata (future use by `otshape`):
    - `Cluster` (uint32)
    - `Flags` (bitset: mark/base/ligature, unsafe-to-break, etc.).

Flow of information (rough):
1) Input → `GlyphBuffer` (glyph IDs).
2) GSUB mutates `GlyphBuffer` only.
3) GPOS operates on (`GlyphBuffer`, `PosBuffer`) and updates advances/offsets and attachments.
4) Stream-out resolves `AnchorRef` to final positions (or emits relative offsets if the client prefers).

Rationale:
- Keeps GSUB standalone and minimal.
- Allows clients to ignore positioning if only glyph IDs are needed.
- Enables later `otshape` to provide a streaming interface (`RuneRead` → `GlyphWriter`) without
  forcing immediate positioning resolution.

### GPOS helper groundwork (plan)

Before implementing any GPOS lookup types, add shared helpers and buffer maintenance so that
lookup code stays small and consistent.

1) Position buffer lifecycle helpers
   - `NewPosBuffer(n int) PosBuffer`: returns a buffer with `AttachTo = -1` for all items.
   - `PosBuffer.ResizeLike(buf GlyphBuffer)`: ensure length matches glyph buffer length.
   - `PosBuffer.ApplyEdit(edit *EditSpan)`: mirror GSUB edits so positions stay aligned.

2) ValueRecord application helpers
   - `applyValueRecord(pos *PosItem, vr ot.ValueRecord)`:
     - Adds deltas to `XAdvance/YAdvance` and `XOffset/YOffset` only for fields present in `vr`.
   - `applyValueRecordPair(p1, p2 *PosItem, v1, v2 ot.ValueRecord)`:
     - Applies paired adjustments for GPOS-2.
   - `valueRecordAt(sub *ot.LookupSubtable, idx int) ot.ValueRecord`:
     - Extracts the correct ValueRecord for the given index (format-specific storage).

3) Attachment helpers (unresolved anchors)
   - `setMarkAttachment(pos *PosItem, baseIndex int, kind AttachKind, class uint16, ref AnchorRef)`
   - `setCursiveAttachment(pos *PosItem, baseIndex int, ref AnchorRef)`
   - Do **not** resolve anchors here; leave coordinate resolution for stream-out.

4) Matching helpers reuse
   - Reuse GSUB matching helpers for GPOS-7/8:
     - `matchCoverageForward/Sequence*`, `matchGlyphSequence*`, `matchClassSequence*`,
       `matchChainedForward`.
   - Add thin wrappers only if GPOS storage layouts require different extraction.

5) Lookup-flag integration
   - Use `skipGlyph`, `nextMatchable`, `prevMatchable` in all GPOS matching paths.
   - Enforce mark filtering/attachment type consistently when searching for base/mark targets.

6) EditSpan interaction
   - GPOS lookups should not mutate glyph buffers, but contextual GPOS (types 7/8) can invoke
     nested lookups via `applySequenceLookupRecords` which already handles `EditSpan`.
   - Ensure `PosBuffer.ApplyEdit` exists for any path that reuses GSUB edits.

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
- Parsing collects GDEF requirements during the first and only pass of GSUB/GPOS lookup list parsing and stores them in `Layout.Requirements`.
- `extractLayoutInfo` now requires GDEF only when needed by lookup flags; if required subtables are missing, it raises a critical error.
- If GDEF is present, its version is still validated; GDEF is no longer required unconditionally.

TODO (parsing phase, actionable):
- Extend the requirement collection and cross-checking to JSTF lookups once JSTF parsing is implemented (treat JSTF lookups like GSUB/GPOS for these checks).

Later stages (planned):
- Implement `skipGlyph` using GDEF glyph class definitions: - `IGNORE_BASE_GLYPHS`, `IGNORE_LIGATURES`, `IGNORE_MARKS`.
- Support `LOOKUP_FLAG_MARK_ATTACHMENT_TYPE_MASK` using GDEF mark attachment classes.
- Support `LOOKUP_FLAG_USE_MARK_FILTERING_SET` using GDEF mark glyph sets.
- Ensure backtrack/lookahead matching uses the same skip logic.

### EditSpan tracking

`EditSpan` describes a single buffer mutation so that contextual/chaining helpers can re-map lookup-record positions after a replacement or insertion.

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
  - ✅ Add GPOS dispatch paths (types 1..9) to `applyLookup` or split into `applyLookupGSUB` / `applyLookupGPOS` for clarity.

### GSUB contextual/chaining helpers
- `otlayout/feature.go` (new helper section)
  - `matchInputGlyphSequence(...)`
  - `matchInputClassSequence(...)`
  - `matchInputCoverageSequence(...)`
  - `matchBacktrackLookaheadGlyphs(...)`
  - ✅ `applySequenceLookupRecords(...)`
  - ✅ `applySequenceLookupRecords(...)` should accept edit tracking and update record positions when earlier lookups change the buffer.
  - These helpers should be used by:
    - `gsubLookupType5Fmt1/2/3`
    - `gsubLookupType6Fmt1/2/3`
    - GPOS type 7/8 (shared with GSUB, but for positioning lookups)

### New helper responsibilities

- ✅ `applySequenceLookupRecords` applies nested lookups in record order and re-maps each record position based on earlier edits (using `EditSpan`).
- ✅ Matching helpers should operate on `GlyphBuffer` rather than raw slices to keep mutation semantics centralized and consistent.

### GSUB Extension and Reverse Chaining
- `ot/layout.go`
  - ✅ Ensure extension subtable parsing captures:
    - `extensionLookupType`
    - referenced subtable bytes
  - ✅ If parsing already exists, add accessors on `LookupSubtable` to expose referenced subtable: **not needed**: resolved during parsing
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
  - ✅ Ensure all GSUB/GPOS subtable formats are parsed into `LookupSubtable` structures (or equivalent), including extension subtables.

### Tests
- Structural parsing tests live in `ot/` and compare parsed GSUB structures
  against expected models extracted from TTX (via `internal/ttxtest`).
- Functional tests live in `otlayout/` and apply specific lookups to input
  glyph buffers, checking the output glyph sequence.
- `otlayout/feature_functional_test.go`
  - GSUB-3 (Alternate Substitution) functional test harness and cases.
- `otlayout/gsub_test.go`
  - Legacy tests around feature discovery and application on real fonts.
- Planned:
  - Add functional tests for GSUB types 1/2/4/5/6/7/8 using mini-fonts.
  - Add `otlayout/gpos_test.go` for GPOS types 1..6 and 7/8/9.
  - Prefer minimal test fonts for deterministic results.

## Quick Table: Type → Primary Code Targets

GSUB:
- 1..4: `otlayout/feature.go` (already implemented)
- 5/6: `otlayout/feature.go` + new helpers
- 7: `otlayout/feature.go` + `ot/layout.go` (extension parsing already implemented)
- 8: `otlayout/feature.go` (reverse chaining logic)

GPOS:
- 1..6: `otlayout/feature.go` + `ot/layout.go` (anchors, value records)
- 7/8: `otlayout/feature.go` + new helpers (shared with GSUB)
- 9: `otlayout/feature.go` + `ot/layout.go` (extension parsing already implemented)

## Spec Reference URLs

- Common layout table formats: https://learn.microsoft.com/en-us/typography/opentype/spec/chapter2#features-and-lookups
- GSUB: https://learn.microsoft.com/en-us/typography/opentype/spec/gsub#gsub-table-structures
- GPOS: https://learn.microsoft.com/en-us/typography/opentype/spec/gpos#gpos-table-structures
- GDEF: https://learn.microsoft.com/en-us/typography/opentype/spec/gdef
- JSTF: https://learn.microsoft.com/en-us/typography/opentype/spec/jstf
