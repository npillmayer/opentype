# Partial Progress: Stage 5 Ownership Audit

Status: second bounded pass implemented. Low-level partial-progress tests pass.
The shaping-boundary regression test remains intentionally failing until pass 3.

## Pass 2 Implementation

- BufferState owns an ordered internal edit journal. Set and ReplaceGlyphs record
  mutations; positions are changed only by their owning mutation methods.
- applyLookupConcrete no longer reapplies the position edit. A one-to-one Set
  preserves existing positioning rather than replacing it with default records.
- Contextual calls use the actual BufferState rather than constructing an owner
  from borrowed slices. Each contextual frame checkpoints the journal and remaps
  its inputs using all child edits, including nested composites.
- Contextual frames restore their enclosing cursor on exit; the enclosing lookup
  publishes its returned position. This retains the existing contextual cursor
  convention rather than introducing resumability after failure.
- EnsurePos detaches shared positioning before GPOS writes, even if lengths
  already match. CloneShared starts independent journal history.
- ApplyFeature starts a new journal before applying its lookup sequence. Completed
  edits remain available internally on error. No public API was added; exposing
  completed edits to the shaping package is part of pass 3.

Tests now cover expansion/contraction alignment, nested mapping, journal order,
preservation of suffix position values, contextual GSUB/GPOS copy-on-write, and
multiple edits with zero net length change. The audit below describes the
original defects; shaping synchronization and attachment-index policy remain
outstanding.

## Failure Policy

Option 3 remains the baseline: return application errors without persistently
disabling tables or lookups. Cached parsing failures do not change that policy.
No rollback, automatic retry, fallback-font selection, or new sanitizer is planned.

Low-level ApplyFeature owns no transaction: completed work remains in its
caller-supplied BufferState. On error it must return the accumulated applied flag,
current slice headers, and a position equal to BufferState.Index. That position
is diagnostic, not a safe resume token. A failing leaf should validate before
mutation; completed child operations of a composite are not rolled back.

An error must preserve cause and context. Earlier edits must update each consumer
exactly once. A nil aggregate EditSpan does not mean no edits occurred.

## Public Boundary: Streaming, Not an Atomic Result

Shape and ShapeEvents take a source and GlyphSink and return error, not a shaped
result object. Their workspace owns the in-progress run. Shape calls
shapeMappedRun before writing a flushable prefix. ShapeEvents similarly shapes
and assembles event segments before emitting the current chunk.

Contract for the next passes:
- A lookup error must prevent emission of the failing in-progress chunk.
- Previously emitted prefixes remain in the sink; the whole call is not atomic.
- A sink error can itself leave a partially written prefix.
- Inputs may already have been consumed. Retrying requires caller-retained input
  and a suitable sink; automatic replay is not promised.
- Internal partial buffers remain coherent for cleanup and diagnostics, but are
  not exposed as completed output.
- Callers requiring all-or-nothing delivery should buffer sink output and commit
  only after success.

This corrects the earlier shorthand "return no output on failure": it can apply
to the current chunk, not retroactively to the entire streaming call.

Relevant paths: otshape/shape_api.go (Shape, shapeMappedRun, sink writers);
otshape/shape_events.go (ShapeEvents, event-segment assembly).

## Original Ownership Audit (Pass 1)

