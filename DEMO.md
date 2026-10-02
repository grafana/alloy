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

## Recording runbook (4-minute goal; 5-minute hard limit)

All commands below run from `example/kind`. Keep one single-node kind cluster. Build before recording:

```sh
KIND=/opt/homebrew/bin/kind task demo:prepare
task demo:dashboard
task demo:reset
```

- [Comparison dashboard](https://thampiotr.grafana.net/d/target-allocation-demo?var-mode=ta-packing&theme=light): select the deployment being shown; use the first screen for load bars and scroll for resources, discovery and movement. Source: `config/target-allocation/dashboard/demo.json`.
- [Target explorer](http://localhost:8080/debug/allocation): run `task ui:ta` in another terminal **after** switching to sticky. Restart that command whenever TA restarts.
- [Alloy Prometheus details](https://thampiotr.grafana.net/d/e324cc55567d7f3a8e32860ff8e6d0d9): optional drilldown, not another segment in the video.

| Video time | Say / show | Commands before capturing the metrics |
|---|---|---|
| **0:00–0:50** | Your Excalidraw: every Alloy discovers everything. Show three discovery processes and the production profile/cost evidence. The seven-whale fixture exposes size-blind placement: compare target counts with sample load; hashing can still get lucky on other workloads. | `task demo:mode MODE=vanilla`; [vanilla dashboard](https://thampiotr.grafana.net/d/target-allocation-demo?var-mode=vanilla&theme=light) |
| **0:50–1:30** | Excalidraw: central discovery and HTTP assignments. Show one discovery process, but uneven scrape load: target size is still ignored. | `task demo:mode MODE=ta-hashing`; [hashing dashboard](https://thampiotr.grafana.net/d/target-allocation-demo?var-mode=ta-hashing&theme=light) |
| **1:30–2:20** | Excalidraw: async sizing, sticky ownership, repair sustained overload. Show the real bars become even. | `task demo:mode MODE=ta-packing`; [sticky dashboard](https://thampiotr.grafana.net/d/target-allocation-demo?var-mode=ta-packing&theme=light) |
| **2:20–3:15** | Recorded cuts: scale to five Alloys, then back to three; add/remove eight mixed targets. Show load, endpoint count and moves. | `task demo:alloys REPLICAS=5`, then `REPLICAS=3`; `task demo:targets:extra`, then `task demo:targets:reset` |
| **3:15–4:00** | Explorer: search `payments` / `invoice-worker`, show labels and owner. Close with modeled ~$7.9k/month discovery savings, plus unpriced API/headroom benefits. | `task ui:ta`; open the [explorer](http://localhost:8080/debug/allocation) |

Allow **2–3 minutes between actions** for probes, windows, config reload, scrape and Cloud ingestion; CPU rates need **two minutes**. Wait for endpoint counts and bars to settle rather than timing blindly. Record these waits separately and trim them; label time-compressed transitions. Keep the final minute as contingency, not extra content.

Future-work closing line: **HA and low-churn failover need leader election/routing plus shared sticky ownership; staleness-aware handoff remains unresolved. TA could also be embedded in the Alloy binary, like the OpAMP supervisor, to simplify binary vetting—possibly as a component or extension.** This is an OTel TA fork usable by Alloy through HTTP discovery; Collector interoperability is intended but still needs a smoke test. Do not claim upstream release or completed HA.

## Data collection (45 minutes plus rollout time)

The earlier timed run was aborted. These are the next run's commands, **not a running schedule**. Run from `example/kind`; start with `task demo:reset`. Hold each phase after rollout completes, checking that all endpoints have appeared. Keep the same synthetic pods throughout.

| Phase | Command | Hold |
|---|---|---:|
| Vanilla, three Alloys | `task demo:mode MODE=vanilla` | 10 min |
| TA hashing | `task demo:mode MODE=ta-hashing` | 10 min |
| TA sticky | `task demo:mode MODE=ta-packing` | 10 min |
| Five Alloys | `task demo:alloys REPLICAS=5` | 5 min |
| Three Alloys, extra targets | `task demo:alloys REPLICAS=3`, then `task demo:targets:extra` | 5 min |
| Restore base workload | `task demo:reset` | 5 min |

Sanity-check dashboard windows from **2 October 2026, UTC** (short checks, not the full collection run):

| Mode | Fixed window | Max / mean load |
|---|---|---:|
| Vanilla | [11:22:00–11:22:50](https://thampiotr.grafana.net/d/target-allocation-demo?var-mode=vanilla&from=1790940120000&to=1790940170000&timezone=utc&theme=light) | 1.21× |
| TA hashing | [11:24:00–11:25:05](https://thampiotr.grafana.net/d/target-allocation-demo?var-mode=ta-hashing&from=1790940240000&to=1790940305000&timezone=utc&theme=light) | 1.37× |
| TA sticky | [11:28:00–11:30:00](https://thampiotr.grafana.net/d/target-allocation-demo?var-mode=ta-packing&from=1790940480000&to=1790940600000&timezone=utc&theme=light) | 1.00× |

## Rehearsal and safeguards

- Same heavy-tailed pool throughout: **83 synthetic targets / 149,997 series**, plus Alloy, TA, CoreDNS and KSM. Vanilla has 90 endpoints; TA modes have 91; a fourth Alloy adds one; the extra batch adds eight targets / 8,378 series. Seven whales: 2 × 8,009, 2 × 12,007 and 3 × 24,011 series. The largest is below mean load at five Alloys. Different hash inputs/rings mean vanilla and TA hashing need not have the same placement. The 83 synthetic pods leave room under the node’s 110-pod limit for five Alloys and the extra batch.
- Seven-whale sanity check (2026-10-02), same synthetic pod UIDs/IPs across all modes: vanilla **46,522 / 62,844 / 46,183** samples per scrape (**1.21×** max/mean); TA hashing **35,335 / 71,160 / 49,247** (**1.37×**); sticky approximately **52,002 / 51,854 / 51,905** (**1.00×**). All 91 TA-mode endpoints were healthy. Results and screenshot: ignored `build/target-allocation/seven-whales/`. Real exporter sizes vary, and other placements can balance differently.
- The hashing → sticky fast path reloads Alloy configuration and restarts TA only; pod identities were verified unchanged. Full vanilla ↔ TA changes roll Alloy. The move counter excludes new/deleted targets and resets with TA; it does **not** count cold-start reassignment. Deployment/scale actions annotate the dashboard.
- Dashboard load/health select the newest scraper per endpoint and exclude samples older than 75s. TA size estimates are separate from observed samples and WAL series. Healthy discovered endpoints do not prove complete discovery or gap-free handoff. CPU/heap/RSS include TA overhead; a small local snapshot is not proof of fleet dollar savings.
- `task demo:profiles` captures each local Alloy's heap and 10-second CPU profile under ignored `build/target-allocation/profiles/`. Explore one with `go tool pprof -http=127.0.0.1:8082 FILE`, then open [localhost:8082](http://localhost:8082). Stop pprof afterwards.
- Saved fallback dashboard PNGs and earlier production profiles are in ignored `build/target-allocation/evidence/`. Production profiles aggregate a sampled window; their absolute byte totals are not instantaneous fleet RSS. Keep raw internal profiles out of public commits.
- Customer evidence (internal): [request for help scaling PodMonitor discovery across 15–30 Alloy replicas](https://grafanalabs.enterprise.slack.com/archives/CSN5HV0CQ/p1773181944934339); [internal discovery architecture discussion](https://grafanalabs.enterprise.slack.com/archives/C0AKJ86LHS7/p1783073315329799). Use anonymized motivation in the recording, not customer-identifying details.
- Cost reproduction: in the earlier Ops/dev metrics datasources, evaluate `sum by (resource) (avg_over_time(cluster_namespace:cost_per_minute_dollars_actual:sum{namespace="grafana-agent",resource=~"cpu|memory"}[24h])) * 43200`, with the original prod/dev cluster scope and evaluation date. Above figures retain their 2026-10-01 date; a fresh query will differ.

## Left for recording / future work

- Your Excalidraw diagrams, narration, and the final ≤4-minute edit. Rehearse current placement once before filming; do not rebuild or change target pods between modes.
- Core deployment, dashboard, scale/reset commands, annotations and local profile capture are implemented. End with `task demo:reset` to restore three Alloy instances/base targets, or `task cluster:down` to stop ingestion.
- Deferred: OTel Collector smoke test, full HA/state sharing, staleness-aware handoff, embedded TA, and a production rollout. No remote Kubernetes clusters were contacted.
