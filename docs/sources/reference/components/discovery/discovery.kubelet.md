---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.kubelet/
aliases:
  - ../discovery.kubelet/ # /docs/alloy/latest/reference/components/discovery.kubelet/
description: Learn about discovery.kubelet
title: discovery.kubelet
---

# discovery.kubelet

`discovery.kubelet` discovers Kubernetes pods running on the specified Kubelet and exposes them as scrape targets.

## Usage

```alloy
discovery.kubelet "LABEL" {
}
```

## Requirements

* The Kubelet must be reachable from the `alloy` pod network.
* Follow the [Kubelet authorization][] documentation to configure authentication to the Kubelet API.

[Kubelet authorization]: https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/#kubelet-authorization

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default                   | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|---------------------------|---------
`url`                    | `string`            | URL of the Kubelet server.                                                                       | "https://localhost:10250" | no
`refresh_interval`       | `duration`          | How often the Kubelet should be polled for scrape targets                                        | `5s`                      | no
`namespaces`             | `list(string)`      | A list of namespaces to extract target pods from                                                 |                           | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |                           | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |                           | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`                    | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`                    | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |                           | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                           | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`                   | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |                           | no

The `namespaces` list limits the namespaces to discover resources in.
If omitted, all namespaces are searched.

`discovery.kubelet` appends a `/pods` path to `url` to request the available pods.
You can have additional paths in the `url`.
For example, if `url` is `https://kubernetes.default.svc.cluster.local:443/api/v1/nodes/cluster-node-1/proxy`, then `discovery.kubelet` sends a request on `https://kubernetes.default.svc.cluster.local:443/api/v1/nodes/cluster-node-1/proxy/pods`

 At most, one of the following can be provided:
 - [`bearer_token` argument](#arguments).
 - [`bearer_token_file` argument](#arguments).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

 [arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

The following blocks are supported inside the definition of
`discovery.kubelet`:

Hierarchy           | Block             | Description                                              | Required
--------------------|-------------------|----------------------------------------------------------|---------
basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no
authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no
oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no
oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no
tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no

The `>` symbol indicates deeper levels of nesting.
For example, `oauth2 > tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|----------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the Kubelet API.

Each target includes the following labels:

* `__address__`: The target address to scrape derived from the pod IP and container port.
* `__meta_kubernetes_namespace`: The namespace of the pod object.
* `__meta_kubernetes_pod_name`: The name of the pod object.
* `__meta_kubernetes_pod_ip`: The pod IP of the pod object.
* `__meta_kubernetes_pod_label_<labelname>`: Each label from the pod object.
* `__meta_kubernetes_pod_labelpresent_<labelname>`: `true` for each label from the pod object.
* `__meta_kubernetes_pod_annotation_<annotationname>`: Each annotation from the pod object.
* `__meta_kubernetes_pod_annotationpresent_<annotationname>`: `true` for each annotation from the pod object.
* `__meta_kubernetes_pod_container_init`: `true` if the container is an `InitContainer`.
* `__meta_kubernetes_pod_container_name`: Name of the container the target address points to.
* `__meta_kubernetes_pod_container_id`: ID of the container the target address points to. The ID is in the form `<type>://<container_id>`.
* `__meta_kubernetes_pod_container_image`: The image the container is using.
* `__meta_kubernetes_pod_container_port_name`: Name of the container port.
* `__meta_kubernetes_pod_container_port_number`: Number of the container port.
* `__meta_kubernetes_pod_container_port_protocol`: Protocol of the container port.
* `__meta_kubernetes_pod_ready`: Set to `true` or `false` for the pod's ready state.
* `__meta_kubernetes_pod_phase`: Set to `Pending`, `Running`, `Succeeded`, `Failed` or `Unknown` in the lifecycle.
* `__meta_kubernetes_pod_node_name`: The name of the node the pod is scheduled onto.
* `__meta_kubernetes_pod_host_ip`: The current host IP of the pod object.
* `__meta_kubernetes_pod_uid`: The UID of the pod object.
* `__meta_kubernetes_pod_controller_kind`: Object kind of the pod controller.
* `__meta_kubernetes_pod_controller_name`: Name of the pod controller.

{{< admonition type="note" >}}
The Kubelet API used by this component is an internal API and therefore the data in the response returned from the API can't be guaranteed between different versions of the Kubelet.
{{< /admonition >}}

## Component health

`discovery.kubelet` is reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.kubelet` does not expose any component-specific debug information.

## Debug metrics

`discovery.kubelet` does not expose any component-specific debug metrics.

## Examples

### Bearer token file authentication

This example uses a bearer token file to authenticate to the Kubelet API:

```alloy
discovery.kubelet "k8s_pods" {
  bearer_token_file = "/var/run/secrets/kubernetes.io/serviceaccount/token"
}

prometheus.scrape "demo" {
  targets    = discovery.kubelet.k8s_pods.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```
Replace the following:
  - `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
  - `USERNAME`: The username to use for authentication to the remote_write API.
  - `PASSWORD`: The password to use for authentication to the remote_write API.

### Limit searched namespaces

This example limits the namespaces where pods are discovered using the `namespaces` argument:

```alloy
discovery.kubelet "k8s_pods" {
  bearer_token_file = "/var/run/secrets/kubernetes.io/serviceaccount/token"
  namespaces = ["default", "kube-system"]
}

prometheus.scrape "demo" {
  targets    = discovery.kubelet.k8s_pods.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```
Replace the following:
  - `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
  - `USERNAME`: The username to use for authentication to the remote_write API.
  - `PASSWORD`: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.kubelet` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
