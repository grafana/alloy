# Proposal: Alloy proposal process

* Author: Paulin Todev (@ptodev), Piotr Gwizdala (@thampiotr)
* Last updated: 2024-08-15
* Original issue: https://github.com/grafana/alloy/issues/1443

## Abstract

We are proposing a new feature to the [Alloy standard library][stdlib].
It will be similar to a `map` operation over a collection such as a `list()`.
Each `map` transformation will be done by a chain of components (a "sub-pipeline") created for this transformation.
Each item in the collection will be processed by a different "sub-pipeline".

The final solution may differ from a standard `map` operation, since there may be multiple outputs for the same input.
For example, the sub-pipeline may branch into different `prometheus.relabel` components,
each of which sends outputs to different components outside of the sub-pipeline.

[stdlib]: https://grafana.com/docs/alloy/latest/reference/stdlib/

## Use cases

<!-- TODO: Add more use cases. It'd be helpful to gather feedback from the community and from solutions engineers. -->

### Using discovery components together with prometheus.exporter ones

Discovery components output a list of targets. It's not possible to input those lists directly to most exporter components.

Suppose we have a list of targets produced by a `discovery` component:

```
[
  {"__address__" = "redis-one:9115", "instance" = "one"},
  {"__address__" = "redis-two:9116", "instance" = "two"},
]
```

The [Alloy type][alloy-types] of the list above is `list(map(string))`.
However, you may want to pipe information from this list of targets to a component which doesn't work with a `list()` or a `map()`.
For example, you may want to input the `"__address__"` string to a `prometheus.exporter.redis`,
and you may want to use the `"instance"` string in a `discovery.relabel`.

[alloy-types]: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/expressions/types_and_values/

## Proposal 1: A foreach block

A `foreach` block may start several sub-pipelines for a `collection` specified in its arguments.

```alloy
// All components in the sub-pipeline will be scoped under "foreach.default/1/...".
// Here, "1" is sub-pipeline number 1.
// This way component names won't clash with other sub-pipelines from the same foreach, 
// and with the names of components outside of the foreach.
foreach "default" {
    
  // "collection" is what the for loop will iterate over.
  collection = discovery.file.default.targets

  // Each item in the collection will be accessible via the "target" variable.
  // E.g. `target["__address__"]`.
  var = "target"

  // A sub-pipeline consisting of components which process each target.
  ...
}
```

<details>
  <summary>Example</summary>

```alloy
discovery.file "default" {
  files = ["/Users/batman/Desktop/redis_addresses.yaml"]
}

// Every component defined in the "foreach" block will be instantiated for each item in the collection. 
// The instantiated components will be scoped using the name of the foreach block and the index of the
// item in the collection. For example: /foreach.redis/0/prometheus.exporter.redis.default
foreach "redis" {
  collection = discovery.file.default.targets
  // Here, "target" is a variable whose value is the current item in the collection.
  var = "target"

  prometheus.exporter.redis "default" {
    redis_addr = target["__address__"] // we can also do the necessary rewrites before this.
  }

  discovery.relabel "default" {
    targets = prometheus.exporter.redis.default.targets
    // Add a label which comes from the discovery component.
    rule {
      target_label = "filepath"
      // __meta_filepath comes from discovery.file
      replacement  = target["__meta_filepath"]
    }
  }

  prometheus.scrape "default" {
    targets = discovery.relabel.default.targets
    forward_to = prometheus.remote_write.mimir.receiver
  }
}

prometheus.remote_write "mimir" {
  endpoint {
    url = "https://prometheus-prod-05-gb-south-0.grafana.net/api/prom/push"
    basic_auth {
      username = ""
      password = ""
    }
  }
}
```

</details>

Pros:
* The `foreach` name is consistent with other programming languages.

Cons:
* It looks less like a component than a `declare.dynamic` block.
  In order to instantiate multiple `foreach` blocks with similar config, you'd have to wrap them in a `declare` block.

## Proposal 2: A declare.dynamic block

