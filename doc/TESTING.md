# Test Strategy

We need a set of very small and very specific test fonts. Each font should be designed to include a very small set of lookups. The number of glyphs should be kept to a minimum to ensure that the tests are fast and easy to understand.

## Existing Tools and Fonts

### FontTools (Python)

FontTools ships with a `Tests` directory, where subfolders contain test fonts.

### Harfbuzz (C)

Harfbuzz ships with a `tests` directory, where subfolders contain test fonts.

## Test Driver

Test fonts are rarely labelled in a way which would be useful for our testing purposes. A small helper script exists, which traverses a test directory and greps for specific patterns in the font files. This script can be used to identify and extract the relevant test fonts for our testing purposes.

```shell
python find_test_fonts.py
```

## Implementation Plan (TTX-driven Test Bed)

### Inputs

- Mini-fonts and TTX dumps live in `testdata/fonttools-tests/`.
- TTX files follow the FontTools naming convention for subset dumps, e.g.
  `gsub3_1_simple_f1.ttx.GSUB`.

### Helper Package

Create a reusable helper package (preferred: `internal/ttxtest`) to serve all
subpackages (`ot`, `otlayout`, `otquery`, `otshape`). It should:

- Parse TTX GSUB XML into a normalized "expected" model.
- Parse a font with `ot.Parse` into an "actual" model.
- Compare expected vs actual structures with clear diffs.

### Structural Tests (package `ot`)

Goal: validate parsing correctness and detect structural mismatches.

- Load the OTF and its paired `*.ttx.GSUB`.
- Parse TTX into an expected GSUB model.
- Parse font into actual GSUB structures.
- Compare:
  - lookup count and lookup types
  - subtable formats
  - coverage glyph IDs
  - alternate sets (for GSUB-3 format 1)

Note: GSUB-only mini-fonts may lack required tables; use `Parse(..., IsTestfont)`
in structural tests to relax completeness checks while still enforcing table
validity for present tables.

## Current State (Milestone 2)

- Structural tests now cover GSUB-1 (SingleSubst) and GSUB-4 (LigatureSubst) for
  `gsub_chaining2_next_glyph_f1` (lookups 0 and 1 only).
- SingleSubst comparisons handle both format 1 (delta-based) and format 2
  (explicit mapping) using parsed lookup subtable data.
- Added a focused structural test for lookup index 2 (LigatureSubst with
  `LookupFlag=ignoreMarks`) in the same font.
- Added a focused structural test for GSUB-5 format 1 (ContextSubst) with
  `LookupFlag=ignoreMarks` using `gsub_context1_lookupflag_f1` (lookup index 4).
- Added a focused structural test for GSUB-5 format 1 (ContextSubst) using
  `gsub_context1_next_glyph_f1` (lookup index 4).
- Added a focused structural test for GSUB-5 format 2 (ContextSubst) using
  `classdef2_font4` (lookup index 3).

### Functional Tests (package `otlayout`)

Goal: black-box behavior tests for lookup application.

Constraints:

- Current lookup application does not sequence multiple lookups; for now only
  run functional tests on fonts with a single lookup or where the test can
  explicitly select one lookup.

Test flow:

- Select a lookup (by index) and apply it to a glyph buffer.
- Compare glyph sequence output to expected results.

### Functional Tests Plan (GSUB functional tests)

Purpose: validate lookup application by running one or more GSUB lookups on an
input glyph sequence and comparing output glyphs to expected results.

Scope (initial):

- Start with GSUB-3 (Alternate Substitution) using the existing mini-font.
- Use a single lookup by index; do not rely on feature/script ordering.

Test harness (package `otlayout`):

- Test file (e.g. `otlayout/feature_functional_test.go`).
- Helper: `applyGSUBLookup(font *ot.Font, lookupIndex int, input []ot.GlyphIndex) ([]ot.GlyphIndex, error)`
  - Loads GSUB lookup by index and applies it to a `GlyphBuffer`.
  - Returns output glyph IDs for comparison.
