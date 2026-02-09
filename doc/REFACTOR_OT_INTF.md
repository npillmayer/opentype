# Refactor Plan: Concrete GSUB/GPOS Layout Structures in `ot`

## Summary
Replace interface-driven layout navigation for GSUB/GPOS internals with concrete, typed data structures across the full shared layout graph (ScriptList, Script, LangSys, FeatureList, Feature, LookupList, Lookup), and replace generic lookup payload carriers (`VarArray`, `Support any`) with concrete lookup-specific models.

This plan is for refactoring `ot/` and documenting the transition with temporary compatibility adapters.

## Scope
1. In scope:
   1. `ot/` layout data structures for GSUB/GPOS.
   2. Shared layout graph structures and lookup-specific structures.
   3. Transitional compatibility strategy inside `ot/`.
2. Out of scope for this document:
   1. Detailed migration edits for `otlayout`, `otquery`, `otcli`.
   2. Non-layout tables (`cmap`, `name`, `kern`, etc.).
   3. Unrelated worktree files.

## Current-State Findings
1. Shared layout graph is currently exposed through generic abstractions:
   1. `LayoutTable.ScriptList` is `Navigator`.
   2. `LayoutTable.FeatureList` is `TagRecordMap`.
   3. Script/Feature/LangSys traversal depends on `NavigatorFactory`, `NavMap`, `NavList`, `NavLink`.
2. Lookup internals are partially concrete, partially generic:
   1. `Lookup` is concrete, but subtable payload is generic.
   2. `LookupSubtable` uses `Index VarArray` and `Support any`, requiring type assertions.
   3. Context/chaining helpers parse additional structures indirectly from generic carriers.
3. GSUB/GPOS share most layout structure; lookup implementations differ by table/type/format.

## Terminology
In this plan, “sub-table” means all GSUB/GPOS-internal data structures in the layout graph, not only `LookupSubtable` records.

## Target API and Type Model
1. Introduce concrete shared layout structs as the primary GSUB/GPOS API:
   1. `ScriptList`
   2. `Script`
   3. `LangSys`
   4. `FeatureList`
   5. `Feature`
   6. `LookupList`
   7. `LookupTable` (shared header fields: type/flag/count/markFilteringSet)
2. Replace generic lookup payload model with concrete lookup records:
   1. Remove `LookupSubtable.Index VarArray` and `LookupSubtable.Support any` from the new API surface.
   2. Define concrete GSUB and GPOS subtable structs per lookup type/format.
   3. Use explicit concrete fields in container records (tagged-struct style), not interface-polymorphic payloads.
3. Keep GSUB/GPOS split only where they differ:
   1. Shared: Script/Feature/Lookup-list graph and common helper records.
   2. Separate: GSUB lookup concrete records vs GPOS lookup concrete records.
4. Interface model clarification:
   1. Generic navigation interfaces (`Navigator`, `NavMap`, `NavList`, `NavLink`) are no longer the primary model for GSUB/GPOS internals.
   2. Lean domain interfaces may still be defined where useful; concrete sub-table types may implement them.
   3. Any retained interfaces must be narrow and behavior-specific (no fat interfaces and no emulated map/list/link switching API).
5. Properties to preserve from current data types (important):
   1. The current interface does not instantiate every structure it parses. We will call this behavior “lazy instantiation” of the structure types.
   2. Accessing a (previously parsed) structure member (e.g., doing a lookup of a key) will result in
   instantiating the structure. This instance will be cached to be available for all future accesses.
   3. Walking the links from one structure/sub-table to another is `nil`-safe.
6. Open design decision (deferred):
   1. Loaded fonts should be safe for concurrent read access.
   2. Cached structure creation is functionally pure: concurrent instantiation should produce equivalent objects.
   3. Unresolved: pointer identity/stability guarantees for clients that retain sub-table pointers.
   4. Decision deferred for now; document options and choose during implementation (single-instance synchronized cache vs value-based API without pointer identity guarantees, or equivalent approach).
7. Public type design rule:
   1. Public GSUB/GPOS layout types are semantic API types, not 1:1 byte-layout mirrors.
   2. OT record structs used for parsing/link resolution stay internal implementation details.
8. Concurrency and cache purity policy:
   1. Map-like lazy caches (for example key->object creation one entry at a time) are impure shared-state mutation and must be synchronization-protected.
   2. List-like semantic types (for example `LangSys`) should avoid incremental shared-cache writes to remain lock-free in read paths.
   3. If a list-like type needs derived caching, prefer one-shot initialization (`sync.Once` style) over per-entry mutation.
   4. Feature-link order in `LangSys` is not required as a strict public API contract for OT functionality; internal linkage may keep indices for resolution semantics.

## Transitional Compatibility Strategy
1. Add adapter layer in `ot/` that maps new concrete structures to legacy navigation-compatible behavior.
2. Keep old interface-based entry points temporarily:
   1. Existing `Navigator`/`NavList`/`TagRecordMap` flows remain functional via adapters.
   2. Legacy `LookupSubtable` access remains available during transition.
3. Mark legacy APIs as transitional in docs and comments.
4. Plan a deprecation/removal phase once concrete API adoption is complete.

## Phase 1 Inventory: Concrete Shared Types
This inventory is limited to shared GSUB/GPOS layout-graph structures (not lookup-type-specific payloads).

### Structures now present as concrete public graph types
1. `ScriptList`
   1. Current status: implemented in `ot/refactor.go`, parser-integrated in `ot/parse.go` (parallel to legacy interface path).
   2. Current fields:
      1. `scriptOrder []Tag` (stable declaration order)
      2. `offsetByTag map[Tag]uint16`
      3. `scriptByTag map[Tag]*Script`
      4. `featureGraph *FeatureList`
      5. `mu sync.RWMutex`
      6. `raw binarySegm`
      7. `err error`
2. `Script`
   1. Current status: implemented in `ot/refactor.go`, parser-integrated in `ot/parse.go` (parallel to legacy interface path).
   2. Current fields:
      1. `defaultLangSysOffset uint16`
      2. `langOrder []Tag`
      3. `langOffsetsByTag map[Tag]uint16`
      4. `langByTag map[Tag]*LangSys`
      5. `featureGraph *FeatureList`
      6. `defaultOnce sync.Once`
      7. `defaultLangSys *LangSys`
      8. `mu sync.RWMutex`
      9. `raw binarySegm`
      10. `err error`
3. `LangSys`
   1. Current status: implemented in `ot/refactor.go`, parser-integrated in `ot/parse.go` (parallel to legacy interface path).
   2. Current fields:
      1. `lookupOrderOffset uint16`
      2. `requiredFeatureIndex uint16` (`0xFFFF` means no required feature)
      3. `featureIndices []uint16` (internal linkage into `FeatureList`)
      4. `featureGraph *FeatureList`
      5. `featuresOnce sync.Once`
      6. `features []*Feature` (lazy one-shot resolved semantic list view)
      7. `err error`
