# Feature Switch On/Off

## Feature-on/off Switch for Runs of Input-Runes

Currently, the feature-on/off switch is implemented as a run of input, indexed in terms
of runBuffer positions.

### In-Band vs Out-of-Band

We have to change the semantics here, as this is not a sensible approach for the streaming interface.
First we'll have to decide how the feature-on/off information is passed from the client
to the shaping-pipeline. The client has their own notion of an index into the rune-sequence,
currently not shared with the pipeline. For now, I'd propose to not make the client pass this
index information, as that would defy the idea behind a streaming interface somewhat. The
features-on/off can either be passed in-band or out-of-band. For streaming, in-band would feel
more natural. Currently the rune source only allows for reading runes. I am unsure of the
best way to extend the interface (or introduce an additional interface) for passing
feature-on/off information.

### Taking Advantage of Run-Nesting

Switching features on or off should be viewed as beining nested: A client may switch on “small caps” 
for a run of text (e.g., for an acronym), but after the run the previous feature-set will be restored.
This is a restriction we will impose on the input-sequence. It is reasonable when thinking about
handling of text-properties in word processors. Therefore, we will be using a more strict model
than Harfbuzz.

Nesting should give us the opportunity to manage a stack of active feature-sets. A nested run of
input may be associated with a feature-set, and the features-set (or the whole plan including it)
will be put on a stack. When the on-switch for “small caps” (`smcp`) is sent by the client, a new
feature-set including `smcp` will be created and pushed onto the stack.

Ending the application of a nested feature-set is signaled by the client in the form of
“pop the current feature-set”, not as the inverse of the activation signal. For the `smcp` example:
The client sends an (in-band) `+ smcp` signal, then the acronym, and then a “pop” signal,
not a `- smcp` signal. The feature-set will be popped from the stack and the previous
feature-set will be restored.

### API Options

The API-descision about feature-on/off runs of input is crucial and *very* important. I see the
following options, each posing different trade-offs:

1. **In-Band “special” runes**: Some runes are mis-used as special on/off markers. There are
  enough unused Unicode code-points (or used for very strange pseudo-scripts like “Klingon”) which
  lend themselves to this purpose. *Trade-offs*:
    1.  Does not feel “clean” in terms of interface-design.
    2.  Not very explicit in terms of intent.
    3.  May lead to confusion with other special runes.
    4.  Very simple
3. **Move to a `bufio.Scanner` interface**: The scanner interface would allow for natural 
  breaking up the input sequence into runs. A custom split function would manage to capture
  runs with the same set of features. *Trade-offs*:
    1.  Looses fine control over error-handling. The documentation for `bufio.Scanner` says:
        > Programs that need more control over error handling or large tokens, or must run sequential scans on a reader, should use bufio.Reader instead. 
    2.  Prevents clients from simply providing a bufio.reader wrapped about their text source.
    3.  Maybe awkward to align with runBuffer semantics (not sure about this one)
4. **Extend the `ReadRune` contract**: Add a field to the input interface to hold on/off markers.
  Instead of reading just a rune, a small struct could be read in, consisting of a rune and a
  marker. *Trade-offs*:
    1.  Feels like a “clean” solution in terms of design
    2.  Prevents clients from simply providing a bufio.reader wrapped about their text source.
    3.  Communicates intent explicitly.
    4.  Straightforward to implement.

To preserve the idea that clients should be able to provide a `bufio.Reader`, we could go
with 2 variants of the top-level API, one for `bufio.Reader` and one with an extended
Reader type. Clients with unsophisticated text shaping requirements could just use a
`bufio.Reader`, while clients with more sophisticated requirements could use the extended
Reader type.

### Feedback Summary

The section is strong. Main feedback:

1. **Special runes** should be ruled out.
   1. It pollutes Unicode semantics.
   2. Private-use code points may be valid text content.
   3. It complicates normalization/cmap/fallback handling.
