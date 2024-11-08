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

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `array.combine_maps` function allows you to join two arrays of maps if certain keys have matching values in both maps. It's particularly useful when combining labels of targets coming from different `prometheus.discovery.*` or `prometheus.exporter.*` components.
It takes three inputs:

* The first two inputs are a of type `list(map(string))`. The keys of the map are strings. 
  The value for each key could have any Alloy type such as a string, integer, map, or a capsule.
* The third input is an array containing strings. The strings are the keys whose value has to match for maps to be joined.

The maps that don't contain all the keys provided in the third input will be discarded.

### Examples

```alloy
array.combine_maps(discovery.kubernetes.k8s_pods.targets, prometheus.exporter.postgres, ["instance"])
```

```alloy
array.combine_maps(prometheus.exporter.redis.default.targets, [{"instance"="1.1.1.1", "testLabelKey" = "testLabelVal"}], ["instance"])
```
