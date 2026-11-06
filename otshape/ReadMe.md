# Shaping APIs

Package `otshape` turns a run of Unicode text into positioned OpenType glyphs.
Each shaping call uses one font, script, language, and direction. The caller
owns itemization, font fallback, Unicode segmentation, paragraph bidi handling,
line breaking, and associations with layout metadata.

This document describes the public shaping entry points. Code blocks contain
API declarations only.

## Choosing an interface

| API | Input | Output | Intended use |
| --- | --- | --- | --- |
| `Shaper.Shape` | `RuneSource` | Caller-supplied `GlyphSink` | Direct synchronous shaping with explicit buffer settings. |
| `Shaper.ShapeInto` | `RuneSource` | Caller-owned `[]GlyphRecord` | Short runs and word fragments with bounded output storage. |
| `Shaper.ShapeEvents` | `InputEventSource` | Caller-supplied `GlyphSink` | Nested feature scopes within one uniform run. |
| `StreamingShaper.Shape` | `RuneSource` | Buffered channel | Sequential runs produced and consumed concurrently. |

All four use the same shaping machinery. A slice or channel changes output
delivery, not the shaping algorithm. Internal shaping workspace is still
allocated, including when output storage is supplied by the caller.

## Common parameters and output

### Run parameters

`Params` describes one run:

| Field | Meaning |
| --- | --- |
| `Font` | Non-nil parsed `*ot.Font`. Keep the font and its underlying bytes immutable while in use. |
| `Direction` | Run direction, using `bidi.Direction`. The caller determines it. |
| `Script` | ISO 15924 script, using `language.Script`, for engine and layout selection. |
| `Language` | Language-system selection, using `language.Tag`. |
| `Features` | OpenType feature settings and, where supported, ranges over original input scalars. |

`FeatureRange` identifies a feature with `Feature`, enables or disables it with
`On`, and supplies its value through `Arg`. `Start` is inclusive and `End` is
exclusive. Nonpositive `Start` means the beginning; nonpositive `End` means the
end of input. Both zero selects the whole run.

Indexes count original Unicode scalars, not UTF-8 bytes or output glyphs. They
remain relative to the current call even when the channel API rebases output
clusters. Initial base/mark groups take feature settings from their first scalar;
a feature boundary inside such a group does not split it.

Enabled alternate features use one-based positive values. Zero selects the
first/default alternate; negative values request random selection. An oversized
positive value leaves the glyph unchanged by that alternate substitution.
Engine-collected defaults precede user settings; explicit engine overrides take
precedence over user ranges and event scopes. Random choices may differ between
separate shaping calls.

### Rune input and glyph output

```go
type RuneSource interface {
    ReadRune() (r rune, size int, err error)
}

type GlyphSink interface {
    WriteGlyph(g GlyphRecord) error
}
```

`RuneSource` has the same method signature as `io.RuneReader`. A source must
return `io.EOF` to end a run. The byte-size return value does not determine
cluster numbering. Returning an error from `WriteGlyph` aborts shaping.

`GlyphRecord` contains the glyph ID (`GID`), final positioning (`Pos`), cluster
anchor (`Cluster`), feature mask (`Mask`), and safety hints (`UnsafeFlags`).
Positions are in font design units; horizontal advance already includes the
nominal glyph advance. Use finalized offsets for positioning. Attachment indexes
are not rebased to a global output index, and masks and safety hints should not
be treated as a complete line-breaking interface.

Output is in **logical shaping order**, including RTL runs. Visual ordering is
the caller's responsibility. Normalization and script-specific reordering may
change order inside a cluster. Clusters follow HarfBuzz level-0 grouping: several
scalars can share an anchor, and several output glyphs can share a cluster.
A cluster boundary alone does not establish a safe line break.

## Constructing a Shaper

```go
func NewShaper(engines ...ShapingEngine) *Shaper
```

