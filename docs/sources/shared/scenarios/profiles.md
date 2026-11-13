---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/profiles/
description: Shared content, Alloy scenarios, Profiles
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Continuous profiling][] | Collect and visualize CPU, memory, and goroutine profiles from Go applications with Grafana Pyroscope. |
| [eBPF host profiling][] | Profile every process on a Linux host with `pyroscope.ebpf` -- no language agents, no application code changes. Uses Docker container discovery to attribute samples per workload. |
| [Java profiling][] | Attach async-profiler to a running JVM with `pyroscope.java` -- CPU and allocation flame graphs with no agent jar and no code changes. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Continuous profiling]: https://github.com/grafana/alloy-scenarios/tree/main/continuous-profiling
[eBPF host profiling]: https://github.com/grafana/alloy-scenarios/tree/main/ebpf-host-profiling
[Java profiling]: https://github.com/grafana/alloy-scenarios/tree/main/java-profiling
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios
