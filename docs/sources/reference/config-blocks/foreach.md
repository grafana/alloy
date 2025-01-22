---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/foreach/
description: Learn about foreach
menuTitle: foreach
title: foreach
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# foreach

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `foreach` block runs a separate pipeline for each item inside a list.

## Usage

```alloy
foreach "<LABEL>" {
  collection = [...]
  var        = "<VAR_NAME>"
  template {
    ...
  }
}
```

## Arguments

The following arguments are supported:

Name             | Type        | Description                                                           | Default | Required
-----------------|-------------|-----------------------------------------------------------------------|---------|---------
`collection`     | `list(any)` | A list of items to loop over.                                         |         | yes
`var`            | `string`    | Name of the variable referring to the current item in the collection. |         | yes
`enable_metrics` | `bool`      | Whether to expose debug metrics in the {{< param "PRODUCT_NAME" >}} `/metrics` endpoint.       | `false` | no

The items in the `collection` list can be of any type [type][types], such as a bool, a string, a list, or a map.

{{< admonition type="warning" >}}
Setting `enable_metrics` to `true` when `collection` has lots of elements may cause a large number of metrics to appear on the {{< param "PRODUCT_NAME" >}} `/metric` endpoint.
{{< /admonition >}}

[types]: ../../../get-started/configuration-syntax/expressions/types_and_values

## Blocks

The following blocks are supported inside the definition of `foreach`:

Hierarchy | Block        | Description                  | Required
----------|--------------|------------------------------|---------
template  | [template][] | A component pipeline to run. | yes

[template]: #template-block

### template

The `template` block contains the definition of {{< param "PRODUCT_NAME" >}} components which will be ran for every item in the collection.
The contents of the block look like a normal {{< param "PRODUCT_NAME" >}} configuration file,
except that you can use the keyword defined in `var` to refer to the current item in the collection.

Components inside the `template` block can use exports of components defined outside of the `foreach` block.
However, components outside of the `foreach` cannot use exports from components defined inside the `template` block of a `foreach`.

## Example

The following example shows you how to run Run Prometheus exporters dynamically on service discovery targets.

`prometheus.exporter.*` components often require the address of one particular instance being monitored.
For example, `prometheus.exporter.redis` has a `redis_addr` attribute for the Redis instance under observation.
On the other hand, `discovery.*` components such as `discovery.kubernetes` output a list of targets such as this:

{{< collapse title="Example targets output by discovery.kubernetes" >}}
```json
[
    {
    __address__                                          = "10.42.0.16:5432",
    __meta_kubernetes_namespace                          = "ns1",
    __meta_kubernetes_pod_container_id                   = "containerd://96b77d035d0bbe27bb173d8fc0c56d21965892a50e4e6eab9f6cffdb90b275fb",
    __meta_kubernetes_pod_container_image                = "postgres:bullseye",
    __meta_kubernetes_pod_container_init                 = "false",
    __meta_kubernetes_pod_container_name                 = "pgcont",
    __meta_kubernetes_pod_container_port_name            = "pg-db",
    __meta_kubernetes_pod_container_port_number          = "5432",
    __meta_kubernetes_pod_container_port_protocol        = "TCP",
    __meta_kubernetes_pod_controller_kind                = "ReplicaSet",
    __meta_kubernetes_pod_controller_name                = "postgres-db-cd54547b9",
    __meta_kubernetes_pod_host_ip                        = "172.25.0.2",
    __meta_kubernetes_pod_ip                             = "10.42.0.16",
    __meta_kubernetes_pod_label_name                     = "postgres-db",
    __meta_kubernetes_pod_label_pod_template_hash        = "cd54547b9",
    __meta_kubernetes_pod_labelpresent_name              = "true",
    __meta_kubernetes_pod_labelpresent_pod_template_hash = "true",
    __meta_kubernetes_pod_name                           = "postgres-db-cd54547b9-4zpds",
    __meta_kubernetes_pod_node_name                      = "k3d-asserts-test-server-0",
    __meta_kubernetes_pod_phase                          = "Running",
    __meta_kubernetes_pod_ready                          = "true",
    __meta_kubernetes_pod_uid                            = "7cdcacdc-4a2d-460a-b1fb-6340700c4cac",
    },
    {
    __address__                                          = "10.42.0.20:6379",
    __meta_kubernetes_namespace                          = "ns1",
    __meta_kubernetes_pod_container_id                   = "containerd://68f2f0eacd880eb4a141d833aafc1f297f7d9bdf00f4c787f9fcc964a039d278",
    __meta_kubernetes_pod_container_image                = "redis:latest",
    __meta_kubernetes_pod_container_init                 = "false",
    __meta_kubernetes_pod_container_name                 = "redis-cont",
    __meta_kubernetes_pod_container_port_name            = "redis-db",
    __meta_kubernetes_pod_container_port_number          = "6379",
    __meta_kubernetes_pod_container_port_protocol        = "TCP",
    __meta_kubernetes_pod_controller_kind                = "ReplicaSet",
    __meta_kubernetes_pod_controller_name                = "redis-db-778b66cb7d",
    __meta_kubernetes_pod_host_ip                        = "172.25.0.2",
    __meta_kubernetes_pod_ip                             = "10.42.0.20",
    __meta_kubernetes_pod_label_name                     = "redis-db",
    __meta_kubernetes_pod_label_pod_template_hash        = "778b66cb7d",
    __meta_kubernetes_pod_labelpresent_name              = "true",
    __meta_kubernetes_pod_labelpresent_pod_template_hash = "true",
    __meta_kubernetes_pod_name                           = "redis-db-778b66cb7d-wxmf6",
    __meta_kubernetes_pod_node_name                      = "k3d-asserts-test-server-0",
    __meta_kubernetes_pod_phase                          = "Running",
    __meta_kubernetes_pod_ready                          = "true",
    __meta_kubernetes_pod_uid                            = "ae74e400-8eda-4b02-b4c8-669473fb001b",
    }
]
```
{{< /collapse >}}

You can use a `foreach` to loop over each target and start a separate component pipeline for it.
The following example configuration shows how a `prometheus.exporter.redis` instance is started for each Redis instance discoverd by `discovery.kubernetes`.
Additional Kubernetes labels from `discovery.kubernetes` are also added to the metrics created by `prometheus.exporter.redis`.

```alloy
discovery.kubernetes "default" {
    role = "pod"
}

discovery.relabel "redis" {
    targets = discovery.kubernetes.default.targets

    // Remove all targets except the Redis ones.
    rule {
        source_labels = ["__meta_kubernetes_pod_container_name"]
        regex         = "redis-cont"
        action        = "keep"
    }
}

// Collect metrics for each Redis instance.
foreach "redis" {
    collection = discovery.relabel.redis.output
    var        = "each"

    template {
        prometheus.exporter.redis "default" {
            // This is the "__address__" label from discovery.kubernetes.
            redis_addr = each["__address__"]
        }

        prometheus.scrape "default" {
            targets    = prometheus.exporter.redis.default.targets
            forward_to = [prometheus.relabel.default.receiver]
        }

        // Add labels from discovery.kubernetes.
        prometheus.relabel "default" {
            rule {
                replacement  = each["__meta_kubernetes_namespace"]
                target_label = "k8s_namespace"
                action       = "replace"
            }

            rule {
                replacement  = each["__meta_kubernetes_pod_container_name"]
                target_label = "k8s_pod_container_name"
                action       = "replace"
            }

            forward_to = [prometheus.remote_write.mimir.receiver]
        }
    }
}

prometheus.remote_write "mimir" {
    endpoint {
        url = "https://prometheus-xxx.grafana.net/api/prom/push"

        basic_auth {
            username = sys.env("<PROMETHEUS_USERNAME>")
            password = sys.env("<GRAFANA_CLOUD_API_KEY>")
        }
    }
}
```

Replace the following:

* _`<PROMETHEUS_USERNAME>`_: Your Prometheus username.
* _`<GRAFANA_CLOUD_API_KEY>`_: Your Grafana Cloud API key.
