# Allocation algorithm simulations

Simulations live in the operator checkout's
`cmd/otel-allocator/internal/allocation/simulation_test.go`. Load-shedding now uses
the production strategy and size windows; the other algorithms remain comparison
baselines. Standalone TA selects it with `allocation_strategy: load-shedding`;
configuration and asynchronous target probing are documented in that checkout's
`docs/target-allocator/load-shedding.md`. Run simulations there:

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
