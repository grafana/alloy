---
canonical: https://grafana.com/docs/alloy/latest/scenarios/
description: Run self-contained Grafana Alloy scenarios for each telemetry signal and for the systems you monitor
menuTitle: Scenarios
title: Grafana Alloy scenarios
weight: 250
---

# {{% param "FULL_PRODUCT_NAME" %}} scenarios

A scenario is a small, self-contained project that shows one way to collect or process telemetry with {{< param "PRODUCT_NAME" >}}.
Each scenario ships with an {{< param "PRODUCT_NAME" >}} configuration, the backends it needs, and a README.
Most scenarios configure Grafana with the data sources the stack needs, so you can query your telemetry right away.

Scenarios help you learn and experiment.
Use one to try a pipeline before you adopt it, compare two approaches, or reproduce a problem in isolation.
They aren't production configurations, so read a scenario as a worked example rather than as a template to deploy.

The scenarios live in the [alloy-scenarios][] repository.
Each scenario directory holds its own README with everything you need to deploy and explore it.
You can work straight from the repository if you prefer.

## Browse the scenarios

Start with [Run a scenario][] for the requirements and the setup steps.
Each of the other pages collects the scenarios for one signal or one part of your stack.

{{< section >}}

## Contribute a scenario

The repository accepts both additional scenarios and improvements to the ones already there.
Refer to [Contribute][] for what a scenario should include and how to submit it.

[alloy-scenarios]: https://github.com/grafana/alloy-scenarios
[Contribute]: https://github.com/grafana/alloy-scenarios#contribute
[Run a scenario]: ./run-a-scenario/