2. **`bufio.Scanner`** is not a good primary abstraction for shaping streams.
   1. Scanner token semantics and limits are a poor fit for shaping.
   2. It weakens low-level control that `ReadRune` currently gives.
   3. It awkwardly pushes shaping boundaries into split policy.
3. **Extended read contract with explicit events** is the best direction.
   1. Explicit intent.
   2. Deterministic sequencing for streaming.
   3. Fits the proposed push/pop nesting model naturally.

Recommended design:

1. Keep current rune-only API for simple clients.
2. Add an event-based input interface for advanced clients.
3. Normalize both inputs internally to one event stream.

Concrete model:

```go
type InputEventSource interface {
    ReadEvent() (InputEvent, error)
}

type InputEvent struct {
    Kind InputEventKind // Rune, PushFeatures, PopFeatures
    Rune rune           // when Kind == Rune
    Size int            // optional: source byte width, parity with ReadRune
    Push []FeatureSetting // when Kind == PushFeatures
}
```

`FeatureSetting` should be explicit (`Tag`, `Value`, `Enabled`) so pushed frames can carry deltas clearly.

Behavioral constraints to define up front:

1. `Push` / `Pop` are boundary events between runes.
2. `Pop` underflow is an error.
3. EOF with unclosed stack should be either strict error (recommended) or explicit auto-pop policy.
4. Feature-change boundaries must force shaping boundaries (no lookup crossing boundary).

Overall recommendation:

Choose option 4 (extended contract) plus a compatibility adapter for plain `RuneSource`.

## Decisions

### Input API

Decision:

1. We will implement API option 4: an extended input contract with explicit feature-control events.
2. We will keep compatibility for simple clients by providing an adapter from plain `RuneSource`.

Rationale:

1. Feature intent is explicit and unambiguous.
2. The model is stream-friendly and does not depend on client-side absolute indices.
3. It supports strict push/pop nesting semantics naturally.
4. Existing rune-only clients remain supported.

### Nested Feature-State Management

Decision:

1. We will manage our own explicit stack for nested feature states (feature sets / derived plans).
2. We will not use recursive calls and the Go call stack as the primary nesting mechanism.

Rationale:

1. Streaming control flow is iterative by design; an explicit stack integrates better.
2. It gives direct control over depth limits, validation, and underflow handling.
3. It avoids risks from unbounded recursion depth and absence of tail-call optimization.
4. It is easier to inspect/debug current active feature state during shaping.

Note:

1. Recursion remains conceptually valid for nested scope semantics and can still be useful in localized helpers.
2. For the main shaping pipeline, explicit stack management is the chosen strategy.

### Contextual Continuity and Plan Fences

Decision:

1. We keep a single shared `runBuffer` for the pipeline.
2. We treat plan push/pop boundaries as **semantic fences** for contextual shaping.
3. We do not force hard physical buffer boundaries for these fences in this phase.
4. Cross-fence continuity handling is deferred; for now, semantic strictness is preferred over complexity.

Rationale:

1. Single-buffer processing keeps implementation simple and avoids avoidable copying/splitting complexity.
2. Semantic fences provide explicit and strict behavior when plan-local strategy differs.
3. This avoids introducing fragile revalidation/fixup logic early in development.
4. It aligns with project priorities: strict guardrails and correctness-first semantics over optimization.

## Boundary Semantics Contract

### Definitions

1. **FeatureRange boundary**: index-based on/off/value changes handled through masks inside one compiled plan.
2. **Plan-fence boundary**: push/pop transitions where nested feature state selects a different compiled plan.

### FeatureRange Boundaries (single-plan semantics)

Guarantees:

1. A single compiled plan is used for the full shaped span.
2. Lookup inventory, stage ordering, shaper plan-state, and validation policy are stable across the span.
3. Feature switching is represented by per-index masks only.

Non-guarantees:

1. Full contextual continuity is not promised where masks disable lookups in one sub-range and enable them in a neighbor range.
2. No retroactive fixup pass is guaranteed for earlier decisions affected by later masked regions.

