---
canonical: https://grafana.com/docs/alloy/latest/reference/components/local/local.file/
aliases:
  - ../local.file/ # /docs/alloy/latest/reference/components/local.file/
description: Learn about local.file
title: local.file
---

# local.file

`local.file` exposes the contents of a file on disk to other components.
The file will be watched for changes so that its latest content is always exposed.

The most common use of `local.file` is to load secrets (e.g., API keys) from files.

Multiple `local.file` components can be specified by giving them different labels.

## Usage

```alloy
local.file "LABEL" {
  filename = FILE_NAME
}
```

## Arguments

The following arguments are supported:

Name             | Type       | Description                                        | Default      | Required
-----------------|------------|----------------------------------------------------|--------------|---------
`filename`       | `string`   | Path of the file on disk to watch                  |              | yes
`detector`       | `string`   | Which file change detector to use (fsnotify, poll) | `"fsnotify"` | no
`poll_frequency` | `duration` | How often to poll for file changes                 | `"1m"`       | no
`is_secret`      | `bool`     | Marks the file as containing a [secret][]          | `false`      | no

[secret]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#secrets

{{< docs/shared lookup="reference/components/local-file-arguments-text.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                 | Description
----------|----------------------|---------------------------------------------------
`content` | `string` or `secret` | The contents of the file from the most recent read

The `content` field will have the `secret` type only if the `is_secret` argument was true.

## Component health

`local.file` will be reported as healthy whenever if the watched file was read successfully.

Failing to read the file whenever an update is detected (or after the poll period elapses) will cause the component to be reported as unhealthy.
When unhealthy, exported fields will be kept at the last healthy value.
The read error will be exposed as a log message and in the debug information for the component.

## Debug information

`local.file` does not expose any component-specific debug information.

## Debug metrics

* `local_file_timestamp_last_accessed_unix_seconds` (gauge): The timestamp, in Unix seconds, that the file was last successfully accessed.

## Example

```alloy
local.file "secret_key" {
  filename  = "/var/secrets/password.txt"
  is_secret = true
}
```
