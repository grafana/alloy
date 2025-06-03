---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/expressions/referencing_exports/
aliases:
  - ../../../concepts/configuration-syntax/expressions/referencing_exports/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/referencing_exports/
description: Learn about referencing component exports
title: Referencing component exports
weight: 200
---

# Reference component exports

Referencing exports allows {{< param "PRODUCT_NAME" >}} to configure and connect components dynamically using expressions.
While components can work independently, they're more effective when one component's behavior and data flow depend on another component's exports, creating a dependency relationship.

Such references can only appear in another component's arguments or a configuration block's fields.
Components can't reference themselves.

## Use references

You create references by combining the component's name, label, and named export with dots.

For example, you can refer to the contents of a file exported by the `local.file` component labeled `target` as `local.file.target.content`.
Similarly, a `prometheus.remote_write` component instance labeled `onprem` exposes its receiver for metrics as `prometheus.remote_write.onprem.receiver`.

The following example demonstrates some references.

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

In the preceding example, you created a simple pipeline by writing a few {{< param "PRODUCT_NAME" >}} expressions.

{{< figure src="/media/docs/alloy/diagram-referencing-exports.png" alt="Example of a pipeline" >}}

After the value resolves, it must match the [type][] of the attribute it's assigned to.
While you can only configure attributes using the basic {{< param "PRODUCT_NAME" >}} types, the exports of components can use special internal {{< param "PRODUCT_NAME" >}} types, such as Secrets or Capsules, which provide additional functionality.

[type]: ../types_and_values/
