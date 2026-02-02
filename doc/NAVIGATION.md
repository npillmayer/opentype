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

## Pointers to the relevant code

- `ot/bytes.go`: `NavLink` interface, `parseLink16`/`makeLink16`, `u16List`.
- `ot/layout.go`: `LangSys.List()` returns `u16List` of feature indices.
- `ot/factory.go`: `NavigatorFactory` wiring and link creation for `Script` and
  `LangSys`.