The constructor records explicit candidate engines and ignores nil entries.
There is no global registry or automatic engine registration. Available engine
constructors are `otcore.New`, `othebrew.New`, and `otarabic.New` in their
respective subpackages. Latin and Hebrew are the current correctness focus;
the existence of an engine does not establish full script conformance.

For each call, the shaper selects the highest positive match score, breaking ties
by engine name. No candidates yields `ErrNoShaper`; no matching candidate yields
`ErrNoMatchingShaper`. The selected engine's `New` method supplies its instance;
custom stateful engines should return an independent instance.

A `Shaper` can be reused sequentially across calls with different parameters.
Calls do not retain shaping context or cluster numbering from earlier runs.
Concurrent use of an ordinary `Shaper`, or sharing fonts and engines across
concurrent shaping calls, is not guaranteed safe.

## Shape: caller-supplied sink

```go
func (s *Shaper) Shape(params Params, src RuneSource, sink GlyphSink, bufOpts BufferOptions) error
```

This synchronous call pulls input until EOF and writes final glyph records to
the sink as safe prefixes become available. It can emit before EOF. Cluster
numbering starts at zero for each call. The caller owns the source and sink;
there is no sink-close callback or completion record. The return value reports
completion or failure.

Nil font, source, or sink produces `ErrNilFont`, `ErrNilRuneSource`, or
`ErrNilGlyphSink`. Source, sink, planning, and shaping errors propagate to the
caller. A failed shaping chunk is withheld, but earlier sink writes remain.
A sink error may occur after part of a chunk or cluster has already been written.
Consumed input and output writes are not rolled back.

## ShapeInto: caller-owned slice

```go
func (s *Shaper) ShapeInto(params Params, src RuneSource, dst []GlyphRecord) (n int, err error)
```

This synchronous method uses ordinary `Shape` with default `BufferOptions` and
an internal bounded sink. It is suitable for independently shaped fragments,
including any visible hyphen supplied in the source.

- Writable output space is `len(dst)`, not `cap(dst)`.
- On success, the complete result is `dst[:n]`.
- The method never grows or retains the slice and leaves `dst[n:]` unchanged.
- An exact fit succeeds. Attempting to write another glyph returns
  `ErrGlyphBufferFull`, distinct from the internal-buffer error `ErrBufferFull`.
- A nil or zero-length slice succeeds when shaping succeeds with no output,
  including when substitutions delete all input. Validation still applies.
- On any error, `n` counts records already written. The fragment is incomplete
  and may end inside a cluster; use it only after success.

Retrying overflow requires a larger destination and a fresh or reset source.
The error does not report the total required output size. Neither source
consumption nor destination writes are rolled back. Reusing the shaper or the
slice requires no reset method. Cluster and feature indexes start from the
current run's original input coordinates on every call.

The destination length does not limit internal workspace or replace the ordinary
shaping limits. Use the sink-based API when explicit buffer settings are needed.

## ShapeEvents: nested feature scopes

```go
func (s *Shaper) ShapeEvents(params Params, src InputEventSource, sink GlyphSink, bufOpts BufferOptions) error

type InputEventSource interface {
    ReadEvent() (InputEvent, error)
}

func NewInputEventSource(src RuneSource) InputEventSource
func (ev InputEvent) Validate() error
```

`ShapeEvents` keeps font, script, language, and direction fixed, while consuming
three kinds of `InputEvent`:

| Kind | Payload and effect |
| --- | --- |
| `InputEventRune` | `Rune` supplies one character; `Size` carries its byte size for adapter compatibility. |
| `InputEventPushFeatures` | Nonempty `Push` contains `FeatureSetting` values with `Tag`, `Value`, and `Enabled`; pushes a nested delta over the current settings. |
| `InputEventPopFeatures` | No payload; restores the preceding feature settings. |

`Params.Features` provides global defaults: `Start` and `End` must both be zero
for each nonzero feature tag, otherwise `ErrEventIndexedFeatureRange` is returned.
Range scoping comes from push/pop events. A pop restores the enclosing settings,
which is not necessarily equivalent to disabling a feature. Enabled zero-valued
settings select the first/default alternate.

