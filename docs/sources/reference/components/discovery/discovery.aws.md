---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.aws/
aliases:
  - ../discovery.aws/ # /docs/alloy/latest/reference/components/discovery.aws/
description: Learn about discovery.aws
labels:
  stage: general-availability
  products:
    - oss
title: discovery.aws
---

# `discovery.aws`

`discovery.aws` lets you retrieve scrape targets from AWS services using a single, role-based configuration.
Set `role` to choose which AWS service to discover: `ec2`, `ecs`, `elasticache`, `lightsail`, `msk`, or `rds`.

This unifies AWS service discovery under one component and is the way to discover Amazon ECS targets.
It doesn't replace [`discovery.ec2`][discovery.ec2] or [`discovery.lightsail`][discovery.lightsail], which remain available.

The IAM credentials used must grant the permissions the selected `role` requires to list resources, for example `ec2:DescribeInstances` for the `ec2` role or `ecs:ListClusters`, `ecs:ListServices`, and `ecs:ListTasks` for the `ecs` role.

[discovery.ec2]: ../discovery.ec2/
[discovery.lightsail]: ../discovery.lightsail/

## Usage

```alloy
discovery.aws "<LABEL>" {
  role = "<ROLE>"
}
```

## Arguments

You can use the following arguments with `discovery.aws`:

| Name                     | Type                | Description                                                                                                              | Default | Required |
| ------------------------ | ------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `role`                   | `string`            | The AWS service to discover. One of `ec2`, `ecs`, `elasticache`, `lightsail`, `msk`, `rds`.                             |         | yes      |
| `access_key`             | `string`            | The AWS API key ID. If blank, the environment variable `AWS_ACCESS_KEY_ID` is used.                                     |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                                                    |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                                                      |         | no       |
| `clusters`               | `list(string)`      | Restrict discovery to these clusters. Only used with the `ecs`, `elasticache`, `msk`, and `rds` roles.                  |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                                                | `true`  | no       |
| `endpoint`               | `string`            | Custom endpoint to be used.                                                                                             |         | no       |
| `external_id`            | `string`            | AWS External ID used when assuming `role_arn`.                                                                          |         | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                                            | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.                                 |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying.                        |         | no       |
| `port`                   | `int`               | The port to scrape metrics from. If using the public IP address, this must instead be specified in the relabeling rule. | `80`    | no       |
| `profile`                | `string`            | Named AWS profile used to connect to the API.                                                                           |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                                           |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                                                   | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                                                    |         | no       |
| `refresh_interval`       | `duration`          | Refresh interval to re-read the resource list.                                                                          | `"60s"` | no       |
| `region`                 | `string`            | The AWS region. If blank, the region from the instance metadata is used.                                                |         | no       |
| `request_concurrency`    | `int`               | Maximum number of concurrent AWS API requests. Only used with the `ecs`, `elasticache`, `msk`, and `rds` roles.        |         | no       |
| `role_arn`               | `string`            | AWS Role Amazon Resource Name (ARN), an alternative to using AWS API keys.                                              |         | no       |
| `secret_key`             | `string`            | The AWS API key secret. If blank, the environment variable `AWS_SECRET_ACCESS_KEY` is used.                             |         | no       |

The `clusters` and `request_concurrency` arguments only apply to the `ecs`, `elasticache`, `msk`, and `rds` roles.
When `request_concurrency` isn't set, it defaults to `20` for the `ecs` role and `10` for the `elasticache`, `msk`, and `rds` roles.
The [`filter`][filter] block only applies to the `ec2` role.

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`](#arguments) argument
* [`bearer_token`](#arguments) argument
* [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.aws`:

{{< docs/alloy-config >}}

