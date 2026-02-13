# `otshape` Codebase Notes (Initial Read-Through)

This document summarizes the current state of package `otshape` as of the initial implementation pass.

## 1. Package Scope Right Now

`otshape` is currently an early-stage shaper focused on:

1. script/language tag mapping
2. normalization-aware Unicode-to-glyph mapping
3. per-glyph advance extraction

It does **not** yet implement a full shaping pipeline (no GSUB/GPOS application loop, no reordering, no cluster model, no final positioning pass).

## 2. Files and Their Roles

- `otshape/doc.go`
  - package docs, tracing helper, `NOTDEF` alias (`.notdef` = glyph 0), and local `assert()`.
- `otshape/shaper.go`
  - shaping input types only:
    - `Params` (font, direction, script, language, feature ranges)
    - `FeatureRange` (tag/on-off/range/arg)
  - no exported shaping function yet.
- `otshape/language.go`
  - mapping helpers:
    - `ScriptTagForScript(language.Script) ot.Tag`
    - `LanguageTagForLanguage(language.Tag, confidence) ot.Tag`
    - `prefersDecomposed(script, lang)` for normalization preference.
- `otshape/buffer.go`
  - main implemented logic:
    - `Buffer`, `ShapedGlyph`, `NewBuffer`, `Glyphs`
    - normalization preference (`NFC` vs `NFD`)
    - font-aware representation search for composed/decomposed alternatives
    - `mapGlyphs(...)` initial mapping to glyph IDs + advance widths
  - feature lists are declared, but `applySubstitutionFeature(...)` is empty.
- `otshape/buffer_test.go`, `otshape/lang_test.go`
  - unit/functional tests around mapping and language conversion.

## 3. Implemented Shaping Behavior

### 3.1 Buffer Model

Current output unit is:

```go
type ShapedGlyph struct {
    Index   ot.GlyphIndex
    Advance sfnt.Units
}
```

So the buffer currently stores only:

1. selected glyph index
2. horizontal advance

No offsets, attachment metadata, cluster IDs, or unsafe-to-break flags yet.

### 3.2 Normalization Strategy

`normalizerFor(script, lang)` picks:

- `NFC` + `PREFER_COMPOSED` by default
- `NFD` + `PREFER_DECOMPOSED` when `prefersDecomposed(...)` is true

`scriptPreferDecomposed` currently contains:

- `dev2` (all langs)
- `bng2` (all langs)

### 3.3 Font-Aware Representation Resolution

Core idea in `findRepresentation(...)` and `representation.representNFD(...)`:

1. Try direct composed glyph if preferred and already NFC.
2. Otherwise fully decompose to NFD.
3. Recursively explore two branches per codepoint:
   - merge with nucleus via NFC recomposition (`mergeNucleus`)
   - append mark glyph (`appendMark`)
4. Choose branch based on representability and composed/decomposed preference.

This gives a basic "best available in this font" decision, e.g. choosing between precomposed glyph vs base+mark sequence.

### 3.4 Initial Mapping Step

`Buffer.mapGlyphs(input, font, script, lang)`:

1. normalizes input by preferred form
2. iterates normalization segments using `norm.Iter`
3. resolves each segment via `findRepresentation`
4. writes glyph ID and advance from `otquery.GlyphMetrics`

This is effectively a Stage-0 mapping pass, prior to GSUB/GPOS features.

## 4. Script/Language Tag Mapping Status

### 4.1 Script Mapping

`script2opentype` maps `golang.org/x/text/language.Script` string forms to OpenType script tags.

Observations:

- entries for common scripts are present (`Latn -> latn`, `Arab -> arab`, etc.)
- a large long-tail list exists
- key style is inconsistent: some keys are ISO15924 four-letter codes (`Latn`, `Deva`), others are long names (`Malayalam`, `Tibetan`, etc.), so those long-name entries may never be hit depending on `language.Script.String()` output.

### 4.2 Language Mapping

`LanguageTagForLanguage(...)` uses a matcher over a small curated set:

- Arabic, Chinese, English, Greek, German, Hebrew, Japanese, Portuguese, Romanian, Russian, Turkish

Returns `DFLT` if confidence threshold is not met or mapping is absent.

## 5. Feature Application Status

Defined but not yet implemented:

- `basicFeatures = [locl, ccmp, rlig]`
- `typographicSubstitutionFeatures = [rclt, calt, clig, liga]`
- `Buffer.applySubstitutionFeature(...)` (empty)

