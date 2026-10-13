---
canonical: https://grafana.com/docs/alloy/latest/scenarios/profiles/
description: Runnable scenarios that collect continuous profiles with Grafana Alloy
menuTitle: Profiles
title: Profiles scenarios
weight: 400
---

# Profiles scenarios

These scenarios collect continuous profiles and send them to Pyroscope.

They show three ways to get profiles without changing application code: scraping `pprof` endpoints with [`pyroscope.scrape`][pyroscope.scrape], profiling every process on a host with [`pyroscope.ebpf`][pyroscope.ebpf], and attaching to a running Java virtual machine with [`pyroscope.java`][pyroscope.java].
All three deliver through [`pyroscope.write`][pyroscope.write].

{{< docs/shared lookup="scenarios/profiles.md" source="alloy" version="<ALLOY_VERSION>" >}}

[pyroscope.scrape]: ../../reference/components/pyroscope/pyroscope.scrape/
[pyroscope.ebpf]: ../../reference/components/pyroscope/pyroscope.ebpf/
[pyroscope.java]: ../../reference/components/pyroscope/pyroscope.java/
[pyroscope.write]: ../../reference/components/pyroscope/pyroscope.write/
