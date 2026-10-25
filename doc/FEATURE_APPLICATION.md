# Feature Application (Batch Mode)

This document summarizes the OpenType feature application flow (GSUB/GPOS) for
batch‑mode lookup application, based on the OpenType spec.

## Lookup result and error contract

Status: stages 1 through 6 complete for the bounded format-1 change. `ApplyFeature` returns an independent error and
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

Completed edits now synchronize layout buffers, nested input mappings, and shaping
side arrays on both success and error, including isolated syllable spans.
Glyph-count validation for other substitution formats remains outside this bounded
fix; the implementation is not a comprehensive malformed-data validator.

### Result semantics

GSUB and GPOS application retain the `applied` boolean and return an independent
`error` result. `applied` means that a lookup operation was applied, not
that a value changed or that processing completed without errors. A valid
zero-delta substitution can return `applied == true`.

| Outcome | applied | err |
| --- | --- | --- |
| Normal coverage/context non-match | false | nil |
| Successful application, including a value-preserving operation | true | nil |
| Failure before any operation applied | false | descriptive error |
| Failure after earlier operations applied | true | descriptive error |

Composite operations accumulate `applied` across their completed children. A nil
feature for an unused mandatory-feature slot remains a normal no-op, including
when the buffer state is nil (position zero is returned in that case). Invalid required arguments and malformed lookup data must not be
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
  payloads, invalid lookup references, and detected invalid output glyphs are
  failures. Output glyph-count validation currently covers GSUB 1 format 1,
  not every substitution handler or malformed coverage/payload relationship.
- Describe the failing operation locally and wrap with feature, lookup, and
  subtable context where known, preserving causes with `%w`. Exact message text
  is not an API contract. Runtime errors are returned, not added to parse-time
  font diagnostics or silently replaced by logging.

### Stage 5 implementation checkpoint

All three bounded passes are implemented. See [Partial Progress Audit](PARTIAL_PROGRESS.md)
for ownership, metadata policies, and regression coverage. The public shaping APIs
stream to a sink: a failed chunk is not emitted, but already emitted prefixes
cannot be retracted. Whole-call atomic output is not promised.

### Bounded contextual execution

Every ApplyFeature invocation gets independent execution state shared by its
GSUB/GPOS nested calls. At most 64 lookup invocations may be active simultaneously,
including the top-level lookup. A separate budget of 65536 work units counts
lookup entries and contextual sequence-record visits, including non-matches and
records skipped because an earlier edit deleted their target.

Depth exhaustion stops cycles and excessively deep valid recursion; the work
budget also stops broad acyclic expansion of nested calls. These are fixed
implementation safeguards, not OpenType validity rules. There is no permanent
cycle blacklist: a recursive lookup that terminates within the limits is allowed.

Exhaustion returns an error wrapping otlayout.ErrLookupLimit (use errors.Is).
Normal contextual wrapping retains feature, lookup, subtable, and record details.
Earlier edits and positioning remain applied; shaping follows the existing
partial-progress/sink policy. State is per call, not cached on a font, and is
unwound on all exits. Independent buffers can shape the same font concurrently.

This is not a complete time or memory bound: parser work, matching scans, glyph
allocation sizes, and the number of separate ApplyFeature calls in a streaming
run are not charged by this budget. No public tuning API is introduced.

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
- For contextual/chaining lookups (GSUB 5/6, GPOS 7/8), the main lookup only
  matches; nested lookups do the actual substitutions/positioning, and their own
  lookup flags apply.
  GSUB 8 performs its reverse-chaining substitution directly.

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

## Current API Responsibilities

`otshape` owns text mapping, script/language and feature selection, segmentation,
metadata, and sink delivery. Its public entry points are:

```go
func (s *Shaper) Shape(params Params, src RuneSource, sink GlyphSink, bufOpts BufferOptions) error
func (s *Shaper) ShapeEvents(params Params, src InputEventSource, sink GlyphSink, bufOpts BufferOptions) error
```

`GlyphSink.WriteGlyph(GlyphRecord) error` receives glyph ID, positioning, cluster,
mask, and unsafe flags. Clusters and other shaping metadata remain in `otshape`;
`otlayout.BufferState` owns layout glyphs/positions and its edit journal.
The internal shaping executor calls `otlayout.ApplyFeature`, consumes completed
edits once to update metadata, and handles isolated-span integration. There is no
public `ApplyLookups` API matching the earlier design sketch.

See [Partial Progress](PARTIAL_PROGRESS.md) for ownership and regression coverage,
[Shaping Benchmarks](SHAPING_BENCHMARKS.md) for the measured performance baseline,
and [Code Review](CODE_REVIEW.md) for unresolved work.
