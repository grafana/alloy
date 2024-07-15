---
canonical: https://grafana.com/docs/alloy/latest/configure/clustering/distribute-prometheus-scrape-load/
aliases:
  - ../../../tasks/distribute-prometheus-scrape-load/ # /docs/alloy/latest/tasks/distribute-prometheus-scrape-load/
description: Learn how to distribute your Prometheus metrics scrape load
menuTitle: Distribute metrics scrape load
title: Distribute Prometheus metrics scrape load
weight: 200
---

# Distribute Prometheus metrics scrape load

A good predictor for the size of an {{< param "PRODUCT_NAME" >}} deployment is the number of Prometheus targets each {{< param "PRODUCT_NAME" >}} scrapes.
[Clustering][] with target auto-distribution allows a fleet of {{< param "PRODUCT_NAME" >}}s to work together to dynamically distribute their scrape load, providing high-availability.

## Before you begin

- Familiarize yourself with how to [configure][] existing {{< param "PRODUCT_NAME" >}} installations.
- [Configure Prometheus metrics collection][].
- [Configure clustering][clustering].
- Ensure that all of your clustered {{< param "PRODUCT_NAME" >}}s have the same configuration file.

## Steps

To distribute Prometheus metrics scrape load with clustering:

1. Add the following block to all `prometheus.scrape` components, which should use auto-distribution:

   ```alloy
   clustering {
     enabled = true
   }
   ```

1. Restart or reload {{< param "PRODUCT_NAME" >}}s for them to use the new configuration.

1. Validate that auto-distribution is functioning:

   1. Using the {{< param "PRODUCT_NAME" >}} [UI][] on each {{< param "PRODUCT_NAME" >}}, navigate to the details page for one of the `prometheus.scrape` components you modified.

   1. Compare the Debug Info sections between two different {{< param "PRODUCT_NAME" >}} to ensure that they're not scraping the same sets of targets.

[Clustering]: ../../clustering/
[configure]: ../../../configure/
[Configure Prometheus metrics collection]: ../../../collect/prometheus-metrics/
[UI]: ../../../troubleshoot/debug/#component-detail-page
