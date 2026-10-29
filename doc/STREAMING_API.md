# Streaming Shaping: Implementation and Remaining Guarantees

Validated against the source on 2026-10-26. The repeated fill/shape/flush/compact
loop is implemented. Context-safe equivalence to shaping a complete run is not
established and has known counterexamples; see [TESTING.md](TESTING.md).

## Agreed buffer contract (2026-10-27; not yet implemented)

Prefer a generous, reusable buffer and explicit supported limits over complex
logic for arbitrarily small buffers. A required minimum of hundreds or thousands
of slots is acceptable. Exact sizes and units remain to be selected; the current
4096-codepoint default is not evidence that the guarantees below already hold.
Latin and Hebrew are the initial supported scope.

The agreed requirements are:

1. Keep incomplete normalization and base/mark sequences buffered. Impose a
   documented practical maximum on such sequences; supporting arbitrarily long
   combining sequences is not required.
2. Provide sufficient capacity for supported ligatures and their surrounding
   substitution/positioning context. Ligature component count alone is not the
   required source-window size. Account for normalization and substitution
   expansion separately from raw input capacity.
3. Emit only a prefix whose glyphs, clusters, advances, and offsets cannot change
   with further input. Retain any preceding context needed to shape the suffix.
   A sufficiently large buffer still needs a safe-cut rule: a ligature or mark
   sequence can begin near its end.
4. Treat watermarks as scheduling preferences, subordinate to safe progress.
   Reject configurations below the required capacity. If an accepted request
   reaches a hard limit without a safe cut, return an explicit error instead of
   forcing a cut that changes shaping. Already emitted output must be final;
   errors do not imply rollback of earlier sink writes.
5. Preserve shaping context across feature push/pop boundaries for features
   that remain enabled. Increasing buffer capacity alone does not fix the
   separate event-span context barrier (SHAPE-016).

Choose the minimum capacity together with practical sequence/context limits
and reserved carry/lookahead space. Prefer conservative bounds and explicit
rejection of unsupported requirements to elaborate attempts to fit every font
and input into a tiny window. The numeric limits, precise sequence-counting
rules, and error API are implementation decisions still to be specified. No
silent truncation or insertion of characters is authorized by this policy.

Tests must distinguish invalid configuration, supported shaping, and resource
exhaustion. Supported cases must match the independent full-output oracle;
cases exceeding documented limits must assert the specific error and any
already-emitted valid prefix. Do not accept an arbitrary error as a substitute
for correct shaping. Once the minimum is enforced, tiny-buffer discovery cases
will become rejection tests where applicable. Add long Latin/Hebrew inputs
with ligatures, pairs, marks, and scopes deliberately straddling boundaries at
the supported minimum and default sizes, so larger buffers do not merely hide
the existing failures. Current tests and behavior remain unchanged for now.

## Public API

```go
type RuneSource interface {
    ReadRune() (r rune, size int, err error)
}

type GlyphSink interface {
    WriteGlyph(g GlyphRecord) error
}

type BufferOptions struct {
    FlushBoundary FlushBoundary
    HighWatermark int
    LowWatermark  int
    MaxBuffer     int
}
```

Call `(*Shaper).Shape(Params, RuneSource, GlyphSink, BufferOptions)`, or
`(*Shaper).ShapeEvents(Params, InputEventSource, GlyphSink, BufferOptions)`.
There is no current `ShapeOptions` type. Font, script, language, and direction
are fixed for a request; events change feature scopes only.

