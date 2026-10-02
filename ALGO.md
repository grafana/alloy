# Allocation algorithm simulations

Simulations live in the operator checkout's
`cmd/otel-allocator/internal/allocation/simulation_test.go`. Load-shedding and Sticky use
the production planners and shared size windows; the other algorithms remain comparison
baselines. The demo selects `allocation_strategy: sticky`;
configuration and asynchronous target probing are documented in that checkout's
`docs/target-allocator/sticky.md`. Run simulations there:

```sh
env -u GOROOT GOMAXPROCS=2 GOMEMLIMIT=1GiB go test -p=2 -mod=readonly \
  ./cmd/otel-allocator/internal/allocation -run '^TestSimulatedAllocation_' -count=1 -v
```

## Algorithms

All three reuse TA's existing `buraksezer/consistent` library. Its candidate list
starts with the hash owner, then follows hashed physical-node names; this is not
literal traversal of the virtual-node ring.

- **Consistent hashing:** assign each target to its hash owner; ignore sizes.
- **Capacity-aware:** order targets by five size tiers (ideal-load divisors 2, 8,
  32, 128), then stable hash. Assign each to the first candidate below 1.2× ideal.
  If none fits, use the least-loaded instance, breaking ties by instance ID.
- **Load-shedding:** start with hash homes. Instances above 1.2× ideal shed their
  home targets in stable hash order; donors are also ordered by stable hash.
  Adopt at the first candidate that stays below 1.1× ideal. Donors never adopt.
  Skip moves leaving the donor below 0.8× ideal. If adoption fails, keep the target
  home and report unresolved overload. No previous assignments are retained.

Both size-aware algorithms use the latest membership, a six-cycle maximum for
individual sizes, and another six-cycle maximum over accounting totals.
Ideal load is that stabilized total divided by the current instance count.
Together the windows depend on **11 raw cycles**, not six; at five seconds per
cycle, a spike can affect capacity until 55 seconds later. There is no hidden
history beyond that window. New targets use their supplied size immediately.

## Scenarios

Each deterministic synthetic run lasts 360 cycles: **30 simulated minutes**.
Major changes happen every 30 cycles. This is not a production trace.

| Scenario | Inputs |
| --- | --- |
| Target churn | 1,100–2,000 targets, rotating identities; six instances |
| Size changes | 1,500 targets, six instances; ~1/7 of targets grow/shrink by 0.4–2.5×; all have ±5% per-cycle noise |
| Instance churn | 1,500 targets; 3→4→6→8→10→7→5→3→6→10→4→3 instances |
| Combined | All three changes together |

Base size mix: ~75% at 50–500 series, 20% at 1k–5k, 4.5% at 10k–30k,
and 0.5% at 100k–400k. Sizes change further in the noisy scenarios.

## Results

Movement is the percentage of surviving targets changing owners between cycles;
new/deleted targets and initial placement are excluded. Averages include quiet
cycles. Load imbalance is max/mean using **current raw sizes**, including idle
instances; 1.0 is perfect. Each result cell shows **average / worst cycle**.

| Scenario | Algorithm | Targets moved | Load imbalance |
| --- | --- | --- | --- |
| Targets | Consistent hashing | 0% / 0% | 1.528 / 1.733 |
| Targets | Capacity-aware | 1.162% / 41.615% | 1.225 / 1.686 |
| Targets | Load-shedding | 0.330% / 16.745% | 1.197 / 1.655 |
| Sizes | Consistent hashing | 0% / 0% | 1.271 / 1.317 |
| Sizes | Capacity-aware | 0.554% / 6.533% | 1.215 / 1.298 |
| Sizes | Load-shedding | 0.224% / 2.533% | 1.207 / 1.298 |
| Instances | Consistent hashing | 1.259% / 58.067% | 1.493 / 2.094 |
| Instances | Capacity-aware | 1.613% / 72.333% | 1.200 / 1.200 |
| Instances | Load-shedding | 1.408% / 63.867% | 1.175 / 1.199 |
| Combined | Consistent hashing | 1.262% / 58.274% | 1.642 / 2.945 |
| Combined | Capacity-aware | 2.690% / 73.283% | 1.237 / 1.830 |
| Combined | Load-shedding | 1.743% / 60.728% | 1.261 / 2.074 |

