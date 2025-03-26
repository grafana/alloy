---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.fsdump/
aliases:
  - ../pyroscope.fsdump/ # /docs/alloy/latest/reference/components/pyroscope.fsdump/
description: Learn about pyroscope.fsdump
labels:
  stage: public-preview
title: pyroscope.fsdump
---

# `pyroscope.fsdump`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `pyroscope.fsdump` component receives performance profiles from other components and dumps them to files in a specified filesystem directory.
Each profile is written to a unique file with a UUID-based filename, including both the raw profile data and the associated series labels.

You can use `pyroscope.fsdump` alongside `pyroscope.write` to archive profiles locally while still forwarding them to a remote Pyroscope instance.
This is useful for debugging, auditing, or creating backup copies of performance profiles.

## Usage

```alloy
pyroscope.fsdump "<LABEL>" {
  target_directory = "<DIRECTORY_PATH>"
  
  // Optional settings
  max_size_bytes = 1073741824  // 1GB default
  external_labels = {
    "environment" = "production",
  }
  
  rule {
    // Optional relabeling rules
  }
}
```

## Arguments

You can use the following arguments with `pyroscope.fsdump`:

| Name               | Type          | Description                                                       | Default | Required |
| ------------------ | ------------- | ----------------------------------------------------------------- | ------- | -------- |
| `target_directory` | `string`      | Directory where profile files should be written.                  |         | yes      |
| `max_size_bytes`   | `int`         | Maximum total size of all files in the target directory in bytes. | 1GB     | no       |
| `external_labels`  | `map(string)` | Labels to add to all profiles before writing to files.            |         | no       |

## Blocks

The following blocks are supported inside the definition of `pyroscope.fsdump`:

| Block                        | Description                                       | Required |
| ---------------------------- | ------------------------------------------------- | -------- |
| [`rule`][rule]               | Relabeling rules to apply to profiles.            | no       |

[rule]: #rule

### rule

{{< docs/shared lookup="reference/components/rule-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type       | Description                                               |
| ---------- | ---------- | --------------------------------------------------------- |
| `receiver` | `receiver` | A value that other components can use to send profiles to.|

## Component health

`pyroscope.fsdump` is only reported as unhealthy if given an invalid configuration.

## Debug information

`pyroscope.fsdump` exposes the following component-specific debug metrics:

* `pyroscope_fsdump_profiles_received_total` (counter): Total number of profiles received.
* `pyroscope_fsdump_profiles_written_total` (counter): Total number of profiles written to files.
* `pyroscope_fsdump_profiles_dropped_total` (counter): Total number of profiles dropped by relabeling rules.
* `pyroscope_fsdump_bytes_written_total` (counter): Total number of bytes written to files.
* `pyroscope_fsdump_write_errors_total` (counter): Total number of errors encountered when writing profiles.
* `pyroscope_fsdump_files_removed_total` (counter): Total number of files removed by cleanup operations.
* `pyroscope_fsdump_current_size_bytes` (gauge): Current total size of all files in the target directory.

## Example

```alloy
pyroscope.fsdump "archiver" {
  target_directory = "/var/profiles"
  max_size_bytes = 5368709120  // 5GB
  
  external_labels = {
    "environment" = "production",
    "region" = "us-west-1",
  }
  
  // Only keep CPU profiles for certain services
  rule {
    source_labels = ["__name__", "service_name"]
    regex = ".*\\.cpu;(api|database|cache).*"
    action = "keep"
  }
}

// Use both pyroscope.write and pyroscope.fsdump in parallel
pyroscope.scrape "default" {
  targets = [
    {"__address__" = "alloy:12345", "service_name" = "alloy"},
    {"__address__" = "api:12345", "service_name" = "api"},
  ]
  forward_to = [
    pyroscope.write.production.receiver,
    pyroscope.fsdump.archiver.receiver,
  ]
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.fsdump` has exports that can be consumed by the following components:

- Components that consume [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS --> 