Enabled alternate values use one-based positive selections, zero for the first
alternate, and negatives for random selection; oversized positives leave the
glyph unchanged by that substitution. A request-local seed is shared by all
event plans. Random lookup attempts derive their generator from that seed, the
original scalar anchor, and the outer lookup index, so reshaping the same carry
anchor repeats the choice. Separate requests can differ. This does not repair
context loss. SHAPE-018 now anchors forward lookup starts to the checked glyph,
including its mask/value and source anchor; reverse traversal remains separate. See [SHAPE-003](SHAPING_FINDINGS.md#shape-003--feature-values-do-not-select-alternates).

The source must return `io.EOF` to terminate. Rune byte sizes are ignored by
ordinary ingestion; cluster IDs are monotonically assigned codepoint indexes.
The sink receives individual records, not batches or flush notifications.

## State and ownership

| Source | Responsibility |
| --- | --- |
| [shape_api.go](../otshape/shape_api.go) | Ordinary streaming loop, shared shaping hooks, and output materialization. |
| [shape_events.go](../otshape/shape_events.go) | Event ingestion, feature-plan spans, and the event streaming loop. |
| [shape_streaming.go](../otshape/shape_streaming.go) | Configuration, raw carry, cut selection, safety checks, and compaction. |
| [shape_runtime.go](../otshape/shape_runtime.go) | Shared ingestor/compiler adapters and reusable scratch workspace. |
| [plan_stack.go](../otshape/plan_stack.go) | Nested feature sets and compiled-plan frames. |
| [runbuffer.go](../otshape/runbuffer.go) | Mutable glyph/position/metadata arrays. |

Raw carry is the source of truth between cycles: `rawRunes`, `rawClusters`,
optional `rawPlanIDs`, `nextCluster`, and EOF/configuration state.
A separate scratch run is rebuilt and shaped from that carry. Scratch allocation
is reused through `shapeWorkspace`; the logical shaping result is recomputed
rather than incrementally resumed after a GSUB/GPOS cursor.

`MaxBuffer` caps raw rune count. It is not a hard limit on total memory:
normalization/substitutions can expand glyph buffers, scratch storage is separate,
and event plans remain in a request-wide map after their scopes are popped.

## Watermark configuration

Current defaults in `resolveStreamingConfig` are:

| Option | Default | Unit |
| --- | ---: | --- |
| `HighWatermark` | 3500 | Raw codepoints buffered before an ordinary shaping attempt |
| `LowWatermark` | 1024 | Raw codepoints covered by a candidate flush prefix |
| `MaxBuffer` | 4096 | Maximum raw codepoints held before forced-progress handling |

Zero selects a default, including `LowWatermark=0`; callers cannot use zero to
request an explicit zero threshold. If a smaller high watermark is supplied
without a low watermark, the default low is clamped down to the high value.
If high exceeds the default max and max is omitted, max is raised to high.

Negative values are rejected. The resolved configuration must satisfy
`high > 0`, `0 <= low <= high`, and `max >= high`.
Explicit inconsistent nonzero values return an error rather than being clamped.

## Current loop

For ordinary rune input, `Shape` selects one engine and compiles one plan before
reading. It then:

1. Fills raw carry up to the high watermark or EOF.
2. Copies carry into scratch, normalizes, and maps codepoints through cmap.
3. Runs preprocess/reorder/pre-GSUB hooks, masks, GSUB/GPOS stages, and
   postprocessing. Masks are initialized before `SetupMasks`, and its changes
   are preserved during execution (SHAPE-001 fixed). The exact hook order is described in
   [SHAPER_INTERFACES.md](SHAPER_INTERFACES.md).
4. Calls `findFlushCut`.
5. If no cut is ready, fills up to max and reshapes.
6. Emits the selected glyph prefix, then removes the corresponding raw prefix
   with `compactCarry`.
7. Repeats until EOF and no carry remains.

Compaction preserves global raw cluster IDs. It copies the unconsumed suffix
within the existing slices; it does not renumber clusters or simply remove
`glyphCut` runes. A decision contains separate `glyphCut` and `rawFlush`
counts.

Source errors abort before shaping that newly filled carry. Lookup/hook errors
prevent emission of the failing chunk. Previously written output and consumed
input are not rolled back.

## Actual cut selection

For a nonempty shaped run:

1. At EOF, flush the entire run and all raw carry.
2. If shaped glyph count differs from raw rune count, do not partially compact.
   Wait for more input, or flush the whole buffer when max is reached.
3. Otherwise enumerate contiguous cluster spans in glyph order.
4. Check the boundary with `isBreakSafeCut`.
5. Require `rawFlushForGlyphPrefix` to demonstrate strictly consecutive,
   one-to-one cluster coverage starting at the raw carry's first cluster.
   Repeated, decreasing, or skipped cluster IDs reject partial compaction.
6. Select the first eligible prefix covering at least the low watermark.
7. At max, if no prefix qualifies, force a full-buffer flush.

This is more conservative than the earlier proposed general cluster mapping,
but the forced full flush can still break contextual shaping.

`isBreakSafeCut` treats interior boundaries as unsafe only when both adjacent
glyphs carry one of the internal unsafe-to-break/concat bits. Missing flag
storage permits the cut; a cut at the run end is always allowed. The predicate
exists and is unit-tested, but the production pipeline does not generate
sufficient unsafe-span information, and `RunContext` provides no flag setter.
There is no script boundary-veto hook.

A cluster boundary is therefore a candidate, not proof that context can safely
be discarded. Joining, ligatures, pair positioning, cursive attachment, and
combining sequences can require context across that boundary.

## Flush modes and output

The sink's agreed order is logical shaping order, also for RTL; the caller owns
visual ordering. Cluster anchors refer to zero-based original input scalars,
with HarfBuzz level-0 grouping/merging. General base/mark cluster formation is
still incomplete (SHAPE-011), and the conservative compaction rules above have
not been redesigned to accommodate it. Logical ordering does not mean retaining
original scalar order within a cluster after normalization/script reordering.

- `FlushOnRunBoundary` writes the selected prefix in glyph order. The name does
  not imply waiting for EOF or emitting an atomic whole run.
- `FlushOnClusterBoundary` iterates cluster spans within that same prefix and
  rejects a cut inside a span.
- Both modes call `WriteGlyph` once per glyph; neither provides atomic cluster
  delivery. If the sink fails midway, earlier writes remain.
- `FlushExplicit` returns `ErrFlushExplicitUnsupported`; no explicit flush
  event exists.
- Unknown flush-enum values currently fall through to ordinary range emission,
  rather than failing validation.

Materialization copies glyph/position/cluster/mask/unsafe data and adds nominal
horizontal advance to the positioning delta. For GPOS-enabled Latin/Hebrew
plans, the post-GPOS policy sets GDEF marks' deltas to cancel nominal advances,
so their emitted advances are zero. Missing GDEF/class tables and Unicode-only
mark inference are outside this policy; offsets and attachments are preserved.
Mark/cursive anchors are not
resolved into final coordinates. Attachment indexes are not converted to a
global sink index during emission.

For atomic output, collect records and publish only after success. Raising
watermarks above a known finite input length allows a whole-input baseline;
the defaults alone do not guarantee that baseline.

## Event input

`InputEventSource.ReadEvent` returns rune, push-features, or pop-features events.
`NewInputEventSource` adapts a rune source, or returns an existing event source
unchanged. Ordinary `Shape` does not automatically switch to event processing.

`ShapeEvents` rejects non-global ranges for nonzero feature tags.
A push clones the current assignments, applies a delta, compiles a plan, and
allocates a new `uint16` plan ID. A pop restores the preceding frame. Underflow,
unclosed scopes at EOF, malformed events, and plan-ID exhaustion return errors.
The final unclosed-stack check can occur after glyphs have already been emitted.

Each rune carries the active plan ID. `shapeEventCarry` normalizes and shapes
each contiguous plan-ID span separately, then appends it to an output run.
Consequently, a feature-scope boundary also separates normalization and lookup
context. It is not just a mask change inside one contextual shaping run.

Further integration limits visible in the source:

- All plans share the selected engine. Compilation mutates engine-local plan
  state; shaping an older span or popping does not restore that state.
- `AppendRun` copies position records without rebasing span-local attachment
  targets to the concatenated run.
- Plans are retained in `plansByID` for the request; max raw buffer size does
  not bound plan count or total plan storage.

These are inspection findings requiring dedicated event/attachment regressions.

## Guarantees, counterexamples, and tests

The code asserts raw-array alignment, monotonic raw clusters, resolved
configuration validity, raw buffer size, and emitted cut bounds. It does not
assert a complete semantic context-safety invariant or prove progress for every
possible engine transformation.

In particular, ordinary `Shape` checks for empty mapped output before GSUB,
but has no equivalent consume-carry branch when shaping later deletes all
glyphs. `findFlushCut` returns no decision for an empty run. That path needs a
termination regression; the event loop does have an empty-shaped-output branch.

The review reproduced:

- A cursive attachment lost with high/low/max `1/1/2`, compared with default
  buffering on `gpos3_font1.otf`, input U+0012 U+0013, feature `test`.

SHAPE-002's indexed-range reproduction now passes: mask assignment uses retained
original scalar anchors after compaction and normalization. Ordinary tests cover
that correspondence; ranges splitting merged source sequences remain out of scope.

Current tests include configuration/fill/compaction/cut helpers in
`shape_streaming_test.go`, loop exercises in `shape_streaming_loop_test.go`,
flush tests, event-stack tests, and engine streaming-parity tests.
Arabic parity uses non-joining digits; Hebrew parity uses plain letters.
Passing these tests and the full suite does not establish arbitrary-context
equivalence.

## Remaining acceptance criteria

The mechanical streaming loop and basic parity tests are implemented. Completion
still requires:

1. A context-retention/boundary policy covering joining, lookback/lookahead,
   normalization, substitutions, and attachments, including max-buffer handling.
2. Defined range behavior inside merged source sequences and reliable provenance
   through future cluster grouping. Source-anchor range assignment across
   compaction and normalization is covered by the SHAPE-002 fix.
3. Final attachment resolution and consistent attachment-index semantics across
   spans and emitted prefixes.
4. Per-plan state isolation and defined context behavior for feature events.
5. Termination tests for empty shaped output and other length-changing cases.
6. Independent expected outputs and boundary sweeps, not only comparison of two
   configurations of the same implementation.

Use [TESTING.md](TESTING.md) for commands and fixture conventions.
