## buffer.go

`buffer.go` defines the mutable working state for shaping. It owns both input text state and shaped
output state, and provides the low-level operations used throughout GSUB/GPOS processing.

- Primary purpose: implement `Buffer`, the core container for glyph info, glyph positions, segment
  properties, shaping flags, context, and plan cache (buffer.go:54, buffer.go:56, buffer.go:108).
- Input staging: accepts text via `AddRune` / `AddRunes`, tracks pre/post context for cross-run
  shaping behavior, and can infer missing script/direction/language (`GuessSegmentProperties`)
  (buffer.go:127, buffer.go:148, buffer.go:193).
- In-place transformation engine: exposes cursor-based primitives (`nextGlyph`, `replaceGlyphs`,
  `copyGlyph`, `swapBuffers`, `moveTo`) that shapers and lookup code use to rewrite glyph streams
  safely (buffer.go:296, buffer.go:342, buffer.go:718, buffer.go:743).
- Cluster integrity and flags: centralizes cluster merge/update behavior, unsafe-break and
  unsafe-concat flag propagation, and range mask application, which preserves shaping correctness
  across substitutions/deletions/reordering (buffer.go:426, buffer.go:553, buffer.go:566,
  buffer.go:411, buffer.go:502).
- Ordering and traversal helpers: provides reverse/group operations and iterators over graphemes,
  clusters, and syllables to support script logic and final direction handling (buffer.go:664,
  buffer.go:689, buffer.go:792, buffer.go:819, buffer.go:843).
- Lifecycle/memory reuse: `Clear`, `clearPositions`, and output/context reset methods allow reuse of
  allocated storage between shaping runs (buffer.go:221, buffer.go:633, buffer.go:656).

In short, `buffer.go` is the operational backbone of shaping: it is where text becomes mutable
glyph data and where correctness-critical cluster/flag invariants are maintained.

## Cluster Policy Decision (Implemented)

The project decision is to simplify cluster handling to one fixed behavior and one input model.

- Single cluster mode only: monotone grapheme semantics (HarfBuzz cluster level 0) are the only
  supported mode.
- Unsupported modes: level 1 and level 2 cluster behavior are removed as user-visible/runtime
  options.
- Input model: explicit cluster-id input is removed from the public API. Cluster ids are
  auto-assigned in ascending input order.
- Feature-range semantics: `Feature.Start`/`Feature.End` remain cluster-index based; with automatic
  numbering they naturally refer to input-order ranges.
- Strict legacy parsing: test CLI compatibility accepts `--cluster-level=0` only and rejects
  `1`/`2`.

### Implementation Summary

1. API and state simplified:
removed `ClusterLevel` as a public/runtime option, dropped `Buffer.ClusterLevel`, and kept automatic
ascending cluster assignment as the only input model.
2. Internals simplified to monotone-grapheme semantics only:
removed level-1/2 branches in cluster merge/flag helpers and always use grapheme-based cluster
formation.
3. Strict legacy option handling in tests/tooling:
`--cluster-level=0` is accepted, `1`/`2` are rejected, and cluster-level plumbing was removed from
buffer setup.
4. Call sites and verification helpers updated:
`AddRune` now uses auto-cluster assignment, tests were updated, and monotonicity/unsafe-break checks
unsafe-break verification assumptions to fixed level-0 mode.
5. Validation:
`go test .` and `go test ./...` pass after the refactor.

## fonts.go

`fonts.go` is the font adapter layer between shaping logic and the underlying face data structures.

- Primary purpose: define `Font`, a cached wrapper around `Face` with GSUB/GPOS lookup accelerators
  and scale state used by shaping (fonts.go:18, fonts.go:21, fonts.go:64).
- Metric scaling bridge: converts font-space values into HarfBuzz `Position` units via
  `emScale*`/`emScalef*` helpers and applies those conversions consistently to advances, origins,
  extents, and device/variation deltas (fonts.go:106, fonts.go:113, fonts.go:148, fonts.go:310).
- Direction-aware metric/origin access: centralizes horizontal/vertical advance and origin handling,
  including fallback behavior when vertical metrics/origins are absent (fonts.go:148, fonts.go:164,
  fonts.go:193, fonts.go:200, fonts.go:213).
- Variable font support: exposes coordinate setup (`SetVarCoordsDesign`) and reads variation-aware
  deltas for positioning and ligature caret values (fonts.go:85, fonts.go:308, fonts.go:322,
  fonts.go:334).
- OT utility queries: provides shaping-specific helpers such as nominal glyph lookup, extents by
  direction, and GDEF ligature caret extraction (fonts.go:96, fonts.go:287, fonts.go:336).

In short, `fonts.go` converts raw font table data into a stable, scaled, direction-aware API that
the shaper can consume efficiently.

## glyph.go

`glyph.go` defines the core glyph records and bitfields that carry shaping state through the
pipeline.

- Primary purpose: define `GlyphInfo` (identity, cluster, masks, Unicode and shaping internals) and
  `GlyphPosition` (advances, offsets, attachment metadata) as the canonical per-glyph containers
  (glyph.go:16, glyph.go:130).
- Unicode property packing: defines `unicodeProp` layout and flag masks used to encode category,
  default-ignorable/ZWJ/ZWNJ state, continuation, and modified combining class in a compact form
  (glyph.go:37, glyph.go:53).
- Shaping-safety flags: defines public glyph flags (`GlyphUnsafeToBreak`, `GlyphUnsafeToConcat`,
  `GlyphSafeToInsertTatweel`) used by layout engines to reason about safe rebreak/concat boundaries
  (glyph.go:74).
- Ligature/component bookkeeping: stores and manipulates ligature id/component metadata used by
  GSUB/GPOS mark attachment and ligature-aware positioning (`ligProps`, `setLigProps*`,
  `getLigComp`, `getLigNumComps`) (glyph.go:159, glyph.go:275, glyph.go:297).
- Property/membership helpers: provides small predicates and mutators (`isMark`, `isZwj`,
  `isDefaultIgnorable`, `setCluster`, etc.) used everywhere in substitution/positioning logic
  (glyph.go:209, glyph.go:234, glyph.go:305, glyph.go:321).

In short, `glyph.go` defines the data contract and compact state model for every glyph as it moves
from Unicode input to positioned shaping output.

## harfbuzz.go

`harfbuzz.go` provides the package’s foundational public types and parsing utilities used across the
entire shaping pipeline.

- Primary purpose: define shared core API types and constants (`Direction`, `SegmentProperties`,
  `ShappingOptions`, `Feature`, `GID`) that other files consume (harfbuzz.go:30, harfbuzz.go:37,
  harfbuzz.go:95, harfbuzz.go:121, harfbuzz.go:167).
- Direction model: implements direction semantics and helpers (`isHorizontal`, `isBackward`,
  `Reverse`) plus script-to-default-direction mapping (`getHorizontalDirection`) used when segment
  properties are inferred or normalized (harfbuzz.go:52, harfbuzz.go:74, harfbuzz.go:90).
