---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-keepalive-server-block/
description: Shared content, otelcol keepalive server block
headless: true
---

The following arguments are supported:

| Name          | Type       | Description                                                | Default | Required |
| ------------- | ---------- | ------------------------------------------------------------ | ------- | -------- |
| `idle_timeout` | `duration` | Maximum idle time before closing a keep-alive connection. | `"1m"`  | no       |

When set, `keepalive` takes precedence over the deprecated `idle_timeout` argument, which is then ignored.
HTTP server keep-alives are always enabled. The deprecated `keep_alives_enabled` argument returns an error
when you set it to `false`, because the OpenTelemetry Collector no longer supports disabling them.