Load-shedding moves fewer targets than capacity-aware in these fixtures, but its
combined-change balance is worse. In the size scenario, 975 of its 1,205 moves
return to the preceding owner (including deliberate size reversals). Stability
is not solved. Focused tests pass; the allocation package takes about 20 seconds locally.

### Demo regression: small moves block a large target

The 60-target demo (2026-10-01) hashes to loads **1,220 / 2,010 / 500**.
Load-shedding moves 20 small targets, ending at **1,220 / 1,600 / 910**
(max/mean **1.287**). Moving just one 800-series target from the original
placement would instead give **1,220 / 1,210 / 1,300** (max/mean **1.046**).

Stable hash order moves 80 series before considering the first large target:
the recipient would reach 1,380, exceeding the 1.1× adoption cap of 1,367.7.
The second large target is also blocked; the pass never undoes earlier moves.
Identical inputs reproduce the same result after the entire sizing window settles.
This is a greedy-placement limitation, not an estimation or discovery mismatch.

Simulation tests `demo_whales` and `small_moves_block_feasible_whale_move`
capture the real assignments and a six-target reduction. They document current
behavior and a feasible better placement; they do not endorse this balance.
Candidates to discuss: large-first shedding within each donor, or relaxing the
adoption cap. Neither change has been applied to the algorithm.

## Adversarial simulations (2026-10-01)

Ten small fixtures, 90 cycles each, K=6. Dynamic inputs change every 15 cycles;
all algorithms receive identical inputs. Exhaustive whole-target placement gives
the exact best possible peak for each cycle. This separates avoidable imbalance
from whales that cannot be split. These complement the larger workload tests.

Balance is max/mean using current sizes. Static results:

| Scenario | Hashing | Capacity-aware | Load-shedding | Load-shedding excess over optimum |
|---|---:|---:|---:|---:|
| Small targets first | 1.617 | 1.046 | 1.287 | 23.1% |
| Large targets first | 1.617 | 1.046 | 1.046 | 0.0% |
| More recipient headroom | 1.661 | 1.008 | 1.091 | 8.2% |
| Donor floor blocks moves | 1.287 | 1.054 | 1.287 | 22.1% |
| Two overloaded donors | 1.214 | 1.143 | 1.143 | 0.0% |
| Indivisible whale | 2.227 | 2.209 | 2.209 | 0.0% |

Dynamic balance (**average / worst**):

| Scenario | Hashing | Capacity-aware | Load-shedding |
|---|---:|---:|---:|
| Headroom oscillation | 1.605 / 1.617 | 1.054 / 1.063 | 1.276 / 1.308 |
| Whales grow/shrink | 1.433 / 1.617 | 1.147 / 1.249 | 1.268 / 1.287 |
| Small-target churn | 1.594 / 1.617 | 1.063 / 1.080 | 1.184 / 1.287 |
| Instance joins/leaves | 1.666 / 1.716 | 1.177 / 1.308 | 1.501 / 1.716 |

Surviving targets moved per cycle (**average % / worst %**); initial placement
and additions/removals are excluded. Static cases have no inter-cycle moves.

| Scenario | Hashing | Capacity-aware | Load-shedding |
|---|---:|---:|---:|
| Headroom oscillation | 0.00 / 0.00 | 0.00 / 0.00 | 1.50 / 33.33 |
| Whales grow/shrink | 0.00 / 0.00 | 1.87 / 33.33 | 1.69 / 33.33 |
| Small-target churn | 0.00 / 0.00 | 0.00 / 0.00 | 2.25 / 40.00 |
| Instance joins/leaves | 1.87 / 33.33 | 2.81 / 50.00 | 3.75 / 66.67 |

