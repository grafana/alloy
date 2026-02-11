---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/pipelines/
description: Learn how telemetry flows through Grafana Alloy pipelines from ingestion to export
menuTitle: Pipelines
title: Telemetry pipelines in Grafana Alloy
weight: 200
---

# Telemetry pipelines in Grafana Alloy

Purpose

Explain how telemetry flows through Alloy from ingestion to export.

Content Outline

1. Ingestion

Explain:

Receivers accept telemetry.

Telemetry enters Alloy in signal-specific formats.

Alloy does not generate telemetry on its own.

2. Processing

Explain:

Processing components sit between receivers and exporters.

They can modify, filter, or forward telemetry.

If no processing components are inserted, telemetry passes through unchanged.

This page introduces the concept of modification without categorizing it.

3. Export

Explain:

Exporters deliver telemetry to external systems.

Delivery success or failure occurs at exporter level.

Pipelines may have multiple exporters.