4. `FeatureList`
   1. Current status: implemented in `ot/refactor.go`, parser-integrated in `ot/parse.go` (parallel to legacy interface path).
   2. Current fields:
      1. `featureOrder []Tag`
      2. `featureOffsetsByIndex []uint16`
      3. `featuresByIndex map[int]*Feature` (lazy materialization keyed by record index)
      4. `indicesByTag map[Tag][]int` (feature tags may repeat)
      5. `mu sync.RWMutex`
      6. `raw binarySegm`
      7. `err error`
5. `Feature`
   1. Current status: implemented in `ot/refactor.go`, parser-integrated in `ot/parse.go` (parallel to legacy interface path).
   2. Current fields:
      1. `featureParamsOffset uint16`
      2. `lookupListIndices []uint16`
      3. `raw binarySegm`
      4. `err error`

### Parser policy status (implemented)
1. Shared graph objects are validated eagerly during parse of ScriptList/FeatureList links.
2. Concrete semantic objects are instantiated lazily on accessor calls and cached.
3. Legacy interface traversal remains active in parallel during transition.

### Immutability remark
1. API returns should be immutable by design.
2. For slices or aggregate values, return defensive copies.
3. For linked semantic objects, prefer opaque struct types (or equivalent encapsulation) to prevent client mutation through exposed fields.
4. Final choice (copy-heavy API vs stronger opaque-type boundaries) remains deferred to API-hardening, but all public returns must preserve immutability guarantees.

### Record/helper types policy
1. Record structs (`scriptRecord`, `featureRecord`, language-tag record forms) remain internal parser/linkage details.
2. They are not part of the public API model and are not exposed as client-facing concrete types.
3. The public API exposes semantic containers (`ScriptList`, `Script`, `LangSys`, `FeatureList`, `Feature`) and resolution methods.

### Already concrete (Phase 1 does not require new type)
1. `LookupList` exists as a concrete type.
2. `Lookup` exists as a concrete type.
3. Phase 1 for these two is adapter/coexistence alignment, not first-time type creation.

## Phase 1 Method Matrix (Public-Minimal vs Internal/Transitional)
This matrix classifies methods as:
1. `keep`: part of the intended public-minimal API for Phase 1.
2. `internal-transitional`: may be needed for bridge/adapters but not intended as long-term public contract.
3. `remove`: not part of concrete API surface.

### `ScriptList`
| Method | Status | Notes |
|---|---|---|
| `Len() int` | keep | cardinality helper |
| `Script(tag Tag) *Script` | keep | direct semantic lookup |
| `Range() iter.Seq2[Tag, *Script]` | keep | primary ordered traversal API |
| `Error() error` | keep | aligns with nil-safe error-tolerant model |
| `Tags() []Tag` | remove | superseded by `Range()` |
| `TagAt(i int) (Tag, bool)` | remove | index accessor not required by minimal API |
| `Has(tag Tag) bool` | remove | redundant with `Script(tag) != nil` |
| `ScriptAt(i int) *Script` | remove | index accessor not required by minimal API |

### `Script`
| Method | Status | Notes |
|---|---|---|
| `DefaultLangSys() *LangSys` | keep | explicit access to script-local default language system |
| `LangSys(tag Tag) *LangSys` | keep | language-system lookup by language tag only (no `DFLT` overloading) |
| `Range() iter.Seq2[Tag, *LangSys]` | keep | primary ordered traversal API |
| `Error() error` | keep | aligns with nil-safe error-tolerant model |
| `LangTags() []Tag` | remove | superseded by `Range()` |
| `HasLang(tag Tag) bool` | remove | redundant with `LangSys(tag) != nil` |
| `LangSysAt(i int) *LangSys` | remove | index accessor not required by minimal API |

DFLT semantics:
1. `DFLT` is script-selection scope (`ScriptList`), not a `LangSys` feature selector.
2. If `ScriptList` contains a `DFLT` script record, that script must define `defaultLangSysOffset`.
3. `requiredFeatureIndex` in `LangSys` is independent of `DFLT`.

### `LangSys`
| Method | Status | Notes |
|---|---|---|
| `RequiredFeatureIndex() (uint16, bool)` | keep | semantic exposure of required feature linkage (`bool=false` means unset/`0xFFFF`) |
| `FeatureAt(i int) *Feature` | keep | list-like access for linked features |
| `Features() []*Feature` | keep | immutable slice copy in link order |
| `Error() error` | keep | aligns with nil-safe error-tolerant model |
| `LookupOrderOffset() uint16` | internal-transitional | OT raw field; not needed by long-term public semantic API |
| `Len() int` | internal-transitional | convenience for adapters; not required if `Features()`/iteration is primary |
| `FeatureIndexAt(i int) (uint16, bool)` | internal-transitional | raw linkage index; adapter/internal concern |

### `FeatureList`
| Method | Status | Notes |
|---|---|---|
| `Len() int` | keep | cardinality helper |
| `Range() iter.Seq2[Tag, *Feature]` | keep | primary ordered traversal API; preserves duplicate tags |
| `First(tag Tag) *Feature` | keep | semantic lookup convenience |
| `All(tag Tag) []*Feature` | keep | duplicate-tag aware semantic lookup |
| `Error() error` | keep | aligns with nil-safe error-tolerant model |
| `Indices(tag Tag) []int` | internal-transitional | raw position data mainly for bridge/adapters |
| `Tags() []Tag` | remove | superseded by `Range()` |
| `TagAt(i int) (Tag, bool)` | remove | index accessor not required by minimal API |
| `FeatureAt(i int) *Feature` | remove | index accessor not required by minimal API |

### `Feature`
| Method | Status | Notes |
|---|---|---|
| `LookupCount() int` | keep | minimal public linkage cardinality |
| `Error() error` | keep | aligns with nil-safe error-tolerant model |
| `FeatureParamsOffset() uint16` | internal-transitional | OT raw field; implementation detail |
| `LookupIndex(i int) (uint16, bool)` | internal-transitional | raw linkage index; adapter/internal concern |
| `LookupIndices() []uint16` | internal-transitional | raw linkage vector; adapter/internal concern |

### Cache Population Strategies

1. ScriptList:
   1. Real-world use case is usually for the client to use exactly one script. Mixing of languages is predominately occuring in browsers, but even then at most with two scripts. Pre-instantiating all scripts would be a massive waste. Example: Font "GentiumPlus-R.ttf" (testdata/ directory) has a ScriptList of `[DFLT cyrl grek latn]`. If a user loads the font to typeset Spanish, she will walk the "latn" path and never use any of the other 3, so there is no benefit of loading them.
   2. We need to load and cache scripts individually and not pre-load the whole structure as soon as one Script link is instantiated.
   3. `ScriptList` has to be guarded for thread-safety
