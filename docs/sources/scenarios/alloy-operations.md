---
canonical: https://grafana.com/docs/alloy/latest/scenarios/alloy-operations/
description: Runnable scenarios that run, scale, and monitor Grafana Alloy itself
review_date: 2026-10-15
menuTitle: Alloy operations
title: Grafana Alloy operations scenarios
weight: 700
---

# {{% param "FULL_PRODUCT_NAME" %}} operations scenarios

These scenarios are about running {{< param "PRODUCT_NAME" >}} itself rather than about collecting a particular signal.

Spread scrape targets across a [cluster][] so no single instance owns everything.
Collect {{< param "PRODUCT_NAME" >}}'s own metrics and logs so you can tell when a pipeline stalls.
Load credentials at runtime with [`remote.vault`][remote.vault] instead of writing them into a configuration file.

{{< docs/shared lookup="scenarios/alloy-operations.md" source="alloy" version="<ALLOY_VERSION>" >}}

[cluster]: ../../get-started/clustering/
[remote.vault]: ../../reference/components/remote/remote.vault/