Learnings: identical home loads can produce different outcomes solely from
stable processing order; small headroom changes can reverse those outcomes;
the donor floor can require swaps rather than one-way shedding. Stability must
be measured separately from balance: a permanently bad allocation has zero churn.
The indivisible-whale control reaches 2.209× even at the optimum.

Capacity-aware reaches the exact optimum in these selected tiny cases, but this
is not evidence of universal optimality or superiority on the larger benchmarks.
Tests enforce valid assignments, settling after window expiry, and per-strategy
worst-case excess-over-optimum ceilings. Known failures remain explicit; future
improvements should tighten those ceilings. No algorithm changes were made.

Run: `go test ./cmd/otel-allocator/internal/allocation -run "TestSimulatedAllocation_(Adversarial|OptimalPeak|LoadShedding)$" -v`.

## Parameter sensitivity (2026-10-01)

108 runs: nine parameter sets × four scenarios × three algorithms; 360 cycles
per run, K=6. One parameter changes at a time. Target counts and identity-rotation
stride scale together (550–4,000 targets across variants); membership scales to
2–20 instances, rounded to nearest integer with a minimum of two. Whale scaling
changes only the largest 0.5% size bucket. Variation scales growth/shrink deviations
from 1× and per-cycle noise (integer rounding; growth multiplier floored at 0.1×).
Hash identities stay deterministic; these are parameter sweeps, not random-seed
trials or production traces. Baseline results reproduce the earlier table.

Combined scenario, surviving targets moved per cycle (**average % / worst %**):

| Parameter | Hashing | Capacity-aware | Load-shedding |
|---|---:|---:|---:|
| Baseline | 1.26 / 58.27 | 2.69 / 73.28 | 1.74 / 60.73 |
| Targets ×0.5 | 1.26 / 57.53 | 3.57 / 86.05 | 1.83 / 75.44 |
| Targets ×2 | 1.26 / 58.39 | 2.00 / 57.73 | 1.43 / 58.16 |
| Instances ×0.5 | 0.87 / 57.24 | 1.34 / 57.24 | 1.05 / 57.24 |
| Instances ×2 | 1.41 / 68.65 | 4.36 / 84.45 | 2.36 / 74.35 |
| Whale sizes ×0.5 | 1.26 / 58.27 | 1.87 / 61.08 | 1.46 / 59.54 |
| Whale sizes ×2 | 1.26 / 58.27 | 4.21 / 85.62 | 2.21 / 71.21 |
| Size variation ×0.5 | 1.26 / 58.27 | 2.38 / 73.19 | 1.64 / 60.65 |
| Size variation ×2 | 1.26 / 58.27 | 3.00 / 73.38 | 1.80 / 60.02 |

Combined load imbalance, max/mean (**average / worst**):

| Parameter | Hashing | Capacity-aware | Load-shedding |
|---|---:|---:|---:|
| Baseline | 1.642 / 2.945 | 1.237 / 1.830 | 1.261 / 2.074 |
| Targets ×0.5 | 1.481 / 2.350 | 1.220 / 2.132 | 1.204 / 2.001 |
| Targets ×2 | 1.422 / 2.070 | 1.222 / 1.741 | 1.199 / 1.637 |
| Instances ×0.5 | 1.215 / 1.556 | 1.141 / 1.479 | 1.139 / 1.479 |
| Instances ×2 | 2.212 / 3.134 | 1.340 / 1.961 | 1.486 / 2.692 |
| Whale sizes ×0.5 | 1.425 / 2.309 | 1.206 / 1.780 | 1.189 / 1.598 |
| Whale sizes ×2 | 1.941 / 3.714 | 1.254 / 1.658 | 1.374 / 3.091 |
| Size variation ×0.5 | 1.642 / 2.907 | 1.228 / 1.753 | 1.252 / 2.023 |
| Size variation ×2 | 1.641 / 3.013 | 1.251 / 1.784 | 1.270 / 2.179 |

