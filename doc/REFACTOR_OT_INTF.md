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

### Known bug: cache lost due to value receivers
1. The code intends to cache lazy parse results (`lookupsCache`, `subTablesCache`).
2. `LookupList.Navigate` and `Lookup.Subtable` currently use value receivers.
3. Cache writes therefore happen on copies, so cached instances are not reliably reused across calls.
4. This is a bug (not an intentional tradeoff): lazy parsing currently degenerates into repeated parsing in common call patterns.
5. Required fix: move cache ownership to stable instances (for example pointer receivers and/or explicit owner objects) and define cache lifetime semantics consistent with thread-safe read access and deferred pointer-identity decision.

## Implementation Phases
1. Phase 1: Introduce concrete shared layout graph types in parallel with existing interfaces.
2. Phase 2: Parse GSUB/GPOS into concrete shared graph + concrete lookup payload types.
3. Phase 3: Add compatibility adapters from concrete model back to legacy interfaces.
4. Phase 4: Convert internal `ot/` consumers to concrete types.
5. Phase 5: Deprecate and later remove legacy interface-heavy surface.

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
