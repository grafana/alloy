---
canonical: https://grafana.com/docs/alloy/latest/scenarios/alloy-operations/
description: Runnable scenarios that run, scale, and monitor Grafana Alloy itself
menuTitle: Alloy operations
title: Grafana Alloy operations scenarios
weight: 700
---

# {{% param "FULL_PRODUCT_NAME" %}} operations scenarios

These scenarios are about running {{< param "PRODUCT_NAME" >}} itself rather than about collecting a particular signal.

They cover spreading scrape targets across a [cluster][] so no single instance owns everything, collecting {{< param "PRODUCT_NAME" >}}'s own metrics and logs so you can tell when a pipeline stalls, and loading credentials at runtime with [`remote.vault`][remote.vault] instead of writing them into a configuration file.

{{< docs/shared lookup="scenarios/alloy-operations.md" source="alloy" version="<ALLOY_VERSION>" >}}

[cluster]: ../../get-started/clustering/
[remote.vault]: ../../reference/components/remote/remote.vault/
