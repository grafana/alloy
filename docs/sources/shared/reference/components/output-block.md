---
aliases:
- ../../otelcol/output-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/output-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/output-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/output-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/output-block/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/output-block/
description: Shared content, output block
headless: true
---

The `output` block configures a set of components to forward resulting telemetry data to.

The following arguments are supported:

Name      | Type                     | Description                           | Default | Required
----------|--------------------------|---------------------------------------|---------|---------
`logs`    | `list(otelcol.Consumer)` | List of consumers to send logs to.    | `[]`    | no
`metrics` | `list(otelcol.Consumer)` | List of consumers to send metrics to. | `[]`    | no
`traces`  | `list(otelcol.Consumer)` | List of consumers to send traces to.  | `[]`    | no

You must specify the `output` block, but all its arguments are optional.
By default, telemetry data is dropped.
Configure the `metrics`, `logs`, and `traces` arguments accordingly to send telemetry data to other components.
