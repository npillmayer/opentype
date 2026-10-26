# Recovered HarfBuzz/AOTS shaping cases

Recovered on 2026-10-26 from [HarfBuzz commit
29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b](https://github.com/harfbuzz/harfbuzz/tree/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/test/shape/data/aots).
These files preserve the upstream source material. HarfBuzz was not built or
run during the initial recovery, and recovery itself established no shaping
pass/fail results. The subsequent [adapter batches](ADAPTER.md) now exercise
five active GSUB and nine active GPOS records: four pass in the ordinary suite,
and ten reproduce documented failures in the tagged suite. The GSUB simple
alternate record also has a failing streaming execution. See the
[GPOS report](GPOS.md) for origin comparisons and the [pair report](PAIR.md)
for matching/nonmatching pair coverage and the flagged advance-changing case.

## Scope and provenance

The original 12 fonts and two subsequently imported pair fonts in
[`testdata/fonttools`](../../fonttools/) have records in the pinned upstream
AOTS suite. All 14 local binaries are byte-for-byte
identical to the upstream fonts. The selected source files and fonts were also
checked against the blob hashes in GitHub's tree for the pinned commit.

The 13 files in [`tests/`](tests/) are complete, byte-for-byte upstream copies.
They retain comments, the `@shaper=ot` directive, original font paths, options,
inputs, expected output, and line numbers. They are registered in upstream
[`meson.build`](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/test/shape/data/aots/meson.build).
Existing fonts were not duplicated or changed; the two additions are documented
in [PAIR.md](PAIR.md). The original `../fonts/`
paths do not resolve within this snapshot; the adapter uses the
local paths recorded in the manifest.

[`manifest.json`](manifest.json) records source paths, revision, SHA-256 hashes,
font identity, and one entry per record, identified by source filename and
one-based line number. Its statuses distinguish:

- **candidate:** an active upstream record with an existing local font: 21.
- **upstream_disabled:** a commented record with an existing local font: 9.
  These are retained as evidence, not accepted expectations or known failures
  of our shaper. In particular, `gpos3_font2` has no active upstream record here.
- **font_not_in_local_subset:** 5 records retained because their source files
  also contain matching cases. Their fonts are not imported: three sibling
  `gpos_chaining3_boundary` fonts, `gsub_context1_lookupflag_f2`, and
  `gpos2_1_lookupflag_f1`.

There are 35 records in total, of which 30 reference our fonts. The `candidate`
status says nothing about whether the case is implemented, currently passes
HarfBuzz, or belongs in our first Latin/Hebrew test batch.

| Local font (without `.otf`) | Upstream test file | Active matching records | Disabled matching records |
| --- | --- | ---: | ---: |
| `classdef2_font4` | [classdef2.tests](tests/classdef2.tests) | 1 | 0 |
| `gsub3_1_simple_f1` | [gsub3_1_simple.tests](tests/gsub3_1_simple.tests) | 1 | 0 |
| `gsub3_1_lookupflag_f1` | [gsub3_1_lookupflag.tests](tests/gsub3_1_lookupflag.tests) | 1 | 0 |
| `gsub_chaining2_next_glyph_f1` | [gsub_chaining2_next_glyph.tests](tests/gsub_chaining2_next_glyph.tests) | 1 | 0 |
| `gsub_context1_lookupflag_f1` | [gsub_context1_lookupflag.tests](tests/gsub_context1_lookupflag.tests) | 1 | 0 |
| `gsub_context1_next_glyph_f1` | [gsub_context1_next_glyph.tests](tests/gsub_context1_next_glyph.tests) | 1 | 0 |
| `gpos_chaining3_boundary_f2` | [gpos_chaining3_boundary.tests](tests/gpos_chaining3_boundary.tests) | 1 | 0 |
| `gpos3_font1` | [gpos3.tests](tests/gpos3.tests) | 1 | 1 |
| `gpos3_font2` | [gpos3_lookupflag.tests](tests/gpos3_lookupflag.tests) | 0 | 2 |
| `gpos3_font3` | [gpos3.tests](tests/gpos3.tests) | 5 | 4 |
| `gpos4_simple_1` | [gpos4_simple.tests](tests/gpos4_simple.tests) | 3 | 2 |
| `gpos5_font1` | [gpos5.tests](tests/gpos5.tests) | 2 | 0 |
| `gpos2_1_simple_f1` | [gpos2_1_simple.tests](tests/gpos2_1_simple.tests) | 2 | 0 |
| `gpos2_1_lookupflag_f2` | [gpos2_1_lookupflag.tests](tests/gpos2_1_lookupflag.tests) | 1 | 0 |

## Meaning and limits of the expectations

The upstream [Python runner](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/test/shape/run-tests.py)
reads four semicolon-separated fields:

```text
fontfile;options;unicodes;glyphs_expected
```

A leading `#` disables the record. Preserve that distinction when adapting it;
do not strip the comment and silently turn it into a golden case. The AOTS
[generator](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/test/shape/data/aots/hb-aots-tester.cpp)
emits such comments when its comparison fails, but this recovery does not
establish the historical cause or current behavior of each disabled case.

The stored output is deliberately narrower than our current `hbcmp` JSON:

- All recovered records specify `--no-clusters --no-glyph-names`: numeric
  glyph IDs are available; expected clusters are not.
- GSUB records specify `--no-positions`, so they provide only the glyph sequence.
- GPOS records specify `--ned` (No Extra Data), suppressing clusters and
  advances. In this mode, `@x,y` represents a glyph origin: cumulative preceding
  advances plus that glyph's offset. It is not the raw `XOffset/YOffset` pair.
  A missing `@` represents origin `(0,0)`. See upstream
  [format options](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/util/shape-format.hh)
  and [serializer](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/src/hb-buffer-serialize.cc).
- The alternate cases use per-input feature selections such as `test[5]=2`,
  combining ranged on/off requests and alternate values. They relate directly
  to SHAPE-002/003; they are not just global alternate-selection tests.
- Inputs include control codepoints and U+0000 used by the synthetic fonts.
  Preserve Unicode scalar values exactly; do not interpret them as prose or
  remove them during fixture preparation.
- The records select `@shaper=ot` and `--single-par`, but do not explicitly set
  script, language, or direction. The AOTS generator sets Latin, English, LTR,
  and font-unit scale. Any adapter must resolve and document the context
  against the upstream runner's behavior rather than silently assuming that
  generation settings and command-line defaults are interchangeable.

Therefore these records cannot be mechanically expanded into full `hbcmp`
records by filling absent fields with zero. Adapters must either compare
only the independently supplied fields (including calculated glyph origins),
or run a pinned HarfBuzz build to obtain richer output and preserve that output
as a separate, documented derivation. Full cluster/advance/offset expectations
must never be inferred from our own shaper's output.

The [adapter](ADAPTER.md) implements glyph-only comparison for five GSUB records
and glyph-origin comparison for nine GPOS records, verifying their explicit
context against HarfBuzz. It supports global and
single-index feature requests, including alternate values and explicit
disabling. Pair format 1 now has independent matching and nonmatching coverage;
the flagged advance-changing case remains blocked by SHAPE-005. Broader feature
syntax and class-based pair positioning remain later work.

Cursive cases are recovered for completeness of the existing-font inventory;
they remain lower priority under the Latin/Hebrew scope. This synthetic AOTS
subset supplies no independent Hebrew text baseline by itself.

## Reproduce or verify recovery

From the repository root, download the pinned archive once:

```sh
curl -fL https://codeload.github.com/harfbuzz/harfbuzz/tar.gz/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b -o /tmp/opentype-harfbuzz-upstream.tar.gz
python3 testdata/scripts/recover_harfbuzz_aots.py /tmp/opentype-harfbuzz-upstream.tar.gz --check
```

Omit `--check` to regenerate the source copies and manifest. The script uses
only the Python standard library, reads specific members without extracting
the archive, verifies font equality, and fails if a local font has no matching
upstream record. It does not invoke HarfBuzz, FontTools, or our shaper. The
human-written README and full Apache license text are not generated by it.

Licensing: [`COPYING`](COPYING) is the original AOTS Adobe/Apache-2.0 notice;
[`LICENSE-2.0.txt`](LICENSE-2.0.txt) is the full license text from
<https://www.apache.org/licenses/LICENSE-2.0.txt>.
[`COPYING.harfbuzz`](COPYING.harfbuzz) preserves HarfBuzz's top-level notice and
its statement that subdirectory licenses take precedence where applicable.
