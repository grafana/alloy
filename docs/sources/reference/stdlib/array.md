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

```alloy
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

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `array.combine_maps` function allows you to join two arrays of maps if certain keys have matching values in both maps. It's particularly useful when combining labels of targets coming from different `prometheus.discovery.*` or `prometheus.exporter.*` components.
It takes three arguments:

* The first two arguments are a of type `list(map(string))`. The keys of the map are strings.
  The value for each key could be of any Alloy type such as a `string`, `integer`, `map`, or a `capsule`.
* The third input is an `array` containing strings. The strings are the keys whose value has to match for maps to be combined.
* (optional) The fourth input is a `boolean` which defaults to `false`.
  When it is set to `true`, each item from the first argument will be passed through even if there is no match.
  This is helpful if you want to enrich the first list with attributes from the second list, without losing any of the information from the first list.

The maps that don't contain all the keys provided in the third argument will be discarded. When maps are combined and both contain the same keys, the last value from the second argument will be used.

Pseudo function code:

```text
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
> array.combine_maps(prometheus.exporter.postgres.example.targets, discovery.kubernetes.k8s_pods.targets, ["instance"], true)

> array.combine_maps(prometheus.exporter.redis.default.targets, [{"instance"="1.1.1.1", "testLabelKey" = "testLabelVal"}], ["instance"], true)
```

In the examples above the fourth argument is set to `true` so that the original targets from the exporters will still be preserved and scraped
even if they cannot be enriched with values from the second argument.

You can find more examples in the [tests][].

{{< admonition type="note" >}}
`array.combine_maps` can be useful for enriching Prometheus service discovery targets prior to a Prometheus scrape.
It cannot be used to enrich Prometheus metrics with labels from service discovery components.
You can use the [prometheus.enrich][] component for this purpose.

[prometheus.enrich](../components/prometheus/prometheus.enrich)
{{< /admonition >}}

[tests]: https://github.com/grafana/alloy/blob/main/syntax/vm/vm_stdlib_test.go

## array.group_by

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `array.group_by` function groups an array of objects by a given key.

* The first argument is an array of objects.
* The second argument is a string that is the key to group by. The value of the key must be a string and should be present at the top level of the object.
* The third argument is a boolean that indicates whether the elements that don't match the key should be dropped (true) or added to the empty group (false).

### Examples

```alloy
> array.group_by([{"type" = "fruit", "name" = "apple"}, {"type" = "fruit", "name" = "banana"}, {"type" = "vegetable", "name" = "carrot"}, {"name" = "rock"}], "type", false)
[{"type" = "fruit", "items" = [{"type" = "fruit", "name" = "apple"}, {"type" = "fruit", "name" = "banana"}]}, {"type" = "vegetable", "items" = [{"type" = "vegetable", "name" = "carrot"}]}, {"type" = "", "items" = [{"name" = "rock"}]}]

> array.group_by([{"type" = "fruit", "name" = "apple"}, {"type" = "fruit", "name" = "banana"}, {"type" = "vegetable", "name" = "carrot"}, {"name" = "rock"}], "type", true)
[{"type" = "fruit", "items" = [{"type" = "fruit", "name" = "apple"}, {"type" = "fruit", "name" = "banana"}]}, {"type" = "vegetable", "items" = [{"type" = "vegetable", "name" = "carrot"}]}]
```

The following example shows how to use the `array.group_by` function with a `foreach` block to group targets by match labels and create a `prometheus.scrape` component for each group dynamically.
The targets in this example should have a label "match" that contains instant vector selectors separated by slash (refer to [Federation][federation] for more information on the match parameter).

```alloy
foreach "federation" {
 collection = array.group_by(discovery.file.example.targets, "match", false)
 var = "each"
 id  = "match"

 template {
   prometheus.scrape "default" {
     targets    = each["items"]
     honor_labels = true
     metrics_path = "/federate"
     params = {
       "match[]" = string.split(coalesce(each["match"], "{__name__!=\"\"}"), "/"),
     }
     forward_to = [prometheus.remote_write.default.receiver]
   }
 }
}
```

[federation]: https://prometheus.io/docs/prometheus/latest/federation/#configuring-federation