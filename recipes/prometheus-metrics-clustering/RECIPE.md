---
name: prometheus-metrics-clustering
description: >-
  Shares Prometheus scrape targets between several Alloy instances with
  clustering. Use when more than one Alloy instance scrapes the same targets,
  such as a StatefulSet or Deployment with several replicas. It doesn't add a
  source, a processing step or a destination. It changes the scrape
  components of other recipes.
---

# Prometheus metrics clustering

## Instructions

- Add a `clustering` block with `enabled = true` to each `prometheus.scrape`,
  `prometheus.operator.podmonitors` and `prometheus.operator.servicemonitors`
  component.
- Calculate the largest safe value of `--cluster.wait-for-size` from the HPA
  minimum:
  1. Calculate the pods that are still up after a node failure: 80% of the
     HPA minimum, rounded down.
  2. Calculate the extra margin: the larger of 2 pods and 50% of the result
     of step 1, rounded down.
  3. Subtract the extra margin from the result of step 1. If the result is
     less than 1, don't set the flag.
- To see which pod scrapes each target, add a `scraped_by` label to
  `scrape_samples_scraped`.

The rules are in [rules.yaml](rules.yaml).

## Pipeline test guidance

A pipeline test runs one instance, so it can't check clustering. The static
checks and the cluster health metrics check it instead.
