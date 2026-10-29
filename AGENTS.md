# Repository guidance for coding agents

## Working agreement

- The user performs all git commits and pushes. Do not commit or push.
- Inspect the working tree before editing and preserve unrelated changes.
- For the interactive shaping-fix workflow, tackle one user-selected finding at
  a time. Explain the proposed fix and any unresolved contract decisions before
  editing; wait for agreement unless the user has already authorized that scope.
  After implementation and verification, report the result and wait for review
  before moving to another finding. Discuss blocking dependencies rather than
  silently expanding scope.
- Prioritize shared correctness, Latin, and Hebrew. Arabic/Syriac and other
  script-specific extensions are deferred unless explicitly requested.
- The initial testing framework is in place. Add coverage needed to verify the
  selected work; do not launch an unrelated broad test-expansion effort.

## Project and architecture

This Go module parses OpenType fonts, executes layout lookups, and shapes text.
The pipeline is implemented but has known correctness gaps; lookup-type coverage
must not be presented as complete shaping conformance.

| Location | Responsibility |
| --- | --- |
| [opentype.go](opentype.go) | Root convenience API for font loading, names, and shaping. |
| [ot/](ot/) | Font parsing, typed table access, and concrete GSUB/GPOS layout graphs. |
| [otquery/](otquery/) | Cmap, names, glyph/font metrics, and script/layout queries. |
| [otlayout/](otlayout/) | GSUB/GPOS lookup execution, glyph edits, positioning, and execution limits. |
| [otshape/](otshape/) | Public `Shape`/`ShapeEvents` APIs, plans, normalization, masks, streaming, and output. |
| [otshape/otcore/](otshape/otcore/) | Generic shaping engine and public-API integration tests. |
| [otshape/othebrew/](otshape/othebrew/), [otshape/otarabic/](otshape/otarabic/) | Script-specific engines and tests. |
| [internal/ttxtest/](internal/ttxtest/) | TTX XML expectations for parser comparisons; not a shaping oracle. |
| [internal/hbcmp/](internal/hbcmp/) | Offline comparisons against independent HarfBuzz output, including the streaming/event matrix. |
| [internal/fontload/](internal/fontload/) | Shared font-byte and SFNT loading. |
| [testdata/](testdata/) | Checked-in fonts, TTX dumps, upstream cases, provenance, licenses, and preparation scripts. |
| [ot-tools/](ot-tools/) | CLI with `shape`, `view`, and `font` commands; see [OT_TOOLS.md](doc/OT_TOOLS.md). |

The main shaping dependency path is `otshape` → `otlayout` → `ot`, with
`otquery` supplying mapping and metrics. Concrete engines plug into `otshape`.
Keep lower-level packages independent of the shaping engines.

## Read before shaping work

- [TESTING.md](doc/TESTING.md): test architecture, fixture workflows, coverage,
  and priorities.
- [SHAPING_FINDINGS.md](doc/SHAPING_FINDINGS.md): stable `SHAPE-NNN` findings,
  reproductions, decisions, and fix/promotion process.
- [STREAMING_API.md](doc/STREAMING_API.md): current buffering behavior and the
  agreed, not yet fully implemented buffer contract.
- [SHAPER_INTERFACES.md](doc/SHAPER_INTERFACES.md) and
  [SHAPING_OVERVIEW.md](doc/SHAPING_OVERVIEW.md): interfaces and pipeline overview.
- [PARTIAL_PROGRESS.md](doc/PARTIAL_PROGRESS.md): lookup edits, progress, and
  error propagation.
- [TO_DO.md](doc/TO_DO.md): broader roadmap. Keep detailed bug evidence in the
  findings file rather than duplicating it here or in the roadmap.

Documents may describe historical snapshots or intended behavior. Confirm
implementation claims against source and tests; distinguish an agreed contract
from a guarantee that the current code actually provides.

### Agreed shaping contracts

- The sink emits logical shaping order, including RTL. Visual ordering belongs
  to the caller.
- Clusters follow HarfBuzz level-0 grouping, anchored to zero-based original
  Unicode scalar indexes, not UTF-8 byte offsets. Preserve source correspondence
  through normalization, substitution, and reordering.
- SHAPE-003: positive alternate values are one-based; zero selects the default/
  first alternate; negative values request random selection; out-of-range
  positive values leave the glyph unchanged by that alternate substitution.
  Public feature values and lower-level `ApplyFeature` must agree. See the
  finding for regression coverage and the subsequent SHAPE-018 forward-start fix.
- Buffering may require a generous minimum capacity and practical sequence/
  context limits. Prefer simpler safe-boundary logic over supporting tiny
  buffers. Emit only final output; reject invalid capacity or fail explicitly
  when safe progress cannot fit within a hard limit. Exact limits and error
  semantics remain to be agreed. See the buffer contract for test implications;
  do not silently weaken existing assertions.

