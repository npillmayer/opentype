# Feature Scopes: Decisions and Remaining Work

Reviewed against the source on 2026-10-26. This document preserves the design
decisions behind feature-control events and tracks remaining scope/boundary
work. It is not the complete shaping backlog; [TESTING.md](TESTING.md) owns the
current testing priorities.

The immediate focus is Latin-script text and Hebrew. Arabic/Syriac and other
scripts are deferred. Shared fixes remain in scope, and existing tests for
deferred scripts should continue to pass.

## Implemented decisions

### Explicit events alongside ordinary rune input

The selected design keeps `RuneSource.ReadRune` for simple clients and adds
`InputEventSource.ReadEvent` for feature-control events. Both APIs exist:

- `(*Shaper).Shape` accepts rune input.
- `(*Shaper).ShapeEvents` accepts rune, push-features, and pop-features events.
- `NewInputEventSource` adapts a rune source to events, or returns an existing
  event source unchanged.
- `FeatureSetting` carries `Tag`, `Value`, and `Enabled`, making feature
  deltas explicit.

See [input_events.go](../otshape/input_events.go) and
[shape_events.go](../otshape/shape_events.go).

The earlier suggestion to normalize both entry points internally into one event
stream was not implemented. Ordinary shaping reads runes directly; the two
paths share runtime helpers but retain separate ingestion loops.

### Explicit stack and restoration by pop

Nested feature scopes use an explicit stack in
[plan_stack.go](../otshape/plan_stack.go). Push clones the current assignments,
applies a delta, compiles a plan, and assigns a plan ID. Pop restores the previous
frame.

For example, a client can push `smcp=1`, supply an acronym, and pop. The pop
restores the previous assignment, which is not necessarily equivalent to
explicitly disabling `smcp`. This nesting behavior is implemented.

A pop at the root is an error. EOF with unclosed scopes is also an error; there
is no auto-pop mode. That final validation can occur after output has been
emitted, so strict stack validation does not mean atomic output.

The stack is iterative rather than recursive. The implementation detects
`uint16` plan-ID exhaustion, but does not expose a configurable nesting-depth
limit. Compiled plans remain in a request-wide map even after pop.

## Historical alternatives and rationale

The original discussion considered three approaches:

| Approach | Decision and rationale |
| --- | --- |
| Special Unicode runes as control markers | Rejected: control would conflict with valid text, including private-use characters, and complicate normalization and cmap handling. |
| `bufio.Scanner` as the primary input abstraction | Not selected: tokenization would move shaping boundaries into split policy and lose the direct rune-reader model. |
| Explicit event records plus a rune-source adapter | Selected and implemented: intent and ordering are explicit, nesting is natural, and clients can still use `bufio.Reader` for ordinary input. |

The choice of an explicit stack made feature restoration and validation
inspectable without tying nesting to the Go call stack.

These API decisions are settled. They do not settle every question about what
context a lookup may read or modify across a feature-scope boundary.

## Boundary semantics: intent versus implementation

There are three different boundaries:

| Boundary | Current meaning |
| --- | --- |
| Indexed `FeatureRange` boundary | Mask changes inside one compiled plan used by `Shape`. The intended indexes are input codepoints, but runtime assignment currently uses chunk-local buffer indexes. |
| Event plan boundary | A change of plan ID caused by feature push/pop. `ShapeEvents` permits only global defaults in `Params.Features`; nested scope comes from events. |
| Streaming flush boundary | A prefix emitted to the sink and removed from raw carry. It can occur independently of feature scopes. |

### The earlier continuity goal

The original design expressed a preference for retaining neighboring glyph
context in one shared buffer even when the active feature plan changed.
Its example was a lookup starting at the last glyph of a small-caps scope and
examining the following glyph under a different feature set.

A later decision recorded semantic fences and deferred cross-fence continuity,
while still proposing one physical `runBuffer` without forced physical splits.
Those were design intentions, not a description of the implementation now in
place.

### What the code actually does

[shapeEventCarry](../otshape/shape_events.go) finds contiguous plan-ID spans,
normalizes and maps each span into a reusable segment buffer, shapes it
separately, and appends the result to an output buffer. Normalization and
lookups therefore do not see neighboring spans.

