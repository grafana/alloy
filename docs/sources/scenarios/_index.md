---
canonical: https://grafana.com/docs/alloy/latest/scenarios/
description: Run self-contained Grafana Alloy scenarios for each telemetry signal and for the systems you monitor
menuTitle: Scenarios
review_date: 2026-11-17
title: Grafana Alloy scenarios
weight: 250
---

# {{% param "FULL_PRODUCT_NAME" %}} scenarios

A scenario is a small, self-contained project that shows one way to collect or process telemetry with {{< param "PRODUCT_NAME" >}}.
Each scenario includes an {{< param "PRODUCT_NAME" >}} configuration, the backends it needs, and a README.
Most scenarios configure Grafana with the data sources the stack needs, so you can query your telemetry immediately.

Scenarios help you learn and experiment.
Use one to try a pipeline before you adopt it, compare two approaches, or reproduce a problem in isolation.
They aren't production configurations, so read a scenario as a worked example rather than as a template to deploy.

The scenarios are in the [alloy-scenarios][] repository.
Refer to [Run a scenario][] for the requirements and the setup steps.

## Browse the scenarios

Each group covers one signal or one part of your stack.

{{< section >}}

## Contribute a scenario

Add a scenario, or improve one that's already there.
Refer to [Contribute][] for what a scenario should include and how to submit it.

[alloy-scenarios]: https://github.com/grafana/alloy-scenarios
[Contribute]: https://github.com/grafana/alloy-scenarios#contribute
[Run a scenario]: ./run-a-scenario/
