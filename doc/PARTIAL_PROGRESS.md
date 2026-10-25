# Lookup Application and Partial Progress

This is the architectural guide to tracking completed lookup operations when a
later operation fails. For the public result contract and execution limits, see
[Feature Application](FEATURE_APPLICATION.md). Open work is tracked in
[Code Review](CODE_REVIEW.md); measurements are in
[Shaping Benchmarks](SHAPING_BENCHMARKS.md).

## Contract

`ApplyFeature` returns `(position, applied, error)` and mutates the supplied
`BufferState`. `applied` means at least one operation applied, not that the whole
invocation succeeded or that a value changed. A value-preserving substitution
can apply successfully. Earlier successful operations survive a later error;
there is no rollback, automatic retry, or persistent disabling of a lookup.

Selected GSUB outputs are checked against the font glyph count before the leaf
mutates state. A multiple substitution validates its entire selected sequence
first. A failed output check produces no leaf edit, but does not undo earlier
children of a contextual operation. This is not exhaustive table sanitization.

The returned position agrees with `BufferState.Index`. It is diagnostic, not a
resume token. Errors retain their causes and gain feature, lookup, subtable, and
sequence-record context as they propagate. Execution-limit errors follow the
same partial-progress policy.

## Ownership

| Owner | Responsibility | Must not do |
| --- | --- | --- |
| `BufferState` mutation methods | Change glyphs/positions and record glyph edits | Expect dispatch to repeat position edits |
| Lookup dispatcher | Publish returned buffers/cursor and propagate results | Treat errors as ordinary non-matches or try a fallback subtable |
| Contextual frame | Track original matched inputs through child edits | Replay glyph or position mutations |
| Shaping executor | Apply edits to side metadata and publish current slice headers | Return on error before synchronization |
| Isolated-span integration | Translate coordinates and merge completed local state | Discard partial changes merely because application failed |
| Public streaming API | Emit successfully shaped chunks | Promise whole-call rollback |

Implementation entry points are in `otlayout/feature.go`, `otlayout/gsub.go`,
`otlayout/gpos.go`, `otshape/lookups.go`, and `otshape/runbuffer.go`.

`CloneShared` starts independent journal history while sharing backing buffers.
Mutation methods detach shared storage before writing. `EnsurePos` also detaches
an already aligned shared position buffer before GPOS writes. Nested calls must
use the actual owning `BufferState`, not a new owner made from borrowed slices.

## Ordered Edit Journal

`BufferState.edits` is an internal ordered list of `EditSpan` values.
`Set` and `ReplaceGlyphs` append entries describing mutations already performed.
An entry replaces the half-open range `[From, To)` with `Len` glyphs. Coordinates
refer to the buffer immediately before that entry, not the original input.

- `Set` records a one-to-one edit and preserves positions.
- `ReplaceGlyphs` records `ResetsPositions: true` and updates existing positions.
- `ResetsPositions` distinguishes these operations even when their lengths match.
- Positioning-only operations need not create glyph-edit entries.
- A contextual operation may have many entries and no single aggregate edit.
- Equal initial/final lengths do not imply that no edits occurred.

Every `ApplyFeature` invocation clears the journal on a non-nil state, including
no-ops and early failures. Nested internal lookup calls append to that same
journal rather than starting another feature invocation.

`AppliedEdits()` returns a defensive copy, including completed edits on error.
It does not drain the journal: repeated reads return the same history until a
new invocation or mutation changes it. Consumers must synchronize each batch
once, before the next `ApplyFeature` resets it. The journal is neither an undo
log nor a complete record of positioning changes.

## Contextual Propagation

`applySequenceLookupRecords` builds a map from matched-input sequence indices to
current buffer positions. Before each child call it records `len(st.edits)` as a
checkpoint and sets the cursor to the mapped target.

After a successful child call, it consumes all entries since the checkpoint,
including edits from deeper nested contexts. For each entry:

- Positions after the replaced range shift by `Len - (To - From)`.
- Positions inside a replacement map to its first output position.
- Positions inside a deletion become unavailable; subsequent records targeting
  them are skipped.

Each enclosing frame updates only its own input map. The shared buffer already
contains the child mutations. On error, execution stops and the journal is
retained; the frame need not remap inputs for further records because it does
not continue. It restores its enclosing cursor on exit, while lookup dispatch
publishes the operation's returned cursor. This convention is not resumability.

## Shaping Integration

`applyLookupSpanInternal` consumes `AppliedEdits()` before testing the returned
error. It translates coordinates when needed and calls `run.applyLayoutEdit`
for side arrays only. Whole-run execution publishes glyph/position slice headers
before returning, including reallocated buffers and positioning-only changes.
Simply resizing metadata to the final glyph count would lose correspondence.

Replacement policies:

- Unaffected prefixes and suffixes retain their entries.
- Inserted replacement slots inherit the first consumed input's codepoint,
  plan ID, feature mask, syllable, and joiner annotations.
- Clusters use the minimum consumed cluster; unsafe flags are combined with OR.
- One-to-one edits preserve side metadata.
- Pure insertion uses default metadata; deletion removes consumed entries.