This contradicts the former guarantee of one physical shaping buffer with
context retained across plan boundaries. Sharing raw carry and reusing scratch
allocation do not provide cross-span lookup context.

Each span selects its stored compiled schedule. However, all plans share the
selected engine instance: compilation can mutate engine-local state, and pop or
replay does not restore that state. The old assertion that every region uses
only its own plan state was therefore too strong.

Other current limitations relevant to scopes are:

- Indexed feature ranges can change behavior after streaming compaction.
- The executor overwrites masks set by `SetupMasks`.
- Feature values do not currently select alternate glyphs correctly.
- Appending shaped spans copies attachment targets without rebasing them to the
  combined output run.
- There is no cross-fence overlap repair or revalidation pass.
- Push/pop events are not rejected merely because they occur inside a
  shaping-sensitive sequence. The proposed break-safe-boundary error check is
  not implemented.

See [SHAPER_PLAN.md](SHAPER_PLAN.md),
[SHAPER_INTERFACES.md](SHAPER_INTERFACES.md), and
[STREAMING_API.md](STREAMING_API.md) for the implementation details.
The review's reproduced failures are recorded in [TESTING.md](TESTING.md);
the state-isolation and attachment-rebasing concerns are inspection findings
that still need dedicated regressions.

## Why continuity still matters for Latin and Hebrew

The concern is not limited to Arabic joining:

- **Latin pair positioning:** a kerning pair can straddle a feature scope or
  emitted prefix.
- **Latin ligatures/contextual substitutions:** a lookup may inspect or consume
  multiple glyphs while feature activation differs among them.
- **Latin accents and Hebrew marks:** separating a base from a mark, or one
  mark from another, can change attachment.
- **Normalization:** a scope or flush cut between a base and combining mark can
  remove a composition/reordering opportunity.

Coverage matching alone does not define the desired behavior. Even with both
neighbors available, the implementation needs rules for where a lookup may
start, what context it may inspect, and which glyphs it may replace or position
when those glyphs have different feature assignments.

The earlier proposal to reject unsafe feature boundaries remains a possible
design choice, not a shipped guardrail. Preserving context or using overlap
repair are other possibilities. This status update does not select a new
continuity policy or turn the current physical split into a final API contract.

## Remaining work

Follow the overall ordering in [TESTING.md](TESTING.md). For feature scopes,
the concrete tasks are:

1. **Cover shared feature semantics through the public API.** Add regressions
   for mask preservation, global input-coordinate ranges, and alternate values
   using Latin fixtures or synthetic engines. Test nested enable/disable/value
   changes and restoration of the exact previous assignments.
2. **Isolate plan-local engine state.** Use a synthetic stateful engine with
   distinguishable per-plan state. Verify root/pushed/popped spans and repeated
   streaming cycles before choosing how state is stored or restored. This work
   does not require expanding Arabic coverage.
3. **Define expected Latin/Hebrew boundary behavior.** Build independent
   baselines for ligatures, kerning, accents, Hebrew marks, and RTL glyph/cluster
   order. Specify expected scope behavior separately from current output; tests
   should not silently make current defects the contract.
4. **Resolve context and attachment handling.** Specify lookup-start,
   context-read, and mutation rules across feature scopes. Test final mark
   coordinates and attachment correspondence when spans are combined. Decide
   whether sensitive boundaries preserve context, require repair, or produce an
   explicit error; document and test the chosen policy.
5. **Exercise streaming and failure boundaries.** Sweep watermarks and push/pop
   locations around the Latin/Hebrew cases, including length-changing and empty
   output. Verify progress, cluster/range preservation, source/sink failures,
   and the documented retention of previously emitted output.

A unified internal event stream, configurable scope-depth limits, and releasing
unused plans are potential later improvements. They should not displace the
current correctness work without evidence that they are needed.

Arabic/Syriac joining, script-specific cursive behavior, and broader script
coverage remain deferred. The historical continuity concerns still apply to
them, but they do not drive the immediate fixture backlog.

For orientation, see [SHAPING_OVERVIEW.md](SHAPING_OVERVIEW.md). Use
[TESTING.md](TESTING.md) for test commands and fixture conventions rather than
treating the historical proposals in this document as unfinished API work.
