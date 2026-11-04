---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-cookies-client-block/
description: Shared content, otelcol cookies client block
headless: true
---

The following arguments are supported:

| Name      | Type   | Description                   | Default | Required |
| --------- | ------ | ----------------------------- | ------- | -------- |
| `enabled` | `bool` | Deprecated and has no effect. | n/a     | no       |

{{< admonition type="caution" >}}
Starting with Alloy v1.22, the `enabled` argument is deprecated.
It will be removed in a future release.

Cookies are turned on whenever the `cookies` block is present, regardless of the value of `enabled`.
{{< /admonition >}}