- Feature and variation parsing: implements user-facing parsers for OpenType feature strings and
  variation strings (`ParseFeature`, `ParseVariation`) including range/value syntax and CSS-like
  forms (harfbuzz.go:217, harfbuzz.go:340, harfbuzz.go:392, harfbuzz.go:445).
- Parser machinery: contains the internal `parser` tokenizer/state machine used by both parsers
  (`parseTag`, `parseFeatureIndices`, value parsers, prefix/postfix handling) (harfbuzz.go:222,
  harfbuzz.go:277, harfbuzz.go:340, harfbuzz.go:370, harfbuzz.go:383).
- Shared low-level helpers: centralizes common utilities (`min`/`max`, ASCII case helpers,
  `bitStorage`, `roundf`, `maxInt`) used by shaping internals in multiple files (harfbuzz.go:450,
  harfbuzz.go:478, harfbuzz.go:494, harfbuzz.go:497, harfbuzz.go:499).
- Policy note: cluster behavior is fixed to level-0 monotone grapheme semantics, with automatic
  cluster-id assignment.

In short, `harfbuzz.go` is the package’s “base layer”: it defines common shaping vocabulary and
turns external option strings into structured feature/variation inputs.

## ot_arabic_support.go / ot_arabic_fallback.go / arabic_export.go

Base Arabic code is now support-only, while the active Arabic engine lives in `otcomplex`.

- `ot_arabic_support.go`: joining-category helpers and Unicode-category heuristics used by the
  generated joining table (`ot_arabic_table.go`) and exported bridge APIs.
- `ot_arabic_fallback.go`: fallback GSUB synthesis/application from Arabic presentation-form data
  (single and ligature lookup builders, fallback plan execution).
- `arabic_export.go`: narrow exported bridge consumed by `otcomplex` for joining/category queries
  and fallback-plan creation/execution.

In short, Arabic complex shaping behavior moved out of base; base now hosts reusable Arabic support
primitives and fallback lookup synthesis only.

## ot_hebrew.go

`ot_hebrew.go` provides Hebrew-specific helper logic used by the `otcomplex` Hebrew engine.

- Hebrew composition fallback: extends canonical composition with Hebrew presentation-form
  combinations (hiriq/patah/qamats/holam/dagesh/rafe/shin-dot/sin-dot cases) for compatibility with
  older fonts when GPOS mark positioning is unavailable (ot_hebrew.go:50, ot_hebrew.go:53).
- GPOS script tag helper: returns explicit `'hebr'` for Hebrew GPOS script selection (ot_hebrew.go:105).
- Mark reordering heuristic: applies targeted reordering for specific Hebrew combining-mark
  sequences (patah/qamats + sheva/hiriq + meteg/below), with cluster merge before swap to preserve
  cluster integrity (ot_hebrew.go:111, ot_hebrew.go:122).

In short, `ot_hebrew.go` is helper logic; Hebrew engine ownership is in `otcomplex`.

## ot_language_table.go

`ot_language_table.go` is the generated OpenType language-tag mapping database.

- Primary purpose: host `otLanguages`, the canonical generated language-to-OT-tag table consumed by
  strict language lookup logic (ot_language_table.go:13).
- Generated data file: marked "DO NOT EDIT"; produced by the generator at
  `typesetting-utils/generators/langs/gen.go` (ot_language_table.go:11).
- Ambiguity resolver: `ambiguousTagToLanguage` maps ambiguous OT language tags back to preferred
  BCP-47 language values when a tag corresponds to multiple languages (ot_language_table.go:1620).
- Scope in shaping: supports OT script/language-system selection by feeding stable language-tag
  candidates into feature lookup planning.

In short, `ot_language_table.go` is the canonical generated source of OpenType language-tag
knowledge used during script/language selection in shaping.

## ot_language.go

`ot_language.go` defines the `langTag` record used by generated OpenType language mapping data.

- Primary purpose: provide the shared struct (`language`, `tag`) used by `otLanguages` in
  `ot_language_table.go` and by the runtime language index builder.
- Current architecture: lookup/search logic was moved to strict parsing (`x/text/language`) and a
  map-based resolver (`ot_language_lookup.go`), so this file is intentionally minimal.

In short, `ot_language.go` is now the data-model glue for generated language-tag entries, while
lookup behavior lives in strict parser/index code.

## ot_layout.go

`ot_layout.go` is the OpenType layout orchestration layer that ties script/language selection and
lookup execution to the shaping pipeline.

- Primary purpose: run lookup application loops (`applyString`, forward/backward traversal) over
  accelerators built from GSUB/GPOS lookup data (ot_layout.go:43, ot_layout.go:72, ot_layout.go:92).
- Script/language selection: resolves script and language systems in GSUB/GPOS with fallback policy
  (`DFLT`, `dflt`, `latn`) and default language handling (ot_layout.go:185, ot_layout.go:216).
- Feature lookup helpers: provides required-feature and variation-aware lookup list extraction for
  selected script/language (`findFeatureForLang`, `getRequiredFeature`, `getFeatureLookupsWithVar`)
  (ot_layout.go:247, ot_layout.go:264, ot_layout.go:280).
- Pipeline hooks: initializes substitution state (`layoutSubstituteStart`), performs in-place glyph
  deletion with cluster merge guarantees (`otLayoutDeleteGlyphsInplace`), and runs pre/post GPOS
  offset stages (`otLayoutPositionStart`, `otLayoutPositionFinishOffsets`) (ot_layout.go:315,
  ot_layout.go:327, ot_layout.go:377, ot_layout.go:382).

In short, `ot_layout.go` is the coordinator that selects OT script/language/feature scope and
invokes the shared GSUB/GPOS engines.

## ot_layout_gsubgpos.go

`ot_layout_gsubgpos.go` implements the shared execution engine used by both GSUB and GPOS.

- Primary purpose: define the common lookup abstraction (`layoutLookup`) and accelerator
  (`otLayoutLookupAccelerator`) that prebuilds subtable dispatch state for runtime application
  (ot_layout_gsubgpos.go:14, ot_layout_gsubgpos.go:31, ot_layout_gsubgpos.go:38).
- Matching and skipping engine: provides matcher functions plus `skippingIterator` logic that
  applies lookup flags, masks, ZWJ/ZWNJ policy, and per-syllable matching consistently
  (ot_layout_gsubgpos.go:107, ot_layout_gsubgpos.go:134, ot_layout_gsubgpos.go:180).
- Apply context state: `otApplyContext` stores mutable GSUB/GPOS execution state (GDEF, variation
  store, iterators, recursion limits, lookup props/masks, random state) and reset/init helpers
  (ot_layout_gsubgpos.go:321, ot_layout_gsubgpos.go:354, ot_layout_gsubgpos.go:388).
