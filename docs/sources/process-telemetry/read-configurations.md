---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/read-configurations/
description: Learn how to interpret Grafana Alloy configurations using the graph model to trace data flow
menuTitle: Read configurations
title: Read a Grafana Alloy configuration as a data flow
weight: 400
---

# Read a Grafana Alloy configuration as a data flow

Purpose

Teach users how to interpret configurations using the graph model.

This reduces "why isn't my data working?" confusion.

Content Outline

1. Start at receivers

Identify where telemetry enters.

2. Follow connections

Trace outputs to downstream components.

Identify processing stages.

3. End at exporters

Determine where telemetry leaves Alloy.

4. Identify gaps

If data does not reach an exporter, the pipeline is incomplete.

If no processing is present, no modification occurs.
