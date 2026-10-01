# Target allocation demo

## Why it matters

Reported `grafana-agent` CPU/memory cost across prod and dev, including PoPs:

| Environment | Memory / month | CPU / month | Total / month |
|---|---:|---:|---:|
| Prod | $30,162 | $14,312 | $44,474 |
| Dev | $7,620 | $3,627 | $11,247 |
| **Combined** | **$37,781** | **$17,939** | **$55,720** |

Source: GCX, Ops Grafana `ops-cortex` and `dev-cortex`, 2026-10-01. Previous 24-hour mean of `cluster_namespace:cost_per_minute_dollars_actual:sum`, multiplied by 43,200 minutes. These are resource-request cost allocations, excluding storage/network; rounding is independent. Prod covers 68 clusters; six additional clusters had no matching cost records in the earlier inventory. This is the reported baseline, not a complete fleet invoice.

Production profiles collected on 2026-09-30 attribute **18.4% of retained Go heap** and **16.4% of CPU samples** to Kubernetes/discovery stacks. Heap includes Kubernetes objects, metadata and discovery targets—not just final scrape targets, and not total process RSS. These aggregate profile shares are a planning proxy, not a price-weighted fleet measurement; applying them to dev is an extrapolation.

With N Alloy instances doing equivalent discovery, centralizing it ideally reduces that duplicated part from N copies to one: **discovery overhead falls by 1 − 1/N** (90% at 10 instances; 97.5% at 40). TA, HTTP discovery, allocation and probing still cost resources.

Assuming an **80% net reduction in discovery overhead**, and proportional resource-request reductions:

- Memory: 18.4% × 80% ≈ **14.7% lower**, worth **~$5.6k/month**.
- CPU: 16.4% × 80% ≈ **13.1% lower**, worth **~$2.3k/month**.
- Combined opportunity: **~$7.9k/month**, before any additional benefit from better balancing.

These are modeled savings, not measured results. Heap share does not directly establish RAM/request savings; capacity must actually be reduced or avoided to save money. The 80% assumption includes replacement TA overhead.

**Additional upside, not priced above:** central discovery replaces repeated Kubernetes LIST/WATCH clients with a shared set, reducing API-server response serialization, watch delivery and network traffic. The reduction depends on discovery configuration and TA replica behavior. Total resource savings could therefore be larger; control-plane dollar savings depend on whether its capacity or charges can actually decrease.

**Better balancing offers further savings:** more even scrape load lets us provision less headroom per instance instead of sizing for the hottest instance. It should also make the fleet easier to operate, with fewer overload-related incidents and less manual tuning. These benefits are not included in the dollar estimate above.

## What we show

1. **Uneven baseline:** clustered Alloy repeats discovery and distributes targets without considering their sizes. Real example: `prod-us-central-0` had 1.13× max/mean target count but 1.96× active-series load.
2. **Central discovery:** standalone TA discovers once; Alloy fetches its assignments through HTTP discovery.
3. **Size-aware allocation:** show plain hashing versus load-shedding, estimated load per instance, and target lookup in the planned UI.
4. **Adaptation:** grow a target, introduce new targets, then add an Alloy instance. Show immediate initial placement, asynchronous sizing and assignments settling.

Keep balancing benefits separate: a 1.2–1.3× load-skew goal is illustrative, not guaranteed, and does not translate directly into the same percentage of memory or dollar savings. HA and staleness-aware handoff remain deferred.
