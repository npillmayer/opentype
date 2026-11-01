# Shared feature-scope context (SHAPE-016)

**Status:** SHAPE-016 is fixed and accepted; its contract and recorded
implementation verification are archived in
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md#shape-016--feature-scopes-sever-unrelated-lookup-context).
SHAPE-022 remains open with unchanged full expectations.

Prepared 2026-11-01 with HarfBuzz 15.0.0. `scopes.json` records the exact capture
commands, tool/font hashes, scalar inputs, settings and all six output fields.
Fonts are unchanged Gentium Plus and Noto Sans Hebrew fixtures; their provenance
and licenses remain in `testdata/fonts`. No existing oracle was regenerated.

Reproduce without writing:

```sh
python3 testdata/scripts/capture_harfbuzz_scopes.py
```

The preparation script requires HarfBuzz 15; Go tests use only checked-in data.
Capture settings explicitly select the OpenType shaper and font functions,
UPM-sized output, cluster level 0 and numeric glyph IDs. RTL results retain the
existing whole-sequence logical-order adaptation.

`scopes_test.go` executes each case through `Shape` ranges and independently
specified `ShapeEvents` push/pop recipes, using both flush modes with whole-input
buffers. The test-side stack checks assignments at every original scalar before
shaping. Full GIDs, clusters, offsets and advances are compared to HarfBuzz.

| Case | Purpose | Result |
| --- | --- | --- |
| `scope_liga_second_off`, `scope_liga_first_off` | Either disabled input prevents `fi` ligation | ordinary |
| `scope_kern_second_off`, `scope_kern_first_off` | Either disabled input prevents `AV` kerning | ordinary |
| `scope_unrelated_ligature` | Changing `kern` inside `fi` preserves the ligature | ordinary |
| `scope_composition` | An unrelated event inside e + acute preserves composition | ordinary |
| `scope_mark_inside_no_mkmk` | Group-start settings at both ends of a mark range | ordinary |
| `scope_implicit_restore` | A popped scope restores default ligation without explicit global `liga` | ordinary |
| `scope_nested_ligatures` | Nested overrides and pop restore enclosing assignments | ordinary |
| `scope_hebrew_group` | An event inside vav + holam preserves RTL grouping and mark placement | ordinary |
| `scope_mark_inside` | Same group-range test with default `mkmk`; exposes attachment across a base | tagged SHAPE-022 |

Eleven captures produce forty ordinary and four tagged comparisons.
`scope_mark_inside` is retained unchanged: the final accent should have `(0,0)`
offsets, but attaches across the intervening `x` and receives `(-999,360)`.
This affects both entry points independently of SHAPE-016. Its separately
captured `mkmk=0` control isolates range/group selection without weakening the
failing assertion.

The original `scope_pair` matrix capture remains in `streaming.json`: six event
comparisons are promoted unchanged, and four tiny/forced cases retain the full
expectation under SHAPE-015. The two existing normalization/range captures also
now run as equivalent events. This batch does not claim safe streaming cuts,
cursive correctness, engine precedence, or general script conformance.