2. Script:
   1. This map is usually small, but will contain entries which are useful for a minority of users only. Example of "GentiumPlus-R.ttf" again: When choosing Script "latn" from the ScriptList, the LangSys entries for are `[IPPH VIT ]`, meaning special "language" phonetic alphabet and Vietnamese. Most users will be using the default LangSys entry. 
   2. We need to load and cache LangSys links individually and not pre-load all.
   3. `Script`has to be guarded for thread-safety.
3. LangSys:
   1. This is a list of Feature indices which will always be used as a set. No individual use of a single feature entry except for debugging.
   2. LangSys is essentially doing a subset on the set of features available in the font.
   3. As soon as a client reads a LangSys, all Feature links may be established at once
4. FeatureList:
   1. Features are used as sets: features selected by a LangSys, features selected by the user, features selected by the shaper.
   2. The main function of features is to create a set of applicable `Lookup`s.
   3. Features may be many and mutually excluding for different LangSys selected. Instantiating all of them is a waste. 

Synchronization policy for map-like caches:
1. Use the same synchronization strategy for `ScriptList` and `FeatureList`:
   1. typed Go maps with `sync.RWMutex` on the owning struct,
   2. no `sync.Map` for these caches.
2. Rationale:
   1. Both structures are OT tag-record maps at wire level (count + tag/offset records),
   2. typed maps preserve type safety and invariant clarity better than `sync.Map`,
   3. owner-scoped locking is easier to reason about than global-ish concurrent maps.
3. Population pattern:
   1. read path: `RLock` -> cache check -> unlock,
   2. miss path: build candidate outside lock,
   3. publish path: `Lock` -> re-check -> store canonical instance -> unlock.
4. Key-domain nuance (important):
   1. `ScriptList`: cache key can be script tag (1:1 mapping),
   2. `FeatureList`: feature tags may repeat; canonical cache key must be record index,
      while tag-based APIs first resolve to index set(s) and then instantiate/index-cache entries.

### Current `refactor.go` status check
1. Methods currently present and aligned with this matrix:
   1. `ScriptList`: `Len`, `Script`, `Range`, `Error`
   2. `Script`: `DefaultLangSys`, `LangSys`, `Range`, `Error`
   3. `LangSys`: `RequiredFeatureIndex`, `FeatureAt`, `Features`, `Error`
   4. `FeatureList`: `Len`, `Range`, `Indices`, `First`, `All`, `Error`
   5. `Feature`: `LookupCount`, `Error`
2. Remaining intentional gap:
   1. Methods classified as `internal-transitional` but not currently present in `refactor.go` are deferred until adapter implementation demands them.

### Semantics and constraints
1. Methods are `nil`-safe in Phase 1.
2. Slice-returning methods return copies where applicable.
3. `Range()` is the primary traversal form for map/list-like semantic structures.
4. Methods may lazily populate internal caches; this is an implementation side-effect but does not change logical API results.
5. Map-like lazy cache population is synchronization-sensitive and guarded by owner-scoped locking (`sync.RWMutex` + re-check publish).

## LookupSubtable, Coverage, ClassDef: Instantiation Model
This section records the current behavior in `ot/` for lookup decomposition details.

### Present types
1. `LookupSubtable` exists as a concrete type with mixed generic payload fields:
   1. `Coverage Coverage`
   2. `Index VarArray`
   3. `Support any`
   4. `LookupRecords []SequenceLookupRecord`
2. `Coverage` exists as a concrete wrapper type.
3. `ClassDefinitions` exists as a concrete wrapper type.

### Parse timing (lazy vs immediate)
1. `LookupSubtable` parsing is lazy:
   1. `LookupList` parsing stores lookup offsets.
   2. `Lookup` parsing stores subtable offsets.
   3. Actual lookup-subtable parsing happens when `Lookup.Subtable(i)` is called.
2. `Coverage` parsing is immediate once a specific lookup subtable is parsed.
3. `ClassDefinitions` parsing is immediate for subtable formats that reference ClassDef tables (for example context/chaining format 2 and GPOS pair format 2).

### Materialization depth
1. `Coverage` is partially materialized:
   1. Header fields are parsed immediately.
   2. Glyph-range matching still reads from underlying bytes on demand.
2. `ClassDefinitions` is partially materialized:
   1. ClassDef format and record views are parsed immediately.
   2. `Lookup/Class` operations still decode from stored byte views on demand.

### Known bug: cache lost due to value receivers (resolved 2026-02-09)
1. The code intends to cache lazy parse results (`lookupsCache`, `subTablesCache`).
2. `LookupList.Navigate` and `Lookup.Subtable` currently use value receivers.
3. Cache writes therefore happen on copies, so cached instances are not reliably reused across calls.
4. This is a bug (not an intentional tradeoff): lazy parsing currently degenerates into repeated parsing in common call patterns.
5. Implemented fix:
   1. cache backing arrays are allocated on stable instances during parse/construction and shared across value copies,
   2. explicit parsed sentinels (`lookupsParsed`, `subTablesParsed`) prevent repeated parsing and make cache state robust for zero-value payloads,
   3. regression coverage added for repeated value-receiver traversal (`LookupList.Navigate(...).Subtable(...)`).
6. Remaining caveat:
   1. this fix addresses cache-loss and repeated parse behavior; broader legacy-path concurrency guarantees are still a separate hardening topic.

## Implementation Phases
1. Phase 1: Introduce concrete shared layout graph types in parallel with existing interfaces.
2. Phase 2: Parse GSUB/GPOS into concrete shared graph + concrete lookup payload types.
3. Phase 3: Add compatibility adapters from concrete model back to legacy interfaces.
4. Phase 4: Convert internal `ot/` consumers to concrete types.
5. Phase 5: Deprecate and later remove legacy interface-heavy surface.

### Phase 2 Implementation Plan (detailed)

#### Scope and intent
1. Introduce a concrete typed lookup graph for GSUB/GPOS in parallel with existing `LookupList`/`LookupSubtable`.
2. Remove `VarArray`/`any` from the new concrete API path while keeping legacy path functional during transition.
3. Preserve lazy-instantiation behavior with eager parse-time validation.

#### Public/concrete additions in `ot/`
1. Add a concrete lookup graph field on `LayoutTable`:
   1. `lookupGraph *LookupListGraph`
   2. accessor `LookupGraph() *LookupListGraph` (nil-safe, parallel to `ScriptGraph()`/`FeatureGraph()`).
2. Add new concrete lookup graph types (in dedicated refactor file(s)):
   1. `LookupListGraph`
   2. `LookupTable`
   3. `LookupNode` (common metadata + concrete typed payload fields, no `any` payload).
3. Keep existing legacy fields during transition:
   1. `LayoutTable.LookupList`
   2. `LookupSubtable` with `Index`/`Support` (transitional only).

#### Concrete type model for Phase 2
1. `LookupListGraph`:
   1. lookup offsets and declaration order.
   2. lazy cache for `LookupTable` by lookup index.
   3. parse root bytes and parse error state.
2. `LookupTable`:
   1. `Type`, `Flag`, `SubTableCount`, `markFilteringSet`.
   2. subtable offsets.
   3. lazy cache for concrete subtables by subtable index.
   4. local parse error state.
