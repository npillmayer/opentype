# Real Latin and Hebrew text baselines

Captured on **2026-10-26** with installed `hb-shape (HarfBuzz) 15.0.0`.
Verified against repository source revision
`3acce4440f5a215e9752cd2f7c25430b3279994a` plus the current test changes.
The initial eleven-case batch was verified at `f1deff457223a87754f86d8e17ab133682566787`.
These are new independent captures, separate from the recovered AOTS records.
No expected value comes from the repository shaper.

The later [streaming/scope extension](STREAMING.md) adds twelve captures in
[streaming.json](streaming.json), reuses four of these cases, and tests both
public APIs across buffer profiles and flush modes. Its event/progress driver
uses the same comparison contract. The 24-case counts below refer specifically
to `baselines.json` and its whole-input runner.

## Files and execution

- [baselines.json](baselines.json) contains 24 cases, exact Unicode inputs,
  complete HarfBuzz output, font hashes, capture command arguments, and the
  version and SHA-256 of the capturing executable.
- [capture_harfbuzz_text.py](../../../../testdata/scripts/capture_harfbuzz_text.py)
  invokes HarfBuzz directly, using only Python's standard library. The default
  operation recaptures and compares without writing; `--write` explicitly
  replaces the snapshot. It rejects a different HarfBuzz version, incomplete
  output, and missing-glyph results.
- [text_baselines_test.go](../../text_baselines_test.go) validates all cases and
  font hashes, including the tagged cases, and runs ten passing comparisons.
  It reuses `fixture` parameter conversion, `glyphCollector`, and the exact
  positioned-glyph comparison from `hb_compare.go`.
- [text_knownbugs_test.go](../../text_knownbugs_test.go) runs fourteen failing cases
  in seven top-level tests under `shaping_knownbugs`. Raw expectations remain the
  original HarfBuzz output; comparison explicitly converts RTL to logical order; promote cases as their blockers are resolved.
- [Noto font provenance](../../../../testdata/fonts/NotoSansHebrew-README.md)
  pins the new Hebrew font and its OFL license. Latin uses the existing
  `GentiumPlus-R.ttf`, pinned by its hash in every corresponding record.

From the repository root:

```sh
# Offline: no HarfBuzz, Python, FontTools, network or installed fonts needed.
go test ./internal/hbcmp -run '^TestHarfBuzzTextBaselines$' -count=1 -v

# Discovery: includes both AOTS and real-text findings.
go test -tags shaping_knownbugs ./internal/hbcmp -run '^TestKnownBugSHAPE' -count=1 -v

# Preparation only: requires hb-shape 15.0.0; default is read-only verification.
python3 testdata/scripts/capture_harfbuzz_text.py
python3 testdata/scripts/capture_harfbuzz_text.py --write
```

`--hb-shape /path/to/hb-shape` selects a capture executable. A different binary
hash counts as a change even if its outputs match. The recorded executable hash
identifies the installed tool used here, not a reproducible build or a pin of
all its dynamic libraries. Review tool provenance as well as output before
refreshing the snapshot; changing the version requires an intentional update
to the script and loader. The Go tests never invoke that executable.

## Comparison contract

Every capture explicitly selects the OpenType shaper and font functions, font
units (`--font-size=upem`), script, language, direction, and cluster level 0.
`--unicodes` supplies Unicode scalars; cluster indexes are scalar indexes, not
UTF-8 byte offsets. `--single-par` keeps one run. The exact command is stored
per case. Input is unmodified: HarfBuzz uses its default normalization; our
engine uses its own default policy. Features listed in the table override
defaults; an empty list leaves defaults active.

The test runs the public `Shaper.Shape` with core and Hebrew engines registered,
real font parsing (no `IsTestfont`), and high/low/max watermarks `n+1/1/n+2`,
where `n` is the input scalar count. This keeps the input together. It asserts
the complete emitted sequence: GID, cluster, X/Y offsets and X/Y advances.
For RTL, `textLogicalOutput` copies and reverses the complete captured record
sequence, undoing HarfBuzz's final visual-order reversal. It preserves each
record's positions and clusters and never sorts by cluster. LTR is unchanged.
Missing expected fields cannot become implicit zeros. This is an explicit
contract conversion, not a correction of our output.

