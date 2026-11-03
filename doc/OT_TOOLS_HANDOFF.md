# Prompt for the next Codex session

Copy the following prompt into a new session working in this repository:

```text
We are developing a practical text-shaping proof tool in
/Users/npi/prg/go/opentype. The CLI is implemented; the next milestone is a
lightweight local interactive workbench, served by Go and used in a browser.

First read:
- AGENTS.md
- ot-tools/AGENTS.md
- doc/OT_TOOLS.md
- doc/SHAPER_INTERFACES.md
- doc/STREAMING_API.md
- doc/TESTING.md

Inspect git status, the existing diff, ot-tools source/tests, and
testdata/proof-cases. The preceding session left implemented CLI changes
modified/untracked; I may have committed them since. Establish the actual
checkout state and preserve existing work. Do not assume HEAD contains the CLI
milestone, and do not reset or replace it.

The CLI currently provides:
- shape: numerical glyph output;
- view: full-run or selected-glyph PNGs, automatic sizing and diagnostic guides;
- font: font/table/parser diagnostics;
- proof: labeled feature comparisons from version-1 JSON cases, with PNG,
  numerical JSON and replayable .case.json outputs.
It supports Latin/Hebrew presets, feature ranges, codepoints, file/stdin input,
buffer controls, cluster colors, pen/origin markers and advance diagnostics.
Font hashes are checked and recorded. Consult OT_TOOLS.md for precise behavior.

Implement the following milestone:
1. Extract case execution and the reusable proof/geometry code from package main
   into a shared internal Go package. Keep the CLI working and preserve saved-case
   compatibility. Keep process exits and frontend-specific I/O at the edges.
2. Add a lightweight local browser interface, served on loopback by Go. Prefer
   simple HTML/CSS/JavaScript and existing dependencies unless there is a concrete
   reason for more infrastructure. Choose a clear CLI entry point for starting it.
3. Let me open an existing case or enter text and choose a font; edit features,
   ranges, script, language, direction and comparison variants; and see the proof
   update without rerunning shell commands. Surface validation/shaping errors
   clearly and prevent stale responses from replacing newer results.
4. Make glyph selection connect the preview, numerical glyph details and original
   source text. Show ID, cluster, advances, offsets and relevant positioning guides.
   Handle zero-advance marks and overlapping glyphs, with selection via the glyph
   list as well as the preview. Highlight the corresponding source cluster, without
   claiming an exact one-character mapping that the output does not provide.
5. Support saving/reopening cases and exporting proofs and glyph data through the
   same shared implementation used by the CLI. Retain font fingerprints and the
   existing comparison alignment conventions.
6. Add focused tests, run appropriate broader checks, exercise the actual browser
   workflow with Latin ligatures/combining marks and pointed Hebrew, and update
   documentation with runnable instructions.

Important contracts:
- Our sink emits logical shaping order, including RTL. Visual RTL rendering
  reverses the full record sequence, including records within a cluster, while
  preserving positions. Do not sort by cluster.
- Render our final glyph IDs and positions. Do not hand the sample to browser
  text shaping and mistake that result for our shaper's output.
- Advances already contain nominal font advances; do not add them again.
- Clusters and ranges use original Unicode scalar indexes. JavaScript UTF-16
  indexes require explicit conversion. Preserve original text and normalization.
- Keep cluster-level highlighting honest for ligatures, marks and merged clusters.
- The report uses font units with upward Y; screen geometry uses downward Y.
- Preserve safe-buffer/error behavior and independent regression expectations.
- Random alternate choices can vary between requests; do not promise deterministic
  replay for them or silently change the shaper's random-selection contract.

Scope is one font and one Latin/Hebrew run, including the existing comparison
variants. Do not add paragraph layout, mixed-script itemization, font fallback,
new script engines, event-input editing, detailed lookup tracing or cloud hosting.
Do not implement HarfBuzz comparison functionality: I explicitly ruled it out.
Existing offline HarfBuzz oracle tests must remain intact.

I delegate routine tool design and implementation decisions to you. Begin with
a brief assessment and concrete implementation plan, then proceed within this
scope; do not stop after planning or ask approval for every routine choice.
Discuss material product decisions or an unexpected core-shaper blocker with me
rather than silently expanding scope. Do not reopen resolved/deferred shaping
findings automatically.

I perform all git commits and pushes myself. Never commit or push. Finish with a
reviewable milestone, instructions to run it, the checks you actually performed,
and any remaining limitations.
```