| Layer | Current ownership/action | Gap / intended responsibility |
| --- | --- | --- |
| BufferState | Glyph/position slice headers; copy-on-write flags | Primary owner of layout mutations |
| ReplaceGlyphs | Replaces glyph range and already edits Pos | This position edit must not be repeated by its caller |
| applyLookupConcrete | Stores returned headers, calls ApplyEdit, sets Index | Reapplies ReplaceGlyphs' position edit; expansion from 2 to 4 glyphs produces 6 positions |
| Contextual leaves | Return child buffers, generally nil aggregate edit | Must expose completed child edits to enclosing consumers |
| applySequenceLookupRecords | Makes a child BufferState and remaps matched inputs using one returned edit | Nested composite edits disappear; later records can target inserted glyphs instead of the original matched input |
| applyLookupSpan | Advances cursor and realigns on successful length changes | Error return precedes synchronization; run headers and side arrays remain stale |
| applyLookups | Builds a BufferState from run slices; publishes headers after success | Early return bypasses publication, including reallocated position buffers |
| applyLookupIsolatedSpan | Copies a syllable into detached buffers, merges on success | On error completed edits are discarded; successful inner realignment also needs coordinate/ownership review |
| realignSideArrays | Resizes selected arrays and regenerates masks | Length alone cannot preserve interior edit correspondence; PlanIDs are not handled |
| runBuffer.ApplyEdit | Edits glyphs, positions, and side arrays | Cannot be replayed wholesale after layout already edited glyphs/positions |

Additional audit risks for follow-up tests:
- NewBufferState around borrowed child slices does not inherit copy-on-write
  ownership flags from an enclosing shared BufferState.
- EnsurePos does not detach an already aligned shared Pos buffer before GPOS
  writes to it.
- Contractions and multiple edits with zero net length change require edit-aware
  metadata handling, not only length comparisons.
- AttachTo and other index-bearing metadata need an explicit remapping policy;
  equal array lengths alone do not prove coherent attachment relationships.

These additional risks are inspection findings, not all reproduced by this pass.

## Internal Design and Remaining Integration

Use an ordered internal edit stream rather than forcing composites into one
EditSpan. Each entry describes coordinates immediately before that edit.
The stream must include completed edits on error and allow a caller to delimit
the edits produced by one child invocation.

Responsibilities:
1. BufferState mutation methods change glyphs and positions once.
2. Contextual frames consume child edits to update their own matched-input maps;
   they must not replay glyph or position mutations.
3. The shaping executor consumes edits to update side arrays and publishes current
   headers on both success and error.
4. An isolated span translates edit coordinates into parent coordinates and merges
   its completed state on both success and error.
5. The public streaming layer emits only successfully shaped chunks.

Pass 2 uses a BufferState journal with invocation checkpoints. Pass 3 must provide
a safe consumption mechanism across the package boundary. Avoid a general
transaction/rollback framework.

Side-array policies must be explicit: preserve unaffected suffix entries;
determine inheritance/defaults for inserted or merged clusters, masks, plan IDs,
syllables, joiners, and unsafe flags. Reuse existing policies where appropriate,
but do not assume simple resizing is sufficient.

## Regression Tests Added

Run:
    go test ./otlayout ./otshape -run TestPartialProgress -count=1

- TestPartialProgressExpansionEditsPositionsOnce: expansion followed by an invalid
  lookup must retain the expansion and have one position entry per glyph.
- TestPartialProgressNestedEditRemapsOuterInput: nested expansion, then an outer
  substitution, then failure. Coverage deliberately includes an inserted glyph
  so forward matching cannot conceal a stale target index.
- TestPartialProgressIsolatedSpanRetainsCompletedEdits/whole_span: an expansion
  followed by a nested-reference error must publish updated run headers and keep
  clusters and plan IDs aligned.
- The isolated_span variant requires those same completed changes to be merged
  back into the parent instead of discarded.

Fixtures are independently parsed and their cached nodes are replaced with small,
controlled lookup graphs. No real font files are modified. Expansion inputs have
insufficient capacity, exercising changed slice headers.

## Remaining Passes

Pass 2 is complete for the low-level scope described above.

Pass 3: synchronize shaping side arrays on all exits, merge isolated spans with
coordinate translation, and make shaping tests green. Add sink-boundary tests
(including a previously emitted prefix), history-independence tests, and
positioning-only failure cases. Run the full suite and targeted race checks.

Do not weaken or skip the remaining shaping-boundary test to make the suite green.