Feature scopes share normalization and lookup context within the shaped carry.
Initial base/mark groups use the settings at their first scalar. Feature events
do not contribute to cluster indexes or change the run's fixed properties.

The source returns `io.EOF` to finish the run. Invalid events, stack underflow,
unclosed scopes, and exhausted scope IDs return errors. `InputEvent.Validate`
checks event payloads, not stack balance; `ShapeEvents` performs both kinds of
validation. Unclosed scopes may be detected after output has been emitted.
The same partial-output contract as `Shape` applies. A nil source returns
`ErrNilEventSource`.

`NewInputEventSource` adapts ordinary rune reads into rune events. A source that
already implements `InputEventSource` is returned unchanged; nil produces nil.
Using this adapter does not create feature scopes automatically. Ordinary
`Shape` does not switch to event processing based on the source's extra methods.

## StreamingShaper: buffered channel

```go
func NewStreamingShaper(capacity int, engines ...ShapingEngine) (*StreamingShaper, error)
func (s *StreamingShaper) Shape(ctx context.Context, params Params, src RuneSource, opts BufferOptions) error
func (s *StreamingShaper) SetInitialCluster(id uint32) error
func (s *StreamingShaper) Glyphs() <-chan GlyphRecord
func (s *StreamingShaper) Finish() error
func (s *StreamingShaper) Err() error
```

The constructor creates an ordinary shaper, a buffered channel, and its internal
sink. Capacity must be positive or construction returns `ErrChannelCapacity`.
Capacity counts glyph records and is independent of internal shaping buffers.
Construction starts no goroutines. Use the constructor; the zero value is not
usable, and a constructed value must not be copied.

### Producing runs and consuming glyphs

`Shape` is synchronous. Typically the caller runs a producer that submits runs
sequentially while another goroutine drains `Glyphs`. Each run supplies its own
parameters and buffer settings and is shaped independently. A full channel
blocks the producer until space becomes available or the context is canceled.

`Glyphs` always returns the same receive-only channel. It contains glyph records
only, without error sentinels, run boundaries, or cluster-completion markers.
The caller maintains any metadata associations. There is no channel variant
of `ShapeEvents`.

### Completion and errors

| Operation or result | Effect |
| --- | --- |
| Successful `Shape`, including empty input | Finishes the run and leaves the channel open. |
| `Finish` while idle | Closes the channel after the final run, including an empty sequence of runs. Queued records remain available. |
| Failed or canceled accepted `Shape` | Stores the error and closes the channel after sending has stopped. Earlier queued records remain. |
| `Shape`, `SetInitialCluster`, or `Finish` during an active `Shape` | Returns `ErrStreamBusy` without altering the active call or terminal result. |
| `Shape` or `SetInitialCluster` after closure | Returns `ErrStreamClosed`. |
| Repeated `Finish` after closure | Returns the stored terminal result without further effects. |

After draining the closed channel, call `Err` to distinguish success from failure.
The error is stored before closure and also returned directly by the failed
`Shape` call. A nil `Err` while the channel is open does not imply completion.
Lifecycle misuse errors are not stored as the terminal result. A terminated
stream cannot be reopened; create a new one to retry.

### Cluster continuation

The initial output cluster offset is zero. Each successful run advances the
offset by the number of original input scalars, including characters merged
into clusters or producing no glyphs. `SetInitialCluster` overrides the next
offset before or between calls.

Only emitted cluster IDs are rebased. Feature ranges, diagnostic source indexes,
and other glyph fields retain their ordinary run-local meanings. Unrepresentable
source positions or output cluster IDs fail with `ErrClusterOverflow`; they do
not wrap. One call is limited to `MaxUint32` original scalars so the internal
next-index counter remains representable. After global numbering is exhausted,
empty runs and `Finish` still succeed, or a cluster reset can permit another run.

### Cancellation and concurrency

