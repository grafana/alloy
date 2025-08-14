---
canonical: https://grafana.com/docs/alloy/latest/reference/components/remote/remote.s3/
aliases:
  - ../remote.s3/ # /docs/alloy/latest/reference/components/remote.s3/
description: Learn about remote.s3
labels:
  stage: general-availability
  products:
    - oss
title: remote.s3
---

# `remote.s3`

`remote.s3` exposes the string contents of a file located in [AWS S3](https://aws.amazon.com/s3/) to other components.
The file is polled for changes so that the most recent content is always available.

The most common use of `remote.s3` is to load secrets from files.

You can specify multiple `remote.s3` components by using different name labels.
By default, [AWS environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html) are used to authenticate against S3.
The `key` and `secret` arguments inside `client` blocks can be used to provide custom authentication.

{{< admonition type="note" >}}
Other S3-compatible systems can be read  with `remote.s3` but may require specific authentication environment variables.
There is no  guarantee that `remote.s3` will work with non-AWS S3 systems.
{{< /admonition >}}

## Usage

```alloy
remote.s3 "<LABEL>" {
  path = "<S3_FILE_PATH>"
}
```

## Arguments

You can use the following arguments with `remote.s3`:

| Name             | Type       | Description                                                              | Default | Required |
| ---------------- | ---------- | ------------------------------------------------------------------------ | ------- | -------- |
| `path`           | `string`   | Path in the format of `"s3://bucket/file"`.                              |         | yes      |
| `is_secret`      | `bool`     | Marks the file as containing a [secret][].                               | `false` | no       |
| `poll_frequency` | `duration` | How often to poll the file for changes. Must be greater than 30 seconds. | `"10m"` | no       |

{{< admonition type="note" >}}
`path` must include a full path to a file.
This doesn't support reading of directories.
{{< /admonition >}}

[secret]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#secrets

## Blocks

You can use the following block with `remote.s3`:

| Name               | Description                                       | Required |
| ------------------ | ------------------------------------------------- | -------- |
| [`client`][client] | Additional options for configuring the S3 client. | no       |

[client]: #client

### `client`

The `client` block customizes options to connect to the S3 server.

| Name             | Type     | Description                                                                            | Default | Required |
| ---------------- | -------- | -------------------------------------------------------------------------------------- | ------- | -------- |
| `key`            | `string` | Used to override default access key.                                                   |         | no       |
| `secret`         | `secret` | Used to override default secret value.                                                 |         | no       |
| `endpoint`       | `string` | Specifies a custom URL to access, used generally for S3-compatible systems.            |         | no       |
| `disable_ssl`    | `bool`   | Used to disable SSL, generally used for testing.                                       | `false` | no       |
| `use_path_style` | `bool`   | Path style is a deprecated setting that's generally enabled for S3 compatible systems. | `false` | no       |
| `region`         | `string` | Used to override default region.                                                       |         | no       |
| `signing_region` | `string` | Used to override the signing region when using a custom endpoint.                      |         | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                 | Description               | Default | Required |
| --------- | -------------------- | ------------------------- | ------- | -------- |
| `content` | `string` or `secret` | The contents of the file. |         | no       |

The `content` field will be secret if `is_secret` is set to true.

## Component health

Instances of `remote.s3` report as healthy if the most recent read of the watched file was successful.

## Debug information

`remote.s3` doesn't expose any component-specific debug information.

## Debug metrics

`remote.s3` doesn't expose any component-specific debug metrics.

## Example

```alloy
remote.s3 "data" {
  path = "s3://test-bucket/file.txt"
}
```
