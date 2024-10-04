---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/map/
description: Learn about map functions
aliases:
  - ./concat/ # /docs/alloy/latest/reference/stdlib/concat/
menuTitle: map
title: map
---

# map

The `map` namespace contains functions related to objects.

## map.inner_join

The `map.inner_join` function allows you to join two arrays containing maps if certain keys have matching values in both maps.
It takes several inputs, in this order:

1. An array of maps (objects) which will be joined. The keys of the map are strings. 
   Its value could have any Alloy type such as a string, integer, map, or a capsule.
2. An array of maps (objects) which will be joined. The keys of the map are strings. 
   Its value could have any Alloy type such as a string, integer, map, or a capsule.
3. An array containing strings. The strings are the keys whose value has to match for maps to be joined.
   If the set of keys don't identify a map uniquely, the resulting output may contain more maps than the total sum of maps from both input arrays.
4. (optional; default: `"update"`) A merge strategy. It describes which value will be used if there is Can be set to either of:
   * `update`: If there is already a key with the same name in the first map, it will be updated with the value from the second map.
   * `none`: If both maps have different values for the same key, no such key will be present in the output map.
5. (optional; default: `[]`) An array containing strings. Only keys listed in the array will be present in the output map.
6. (optional; default: `[]`) An array containing strings. Keyls listed in the array will not be present in the output map.

### Examples

```alloy
map.inner_join(prometheus.exporter.unix.default.targets, [{"instance"="1.1.1.1", "testLabelKey" = "testLabelVal", "BadKey" = "BadVal"}], ["instance"], "update", [], ["BadKey"])
```