- Context lookup interpreter: executes context/chained-context rules, backtrack/lookahead matching,
  and nested lookup application with position fixups after buffer-length changes
  (ot_layout_gsubgpos.go:530, ot_layout_gsubgpos.go:547, ot_layout_gsubgpos.go:892).
- Ligature mechanics: contains core ligature matching and ligature-component bookkeeping used by
  GSUB ligature substitution and later GPOS attachment behavior (ot_layout_gsubgpos.go:659,
  ot_layout_gsubgpos.go:764).

In short, `ot_layout_gsubgpos.go` is the shared "interpreter" for OpenType lookup matching,
recursion, and context application that GSUB and GPOS both depend on.

## ot_layout_gsub.go

`ot_layout_gsub.go` is the GSUB-specific adapter and substitution dispatcher built on top of the
shared engine in `ot_layout_gsubgpos.go`.

- Primary purpose: implement `layoutLookup` for GSUB (`lookupGSUB`) so GSUB lookups plug into the
  common accelerator/apply flow (ot_layout_gsub.go:16, ot_layout_gsub.go:20, ot_layout_gsub.go:26).
- Lookup probing: `wouldApply` / `wouldApplyGSUB` provide "would this substitute?" logic used by
  higher-level checks before application (ot_layout_gsub.go:41, ot_layout_gsub.go:87).
- GSUB dispatch: `applyGSUB` handles OpenType substitution lookup types (single, multiple,
  alternate, ligature, contextual, chained contextual, reverse-chain single) (ot_layout_gsub.go:131).
- Substitution helpers: `applySubsSequence`, `applySubsAlternate`, and `applySubsLigature` perform
  replacement decisions, random alternates (`rand`) handling, unsafe-break propagation, and
  handoff to shared ligature bookkeeping (ot_layout_gsub.go:216, ot_layout_gsub.go:245, ot_layout_gsub.go:275).
- Recursive lookup hook: `applyRecurseGSUB` connects nested GSUB lookup invocation to the shared
  recursion framework (ot_layout_gsub.go:65).

In short, `ot_layout_gsub.go` is the GSUB front-end that maps OpenType substitution lookup types to
the common apply-context machinery.

## ot_layout_gpos.go

`ot_layout_gpos.go` is the GPOS-specific adapter and positioning dispatcher built on the shared
engine in `ot_layout_gsubgpos.go`.

- Primary purpose: implement `layoutLookup` for GPOS (`lookupGPOS`) and route GPOS subtables
  through shared lookup acceleration and apply context (ot_layout_gpos.go:15, ot_layout_gpos.go:19,
  ot_layout_gpos.go:25).
- Positioning lifecycle: initializes attachment state and finalizes propagated attachment offsets
  after lookup application (`positionStartGPOS`, `positionFinishOffsetsGPOS`) (ot_layout_gpos.go:51,
  ot_layout_gpos.go:103).
- GPOS dispatch: `applyGPOS` handles single positioning, pair positioning, cursive attachment,
  mark-base, mark-ligature, mark-mark, and contextual/chained contextual positioning
  (ot_layout_gpos.go:127).
- Value and anchor evaluation: `applyGPOSValueRecord` scales placement/advance values (including
  device/variation deltas), and `getAnchor` resolves anchor formats 1/2/3 (ot_layout_gpos.go:195,
  ot_layout_gpos.go:461).
- Attachment modeling: cursive and mark attachment helpers set attachment chains/types and unsafe
  boundaries so offsets can be accumulated correctly in final positioning
  (ot_layout_gpos.go:345, ot_layout_gpos.go:499, ot_layout_gpos.go:526, ot_layout_gpos.go:637).

In short, `ot_layout_gpos.go` is the GPOS front-end that converts OpenType positioning subtables
into concrete advances, offsets, and attachment chains.

## ot_map.go

`ot_map.go` builds and executes the shaping feature map that connects requested features to concrete
GSUB/GPOS lookups.

- Primary purpose: define `otMapBuilder`/`otMap` and compile feature requests into lookup schedules,
  masks, and stages for GSUB and GPOS (ot_map.go:57, ot_map.go:126, ot_map.go:379).
- Script/language binding: `newOtMapBuilder` resolves GSUB/GPOS script and language indices from
  segment properties so only relevant OT features/lookup lists are selected
  (ot_map.go:69, ot_map.go:77, ot_map.go:79, ot_map.go:82).
- Feature model and flags: tracks per-feature behavior (global scope, fallback, ZWJ/ZWNJ policy,
  random alternates, per-syllable matching) via `otMapFeatureFlags` and `featureInfo`
  (ot_map.go:16, ot_map.go:18, ot_map.go:43).
- Mask/bit allocation: `compile` merges duplicate feature entries, then allocates feature-value bit
  slices into `GlyphInfo.Mask` (a 32-bit per-glyph field shared with shaping safety flags) so
  feature state can be tested in O(1) during lookup traversal (glyph.go:72, glyph.go:123,
  glyph.go:152, ot_map.go:149, ot_map.go:183).
- Bit packing policy: low bits are already used by shaping safety flags, so feature allocation starts
  after those bits; boolean global features can share a reserved "global bit" (bit 31), while
  non-global and multi-valued features get dedicated contiguous ranges (`shift` + `mask`)
  (ot_map.go:126, ot_map.go:182, ot_map.go:229, ot_map.go:235).
- Default and range overrides: compiled default values are packed into `globalMask`; shaping initializes
  every glyph mask from that value, then range features overwrite only their own bit slice for selected
  clusters (`Feature.Start/End`) (ot_map.go:237, ot_map.go:383, ot_shaper.go:398, buffer.go:404,
  harfbuzz.go:194).
- Range-feature scope and tradeoff: range features are cluster-range selectors (not just raw codepoint
  spans) and preserve one shaping context while varying feature activation inside that context. For
  simple Latin styling this can often be approximated by shaping substrings separately, but that is
  not equivalent in general because cross-boundary shaping interactions may change results.
- Runtime lookup gating: each lookup carries the feature mask decided at compile time; the apply loop
  only runs a lookup on glyphs whose mask intersects that lookup mask. For alternate substitutions,
  the selected alternate index is decoded directly from the feature bit slice
  (ot_map.go:443, ot_map.go:508, ot_layout.go:77, ot_layout_gsub.go:247).
- Spec boundary: OpenType specifies feature-to-lookup processing order and per-feature glyph
  subsequence application, but does not mandate this internal bit-packing model; the mask allocator is
  a HarfBuzz engine strategy for efficient runtime filtering
  (chapter2 "Features and lookups"; ot_map.go:125, ot_layout.go:77).
- Stage and lookup scheduling: collects per-stage lookup lists (including required features and
  variation substitutions), sorts/deduplicates lookups, and inserts pause hooks
  (ot_map.go:251, ot_map.go:255, ot_map.go:263, ot_map.go:281, ot_map.go:303).
