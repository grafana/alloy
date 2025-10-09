---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/referencing_exports/
aliases:
  - ./configuration-syntax/expressions/referencing_exports/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/referencing_exports/
description: Learn about referencing component exports
title: Reference component exports
weight: 20
---

# Reference component exports

Referencing exports allows {{< param "PRODUCT_NAME" >}} to configure and connect components dynamically using expressions.
Components are more effective when they depend on other components' exports, creating data flow relationships.

References can only appear in component arguments or configuration block fields.
Components can't reference themselves.

## Use references

You create references by combining the component's name, label, and named export with dots.

For example, you can refer to the contents of a file exported by the `local.file` component labeled `target` as `local.file.target.content`.
Similarly, a `prometheus.remote_write` component instance labeled `onprem` exposes its receiver for metrics as `prometheus.remote_write.onprem.receiver`.

The following example demonstrates references:

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

In this example, you created a basic pipeline by writing a few {{< param "PRODUCT_NAME" >}} expressions.

{{< figure src="/media/docs/alloy/diagram-referencing-exports.png" alt="Example of a pipeline" >}}

After the value resolves, it must match the [type][] of the attribute it's assigned to.
While you can only configure attributes using the basic {{< param "PRODUCT_NAME" >}} types, the exports of components can use special internal {{< param "PRODUCT_NAME" >}} types, such as Secrets or Capsules, which provide additional functionality.

## Next steps

Learn more about building expressions:

- [Types and values][] to understand component export types and value compatibility
- [Operators][] to combine and manipulate component exports in expressions
- [Function calls][] to transform component exports using standard library functions

[type]: ../types_and_values/
[Types and values]: ./types_and_values/
[Operators]: ./operators/
[Function calls]: ./function_calls/