3. `LookupNode`:
   1. common fields: lookup type, format, coverage, parse error.
   2. concrete payload fields for GSUB/GPOS type+format variants (explicit structs, no polymorphic payload interface).
4. Shared helper payload structs:
   1. sequence/chaining context payloads (glyph/class/coverage forms),
   2. sequence lookup records,
   3. reverse-chaining payload,
   4. GPOS value/anchor/class payload groups where needed.

#### Parsing and instantiation policy
1. Eager validation at parse wiring time:
   1. lookup count/offset bounds,
   2. subtable offset bounds,
   3. type+format legality,
   4. extension depth guards.
2. Lazy concrete instantiation at accessor time:
   1. `LookupListGraph.Lookup(i)` resolves a single lookup lazily.
   2. `LookupTable.Subtable(i)` resolves a single typed subtable lazily.
3. Cache synchronization strategy:
   1. list-index caches use typed slices + one-shot guards (`sync.Once` per slot),
   2. canonical-object publication must be thread-safe and pointer-stable.
4. Extension handling:
   1. normalize extension subtables to effective underlying type payload while preserving extension-origin metadata for diagnostics.

#### Transitional coexistence rules
1. `parseLookupList` continues to populate legacy `LayoutTable.LookupList`.
2. In the same parse pass, populate `LayoutTable.lookupGraph` from the same source bytes/offsets.
3. Legacy and concrete paths must remain behaviorally equivalent for covered tests.
4. No consumer migration in this phase; consumers remain on legacy path until Phase 4.

#### Implementation slices
1. Slice 2.0: Scaffolding and wiring. (`Status: complete`)
   1. Add `LookupListGraph`/`LookupTable`/`LookupNode` scaffolds.
   2. Add `LayoutTable.LookupGraph()` and parser hook.
   3. Add baseline “graph exists + count parity” tests.
2. Slice 2.1: GSUB typed payloads. (`Status: complete`)
   1. Implement concrete payload parsing for GSUB lookup types 1–8.
   2. Include extension type 7 normalization and reverse chaining type 8 payload.
3. Slice 2.2: GPOS typed payloads. (`Status: complete`)
   1. Implement concrete payload parsing for GPOS lookup types 1–9.
   2. Include extension type 9 normalization and class/anchor payload groups.
4. Slice 2.3: Context/chaining typed rule materialization. (`Status: complete`)
   1. Move contextual/chaining rule decoding into explicit typed structures.
   2. Eliminate dependency on `VarArray` for the new concrete path.
5. Slice 2.4: Legacy cache bug fix (required stabilization). (`Status: complete`)
   1. Fix value-receiver cache loss in legacy lookup traversal (`LookupList.Navigate`, `Lookup.Subtable`) via stable cache ownership.
   2. Keep external legacy behavior unchanged.
6. Slice 2.5: Parity and concurrency hardening. (`Status: complete`)
   1. Add concrete-vs-legacy parity tests for GSUB/GPOS lookup traversal and payload summaries.
   2. Add concurrent lazy-load tests for lookup and subtable pointer stability.

#### Phase 3 kickoff slices (lookup compatibility adapter track)
1. Slice A: Add legacy adapter projection for lookup subtables. (`Status: complete`)
   1. Implement internal adapter `legacyLookupSubtableFromConcrete(*LookupNode) LookupSubtable`.
   2. Cover GSUB/GPOS type+format projections, including extension unwrapping.
2. Slice B: Wire legacy lookup-subtable traversal through concrete path. (`Status: complete`)
   1. `Lookup.Subtable(i)` now parses `LookupNode` and projects via the adapter into cached legacy `LookupSubtable`.
   2. Legacy external API shape remains unchanged.
3. Slice C: Route transitional `parseLookupSubtable` through concrete parser + adapter. (`Status: complete`)
   1. `parseLookupSubtableWithDepth` now parses a concrete `LookupNode` and projects to legacy via adapter.
   2. This removes the remaining internal dual-parser behavior from the transitional entrypoint while preserving legacy return shape.
4. Slice D: Switch `otlayout` contextual/chaining runtime to concrete-first lookup payloads. (`Status: complete`)
   1. `otlayout` lookup dispatch now threads concrete lookup table/node context in parallel with legacy lookup structs.
   2. GSUB contextual/chaining paths (types 5/6/8) and GPOS contextual/chaining paths (types 7/8) now consume concrete payloads.
   3. Nested sequence-lookup application now resolves concrete nested lookups through `LookupGraph` when available.
5. Slice E: Switch `otlayout` GSUB non-context runtime to concrete-first lookup payloads. (`Status: complete`)
   1. GSUB simple/multi/alternate/ligature execution (types 1/2/3/4) now consumes concrete payloads first.
   2. Runtime path now executes concrete payload semantics only.
6. Slice F: Switch `otlayout` GPOS non-context runtime to concrete-first lookup payloads. (`Status: complete`)
   1. GPOS single/pair/cursive/mark-attachment execution (types 1/2/3/4/5/6) now consumes concrete payloads first.
   2. Runtime path now executes concrete payload semantics only.

#### Current status snapshot (2026-02-09)
1. `Phase 1` is complete for the shared graph migration track:
   1. concrete `ScriptList` / `Script` / `LangSys` / `FeatureList` / `Feature` and lazy graph access are present in parallel with legacy interfaces.
2. `Phase 2` is active:
   1. GSUB concrete payload path is implemented for types 1–8, including extension type 7 unwrapping and reverse chaining.
   2. GPOS concrete payload path is wired and implemented for types 1–9, including extension type 9 unwrapping.
   3. Lookup graph lazy caches are guarded with one-shot synchronization and have concurrent pointer-stability tests.
   4. Parity/concurrency hardening now includes extension/context-heavy checks:
      1. GSUB extension parity over lookup traversal,
      2. GPOS extension parity over lookup traversal,
      3. GSUB contextual/chaining format-2/3 parity against legacy support payloads,
      4. GPOS payload parity checks across effective payloads (including non-extension nodes on available fonts),
      5. synthetic contextual/chaining parity checks for GPOS format-1/2/3 edge forms against legacy traversal,
      6. negative malformed-input checks for concrete GSUB/GPOS parsing (offset/format truncation and recursive extension guards),
      7. concurrent access checks for extension-resolved and context-heavy concrete nodes,
      8. `otlayout` runtime golden behavior checks for concrete-first GPOS application (single/pair/chaining forwarding plus mark-base/mark-ligature attachment metadata).
3. Transitional coexistence remains intact:
   1. parser still populates both legacy `LookupList` and concrete `LookupGraph`.
   2. legacy consumers still call the old API surface, now with lookup-subtable materialization adapter-backed from concrete lookup nodes.
   3. transitional helper parsing (`parseLookupSubtable`) is also adapter-backed from concrete lookup parsing.
   4. `otlayout` GSUB runtime (lookup types 1–8, with type 7 unwrapped) now executes concrete payloads only.
   5. `otlayout` GPOS runtime (lookup types 1–8, with type 9 unwrapped) now executes concrete payloads only, including unresolved anchor-reference bookkeeping for cursive/mark-attachment paths.