- Runtime execution: `substitute`/`position` call `apply`, which iterates stage-by-stage, filters
  lookups by feature stage/mask and apply-context flags, then runs lookup interpreters directly
  (ot_map.go:458, ot_map.go:472, ot_map.go:485, ot_map.go:507).

In short, `ot_map.go` is the shaping-plan execution map: it decides which OT lookups run, with what
mask/behavior, and in what stage order.

## ot_shape_complex.go

`ot_shape_complex.go` defines the complex-shaper contract and selects which script-specific shaper
implementation drives script-dependent shaping behavior.

- Primary purpose: define `otComplexShaper`, the lifecycle interface used by the OT shaper for
  feature collection/override, normalization hooks, mask setup, mark reordering, and pre/post
  shaping callbacks (ot_shape_complex.go:17).
- Script categorization: `categorizeComplex` chooses the shaper implementation from segment script
  and direction, currently routing Arabic/Syriac to Arabic logic (when applicable), Hebrew to
  Hebrew logic, and all other cases to default logic (ot_shape_complex.go:56).
- Default/no-op base: `complexShaperNil` provides baseline no-op behavior plus Unicode compose/
  decompose fallbacks, reducing boilerplate for script specializations (ot_shape_complex.go:77,
  ot_shape_complex.go:84, ot_shape_complex.go:88).
- Generic fallback shaper: `complexShaperDefault` defines default mark and normalization policy,
  including late GDEF zero-width-mark behavior and optional normalization disable for "dumb"
  operation modes (ot_shape_complex.go:97, ot_shape_complex.go:106, ot_shape_complex.go:113).
- Syllabic fallback helper: `syllabicInsertDottedCircles` inserts U+25CC into broken syllables
  under configurable category/placement rules when shaping flags and font coverage allow it
  (ot_shape_complex.go:120, ot_shape_complex.go:127, ot_shape_complex.go:131).

In short, `ot_shape_complex.go` is the dispatch and contract layer for script-specific shaping
policy on top of the shared OpenType pipeline.

## ot_shape_fallback.go

`ot_shape_fallback.go` implements non-GPOS fallback positioning logic for marks and spaces.

- Primary purpose: provide geometry-based fallback mark placement and space-width fallback behavior
  used when full OpenType positioning support is unavailable or insufficient
  (ot_shape_fallback.go:342, ot_shape_fallback.go:357).
- Mark class recategorization: maps Unicode combining classes to placement-oriented buckets (above,
  below, left/right variants, attached variants), including script-specific tweaks (Hebrew, Arabic/
  Syriac, Thai, Lao, Tibetan) (ot_shape_fallback.go:24, ot_shape_fallback.go:127).
- Mark placement engine: `positionMark` and `positionAroundBase` compute offsets from glyph extents,
  ligature component attachment context, and run direction, while zeroing mark advances and marking
  unsafe break ranges (ot_shape_fallback.go:152, ot_shape_fallback.go:228, ot_shape_fallback.go:231).
- Cluster-level fallback: `positionCluster` and `fallbackMarkPosition` walk cluster/base+mark runs
  and apply fallback placement across the buffer (ot_shape_fallback.go:316, ot_shape_fallback.go:342).
- Space fallback widths: `fallbackSpaces` adjusts advances for Unicode space variants (EM fractions,
  figure/punctuation/narrow spaces) and invisible-space fallback glyphs (ot_shape_fallback.go:357).

In short, `ot_shape_fallback.go` is the fallback positioning layer that preserves usable mark and
space layout when GPOS-driven positioning cannot fully handle a run.

## ot_shape_normalize.go

`ot_shape_normalize.go` performs shaping-aware Unicode normalization that is constrained by font
glyph availability.

- Primary purpose: implement `otShapeNormalize`, the preprocessing stage that decomposes/reorders/
  recomposes characters before GSUB/GPOS, with policy selected by script shaper normalization mode
  (ot_shape_normalize.go:243, ot_shape_normalize.go:52).
- Normalization context: `otNormalizeContext` wires shaper-provided compose/decompose hooks with
  current plan/buffer/font, allowing script-specific normalization behavior and backend-selected
  canonical data usage (ot_shape_normalize.go:64).
- Decomposition pass: decomposes characters only when resulting glyphs are supported, handles
  unsupported codepoints with space and U+2011 fallbacks, and preserves variation-selector clusters
  via dedicated handling (ot_shape_normalize.go:89, ot_shape_normalize.go:133, ot_shape_normalize.go:178).
- Reordering pass: sorts combining marks by modified combining class (bounded for performance) and
  allows shaper-specific post-sort reordering hooks (ot_shape_normalize.go:232, ot_shape_normalize.go:329).
- Recomposition pass: optionally recomposes marks with starters when composition is legal and glyphs
  exist, while preserving cluster correctness; also applies CGJ unhide logic for non-blocking cases
  (ot_shape_normalize.go:365, ot_shape_normalize.go:377, ot_shape_normalize.go:405).

In short, `ot_shape_normalize.go` is the font-aware normalization engine that prepares Unicode input
for stable OpenType shaping behavior.

## Normalization Refactor Checklist (x/text)

This checklist describes a safe migration path to treat `golang.org/x/text/unicode/norm` as the
canonical Unicode normalization source while preserving HarfBuzz-specific shaping behavior.

- Scope: focus on OpenType shaping for Latin/Cyrillic/Hebrew/Arabic. Chinese-specific behavior is
  out of scope for this refactor.
- Non-goal: do not replace glyph-availability gating, variation-selector handling, cluster merging,
  or script-specific mark reordering with generic Unicode normalization calls.

Phase 0 (baseline and guardrails):

1. Capture current behavior with `go test .` and keep a list of representative shaping cases for
   Latin/Hebrew/Cyrillic/Arabic.
2. Record key invariants to preserve: cluster monotonicity, no spurious glyph-not-found regressions,
   stable fallback behavior for space/U+2011 handling.

Phase 0 baseline commands:

```bash
go version
go test . -count=1
go test . -run TestPhase0 -count=1 -v
```

- Record the date and Go version next to the baseline run in this file.
- Treat `ot_shape_normalize_phase0_test.go` as the frozen guardrail suite for:
  canonical-equivalence checks (Latin/Cyrillic/Arabic), cluster monotonicity, `.notdef` regressions,
  and space/U+2011 fallback behavior.

Phase 1 (backend abstraction, no behavior change):

1. Introduce an internal normalization backend abstraction used only by `ot_shape_normalize.go`.
2. Keep the current backend as default (`internal/unicodedata`) so runtime behavior is unchanged.
3. Add a second backend implemented with `x/text/unicode/norm` primitives for canonical data access.

Status checkpoint (2026-02-07):

