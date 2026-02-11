---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/modify-telemetry/
description: Learn where and how telemetry is modified in Grafana Alloy processing stages
menuTitle: Modify telemetry
title: Where telemetry is modified
weight: 300
---

# Where telemetry is modified

Purpose

This is the anchor page for future expansion (but still within #5430 scope).

It should:

Clarify that all transformation happens in processing components.

Emphasize that modification is explicit.

Avoid listing all capability categories (that belongs to #5494).

Content Outline

1. Modification occurs in processing stages

Receivers ingest.

Processing components modify.

Exporters deliver.

2. Modification is explicit and configurable

Alloy does not automatically redact, filter, or rewrite data.

If telemetry is changed, it is because the configuration specifies it.

3. Signal-specific behavior

Logs, metrics, and traces have distinct processing chains.

Processing components are signal-aware.

This page creates a conceptual hook for later capability grouping without implementing it now.