**Agreed contract:** logical sink order and HarfBuzz cluster-level-0 grouping,
anchored at original Unicode scalar indexes. Visual ordering belongs to the
caller. Logical order includes normalization and script-specific mark reordering;
it is not necessarily original scalar order within a cluster. See
[NORMALIZATION.md](NORMALIZATION.md) for position conventions, adapter tests,
and the thirteen added cases. These are short single-script runs, not paragraph
bidi layout, streaming cuts, events, or general Latin/Hebrew conformance.

## Results

Ten cases pass every output field; fourteen remain tagged. All calls reaching
`Shape` return nil. The NoGPOS font is rejected by the normal parser before
shaping (a documented library scope restriction). The original eleven cases
are listed below; [NORMALIZATION.md](NORMALIZATION.md) lists the thirteen additions.

| Case ID | Text / context | Features | Result |
| --- | --- | --- | --- |
| `latin_office_liga_on` | `office`, Latn/en/LTR | `liga=1` | Pass: `ffi` ligature, 4 output glyphs |
| `latin_office_liga_off` | `office`, Latn/en/LTR | `liga=0` | Pass: 6 output glyphs |
| `latin_avatar_kern_on` | `AVATAR`, Latn/en/LTR | `kern=1` | Pass: adjusted advances |
| `latin_avatar_kern_off` | `AVATAR`, Latn/en/LTR | `kern=0` | Pass: nominal advances |
| `latin_cafe_nfc` | `caf` + U+00E9, Latn/fr/LTR | defaults | Pass |
| `latin_cafe_nfd` | `cafe` + U+0301, Latn/fr/LTR | defaults | Pass: same GIDs/clusters/positions as NFC |
| `hebrew_shin` | U+05E9, Hebr/he/RTL | defaults | Pass: GID 96, cluster 0, advance 730 |
| `hebrew_shalom` | U+05E9 U+05DC U+05D5 U+05DD, Hebr/he/RTL | defaults | Pass in logical order; SHAPE-009 resolved |
| `hebrew_shin_pointed` | U+05E9 U+05B8 U+05C1, Hebr/he/RTL | defaults | SHAPE-007/011: offsets, internal mark order, clusters |
| `hebrew_vav_holam` | U+05D5 U+05B9, Hebr/he/RTL | defaults | SHAPE-010: unexpected presentation-form fallback |
| `hebrew_shalom_pointed` | U+05E9 U+05B8 U+05C1 U+05DC U+05D5 U+05B9 U+05DD, Hebr/he/RTL | defaults | SHAPE-007/010/011 combined |

The diagnostic sequences below retain the original **raw visual-order HB**
expectations for reference; the current runner converts them to logical order.
For these details, notation is `GID/cluster (X-offset, X-advance)`;
all Y offsets/advances are zero. Full six-field oracles are in the JSON.

**Plain shalom:** expected `23/3 (0,684), 124/2 (-10,291), 55/1 (0,522),
96/0 (0,730)`; actual is that exact sequence in reverse order. Thus per-glyph
metrics and the −10 positioning adjustment match for this word; the current
logical-order comparison passes.

**Pointed shin:** expected `79/0 (227,0), 100/0 (539,0), 96/0 (0,730)`;
actual `96/0 (0,730), 79/1 (0,0), 100/2 (0,0)`. Both marks have attachment
metadata pointing to base index 0, with classes 4/5, but unresolved offsets.
Their nominal advances are already zero in this font: this is not new evidence
of SHAPE-008. The absent cluster merging is separate from attachment resolution.

**Vav + holam:** expected `46/0 (92,0), 124/0 (0,301)`;
actual `126/0 (0,301)`. GID 126 is the mapped presentation form U+FB4B.
This one-base case isolates the composition mismatch from the rest of the word.

**Pointed shalom:** expected `23/6 (0,684), 46/4 (82,0), 124/4 (-10,291),
55/3 (0,522), 79/0 (227,0), 100/0 (539,0), 96/0 (0,730)`;
actual `96/0 (0,730), 79/1 (0,0), 100/2 (0,0), 55/3 (0,522),
126/4 (0,301), 23/6 (0,684)`. Fixing only order cannot make it pass.

See [SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md) for diagnosis
and agreed contracts. This work changes tests and public contract comments,
not production shaping behavior.