- Phase 0 is complete.
- Phase 0 guardrails are implemented in `ot_shape_normalize_phase0_test.go`.
- Phase 1 is complete.
- Backend abstraction and implementations are in `ot_normalize_backend.go`.
- `ot_shape_normalize.go` at this checkpoint built an `otNormalizeContext` with
  `defaultOTNormalizeBackend()` (before Phase 2 selector wiring).
- `complexShaperNil` and Hebrew compose hooks now call normalization primitives through context
  methods (`decomposeUnicode` / `composeUnicode`) instead of direct `uni.compose/decompose`.
- Runtime behavior is unchanged: default backend remains `internal/unicodedata`.
- `x/text` backend is present but not selected as default yet.
- Validation pass at this checkpoint:
  `go test . -run 'TestOTNormalizeBackendParitySmoke|TestPhase0' -count=1 -v`
  and `go test . -count=1`.

Phase 2 (hybrid integration):

1. Route canonical combining-class lookup through the backend.
2. Route per-rune canonical decomposition through the backend.
3. Route pair composition checks through the backend.
4. Keep these paths unchanged: glyph checks (`NominalGlyph`), shaper hooks (`compose`/`decompose`/
   `reorderMarks`), variation selector clusters, and cluster bookkeeping.

Status checkpoint (2026-02-07):

- Phase 2 is complete.
- `ot_shape_normalize.go` now initializes normalization with `currentOTNormalizeBackend()` and
  applies modified combining classes from backend canonical CCC values inside normalization output
  paths.
- Canonical decomposition and composition continue to flow through shaper hooks backed by the
  selected normalization backend.
- Variation-selector handling, glyph availability gating, and cluster bookkeeping are unchanged.
- Backend parity test coverage was expanded in `ot_normalize_backend_test.go`:
  representative-script parity and reference-case parity (legacy vs `x/text`).
- Midpoint reevaluation gate passed:
  `go test . -run 'TestOTNormalizeBackendParitySmoke|TestPhase2NormalizationBackendParityRepresentative|TestPhase2NormalizationBackendParityReferenceCases|TestPhase0' -count=1 -v`
  and `go test . -count=1`.

Midpoint reevaluation gate (mandatory before cleanup):

1. Run `go test .`.
2. Diff shaping outputs for the representative script set.
3. Stop and fix before proceeding if regressions are not clearly attributable to intended canonical
   Unicode behavior.

Phase 3 (switch default backend):

1. Make the `x/text` backend the default for canonical normalization primitives.
2. Keep legacy backend available behind a temporary debug switch for side-by-side comparison.

Status checkpoint (2026-02-07):

- Phase 3 is complete.
- Default canonical normalization backend selection is now `x/text`.
- A temporary process-level debug switch was introduced for side-by-side comparison
  during Phase 3; it is removed in Phase 4 cleanup.
- Selection/parsing checks were added in `ot_normalize_backend_test.go`.
- Validation pass at this checkpoint:
  `go test . -run 'TestParseOTNormalizeBackendKind|TestCurrentOTNormalizeBackendSelection|TestOTNormalizeBackendParitySmoke|TestPhase2NormalizationBackendParityRepresentative|TestPhase2NormalizationBackendParityReferenceCases|TestPhase0' -count=1 -v`
  and `go test . -count=1`.

Phase 4 (cleanup):

1. Remove temporary comparison wiring once behavior is accepted.
2. Minimize direct reliance on `internal/unicodedata` for canonical compose/decompose in this
   package, while retaining HarfBuzz-specific modified combining class and shaping heuristics.

Status checkpoint (2026-02-07):

- Phase 4 is complete.
- Temporary comparison wiring was removed from `ot_normalize_backend.go`:
  no backend kind selector, no environment override, no testing swap helper.
- Normalization backend selection is now fixed to `x/text` through
  `currentOTNormalizeBackend()`.
- Legacy canonical backend implementation used only for comparison was removed.
- `ot_normalize_backend_test.go` now contains focused `x/text` smoke tests only.
- HarfBuzz-specific modified combining class mapping and script heuristics in shaping are unchanged.

Risk notes:

1. Avoid full-cluster `NFD`/`NFC` transforms in shaping loops; they may trigger stream-safe behavior
   not suitable for cluster-level shaping normalization.
2. Keep script-specific overrides (notably Hebrew/Arabic shaping nuances) as first-class behavior.

## ot_shaper.go

`ot_shaper.go` is the top-level OpenType shaping coordinator that builds plans and executes the full
shape pipeline on a buffer.

- Primary purpose: define planning/runtime structs (`otShapePlanner`, `otShapePlan`, `otContext`,
  `shaperOpentype`) and wire feature collection, map compilation, and execution policy
  (ot_shaper.go:29, ot_shaper.go:104, ot_shaper.go:240, ot_shaper.go:693).
- Plan compilation policy: applies an OpenType-focused positioning policy (GPOS when present;
  no legacy `kern` execution path), and computes key feature masks (`frac/numr/dnom/rtlm`)
  plus fallback mark-positioning toggles (ot_shaper.go:52, ot_shaper.go:75, ot_shaper.go:81,
  ot_shaper.go:85, ot_shaper.go:91).
- Feature registration stage: adds core/common/horizontal features, direction-specific features,
  user features, and shaper-specific feature hooks into the map builder in shaping order
  (ot_shaper.go:156, ot_shaper.go:178, ot_shaper.go:209, ot_shaper.go:225, ot_shaper.go:233).
- Substitution preprocessing/postprocessing: handles mirroring/vertical-char rotation, normalization,
  mask setup, fallback mark recategorization, glyph-class synthesis, GSUB execution, and
  post-substitution default-ignorable handling (ot_shaper.go:341, ot_shaper.go:414, ot_shaper.go:493,
  ot_shaper.go:515, ot_shaper.go:524).
- Positioning orchestration: sets default advances/origins, applies GPOS positioning, controls
  mark-zeroing timing, finalizes offsets, optional fallback mark positioning, and cluster
  glyph-flag propagation (ot_shaper.go:556, ot_shaper.go:577, ot_shaper.go:600, ot_shaper.go:622,
  ot_shaper.go:645).
- End-to-end entrypoint: `shaperOpentype.shape` executes the complete pipeline (Unicode props,
  clustering, native-direction handling, preprocess, substitute, position, postprocess) and restores
  final buffer direction/state limits (ot_shaper.go:715).

In short, `ot_shaper.go` is the pipeline director that turns a prepared shaping plan into concrete
GSUB/GPOS application and final positioned glyph output.

## Refactoring of Shaper Engine

The current architecture is one shared OpenType pipeline with script-specific hooks, not two fully
separate shaping engines. A single script selector chooses one shaper implementation, then the
shared GSUB/GPOS pipeline calls hook methods at defined points.

### Implemented Interface Boundary

The temporary all-encompassing interface is now `ShapingEngine` (`refactoring.go`). It is wired
into planner and plan state (replacing the old `otComplexShaper` field types), while preserving the
single shared OT executor.

