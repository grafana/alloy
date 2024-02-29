---
aliases:
- ../../otelcol/output-block-metrics/
- /docs/grafana-cloud/agent/shared/flow/reference/components/output-block-metrics/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/output-block-metrics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/output-block-metrics/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/output-block-metrics/
description: Shared content, output block metrics
headless: true
---

The `output` block configures a set of components to forward resulting telemetry data to.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`metrics` | `list(otelcol.Consumer)` | List of consumers to send metrics to. | `[]` | no

You must specify the `output` block, but all its arguments are optional.
By default, telemetry data is dropped.
Configure the `metrics` argument accordingly to send telemetry data to other components.
