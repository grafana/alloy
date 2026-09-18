---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-keepalive-client-block/
description: Shared content, otelcol keepalive client block
headless: true
---

The following arguments are supported:

| Name                      | Type       | Description                                                            | Default | Required |
| ------------------------- | ---------- | ------------------------------------------------------------------------ | ------- | -------- |
| `idle_conn_timeout`       | `duration` | Time to wait before an idle connection closes itself.                  | `"90s"` | no       |
| `max_idle_conns`          | `int`      | Limits the number of idle HTTP connections the client can keep open.   | `100`   | no       |
| `max_idle_conns_per_host` | `int`      | Limits the number of idle HTTP connections the host can keep open.     | `0`     | no       |

When set, `keepalive` takes precedence over the deprecated `idle_conn_timeout`, `max_idle_conns`, and
`max_idle_conns_per_host` arguments, which are then ignored.
