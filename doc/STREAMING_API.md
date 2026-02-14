# Streaming API Realization Plan

## 1. Target Behavior

The streaming shaper shall work as a bounded, repeated fill/shape/flush/compact loop:

1. Read runes from source into an internal buffer until either:
   1. a high-watermark fill level is reached, or
   2. source returns EOF.
2. Shape buffered content until a flush-safe cut can be selected:
   1. cut must be at or beyond low-watermark, and
   2. cut must be at a break-safe cursor position.
3. Flush shaped glyph records for the safe prefix (SoA -> AoS boundary conversion).
4. Compact buffer by moving unflushed (unshaped-for-output) tail to buffer start.
5. Repeat from step 1 until EOF and no carry remains.

## 2. Core Design Decision

Use two layers:

1. **Raw carry buffer** (source of truth across cycles):
   1. stores buffered codepoints and cluster ids not yet committed to sink,
   2. survives across loop iterations.
2. **Scratch shaping buffer** (ephemeral per cycle):
   1. built from current raw carry,
   2. fully shaped each iteration,
   3. discarded after flush decision.

This avoids fragile partial GSUB/GPOS continuation logic while still enabling real streaming output.

## 3. Required State

Add internal streaming state (likely in `otshape/shape_api.go`):

1. `rawRunes []rune`
2. `rawClusters []uint32` (monotonic global cluster ids)
3. `nextCluster uint32`
4. `eof bool`
5. `highWatermark int`
6. `lowWatermark int`
7. `maxBuffer int` (hard cap for forced progress)

Suggested defaults:

1. `highWatermark = 256`
2. `lowWatermark = 128`
3. `maxBuffer = 4096`

Validation rules:

1. `highWatermark > 0`
2. `0 <= lowWatermark <= highWatermark`
3. `maxBuffer >= highWatermark`

## 4. Streaming Loop Algorithm

### 4.1 Fill phase

1. While `len(rawRunes) < highWatermark` and `!eof`:
   1. `ReadRune()`
   2. append rune to `rawRunes`
   3. append `nextCluster` to `rawClusters`
   4. increment `nextCluster`
3. On `io.EOF`, set `eof = true`.
4. On read error, return error.

### 4.2 Shape phase

1. If `len(rawRunes) == 0`:
   1. if `eof`, return success,
   2. else continue fill loop.
2. Normalize current `rawRunes/rawClusters` according to existing normalization logic.
3. Map normalized stream to temporary `runBuffer`.
4. Execute existing pipeline:
   1. preprocess/reorder/pre-GSUB hooks,
   2. masks setup,
   3. GSUB/GPOS apply,
   4. postprocess hook.

### 4.3 Cut selection phase

Determine `cut` as number of shaped glyphs to flush:

1. If `eof`, `cut = shapedLen` (flush all).
2. Else compute candidate boundaries:
   1. cluster boundaries are baseline candidates.
3. Select first break-safe boundary where cursor `>= lowWatermark`.
4. If no such boundary:
   1. if `len(rawRunes) < maxBuffer`, return to fill phase without flush,
   2. else force progress (see 4.5).

### 4.4 Flush phase

1. Flush shaped range `[0:cut)` to sink as `GlyphRecord` (AoS).
2. Preserve current flush mode semantics:
   1. `FlushOnRunBoundary`: one contiguous write for selected prefix,
   2. `FlushOnClusterBoundary`: cluster chunking within selected prefix.
3. On sink error, return error.

### 4.5 Compact phase

1. Remove first `cut` codepoints/clusters from raw carry:
   1. `rawRunes = rawRunes[cutCodepointEquivalent:]`
   2. `rawClusters = rawClusters[cutCodepointEquivalent:]`
2. Preserve monotonic global cluster ids (do not renumber remaining tail).
3. Loop back to fill phase.

**Important note on cut equivalence:**  
The first implementation should preserve 1:1 rune<->glyph flush assumptions at cluster granularity for correctness. If edits alter length in a cycle, compact based on cluster boundary mapping, not raw index equality.

## 5. Break-Safe Boundary Model

Implement in stages:

1. **Stage 1 (mandatory):**
   1. any cluster boundary is break-safe.
2. **Stage 2 (mandatory):**
   1. if `UnsafeFlags` are populated, disallow cuts through unsafe regions.
3. **Stage 3 (optional):**
   1. add script-specific veto/approval hook for boundary safety.

This keeps first delivery simple and makes safety stricter without architectural changes.

