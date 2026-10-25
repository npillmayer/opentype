# Shaping Performance Baseline

## Scope and Method

Stage 6, step 4 adds allocation-reporting benchmarks in
`otshape/shaping_bench_test.go`. No production optimization is included.

Measured on Apple M1, darwin/arm64, with benchmark parallelism suffix `-8`.
The baseline is commit `4b97e78`, before the three stage-5 bounded passes.
The current measurement uses `44ff3c9` plus the stage-6 attachment and execution
limit changes in the working tree. The same benchmark file was copied into a
`git archive` of the baseline; the working checkout was not changed.

Commands, run sequentially in each revision:

```sh
go test ./otshape -run '^$' -bench 'Benchmark(LookupApplication|ShapeOrdinary)$' -benchmem -benchtime=100ms -count=3
go test ./otshape -run '^$' -bench BenchmarkPositionReplacement -benchmem -benchtime=100ms -count=3
```

- Lookup cases use 128 glyphs and either whole-run execution or eight-glyph
  syllables. Font parsing, lookup preparation, and plan setup are outside the
  timer; fresh buffers, application, and result-length checks are inside it.
- Lookup buffers include clusters, syllables, and masks, but not positions.
  The baseline's duplicate position-edit bug would confound that comparison.
- Position replacement separately measures one replacement of one glyph by two
  in buffers of 128 and 1024 positions, without dispatcher involvement.
- Ordinary shaping exercises the public Shape entry point on a small fixture
  through a non-collecting sink. Font parsing is excluded; per-call setup is not.
  This is not a representative complex-script corpus.

## Results

Medians of three short samples; time is microseconds per operation. Percentages
use unrounded measurements. These are exploratory observations, not statistically
established regression estimates. Allocation counts provide a less noisy signal.

| Case | Baseline us | Current us | Time change | B/op old / new | allocs/op old / new |
| --- | ---: | ---: | ---: | ---: | ---: |
| LookupApplication/miss/whole | 133.1 | 138.9 | +4% | 45916 / 47999 | 1287 / 1415 |
| LookupApplication/miss/syllables | 52.3 | 56.4 | +8% | 53746 / 56340 | 1359 / 1487 |
| LookupApplication/single/whole | 49.8 | 57.1 | +15% | 53087 / 64387 | 1543 / 1927 |
| LookupApplication/single/syllables | 52.6 | 63.0 | +20% | 60917 / 80413 | 1615 / 2063 |
| LookupApplication/multiple/whole | 116.9 | 140.5 | +20% | 401045 / 576686 | 2224 / 2732 |
| LookupApplication/multiple/syllables | 114.8 | 137.6 | +20% | 365629 / 517540 | 2233 / 2872 |
| LookupApplication/ligature/whole | 35.3 | 54.2 | +53% | 46184 / 144693 | 880 / 1387 |
| LookupApplication/ligature/syllables | 35.4 | 55.6 | +57% | 47057 / 137522 | 896 / 1517 |
| LookupApplication/context/whole | 95.5 | 103.5 | +8% | 104309 / 107413 | 2951 / 3207 |
| LookupApplication/context/syllables | 99.8 | 111.5 | +12% | 112142 / 123444 | 3023 / 3343 |
| ShapeOrdinary | 20.9 | 21.4 | +2% | 27984 / 27985 | 336 / 336 |
| PositionReplacement/128 | 0.6 | 1.2 | +97% | 6240 / 12384 | 2 / 3 |
| PositionReplacement/1024 | 7.7 | 11.9 | +54% | 106593 / 155745 | 3 / 4 |

## Interpretation and Follow-Up

Ordinary shaping is approximately unchanged in this run. Substitution-heavy
cases allocate more and take longer, especially contraction and positional
replacement. The baseline is not correctness-equivalent: it lacks the completed
metadata and attachment handling, partial-progress guarantees, and execution
limits. The comparison therefore measures additional work as well as possible
inefficiency; restoring the old behavior is not an acceptable optimization.

Candidates for profiling, based on source inspection rather than attribution by
CPU/allocation profiles:

- Per-feature execution-state allocation, including coverage misses.
- Per-call edit journals and defensive AppliedEdits snapshots.
- PosBuffer.ApplyEdit replacement construction followed by a whole-buffer copy
  for attachment remapping. Investigate single-allocation construction while
  preserving ownership and attachment semantics.
- Edit-aware metadata slice replacements, which do more work than the old resize
  logic. Investigate capacity reuse only with aliasing and suffix preservation
  covered by tests.

Before changing APIs or storage, collect CPU and allocation profiles and repeat
longer, interleaved baseline/current runs (for example, 1s and count=10), comparing
with benchstat. Expand the public-path corpus to real script workloads. These
benchmarks do not measure peak memory or execution-limit exhaustion.

