---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/array/
description: Learn about array functions
aliases:
  - ./concat/ # /docs/alloy/latest/reference/stdlib/concat/
menuTitle: array
title: array
---

# array

The `array` namespace contains functions related to arrays.

## array.concat

The `array.concat` function concatenates one or more lists of values into a single list.
Each argument to `array.concat` must be a list value.
Elements within the list can be any type.

### Examples

```
> array.concat([])
[]

> array.concat([1, 2], [3, 4])
[1, 2, 3, 4]

> array.concat([1, 2], [], [bool, null])
[1, 2, bool, null]

> array.concat([[1, 2], [3, 4]], [[5, 6]])
[[1, 2], [3, 4], [5, 6]]
```

## array.combine_maps

> **EXPERIMENTAL**: This is an [experimental][] feature. Experimental
> features are subject to frequent breaking changes, and may be removed with
> no equivalent replacement. The `stability.level` flag must be set to `experimental`
> to use the feature.

The `array.combine_maps` function allows you to join two arrays of maps if certain keys have matching values in both maps. It's particularly useful when combining labels of targets coming from different `prometheus.discovery.*` or `prometheus.exporter.*` components.
It takes three arguments:

* The first two arguments are a of type `list(map(string))`. The keys of the map are strings. 
  The value for each key could be of any Alloy type such as a `string`, `integer`, `map`, or a `capsule`.
* The third input is an `array` containing strings. The strings are the keys whose value has to match for maps to be combined.

The maps that don't contain all the keys provided in the third argument will be discarded. When maps are combined and both contain the same keys, the last value from the second argument will be used.

Pseudo function code:
```
for every map in arg1:
  for every map in arg2:
    if the condition key matches in both:
       merge maps and add to result
```

### Examples

```alloy
> array.combine_maps([{"instance"="1.1.1.1", "team"="A"}], [{"instance"="1.1.1.1", "cluster"="prod"}], ["instance"])
[{"instance"="1.1.1.1", "team"="A", "cluster"="prod"}]

// Second map overrides the team in the first map
> array.combine_maps([{"instance"="1.1.1.1", "team"="A"}], [{"instance"="1.1.1.1", "team"="B"}], ["instance"])
[{"instance"="1.1.1.1", "team"="B"}]

// If multiple maps from the first argument match with multiple maps from the second argument, different combinations will be created.
> array.combine_maps([{"instance"="1.1.1.1", "team"="A"}, {"instance"="1.1.1.1", "team"="B"}], [{"instance"="1.1.1.1", "cluster"="prod"}, {"instance"="1.1.1.1", "cluster"="ops"}], ["instance"])
[{"instance"="1.1.1.1", "team"="A", "cluster"="prod"}, {"instance"="1.1.1.1", "team"="A", "cluster"="ops"}, {"instance"="1.1.1.1", "team"="B", "cluster"="prod"}, {"instance"="1.1.1.1", "team"="B", "cluster"="ops"}]
```

Examples using discovery and exporter components:
```alloy
> array.combine_maps(discovery.kubernetes.k8s_pods.targets, prometheus.exporter.postgres, ["instance"])

> array.combine_maps(prometheus.exporter.redis.default.targets, [{"instance"="1.1.1.1", "testLabelKey" = "testLabelVal"}], ["instance"])
```

You can find more examples in the [tests][].

[tests]: https://github.com/grafana/alloy/blob/main/syntax/vm/vm_stdlib_test.go
[experimental]: https://grafana.com/docs/release-life-cycle/