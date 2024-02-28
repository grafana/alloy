---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/remote.s3/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/remote.s3/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/remote.s3/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/remote.s3/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/remote.s3/
description: Learn about remote.s3
title: remote.s3
---

# remote.s3

`remote.s3` exposes the string contents of a file located in [AWS S3](https://aws.amazon.com/s3/)
to other components. The file will be polled for changes so that the most
recent content is always available.

The most common use of `remote.s3` is to load secrets from files.

Multiple `remote.s3` components can be specified using different name
labels. By default, [AWS environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html) are used to authenticate against S3. The `key` and `secret` arguments inside `client` blocks can be used to provide custom authentication.

> **NOTE**: Other S3-compatible systems can be read  with `remote.s3` but may require specific
> authentication environment variables. There is no  guarantee that `remote.s3` will work with non-AWS S3
> systems.

## Usage

```river
remote.s3 "LABEL" {
  path = S3_FILE_PATH
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`path` | `string` | Path in the format of `"s3://bucket/file"`. | | yes
`poll_frequency` | `duration` | How often to poll the file for changes. Must be greater than 30 seconds. | `"10m"` | no
`is_secret` | `bool` | Marks the file as containing a [secret][]. | `false` | no

> **NOTE**: `path` must include a full path to a file. This does not support reading of directories.

[secret]: {{< relref "../../concepts/config-language/expressions/types_and_values.md#secrets" >}}

## Blocks

Hierarchy | Name       | Description | Required
--------- |------------| ----------- | --------
client | [client][] | Additional options for configuring the S3 client. | no

[client]: #client-block

### client block

The `client` block customizes options to connect to the S3 server.

Name | Type | Description                                                                             | Default | Required
---- | ---- |-----------------------------------------------------------------------------------------| ------- | --------
`key` | `string` | Used to override default access key.                                                    | | no
`secret` | `secret` | Used to override default secret value.                                                  | | no
`endpoint` | `string` | Specifies a custom url to access, used generally for S3-compatible systems.             | | no
`disable_ssl` | `bool` | Used to disable SSL, generally used for testing.                                        | | no
`use_path_style` | `string` | Path style is a deprecated setting that is generally enabled for S3 compatible systems. | `false` | no
`region` | `string` | Used to override default region.                                                        | | no
`signing_region` | `string` | Used to override the signing region when using a custom endpoint.                       | | no


## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`content` | `string` or `secret` | The contents of the file. | | no

The `content` field will be secret if `is_secret` was set to true.

## Component health

Instances of `remote.s3` report as healthy if the most recent read of
the watched file was successful.

## Debug information

`remote.s3` does not expose any component-specific debug information.

## Debug metrics

`remote.s3` does not expose any component-specific debug metrics.

## Example

```river
remote.s3 "data" {
  path = "s3://test-bucket/file.txt"
}
```