Across all 36 parameter/scenario pairs, shedding has lower average movement in
35; the exception is instance churn with half as many instances (0.851% versus
0.839%). It has lower average imbalance in 27, but the difficult combined cases
favor capacity-aware. Doubling whales gives shedding a 3.091× worst imbalance
versus 1.658× for capacity-aware; doubling instances gives 2.692× versus 1.961×.
The broad movement advantage survives, but comparable balance does not. Together
with the small adversarial cases, this supports treating balance robustness and
movement as a trade-off, rather than assuming shedding is uniformly preferable.

Run `go test ./cmd/otel-allocator/internal/allocation -run '^TestSimulatedAllocation_Sensitivity$' -count=1 -v`.
All runs passed in 166 seconds with `GOMAXPROCS=2`, `GOMEMLIMIT=1GiB`, and `-p=2`.
No production algorithm or deployment changes.

## Sticky placement experiment (2026-10-01)

Simulation only, in `simulation_sticky_test.go`. Keep every surviving target on
its live owner. Place new/orphaned targets largest-first on the least-loaded
instance (LPT-style greedy placement); stable target hash and instance ID break
ties. Use the existing K=6 size and total windows. After an instance exceeds
1.2× ideal for six consecutive cycles, process donors heaviest-first and move
largest-fitting targets to the least-loaded instance, requiring its resulting
load ≤1.1× ideal and a strict reduction in the pair's peak. Stop when the donor
is below 1.2× ideal. No swaps, donor floor, global repacking, or move-count cap.
New instances receive new targets immediately; redistribution waits for overload.
Assignment history persists indefinitely; measurement history does not.

Baseline results, **average / worst cycle**. Movement excludes initial placement
and new/deleted targets, and includes forced moves when an instance disappears.

| Scenario | Algorithm | Targets moved % | Load max/mean |
|---|---|---:|---:|
| targets | Hashing | 0.000 / 0.000 | 1.528 / 1.733 |
| targets | Capacity-aware | 1.162 / 41.615 | 1.225 / 1.686 |
| targets | Load-shedding | 0.330 / 16.745 | 1.197 / 1.655 |
| targets | Sticky | 0.001 / 0.080 | 1.090 / 1.322 |
| sizes | Hashing | 0.000 / 0.000 | 1.271 / 1.317 |
| sizes | Capacity-aware | 0.554 / 6.533 | 1.215 / 1.298 |
| sizes | Load-shedding | 0.224 / 2.533 | 1.207 / 1.298 |
| sizes | Sticky | 0.000 / 0.000 | 1.028 / 1.070 |
| collectors | Hashing | 1.259 / 58.067 | 1.493 / 2.094 |
| collectors | Capacity-aware | 1.613 / 72.333 | 1.200 / 1.200 |
| collectors | Load-shedding | 1.408 / 63.867 | 1.175 / 1.199 |
| collectors | Sticky | 0.052 / 3.667 | 1.131 / 2.011 |
| combined | Hashing | 1.262 / 58.274 | 1.642 / 2.945 |
| combined | Capacity-aware | 2.690 / 73.283 | 1.237 / 1.830 |
| combined | Load-shedding | 1.743 / 60.728 | 1.261 / 2.074 |
| combined | Sticky | 0.143 / 12.960 | 1.148 / 2.240 |

Series-weighted movement uses current sizes: series belonging to moved surviving
targets divided by series belonging to all surviving targets, **average % / worst %**.
This is a migration-cost proxy, not a measurement of scrape gaps.

| Scenario | Hashing | Capacity-aware | Load-shedding | Sticky |
|---|---:|---:|---:|---:|
| targets | 0.000 / 0.000 | 0.440 / 15.677 | 0.452 / 20.199 | 0.054 / 7.064 |
| sizes | 0.000 / 0.000 | 0.146 / 2.299 | 0.150 / 2.137 | 0.000 / 0.000 |
| collectors | 1.405 / 65.081 | 1.562 / 65.081 | 1.491 / 65.176 | 0.939 / 54.660 |
| combined | 1.522 / 68.586 | 2.247 / 66.189 | 1.976 / 68.373 | 0.742 / 52.524 |