### Isolated spans and attachments

`applyLookupIsolatedSpan` works on detached glyph/position slices, for example
for one syllable. Local edits are translated into parent coordinates for side
metadata. Local slice headers are not published as if they represented the whole
run. Completed glyphs and positions are merged back on both success and error.

The merge replays structural position edits onto the parent positions only when
`ResetsPositions` is true, then copies the completed local positions into place.
This updates the separate parent representation, rather than editing the local
positions twice. A one-to-one replacement still resets positions; `Set` does not.

Attachments inside the span use local indices during application. References
outside the span are preserved separately and translated back during the merge.
Retained targets shift with structural edits. References to replaced targets are
cleared rather than redirected to a new glyph identity. This is conservative
attachment bookkeeping, not a new ligature-component positioning model.

## Worked Example

Consider a contextual lookup matching `[10, 20]`, with glyph count 100. Its child
operations target the original matched inputs, not fixed buffer offsets:

| Step | Operation | Current glyphs | New journal entry |
| --- | --- | --- | --- |
| Initial | Match inputs at positions 0 and 1 | `[10, 20]` | None |
| 1 | Replace input 0 with `[11, 12, 13]` | `[11, 12, 13, 20]` | `{From:0, To:1, Len:3, ResetsPositions:true}` |
| 2 | Set original input 1 to glyph 21 | `[11, 12, 13, 21]` | `{From:3, To:4, Len:1, ResetsPositions:false}` |
| 3 | Attempt to replace that input with `[22, 100]` | `[11, 12, 13, 21]` | None: output 100 is invalid |

After step 1, the contextual map changes from `[0, 1]` to `[0, 3]`. Step 2
therefore targets the original second input, not inserted glyph 12. Step 3
checks both outputs before writing anything.

The result has `applied == true` and a non-nil error. Both completed journal
entries remain available; the returned position equals the state's cursor but
is not a safe restart point. If positions existed initially, four remain aligned
with the glyphs. The first three replacement positions are reset; the original
second input's position survives the expansion and subsequent `Set`.

For initial clusters `[0, 1]`, shaping replays the two entries to produce
`[0, 0, 0, 1]` before returning the error. It does not replay the glyph expansion.
An isolated span performs the same bookkeeping with translated parent indices.
This coherent partial run is internal state, not successful public output.

## Public Output Boundary

`Shape` and `ShapeEvents` write to a `GlyphSink` and return an error; they do not
return a partial shaped-result object. A lookup failure withholds the current
in-progress chunk. Previously emitted prefixes remain in the sink, and input
may already have been consumed. A sink error can itself leave partially written
output.

Applications requiring atomic delivery must stage sink output and commit only
after success. Retrying requires retained input and an appropriate sink; neither
automatic replay nor rollback is provided. See `otshape/shape_api.go` and
`otshape/shape_events.go` for these boundaries.

## Regression-Test Map

| Invariant | Tests |
| --- | --- |
| Glyph and position edits happen once | `otlayout/partial_progress_test.go`: `TestPartialProgressExpansionEditsPositionsOnce` |
| Nested edits remap original input positions | Same file: `TestPartialProgressNestedEditRemapsOuterInput` |
| Shared buffers retain ownership semantics | Same file: `TestPartialProgressContextPreservesSharedOwnership` |
| Zero net length still retains edit history | Same file: `TestEditJournalRetainsZeroNetLengthChanges` |
| Snapshot isolation and per-call reset | Same file: `TestAppliedEditsSnapshotAndReset` |
| Whole/isolated partial state and metadata | `otshape/partial_progress_test.go`: `TestPartialProgressIsolatedSpanRetainsCompletedEdits` |
| Position-only failure publishes state | Same file: `TestPartialProgressPositionOnlyErrorPublishesBuffer` |
| One-to-one attachment parity | Same file: `TestOneToOneAttachmentParity` |
| Failed leaves leave state unchanged | `otlayout/gsub_output_bounds_test.go`: `TestExplicitOutputGlyphBounds` |
| Earlier success survives invalid output | Same file: `TestExplicitOutputNestedPartialProgress`; `otshape/output_bounds_test.go`: `TestInvalidMultipleOutputRetainsEarlierProgress` |
| Failing chunks withheld; prior output retained | `otshape/partial_progress_test.go`: `TestPartialProgressPublicSinkBoundary`; `otshape/output_bounds_test.go`: `TestInvalidMultipleOutputPublicSinkBoundary` |
| Success preserves feature ranges and syllable boundaries | `otshape/successful_edits_test.go` |
| Cycles/work limits preserve the failure contract | `otlayout/execution_limits_test.go` |

Run the full suite and targeted race checks:

```sh
go test ./... -count=1
go test ./otlayout ./otshape ./otshape/otarabic ./otshape/othebrew -race -count=1
```

Controlled fixture graphs exercise these invariants without modifying font files.
Passing tests are not exhaustive script coverage or font sanitization. Existing
font-golden tests can skip malformed subtables; that limitation remains tracked
in the code review. Benchmarks measure the cost of the mechanism, not a guarantee
of bounded total shaping time or memory.
