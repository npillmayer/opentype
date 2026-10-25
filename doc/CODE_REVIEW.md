# Code Review: ot/ and otlayout/

Updated: 2026-10-25

Scope: unresolved findings in `ot/` and `otlayout/`, with supporting test and build-maintenance concerns. Locations refer to the current source. Severity distinguishes observable correctness issues from maintenance opportunities and deferred functionality.

## Medium: inconsistent nil handling in layout APIs

Locations: `otlayout/feature.go:183`, `otlayout/layout.go:128`.

`ApplyFeature` dereferences `st` for a nil feature before checking whether `st` itself is nil:

```go
if feat == nil {
    return st.Index, false
} else if st == nil || st.Glyphs == nil {
    // ...
}
```

Consequently, passing both nil panics despite the surrounding handling of unusable buffer state. The function also assumes that the font and appropriate layout table exist when extracting `AsGSub()` or `AsGPos()`. That table precondition is documented, but should be considered alongside the overall argument policy.

`peekLayoutProperty`, described as extracting a property safely, calls `table.Self()` without checking for a nil table.

Suggested work:

- Check buffer-state validity before dereferencing it, including the nil-feature path.
- Decide and consistently document font/table preconditions, or return a controlled failure for invalid inputs.
- Test nil feature plus nil state, nil table in layout accessors, and missing layout tables according to the chosen contract.

These are API robustness concerns; they do not by themselves demonstrate a font-data parsing vulnerability.

## Low: cmap factory retains an invariant panic

Location: `ot/cmap.go:118`, `makeGlyphIndex`.

```go
panic("unreachable") // unsupported formats should have been weeded out beforehand
```

The factory already returns an error but panics for a format outside its supported switch. Existing format selection is intended to prevent this path, so direct reachability from malformed font input has not been established.

Suggested work: return a contextual unsupported-format error and test the factory with an unsupported format. Treat this as defensive hardening, not a demonstrated high-severity crash path.

## Low: glyph-count configuration depends on concrete cmap implementations

Location: `ot/parse.go:258`.

```go
switch gim := otf.CMap.GlyphIndexMap.(type) {
case format4GlyphIndex:
    gim.numGlyphs = maxp.NumGlyphs
    otf.CMap.GlyphIndexMap = gim
case format12GlyphIndex:
    gim.numGlyphs = maxp.NumGlyphs
    otf.CMap.GlyphIndexMap = gim
}
```

Cross-table initialization knows every concrete cmap representation. A new implementation can omit glyph-count validation unless this switch is also updated.

Suggested work: centralize glyph-count configuration through an internal capability or construction step. Keep this concern internal rather than adding unrelated methods to the public lookup interface. Test that every supported cmap implementation receives the count.

## Low: duplicate layout-table parsing orchestration

Locations: `ot/parse_gsubgpos.go:4`, `ot/parse_gsubgpos.go:24`.

The GSUB and GPOS wrappers repeat the same sequence:

```go
err = parseLayoutHeader(...)
err = parseLookupList(...)
err = parseFeatureList(...)
err = parseScriptList(...)
```

This increases the chance of validation or parse-order changes diverging between the two families.

Suggested shared helper: accept the wrapper's existing embedded `*LayoutTable`, binary data, table tag, lookup family, and error collector. Keep concrete table construction in the wrappers. Preserve parse order, error short-circuiting, and table-specific diagnostic context. Avoid allocating a second layout object merely to copy it back.

## Low: duplicate chained-rule sequencing

Locations: `ot/parse_lookup_gsub.go:735`, `ot/parse_lookup_gsub.go:768`.

`parseGSubChainedSequenceRule` and `parseGSubChainedClassRule` repeat the backtrack/input/lookahead/lookup-record sequence. They already delegate individual reads to shared helpers, so the remaining duplication is orchestration rather than a separate demonstrated bounds bug.

Suggested shared helper: use typed internal rule parts or a narrowly constrained generic reader to share the sequence while preserving glyph/class distinctions. Preserve the input-count-minus-one convention, failure behavior, and typed results. Avoid an `interface{}` callback that would introduce new runtime assertions. Refactor only if it makes the existing code easier to follow.

## Low: commented-out matching implementations obscure active code

Locations: `otlayout/feature.go:769`, `otlayout/feature.go:797`, `otlayout/feature.go:905`, `otlayout/feature.go:933`, `otlayout/feature.go:991`, `otlayout/feature.go:1017`.

Directional matching wrappers contain substantial commented-out function bodies after their active return statements. These make a complex execution path harder to read and leave alternative implementations in the source without an active purpose.

Suggested work: remove the dead commented-out bodies. Keep documentation explaining the direction and starting-offset conventions where useful.

## Deferred: JSTF requirement validation

Location: `ot/parse.go:312`.

The source explicitly defers applying lookup-flag/GDEF requirement checks to JSTF until JSTF parsing is enabled.

Suggested work: include these checks in the acceptance criteria for future JSTF support. This is a deferred capability requirement, not a failure of the currently enabled GSUB/GPOS validation path.

## Supporting test and build concerns

### Font-golden tests can skip malformed subtables

Locations: `ot/parse_concrete_golden_fonts_test.go:317`, `ot/parse_concrete_golden_fonts_test.go:369`.

These tests count and skip subtables reporting parse errors. A passing suite therefore does not guarantee that every fixture subtable was successfully parsed and compared.

Suggested work: distinguish explicitly expected malformed fixtures from unexpected parser failures, and fail on newly introduced skips or maintain a reviewed expectation list.

### Absolute local module replacement

Location: `go.mod:7`.

```go
replace github.com/go-text/typesetting => /Users/npi/prg/go/extern/typesetting
```

The replacement ties resolution of this module to a developer-local path if the dependency is used. Its presence alone has not prevented the current test suite from running.

Suggested work: remove the replacement if unused, or move local development overrides to a workspace configuration while keeping shared module configuration portable.