Repeated all nine sensitivity variants: 144 runs across four algorithms.
Sticky combined results below use the same inputs as the earlier sensitivity tables.

| Variant | Targets moved % (avg / worst) | Load max/mean (avg / worst) |
|---|---:|---:|
| baseline | 0.143 / 12.960 | 1.148 / 2.240 |
| targets_half | 0.346 / 33.597 | 1.190 / 3.251 |
| targets_double | 0.139 / 12.875 | 1.124 / 2.504 |
| collectors_half | 0.081 / 10.375 | 1.092 / 2.164 |
| collectors_double | 0.617 / 50.435 | 1.244 / 3.081 |
| whales_half | 0.152 / 14.062 | 1.125 / 2.645 |
| whales_double | 0.309 / 24.307 | 1.165 / 2.838 |
| variation_half | 0.143 / 12.720 | 1.135 / 2.219 |
| variation_double | 0.149 / 14.640 | 1.177 / 2.286 |

Sticky lowers average and worst target-count movement in all 36 workload/parameter
pairs versus both size-aware predecessors. Average balance improves in all 36
versus capacity-aware and 35 versus shedding. Worst balance improves in only 18
and 20 respectively: waiting for sustained overload permits transient peaks.
In the baseline target-churn scenario only three established targets move.
The series-weighted improvement is smaller than the target-count improvement.

All ten adversarial fixtures were also rerun. Sticky cold placement is optimal in
the six static fixtures, but whale growth/shrink retains an avoidable 23.1% peak
excess over optimum with zero movement: single-target repairs cannot always fix
an established placement. A separate unchanged-cohort test verifies zero movement
under small unrelated churn when no sustained overload occurs. These fixtures
supply sizes immediately; asynchronous initial estimation is not modeled here.

Repeated failover comparisons for all algorithms are recorded below.

Validation: sensitivity + adversarial + focused sticky tests passed in 206s;
common validity, baseline series movement, sticky and failover checks passed in
25s. Both runs used two Go workers and a 1GiB Go memory limit. No full tests,
lints, production allocator changes, or deployments.

## Repeated failover without shared state (2026-10-01)

`simulation_failover_test.go` replaces the earlier sticky-only failover test.
Eight deterministic workload/takeover offsets × four observation modes × four
algorithms = 128 trials, using the combined workload (1,100–2,000 targets and
3–10 instances across histories). Each leader runs 90 cycles. A standby either
sees identical inputs, remains 1–3 cycles behind, misses/coalesces 1–3 updates at
change boundaries, or starts at cycle 60. There is no shared assignment state.
Sticky retains its existing six-cycle overload delay; no algorithm was changed.

At takeover both replicas process the current discovery snapshot at the same
cycle boundary, retaining independent histories. Every current target must have
a live owner. The table measures ownership disagreement against the primary at
that same boundary, not extra normal workload churn. Afterward both receive 20
quiet cycles; settled disagreement is against the counterfactual continuing
primary, not a cumulative count of subsequent moves. Sizes weight series metrics.
Each cell below is **average % / worst % across eight trials**.

| Standby observations | Algorithm | Target movement at takeover | Series movement at takeover | Settled owner disagreement |
|---|---|---:|---:|---:|
| lagged | Hashing | 0.00 / 0.00 | 0.00 / 0.00 | 0.00 / 0.00 |
| lagged | Capacity-aware | 0.72 / 1.69 | 0.33 / 1.38 | 0.00 / 0.00 |
| lagged | Load-shedding | 0.14 / 0.80 | 0.05 / 0.14 | 0.00 / 0.00 |
| lagged | Sticky | 3.60 / 28.60 | 3.99 / 22.50 | 3.59 / 28.60 |
| missed_updates | Hashing | 0.00 / 0.00 | 0.00 / 0.00 | 0.00 / 0.00 |
| missed_updates | Capacity-aware | 0.56 / 1.46 | 0.25 / 1.37 | 0.00 / 0.00 |
| missed_updates | Load-shedding | 0.17 / 0.90 | 0.04 / 0.15 | 0.00 / 0.00 |
| missed_updates | Sticky | 36.99 / 49.36 | 39.32 / 62.41 | 37.08 / 49.36 |
| late_start | Hashing | 0.00 / 0.00 | 0.00 / 0.00 | 0.00 / 0.00 |
| late_start | Capacity-aware | 0.00 / 0.00 | 0.00 / 0.00 | 0.00 / 0.00 |
| late_start | Load-shedding | 0.00 / 0.00 | 0.00 / 0.00 | 0.00 / 0.00 |
| late_start | Sticky | 75.41 / 88.00 | 75.49 / 90.95 | 75.41 / 88.14 |

