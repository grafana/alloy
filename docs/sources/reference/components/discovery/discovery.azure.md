---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.azure/
aliases:
  - ../discovery.azure/ # /docs/alloy/latest/reference/components/discovery.azure/
description: Learn about discovery.azure
title: discovery.azure
---

# discovery.azure

`discovery.azure` discovers [Azure][] Virtual Machines and exposes them as targets.

[Azure]: https://azure.microsoft.com/en-us

## Usage

```alloy
discovery.azure "<LABEL>" {
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default              | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|----------------------|---------
`environment`            | `string`            | Azure environment.                                                                               | `"AzurePublicCloud"` | no
`port`                   | `number`            | Port to be appended to the `__address__` label for each target.                                  | `80`                 | no
`subscription_id`        | `string`            | Azure subscription ID.                                                                           |                      | no
`refresh_interval`       | `duration`          | Interval at which to refresh the list of targets.                                                | `5m`                 | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |                      | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                      | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`              | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |                      | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`               | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`               | no

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks
The following blocks are supported inside the definition of
`discovery.azure`:

Hierarchy        | Block                | Description                                      | Required
-----------------|----------------------|--------------------------------------------------|---------
oauth            | [oauth][]            | OAuth configuration for Azure API.               | no
managed_identity | [managed_identity][] | Managed Identity configuration for Azure API.    | no
tls_config       | [tls_config][]       | TLS configuration for requests to the Azure API. | no

Exactly one of the `oauth` or `managed_identity` blocks must be specified.

[oauth]: #oauth-block
[managed_identity]: #managed_identity-block
[tls_config]: #tls_config-block

### oauth block
The `oauth` block configures OAuth authentication for the Azure API.

Name            | Type     | Description          | Default | Required
----------------|----------|----------------------|---------|---------
`client_id`     | `string` | OAuth client ID.     |         | yes
`client_secret` | `string` | OAuth client secret. |         | yes
`tenant_id`     | `string` | OAuth tenant ID.     |         | yes

### managed_identity block
The `managed_identity` block configures Managed Identity authentication for the Azure API.

Name        | Type     | Description                 | Default | Required
------------|----------|-----------------------------|---------|---------
`client_id` | `string` | Managed Identity client ID. |         | yes

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|--------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the Azure API.

Each target includes the following labels:

* `__meta_azure_subscription_id`: The Azure subscription ID.
* `__meta_azure_tenant_id`: The Azure tenant ID.
* `__meta_azure_machine_id`: The UUID of the Azure VM.
* `__meta_azure_machine_resource_group`: The name of the resource group the VM is in.
* `__meta_azure_machine_name`: The name of the VM.
* `__meta_azure_machine_computer_name`: The host OS name of the VM.
* `__meta_azure_machine_os_type`: The OS the VM is running (either `Linux` or `Windows`).
* `__meta_azure_machine_location`: The region the VM is in.
* `__meta_azure_machine_private_ip`: The private IP address of the VM.
* `__meta_azure_machine_public_ip`: The public IP address of the VM.
* `__meta_azure_machine_tag_*`: A tag on the VM. There will be one label per tag.
* `__meta_azure_machine_scale_set`: The name of the scale set the VM is in.
* `__meta_azure_machine_size`: The size of the VM.

Each discovered VM maps to a single target. The `__address__` label is set to the `private_ip:port` (`[private_ip]:port` if the private IP is an IPv6 address) of the VM.

## Component health

`discovery.azure` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.azure` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.azure` doesn't expose any component-specific debug metrics.

## Example

```alloy
discovery.azure "example" {
  port = 80
  subscription_id = <AZURE_SUBSCRIPTION_ID>
  oauth {
      client_id = <AZURE_CLIENT_ID>
      client_secret = <AZURE_CLIENT_SECRET>
      tenant_id = <AZURE_TENANT_ID>
  }
}

prometheus.scrape "demo" {
  targets    = discovery.azure.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = <PROMETHEUS_REMOTE_WRITE_URL>

    basic_auth {
      username = <USERNAME>
      password = <PASSWORD>
    }
  }
}
```

Replace the following:
  - _`<AZURE_SUBSCRIPTION_ID>`_: Your Azure subscription ID.
  - _`<AZURE_CLIENT_ID>`_: Your Azure client ID.
  - _`<AZURE_CLIENT_SECRET>`_: Your Azure client secret.
  - _`<AZURE_TENANT_ID>`_: Your Azure tenant ID.
  - _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
  - _`<USERNAME>`_: The username to use for authentication to the remote_write API.
  - _`<PASSWORD>`_: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.azure` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
