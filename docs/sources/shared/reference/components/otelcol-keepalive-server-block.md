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
Setting the `keepalive` block also enables keep-alives unconditionally, so it can't be combined with
`keep_alives_enabled = false`. To disable keep-alives, don't set the `keepalive` block, and set
`keep_alives_enabled` to `false` instead.