Current hook groups:

- Policy: `marksBehavior`, `normalizationPreference`, `gposTag`.
- Plan-time: `collectFeatures(plan, script)`, `overrideFeatures(plan)`, `dataCreate(plan)`.
- Runtime: `preprocessText(buffer, font)`, `setupMasks(buffer, font, script)`,
  `reorderMarks(buffer, start, end)`, `postprocessGlyphs(buffer, font)`.
- Normalization hooks remain context-based: `decompose(c, ab)`, `compose(c, a, b)`.

### Extracted Narrow Interfaces

- `FeaturePlanner`: exposes only feature-map and GSUB-pause operations needed during
  `collectFeatures`/`overrideFeatures`.
- `NormalizeContext`: exposes normalization primitives (`decomposeUnicode`, `composeUnicode`) and
  `hasGposMark`.

`RuntimeContext` was introduced temporarily and then removed; runtime hooks now use explicit
parameters only.

### Script Handling Decision

`language.Script` is now threaded explicitly to only the hook paths that actually need it:

- `collectFeatures(plan, script)`
- `setupMasks(buffer, font, script)`

The previous `script()` accessor on temporary interfaces was removed. This matches the project’s
preference for explicit dataflow over dynamic context access.

### Status Against Plan

Completed:
1. Extracted and wired interface boundary (`ShapingEngine`) across planner/plan/shaper call sites.
2. Kept one shared executor (`shaperOpentype.shape` + `otMap`) unchanged.
3. Introduced narrow plan/normalization interfaces and removed broad runtime context coupling.
4. Replaced hard-wired script switch with registry-based shaper resolution (`categorizeComplex` now resolves from `SelectionContext`).
5. Added deterministic resolver behavior (score first, then name, then registration order) and resolver tests.
6. Added focused end-to-end parity fixtures for Latin/Hebrew/Arabic shaping invariants in `ot_shaper_parity_test.go`.
7. Added package split wiring boundary with new subpackages:
   - `otcore` (default/core shaper registration helpers)
   - `otcomplex` (Hebrew/Arabic shaper registration helpers)
8. Moved Hebrew/Arabic `ShapingEngine` implementations into `otcomplex` and switched base built-ins to core-only (`default`).
9. Removed duplicated legacy base Arabic engine implementation (`ot_arabic.go`) after migrating parity fixtures to registered `otcomplex` shapers.

Pending:
1. Finalize and document the reduced Arabic bridge API as the intended stable compatibility layer between base and `otcomplex`.
2. Decompose `ShapingEngine` and related helper contracts into externally implementable interfaces (exported method surface) to avoid adapter leakage.
3. Optional migration from global registry to constructor-injected registries in the future API redesign.

Current validation state: `go test .` passes after each refactor step.

### Package Split Status (Detailed)

Current state is an **engine split** with helper internals still in base.

What is already split:

- New subpackages exist and compile:
  - `harfbuzz/otcore` (`New()`, `Register()`)
  - `harfbuzz/otcomplex` (`NewHebrew()`, `NewArabic()`, `Register()`)
- `otcomplex` now owns concrete Hebrew/Arabic `ShapingEngine` types and their runtime/plan logic.
- Base registry built-ins are now core-only (`default`); complex shapers are registered from `otcomplex`.
- Dependency direction is correct for modularity:
  - `otcore`/`otcomplex` import `harfbuzz`
  - `harfbuzz` does **not** import `otcore`/`otcomplex`
- Registry duplicate handling now has an explicit sentinel (`ErrShaperAlreadyRegistered`), allowing idempotent `Register()` calls in split packages.

What remains in base package (not yet split):

- Default/core shaper implementation (`ot_shape_complex.go`).
- Arabic support/fallback bridges used by `otcomplex` remain in base:
  - joining helpers/types (`ot_arabic_support.go`, generated `ot_arabic_table.go`)
  - fallback lookup synthesis (`ot_arabic_fallback.go`)
  - exported bridge surface (`arabic_export.go`)
- `otcomplex` Arabic uses exported bridges from base for:
  - joining/category classification (`UnicodeGeneralCategory`, `ArabicJoiningType`, `ArabicIsWord`)
  - fallback plan synthesis (`NewArabicFallbackPlan([]GlyphMask, *Font)` returning `ArabicFallbackPlan`)
- Legacy Hebrew helper API still exists in base (`ot_hebrew.go`) for in-package consumers/tests, but `otcomplex` no longer delegates Hebrew shaping behavior to it.
- Default startup registration still happens in base (`builtInShapers()` in `shaper_registry.go`), and now registers only core/default.

Main blockers to a full source move:

- `ShapingEngine` externalization blocker is mostly resolved:
  - hook contracts are exported (`ShapingEngine`, `FeaturePlanner`, `NormalizeContext`, `PlanContext`, `PauseContext`),
    and `otcomplex` now ships real Hebrew/Arabic engines against that surface.
  - remaining work here is API hardening/cleanup (temporary broad interface), not basic external implementability.
- Plan/runtime coupling blocker is mostly resolved:
  - complex plan/runtime behavior now lives in `otcomplex` (including Arabic).
  - remaining coupling is intentionally narrow bridge usage from `otcomplex` into base Arabic support/fallback helpers.
  - cleanup focus is documenting/locking this reduced bridge surface.

Practical interpretation:

- Package boundary and registration API are working.
- Concrete complex-engine ownership moved to `otcomplex`.
- Remaining split work is focused on shrinking and hardening temporary compatibility bridges.
  - Bridge cleanup progress: removed unused Arabic bridge exports (joining constants and fallback-tag exporter) and decoupled fallback-plan creation from fixed-size public array constants.

### Low-Level Exports Avoided (Arabic)

If we moved Arabic fallback/support code fully into `otcomplex` without the current bridge wrappers,
the following base internals would need to be exported:

- GSUB fallback construction/runtime internals:
  `lookupGSUB` (`ot_layout_gsub.go`), `otLayoutLookupAccelerator` + `init`
  (`ot_layout_gsubgpos.go`), and `otApplyContext` plus
  `reset`/`setLookupMask`/`substituteLookup` (`ot_layout_gsubgpos.go`).
- Lookup-flag internals:
  `otIgnoreMarks` (`ot_layout.go`).
- Internal glyph-ID alias:
  `gID` (`harfbuzz.go`).
- Arabic shaping fallback data symbols:
  `firstArabicShape`, `lastArabicShape`, `arabicShaping`, and ligature table
  types/data (`arabicLig`, `arabicTableEntry`, `arabicLigatureTable`,
  `arabicLigatureMarkTable`, `arabicLigature3Table`) from
  `ot_arabic_table.go`.
- Joining/category internals (for joining-classification support):
  `generalCategory` + category constants and `uni.generalCategory`
  (`unicode.go`), plus `arabicJoining` and related joining helpers/data
  (`ot_arabic_support.go`, `ot_arabic_table.go`).