#### GPOS runtime migration status
1. Concrete-first runtime consumption is complete for all GPOS lookup families used by `otlayout`:
   1. non-context: types 1/2/3/4/5/6,
   2. context/chaining: types 7/8,
   3. extension: type 9 unwrapping to effective payload.
2. Legacy fallback for lookup behavior has been removed from runtime application paths.
3. Dead legacy helper paths for GPOS support-shape parsing have been removed from `otlayout/gpos_helpers.go`.
4. Runtime golden coverage added in `otlayout`:
   1. type 1/2 concrete-first value adjustments,
   2. type 8 chaining-to-single forwarding behavior,
   3. type 4/5 attachment metadata behavior.
5. Bug fix applied during runtime-golden pass:
   1. `gposLookupType1Fmt1` incorrectly rejected covered glyphs by requiring coverage index `== 1`; this guard was removed.

#### Batch 2.0 status (historical mode harness)
1. Batch 2.0 is complete.
2. Batch 2.0 temporarily introduced a runtime execution-mode switch and mode-parity harness during fallback removal.
3. This transitional switch/harness was removed in Slice 2.1-G after runtime behavior became concrete-only.

#### Batch 2.1 status (fallback removal by lookup family)
1. Batch 2.1 is complete for lookup-behavior fallback removal.
2. Slice 2.1-A is complete:
   1. GPOS non-context runtime families (types 1/2/3/4/5/6) no longer execute legacy fallback branches.
   2. These families now require concrete payloads unconditionally.
3. Slice 2.1-B is complete:
   1. GPOS context/chaining runtime families (types 7/8) no longer execute legacy fallback branches.
   2. These families now require concrete payloads unconditionally.
4. Slice 2.1-C is complete:
   1. GSUB runtime families (types 1/2/3/4/5/6/8, with type 7 unwrapped at parse-time) no longer execute legacy fallback branches.
   2. Synthetic GSUB dispatch tests now require explicit concrete payload nodes.
5. Slice 2.1-D is complete:
   1. GPOS cursive/mark-attachment runtime (types 3/4/5/6) now sources unresolved `AnchorRef` offsets from concrete payload metadata, not from legacy `lksub.Index`/`lksub.Support`.
   2. Added concrete payload accessor coverage for anchor-offset metadata in `ot` parser tests and `otlayout` runtime golden tests.
6. Slice 2.1-E is complete:
   1. removed dead legacy helper code in `otlayout/gpos_helpers.go` that parsed legacy `Support` shapes.
   2. validated `otlayout` and `ot` test suites after cleanup.
7. Slice 2.1-F is complete:
   1. `otlayout` lookup dispatch now threads concrete `*ot.LookupNode` directly into GSUB/GPOS handlers instead of synthesizing transitional `LookupSubtable` wrappers.
   2. GSUB/GPOS runtime handler signatures in `otlayout/gsub.go` and `otlayout/gpos.go` now consume concrete lookup nodes directly.
   3. Removed dead chained-rule legacy parsing helpers from `otlayout/feature.go`; runtime path no longer references `ot.LookupSubtable`.
   4. validated `otlayout`, `ot`, and `otquery` test suites after the dispatch-signature cleanup.
8. Slice 2.1-G is complete:
   1. removed dead transitional fallback scaffolding in `otlayout` (`runtime_mode.go` and unused fallback hooks in `feature.go`).
   2. removed dead legacy `VarArray` helper functions in `otlayout/feature.go` (`lookupGlyph`, `lookupGlyphs`) that were no longer reachable after concrete-only runtime migration.
   3. replaced mode-parity tests with deterministic concrete-runtime checks in `otlayout/concrete_mode_parity_test.go`.
   4. validated `otlayout`, `ot`, and `otquery` test suites after cleanup.
9. Batch 6 is complete (legacy layout helper API cleanup; intentional breaking change):
   1. removed legacy `otlayout` helper APIs tied to navigation abstractions (`Navigator`, `NavList`, `TagRecordMap`) from `otlayout/layout.go` and removed `otlayout/list.go`.
   2. introduced concrete helper APIs in `otlayout/layout.go`:
      1. `GetScriptGraph(table)`,
      2. `GetFeatureGraph(table)`,
      3. `GetLookupGraph(table)`,
      4. `ScriptTags(scriptGraph)`,
      5. `FeatureTags(featureGraph)`,
      6. `FeaturesForLangSys(langSys)`,
      7. `LookupsForFeature(feature, lookupGraph)`.
   3. updated `otcli` callsites to stop depending on removed legacy `otlayout` helpers (legacy subset/key listing now handled locally in `otcli`).
   4. validated `otlayout`, `ot`, `otquery`, and `otcli` package builds/tests after the API cleanup.
10. Batch 7 is complete (test migration and hardening):
   1. added dedicated concrete-helper coverage in `otlayout/layout_test.go` for:
      1. successful graph/helper resolution (`GetScriptGraph`, `GetFeatureGraph`, `GetLookupGraph`),
      2. tag extraction helpers (`ScriptTags`, `FeatureTags`),
      3. feature-to-lookup resolution (`FeaturesForLangSys`, `LookupsForFeature`).
   2. added negative-path hardening tests for:
      1. non-layout table rejection,
      2. nil/empty argument handling (`ErrVoid`, `ErrNoLookupGraph`, `ErrFeatureHasNoRefs`).
   3. validated `otlayout`, `ot`, `otquery`, and `otcli` package builds/tests after test migration.

#### What remains to finish Phase 2
1. No open verification gaps are currently tracked for Phase 2.
2. Remaining work shifts to transition planning/execution for later phases (compatibility adapters and consumer migration).
3. Deferred cleanup note (non-GSUB/GPOS):
   1. `name` table access is still map/navigation-driven (currently in `otquery/info.go`) and not modeled as a concrete semantic type.
   2. We plan a later cleanup pass introducing a lightweight hybrid `name` representation (semantic API + compact raw-backed internals) to replace map-style traversal for this table.

#### Test plan for Phase 2
1. Parse and count parity:
   1. lookup count parity between legacy and concrete graphs,
   2. subtable count parity per lookup,
   3. lookup flag/markFilteringSet parity.
2. Type+format coverage:
   1. per-type/per-format payload assertions for supported GSUB/GPOS forms,
   2. negative tests for malformed format/offset combinations.
3. Extension behavior:
   1. GSUB type 7 and GPOS type 9 resolve to expected effective payload type,
   2. depth guard remains enforced.
4. Concurrency:
   1. repeated concurrent access to same lookup/subtable index returns canonical cached object.
5. Runtime behavior goldens (`otlayout`):
   1. concrete-first GPOS application mutates `PosBuffer` consistently with concrete payload semantics for representative type 1/2/4/5/8 cases.

