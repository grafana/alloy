---
canonical: https://grafana.com/docs/alloy/latest/reference/components/local/local.file_match/
aliases:
  - ../local.file_match/ # /docs/alloy/latest/reference/components/local.file_match/
description: Learn about local.file_match
labels:
  stage: general-availability
title: local.file_match
---

# `local.file_match`

`local.file_match` discovers files on the local filesystem using glob patterns and the [doublestar][] library.

[doublestar]: https://github.com/bmatcuk/doublestar

## Usage

```alloy
local.file_match "LABEL" {
  path_targets = [{"__path__" = DOUBLESTAR_PATH}]
}
```

## Arguments

You can use the following arguments with `local.file_match`:

Name                | Type                | Description                                                                                | Default | Required
---------------     | ------------------- | ------------------------------------------------------------------------------------------ |---------| --------
`path_targets`      | `list(map(string))` | Targets to expand; looks for glob patterns on the  `__path__` and `__path_exclude__` keys. |         | yes
`ignore_older_than` | `duration`          | Ignores files which are modified before this duration.                                     |  `"0s"` | no
`sync_period`       | `duration`          | How often to sync filesystem and targets.                                                  | `"10s"` | no

`path_targets` uses [doublestar][] style paths.

* `/tmp/**/*.log` matches all subdirectories of `tmp` and include any files that end in `*.log`.
* `/tmp/apache/*.log` matches only files in `/tmp/apache/` that end in `*.log`.
* `/tmp/**` matches all subdirectories of `tmp`, `tmp` itself, and all files.

`local.file_match` doesn't ignore files when `ignore_older_than` is set to the default, `0s`.

## Blocks

The `local.file_match` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|---------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the filesystem.

Each target includes the following label:

* `__path__`: Absolute path to the file.

## Component health

`local.file_match` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`local.file_match` doesn't expose any component-specific debug information.

## Debug metrics

`local.file_match` doesn't expose any component-specific debug metrics.

## Examples

The following examples show you how to use `local.file_match` to find and send log files to Loki

### Send `/tmp/logs/*.log` files to Loki

This example discovers all files and folders under `/tmp/logs`.
The absolute paths are used by `loki.source.file.files` targets.

```alloy
local.file_match "tmp" {
  path_targets = [{"__path__" = "/tmp/logs/**/*.log"}]
}

loki.source.file "files" {
  targets    = local.file_match.tmp.targets
  forward_to = [loki.write.endpoint.receiver]
}

loki.write "endpoint" {
  endpoint {
      url = <LOKI_URL>
      basic_auth {
          username = <USERNAME>
          password = <PASSWORD>
      }
  }
}
```

Replace the following:

* _`<LOKI_URL>`_: The URL of the Loki server to send logs to.
* _`<USERNAME>`_: The username to use for authentication to the Loki API.
* _`<PASSWORD>`_: The password to use for authentication to the Loki API.

### Send Kubernetes Pod logs to Loki

This example finds all the logs on Pods and monitors them.

```alloy
discovery.kubernetes "k8s" {
  role = "pod"
}

discovery.relabel "k8s" {
  targets = discovery.kubernetes.k8s.targets

  rule {
    source_labels = ["__meta_kubernetes_namespace", "__meta_kubernetes_pod_label_name"]
    target_label  = "job"
    separator     = "/"
  }

  rule {
    source_labels = ["__meta_kubernetes_pod_uid", "__meta_kubernetes_pod_container_name"]
    target_label  = "__path__"
    separator     = "/"
    replacement   = "/var/log/pods/*$1/*.log"
  }
}

local.file_match "pods" {
  path_targets = discovery.relabel.k8s.output
}

loki.source.file "pods" {
  targets = local.file_match.pods.targets
  forward_to = [loki.write.endpoint.receiver]
}

loki.write "endpoint" {
  endpoint {
      url = <LOKI_URL>
      basic_auth {
          username = <USERNAME>
          password = <PASSWORD>
      }
  }
}
```

Replace the following:

* _`<LOKI_URL>`_: The URL of the Loki server to send logs to.
* _`<USERNAME>`_: The username to use for authentication to the Loki API.
* _`<PASSWORD>`_: The password to use for authentication to the Loki API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`local.file_match` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)

`local.file_match` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