A new `declare.dynamic` block would create a custom component which starts several sub-pipelines internally.
Users can use `argument` and `export` blocks, just like in a normal `declare` block.

```alloy
declare.dynamic "ex1" {
  argument "input_targets" {
    optional = false
    comment = "We will create a sub-pipeline for each target in input_targets."
  }

  argument "output_metrics" {
    optional = false
    comment = "All the metrics gathered from all pipelines."
  }

  // A sub-pipeline consisting of components which process each target.
  ...
}

declare.dynamic.ex1 "default" {
  input_targets = discovery.file.default.targets
  output_metrics = [prometheus.remote_write.mimir.receiver]
}
```

<details>
  <summary>Example</summary>

```alloy
// declare.dynamic "maps" each target to a sub-pipeline.
// Each sub-pipeline has 1 exporter, 1 relabel, and 1 scraper.
// Internally, maybe one way this can be done via serializing the pipeline to a string and then importing it as a module?
declare.dynamic "redis_exporter" {
  argument "input_targets" {
    optional = false
    comment = "We will create a sub-pipeline for each target in input_targets."
  }

  argument "output_metrics" {
    optional = false
    comment = "All the metrics gathered from all pipelines."
  }

  // "id" is a special identifier for every "sub-pipeline".
  // The number of "sub-pipelines" is equal to len(input_targets).
  prometheus.exporter.redis id {
    redis_addr = input_targets["__address__"]
  }

  discovery.relabel id {
    targets = prometheus.exporter.redis[id].targets
    // Add a label which comes from the discovery component.
    rule {
      target_label = "filepath"
      // __meta_filepath comes from discovery.file
      replacement  = input_targets["__meta_filepath"]
    }
  }

  prometheus.scrape id {
    targets = prometheus.exporter.redis[id].targets
    forward_to = output_metrics
  }

}
discovery.file "default" {
  files = ["/Users/batman/Desktop/redis_addresses.yaml"]
}

declare.dynamic.redis_exporter "default" {
  input_targets = discovery.file.default.targets
  output_metrics = [prometheus.remote_write.mimir.receiver]
}

prometheus.remote_write "mimir" {
  endpoint {
    url = "https://prometheus-prod-05-gb-south-0.grafana.net/api/prom/push"
    basic_auth {
      username = ""
      password = ""
    }
  }
}
```

</details>

Pros:
* Looks more like a component than a `foreach` block.
* Flexible number of inputs and outputs.

Cons:
* A name such as `declare.dynamic` doesn't sound as familiar to most people than `foreach`.
* It may not be practical to implement this in a way that there's more than one possible input collection.
  * How can we limit users to having just one collection?
* Having another variant of the `declare` block can feel complex.
  Can we just add this functionality to the normal `declare` block, so that we can avoid having a `declare.dynamic` block?

## Proposal 3: Do nothing

It is customary to always include a "do nothing" proposal, in order to evaluate if the work is really required.

Pros:
* No effort required.
* Alloy's syntax is simpler since we're not adding any new types of blocks.

Cons:
* Not possible to integrate most `prometheus.exporter` components with the `discovery` ones.

## Unknowns

We should find answers to the unknowns below before this proposal is accepted:

* Will the solution only work for `list()`? What about `map()`?
  * If we go with a `foreach`, we could have a `key` attribute in addition to the `var` one. That way we can also access the key. The `key` attribute can be a no-op if `collection` is a map?
* What about debug metrics? Should we aggregate the metrics for all "sub-pipelines"?
  * If there is 1 series for each sub-pipeline, the amount of metrics could be huge. 
    Some service discovery mechanisms may generate a huge number of elements in a list of targets.
  * If we want to aggregate the metrics, how would we do that? Is it even possible to do in within Alloy?
  * Can we have a configuration parameter which dictates whether the metrics should be aggregated or not?
* Do we have to recreate the sub-pipelines every time a new collection is received,
  even if the new collection has the same number of elements?
* Do we need to have more than one output, of a different type?
* Do we need to have more than one input, of a different type?

## Recommended solution

<!-- TODO: Fill this later -->