#### Completion criteria for Phase 2
1. Concrete lookup graph is parser-integrated and publicly reachable in `ot` in parallel with legacy path.
2. Concrete payload model contains no generic `any`/`VarArray` fields in the new API path.
3. Existing legacy tests remain green, plus new parity/concurrency tests for concrete path pass.
4. Known legacy cache-loss bug is fixed or isolated with explicit stabilization coverage.

### Lookup-Type Behavior Matrix (for concrete payload/API design)

Flag legend:
1. `RTL` = `RIGHT_TO_LEFT`
2. `IB/IL/IM` = `IGNORE_BASE / IGNORE_LIGATURES / IGNORE_MARKS`
3. `MFS/MAT` = `USE_MARK_FILTERING_SET / MARK_ATTACHMENT_TYPE_MASK`

Global flag notes:
1. `IB/IL/IM` are generally meaningful for matching/traversal behavior.
2. `MFS/MAT` are most relevant where marks participate in matching/attachment.
3. `RTL` is meaningful for GPOS type 3 and otherwise typically ignored.

#### GSUB lookup types
| Type | Coverage/context needed | ClassDef use | Buffer operation | Glyph count delta | Forwards to other lookups | Flag notes |
|---|---|---|---|---|---|---|
| 1 Single | coverage of input glyphs | no | replace 1 glyph with 1 glyph | 0 | no | IB/IL/IM |
| 2 Multiple | coverage of input glyphs | no | replace 1 glyph with sequence | `+n-1` | no | IB/IL/IM |
| 3 Alternate | coverage of input glyphs | no | replace 1 glyph with selected alternative | 0 | no | IB/IL/IM |
| 4 Ligature | coverage for first component | no | replace sequence with 1 ligature glyph | `-(n-1)` | no | IB/IL/IM |
| 5 Contextual | coverage + rules (glyph/class/coverage by format) | format 2 | conditional substitutions on matched input span | variable | yes (SequenceLookupRecords) | IB/IL/IM, MFS/MAT as needed |
| 6 Chaining Contextual | input + backtrack + lookahead context | format 2 | conditional substitutions with backtrack/lookahead | variable | yes (SequenceLookupRecords) | IB/IL/IM, MFS/MAT as needed |
| 7 Extension | none directly; wraps another lookup subtable | inherited | delegate | inherited | yes (to underlying type) | inherited |
| 8 Reverse Chaining Single | input coverage + backtrack/lookahead coverages | no | replace 1 glyph in reverse-chaining context | 0 | no | IB/IL/IM |

#### GPOS lookup types
| Type | Coverage/context needed | ClassDef use | Buffer operation | Glyph count delta | Forwards to other lookups | Flag notes |
|---|---|---|---|---|---|---|
| 1 Single Adjustment | coverage of adjusted glyphs | no | apply value record to one glyph | 0 | no | IB/IL/IM |
| 2 Pair Adjustment | coverage of first glyph + pair/class data | format 2 | adjust pair positioning/advance | 0 | no | IB/IL/IM |
| 3 Cursive Attachment | coverage of cursive glyphs + entry/exit anchors | no | connect adjacent cursive glyphs | 0 | no | `RTL` meaningful here |
| 4 MarkToBase | mark coverage + base coverage + anchors | no (mark classes, not ClassDef table) | attach mark to base anchor | 0 | no | IM/MFS/MAT important |
| 5 MarkToLigature | mark coverage + ligature coverage + anchors | no (mark classes, not ClassDef table) | attach mark to ligature component anchor | 0 | no | IM/MFS/MAT important |
| 6 MarkToMark | mark1 coverage + mark2 coverage + anchors | no (mark classes, not ClassDef table) | attach mark to mark anchor | 0 | no | IM/MFS/MAT important |
| 7 Contextual Positioning | coverage + rules (glyph/class/coverage by format) | format 2 | conditional positioning on matched span | 0 | yes (SequenceLookupRecords) | IB/IL/IM, MFS/MAT as needed |
| 8 Chaining Contextual Positioning | input + backtrack + lookahead context | format 2 | conditional positioning with backtrack/lookahead | 0 | yes (SequenceLookupRecords) | IB/IL/IM, MFS/MAT as needed |
| 9 Extension | none directly; wraps another lookup subtable | inherited | delegate | 0 | yes (to underlying type) | inherited |

#### Consequences for Coverage/ClassDef abstraction
1. `Coverage` can remain format-hidden as long as semantic API provides `Match(glyph) -> (coverageIndex, ok)`.
2. `ClassDef` can remain format-hidden as long as semantic API provides `Lookup(glyph) -> classID` with class `0` as default/unmapped.
3. `otlayout` does not need raw Coverage/ClassDef format knowledge for shaping logic; typed lookup payload semantics are the high-value refactor target.

### Minimum Payload API Surface (separate design table)
This section defines the minimum semantic payload contract needed by `otlayout`, independent of on-disk format layout.

#### Shared helper payload API
| Helper | Minimum API surface | Notes |
|---|---|---|
| `CoverageRef` | `Match(g GlyphIndex) (int, bool)` | hides coverage format 1/2 |
| `ClassDefRef` | `Lookup(g GlyphIndex) int` | hides classdef format 1/2 |
| `SequenceLookupRecord` | `SequenceIndex`, `LookupListIndex` | nested lookup application |
| `SequenceRule` | `InputGlyphs []GlyphIndex`, `Records []SequenceLookupRecord` | contextual glyph form |
| `ClassSequenceRule` | `InputClasses []uint16`, `Records []SequenceLookupRecord` | contextual class form |
| `ChainedSequenceRule` | `Backtrack []GlyphIndex`, `Input []GlyphIndex`, `Lookahead []GlyphIndex`, `Records []SequenceLookupRecord` | chaining glyph form |
| `ChainedClassRule` | `Backtrack []uint16`, `Input []uint16`, `Lookahead []uint16`, `Records []SequenceLookupRecord` | chaining class form |

