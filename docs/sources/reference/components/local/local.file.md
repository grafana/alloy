---
canonical: https://grafana.com/docs/alloy/latest/reference/components/local/local.file/
aliases:
  - ../local.file/ # /docs/alloy/latest/reference/components/local.file/
description: Learn about local.file
labels:
  stage: general-availability
  products:
    - oss
title: local.file
---

# `local.file`

`local.file` exposes the contents of a file on disk to other components.
The file is watched for changes so that its latest content is always exposed.

The most common use of `local.file` is to load secrets (for example, API keys) from files.

You can specify multiple `local.file` components by giving them different labels.

## Usage

```alloy
local.file "<LABEL>" {
  filename = "<FILE_NAME>"
}
```

## Arguments

You can use the following arguments with `local.file`:

| Name             | Type       | Description                                              | Default      | Required |
|------------------|------------|----------------------------------------------------------|--------------|----------|
| `filename`       | `string`   | Path of the file on disk to watch.                       |              | yes      |
| `detector`       | `string`   | Which file change detector to use, `fsnotify` or `poll`. | `"fsnotify"` | no       |
| `is_secret`      | `bool`     | Marks the file as containing a [secret][].               | `false`      | no       |
| `poll_frequency` | `duration` | How often to poll for file changes.                      | `"1m"`       | no       |

[secret]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#secrets

{{< docs/shared lookup="reference/components/local-file-arguments-text.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

The `local.file` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                 | Description                                         |
|-----------|----------------------|-----------------------------------------------------|
| `content` | `string` or `secret` | The contents of the file from the most recent read. |

The `content` field has the `secret` type only if the `is_secret` argument is true.

You can use `local.file.LABEL.content` to access the contents of the file.

## Component health

`local.file` is reported as healthy whenever if the watched file was read successfully.

Failing to read the file whenever an update is detected (or after the poll period elapses) causes the component to be reported as unhealthy.
When unhealthy, exported fields is kept at the last healthy value.
The read error is exposed as a log message and in the debug information for the component.

## Debug information

`local.file` doesn't expose any component-specific debug information.

## Debug metrics

* `local_file_timestamp_last_accessed_unix_seconds` (gauge): The timestamp, in Unix seconds, that the file was last successfully accessed.

## Example

The following example shows a simple `local.file` configuration that watches a passwords text file and uses the exported content field.

```alloy
local.file "secret_key" {
  filename  = "/var/secrets/password.txt"
  is_secret = true
}
grafana_cloud.stack "receivers" {
  stack_name = "mystack"
  token = local.file.secret_key.content
}
```
