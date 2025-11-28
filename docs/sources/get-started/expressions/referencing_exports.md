---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/referencing_exports/
aliases:
  - ./configuration-syntax/expressions/referencing_exports/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/referencing_exports/
  - ./concepts/configuration-syntax/expressions/referencing_exports/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/referencing_exports/
description: Learn about referencing component exports
title: Component exports
weight: 20
---

# Component exports

You learned about expressions and how the component controller evaluates them in the previous topics.
Now you'll learn how to use component references. These are expressions that connect components by referencing their exports.

Component references create the connections that make {{< param "PRODUCT_NAME" >}} pipelines work.
When you reference one component's exports in another component's arguments, you establish a dependency relationship that the component controller manages automatically.

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

In the preceding example, you created a pipeline by writing a few {{< param "PRODUCT_NAME" >}} expressions:

1. The `local.file` component reads a file and exports its `content`.
1. The `prometheus.scrape` component references that content in its `targets` field.
1. The `prometheus.scrape` component exports a `receiver` for scraped metrics.
1. The `prometheus.remote_write` component receives and forwards those metrics to a remote endpoint.

When the component controller evaluates these components, it:

1. Evaluates the `local.file` component first because it has no dependencies.
1. Evaluates the `prometheus.scrape` component next, using the file content for its targets.
1. Evaluates the `prometheus.remote_write` component last, connecting it to receive metrics.

Each time the file content changes, the component controller automatically reevaluates the `prometheus.scrape` component with the new target value.

{{< figure src="/media/docs/alloy/diagram-referencing-exports.png" alt="Example of a pipeline" >}}

The component controller evaluates component references during the component evaluation process.
When you reference another component's export, the component controller ensures that:

1. The referenced component is evaluated first.
1. The reference resolves to the correct export value.
1. The resolved value's type is compatible with the target field.

After a reference resolves, the value must match the [type][] of the attribute it's assigned to.
While you can only configure attributes using the basic {{< param "PRODUCT_NAME" >}} types, component exports can use special internal {{< param "PRODUCT_NAME" >}} types, such as Secrets or Capsules, which provide additional functionality.

## Next steps

Now that you understand component references, continue learning about expressions:

- [Types and values][type] - Learn how the type system ensures references work correctly with different data types
- [Function calls][functions] - Use built-in functions to transform values from referenced exports
- [Operators][operators] - Combine referenced values with other expressions using mathematical and logical operators

[type]: ./types_and_values/
[functions]: ./function_calls/
[operators]: ./operators/