#### GSUB minimum payload API
| GSUB type | Minimum semantic payload API surface | Notes for `otlayout` |
|---|---|---|
| 1 Single | `DeltaGlyphID() (int16, bool)` (fmt1), `SubstituteByCoverage(inx int) (GlyphIndex, bool)` (fmt2) | one-in one-out substitution |
| 2 Multiple | `SequenceByCoverage(inx int) []GlyphIndex` | one-in many-out |
| 3 Alternate | `AlternatesByCoverage(inx int) []GlyphIndex` | user/feature choice among alternates |
| 4 Ligature | `LigaturesByCoverage(inx int) []LigatureRule` where `LigatureRule{Components []GlyphIndex, Ligature GlyphIndex}` | first component selected by coverage |
| 5 Contextual | fmt1: `RulesByCoverage(inx int) []SequenceRule`; fmt2: `ClassDef() ClassDefRef`, `RulesByFirstClass(class int) []ClassSequenceRule`; fmt3: `InputCoverages() []CoverageRef`, `Records() []SequenceLookupRecord` | no raw `VarArray` in API |
| 6 Chaining Contextual | fmt1: `RulesByCoverage(inx int) []ChainedSequenceRule`; fmt2: `BacktrackClassDef()`, `InputClassDef()`, `LookaheadClassDef()`, `RulesByFirstClass(class int) []ChainedClassRule`; fmt3: `BacktrackCoverages()`, `InputCoverages()`, `LookaheadCoverages()`, `Records()` | includes backtrack/lookahead |
| 7 Extension | `ResolvedType() LayoutTableLookupType`, `ResolvedSubtable() *LookupNode` | unwrap indirection, preserve diagnostics metadata |
| 8 Reverse Chaining Single | `BacktrackCoverages() []CoverageRef`, `LookaheadCoverages() []CoverageRef`, `SubstituteByCoverage(inx int) (GlyphIndex, bool)` | replacement is indexed by input coverage index |

#### GPOS minimum payload API
| GPOS type | Minimum semantic payload API surface | Notes for `otlayout` |
|---|---|---|
| 1 Single Adjustment | `ValueByCoverage(inx int) (ValueRecord, ValueFormat, bool)` | fmt1 shared value, fmt2 per-coverage value |
| 2 Pair Adjustment | fmt1: `PairSetByCoverage(inx int) []PairValueRecord`; fmt2: `ClassDef1()`, `ClassDef2()`, `ClassValue(c1, c2 int) (ValueRecord, ValueRecord, bool)` | supports glyph-pair and class-pair positioning |
| 3 Cursive Attachment | `EntryExitByCoverage(inx int) (entry Anchor, exit Anchor, ok bool)` | connection logic uses neighboring glyph selection + lookup flag semantics |
| 4 MarkToBase | `MarkRecordByCoverage(inx int) (markClass uint16, markAnchor Anchor, ok bool)`, `BaseAnchor(baseCoverageInx int, markClass uint16) (Anchor, bool)`, `BaseCoverage() CoverageRef` | mark attachment |
| 5 MarkToLigature | `MarkRecordByCoverage(inx int) (markClass uint16, markAnchor Anchor, ok bool)`, `LigatureAnchors(ligCoverageInx int, componentInx int, markClass uint16) (Anchor, bool)`, `LigatureCoverage() CoverageRef` | mark to ligature component attachment |
| 6 MarkToMark | `Mark1RecordByCoverage(inx int) (markClass uint16, markAnchor Anchor, ok bool)`, `Mark2Anchor(mark2CoverageInx int, markClass uint16) (Anchor, bool)`, `Mark2Coverage() CoverageRef` | mark to mark attachment |
| 7 Contextual Positioning | same semantic contract as GSUB-5 contextual payloads | output is positioning via nested lookups |
| 8 Chaining Contextual Positioning | same semantic contract as GSUB-6 chaining payloads | output is positioning via nested lookups |
| 9 Extension | `ResolvedType() LayoutTableLookupType`, `ResolvedSubtable() *LookupNode` | unwrap indirection, preserve diagnostics metadata |

## Public APIs, Interfaces, and Type Changes
1. Additions:
   1. New concrete types for shared layout graph and lookup payloads.
   2. Constructors/accessors returning concrete structures directly.
2. Transitional:
   1. Existing interface-based accessors remain but are adapter-backed.
3. Future removals:
   1. `LookupSubtable` generic payload fields (`Index`, `Support`) from public-facing flows.
   2. Interface-first traversal as primary API for GSUB/GPOS internals.

## Test Cases and Scenarios
1. Parse invariants:
   1. Script/Feature/Lookup counts remain unchanged on baseline fonts.
   2. Lookup flag and mark-filtering metadata remain preserved.
2. Concrete typing:
   1. For each covered GSUB/GPOS type+format in tests, expected concrete struct is populated.
   2. New API path avoids runtime type assertions on `any`.
3. Extension handling:
   1. GSUB type 7 and GPOS type 9 still unwrap correctly.
   2. Recursion/depth guard behavior unchanged.
4. Compatibility parity:
   1. Adapter-backed legacy outputs match concrete model outputs on sampled fonts.
   2. Existing `ot/` tests continue to pass during transition.

## Risks and Mitigations
1. Risk: Breaking downstream expectations tied to `Navigator` chains.
   1. Mitigation: Keep transitional adapters and parity tests.
2. Risk: Missing subtle lookup-format semantics when de-genericizing.
   1. Mitigation: Per-type/per-format concrete tests plus baseline font comparisons.
3. Risk: Increased code duplication between GSUB and GPOS.
   1. Mitigation: Share common graph structures and helper records; split only lookup-specific payloads.

## Acceptance Criteria
1. `ot/` exposes concrete GSUB/GPOS layout structures for shared graph and lookup payloads.
2. Existing `ot/` behavior remains stable under transitional adapters.
3. Parsing and extension handling invariants are unchanged.
4. Tests validate concrete typing and backward-compat parity.

## Assumptions and Defaults
1. Use a transitional compatibility strategy (not a hard break in one step).
2. This document focuses on `ot/` only.
3. Downstream package migration is intentionally deferred to separate documents/tasks.

## Filtered Legacy-Navigation Cleanup Matrix (Code Only)
Scope of this matrix:
1. Include code in `ot/`, `otlayout/`, `otquery/`, and their tests.
2. Exclude documentation references, examples, and `otcli/`.
3. Keep `Table.Fields()` / `tableBase.Fields()` decision deferred for now.