- Potential font-storage internals:
  direct `Font.face` access patterns in fallback code (`fonts.go`).
  Note: this can be avoided by using `Font.Face()` instead of exporting the
  field.

This list is the main reason we currently keep narrow bridge APIs in base
(`arabic_export.go`) instead of exporting core shaping internals broadly.

### Minimal Hook Contracts: Phase-Ordered Patch Plan

Goal: make Hebrew/Arabic shapers externally implementable in `otcomplex` while keeping the shared
OT pipeline in base and avoiding broad internals export.

#### Phase A: Export naming-only API boundary (no behavior change)

1. Export current hook method names and enum/type names used by engines:
   `ZeroWidthMarksMode`, `NormalizationMode`, `FeatureFlags`, exported `FeaturePlanner` and
   `NormalizeContext` method names.
2. Keep behavior and call graph unchanged; this is a pure API-surface rename/alias step.
3. Add compile-time assertions and targeted tests to ensure old built-ins still satisfy the
   updated interfaces.

Acceptance:
- `go test .` remains green.
- No shaping diffs in existing parity tests.

Status:
- Phase A is complete.

#### Phase B: Replace `dataCreate(*otShapePlan)` with narrow plan view

1. Introduce exported `PlanContext` to expose only what Hebrew/Arabic need at plan-init time:
   script/direction, feature mask lookup (`FeatureMask1`), and feature fallback status
   (`FeatureNeedsFallback`).
2. Replace engine hook `dataCreate(plan *otShapePlan)` with `InitPlan(ctx PlanContext)`.
3. Move Arabic per-plan state initialization (`newArabicPlan` equivalent) behind this narrow
   context.

Acceptance:
- No engine type outside base references `otShapePlan`.
- Existing Arabic/Hebrew parity tests stay green.

Status:
- Phase B is complete.
- `ShapingEngine` now uses `InitPlan(plan PlanContext)` instead of
  `DataCreate(plan *otShapePlan)`.
- Base `otShapePlan` now implements `PlanContext` (`Script`, `Direction`,
  `FeatureMask1`, `FeatureNeedsFallback`) and Arabic per-plan setup consumes only
  that narrow view.

#### Phase C: Export minimal runtime buffer/glyph operations for complex shapers

1. Add a minimal exported method set needed by Hebrew/Arabic runtime hooks:
   - buffer cluster/flag operations (`MergeClusters`, `UnsafeToBreak`, `UnsafeToConcat`,
     `UnsafeToConcatFromOutbuffer`, `SafeToInsertTatweel`, `PreContext`, `PostContext`)
   - glyph accessors/mutators (`Codepoint`, `SetCodepoint`, `ComplexAux`, `SetComplexAux`,
     `ModifiedCombiningClass`, `SetModifiedCombiningClass`, `GeneralCategory`,
     `IsDefaultIgnorable`, `Multiplied`, `LigComp`).
2. Keep storage layout unchanged; add wrappers only.

Acceptance:
- Arabic/Hebrew implementations can compile using only exported members.
- No measurable behavior drift in parity fixtures.

Status:
- Phase C is complete.
- Exported runtime buffer operations are now available:
  `MergeClusters`, `UnsafeToBreak`, `UnsafeToConcat`,
  `UnsafeToConcatFromOutbuffer`, `SafeToInsertTatweel`, `PreContext`,
  `PostContext`.
- Exported glyph accessors/mutators are now available:
  `Codepoint`, `SetCodepoint`, `ComplexAux`, `SetComplexAux`,
  `ModifiedCombiningClass`, `SetModifiedCombiningClass`,
  `GeneralCategory`, `IsDefaultIgnorable`, `Multiplied`, `LigComp`.
- External compile-surface checks were added in `otcomplex/runtime_surface_test.go`.

#### Phase D: Extract GSUB pause context and Arabic fallback integration

1. Replace internal `pauseFunc(plan *otShapePlan, font *Font, buffer *Buffer)` dependency with
   exported `GSUBPauseFunc(PauseContext) bool`.
2. Provide a narrow pause context exposing only `Font()` and `Buffer()`.
3. For Arabic fallback, expose a narrow base helper API for fallback lookup application, rather
   than exporting low-level layout accelerator internals.

Acceptance:
- Arabic pause hooks (`stch` recorder and fallback shape pause) no longer require `otShapePlan`
  type assertions.

Status:
- Phase D is complete.
- Pause hooks now use exported pause contracts:
  `GSUBPauseFunc(PauseContext) bool` with `PauseContext` exposing only
  `Font()` and `Buffer()`.
- Planner/map pause plumbing was rewired to the new type (`FeaturePlanner.AddGSUBPause`,
  `otShapePlanner.AddGSUBPause`, `otMapBuilder`/`stageMap` pause storage and execution).
- Arabic pause hooks were decoupled from `*otShapePlan`:
  `recordStch` and fallback shape pause now run as bound engine methods and read
  per-plan state from `complexShaperArabic.plan`.
- Arabic fallback lookup mask selection is now initialized during `InitPlan` and passed through
  narrow data (`fallbackMaskArray`) into fallback-plan creation; no pause-time plan assertions remain.
- Validation: `go test .` and `go test ./...` pass.

#### Phase E: Move implementations to `otcomplex`

1. Move Hebrew and Arabic engine implementations into `harfbuzz/otcomplex`.
2. Keep selection/registration behavior identical (`Name`, `Match`, `New` unchanged).
3. Base package continues to host pipeline execution and default/core engine only.

Acceptance:
- Active complex-engine path (`otcomplex`) owns Hebrew/Arabic behavior without delegating to base plan-state helpers.
- `go test ./...` passes.

Status:
- Phase E is complete.
- Completed:
  - Hebrew/Arabic `ShapingEngine` implementations now live in `harfbuzz/otcomplex`.
  - Hebrew shaping behavior is now implemented directly inside `otcomplex` (compose/reorder/tag), rather than delegated to base helpers.
  - Arabic shaping behavior is now implemented directly inside `otcomplex` (feature collection, plan init, joining/setup masks, mark reordering, stretch postprocess, fallback pause orchestration), rather than delegating to `harfbuzz.ArabicPlanState`.
  - Base built-ins are now core-only (`default`); complex engines are provided/registered via `otcomplex`.
  - Legacy duplicated base Arabic engine file (`ot_arabic.go`) was removed.
  - Parity fixtures now rely on registered `otcomplex` shapers instead of base Arabic plan-state types.
  - Validation: `go test .` and `go test ./...` pass.

#### Phase F: Cleanup and hardening

1. Remove temporary adapters/aliases introduced during A-D.
2. Add explicit API docs for exported shaper contracts.
3. Add split-focused regression tests (selection, Hebrew compose fallback, Arabic joining/stch and
   fallback pause invariants).

