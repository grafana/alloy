---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.ec2/
aliases:
  - ../discovery.ec2/ # /docs/alloy/latest/reference/components/discovery.ec2/
description: Learn about discovery.ec2
title: discovery.ec2
---

# discovery.ec2

`discovery.ec2` lets you retrieve scrape targets from EC2 instances. The private IP address is used by default, but you can change it to the public IP address using relabeling.

The IAM credentials used must have the `ec2:DescribeInstances` permission to discover scrape targets, and may optionally have the `ec2:DescribeAvailabilityZones` permission to make the availability zone ID available as a label.

## Usage

```alloy
discovery.ec2 "LABEL" {
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                                             | Default | Required
-------------------------|---------------------|-------------------------------------------------------------------------------------------------------------------------|---------|---------
`endpoint`               | `string`            | Custom endpoint to be used.                                                                                             |         | no
`region`                 | `string`            | The AWS region. If blank, the region from the instance metadata is used.                                                |         | no
`access_key`             | `string`            | The AWS API key ID. If blank, the environment variable `AWS_ACCESS_KEY_ID` is used.                                     |         | no
`secret_key`             | `string`            | The AWS API key secret. If blank, the environment variable `AWS_SECRET_ACCESS_KEY` is used.                             |         | no
`profile`                | `string`            | Named AWS profile used to connect to the API.                                                                           |         | no
`role_arn`               | `string`            | AWS Role Amazon Resource Name (ARN), an alternative to using AWS API keys.                                              |         | no
`refresh_interval`       | `string`            | Refresh interval to re-read the instance list.                                                                          | 60s     | no
`port`                   | `int`               | The port to scrape metrics from. If using the public IP address, this must instead be specified in the relabeling rule. | 80      | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                                                    |         | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                                                                                      |         | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                                                | `true`  | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                                            | `true`  | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                                                    |         | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying.                        |         | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                                                   | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                                           |         | no

 At most, one of the following can be provided:
 - [`bearer_token` argument](#arguments).
 - [`bearer_token_file` argument](#arguments).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

 {{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

The following blocks are supported inside the definition of
`discovery.ec2`:

Hierarchy           | Block             | Description                                              | Required
--------------------|-------------------|----------------------------------------------------------|---------
basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no
authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no
filter              | [filter][]        | Filters discoverable resources.                          | no
oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no
oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no
tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no

[filter]: #filter-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block
[basic_auth]: #basic_auth-block

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### filter block

Filters can be used optionally to filter the instance list by other criteria.
Available filter criteria can be found in the [Amazon EC2 documentation](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html).

Name     | Type           | Description                   | Default | Required
---------|----------------|-------------------------------|---------|---------
`name`   | `string`       | Filter name to use.           |         | yes
`values` | `list(string)` | Values to pass to the filter. |         | yes

Refer to the [Filter API AWS EC2 documentation][filter api] for the list of supported filters and their descriptions.

[filter api]: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html

### oauth2 block

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`targets` | `list(map(string))` | The set of discovered EC2 targets.

Each target includes the following labels:

* `__meta_ec2_ami`: The EC2 Amazon Machine Image.
* `__meta_ec2_architecture`: The architecture of the instance.
* `__meta_ec2_availability_zone`: The availability zone in which the instance is running.
* `__meta_ec2_availability_zone_id`: The availability zone ID in which the instance is running (requires `ec2:DescribeAvailabilityZones`).
* `__meta_ec2_instance_id`: The EC2 instance ID.
* `__meta_ec2_instance_lifecycle`: The lifecycle of the EC2 instance, set only for 'spot' or 'scheduled' instances, absent otherwise.
* `__meta_ec2_instance_state`: The state of the EC2 instance.
* `__meta_ec2_instance_type`: The type of the EC2 instance.
* `__meta_ec2_ipv6_addresses`: Comma-separated list of IPv6 addresses assigned to the instance's network interfaces, if present.
* `__meta_ec2_owner_id`: The ID of the AWS account that owns the EC2 instance.
* `__meta_ec2_platform`: The Operating System platform, set to 'windows' on Windows servers, absent otherwise.
* `__meta_ec2_primary_subnet_id`: The subnet ID of the primary network interface, if available.
* `__meta_ec2_private_dns_name`: The private DNS name of the instance, if available.
* `__meta_ec2_private_ip`: The private IP address of the instance, if present.
* `__meta_ec2_public_dns_name`: The public DNS name of the instance, if available.
* `__meta_ec2_public_ip`: The public IP address of the instance, if available.
* `__meta_ec2_region`: The region of the instance.
* `__meta_ec2_subnet_id`: Comma-separated list of subnets IDs in which the instance is running, if available.
* `__meta_ec2_tag_<tagkey>`: Each tag value of the instance.
* `__meta_ec2_vpc_id`: The ID of the VPC in which the instance is running, if available.

## Component health

`discovery.ec2` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.ec2` does not expose any component-specific debug information.

## Debug metrics

`discovery.ec2` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.ec2 "ec2" {
  region = "us-east-1"
}

prometheus.scrape "demo" {
  targets    = discovery.ec2.ec2.targets
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

`discovery.ec2` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