## 6. API Surface Changes

No public API break required.

Possible additions to `ShapeOptions` (optional but recommended):

1. `HighWatermark int`
2. `LowWatermark int`
3. `MaxBuffer int`

If omitted/zero, use internal defaults.

## 7. PR Breakdown

### PR A: Streaming state and configuration

1. Introduce internal streaming state struct.
2. Add watermark config parsing/defaulting.
3. Add helper functions:
   1. `fillUntilHighWatermark(...)`
   2. `compactCarry(...)`

### PR B: Replace one-shot read with loop

1. Replace one-shot full-input read/shape flow in `Shaper.Shape`.
2. Keep shaping internals unchanged per iteration.
3. Ensure EOF empty-input semantics remain unchanged.

### PR C: Flush cut selection

1. Add `findFlushCut(...)` with low-watermark + cluster boundary logic.
2. Add forced-progress behavior at `maxBuffer`.
3. Ensure loop makes forward progress in all states.

### PR D: Break safety from unsafe flags

1. Integrate `UnsafeFlags` into boundary eligibility.
2. Ensure no cut inside unsafe spans.
3. Add tests for unsafe-flag constrained cuts.

### PR E: Hardening and docs

1. Add end-to-end streaming parity tests.
2. Add regression tests for Hebrew/Arabic/core behavior across multiple cycles.
3. Document invariants and failure modes.

## 8. Testing Strategy

Add/extend tests in `otshape/shape_flush_test.go` and new `otshape/shape_streaming_test.go`.

### 8.1 Correctness parity

1. One-shot vs streaming output parity on long mixed input.
2. Same glyph ids, positions, clusters, masks, unsafe flags.

### 8.2 Watermark mechanics

1. Fill stops at high-watermark unless EOF.
2. Flush does not occur before low-watermark except forced-progress/EOF.
3. Carry compaction preserves tail content and ordering.

### 8.3 Boundary safety

1. Cuts only at cluster boundaries.
2. Unsafe-flag protected regions are not cut through.

### 8.4 EOF behavior

1. Final partial carry is flushed completely on EOF.
2. Empty source yields no output and no error.

### 8.5 Progress guarantees

1. If no break-safe boundary exists before `maxBuffer`, forced progress happens.
2. Loop termination guaranteed for finite input.

### 8.6 Script regressions

1. Arabic:
   1. `.notdef` fallback repair still works across cycle boundaries,
   2. no regression in strict fallback policy.
2. Hebrew:
   1. reorder/compose behavior preserved with chunked execution.
3. Core Latin:
   1. GPOS pair/mark/cursive expectations unchanged.

## 9. Invariants

Must always hold and be asserted using `assert(…)` calls:

1. Raw carry arrays remain aligned (`len(rawRunes) == len(rawClusters)`).
2. Cluster ids are monotonic globally across entire stream.
3. Flush prefix is always a valid cut under break-safe policy.
4. Every loop iteration either:
   1. reads additional input, or
   2. flushes at least one cluster, or
   3. terminates.

## 10. Risks and Mitigations

1. **Length-changing substitutions can desync cut mapping.**
   1. Mitigation: cut on cluster boundaries and compact using cluster mapping, not naive index.
2. **No break-safe boundary for long spans.**
   1. Mitigation: `maxBuffer` forced-progress policy.
3. **Performance overhead from re-shaping carry each cycle.**
   1. Mitigation: acceptable first implementation; optimize later with plan caching and incremental normalization.

## 11. Acceptance Criteria

Streaming API realization is complete when:

1. `Shape` no longer requires reading full source before first output flush.
2. Output is identical to one-shot shaping for regression suites.
3. Watermark and break-safe rules are enforced by tests.
4. Arabic/Hebrew/core shaper regressions are clean.
5. Full `go test ./...` passes.

## 12. PR E Status

Implemented in repository:

1. Streaming parity tests for core shaper in `otshape/otcore/streaming_parity_test.go`.
2. Streaming parity tests for Hebrew shaper in `otshape/othebrew/streaming_parity_test.go`.
3. Streaming parity tests for Arabic shaper in `otshape/otarabic/streaming_parity_test.go`.
4. Test runs include both `go test ./otshape/...` and full `go test ./...`.

Scope note:

1. Current parity fixtures for Arabic/Hebrew focus on stable multi-cycle behavior.
2. Boundary veto hooks for script-specific context cuts remain a later enhancement path.
