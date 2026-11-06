---
canonical: https://grafana.com/docs/alloy/latest/scenarios/logs/
description: Runnable scenarios that collect, parse, route, and redact logs with Grafana Alloy
review_date: 2026-10-15
menuTitle: Logs
title: Logs scenarios
weight: 200
---

# Logs scenarios

These scenarios collect logs from files, network protocols, message queues, and browsers, then forward them to Loki.

Most follow the same shape.
A source component such as [`loki.source.file`][loki.source.file] feeds a chain of [`loki.process`][loki.process] stages, which parses each line and promotes fields to labels.
[`loki.write`][loki.write] then delivers the result.

Stage chains are where most log pipelines become complicated.

{{< docs/shared lookup="scenarios/logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

[loki.source.file]: ../../reference/components/loki/loki.source.file/
[loki.process]: ../../reference/components/loki/loki.process/
[loki.write]: ../../reference/components/loki/loki.write/