The context applies to the active call. Cancellation releases blocked channel
sends and is checked between source reads and before successful return. A nil
context terminates an accepted call with `ErrNilContext`.

Cancellation cannot interrupt an already executing `ReadRune` or shaping lookup.
A source that can block must cooperate with cancellation itself. A consumer that
stops reading early must cancel the active producer context. While the stream is
idle, cancellation alone does not close it: the producer must call `Finish` or
submit another run. `Finish` does not wait for an active call; it returns busy.

Lifecycle methods and `Err` are synchronized, but shaping calls on one stream
must remain sequential. This synchronization does not make fonts or engines
safe to share across other concurrent shaping operations.

## Internal buffering and limits

`Shape`, `ShapeEvents`, and `StreamingShaper.Shape` accept `BufferOptions`.
`ShapeInto` uses its zero-value defaults.

| Setting | Default | Meaning |
| --- | ---: | --- |
| `HighWatermark` | 3500 | Raw scalars buffered before an ordinary shaping attempt. |
| `LowWatermark` | 1024 | Preferred raw-prefix length for a safe flush. |
| `MaxBuffer` | 4096 | Maximum retained raw scalars. |
| `FlushBoundary` | `FlushOnRunBoundary` | Emit a certified prefix in logical glyph order. |

Zero selects defaults, including for `LowWatermark`. Smaller positive
`MaxBuffer` requests are raised to `MinBuffer` (4096). An omitted maximum grows
to accommodate a larger high watermark; an omitted low watermark is clamped
to a smaller high watermark. Negative or inconsistent settings return
`ErrBufferConfig`.

`FlushOnRunBoundary` does not mean atomic delivery or waiting for the whole run.
`FlushOnClusterBoundary` additionally checks that the selected prefix ends at a
cluster boundary. Both call `WriteGlyph` per glyph and can leave partial output
on a sink failure. `FlushExplicit` is unsupported and returns
`ErrFlushExplicitUnsupported`.

The shared Latin/Hebrew pipeline emits only final output at certified safe
boundaries. Lack of safe progress at capacity returns `ErrBufferFull`, rather
than forcing a cut. Provisional zero output at capacity returns
`ErrZeroOutputBufferFull`, which wraps `ErrBufferFull`. There is no extra read
solely to discover EOF at full capacity, so an input ending exactly at capacity
can still fail. Some custom engines and unsupported context patterns require
whole-run buffering and may need larger capacity.

Initial base/mark groups are limited to `MaxInputGroup` (256 original scalars).
Each normalized-rune or glyph buffer is limited to `MaxIntermediateEntries`
(65536 entries). These and lookup-execution limits report `ErrSupportedLimit`;
`LimitError` exposes resource, limit, and source index where applicable.
These are operational bounds, not a total-memory guarantee.

Use `errors.Is` for sentinel errors and `errors.As` for structured diagnostics.
Source/sink errors and planning/layout errors can also propagate. If complete
delivery must be atomic, collect output and publish it only after success.
See the detailed [buffer contract](../doc/STREAMING_API.md).

## Root-package convenience API

The parent `opentype` package also provides:

```go
func ShapeLatinText(otf *ot.Font, text string) ([]otshape.GlyphRecord, error)
```

It selects the core engine, Latin script, English language, and LTR direction,
allocates output storage, and returns glyphs in logical order. Nil font or empty
text returns `nil, nil`. Otherwise, shaping errors propagate with any collected
prefix. Use `otshape` directly for other parameters, caller-owned storage,
feature scopes, or channel delivery.

## Further reference

- [Shaping interfaces and engine hooks](../doc/SHAPER_INTERFACES.md)
- [Pipeline overview](../doc/SHAPING_OVERVIEW.md)
- [Streaming and buffer contracts](../doc/STREAMING_API.md)
- [Known findings and accepted fixes](../doc/SHAPING_FINDINGS.md)
- [Testing and fixture provenance](../doc/TESTING.md)
