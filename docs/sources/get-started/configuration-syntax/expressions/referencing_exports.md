---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/expressions/referencing_exports/
aliases:
  - ../../../concepts/configuration-syntax/expressions/referencing_exports/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/referencing_exports/
description: Learn about referencing component exports
title: Referencing component exports
weight: 200
---

# Referencing component exports

Referencing exports enables {{< param "PRODUCT_NAME" >}} to configure and connect components dynamically using expressions.
While components can work in isolation, they're more useful when one component's behavior and data flow are bound to the exports of another, building a dependency relationship between the two.

Such references can only appear as part of another component's arguments or a configuration block's fields.
Components can't reference themselves.

## Using references

You build references by combining the component's name, label, and named export with dots.

For example, you can reference the contents of a file exported by the `local.file` component labeled `target` as `local.file.target.content`.
Similarly, a `prometheus.remote_write` component instance labeled `onprem` exposes its receiver for metrics on `prometheus.remote_write.onprem.receiver`.

The following example shows some references.

```alloy
local.file "target" {
  filename = "/etc/alloy/target"
}

prometheus.scrape "default" {
  targets    = [{ "__address__" = local.file.target.content }]
  forward_to = [prometheus.remote_write.onprem.receiver]
}

prometheus.remote_write "onprem" {
  endpoint {
    url = "http://prometheus:9009/api/prom/push"
  }
}
```

In the preceding example, you wired together a very simple pipeline by writing a few {{< param "PRODUCT_NAME" >}} expressions.

{{< figure src="/media/docs/alloy/diagram-referencing-exports.png" alt="Example of a pipeline" >}}

After the value is resolved, it must match the [type][] of the attribute it is assigned to.
While you can only configure attributes using the basic {{< param "PRODUCT_NAME" >}} types,
the exports of components can take on special internal {{< param "PRODUCT_NAME" >}} types, such as Secrets or Capsules, which expose different functionality.

[type]: ../types_and_values/
