# Feature Application (Batch Mode)

This document summarizes the OpenType feature application flow (GSUB/GPOS) for
batch‑mode lookup application, based on the OpenType spec.

## Lookup result and error contract

Status: stages 2 through 4 implemented. `ApplyFeature` returns an independent error and
rejects missing or invalid `maxp.NumGlyphs` before mutation. A read-only glyph-count
snapshot is passed by value through direct and nested GSUB/GPOS lookups, separately
from the mutable buffer state. Its valid range is 1 through 65535; no unbounded
fallback is used. GSUB/GPOS leaves, dispatchers, and contextual helpers now carry
errors to the public API and shaping caller. Lazy graph/table/node errors (including
resolved extension errors), missing required payloads, unsupported lookup formats,
and invalid lookup/sequence references stop application rather than becoming
non-matches. Wrapping preserves causes for `errors.Is` / `errors.As`.

Earlier applied operations retain their buffers and applied flag when a later
operation fails. GSUB single substitution format 1 now computes the output modulo
65536 and checks it against `maxp.NumGlyphs` before mutation. An out-of-range output
returns a descriptive error, no application, and no edit; it does not trigger
subtable fallback. Zero delta and a valid result of glyph zero remain successful
applications. This check also applies inside contextual and extension lookups.

Full partial-progress bookkeeping, including shaping side-array synchronization
(stage 5), remains outstanding. Glyph-count validation for other substitution
formats is outside this bounded fix. Existing leaf bounds checks are not yet a complete
malformed-data audit. The full contract below is therefore still a target,
not a guarantee of the current code.

### Result semantics

Keep the `applied` boolean and add an independent `error` result throughout GSUB
and GPOS application. `applied` means that a lookup operation was applied, not
that a value changed or that processing completed without errors. A valid
zero-delta substitution can return `applied == true`.

| Outcome | applied | err |
| --- | --- | --- |
| Normal coverage/context non-match | false | nil |
| Successful application, including a value-preserving operation | true | nil |
| Failure before any operation applied | false | descriptive error |
| Failure after earlier operations applied | true | descriptive error |

Composite operations accumulate `applied` across their completed children. A nil
feature for an unused mandatory-feature slot remains a normal no-op with a valid
buffer state. Invalid required arguments and malformed lookup data must not be
reported as ordinary non-matches.

The public signature is:

```go
func ApplyFeature(otf *ot.Font, feat Feature, st *BufferState, alt int) (
    pos int, applied bool, err error,
)
```

Leaf GSUB/GPOS functions retain position, applied flag, glyph buffer, and edit
metadata, and add a final error result. Dispatchers and contextual helpers carry
that error through to `ApplyFeature` and the shaping caller.

### Dispatch and diagnostics

- After a normal non-match, dispatch may try the next subtable.
- After successful application, dispatch stops searching that lookup's subtables.
- After an error, stop the current application and propagate it; do not try an
  alternative subtable or advance the shaping cursor as though this were a miss.
- Check errors on lazy lookup nodes before using their payloads. Missing required
  payloads, invalid lookup references, and invalid output glyphs are failures.
- Describe the failing operation locally and wrap with feature, lookup, and
  subtable context where known, preserving causes with `%w`. Exact message text
  is not an API contract. Runtime errors are returned, not added to parse-time
  font diagnostics or silently replaced by logging.

### Stage 5 implementation checkpoint

The second bounded pass fixes low-level edit ownership, nested edit transport,
and shared-buffer isolation. Low-level regression tests pass; the shaping-boundary
test remains intentionally failing until pass 3. See [Partial Progress Audit](PARTIAL_PROGRESS.md)
for the confirmed defects, edit-ownership proposal, and next implementation passes.
The public shaping APIs stream to a sink: a failed chunk must not be emitted, but
already emitted prefixes cannot be retracted. Whole-call atomic output is not promised.

### Mutation and partial progress

Validate a leaf operation before mutating glyphs, positions, or its cursor. A
failed single substitution leaves its input state unchanged and reports no edit.
Do not substitute `.notdef` automatically for an invalid GSUB output.

Application is not transactional. If a later lookup or nested record fails,
earlier successful operations remain applied. Return the actual current glyph
and position buffers and the accumulated applied flag, together with the error.
The returned position must agree with the buffer state's cursor at the stopping
point; it is not a promise that shaping can safely resume after the error.

Preserve completed edit bookkeeping, including position-buffer alignment and
contextual index remapping. Apply each edit exactly once. Shaping callers must
retain current slice headers and synchronize side arrays before returning an
error; they must not expose fabricated records or stale metadata. A nil edit
does not imply no application: positioning and composite operations can apply
without returning a single replacement span.

Callers must check `err` even when `applied` is true. They may discard an incomplete
run, but must not assume that an error restored the original state. Rollback and
automatic recovery are outside this change.

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
