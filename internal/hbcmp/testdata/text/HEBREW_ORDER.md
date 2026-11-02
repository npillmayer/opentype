# Hebrew mark ordering and recomposition (SHAPE-023)

**Status:** fixed and accepted 2026-11-02. The contract, scope and recorded
implementation verification are archived in
[SHAPING_FINDINGS.md](../../../../doc/SHAPING_FINDINGS.md#shape-023--hebrew-combining-marks-use-the-wrong-internal-order).

Captured 2026-11-02 with HarfBuzz 15.0.0 against the existing pinned
`NotoSansHebrew-Regular.ttf`. Font provenance/license are recorded in
[testdata/fonts/NotoSansHebrew-README.md](../../../../testdata/fonts/NotoSansHebrew-README.md).
`hebrew_order.json` records the font and executable hashes, command arguments,
Hebr/he/RTL context, Unicode scalar clusters at level zero, and font-unit
positions. Every output record has all six fields. The original text, scope and
normalization corpora are unchanged.

Reproduce the nine captures independently of the Go shaper:

```sh
python3 testdata/scripts/capture_harfbuzz_hebrew_order.py
# Only to intentionally replace this separate corpus:
python3 testdata/scripts/capture_harfbuzz_hebrew_order.py --write
```

The default command checks byte-for-byte agreement. Go tests consume the saved
records and never invoke HarfBuzz. RTL comparison reverses the complete captured
record sequence into logical order; it does not sort marks or relax fields.

| Case | Purpose |
| --- | --- |
| `hebrew_order_sin` | Sin dot sorts before qamats. |
| `hebrew_order_dagesh` | Shin dot, dagesh and qamats use modified-class order. |
| `hebrew_order_equal_class` | Qamats qatan and qamats retain their equal-class input order. |
| `hebrew_order_meteg` | Patah/sheva/meteg receives the special adjustment. |
| `hebrew_order_two_sequences` | Each base/mark sequence receives its own adjustment. |
| `hebrew_compose_across_dot` | With mark positioning disabled, alef+patah composes across an unconsumed shin dot; raw CCC would incorrectly block it. |
| `hebrew_compose_blocked_equal` | An intervening qamats qatan blocks alef+qamats composition. |
| `hebrew_meteg_before_compose` | The meteg adjustment happens before alef+patah composition consumes the patah. |
| `hebrew_order_range_inside` | Disabling mark/mkmk inside the initial shin group excludes that group; reordered marks retain the group's settings. |

`TestSHAPE023HebrewOrdering` runs these nine cases, the original pointed-shin and
pointed-shalom oracles, and the already-passing reordered-shin control through
both APIs: 24 complete comparisons. The range case has an explicit push/pop
recipe, independently validated against its scalar ranges. All runs keep the
whole input buffered. SHAPE-015 separately verifies repeated boundaries with a
long pointed-Hebrew capture under the implemented buffer contract.

The original two known bugs are promoted unchanged into the ordinary text
suite. Lower normalization tests cover source indexes, leading marks,
starters/CGJ barriers and equal-class stability. They establish the ordering
barrier contract, not full-output rendering of default-ignorable CGJ.
