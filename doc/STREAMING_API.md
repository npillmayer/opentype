# Streaming Shaping: Implementation and Remaining Guarantees

SHAPE-015 implements conservative safe streaming for the shared Latin/Hebrew
pipeline. Output is final when emitted; a request that cannot establish a safe
boundary within its capacity fails explicitly. This does not establish broader
script or arbitrary custom-engine conformance. The accepted fix and regression
evidence are archived under
[SHAPE-015](SHAPING_FINDINGS.md#shape-015--streaming-cuts-lose-shaping-context).

## Agreed buffer contract (2026-11-02)

| Limit | Value and behavior |
| --- | --- |
| Raw rune capacity | `MinBuffer = 4096`; default 4096. Smaller positive `MaxBuffer` values are silently raised. Callers may request more. |
| Initial base/mark group | `MaxInputGroup = 256` original scalars, before normalization. |
| Each normalized-rune or glyph buffer | `MaxIntermediateEntries = 65536`; storage grows as needed and scratch is reused. This is an emergency expansion ceiling, not an upfront allocation. |
| Lookup execution | Existing 64 nested invocations and 65536 work units per application remain enforced. |

Group counting follows `formInputClusters`: marks (including variation selectors)
and ZWJ extend the preceding group; an ordinary new base, including ZWNJ, starts
a new group. Leading marks form one group. Feature events do not reset the count.
The 257th scalar fails with the original group-start index. EOF or a larger raw
capacity does not bypass this limit. No characters are inserted or truncated.

Watermarks are scheduling preferences. Emit and discard only at a boundary where
both the prefix and suffix can be shaped independently. Keep necessary earlier
context **un-emitted**; there is no replay of already-emitted overlap. The raw
capacity is also the context budget; there is no additional fixed context count.

At full capacity, no extra rune is read merely to discover EOF. Consequently an
input ending exactly at capacity can fail when no safe cut exists and EOF is
still unknown. Increasing capacity can help; retry from the original input with
a fresh sink. Errors are not resumable and do not roll back earlier sink writes.

Error categories, recognizable through wrapping:

- `ErrBufferConfig`: negative or contradictory watermark/capacity settings.
- `ErrBufferFull`: no certified cut at raw capacity. `ErrZeroOutputBufferFull`
  remains the more specific sentinel for provisional empty output and also wraps
  `ErrBufferFull`.
- `ErrSupportedLimit`: input-group, intermediate-buffer, or lookup-execution
  limits. `LimitError` exposes resource, limit, and original index for input,
  normalization, and hook-growth limits; layout failures also retain
  `otlayout.ErrGlyphLimit` or `otlayout.ErrLookupLimit`.

These are supported-operation limits, not a total-memory guarantee: raw copies,
positions, provenance, scratch capacity, font data, and event scope storage are
separate. Raising `MaxBuffer` does not raise the intermediate ceiling.

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
are fixed for a request; events change feature scopes only. Engine
`CollectFeatures` defaults precede user settings; explicit `OverrideFeatures`
assignments supersede all user ranges/scopes for that tag across the whole input
(SHAPE-021). Scope pops restore enclosing user/default settings before this
final override is applied.

Enabled alternate values use one-based positive selections, zero for the first
alternate, and negatives for random selection; oversized positives leave the
glyph unchanged by that substitution. A request-local seed is shared by all
event plans. Random lookup attempts derive their generator from that seed, the
original scalar anchor, and the outer lookup index, so reshaping the same carry
anchor repeats the choice. Separate requests can differ.  SHAPE-018 now anchors forward lookup starts to the checked glyph,
including its mask/value and source anchor; reverse traversal remains separate. See [SHAPE-003](SHAPING_FINDINGS.md#shape-003--feature-values-do-not-select-alternates).

When active features share one alternate lookup in a stage, equal values run
once (zero/default equals one; all negatives mean random). Incompatible values
return `ErrConflictingAlternates` only if a nonempty alternate set matches,
including through context or extension lookups. The failing chunk is withheld;
earlier sink writes remain. This SHAPE-020 contract applies to both public APIs.

The source must return `io.EOF` to terminate. Rune byte sizes are ignored by
ordinary ingestion; cluster IDs are monotonically assigned codepoint indexes.
The sink receives individual records, not batches or flush notifications.

## Caller-owned output slice

For short runs such as word fragments, use the synchronous `Shaper.ShapeInto`
method. It needs no channel, custom sink, or completion call:

```go
var storage [64]otshape.GlyphRecord
n, err := shaper.ShapeInto(params, strings.NewReader("hyphen-"), storage[:])
if err != nil {
    return err
}
glyphs := storage[:n]
```

`ShapeInto(params Params, src RuneSource, dst []GlyphRecord) (n int, err error)`
delegates to ordinary `Shape` with default `BufferOptions`. It shapes exactly
the supplied run, including a visible hyphen if supplied, in logical order.
Cluster and feature indexes remain relative to this call's original scalars.
The shaper and output storage can be reused across calls.

Output space is `len(dst)`, not `cap(dst)`. The method never grows or retains
the slice and leaves `dst[n:]` unchanged. To expose all reusable storage, pass
`scratch[:cap(scratch)]`. An exact fit succeeds; attempting to emit another
glyph returns `ErrGlyphBufferFull`, distinct from the internal shaping error
`ErrBufferFull`. Nil and zero-length destinations are permitted: they succeed
if shaping succeeds with no output, including when substitutions delete all
input. Destination size does not bypass input validation or shaping limits.

On any error, `n` reports the records already written. They remain in the
destination, but the fragment is incomplete and may end inside a cluster;
use the fragment only after success. Neither input consumption nor destination
writes are rolled back. To retry overflow, provide a larger destination and a
fresh or reset reader. The overflow error does not give the total required size.

This API reuses caller-owned output storage, but still allocates internal
shaping workspace. Its internal buffer limits are independent of output space.
Clients requiring explicit buffer settings can continue to use `Shape` with a
sink. See `ExampleShaper_ShapeInto` in
[example_test.go](../otshape/example_test.go) for a complete example.

## Buffered channel API

`NewStreamingShaper(capacity, engines...)` creates an ordinary `Shaper`, a
buffered glyph channel, and an internal channel sink. Capacity must be positive
and counts output records; it is separate from `BufferOptions` and its raw-input
limits. Construction starts no goroutines.

```go
func NewStreamingShaper(capacity int, engines ...ShapingEngine) (*StreamingShaper, error)
func (s *StreamingShaper) Shape(ctx context.Context, params Params, src RuneSource, opts BufferOptions) error
func (s *StreamingShaper) SetInitialCluster(id uint32) error
func (s *StreamingShaper) Glyphs() <-chan GlyphRecord
func (s *StreamingShaper) Finish() error
func (s *StreamingShaper) Err() error
```

Each synchronous `Shape` call handles one uniform run with its own parameters
and buffer settings. A producer goroutine can call it repeatedly while a
consumer drains `Glyphs()`. Runs are shaped independently, with no retained
shaping context between calls. Output stays in logical order, with no run or
cluster completion records. Itemization, metadata associations, and Unicode
segmentation remain the caller's responsibility. This wrapper accepts ordinary
`RuneSource` input; it does not add a channel version of `ShapeEvents`.

The stream starts with cluster offset zero. After each successful run, the
offset advances by its original scalar count, including scalars merged into
clusters or producing no output. `SetInitialCluster` overrides the next offset
before or between calls. Only output cluster IDs are rebased; indexed feature
ranges, diagnostic source indexes, and other glyph fields keep their ordinary
run-local meanings. IDs must fit `uint32`; an overflowing source position or
output cluster fails with `ErrClusterOverflow` rather than wrapping. A single
call is also limited to `MaxUint32` scalars so the existing internal next-index
counter remains representable. After the last representable scalar, empty runs
and `Finish` still succeed, and an explicit cluster reset permits another run.

| Operation/result | Stream behavior |
| --- | --- |
| Successful `Shape`, including empty input | Leaves the channel open for the next run. |
| `Finish` while idle | Closes the channel; queued records can still be drained. Repeated calls return the same terminal result. |
| Accepted `Shape` fails or is canceled | Stores its error before closing the channel; earlier output remains queued. The error is also returned directly. |
| `Shape`, `SetInitialCluster`, or `Finish` during an active call | Returns `ErrStreamBusy` without altering that call or the stream's terminal error. |
| `Shape` or `SetInitialCluster` after closure | Returns `ErrStreamClosed`. |

After draining the closed channel, `Err()` returns the final result: nil on
successful completion, otherwise the shaping/source/cancellation error. A nil
`Err()` while the channel remains open does not establish completion. No error
sentinel is mixed into the glyph channel, and failures are not resumable.

Channel sends select on the supplied context, so cancellation releases a
producer blocked by a full channel. Cancellation is also checked between source
reads and before returning success. It does not interrupt a `ReadRune` already
executing or an executing shaping lookup. Blocking sources must cooperate with
cancellation themselves. The context applies during that `Shape` call, not
while the stream is idle: the producer must eventually call `Finish` or submit
another run. A consumer stopping early must cancel the producer's active
context. The active producer closes its channel only after it has stopped
sending; `Finish` does not close underneath it.

Lifecycle operations and `Err` are synchronized. This does not establish
concurrent safety for fonts or engines shared with other shapers. An instance
must be constructed with `NewStreamingShaper` and must not be copied.

See `ExampleStreamingShaper` in [example_test.go](../otshape/example_test.go)
for a complete producer/consumer example. `TestStreamingShaper*` covers reuse,
output parity across Latin/Hebrew runs and internal flushes, cluster rebasing,
terminal errors, and cancellation under backpressure. The example's glyph
expectations come from the existing independent HarfBuzz `office` capture.

## State and ownership

| Source | Responsibility |
| --- | --- |
| [shape_api.go](../otshape/shape_api.go) | Ordinary streaming loop and shared shaping hooks. |
| [output.go](../otshape/output.go) | Output materialization. |
| [clusters.go](../otshape/clusters.go) | Cluster formation, merging, and the cluster spans used for cuts and flushing. |
| [shape_events.go](../otshape/shape_events.go) | Event ingestion, shared feature-range plans, and the event streaming loop. |
| [shape_streaming.go](../otshape/shape_streaming.go) | Configuration, raw carry, cut selection, safety checks, and compaction. |
| [shape_runtime.go](../otshape/shape_runtime.go) | Shared ingestor/compiler adapters and reusable scratch workspace. |
| [plan_stack.go](../otshape/plan_stack.go) | Nested immutable feature sets and scope IDs. |
| [runbuffer.go](../otshape/runbuffer.go) | Mutable glyph/position/metadata arrays. |

Raw carry is the source of truth between cycles: `rawRunes`, `rawClusters`,
optional `rawPlanIDs`, `nextCluster`, and EOF/configuration state.
A separate scratch run is rebuilt and shaped from that carry. Scratch allocation
is reused through `shapeWorkspace`; the logical shaping result is recomputed
rather than incrementally resumed after a GSUB/GPOS cursor.

`MaxBuffer` caps raw rune count. It is not a hard limit on total memory:
normalization/substitutions can expand glyph buffers, scratch storage is separate,
and event scope assignments remain in a request-wide map after they are popped.

## Watermark configuration

Current defaults in `resolveStreamingConfig` are:

| Option | Default | Unit |
| --- | ---: | --- |
| `HighWatermark` | 3500 | Raw codepoints buffered before an ordinary shaping attempt |
| `LowWatermark` | 1024 | Raw codepoints covered by a candidate flush prefix |
| `MaxBuffer` | 4096 | Maximum retained raw codepoints |

Zero selects a default, including `LowWatermark=0`; callers cannot use zero to
request an explicit zero threshold. If a smaller high watermark is supplied
without a low watermark, the default low is clamped down to the high value.
If high exceeds the default max and max is omitted, max is raised to high.

Negative values are rejected. The resolved configuration must satisfy
`high > 0`, `0 <= low <= high`, and `max >= high`.
Positive `MaxBuffer` below 4096 is first raised to 4096; remaining contradictions
return `ErrBufferConfig`. No other explicit threshold is silently adjusted.

## Current loop

For ordinary rune input, `Shape` selects one engine and compiles one plan before
reading. It then:

1. Fills raw carry up to the high watermark or EOF.
2. Copies carry into scratch, normalizes, and maps codepoints through cmap.
3. Runs preprocess/reorder/pre-GSUB hooks, masks, GSUB/GPOS stages, and
   postprocessing. Masks are initialized before `SetupMasks`, and its changes
   are preserved during execution (SHAPE-001 fixed). The exact hook order is described in
   [SHAPER_INTERFACES.md](SHAPER_INTERFACES.md).
4. Handles zero output as described below; otherwise calls `findFlushCut`.
5. If no cut is ready, fills up to max and reshapes; if already at max, returns
   `ErrBufferFull`.
6. Emits the selected glyph prefix, then removes the corresponding raw prefix
   with `compactCarry`.
7. Repeats until EOF and no carry remains.

For an empty shaped result, both APIs retain raw carry until EOF is known.
They fill up to `MaxBuffer` and reshape, because later context can change an
initially empty result. At EOF they consume the raw carry without emitting
anything; event mode also validates scope closure. If raw capacity is reached
before EOF is known and output is still empty, they return
`ErrZeroOutputBufferFull`, detectable with `errors.Is`. Input ending exactly at
the cap is included: no extra read is made to discover EOF. Earlier sink writes
remain. This SHAPE-017 rule also handles empty normalized/mapped results; it does
also serves as the empty-output branch of SHAPE-015.

Compaction preserves global raw cluster IDs. It copies the unconsumed suffix
within the existing slices; it does not renumber clusters or simply remove
`glyphCut` runes. A decision contains separate `glyphCut` and `rawFlush`
counts.

Source errors abort before shaping that newly filled carry. Lookup/hook errors
prevent emission of the failing chunk. Previously written output and consumed
input are not rolled back.

## Safe cut selection

At EOF the complete shaped carry can be emitted, subject to supported limits.
Before EOF, a separate raw-source trace certifies boundaries. Public clusters
and source anchors keep their existing meanings.

- Initial groups and compositions cannot be split; the last initial group stays
  buffered for possible additional marks/composition.
- Successful lookup attempts join the raw intervals they read, including nested
  lookups, ignored glyphs, context, and attachments. Failed attempts reaching the
  unfinished right edge retain their entire dependency interval. Interior
  non-matches alone do not forbid a cut.
- Before lookup stages and between operations, retain backward context for future
  starts. Count eligible glyphs with the lookup's own flags, including nested
  flags. Mark/base and mark/ligature scans retain the most recent eligible
  covered target; mark/mark retains the nearest eligible predecessor. This
  follows current lookup semantics without changing target selection.
- Event requests also retain context for selectable features that later events
  could enable, even when currently absent from the combined plan.
- Raw-source intervals follow edits immediately, including nested expansion,
  contraction, deletion, and surviving skipped marks. A candidate must separate
  all output prefix/suffix source intervals and complete public clusters.

The first certified prefix meeting the low watermark is emitted. At capacity a
shorter certified prefix may be used. If none exists, return `ErrBufferFull`.
Length changes and repeated clusters no longer require a full-buffer flush.

Unknown/stateful run or plan hooks, stage pauses, and per-syllable processing use
whole-run fallback. Normalization hooks require the optional
`ShapingEngineStreamingNormalization` locality promise; the Hebrew engine
implements it. Reverse chaining also falls back when a buffered glyph can
participate. These requests work if EOF is discovered within capacity; otherwise
they fail without forced output.

Hebrew presentation-form fallback depends on plan-wide `mark` support. Event
requests whose root plan lacks that support use whole-run fallback, because a
future event could change normalization of earlier text. The existing plan-wide
contract is preserved, including the conservative no-mark-font case. Shared
lookup contributors with differing plan-wide random policies also trigger event
fallback; differing joiner policies trigger it when joiners are present. A future
feature selection could otherwise change those policies for earlier text.

Conservative dependencies may retain more text than strictly necessary. In
particular the existing mark/base and mark/ligature predecessor search can reach
distant targets; this change does not repair that separate target-selection
limitation. A larger buffer or whole-input request may be needed.

## Flush modes and output

The sink's agreed order is logical shaping order, also for RTL; the caller owns
visual ordering. Cluster anchors refer to zero-based original input scalars,
with HarfBuzz level-0 grouping/merging. Latin/Hebrew base/mark formation is now
implemented (SHAPE-011). Per-glyph source anchors are retained separately, and the trace supports repeated output clusters. Logical ordering does not mean retaining
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
Within this horizontal Latin/Hebrew scope, GPOS mark-to-base, mark-to-ligature,
and mark-to-mark anchors now produce design-unit offsets. Finalization runs
after GPOS/advance zeroing and before `PostprocessRun`, incorporating target
placement and final pen differences while retaining logical output order.
`PosItem.MarkAttachmentPending` is cleared; attachment diagnostics remain.
Cursive, hint/device/variation adjustments,
and attachment context lost at buffer cuts remain outside this fix.
SHAPE-019 now preserves skipped marks and carries GSUB ligature-component
association through a shaped span, including nested ligatures. GPOS 5 uses it
to choose the component anchor; input ligatures and unassociated marks retain
the last-component fallback. This provenance does not span independently shaped
buffer windows and does not establish safe cuts. Final glyph records omit
the transient identity/component data while retaining attachment diagnostics.
Attachment indexes are not converted to a global sink index during emission;
consumers use the finalized offsets for rendering.

For atomic output, collect records and publish only after success. Raising
watermarks above a known finite input length allows a whole-input baseline;
the defaults alone do not guarantee that baseline.

## Event input

`InputEventSource.ReadEvent` returns rune, push-features, or pop-features events.
`NewInputEventSource` adapts a rune source, or returns an existing event source
unchanged. Ordinary `Shape` does not automatically switch to event processing.

`ShapeEvents` rejects non-global ranges for nonzero feature tags.
A push clones the current assignments, applies a delta, and allocates a new
`uint16` scope ID (internally still called a plan ID). A pop restores the
preceding frame. Underflow, unclosed scopes at EOF, malformed events, and ID
exhaustion return errors. The final unclosed-stack check can occur after glyphs
have already been emitted.

Each rune carries the active scope ID. Before shaping, `eventFeatureRanges`
expresses those assignments in original scalar coordinates, preserving explicit
and implicit root defaults outside each scope. `shapeEventCarry` compiles one
combined plan/mask layout, creates and initializes its engine, and normalizes,
maps, and shapes the whole carry. Run hooks execute once for that carry.
Feature boundaries no longer split normalization, lookup context, or attachment
indexes. Scope changes inside initial base/mark groups follow the group-start
rule used by `Shape` ranges; they never split the group.

Actual matched inputs must have an active lookup mask. Disabled inputs stop a
ligature, pair, or contextual input match; backtrack/lookahead remain available
across settings boundaries, and ignored-glyph skipping still follows lookup flags.
This does not implement all cursive or script-specific matching rules.

GPOS mark-to-mark attachment additionally stops at the nearest eligible
predecessor, which must be a GDEF mark in Mark2Coverage (SHAPE-022). An intervening
base/ligature or eligible uncovered mark blocks attachment to earlier marks.
This does not add general ligature/component compatibility.

The root plan is validated before ingestion. Push/pop only stores assignments;
a nonempty carry compiles its combined plan before shaping. Compilation or
validation failure suppresses that carry's output. The combined plan uses the
existing 31-bit feature-mask budget; it is not a separate budget per scope.
Balanced empty scopes have no run effect. Scope snapshots are retained for the
request, so the raw buffer limit does not bound their storage. Stateful engines
must return independent instances from `New`; nil retains the existing fallback
to reuse. Request random seeds remain stable across combined-plan rebuilds.

These changes resolve SHAPE-016's event barrier; SHAPE-015 still governs safe
buffer cuts. SHAPE-021 now resolves engine/user setting precedence for both APIs.

## Verification and remaining scope

`TestHarfBuzzStreamingBaselines` retains all 340 complete comparisons from the
original short matrix. The tiny profiles now test silent capacity clamping and
output correctness; they are not evidence of streaming at tiny capacities.
`TestSHAPE015LongStreamingHarfBuzz` uses four separately captured complete
HarfBuzz 15 outputs (11,200–12,000 scalars), both APIs and flush modes, default
settings, explicit 4096 capacity, and a larger 8192 capacity. It requires at least
three pre-EOF emission cycles and compares every output field. Indexed features
and equivalent nested event schedules cross actual streaming windows.

Focused tests cover 256/257-scalar groups across events, emergency expansion
limits, unknown-engine fallback, exact-cap EOF behavior, zero-output carry,
earlier emitted output, context/lookahead, ignored glyphs, nested lookups,
source provenance, and buffer ownership. The source/sink budgets and test timeout
also detect failure to make progress.

Broader script-specific shaping, cursive finalization, unrestricted custom
engines, and arbitrary-length dependencies remain outside this guarantee.
See [TESTING.md](TESTING.md) for commands and fixture conventions.
