# OT-Tools development guidance

This supplements the repository-root [AGENTS.md](../AGENTS.md). Read both.
Keep this file with the tooling as its implementation evolves; update its source
map after extracting code into another package.

## Intent and working agreement

`ot-tools` is a practical proof and diagnostics application for this repository's
OpenType shaper. The CLI is an intermediate step toward a local interactive
text-shaping workbench. Its purpose is to connect visible glyph placement,
numerical shaping output, and original input text in repeatable experiments.

The user has delegated routine tool design and implementation decisions to the
agent. Work in focused, reviewable milestones. Explain the intended approach,
then proceed within the agreed scope; ask about material scope changes or
unresolved product decisions rather than repeatedly asking permission for
routine edits. The root agreement still governs fixes to shaping-engine findings:
do not silently expand a tooling task into a core shaping fix.

- The user performs **all commits and pushes**. Never commit or push.
- Inspect the working tree and preserve existing work. The CLI implementation
  was still modified/untracked at this handoff; it may not be present in HEAD.
  Verify current state rather than resetting or replacing it.
- Prioritize single-run Latin and Hebrew. Arabic registration is not a promise
  of supported interactive Arabic shaping.
- **Do not implement HarfBuzz comparison functionality.** The user explicitly
  excluded it. Preserve the existing offline HarfBuzz regression tests and their
  independent expectations; that is a separate concern.

## Read first

- [OT_TOOLS.md](../doc/OT_TOOLS.md): current commands, case/report formats,
  rendering conventions, limits, and runnable examples.
- [SHAPER_INTERFACES.md](../doc/SHAPER_INTERFACES.md): shaping interfaces and
  intentional compatibility conventions.
- [STREAMING_API.md](../doc/STREAMING_API.md): output/buffering/error contracts.
- [TESTING.md](../doc/TESTING.md): independent expectations and fixture policy.
- [SHAPING_FINDINGS.md](../doc/SHAPING_FINDINGS.md): consult when a result suggests
  a shaping defect; resolved/deferred findings are not automatically reopened.

## Current implementation (2026-11-02)

This directory is `package main`, building one executable with five commands.
Reusable case execution and rendering live in `internal/proof`:

| Command | Implemented behavior |
| --- | --- |
| `shape` | Logical glyph stream with IDs, scalar clusters, advances and offsets. |
| `view` | Full-run PNG by default; `--index` isolates one logical glyph. Automatic canvas sizing and optional diagnostics. |
| `font` | Font identity, table information and parser diagnostics. |
| `proof` | Executes a saved JSON case with labeled feature variants, producing a comparison PNG, glyph report, and replayable case snapshot. |
| `serve` | Loopback browser workbench for cases, live comparisons, glyph/cluster selection and downloads. |

Shared shaping controls include Latin/Hebrew presets with explicit overrides,
global and scalar-ranged features, codepoints, file/stdin input, and buffer
options. Input whitespace and normalization form are preserved. There is no
paragraph layout or automatic script/direction detection.

`--diagnostics` enables bounding boxes, baseline, positioned origins, pen
positions, advance spans/values, glyph labels, and cluster coloring. Individual
switches are also available. PNGs are unhinted. Captions use an ASCII diagnostic
font; Unicode caption text is escaped, while sample text uses actual glyph
outlines from the selected font.

Saved cases are version 1 and contain one font/text/run configuration plus
variants. The schema and validation live in `internal/proof/case.go`. Relative font and
text paths resolve against the case file. Snapshots embed the input text, resolve
presets, and record an absolute font path and SHA-256. The font remains external.
Reports also record build metadata when available. Replays preserve inputs and
settings, not a frozen shaper: buffer zeros still use library defaults and random
alternate choices are not reproducible across requests.

Example inputs, which must remain runnable:

- [latin-features.json](../testdata/proof-cases/latin-features.json)
- [hebrew-marks.json](../testdata/proof-cases/hebrew-marks.json)

The browser workbench uses the same execution and comparison renderer as the
CLI. Server-derived geometry drives hit testing and guides; source spans remain
cluster-level. See OT_TOOLS.md for startup, limits and manual verification.

## Source map

| File | Responsibility |
| --- | --- |
| `tools.go` | Command registration, loading/helpers, CLI feature-argument adapter. |
| `shape_cmd.go`, `input.go` | CLI flags, file/stdin input and numerical formatting. |
| `view_cmd.go`, `proof.go` | CLI view orchestration and PNG file output. |
| `case.go` | Frontend path resolution/input reads and CLI report/snapshot output. |
| `serve_cmd.go` | Loopback HTTP server, session/origin/host checks, bounded reads and cancellation. |
| `workbench/` | Embedded HTML/CSS/JS; controls, hit selection, original-source/UTF-16 mapping, revision-gated updates. |
| `../internal/proof/` | Case schema, validation, fingerprinting, shaper assembly, feature parsing, reports, shared comparison geometry and rasterization. No filesystem/network I/O or exits. |
| `font_cmd.go` | Font/table diagnostics. |
| `*_test.go`, `../internal/proof/*_test.go`, `workbench/*_test.mjs` | CLI/HTTP parity, geometry/oracles, validation, scalar mapping, source retention and stale-response tests. |

