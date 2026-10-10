---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape-class/
description: Shared content, prom operator scrape class
headless: true
---

The `scrape_class` block defines a named set of scrape settings that discovered resources can reference through their `scrapeClass` field.
This mirrors the [Prometheus Operator ScrapeClass](https://prometheus-operator.dev/docs/developer/scrapeclass/) feature.
You can define multiple `scrape_class` blocks.

| Name      | Type     | Description                                                                                 | Default | Required |
| --------- | -------- | ------------------------------------------------------------------------------------------- | ------- | -------- |
| `name`    | `string` | Unique, non-empty name of the scrape class, referenced by a resource's `scrapeClass` field. |         | yes      |
| `default` | `bool`   | Apply this class to resources that don't reference a scrape class.                          | `false` | no       |

At most one `scrape_class` block can set `default` to `true`.
A resource's own settings take precedence over the scrape class.
Scrape class relabeling rules and metric relabeling rules are prepended to the resource's own rules, matching the Prometheus Operator.
Referencing a scrape class that isn't defined is an error.

Use a scrape class when several discovered resources need the same scrape settings.
For example, you can define one class that holds the TLS configuration for a service mesh, then reference that class from every resource in the mesh instead of repeating the certificate paths in each one.
Because relabeling rules are prepended, you can also use a scrape class to apply baseline relabeling or metric filtering across many resources at once.

The following example defines a default class that holds the TLS configuration for a service mesh, and a second class that adds node metadata on top of the same TLS settings:

```alloy
scrape_class {
  name    = "mesh"
  default = true

  tls_config {
    ca_file   = "/etc/certs/ca.crt"
    cert_file = "/etc/certs/client.crt"
    key_file  = "/etc/certs/client.key"
  }
}

scrape_class {
  name = "mesh-with-node-metadata"

  tls_config {
    ca_file   = "/etc/certs/ca.crt"
    cert_file = "/etc/certs/client.crt"
    key_file  = "/etc/certs/client.key"
  }

  attach_metadata {
    node = true
  }
}
```

Resources that don't set a `scrapeClass` field use the `mesh` class.
Resources that set `scrapeClass` to `mesh-with-node-metadata` use that class instead.
Each resource resolves to exactly one class, so the second class repeats the TLS settings rather than inheriting them from the default.

Scrape classes are most useful when you don't own the discovered resources.
If other teams own the resources and you own the {{< param "PRODUCT_NAME" >}} configuration, set `default` to `true` so your settings apply to every resource that doesn't reference a class.
