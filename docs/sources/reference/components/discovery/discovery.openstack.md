---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.openstack/
aliases:
  - ../discovery.openstack/ # /docs/alloy/latest/reference/components/discovery.openstack/
description: Learn about discovery.openstack
title: discovery.openstack
---

# discovery.openstack

`discovery.openstack` discovers [OpenStack][] Nova instances and exposes them as targets.

[OpenStack]: https://docs.openstack.org/nova/latest/

## Usage

```alloy
discovery.openstack "LABEL" {
  role   = "hypervisor"
  region = "us-east-1"
}
```

## Arguments

The following arguments are supported:

Name                            | Type       | Description                                                                                          | Default  | Required
--------------------------------|------------|------------------------------------------------------------------------------------------------------|----------|---------
`role`                          | `string`   | Role of the discovered targets.                                                                      |          | yes
`region`                        | `string`   | OpenStack region.                                                                                    |          | yes
`identity_endpoint`             | `string`   | Specifies the HTTP endpoint that is required to work with te Identity API of the appropriate version |          | no
`username`                      | `string`   | OpenStack username for the Identity V2 and V3 APIs.                                                  |          | no
`userid`                        | `string`   | OpenStack userid for the Identity V2 and V3 APIs.                                                    |          | no
`password`                      | `secret`   | Password for the Identity V2 and V3 APIs.                                                            |          | no
`domain_name`                   | `string`   | OpenStack domain name for the Identity V2 and V3 APIs.                                               |          | no
`domain_id`                     | `string`   | OpenStack domain ID for the Identity V2 and V3 APIs.                                                 |          | no
`project_name`                  | `string`   | OpenStack project name for the Identity V2 and V3 APIs.                                              |          | no
`project_id`                    | `string`   | OpenStack project ID for the Identity V2 and V3 APIs.                                                |          | no
`application_credential_name`   | `string`   | OpenStack application credential name for the Identity V2 and V3 APIs.                               |          | no
`application_credential_id`     | `string`   | OpenStack application credential ID for the Identity V2 and V3 APIs.                                 |          | no
`application_credential_secret` | `secret`   | OpenStack application credential secret for the Identity V2 and V3 APIs.                             |          | no
`all_tenants`                   | `bool`     | Whether the service discovery should list all instances for all projects.                            | `false`  | no
`refresh_interval`              | `duration` | Refresh interval to re-read the instance list.                                                       | `60s`    | no
`port`                          | `int`      | The port to scrape metrics from.                                                                     | `80`     | no
`availability`                  | `string`   | The availability of the endpoint to connect to.                                                      | `public` | no

`role` must be one of `hypervisor` or `instance`.

`username` is required if using Identity V2 API. In Identity V3, either `userid` or a combination of `username` and `domain_id` or `domain_name` are needed.

`project_id` and `project_name` fields are optional for the Identity V2 API.
Some providers allow you to specify a `project_name` instead of the `project_id`. Some require both.

`application_credential_id` or `application_credential_name` fields are required if using an application credential to authenticate.
Some providers allow you to create an application credential to authenticate rather than a password.

`application_credential_secret` field is required if using an application credential to authenticate.

`all_tenants` is only relevant for the `instance` role and usually requires admin permissions.

`availability` must be one of `public`, `admin`, or `internal`.

## Blocks
The following blocks are supported inside the definition of `discovery.openstack`:

Hierarchy  | Block          | Description                                          | Required
-----------|----------------|------------------------------------------------------|---------
tls_config | [tls_config][] | TLS configuration for requests to the OpenStack API. | no

[tls_config]: #tls_config-block

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|------------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the OpenStack API.

#### `hypervisor`

The `hypervisor` role discovers one target per Nova hypervisor node. The target
address defaults to the `host_ip` attribute of the hypervisor.

* `__meta_openstack_hypervisor_host_ip`: the hypervisor node's IP address.
* `__meta_openstack_hypervisor_hostname`: the hypervisor node's name.
* `__meta_openstack_hypervisor_id`: the hypervisor node's ID.
* `__meta_openstack_hypervisor_state`: the hypervisor node's state.
* `__meta_openstack_hypervisor_status`: the hypervisor node's status.
* `__meta_openstack_hypervisor_type`: the hypervisor node's type.

#### `instance`

The `instance` role discovers one target per network interface of Nova
instance. The target address defaults to the private IP address of the network
interface.

* `__meta_openstack_address_pool`: the pool of the private IP.
* `__meta_openstack_instance_flavor`: the flavor of the OpenStack instance.
* `__meta_openstack_instance_id`: the OpenStack instance ID.
* `__meta_openstack_instance_image`: the ID of the image the OpenStack instance is using.
* `__meta_openstack_instance_name`: the OpenStack instance name.
* `__meta_openstack_instance_status`: the status of the OpenStack instance.
* `__meta_openstack_private_ip`: the private IP of the OpenStack instance.
* `__meta_openstack_project_id`: the project (tenant) owning this instance.
* `__meta_openstack_public_ip`: the public IP of the OpenStack instance.
* `__meta_openstack_tag_<tagkey>`: each tag value of the instance.
* `__meta_openstack_user_id`: the user account owning the tenant.

## Component health

`discovery.openstack` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.openstack` does not expose any component-specific debug information.

## Debug metrics

`discovery.openstack` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.openstack "example" {
  role   = OPENSTACK_ROLE
  region = OPENSTACK_REGION
}

prometheus.scrape "demo" {
  targets    = discovery.openstack.example.targets
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
  - `OPENSTACK_ROLE`: Your OpenStack role.
  - `OPENSTACK_REGION`: Your OpenStack region.
  - `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
  - `USERNAME`: The username to use for authentication to the remote_write API.
  - `PASSWORD`: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.openstack` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