| Block                                 | Description                                                | Required |
| ------------------------------------- | ---------------------------------------------------------- | -------- |
| [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| [`filter`][filter]                    | Filters discoverable resources. Only used with the `ec2` role. | no   |
| [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

[authorization]: #authorization
[basic_auth]: #basic_auth
[filter]: #filter
[oauth2]: #oauth2
[tls_config]: #tls_config

{{< /docs/alloy-config >}}

### `authorization`

The `authorization` block configures generic authorization to the endpoint.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

The `basic_auth` block configures basic authentication to the endpoint.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `filter`

The `filter` block filters the EC2 instance list by other criteria. It only applies to the `ec2` role.
Refer to the [Amazon EC2 documentation][amazon] for more information about filters.

| Name     | Type           | Description                   | Default | Required |
| -------- | -------------- | ----------------------------- | ------- | -------- |
| `name`   | `string`       | Filter name to use.           |         | yes      |
| `values` | `list(string)` | Values to pass to the filter. |         | yes      |

Refer to the [Filter API AWS EC2 documentation][filter api] for the list of supported filters and their descriptions.

[amazon]: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
[filter api]: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html

### `oauth2`

The `oauth2` block configures OAuth 2.0 authentication to the endpoint.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

The `tls_config` block configures TLS settings for connecting to the endpoint.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                        |
| --------- | ------------------- | ---------------------------------- |
| `targets` | `list(map(string))` | The set of discovered AWS targets. |

The labels attached to each target depend on the `role`.

### `ec2`

* `__meta_ec2_ami`: The EC2 Amazon Machine Image.
* `__meta_ec2_architecture`: The architecture of the instance.
* `__meta_ec2_availability_zone_id`: The availability zone ID in which the instance is running. Requires `ec2:DescribeAvailabilityZones`.
* `__meta_ec2_availability_zone`: The availability zone in which the instance is running.
* `__meta_ec2_instance_id`: The EC2 instance ID.
* `__meta_ec2_instance_lifecycle`: The lifecycle of the EC2 instance, set only for 'spot' or 'scheduled' instances, absent otherwise.
* `__meta_ec2_instance_state`: The state of the EC2 instance.
* `__meta_ec2_instance_type`: The type of the EC2 instance.
* `__meta_ec2_ipv6_addresses`: Comma-separated list of IPv6 addresses assigned to the instance's network interfaces, if present.
* `__meta_ec2_owner_id`: The ID of the AWS account that owns the EC2 instance.
* `__meta_ec2_platform`: The Operating System platform, set to 'windows' on Windows servers, absent otherwise.
* `__meta_ec2_primary_ipv6_addresses`: Comma separated list of the Primary IPv6 addresses of the instance, if present. The list is ordered based on the position of each corresponding network interface in the attachment order.
* `__meta_ec2_primary_subnet_id`: The subnet ID of the primary network interface, if available.
* `__meta_ec2_private_dns_name`: The private DNS name of the instance, if available.
* `__meta_ec2_private_ip`: The private IP address of the instance, if present.
* `__meta_ec2_public_dns_name`: The public DNS name of the instance, if available.
* `__meta_ec2_public_ip`: The public IP address of the instance, if available.
* `__meta_ec2_region`: The region of the instance.
* `__meta_ec2_subnet_id`: Comma-separated list of subnets IDs in which the instance is running, if available.
* `__meta_ec2_tag_<tagkey>`: Each tag value of the instance.
* `__meta_ec2_vpc_id`: The ID of the VPC in which the instance is running, if available.

### `lightsail`

* `__meta_lightsail_availability_zone`: The availability zone in which the instance is running.
* `__meta_lightsail_blueprint_id`: The Lightsail blueprint ID.
* `__meta_lightsail_bundle_id`: The Lightsail bundle ID.
* `__meta_lightsail_instance_name`: The name of the Lightsail instance.
* `__meta_lightsail_instance_state`: The state of the Lightsail instance.
* `__meta_lightsail_instance_support_code`: The support code of the Lightsail instance.
* `__meta_lightsail_private_ip`: The private IP address of the instance.
* `__meta_lightsail_public_ip`: The public IP address of the instance, if available.
* `__meta_lightsail_region`: The region of the instance.
* `__meta_lightsail_tag_<tagkey>`: Each tag value of the instance.

### `ecs`

* `__meta_ecs_availability_zone`: The availability zone of the task.
* `__meta_ecs_cluster`: The name of the ECS cluster.
* `__meta_ecs_cluster_arn`: The ARN of the ECS cluster.
* `__meta_ecs_container_instance_arn`: The ARN of the container instance the task runs on, if available.
* `__meta_ecs_desired_status`: The desired status of the task.
* `__meta_ecs_ec2_instance_id`: The ID of the backing EC2 instance, if available.
* `__meta_ecs_ec2_instance_private_ip`: The private IP of the backing EC2 instance, if available.
* `__meta_ecs_ec2_instance_public_ip`: The public IP of the backing EC2 instance, if available.
* `__meta_ecs_ec2_instance_type`: The type of the backing EC2 instance, if available.
* `__meta_ecs_health_status`: The health status of the task.
* `__meta_ecs_ip_address`: The IP address of the task.
* `__meta_ecs_last_status`: The last known status of the task.
* `__meta_ecs_launch_type`: The launch type of the task, for example `EC2` or `FARGATE`.
* `__meta_ecs_network_mode`: The network mode of the task.
* `__meta_ecs_platform_family`: The platform family of the task.
* `__meta_ecs_platform_version`: The platform version of the task.
* `__meta_ecs_public_ip`: The public IP of the task, if available.
* `__meta_ecs_region`: The region of the task.
* `__meta_ecs_service`: The name of the ECS service.
* `__meta_ecs_service_arn`: The ARN of the ECS service.
* `__meta_ecs_service_status`: The status of the ECS service.
* `__meta_ecs_subnet_id`: The subnet ID of the task, if available.
* `__meta_ecs_tag_<tagkey>`: Each tag value on the task, service, cluster, or backing EC2 instance, prefixed by the resource it belongs to.
* `__meta_ecs_task_arn`: The ARN of the task.
* `__meta_ecs_task_definition`: The task definition of the task.
* `__meta_ecs_task_group`: The task group of the task.

### `elasticache`, `msk`, and `rds`

These roles attach labels under the `__meta_elasticache_`, `__meta_msk_`, and `__meta_rds_` prefixes respectively, describing the discovered cluster, node, instance, and tag attributes.
Refer to the [Prometheus `aws_sd_config` documentation][prometheus aws_sd] for the current set of labels for these roles.

[prometheus aws_sd]: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#aws_sd_config

## Component health

`discovery.aws` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.aws` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.aws` doesn't expose any component-specific debug metrics.

## Example

```alloy
discovery.aws "ecs" {
  role   = "ecs"
  region = "us-east-1"
}

prometheus.scrape "demo" {
  targets    = discovery.aws.ecs.targets
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

`discovery.aws` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
