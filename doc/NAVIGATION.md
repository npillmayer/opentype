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

## Pointers to the relevant code

- `ot/bytes.go`: `NavLink` interface, `parseLink16`/`makeLink16`, `u16List`.
- `ot/layout.go`: `LangSys.List()` returns `u16List` of feature indices.
- `ot/factory.go`: `NavigatorFactory` wiring and link creation for `Script` and
  `LangSys`.
