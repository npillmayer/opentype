# Partial Progress: Implementation and Validation

Status: stage 5 (all three bounded passes) and stage 6 validation/documentation
are complete for the bounded GSUB single-substitution format-1 change. Remaining
validation and performance work is tracked in [Code Review](CODE_REVIEW.md).

## Pass 3 Integration

BufferState.AppliedEdits returns a defensive copy of the latest feature invocation's
journal. Every invocation clears it, including no-ops and early failures. Shaping
consumes the journal before checking the error and never replays glyph/position
mutations. It publishes current headers even for positioning-only changes.

Isolated spans update parent metadata with translated edits, then merge completed
glyphs/positions on both success and error. During local processing they do not
publish detached slice headers as the global run. Parent and local attachment
coordinates are translated; outside-span references are preserved separately.

The edit journal distinguishes in-place Set from ReplaceGlyphs using
EditSpan.ResetsPositions. Isolated integration replays positional replacements
based on that flag, not on whether the edit changes length. Whole-buffer and
isolated one-to-one replacement therefore clear the same outside attachments;
Set continues to preserve positioning. TestOneToOneAttachmentParity covers both
operations, prefix/suffix attachments, and all returned position fields.

Metadata policy for replacements:
- Unaffected prefixes/suffixes retain their entries.
- New slots inherit the first consumed input's codepoint, plan ID, mask, syllable,
  and joiner annotations.
- Clusters use the minimum consumed cluster; unsafe flags are ORed.
- One-to-one substitutions preserve all side metadata.
- Pure insertion has default metadata; deletion removes the consumed entries.
- Retained position attachments follow shifted targets. Attachments to replaced
  targets are cleared rather than redirected to a different glyph identity.
  This is a conservative structural policy, not new ligature-component semantics.

Public Shape and ShapeEvents comments specify that the failing chunk is withheld,
but previously emitted prefixes and consumed input cannot be rolled back.
Applications requiring atomic delivery must stage output themselves.

Tests cover the original whole/isolated regressions with actual metadata values,
suffix positioning and attachment indices, contraction/expansion with zero net
length change, position-only errors, journal snapshot isolation/reset, both public
sink boundaries with/without prior output, and history independence after failure.

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
  edits remain available on error; the shaping consumer reads a defensive snapshot.

Tests now cover expansion/contraction alignment, nested mapping, journal order,
preservation of suffix position values, contextual GSUB/GPOS copy-on-write, and
multiple edits with zero net length change.

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

Implemented contract:
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

## Implemented Ownership Model

BufferState owns an ordered edit journal. Each entry uses coordinates immediately
before that edit. Completed edits remain available on error; contextual frames
use invocation checkpoints rather than a single aggregate replacement span.

Responsibilities:
1. BufferState mutation methods change glyphs and positions once.
2. Contextual frames consume child edits to update their own matched-input maps;
   they must not replay glyph or position mutations.
3. The shaping executor consumes edits to update side arrays and publishes current
   headers on both success and error.
4. An isolated span translates edit coordinates into parent coordinates and merges
   its completed state on both success and error.
5. The public streaming layer emits only successfully shaped chunks.

Pass 2 uses a BufferState journal with invocation checkpoints. Pass 3 provides a defensive snapshot across the package boundary. Avoid a general
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

## Validation and Scope

Run the full suite with go test ./... and the targeted race suite with
go test ./otlayout ./otshape -race. The added regression tests have no intentional
failures or skips. Existing font-golden tests can skip malformed subtables; see
CODE_REVIEW.md.

This work does not add rollback, retry, table disabling, exhaustive font
sanitization. Output validation was subsequently extended as described below.

## Stage 6: Successful-Shaping Regression Coverage

Added in otshape/successful_edits_test.go:
- A three-lookup expansion/substitution/contraction sequence, in whole-run and
  per-syllable modes, asserts exact glyphs, masks, clusters, plan IDs, syllables,
  joiners, unsafe flags, codepoints, and retained suffix positioning after every
  lookup. Feature ranges are initialized once on the original inputs.
- A ligature matrix checks that isolated processing blocks cross-syllable matches
  but permits same-syllable contraction; whole-run processing is the control.
- A public Shape test verifies ranged-feature expansion and exact emitted cluster
  IDs, including disabled inputs before and after the enabled range.

These tests use independently parsed fixture fonts with controlled lookup payloads.
Existing Arabic/Hebrew and core fixture tests are run alongside them; their passing
does not constitute exhaustive real-world complex-script coverage.

## Stage 6: Performance Baseline

Allocation-reporting benchmarks now cover ordinary public shaping, whole-run and
per-syllable lookup application, and positional replacement. See
[Shaping Performance Baseline](SHAPING_BENCHMARKS.md) for the pre-stage-5 comparison,
measurement limitations, and candidate profiling targets. No production
optimization is included.

## Stage 6: Bounded Execution and Closeout

`otlayout/execution_limits_test.go` covers self/mutual recursion, terminating
recursion, depth boundaries, broad work-budget exhaustion, independent calls,
concurrent buffers, and retained partial GSUB/GPOS progress. The contract and
limits are documented in [Feature Application](FEATURE_APPLICATION.md).

The attachment-parity regression, bounded contextual execution, successful
shaping coverage, benchmark baseline, and documentation reconciliation complete
stage 6. This is completion of the agreed bounded change, not a claim of complete
font sanitization, exhaustive script coverage, or performance optimization.

Final validation commands:

```sh
go test ./... -count=1
go test ./otlayout ./otshape ./otshape/otarabic ./otshape/othebrew -race -count=1
git diff --check
```

## Follow-Up: Explicit GSUB Output Bounds

All output-producing GSUB handlers share a glyph-count validator. Each selected
leaf output is checked before mutation, including every glyph of a multiple
substitution. No-match and alternate-selection behavior remain unchanged.
Completed earlier contextual operations are retained when a later output fails.

Regression coverage in `otlayout/gsub_output_bounds_test.go` and
`otshape/output_bounds_test.go` exercises boundaries, unchanged failed leaves,
copy-on-write, unused outputs, subtable fallback prevention, extension dispatch,
nested partial progress, whole/isolated-span metadata, both public sink entry
points, and history independence. This does not add eager table sanitization or
coverage/payload consistency validation.