- Test case struct:
  - name
  - lookupIndex
  - input glyph IDs
  - expected glyph IDs
- Optional helper to map glyph names to IDs for readability, but hardcoding IDs
  is acceptable for now.

Constraints (explicit):

- No sequencing of multiple lookups yet.
- No sidecar files for now; embed vectors in test code.

Incremental path:

1) Implement minimal harness and GSUB-3 test cases (apply/not apply).
2) Reuse harness for GSUB-1/2/4 once stable.

### Functional Tests: How They Work

These tests are black-box checks that run one or more lookups on an input glyph
sequence and verify the resulting glyph IDs. They live in `otlayout/` and focus
on behavior (substitution/positioning), not structure.

Core mechanics:

- Load a mini-font from `testdata/fonttools-tests/` and parse it with `ot.Parse`
  using `IsTestfont` to allow GSUB-only fonts.
- Select a lookup by index from `otf.Layout.GSub.LookupList`.
- Seed a `GlyphBuffer` with the input glyph IDs (the tests currently hardcode
  IDs from the mini-fonts).
- Apply the lookup with the same dispatch path used in production
  (`applyLookup`), passing a minimal Feature wrapper and the current layout
  tables (GSUB + optional GDEF).
- Compare the resulting glyph sequence to the expected output slice.

Current harness:

- `otlayout/feature_functional_test.go` provides:
  - `loadTestFont(...)` for loading a mini-font.
  - `applyGSUBLookup(...)` for applying a single lookup by index to a buffer.
  - A minimal `Feature` implementation for tagging the lookup application.

Current coverage:

- GSUB-3 (Alternate Substitution) functional test with multiple `alt` selections
  and a non-covered glyph case.
- GSUB-5 (Contextual Substitution) format 1 functional test using
  `gsub_context1_lookupflag_f1` (lookup index 4), verifying match, mismatch,
  and offset application through nested lookup application.

Next expansions:

- GSUB-1/2/4 functional tests using the same harness.
- Add optional glyph-name helpers (if needed) for readability.
- Once feature sequencing exists, add multi-lookup tests.

### TTX Parsing Scope (initial)

Start with GSUB-3 (Alternate Substitution) format 1 only. Extend incrementally.
Current scope includes GSUB-1, GSUB-3, GSUB-4, and GSUB-5 (formats 1 and 2) for
structural comparisons. ContextSubst format 1 includes coverage, rule sets,
input glyph sequences, and sequence lookup records. ContextSubst format 2
includes coverage, class definitions, subclass rule sets, and lookup records.

### Open Questions

- Whether functional test vectors should be stored as separate sidecar files
  (e.g. JSON/YAML) or encoded in test code.

  A: We should place a separate file for input-/output-specification of glyphs besides each
  mini-font (same base-name), to be able to test the feature-/lookup-application for an array of
  input-sequences. I will derive the correct output-sequence (i.e. the test-target for functional
  tests) by running Harfbuzz on the mini-font. I will do this for various input sequences to
  create test-cases and describe the in the sidecar-file. I'll code the test-cases in the test code.
  We want to avoid importing new external dependencies.

- How strict structural comparisons should be (only intended lookup type vs all
  lookups present in the font).
- Glyph name to glyph ID mapping for TTX-derived data.

## Current State (Milestone 1)

- Added `internal/ttxtest` with a GSUB-only TTX parser (GSUB-3, format 1).
- Added a structural comparison test in `ot/`:
  - Loads `testdata/fonttools-tests/gsub3_1_simple_f1.otf`
  - Parses expected structure from `gsub3_1_simple_f1.ttx.GSUB`
  - Compares lookup type, format, coverage, and alternate sets
- Parsing options can now relax completeness/consistency checks, allowing GSUB-only
  mini-fonts to be parsed for structural tests.
