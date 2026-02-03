# Navigation in package ot

## What I just found (notes from CLI/LangSys investigation)

- A `NavLink` is **not** created from a `NavList` in this codebase. Lists return
  raw `NavLocation` values; only maps or explicit helpers create `NavLink`s.
- `LangSys.List()` returns a `u16List` of **feature indices**, not offsets, so
  there is no link target/name to set for list entries.
- `u16List.Name()` is hardcoded to `"<unknown>"`, so list items from `LangSys`
  appear unnamed.
- `NavLink` creation happens via `parseLink16`/`makeLink16`, where the caller
  passes the destination name string (this is what `NavLink.Name()` returns).

Implication for the CLI: to navigate from a `LangSys` list entry to a `Feature`,
interpret the list value as an index into the FeatureList, then use the
FeatureList to obtain the actual link/record. There is no automatic list->link
conversion in `ot`.

## How Navigators are created in `ot`

- `Navigator` instances are created by `NavigatorFactory` when a `NavLink` is
  navigated (`NavLink.Navigate()`), or when `Table.Fields()` is called to start
  at a table root.
- A `TagRecordMap` (e.g., `FeatureList`) is **not** itself a `Navigator`. It is
  a map of tags to `NavLink`s.
- There is no prebuilt `Navigator` per FeatureList entry. A `Navigator` exists
  only after a `NavLink` to an entry is followed with `Navigate()`.

### FeatureList example

1. `FeatureList` is parsed as a `TagRecordMap` with target `"Feature"`.
2. `FeatureList.Get(i)` or `FeatureList.LookupTag(tag)` returns a `NavLink`.
3. `NavLink.Navigate()` yields a `Navigator` for the `"Feature"` table entry.

## Why ScriptList is a Navigator but FeatureList is not

- ScriptList/Script need both a **map** (script or langsys records) and a
  **default link** (Script → default LangSys). That requires a `Navigator`
  which can expose multiple facets (`Map()` and `Link()`).
- FeatureList is only map-like: tag → Feature link. There is no “default link”
  or list facet to expose, so a `TagRecordMap` is sufficient and lighter.

## Plan: RootTagMap subsetting (stored for later)

1. **Interfaces**
   - Remove `Subset(NavList)` from `TagRecordMap`.
   - Add `RootTagMap` interface:
     `type RootTagMap interface { TagRecordMap; Subset(indices []int) RootTagMap }`

2. **Implementation**
   - `tagRecordMap16` implements `Subset(indices []int) RootTagMap` with copy semantics
     (allocate new buffer, copy selected records, no index slice retained).
   - `mapWrapper` does **not** implement `RootTagMap`.

3. **FeatureSubsetForLangSys**
   - Change `FeatureSubsetForLangSys(langSys, featureList)` to:
     - Extract `[]int` indices from `langSys` using `langSys.Range()` and read
       U16 values from each `NavLocation`.
     - Assert `featureList` implements `RootTagMap`.
     - Call `RootTagMap.Subset(indices)`.

4. **Call sites**
   - Update any usages of `TagRecordMap.Subset(NavList)` (expected only
     `FeatureSubsetForLangSys`).

5. **Tests**
   - Update TagRecordMap subset tests to use `RootTagMap.Subset([]int)`.
   - Add/extend tests for `FeatureSubsetForLangSys` covering index extraction,
     order/duplication, and repeated subsetting.

## Updates applied after the plan

### RootTagMap + FeatureSubsetForLangSys

- `TagRecordMap.Subset(NavList)` has been removed.
- `RootTagMap` was introduced with `Subset([]int) RootTagMap`.
- `tagRecordMap16` implements `RootTagMap` using copy semantics (no index slice retained).
- `FeatureSubsetForLangSys` now:
  - extracts indices via `langSys.Range()` and `U16(0)`,
  - asserts `featureList` implements `RootTagMap`,
  - calls `RootTagMap.Subset(indices)`.

### NavList.Range + ListAll

- `NavList` gained `Range()` to iterate (`for i, loc := range l.Range()`).
- `NavList.All()` was removed.
- New helper: `otlayout.ListAll(l ot.NavList) []ot.NavLocation` (uses `Range()`).
- `otlayout/feature.go` now uses `ListAll(lsys.List())`.

### NavMap.LookupTag removal

- `NavMap.LookupTag` removed from the interface.
- All real call sites now cast via `IsTagRecordMap()/AsTagRecordMap()` before calling
  `TagRecordMap.LookupTag`.
- Examples/docs and otcli were updated to call `LookupTag` on `TagRecordMap` only.

## Pointers to the relevant code

- `ot/bytes.go`: `NavLink` interface, `parseLink16`/`makeLink16`, `u16List`.
- `ot/layout.go`: `LangSys.List()` returns `u16List` of feature indices.
- `ot/factory.go`: `NavigatorFactory` wiring and link creation for `Script` and
  `LangSys`.
