---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.kubernetes/
aliases:
  - ../discovery.kubernetes/ # /docs/alloy/latest/reference/components/discovery.kubernetes/
description: Learn about discovery.kubernetes
labels:
  stage: general-availability
  products:
    - oss
title: discovery.kubernetes
---

# `discovery.kubernetes`

`discovery.kubernetes` allows you to find scrape targets from Kubernetes resources.
It watches cluster state and ensures targets are continually synced with what's running in your cluster.

If you supply no connection information, this component defaults to an in-cluster configuration.
You can use a `kubeconfig` file or manual connection settings to override the defaults.

## Performance considerations

By default, `discovery.kubernetes` discovers resources across all namespaces in your cluster.

{{< admonition type="caution" >}}
In DaemonSet deployments, each {{< param "PRODUCT_NAME" >}} Pod discovers and watches all resources across the cluster by default.
This can significantly increase API server load and memory usage, and may cause API throttling on managed Kubernetes services such as Azure Kubernetes Service (AKS), Amazon Elastic Kubernetes Service (EKS), or Google Kubernetes Engine (GKE).
{{< /admonition >}}

For better performance and reduced API load:

- Use the [`namespaces`](#namespaces) block to limit discovery to specific namespaces.
- Use [`selectors`](#selectors) to filter resources by labels or fields.  
- Consider the node-local example in [Limit to only Pods on the same node](#limit-to-only-pods-on-the-same-node).
- Use [`discovery.kubelet`](../discovery.kubelet/) for DaemonSet deployments to discover only Pods on the local node.
- Use clustering mode for larger deployments to distribute the discovery load.
- Monitor API server metrics like request rate, throttling, and memory usage, especially on managed clusters.

## Usage

```alloy
discovery.kubernetes "<LABEL>" {
  role = "<DISCOVERY_ROLE>"
}
```

## Arguments

You can use the following arguments with `discovery.kubernetes`:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `role`                   | `string`            | Type of Kubernetes resource to query.                                                            |         | yes      |
| `api_server`             | `string`            | URL of Kubernetes API server.                                                                    |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `kubeconfig_file`        | `string`            | Path of kubeconfig file to use for connecting to Kubernetes.                                     |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

 [arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `role` argument is required to specify what type of targets to discover.
`role` must be one of `node`, `pod`, `service`, `endpoints`, `endpointslice`, or `ingress`.

### `node` role

The `node` role discovers one target per cluster node with the address defaulting to the HTTP port of the `kubelet` daemon.
The target address defaults to the first address of the Kubernetes node object in the address type order of `NodeInternalIP`, `NodeExternalIP`, `NodeLegacyHostIP`, and `NodeHostName`.

The following labels are included for discovered nodes:

* `__meta_kubernetes_node_address_<address_type>`: The first address for each node address type, if it exists.
* `__meta_kubernetes_node_annotation_<annotationname>`: Each annotation from the node object.
* `__meta_kubernetes_node_annotationpresent_<annotationname>`: Set to `true` for each annotation from the node object.
* `__meta_kubernetes_node_label_<labelname>`: Each label from the node object.
* `__meta_kubernetes_node_labelpresent_<labelname>`: Set to `true` for each label from the node object.
* `__meta_kubernetes_node_name`: The name of the node object.
* `__meta_kubernetes_node_provider_id`: The cloud provider's name for the node object.

In addition, the `instance` label for the node is set to the node name as retrieved from the API server.

### `service` role

The `service` role discovers a target for each service port for each service.
This is generally useful for externally monitoring a service.
The address is set to the Kubernetes DNS name of the service and respective service port.

The following labels are included for discovered services:

* `__meta_kubernetes_namespace`: The namespace of the service object.
* `__meta_kubernetes_service_annotation_<annotationname>`: Each annotation from the service object.
* `__meta_kubernetes_service_annotationpresent_<annotationname>`: `true` for each annotation of the service object.
* `__meta_kubernetes_service_cluster_ip`: The cluster IP address of the service. This doesn't apply to services of type `ExternalName`.
* `__meta_kubernetes_service_external_name`: The DNS name of the service. This only applies to services of type `ExternalName`.
* `__meta_kubernetes_service_label_<labelname>`: Each label from the service object.
* `__meta_kubernetes_service_labelpresent_<labelname>`: `true` for each label of the service object.
* `__meta_kubernetes_service_name`: The name of the service object.
* `__meta_kubernetes_service_port_name`: Name of the service port for the target.
* `__meta_kubernetes_service_port_number`: Number of the service port for the target.
* `__meta_kubernetes_service_port_protocol`: Protocol of the service port for the target.
* `__meta_kubernetes_service_type`: The type of the service.

### `pod` role

The `pod` role discovers all Pods and exposes their containers as targets.
For each declared port of a container, a single target is generated.

If a container has no specified ports, a port-free target per container is created.
These targets must have a port manually injected using a [`discovery.relabel` component][discovery.relabel] before metrics can be collected from them.

[discovery.relabel]: ../discovery.relabel/

The following labels are included for discovered Pods:

* `__meta_kubernetes_namespace`: The namespace of the Pod object.
* `__meta_kubernetes_pod_annotation_<annotationname>`: Each annotation from the Pod object.
* `__meta_kubernetes_pod_annotationpresent_<annotationname>`: `true` for each annotation from the Pod object.
* `__meta_kubernetes_pod_container_id`: ID of the container the target address points to. The ID is in the form `<type>://<container_id>`.
* `__meta_kubernetes_pod_container_image`: The image the container is using.
* `__meta_kubernetes_pod_container_init`: `true` if the container is an `InitContainer`.
* `__meta_kubernetes_pod_container_name`: Name of the container the target address points to.
* `__meta_kubernetes_pod_container_port_name`: Name of the container port.
* `__meta_kubernetes_pod_container_port_number`: Number of the container port.
* `__meta_kubernetes_pod_container_port_protocol`: Protocol of the container port.
* `__meta_kubernetes_pod_controller_kind`: Object kind of the Pod controller.
* `__meta_kubernetes_pod_controller_name`: Name of the Pod controller.
* `__meta_kubernetes_pod_host_ip`: The current host IP of the Pod object.
* `__meta_kubernetes_pod_ip`: The Pod IP of the Pod object.
* `__meta_kubernetes_pod_label_<labelname>`: Each label from the Pod object.
* `__meta_kubernetes_pod_labelpresent_<labelname>`: `true` for each label from the Pod object.
* `__meta_kubernetes_pod_name`: The name of the Pod object.
* `__meta_kubernetes_pod_node_name`: The name of the node the Pod is scheduled onto.
* `__meta_kubernetes_pod_phase`: Set to `Pending`, `Running`, `Succeeded`, `Failed` or `Unknown` in the lifecycle.
* `__meta_kubernetes_pod_ready`: Set to `true` or `false` for the Pod's ready state.
* `__meta_kubernetes_pod_uid`: The UID of the Pod object.

### `endpoints` role

The `endpoints` role discovers targets from listed endpoints of a service.
For each endpoint address one target is discovered per port.
If the endpoint is backed by a Pod, all container ports of a Pod are discovered as targets even if they aren't bound to an endpoint port.

The following labels are included for discovered endpoints:

* `__meta_kubernetes_endpoints_label_<labelname>`: Each label from the endpoints object.
* `__meta_kubernetes_endpoints_labelpresent_<labelname>`: `true` for each label from the endpoints object.
* `__meta_kubernetes_endpoints_name:` The names of the endpoints object.
* `__meta_kubernetes_namespace:` The namespace of the endpoints object.

* The following labels are attached for all targets discovered directly from the endpoints list:
  * `__meta_kubernetes_endpoint_address_target_kind`: Kind of the endpoint address target.
  * `__meta_kubernetes_endpoint_address_target_name`: Name of the endpoint address target.
  * `__meta_kubernetes_endpoint_hostname`: Hostname of the endpoint.
  * `__meta_kubernetes_endpoint_node_name`: Name of the node hosting the endpoint.
  * `__meta_kubernetes_endpoint_port_name`: Name of the endpoint port.
  * `__meta_kubernetes_endpoint_port_protocol`: Protocol of the endpoint port.
  * `__meta_kubernetes_endpoint_ready`: Set to `true` or `false` for the endpoint's ready state.

* If the endpoints belong to a service, all labels of the `service` role discovery are attached.
* For all targets backed by a Pod, all labels of the `pod` role discovery are attached.

### `endpointslice` role

The `endpointslice` role discovers targets from Kubernetes endpoint slices.
For each endpoint address referenced in the `EndpointSlice` object, one target is discovered.
If the endpoint is backed by a Pod, all container ports of a Pod are discovered as targets even if they're not bound to an endpoint port.

The following labels are included for discovered endpoint slices:

* `__meta_kubernetes_endpointslice_name`: The name of endpoint slice object.
* `__meta_kubernetes_namespace`: The namespace of the endpoints object.

* The following labels are attached for all targets discovered directly from the endpoint slice list:

  * `__meta_kubernetes_endpointslice_address_target_kind`: Kind of the referenced object.
  * `__meta_kubernetes_endpointslice_address_target_name`: Name of referenced object.
  * `__meta_kubernetes_endpointslice_address_type`: The IP protocol family of the address of the target.
  * `__meta_kubernetes_endpointslice_endpoint_conditions_ready`: Set to `true` or `false` for the referenced endpoint's ready state.
  * `__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname`: Name of the node hosting the referenced endpoint.
  * `__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname`: `true` if the referenced object has a `kubernetes.io/hostname` annotation.
  * `__meta_kubernetes_endpointslice_endpoint_hostname`: Hostname of the referenced endpoint.
  * `__meta_kubernetes_endpointslice_endpoint_node_name`: Name of the Node hosting the referenced endpoint.
  * `__meta_kubernetes_endpointslice_endpoint_zone`: Zone the referenced endpoint exists in (only available when using the `discovery.k8s.io/v1` API group).
  * `__meta_kubernetes_endpointslice_port_name`: Named port of the referenced endpoint.
  * `__meta_kubernetes_endpointslice_port_protocol`: Protocol of the referenced endpoint.
  * `__meta_kubernetes_endpointslice_port`: Port of the referenced endpoint.

* If the endpoints belong to a service, all labels of the `service` role discovery are attached.
* For all targets backed by a Pod, all labels of the `pod` role discovery are attached.

### `ingress` role

The `ingress` role discovers a target for each path of each ingress.
This is generally useful for externally monitoring an ingress.
The address is set to the host specified in the Kubernetes `Ingress`'s `spec` block.

The following labels are included for discovered ingress objects:

* `__meta_kubernetes_ingress_annotation_<annotationname>`: Each annotation from the ingress object.
* `__meta_kubernetes_ingress_annotationpresent_<annotationname>`: `true` for each annotation from the ingress object.
* `__meta_kubernetes_ingress_class_name`: Class name from ingress spec, if present.
* `__meta_kubernetes_ingress_label_<labelname>`: Each label from the ingress object.
* `__meta_kubernetes_ingress_labelpresent_<labelname>`: `true` for each label from the ingress object.
* `__meta_kubernetes_ingress_name`: The name of the ingress object.
* `__meta_kubernetes_ingress_path`: Path from ingress spec. Defaults to /.
* `__meta_kubernetes_ingress_scheme`: Protocol scheme of ingress, `https` if TLS configuration is set. Defaults to `http`.
* `__meta_kubernetes_namespace`: The namespace of the ingress object.

## Blocks

You can use the following blocks with `discovery.kubernetes`:

| Block                                 | Description                                                | Required |
| ------------------------------------- | ---------------------------------------------------------- | -------- |
| [`attach_metadata`][attach_metadata]  | Optional metadata to attach to discovered targets.         | no       |
| [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| [`namespaces`][namespaces]            | Information about which Kubernetes namespaces to search.   | no       |
| [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| [`selectors`][selectors]              | Selectors to filter discovered Kubernetes resources.       | no       |
| [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[attach_metadata]: #attach_metadata
[authorization]: #authorization
[basic_auth]: #basic_auth
[namespaces]: #namespaces
[oauth2]: #oauth2
[selectors]: #selectors
[tls_config]: #tls_config

### `attach_metadata`

The `attach_metadata` block allows you to attach node metadata to discovered targets.

| Name        | Type   | Description                                                                                                                                                  | Default | Required |
| ----------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- | -------- |
| `node`      | `bool` | Attach node metadata. Valid for the `pod`, `endpoints`, and `endpointslice` roles. Requires permissions to list/watch Nodes.                                 |         | no       |
| `namespace` | `bool` | Attach namespace metadata. Valid for the `pod`, `endpoints`, `endpointslice`, `service`, and `ingress` roles. Requires permissions to list/watch Namespaces. |         | no       |

### `authorization`

The `authorization` block configures generic authorization to the endpoint.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

The `basic_auth` block configures basic authentication to the endpoint.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `namespaces`

The `namespaces` block limits the namespaces to discover resources in.
If you omit this block, all namespaces are searched.

| Name            | Type           | Description                                                       | Default | Required |
| --------------- | -------------- | ----------------------------------------------------------------- | ------- | -------- |
| `names`         | `list(string)` | List of namespaces to search.                                     |         | no       |
| `own_namespace` | `bool`         | Include the namespace {{< param "PRODUCT_NAME" >}} is running in. |         | no       |

### `oauth2`

The `oauth` block configures OAuth 2.0 authentication to the endpoint.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `selectors`

The `selectors` block contains optional label and field selectors to limit the discovery process to a subset of resources.

| Name    | Type     | Description            | Default | Required |
| ------- | -------- | ---------------------- | ------- | -------- |
| `role`  | `string` | Role of the selector.  |         | yes      |
| `field` | `string` | Field selector string. |         | no       |
| `label` | `string` | Label selector string. |         | no       |

See Kubernetes' documentation for [Field selectors][] and [Labels and selectors][] to learn more about the possible filters that can be used.

The endpoints role supports Pod, service, and endpoints selectors.
The Pod role supports node selectors when configured with `attach_metadata: {node: true}`.
Other roles only support selectors matching the role itself. For example, node role can only contain node selectors.

{{< admonition type="note" >}}
Using multiple `discovery.kubernetes` components with different selectors may increase load on the Kubernetes API.

Use selectors to retrieve a small set of resources in a very large cluster.
For smaller clusters, use a [`discovery.relabel` component](../discovery.relabel/) to filter targets instead.
{{< /admonition >}}

[Field selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
[Labels and selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/

### `tls_config`

The `tls_config` block configures TLS settings for connecting to the endpoint.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                            |
| --------- | ------------------- | ------------------------------------------------------ |
| `targets` | `list(map(string))` | The set of targets discovered from the Kubernetes API. |

## Component health

`discovery.kubernetes` is reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.kubernetes` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.kubernetes` doesn't expose any component-specific debug metrics.

## Examples

### In-cluster discovery

This example uses in-cluster authentication to discover all Pods:

```alloy
discovery.kubernetes "k8s_pods" {
  role = "pod"
}

prometheus.scrape "demo" {
  targets    = discovery.kubernetes.k8s_pods.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Kubeconfig file authentication

This example uses a `kubeconfig` file to authenticate to the Kubernetes API:

```alloy
discovery.kubernetes "k8s_pods" {
  role = "pod"
  kubeconfig_file = "/etc/k8s/kubeconfig.yaml"
}

prometheus.scrape "demo" {
  targets    = discovery.kubernetes.k8s_pods.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Limit searched namespaces and filter by label

This example limits the searched namespaces and selects only Pods with a specific label:

```alloy
discovery.kubernetes "k8s_pods" {
  role = "pod"

  selectors {
    role = "pod"
    label = "app.kubernetes.io/name=prometheus-node-exporter"
  }

  namespaces {
    names = ["myapp"]
  }
}

prometheus.scrape "demo" {
  targets    = discovery.kubernetes.k8s_pods.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Limit to only Pods on the same node

This example limits the search to Pods on the same node as this {{< param "PRODUCT_NAME" >}}.
This configuration is recommended when running {{< param "PRODUCT_NAME" >}} as a DaemonSet because it significantly reduces API server load and memory usage by only watching local Pods instead of all Pods cluster-wide.

{{< admonition type="note" >}}
This example assumes you used the Helm chart to deploy {{< param "PRODUCT_NAME" >}} in Kubernetes and that `HOSTNAME` is set to the Kubernetes host name.
If you have a custom Kubernetes Deployment, you must adapt this example to your configuration.

As an alternative, you can use [`discovery.kubelet`](../discovery.kubelet/) which queries the local `kubelet` API directly and only returns Pods running on the same node.
{{< /admonition >}}

```alloy
discovery.kubernetes "k8s_pods" {
  role = "pod"
  selectors {
    role = "pod"
    field = "spec.nodeName=" + coalesce(sys.env("HOSTNAME"), constants.hostname)
  }
}

prometheus.scrape "demo" {
  targets    = discovery.kubernetes.k8s_pods.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.kubernetes` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