So `otshape` currently does **no** GSUB/GPOS feature execution.

## 6. Test Coverage and Current Test Run

Covered today:

1. representation choice behavior (`TestRepresentation`)
2. initial mapping behavior (`TestBufferInitialMapping`)
3. language matching (`TestLanguageTagForLanguage`)

`go test -v ./otshape` currently fails because proof image output path is missing:

- `open ../testdata/proofs/cafe.png: no such file or directory`

Also, the draw test path seems incomplete:

- `TestBufferDraw` calls `mapGlyphs` into local `buf`, but `displayBuffer` renders `env.buffer`, which is never populated.

## 7. Integration Readiness vs Existing Infra

### 7.1 Already Available in `otlayout`

`otlayout` already provides major building blocks needed by `otshape` next:

1. `FontFeatures(font, script, lang)` for resolved GSUB/GPOS feature lists
2. `ApplyFeature(font, feature, state, alt)` to execute lookups
3. `BufferState`, editable glyph buffer, and `PosBuffer` (x/y advances and offsets, attachment metadata)

This means feature execution does not need to be invented from scratch inside `otshape`; it can orchestrate around `otlayout`.

### 7.2 Harfbuzz Reference (local `harfbuzz` package)

From the local slimmed Harfbuzz package, useful architectural signals are:

1. shape-plan concept (properties + user features + cached decisions)
2. staged pipeline (collect/resolve features, substitute, then position)
3. default feature sets by direction/script
4. separation between run segmentation responsibilities and shaping responsibilities

These are good design references, but `otshape` currently has only the early normalization/mapping stage implemented.

## 8. Practical Gaps Identified

Main gaps before `otshape` can be considered a shaper:

1. No public shaping entry point (`Shape(...)`-style API).
2. No script engine selection or script-specific preprocessing/reordering.
3. No feature planning/resolution/apply loop using `otlayout`.
4. No cluster mapping (rune->glyph correspondence).
5. No full positioning output (offsets/attachments, not just advance width).
6. No bidi-aware behavior beyond storing direction in `Params`.
7. Buffer capacity assumptions in `mapGlyphs`: writes by index into preallocated buffer without growth checks.

## 9. Suggested Immediate Next Milestone

Smallest useful next step for `otshape`:

1. Define exported shaping API around `Params` + input text.
2. Reuse current `mapGlyphs` as initial glyph population.
3. Convert to `otlayout.BufferState` and run basic GSUB feature sequence (`locl`, `ccmp`, `rlig`, then discretionary defaults).
4. Add deterministic tests for GSUB-only shaping on a few known strings/fonts.

That would move `otshape` from "normalization mapper" to first real shaping stage.

## 10. Data Structure Direction for Streaming Shaping

Based on the current design notes in `FEATURE_APPLICATION.md` and existing runtime behavior in
`otlayout`, the most suitable direction is a hybrid model:

1. internal shaping pipeline uses a SoA-by-concern model
2. external streaming output uses AoS records

### 10.1 Recommendation

Use a **hybrid** structure:

1. Internal shaping buffer: **SoA by concern** (not pure SoA for every field).
2. External streaming output: **AoS** (`ShapedGlyph`-style record per emitted glyph).

### 10.2 Why this fits current code

1. GSUB is edit-heavy (insert/delete/replace), and current `GlyphBuffer` already supports this well.
2. GPOS data is optional early; lazy `PosBuffer` allocation keeps GSUB-only runs light.
3. Streaming sink APIs naturally consume one glyph record at a time (AoS at boundary).
4. Pure AoS internally risks bloating scanning/matching passes; pure SoA everywhere raises edit-sync complexity.

### 10.3 Concrete Internal Shape

Keep `BufferState`-style storage and extend with aligned side arrays:

1. `Glyphs []GlyphIndex` as the primary mutable sequence.
2. `Pos []PosItem`, allocated only when needed.
3. Optional aligned metadata arrays:
   - `Clusters []uint32`
   - `Masks []uint32` (feature/processing flags)
   - possibly `UnsafeFlags []uint16` later
4. Route GSUB edits through one central edit helper that mirrors the same edit span across all active arrays.

### 10.4 Decision for Next Iteration

1. Internal pipeline: hybrid SoA (`Glyphs` + optional aligned side arrays).
2. Output API: AoS writer (`WriteGlyphPos`-style per-glyph emission).
3. Keep cluster mapping in shaping state (not only transiently in `otshape`) so GSUB edits remain mapping-safe through `otlayout`.