All identical-history controls give zero movement. Continuous lag largely
preserves the sequence of observations; missed updates change the sequence and
cause much larger lasting divergence for sticky placement. Late-start standbys
have 30 cycles to warm up, enough for both measurement windows but not sticky
ownership. These are synthetic cases, not probabilities of production failover.

Before the takeover discovery refresh, lagged/missed-update standbys omit an
average 9.10% of current targets in these boundary-heavy fixtures; some retained
owners have also departed. Thus zero hashing movement after refresh does not mean
stale discovery is safe to serve. Assignment completeness is checked after refresh.

The focused suite passed in 118 seconds with two Go workers and a 1GiB Go memory
limit. Run `go test ./cmd/otel-allocator/internal/allocation -run '^TestSimulatedAllocation_Failover$' -count=1 -v`.

## Implementation complexity

| Algorithm | Placement code | HA/state complexity |
|---|---|---|
| Hashing | Simplest: library lookup per target | Lowest; current membership only |
| Capacity-aware | One ordered pass with candidate/fallback checks | Bounded measurement windows; no ownership history |
| Load-shedding | Home loads, donor ordering, two caps, donor floor and candidate traversal | Same bounded-state advantage; more placement edge cases |
| Sticky | Preserve owners, pack arrivals, track overload and repair donors | Highest: ownership history, replication/versioning for predictable failover |

For T targets and N instances, excluding ring construction and common measurement
windows, current placement costs are approximately: hashing O(T); capacity-aware
O(T log T + T N log N); shedding O(T log T + S N log N), with S≤T attempted shed
candidates; sticky O(T log T + T N) worst case. The library's `GetClosestN` hashes
and sorts N names on every call, explaining the N log N term. Sticky currently
scans nodes for least load and targets for each donor. These are implementation
bounds, not timing measurements; N is small in these demos. Window recomputation
in the simulator adds O(K²T) to size-aware strategies; retained window data is
O(KT), with O(T) published owners and sticky's persistent ownership state.

Placement simplicity ranks hashing, then capacity-aware, then sticky/shedding
at a similar level. For a robust HA implementation, sticky is the most complex,
even though its basic placement loop is straightforward.

## Production Sticky demo

The load-shedding milestone is committed as Alloy `14b6b6141` and operator
`56242506`. `task deploy:ta-packing` now builds the operator checkout and selects
`allocation_strategy: sticky`. Probing, size windows, publication and inspection
are shared by both production size-aware strategies. Simulator Sticky calls the
production planner; baseline results match the earlier simulation exactly.
The six-cycle overload delay is retained. Only one TA runs; state replication,
leader election and staleness-aware handoff remain deferred.

Verified local Sticky deployment: all 60 targets assigned, 20 per Alloy; estimated
series 1,250 / 1,240 / 1,240 (99.5% efficiency). HTTP discovery and remote write
are active. Focused planner/configuration/publication/probing/UI checks passed,
as did the four baseline scenarios using the production planner.

The expanded demo includes 76 synthetic targets plus Alloy, TA, CoreDNS and real
kube-state-metrics: 84 endpoints. Real KSM contributes about 3,710 series and is
assigned alone; the other Alloys own 41 and 42 endpoints (~2,902 / 3,322 series).
Grafana Cloud confirms `up=1` for all 84 endpoints. Separate self/TA scrapes were
removed; all metrics endpoints use the same allocation and HTTP discovery path.
