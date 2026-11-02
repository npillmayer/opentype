# Normalization and source-cluster discovery batch

Verified on 2026-10-26 against source revision
`3acce4440f5a215e9752cd2f7c25430b3279994a` plus this batch's tests and documentation.
Production behavior is unchanged; public API comments now record the agreed
logical-order and HarfBuzz level-0 cluster contracts.

**SHAPE-023 resolution (2026-11-02):** Hebrew now uses
modified combining classes for ordering and composition blocking, with its
special meteg adjustment before recomposition. Both original ordering failures
are promoted unchanged. Nine additional independent captures live in the
separate [Hebrew ordering corpus](HEBREW_ORDER.md). Earlier Hebrew exclusions
and tagged counts below are historical snapshots, not the current status.

**SHAPE-012/013 resolution (2026-11-01):** the reordered
Latin marks and both missing-composite cases now pass all original fields and
run in the ordinary suite. The original 35-record corpus is unchanged: 32 cases
are ordinary and three remain tagged. Four original normalization cases also
run through `ShapeEvents`.

Fourteen additional independent captures are stored separately in
[`normalization.json`](normalization.json). All fourteen cases now run through
both APIs (28 comparisons): SHAPE-016 adds equivalent push/pop recipes for the two indexed-range cases. Every comparison
checks all six output fields.
The cases exercise:

- Precomposed, canonical and reversed accents, with original indexes on a
  following `X`.
- Equal-class blocking and composition across an unconsumed lower-class mark.
- Missing a-with-dot-below: all three spellings use supported a-with-acute and
  dot below instead of introducing a missing glyph.
- Missing ring and A-ring-acute: retain A-ring, and decompose unavailable
  A-ring-acute to A-ring plus acute without needing the unavailable ring.
- Indexed ligature ranges after decomposition and reordering.

The new fonts, hashes, source/license and reproducible derivation are recorded
in [font provenance](../../../../testdata/fonts/NORMALIZATION.md). The original
three derived fonts and all previous captures remain unchanged. Reproduce the
new captures with `python3 testdata/scripts/capture_harfbuzz_normalization.py`;
add `--write` only to replace them deliberately. Captures pin HarfBuzz 15.0.0,
its binary hash, OpenType shaper/font functions, Unicode scalar clusters at
level zero, and font-unit positions. Go tests use checked-in data only.

At the SHAPE-012/013 stage, Hebrew bypassed generic canonical sorting by explicit
user decision while sharing font-coverage checks. SHAPE-023 subsequently replaced
that exclusion with modified Hebrew classes and an early reorder hook. Both
original ordering failures now pass their complete unchanged expectations.
Streaming/context limits are unchanged.
The accepted fixes, contracts, and recorded implementation verification are
archived under [SHAPE-012](../../../../doc/SHAPING_FINDINGS.md#shape-012--canonical-equivalent-mark-orders-shape-differently)
and [SHAPE-013](../../../../doc/SHAPING_FINDINGS.md#shape-013--normalization-ignores-font-coverage).

**SHAPE-010 resolution (2026-11-01):** default vav/holam,
explicitly disabled mark features, and explicitly requested absent features now
pass their unchanged full captures. Four HarfBuzz 15 captures extend the corpus
to 35 records (29 ordinary, six tagged), preserving all 31 previous records and
capture provenance:

| New case | Settings | Independent result |
| --- | --- | --- |
| `hebrew_vav_holam_mark_only` | `mark=1,mkmk=0` | Separate vav/holam; logical GIDs `[124,46]`, holam X offset 92 |
| `hebrew_vav_holam_mkmk_only` | `mark=0,mkmk=1` | Presentation-form fallback GID 126 |
| `hebrew_mark_range_on` | Two vav/holam pairs separated by space; both features globally off and on in `[3:5)` | Both pairs stay separate; only the second holam has X offset 92 |
| `hebrew_mark_range_off` | Same input; both features off in `[0:2)` | Same complete output as the ranged-on case |

These captures establish the agreed plan-wide normalization signal: `mark`
support, including ranged enablement, suppresses fallback throughout the plan;
`mkmk` alone does not. Positioning still obeys ranges. Every record contains all
six output fields, exact command arguments, and font/tool hashes. The capture
script reproduces them; Go tests remain offline. Seven global-setting cases
also run through `ShapeEvents`. SHAPE-016 subsequently removed normalization
barriers within each carry and added focused indexed-range/event equivalence
coverage; safe streaming boundaries remain separate.

Pointed shalom now produces all seven expected glyphs with matching clusters
and per-glyph positions, but the two initial shin marks remain reversed. Its
full original oracle subsequently passed unchanged after SHAPE-023 resolved
the remaining order difference.
The accepted fix and contract are archived in
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md#shape-010--default-mark-positioning-is-missed-during-normalization).
The dated discovery diagnostics below remain historical.

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

Font-aware representation and canonical ordering are now resolved under
SHAPE-012/013 within the agreed scope. SHAPE-023 subsequently resolved the
recorded Hebrew ordering failures with modified classes and early reordering.
Effective mark support is resolved under SHAPE-010. Source-coordinate masks
and Latin/Hebrew base/mark grouping have ordinary regressions. Promote complete
passing records when all their blockers are fixed. Initial-group indexed range bounds have focused coverage under
SHAPE-011; feature events inside groups remain separate. The captured ranged
ligature examples lie wholly after the normalized prefix.
Streaming cuts remain the next independent test axis. See
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md) for stable IDs.
