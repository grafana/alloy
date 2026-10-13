---
canonical: https://grafana.com/docs/alloy/latest/scenarios/
description: Run self-contained Grafana Alloy scenarios for metrics, logs, traces, and profiles
menuTitle: Scenarios
title: Grafana Alloy scenarios
weight: 250
---

# {{% param "FULL_PRODUCT_NAME" %}} scenarios

A scenario is a small, self-contained project that shows one way to collect or process telemetry with {{< param "PRODUCT_NAME" >}}.
Each scenario ships with an {{< param "PRODUCT_NAME" >}} configuration, the backends it needs, and a README that explains what to look at once it's running.
Most scenarios include pre-configured Grafana dashboards, so you can query the data once the stack starts.

Scenarios are for learning and experimenting.
Use one to try a pipeline before you adopt it, to compare two approaches side by side, or to reproduce a problem in isolation.
They aren't production configurations, so read a scenario as a worked example rather than as a template to deploy.

The scenarios live in the [alloy-scenarios][] repository.
Clone it to get started:

```bash
git clone https://github.com/grafana/alloy-scenarios.git
```

{{< docs/shared lookup="scenarios/run-a-scenario.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Browse the scenarios

Each page collects the scenarios for one signal or one part of your stack.

{{< section >}}

## Contribute a scenario

The repository accepts both additional scenarios and improvements to the ones already there.
Refer to the [contributing guidance][contribute] for what a scenario should include and how to submit it.

[alloy-scenarios]: https://github.com/grafana/alloy-scenarios
[contribute]: https://github.com/grafana/alloy-scenarios#contribute