## Parsing and implementation guidance

The parser entry point is `ot.Parse(font []byte, options ...ot.ParseOption)
(*ot.Font, error)` in [ot/parse.go](ot/parse.go). Parsed fonts retain references
to the original bytes; keep them immutable for the font's lifetime.

Normal parsing requires the library's layout-enabled subset with both GSUB and
GPOS. GDEF is required when lookup flags need its data. `ot.IsTestfont` relaxes
completeness/consistency checks for appropriate fixtures; do not use it to hide
normal-parser limitations in end-to-end baselines. There is no legacy `kern`
fallback. Required-table policy lives in `ot/parse.go`; do not infer it from
which tables have parser implementations.

The public table interface offers `Binary`, `Extent`, and `Self` with typed
accessors. Layout traversal uses concrete semantic structures such as
`ScriptList`, `Script`, `LangSys`, `FeatureList`, `LookupTable`, and `LookupNode`,
with lazy parsing/caching where implemented. Do not reintroduce the removed
`Navigator`/`NavMap`/`NavList`/`NavLink` API or obsolete `.Fields()` examples.

Useful implementation entry points:

- [ot/parse_gsubgpos.go](ot/parse_gsubgpos.go),
  [ot/parse_lookup_gsub.go](ot/parse_lookup_gsub.go), and
  [ot/parse_lookup_gpos.go](ot/parse_lookup_gpos.go): layout parsing.
- [ot/layout_scripts_features.go](ot/layout_scripts_features.go) and
  [ot/lookup.go](ot/lookup.go): semantic layout graph.
- [ot/bytes.go](ot/bytes.go): internal binary access. Use checked arithmetic and
  bounds-checked access for untrusted offsets/counts; propagate errors.
- [ot/errors.go](ot/errors.go): parse diagnostics. Check returned errors and
  available `Font.Errors()`, `Warnings()`, and `CriticalErrors()`. Lazy-access
  errors also require attention; see [ERROR_HANDLING.md](doc/ERROR_HANDLING.md)
  and [TODO_ERROR_COLLECTION.md](doc/TODO_ERROR_COLLECTION.md).

Preserve parsing bounds checks, recursion/execution limits, and source/glyph
provenance. Variable-font processing, TTC loading, color-glyph rendering, and
vertical metrics are outside current supported functionality; raw table access
alone does not establish semantic support.

## Testing and fixing findings

1. Reproduce the selected finding and identify the expected behavior and its
   independent basis. Use lower-level tests to localize the issue and public-API
   tests to verify integration.
2. Preserve independent expectations. Never regenerate an oracle from this
   shaper, assert known buggy output, or ignore mismatched fields to make a
   regression pass.
3. Keep unresolved desired-behavior tests under `shaping_knownbugs`; the ordinary
   suite must remain green. After a fix, promote passing cases and their helpers
   into the ordinary suite. Retain full expectations for cases with remaining
   blockers and document those blockers. Unexpected tagged passes need review
   and promotion, not a silent skip.
4. Update the finding's status, evidence, and resolution while retaining its ID.
   Update affected contract/testing documentation when behavior changes. Keep
   deferred scope decisions distinct from regressions.

TTX tests validate parsed structures. HarfBuzz captures validate shaped output;
comparisons preserve the agreed logical-order adaptation for RTL. Font hashes,
capture commands, licenses, and provenance belong with the fixtures. Go baseline
runs use checked-in data and need no HarfBuzz or Python process. HarfBuzz and
FontTools are preparation dependencies only; consult each fixture README before
recapturing or rebuilding. Missing fixtures must fail rather than silently skip.

## Development and verification

[go.mod](go.mod) declares Go 1.24.0 and toolchain go1.24.11. It is the source of
truth for dependencies, including `x/image`, `x/text`, `schuko`, `commando`, and
`testify`. A local `go-text/typesetting` replacement is present; do not rewrite
it or run dependency cleanup as an unrelated change.

Run from the repository root:

```sh
# Ordinary regressions; must pass.
go test ./... -count=1

# Focused shaping and lookup tests.
go test ./otshape/... ./otlayout ./internal/hbcmp -count=1

# Discovery suite; expected to fail while documented findings remain open.
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1 -timeout=30s

# One finding; change the ID and packages as appropriate.
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE015' -count=1 -timeout=30s -v
```

Format changed Go files with `gofmt`. Run focused tests during development and
broader ordinary tests before handing back a fix. Use `go build ./...`,
`go vet ./...`, and `go test -race ./...` as appropriate to the change. Separate
expected discovery assertion failures from build, fixture, panic, or timeout
failures. Report checks actually run and any limitations.