The CLI dependency `commando`/`clapper` has quirks. `featureCLIArgs` protects
`--features=liga=0` and negative feature expressions from incorrect token splitting.
Optional empty string flags have their `IsRequired` metadata adjusted explicitly.
Positional variadic text parts are comma-joined by that library; quoted input or
file/stdin avoids this. Preserve CLI tests when changing parsing or registration.

## Contracts to preserve

- Render final **glyph IDs**, not browser/OS-shaped text. `sfnt.LoadGlyph` and
  `vector.Rasterizer` currently provide the outlines and rasterization. Copy
  outline segments before reusing the SFNT scratch buffer.
- Public advances already include nominal font advances. Never add them again.
  OpenType Y is upward; image Y is downward.
- Sink/report arrays stay in logical shaping order. Horizontal RTL display
  reverses the entire record sequence, including records sharing a cluster,
  preserving every record's positions. Do not sort by cluster.
- Clusters and feature ranges use original Unicode scalar indexes, not UTF-8
  bytes, normalized string positions or JavaScript UTF-16 code units. Multiple
  glyphs may share a cluster; a ligature may cover several input scalars.
- CLI `tag=0` disables; bare tags and `+tag` enable value one. Positive alternate
  values are one-based; negatives request random selection. The library's separate
  enabled/default-zero form is not represented by this CLI syntax.
- Comparison rows share scale and plot extents, anchored to the logical start:
  left advance edge for LTR, right advance edge for RTL. Do not independently
  center rows and conceal advance differences.
- JSON glyph reports use font units with Y upward. Pen/origin coordinates are
  measured from the visual run's initial pen, before canvas alignment. They are
  observed results, **not independent test oracles**.
- Retain explicit failures for unsupported input, shaping/outline errors and
  resource limits. No unsafe flushes, skipped glyphs or silently clipped proofs.
  Current tool limits: 1–4096 ppem, 16,777,216 image pixels and 1–64 variants.

## Workbench maintenance contracts

Keep saved version-1 cases CLI-compatible. Both frontends must call the shared
`internal/proof` implementation. Process exits, flags, HTTP and filesystem I/O
stay at the edges. Bind only to loopback and retain the session/origin/Host checks.
Local paths are explicitly chosen inputs; outputs use browser downloads. Do not
add directory browsing, arbitrary file serving or a frontend build dependency.
Exports must use the displayed execution without rerunning random choices.
Invalidate response revisions immediately on edits, before debounce, and gate
both success and error responses. Automatic refresh must not move the text caret.

Selection must connect the preview, glyph data and original text. Expose geometry
from the Go layout instead of duplicating placement rules in JavaScript. Handle
marks, overlaps and repeated clusters; the glyph list should offer an unambiguous
selection route. Highlight cluster-level source correspondence rather than
claiming a one-glyph/one-character mapping. Account explicitly for Unicode scalar
versus UTF-16 indexing and for stale asynchronous preview responses.

Outside this milestone: mixed-script paragraphs, line breaking, font fallback,
new script engines, detailed lookup tracing, event-input editing, cloud hosting,
and HarfBuzz comparison.

## Verification and handoff

From the repository root:

```sh
go test ./ot-tools ./internal/proof -count=1
go test -race ./ot-tools ./internal/proof -count=1
go vet ./...
node --test ot-tools/workbench/*_test.mjs
go test ./... -count=1
go build -o /tmp/ot-tools ./ot-tools
/tmp/ot-tools proof testdata/proof-cases/latin-features.json --output /tmp/latin-comparison
/tmp/ot-tools proof testdata/proof-cases/hebrew-marks.json --output /tmp/hebrew-comparison
```

At the last implementation handoff these checks passed; this is historical
evidence, not a substitute for checking the new session's checkout. Both example
proofs were visually inspected and a saved Latin snapshot replayed to identical
PNG bytes and glyph rows. Existing tests also use seven unchanged HarfBuzz 15
captures at two scales. They validate placement, not an independent rasterizer.

Run the shared package and frontend checks together. Retain focused coverage for
interactive selection, scalar-index conversion, request/error handling and
CLI/UI consistency. Exercise the actual browser workflow, including
pointed Hebrew and combining Latin marks. Update usage documentation, report
checks actually run, and hand back a reviewable milestone without committing.
