---
aliases:
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/remote.kubernetes.configmap/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/remote.kubernetes.configmap/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/remote.kubernetes.configmap/
description: Learn about remote.kubernetes.configmap
title: remote.kubernetes.configmap
---

# remote.kubernetes.configmap

`remote.kubernetes.configmap` reads a ConfigMap from the Kubernetes API server and exposes its data for other components to consume.

This can be useful anytime {{< param "PRODUCT_NAME" >}} needs data from a ConfigMap that is not directly mounted to the {{< param "PRODUCT_ROOT_NAME" >}} pod.

## Usage

```river
remote.kubernetes.configmap "LABEL" {
  namespace = "NAMESPACE_OF_CONFIGMAP"
  name = "NAME_OF_CONFIGMAP"
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`namespace` | `string` | Kubernetes namespace containing the desired ConfigMap. | | yes
`name` | `string` | Name of the Kubernetes ConfigMap | | yes
`poll_frequency` | `duration` | Frequency to poll the Kubernetes API. | `"1m"` | no
`poll_timeout` | `duration` | Timeout when polling the Kubernetes API. | `"15s"` | no

When this component performs a poll operation, it requests the ConfigMap data from the Kubernetes API.
A poll is triggered by the following:

* When the component first loads.
* Every time the component's arguments get re-evaluated.
* At the frequency specified by the `poll_frequency` argument.

Any error while polling will mark the component as unhealthy. After
a successful poll, all data is exported with the same field names as the source ConfigMap.

## Blocks

The following blocks are supported inside the definition of `remote.kubernetes.configmap`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
client | [client][] | Configures Kubernetes client used to find Probes. | no
client > basic_auth | [basic_auth][] | Configure basic authentication to the Kubernetes API. | no
client > authorization | [authorization][] | Configure generic authorization to the Kubernetes API. | no
client > oauth2 | [oauth2][] | Configure OAuth2 for authenticating to the Kubernetes API. | no
client > oauth2 > tls_config | [tls_config][] | Configure TLS settings for connecting to the Kubernetes API. | no
client > tls_config | [tls_config][] | Configure TLS settings for connecting to the Kubernetes API. | no

The `>` symbol indicates deeper levels of nesting. For example, `client > basic_auth`
refers to a `basic_auth` block defined inside a `client` block.

[client]: #client-block
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block

### client block

The `client` block configures the Kubernetes client used to discover Probes. If the `client` block isn't provided, the default in-cluster
configuration with the service account of the running {{< param "PRODUCT_ROOT_NAME" >}} pod is
used.

The following arguments are supported:

Name                     | Type                | Description                                                   | Default | Required
-------------------------|---------------------|---------------------------------------------------------------|---------|---------
`api_server`             | `string`            | URL of the Kubernetes API server.                             |         | no
`kubeconfig_file`        | `string`            | Path of the `kubeconfig` file to use for connecting to Kubernetes. |    | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.          |         | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                            |         | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                      | `true`  | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.  | `true`  | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                          |         | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. | | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.         | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests. |         | no

 At most, one of the following can be provided:
 - [`bearer_token` argument][client].
 - [`bearer_token_file` argument][client].
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

{{< docs/shared lookup="flow/reference/components/http-client-proxy-config-description.md" source="agent" version="<AGENT_VERSION>" >}}

### basic_auth block

{{< docs/shared lookup="flow/reference/components/basic-auth-block.md" source="agent" version="<AGENT_VERSION>" >}}

### authorization block

{{< docs/shared lookup="flow/reference/components/authorization-block.md" source="agent" version="<AGENT_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="flow/reference/components/oauth2-block.md" source="agent" version="<AGENT_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="flow/reference/components/tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}


## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`data` | `map(string)` | Data from the ConfigMap obtained from Kubernetes.

The `data` field contains a mapping from field names to values.

## Component health

Instances of `remote.kubernetes.configmap` report as healthy if the most recent attempt to poll the kubernetes API succeeds.

## Debug information

`remote.kubernetes.configmap` does not expose any component-specific debug information.

## Debug metrics

`remote.kubernetes.configmap` does not expose any component-specific debug metrics.

## Example

This example reads a Secret and a ConfigMap from Kubernetes and uses them to supply remote-write credentials.

```river
remote.kubernetes.secret "credentials" {
  namespace = "monitoring"
  name = "metrics-secret"
}

remote.kubernetes.configmap "endpoint" {
  namespace = "monitoring"
  name = "metrics-endpoint"
}

prometheus.remote_write "default" {
  endpoint {
    url = remote.kubernetes.configmap.endpoint.data["url"]
    basic_auth {
      username = remote.kubernetes.configmap.endpoint.data["username"]
      password = remote.kubernetes.secret.credentials.data["password"]
    }
  }
}
```

This example assumes that the Secret and ConfigMap have already been created, and that the appropriate field names
exist in their data.