Acceptance:
- Public contract is minimal, documented, and sufficient for external complex shapers.
- No functional regressions in existing and new parity suites.

### Registry/Factory Wiring (Implemented)

Goal: keep the OpenType pipeline in the base package while allowing shaper implementations to live
in sub-packages that depend on base. The base package must not import those sub-packages.

Implemented contracts (base package):

- `ShapingEngine` now includes selection hooks:
  `Name() string`, `Match(SelectionContext) int`, `New() ShapingEngine`
- `SelectionContext`:
  carries only data needed for selection (`Script`, `Direction`, chosen/found GSUB/GPOS script tags).
- Registry API:
  `RegisterShaper(ShapingEngine) error`, `ClearRegistry()`, internal `resolveShaperForContext(ctx SelectionContext)`.

Selection model:

1. Build `SelectionContext` in planner creation.
2. Resolve a shaper through registry.
3. Fall back to built-in default shaper if no engine matches (or if `New()` returns `nil`).

Design intent:

- `Match(ctx)` supports script-only and conditional logic (for example, Arabic-specific direction or script-tag behavior).
- Deterministic tie-break is stable: score, then name, then registration order.
- `New()` returns per-plan instances to avoid shared mutable shaper state in runtime hooks.

Wiring options:

- Current: global default registry initialized with built-ins (`default` only).
- Split helpers now available as packages:
  - `harfbuzz/otcore`: `New()` + `Register()` for default/core engine
  - `harfbuzz/otcomplex`: `NewHebrew()`, `NewArabic()`, `Register()` for complex engines
- Future option: constructor injection of registries for non-global wiring.
- In a later stage, we will de-compose `ShapingEngine` into narrower interfaces, allowing more modularity and shared functionality between shaper packages. 

Registration semantics:

- each sub-package can export a type implementing `ShapingEngine`.
- clients can register extra engines through `RegisterShaper(...)`.
- if no registered engine matches a context, the default shaper is used.
- `ClearRegistry()` clears registered engines in the default registry (used primarily for controlled setup in tests).

Determinism/safety rules:

- registration is startup-time only; registry treated read-only during shaping.
- conflict resolution is deterministic (score, then name, then registration order).
- tests should use fresh registries rather than shared global state.


## ot_tag.go

`ot_tag.go` maps Unicode script/language identifiers to OpenType script and language tags used by
layout table selection.

- Primary purpose: provide script/language tag conversion helpers and default tags (`DFLT` / `dflt`)
  for OT script/language system lookup (ot_tag.go:14, ot_tag.go:22, ot_tag.go:52, ot_tag.go:188).
- Script-tag mapping: resolves "new" (`*2/*3`) and legacy OT script tag forms, including exceptional
  mappings, and returns prioritized script-tag candidates (`allTagsFromScript`) (ot_tag.go:52,
  ot_tag.go:84, ot_tag.go:96).
- Language-tag mapping: `otTagsFromLanguage` maps BCP-47 language strings to OT language tags via
  strict `x/text/language` parsing and canonical primary-subtag lookup, with ISO-639-3 uppercase
  fallback for unmapped 3-letter codes (ot_tag.go:104, ot_tag.go:141, ot_tag.go:151).
- Private-use overrides: `parsePrivateUseSubtag` parses `-hbsc` and `-hbot` extension subtags,
  supporting textual tag forms with normalization (ot_tag.go:159, ot_tag.go:181, ot_tag.go:188).
- Combined selector output: `newOTTagsFromScriptAndLanguage` returns final script/language tag
  candidate lists, preferring explicit private-use overrides before standard mappings
  (ot_tag.go:207, ot_tag.go:213, ot_tag.go:222).

In short, `ot_tag.go` is the tag-conversion bridge from script/language properties to the exact
OpenType tags used when selecting GSUB/GPOS script and language systems.

## shape.go

`shape.go` provides the public shaping entrypoint and the shape-plan caching layer that reuses
compiled shaping plans across runs with matching inputs.

- Primary purpose: implement `Buffer.Shape`, which acquires a cached/new shape plan and executes it
  for the current buffer properties, font, variation coordinates, and user features
  (shape.go:29, shape.go:30, shape.go:31).
- Plan data model: `shapePlan` stores segment properties, user-feature signature, and the compiled
  OpenType shaper instance (`shaperOpentype`) used for execution (shape.go:45).
- Plan initialization and normalization: `shapePlan.init` captures props/features and normalizes
  feature ranges when copying, so cache keys compare consistently for global-vs-ranged features
  (shape.go:51, shape.go:58, shape.go:61, shape.go:64).
- Cache key comparison: `userFeaturesMatch` and `equal` define the structural equivalence used to
  determine plan reuse (shape.go:74, shape.go:88).
- Plan construction and compile step: `newShapePlan` allocates a plan, initializes it, and compiles
  the OpenType shaper plan before use (shape.go:95, shape.go:104, shape.go:109).
- Per-buffer plan cache: `newShapePlanCached` maintains a cache keyed by face on the buffer, reusing
  plans when props/features match and inserting compiled plans otherwise (shape.go:130, shape.go:136,
  shape.go:139, shape.go:146, shape.go:149).

In short, `shape.go` is the API and reuse boundary for shaping: it turns a `Shape` call into a
cached compiled plan execution over the OpenType shaping pipeline.

## unicode.go

`unicode.go` is the package’s Unicode preprocessing and property engine for shaping.

- Primary purpose: compute per-rune Unicode properties and drive buffer-level preprocessing
  before GSUB/GPOS (unicode.go:596).
- Core data model: defines compact general-category enums and lookup tables (unicode.go:13,
  unicode.go:49) and exposes them via unicodeFuncs (unicode.go:286).
- Normalization/reordering support: provides modified combining-class logic (Hebrew/Arabic/
  Syriac/etc. shaping-oriented reorder tweaks) via modifiedCombiningClass (unicode.go:91,
  unicode.go:177, unicode.go:288).
- Unicode property helpers used by shaping:
    - default-ignorable detection (unicode.go:305)
    - mirrored-pair lookup for bidi (unicode.go:372)
    - variation-selector checks (unicode.go:439)
    - canonical decompose/compose wrappers (unicode.go:447)
    - space fallback classification (unicode.go:399)
- Buffer preprocessing steps:
    - sets Unicode props and continuation flags on glyph info (unicode.go:461)
    - inserts dotted circle when needed (unicode.go:503)
    - forms grapheme clusters (unicode.go:529)
    - enforces native shaping direction heuristics (unicode.go:547)
- Final property packing: computeUnicodeProps encodes category, combining class, and shaping-
  relevant flags (ignorable/ZWJ/ZWNJ/CGJ/tag handling) into internal bitfields (unicode.go:596).

In short, this file bridges raw Unicode semantics to the exact flags and ordering behavior the
shaper pipeline needs.
