# Feature Application (Batch Mode)

This document summarizes the OpenType feature application flow (GSUB/GPOS) for
batch‑mode lookup application, based on the OpenType spec.

## Summary

Key points from the spec (paraphrased):

- A Feature table lists lookup indices (unordered) that implement the feature.
  These indices refer into the LookupList.
- When a client applies a feature, it processes the lookups referenced by that
  feature in lookup list order (ascending lookup index), regardless of how they
  are listed in the Feature table.
- A client may process multiple features simultaneously. In that case, it
  processes the union of lookups referenced by those features, again in lookup
  list order, and each lookup is applied only to the glyph subsequence(s) where
  its feature(s) are active.
- For contextual/chaining lookups (GSUB 5/6/8, GPOS 7/8), the main lookup only
  matches; nested lookups do the actual substitutions/positioning, and their own
  lookup flags apply.

## Practical Procedure

1. Select Script/LangSys (default vs specific) and choose the feature set to
   apply (including user toggles).
2. For variable fonts, substitute feature tables via FeatureVariations (if
   applicable).
3. Collect all lookup indices referenced by those feature tables.
4. De‑duplicate and sort ascending by lookup index (lookup list order).
5. For each lookup in that order, apply it across the relevant glyph
   subsequence(s) for the feature(s) that referenced it.

## Notes vs Common Misconceptions

- Feature table ordering is irrelevant; lookup list order governs processing.
- You don’t apply all lookups on each position in the input unconditionally;
  each lookup runs only where its feature(s) are active.

## Clarification: “Active Feature” vs “Matched Lookup”

There are two separate gating layers:

1. **Feature selection (eligibility):**  
   A lookup only participates if at least one of its owning features is enabled
   for the current shaping run (script/langsys + user toggles). If a feature is
   disabled, its lookups are skipped entirely.

2. **Lookup matching (actual application):**  
   For an *active* lookup, the lookup still applies only where its own rules
   match (coverage/class/context/chaining, etc.). If the lookup doesn’t match
   at a given glyph position, it has no effect there.

In the common case where script/langsys and feature set are fixed for the
entire run, “feature active” effectively means “eligible for the whole run,”
and the only remaining gating is the lookup’s matching logic.

## Proposed API Split (otshape vs otlayout)

**Separation of concerns**

- `otshape/` should own the *top‑down* shaping API: script/lang selection,
  feature selection (user + defaults), run segmentation (script/language
  changes), and text→glyph mapping (cmap, variation selectors, etc.). It produces
  glyphs + positioning.
- `otlayout/` should own the *bottom‑up* lookup application: apply a prepared
  lookup sequence to an existing glyph buffer + pos buffer.

This keeps `otlayout` reusable for non‑text inputs (e.g. a glyph stream) and
avoids duplicating higher‑level shaping logic.

## Candidate Function Signatures

### 1) Top‑down (otshape)

Streaming interface: read runes, shape into a buffer, emit glyphs+pos.

```go
func Shape(r runeio.Reader, dst *BufferState, sink GlyphsSink, opt ShapeOptions) error
```

Output should be a glyph+positioning stream:

```go
// GlyphsSink receives shaped glyphs with positioning.
type GlyphsSink interface {
    // WriteGlyphPos receives one glyph with its positioning info.
    // cluster is optional but recommended for text->glyph mapping.
    WriteGlyphPos(gid ot.GlyphIndex, pos PosItem, cluster int) error
}
```

`ShapeOptions` should capture script/lang overrides, feature toggles, direction,
and future variation axes.

### 2) Bottom‑up (otlayout)

Apply a lookup list (in lookup‑list order) to an existing buffer.

```go
func ApplyLookups(
    lookups []ot.Lookup, // or []LookupRef
    buf *BufferState,
    opt ApplyOptions,
) error
```

`ApplyOptions` can capture direction, debug/trace flags, and any additional
context needed by lookup handlers.

## Streaming Design Notes

- `otshape` reads runes in chunks (or by script run), maps to glyphs, then calls
  `otlayout.ApplyLookups(...)` on each run.
- Clients may pass a pre‑allocated `BufferState` (or use a pool) to reduce
  allocations.

## Open Questions (to resolve)

1. Should `BufferState` carry cluster info (rune→glyph mapping), or should that
   remain inside `otshape` only?
2. `ApplyLookups` should accept `[]Lookup` already resolved (decision).
3. Where should directionality live: as a `BufferState` field or in
   `ApplyOptions`?

## Data Flow Diagram (Sketch)

```text
           +--------------------+
           |   runeio.Reader    |
           +---------+----------+
                     |
                     v
        +------------+-------------+
        |   otshape: script/lang   |
        |   + feature selection    |
        +------------+-------------+
                     |
                     v
        +------------+-------------+
        |  cmap / text->glyph map  |
        +------------+-------------+
                     |
                     v
        +------------+-------------+
        |   BufferState (glyphs)   |
        |   + PosBuffer (empty)    |
        +------------+-------------+
                     |
                     v
        +------------+-------------+
        | otlayout.ApplyLookups    |
        | (lookup-list order)      |
        +------------+-------------+
                     |
                     v
        +------------+-------------+
        | BufferState (glyphs+pos) |
        +------------+-------------+
                     |
                     v
        +------------+-------------+
        |   glyph stream / output  |
        +--------------------------+
```
