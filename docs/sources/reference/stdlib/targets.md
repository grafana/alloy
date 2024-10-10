---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/targets/
description: Learn about targets functions
menuTitle: targets
title: targets
---

# targets

The `targets` namespace contains functions related to `list(map(string))` arguments.
They are often used by [prometheus][prom-comp] and [discovery][disc-comp] components.
Refer to [Compatible components][] for a full list of components which export and consume targets.

[prom-comp]: ../components/prometheus/
[disc-comp]: ../components/discovery/
[Compatible components]: ../compatibility/#targets

## targets.merge

The `targets.inner_join` function allows you to join two arrays containing maps if certain keys have matching values in both maps.
It takes three inputs:

* The first two inputs are a of type `list(map(string))`. The keys of the map are strings. 
  The value for each key could have any Alloy type such as a string, integer, map, or a capsule.
* The third input is an array containing strings. The strings are the keys whose value has to match for maps to be joined.
  
  
If the set of keys don't identify a map uniquely, the resulting output may contain more maps than the total sum of maps from both input arrays.

### Examples

```alloy
targets.inner_join(discovery.kubernetes.k8s_pods.targets, prometheus.exporter.postgres, ["instance"])
```

```alloy
targets.inner_join(prometheus.exporter.redis.default.targets, [{"instance"="1.1.1.1", "testLabelKey" = "testLabelVal"}], ["instance"])
```