### Plan-Fence Boundaries (cross-plan semantics)

Guarantees:

1. The pipeline keeps one physical `runBuffer`.
2. Fence transitions are explicit and strict in semantics.
3. Each fenced region is shaped under its active plan only.

Non-guarantees:

1. Contextual continuity across fences is not guaranteed in this phase.
2. No cross-fence revalidation/fixpoint behavior is performed.
3. No overlap-window repair strategy is implemented yet.

### Error and Validation Rules

1. Push/pop underflow is an error.
2. EOF with an unclosed push/pop stack follows strict policy unless explicitly relaxed later.
3. Any future “continuity mode” must be opt-in and documented separately from strict mode.

### Future Extension Point

1. A later optimization/correctness extension may introduce cross-fence overlap re-shaping (small repair windows), without changing strict-mode default semantics.

## Danger of Violating Contextual Continuity at Plan-Boundaries

**This section is outdated, but kept for making our decisions traceable.**

Yes, there are clear cases where continuity conflicts with plan boundaries.

`Coverage` alone cannot solve this:
1. `Coverage` only says “this lookup applies to these glyphs” inside one lookup execution.
2. Contextual lookups need neighboring glyphs in the same shaping buffer.
3. If you split into separate plans/runs, those neighbors are missing, so lookups cannot see across the boundary.

**Remarks**:
My intention has never been to associate separate plans with separate runs. Input will always be a sequence of input runes/glyphs in a runBuffer. *On/off*-switches of features are in-band for the event interface, but will always have to be converted explict properties of runs of input-glyphs in the `runBuffer`. However, that should not trigger any kind of buffer-operation to force a hard boundary betwen runs. My preferred model would be to have a certain amount of glyph-context available wherever the cursor is in the runBuffer (except at start of text, EOF, font-change, script-change, bidi-change etc.)

#### Cases where this breaks:

1. Arabic joining forms at boundary
- Last letter of left run and first letter of right run may need each other to become `medi/fina/init`.
- If shaped separately, they can degrade to `isol/fina` incorrectly.

2. Arabic contextual substitutions (`rlig/calt/rclt`) across boundary
- Rules like Lam-Alef-style contextual behavior or other multi-glyph contexts fail if pair is split across plans.

3. Cursive attachment (`GPOS curs`) across boundary
- Connection points require adjacent glyph context.
- Split plans can break joining/attachment.

4. Pair kerning across boundary
- Simple but common: last glyph of run A with first glyph of run B won’t kern if shaped independently.

5. Mark attachment across boundary (`mark/mkmk`)
- If base and combining mark land in different plan runs, attachment can fail or change.

6. Normalization/composition edge effects
- Composition opportunities at boundary (including script-specific compose hooks) can be lost when split.

**Remarks**:

I acknowledge most of those points (I am not in a situation to argue about Arabic shaping). I have no overview if there is a need for having an explicit boundary contract, but I will follow your advice on this one.

### Implications

Practical implication for your strict design:
1. If plan boundaries are hard boundaries, continuity is intentionally not guaranteed there.
2. That is valid, but must be an explicit API contract.
3. If client needs continuity, it must avoid push/pop inside shaping-sensitive spans.

**Remarks**:
Push/pop is for plans only, not necessarily affecting the runBuffer contents. Please explain why it should be difficult for the pipeline to handle this scenario:  
- The cursor is at the last input-glyph of a `smcp` run
- A lookup to be processed wants to do a coverage match with the next glyph
- The next glyph is to be processed with a new plan, which is not yet instantiated as the cursor is not there yet
- The lookup's coverage checks the next glyph as is in the runBuffer, without caring about the feature-set change yet
- The lookup is performed if a match occurs

Strong guardrail option:
1. Treat feature push/pop as “must be break-safe boundary”.
2. If boundary is inside unsafe/context-sensitive region, return an error (strict mode) instead of silently degrading.

That keeps semantics simple and strict, and avoids Harfbuzz-level complexity.