### Production code matrix
| Area | Location | Legacy dependency | Decision | Planned action |
|---|---|---|---|---|
| Generic table fields API | `ot/ot.go` (`Table.Fields`, `tableBase.Fields`) | `Fields() Navigator` and `NavigatorFactory` routing | defer | Keep unchanged for now; revisit after layout migration settles (`head`, `bhea`, `OS/2` use case). |
| Layout table surface | `ot/layout.go` (`LayoutTable.ScriptList`, `LayoutTable.FeatureList`) | legacy `Navigator`/`TagRecordMap` fields in parallel with concrete graphs | migrate | Remove legacy fields after all non-test callsites are concrete-only. |
| BASE internals | `ot/layout.go` (`AxisTable.baseScriptRecords`) | `TagRecordMap` field | keep (out of GSUB/GPOS scope) | Leave unchanged in current refactor scope. |
| Script parser wiring | `ot/parse.go` (`lytt.ScriptList = NavigatorFactory(...)`) | legacy script traversal population | drop | Stop assigning legacy `ScriptList`; keep concrete `scriptGraph` only. |
| Feature parser wiring | `ot/parse.go` (`lytt.FeatureList = ...`) | legacy feature-map population | drop | Stop assigning legacy `FeatureList`; keep concrete `featureGraph` only. |
| GSUB/GPOS structural validation | `ot/parse.go` (checks on `ScriptList.IsVoid`, `FeatureList.Len`) | validation through legacy fields | migrate | Validate against concrete graph accessors and concrete error state only. |
| Legacy layout wrappers | `ot/layout.go` (`langSys`, `feature` navigator wrappers) | wrapper types implementing `Navigator` | drop | Delete wrapper types and their nav methods after parser cutover. |
| Legacy lookup list nav coupling | `ot/layout.go` (`LookupList.Subset` as `RootList`, interface asserts) | `NavList`/`RootList` conformance | migrate/drop | Remove nav-interface conformance and keep concrete lookup graph/list API only. |
| Legacy lookup map coupling | `ot/layout.go` (`Lookup.LookupTag`, `IsTagRecordMap`, `AsTagRecordMap`, assert `NavMap`) | `NavMap` compatibility on `Lookup` | drop | Delete compatibility methods/asserts once no callsites remain. |
| Navigation interface hub | `ot/factory.go` (`Navigator`, `NavList`, `NavMap`, `TagRecordMap`, `Root*`) | public legacy navigation interfaces | migrate | Shrink to minimal non-layout needs or remove entirely after dependent APIs migrate. |
| Factory routing | `ot/factory.go` (`NavigatorFactory` cases for `ScriptList`/`Script`/`LangSys`/`Feature`) | legacy GSUB/GPOS object materialization | drop (layout cases) | Remove layout cases once parser and consumers are concrete-only. |
| Link->Navigator coupling | `ot/bytes.go` (`NavLink.Navigate`, `link16/32.Navigate`) | navigation dispatch via links | migrate | Decouple layout traversal from `Navigate`; keep only non-layout/deferred paths as needed. |
| Tag-record legacy helpers | `ot/bytes.go` (`IsTagRecordMap`, `AsTagRecordMap`, `Subset` returning `RootTagMap`) | nav-era map polymorphism | migrate/drop | Remove when all production code paths stop requiring nav map polymorphism. |
| `otlayout` feature API | `otlayout/feature.go` (`Feature.Params() ot.Navigator`) | legacy method in public interface | drop/migrate | Remove method or replace with concrete/opaque params API. |
| `otquery` name-table read path | `otquery/info.go` (`AsNameRecords(table.Fields())`, `link.Navigate().Name()`) | dependency on nav bridge for `name` | migrate later | Replace with lightweight direct `name` reader API (no nav chain). |
| Name bridge types | `ot/factory.go` (`NameRecords`, `AsNameRecords`) and `ot/ot.go` (`nameNames` returning `NavLink`) | transitional nav-based `name` API | migrate later | Remove after `otquery` and other consumers switch to direct `name` API. |

### Test code matrix
| File | Legacy dependency | Decision | Planned action |
|---|---|---|---|
| `ot/nav_test.go` | direct Navigator-chain behavior and `AsNameRecords(table.Fields())` | migrate/split | Keep only deferred `Fields()`-related tests; remove GSUB/GPOS nav-chain assertions. |
| `ot/parse_test.go` | `assertScriptGraphParity` and legacy traversal checks | migrate | Convert to concrete-only assertions for script/feature/lookup graphs. |
| `ot/parse_refactor_parity_test.go` | legacy-vs-concrete parity via `LookupList.Navigate` | drop | Remove after legacy lookup adapter track is retired. |
| `ot/refactor_lookup_legacy_adapter_test.go` | adapter projection from concrete to legacy | drop | Remove with adapter deletion. |
| `ot/lookup_list_subset_test.go` | `RootList` subset contract | migrate/drop | Replace with concrete lookup selection tests or remove if no public subset API remains. |
| `ot/ttx_gsub_compare_test.go` | legacy lookup traversal via `Navigate` | migrate | Repoint to concrete lookup graph traversal. |
| `ot/ttx_gpos_compare_test.go` | legacy lookup traversal via `Navigate` | migrate | Repoint to concrete lookup graph traversal. |
| `ot/parse_gsub_type8_test.go` | legacy lookup traversal via `Navigate` | migrate | Repoint to concrete lookup graph traversal. |
| `otlayout/feature_functional_test.go` | fixture implements `Params() ot.Navigator` | migrate | Update fixture/API once `Feature.Params` is removed/replaced. |

## Ordered Cleanup Checklist (Code Only)
This checklist follows a low-risk cut order: remove production dependencies first, then delete legacy API scaffolding, then retire test scaffolding.

### Stage 0: Guardrails and scope lock
- [ ] Freeze scope: no changes to docs/examples/`otcli` in this track.
- [ ] Keep `Table.Fields()` / `tableBase.Fields()` marked deferred.
- [ ] Capture baseline green runs for `go test ./ot ./otlayout ./otquery`.

### Stage 1: Remove remaining production callsites to layout navigation
- [ ] Remove/replace `otlayout.Feature.Params() ot.Navigator` and all non-test callsites.
- [ ] Switch `ot/parse.go` GSUB/GPOS structure validation to concrete graph checks only.
- [ ] Stop populating `LayoutTable.ScriptList` and `LayoutTable.FeatureList` in parser wiring.
- [ ] Ensure `otquery` remains green with current deferred `name` bridge.

### Stage 2: Remove legacy GSUB/GPOS fields and wrappers in `ot/layout.go`
- [ ] Delete `LayoutTable.ScriptList` and `LayoutTable.FeatureList` fields.
- [ ] Delete legacy `langSys` and `feature` navigator wrapper types.
- [ ] Delete `Lookup` nav-compat methods (`LookupTag`, `IsTagRecordMap`, `AsTagRecordMap`) if no callsites remain.
- [ ] Remove `LookupList` nav-interface conformance (`RootList`/`NavList`) where no longer needed.

### Stage 3: Shrink/retire navigation factory paths for layout
- [ ] Remove `NavigatorFactory` routing cases for `ScriptList`/`Script`/`LangSys`/`Feature`.
- [ ] Remove layout-only link navigation dependencies on `NavLink.Navigate()`.
- [ ] Keep only deferred/non-layout paths required by `Fields()` and current `name` handling.

### Stage 4: Retire nav-era map/list polymorphism used only by layout
- [ ] Remove `tagRecordMap16` nav-polymorphic helpers not needed outside deferred/non-layout use.
- [ ] Remove unused nav interface fragments from `factory.go` once compilation proves no dependents.
- [ ] Re-run full package tests and static grep for leftover GSUB/GPOS nav calls.

### Stage 5: Test migration and deletion
- [ ] Convert remaining parity tests to concrete-only behavior checks.
- [ ] Delete legacy adapter tests tied to removed code paths.
- [ ] Keep/adjust only tests that exercise deferred `Fields()` behavior.

### Stage 6: Deferred follow-up track (`name` + `Fields`)
- [ ] Design lightweight direct `name` table API (raw-backed, on-demand).
- [ ] Migrate `otquery.NameInfo` from `AsNameRecords(table.Fields())` to direct API.
- [ ] Re-evaluate `Table.Fields()` retention/removal after `name` migration.
