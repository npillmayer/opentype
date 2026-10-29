# Normalization and source-cluster discovery batch

Verified on 2026-10-26 against source revision
`3acce4440f5a215e9752cd2f7c25430b3279994a` plus this batch's tests and documentation.
Production behavior is unchanged; public API comments now record the agreed
logical-order and HarfBuzz level-0 cluster contracts.

**SHAPE-002 update (2026-10-27):** `latin_range_after_nfd` now passes its full
unchanged capture and runs in the ordinary suite. The corpus has 11 passing and
13 tagged cases. Original discovery diagnostics below retain their dated context.

**Reviewed SHAPE-011 resolution (2026-10-29):** the canonical Latin marks,
Latin uncomposable mark, explicitly enabled vav/holam, and reordered shin now
pass complete unchanged captures. The corpus has **15 ordinary / 9 tagged**
cases. Initial base/mark groups select indexed feature ranges by group start;
source anchors remain separate for bookkeeping. The historical pointed-shin
case now agrees on clusters but still differs in internal Hebrew mark order.
The [resolved finding](../../../../doc/SHAPING_FINDINGS.md#shape-011--combining-mark-clusters-differ-from-harfbuzz-level-0)
retains the agreed scope and review evidence; its full Hebrew order assertion
remains tagged for the separate follow-up.
The original diagnostics below describe earlier revisions, not current results.

## Contract and comparison

The sink receives **logical shaping order**, after normalization and script
reordering. Within a cluster this need not be original scalar order. Visual
ordering belongs to the caller. Cluster values are zero-based original Unicode
scalar indexes, with HarfBuzz level-0 grouping and merging semantics.

The raw HarfBuzz JSON remains in its original visual order. `textLogicalOutput`
copies it and reverses the **entire record sequence** for RTL, undoing HarfBuzz's
final reversal; LTR is unchanged. This reverses records inside a repeated cluster
as well. Sorting clusters or merely reversing cluster groups would hide internal
mark order differences. Advances and offsets stay attached to their glyphs;
the adapter does not resolve anchors, rebase positions, or change cluster values.
These are rendering positions: a consumer needs visual ordering before summing
advances for painting. Logical storage order is not a left-to-right drawing loop.

The reversal corresponds to `hb_ot_shape`'s positioning path in
[the recovered source](https://github.com/harfbuzz/harfbuzz/blob/29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b/src/hb-ot-shape.cc):
`hb_buffer_reverse` follows positioning for backward directions. An adapter test
checks repeated clusters, per-glyph offset preservation, and oracle immutability.
The ordinary plain-shalom baseline now passes, resolving SHAPE-009 without a
production reversal. This is deliberately not paragraph bidi implementation.

## New cases and results

Thirteen captures extend the original eleven to **24 cases: 11 passing and
13 tagged** after the SHAPE-002 promotion. The original captures are byte-for-byte unchanged as JSON values.
All tests keep one shaping window with high/low/max `n+1/1/n+2`. Feature ranges
are half-open original scalar indexes. The test-only adapter accepts explicit
`tag=value` and `tag[start:end]=value`; zero disables. Unsupported ranges fail
instead of being ignored. The legacy JSON runner's parser is unchanged.

| Case ID | Input and settings | Result |
| --- | --- | --- |
| `latin_marks_canonical` | Gentium, `a` + U+0323 + U+0301, Latn/en/LTR | SHAPE-007/011: GIDs correct; final acute has cluster 2 instead of 0 and offset 0 instead of 83 |
| `latin_marks_reordered` | Same, `a` + U+0301 + U+0323 | SHAPE-012: canonical-equivalent input produces a different glyph sequence |
| `latin_uncomposable_mark` | Gentium, `x` + U+0301, Latn/en/LTR | SHAPE-007/011: acute cluster 1 instead of 0, X offset 0 instead of 31 |
| `latin_missing_composite_nfc` | NoEacute font, U+00E9 + `X`, Latn/fr/LTR | SHAPE-013: emits missing glyph instead of decomposing |
| `latin_missing_composite_nfd` | NoEacute font, `e` + U+0301 + `X`, Latn/fr/LTR | SHAPE-013: composes into an unmapped character |
| `latin_range_after_nfc` | Gentium, U+00E9 + `office`, `liga=0,liga[2:5]=1`, Latn/fr/LTR | Pass: `ffi` at original cluster 2 |
| `latin_range_after_nfd` | Gentium, `e` + U+0301 + `office`, `liga=0,liga[3:6]=1`, Latn/fr/LTR | Pass after SHAPE-002: `ffi` at original cluster 3; all fields match |
| `hebrew_vav_holam_mark_on` | Noto, U+05D5 U+05B9, `mark=1,mkmk=1`, Hebr/he/RTL | Composition now matches; offsets/clusters fail SHAPE-007/011 |
| `hebrew_vav_holam_mark_off` | Same, `mark=0,mkmk=0` | SHAPE-010: keeps separate glyphs where HB uses presentation-form fallback |
| `hebrew_vav_holam_no_mark` | NoMark font, U+05D5 U+05B9, default features | Pass: fallback GID 126, cluster 0, advance 301 |
| `hebrew_vav_holam_no_mark_requested` | NoMark font, `mark=1,mkmk=1` | SHAPE-010: requesting absent features incorrectly suppresses fallback |
| `hebrew_vav_holam_no_gpos` | NoGPOS font, default features | SHAPE-014: ordinary parser rejects absent GPOS before shaping |
| `hebrew_shin_reordered` | Noto, U+05E9 U+05C1 U+05B8, `mark=1,mkmk=1` | GID order matches logical HB; offsets/clusters fail SHAPE-007/011 |

Font derivation, source hashes, naming, licenses, and reproducible preparation
are in [fonts/NORMALIZATION.md](../../../../testdata/fonts/NORMALIZATION.md).
NoEacute retains glyph outlines and layout tables; only the U+00E9 Unicode
mapping is removed. NoMark retains GPOS, so absent features are tested separately
from the parser's deliberate restriction to fonts containing both GSUB and GPOS.
The no-GPOS capture is valid HarfBuzz output, but the public Go path cannot yet
reach that fallback test. It is a deferred scope issue, not a malformed fixture.

## Diagnostic details

For canonical `a + dot below + acute`, HarfBuzz yields GIDs `[277,2173]`, both
cluster 0, advances `[940,0]` and X offsets `[0,83]`. Our canonical input gives
the same GIDs but clusters `[0,2]` and zero offsets. The reordered input gives
GIDs `[105,2289]`, also clusters `[0,2]`: adjacent-pair composition has consumed
the acute before a later lower-class mark can be reordered. The missing offsets
and cluster merging are separate from this normalization failure.

For the missing-composite cases, HarfBuzz emits `[72,2173,59]`, advances
`[946,0,1260]`, X offsets `[0,107,0]`. Clusters are `[0,0,1]` for NFC and
`[0,0,2]` for NFD: the following `X` retains its original source coordinate.
Our outputs are `[0,59]`, clusters `[0,1]` / `[0,2]`, advances `[1400,1260]`.
The loss is font-aware normalization, not lack of the decomposed glyphs.

For the NFD ranged-ligature case, expected GIDs/clusters are
`[112/0,82/2,741/3,70/6,72/7]`; before SHAPE-002 the actual output was
`[112/0,82/2,73/3,737/4,70/6,72/7]`. Original cluster anchors survive, but
the feature mask was assigned by normalized buffer position. Mask assignment
now uses those retained source anchors and the complete comparison passes. The NFC control
passes with the equivalent original-coordinate range. No streaming cut is
involved, extending SHAPE-002 beyond chunk-local indexing.

Pointed shin's logical expected GIDs are `[96,100,79]`; the earlier input
U+05E9 U+05B8 U+05C1 currently emits `[96,79,100]`. The new opposite-order input
with explicit mark features emits the expected GID order, but still assigns
clusters `[0,1,2]` instead of `[0,0,0]` and zero offsets instead of `[0,539,227]`.
Neither agreement on logical output order nor a cluster merge alone establishes
correct script-specific mark ordering.

## Original batch verification and next steps

`go test ./... -count=1` passes. The combined tagged suite has **20 failing
top-level tests**: thirteen earlier shared/AOTS tests and seven text tests
covering fourteen cases. Nineteen fail on shaping assertions; the no-GPOS test
fails at the documented parser policy. The two adapter-contract tests pass.

```sh
go test ./internal/hbcmp -run '^Test(HarfBuzzTextBaselines|Text)' -count=1 -v
go test -tags shaping_knownbugs ./otshape/... ./internal/hbcmp -count=1
python3 testdata/scripts/capture_harfbuzz_text.py
```

Remaining fixes can address font-aware normalization, canonical/Hebrew mark
ordering, and effective mark support independently. Source-coordinate masks
and Latin/Hebrew base/mark grouping now have ordinary regressions. Promote complete passing records when all their blockers
are fixed. Initial-group indexed range bounds have focused coverage under
SHAPE-011; feature events inside groups remain separate. The captured ranged
ligature examples lie wholly after the normalized prefix.
Streaming cuts remain the next independent test axis. See
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md) for stable IDs.
