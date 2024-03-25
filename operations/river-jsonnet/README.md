# `river-jsonnet` library

The `river-jsonnet` library makes it possible to return River-formatted config
files using Jsonnet.

To manifest a River configuration file, call `river.manifestRiver(value)`.

Field names from objects are expected to follow one of the three forms:

* `<name>` for River attributes (e.g., `foobar`).
* `block <name>` for unlabeled River blocks (e.g., `block exporter.unix`)
* `block <name> <label>` for labeled River blocks (.e.g, `block prometheus.remote_write default`).

Instead of following these naming conventions, helper functions are provided to
make it easier:

* `river.attr(name)` returns a field name that can be used as an attribute.
* `river.block(name, label="", index=0)` returns a field name that represents a block.
  * The `index` parameter can be provided to make sure blocks get marshaled in
    a specific order. If two blocks have the same index, they will be ordered
    lexicographically by name and label.

In addition to the helper functions, `river.expr(literal)` is used to inject a
literal River expression, so that `river.expr('env("HOME")')` is manifested as
the literal River expression `env("HOME")`.

## Limitations

* Manifested River files always have attributes and object keys in
  lexicographic sort order, regardless of how they were defined in Jsonnet.
* The resulting River files are not pretty-printed to how the formatter would
  print files.

## Example

```jsonnet
local river = import 'github.com/grafana/alloy/operations/river-jsonnet/main.libsonnet';

river.manifestRiver({
  attr_1: "Hello, world!",

  [river.block("some_block", "foobar")]: {
    expr: river.expr('env("HOME")'),
    inner_attr_1: [0, 1, 2, 3],
    inner_attr_2: {
      first_name: "John",
      last_name: "Smith",
    },
  },
})
```

results in

```river
attr_1 = "Hello, world"
some_block "foobar" {
  expr = env("HOME")
  inner_attr_1 = [0, 1, 2, 3]
  inner_attr_2 = {
    "first_name" = "John",
    "last_name" = "Smith",
  }
}
```
