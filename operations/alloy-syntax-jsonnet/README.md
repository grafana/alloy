# `alloy-syntax-jsonnet` library

The `alloy-syntax-jsonnet` library makes it possible to create Alloy syntax
config files using Jsonnet.

To manifest a configuration file, call `alloy.manifestAlloy(value)`.

Field names from objects are expected to follow one of the three forms:

* `<name>` for Alloy attributes (e.g., `foobar`).
* `block <name>` for unlabeled Alloy blocks (e.g., `block exporter.unix`)
* `block <name> <label>` for labeled Alloy blocks (.e.g, `block prometheus.remote_write default`).

Instead of following these naming conventions, helper functions are provided to
make it easier:

* `alloy.attr(name)` returns a field name that can be used as an attribute.
* `alloy.block(name, label="", index=0)` returns a field name that represents a block.
  * The `index` parameter can be provided to make sure blocks get marshaled in
    a specific order. If two blocks have the same index, they will be ordered
    lexicographically by name and label.

In addition to the helper functions, `alloy.expr(literal)` is used to inject a
literal Alloy expression, so that `alloy.expr('sys.env("HOME")')` is manifested as
the literal Alloy expression `sys.env("HOME")`.

## Limitations

* Manifested Alloy syntax files always have attributes and object keys in
  lexicographic sort order, regardless of how they were defined in Jsonnet.
* The resulting Alloy syntax files are not pretty-printed to how the formatter
  would print files.

## Example

```jsonnet
local alloy = import 'github.com/grafana/alloy/operations/alloy-syntax-jsonnet/main.libsonnet';

alloy.manifestAlloy({
  attr_1: "Hello, world!",

  [alloy.block("some_block", "foobar")]: {
    expr: alloy.expr('sys.env("HOME")'),
    inner_attr_1: [0, 1, 2, 3],
    inner_attr_2: {
      first_name: "John",
      last_name: "Smith",
    },
  },
})
```

results in

```alloy
attr_1 = "Hello, world"
some_block "foobar" {
  expr = sys.env("HOME")
  inner_attr_1 = [0, 1, 2, 3]
  inner_attr_2 = {
    "first_name" = "John",
    "last_name" = "Smith",
  }
}
```
