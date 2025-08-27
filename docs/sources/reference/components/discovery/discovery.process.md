---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.process/
aliases:
  - ../discovery.process/ # /docs/alloy/latest/reference/components/discovery.process/
description: Learn about discovery.process
labels:
  stage: general-availability
  products:
    - oss
title: discovery.process
---

# `discovery.process`

`discovery.process` discovers processes running on the local Linux OS.

{{< admonition type="note" >}}
To use the `discovery.process` component you must run {{< param "PRODUCT_NAME" >}} as root and inside host PID namespace.
{{< /admonition >}}

## Usage

```alloy
discovery.process "<LABEL>" {

}
```

## Arguments

You can use the following arguments with `discovery.process`:

| Name               | Type                | Description                                                                              | Default | Required |
| ------------------ | ------------------- | ---------------------------------------------------------------------------------------- | ------- | -------- |
| `join`             | `list(map(string))` | Join external targets to discovered processes targets based on `__container_id__` label. |         | no       |
| `refresh_interval` | `duration`          | How often to sync targets.                                                               | `"60s"` | no       |

### Targets joining

If you specify `join`, `discovery.process` joins the discovered processes based on the `__container_id__` label.
This component alternatively joins targets by `__meta_kubernetes_pod_container_id` or `__meta_docker_container_id`, which allows a simple integration with the output from other discovery components like `discovery.kubernetes`.
The example [discovering processes on the local host and joining with `discovery.kubernetes`][example_discovery_kubernetes] demonstrates this.

For example, if `join` is specified as the following external targets:

```json
[
  {
    "pod": "pod-1",
    "__container_id__": "container-1"
  },
  {
    "pod": "pod-2",
    "__container_id__": "container-2"
  }
]
```

And the discovered process targets are:

```json
[
  {
    "__process_pid__": "1",
    "__container_id__": "container-1"
  },
  {
    "__process_pid__": "2"
  }
]
```

The resulting targets are:

```json
[
  {
    "__container_id__": "container-1",
    "__process_pid__": "1",
    "pod": "pod-1"
  },
  {
    "__process_pid__": "2"
  },
  {
    "__container_id__": "container-1",
    "pod": "pod-1"
  },
  {
    "__container_id__": "container-2",
    "pod": "pod-2"
  }
]
```

The four targets are updated as follows:

1. The first external target is merged with the first discovered process target, joined by `__container_id__=1`.
1. The second discovered process target has no matching external target.
1. The first original external target has no matching discovered process target.
1. The second original external target has no matching discovered process target.

[example_discovery_kubernetes]: #example-discovering-processes-on-the-local-host-and-joining-with-discoverykubernetes

## Blocks

You can use the following block with `discovery.process`:

| Block                                | Description                                    | Required |
| ------------------------------------ | ---------------------------------------------- | -------- |
| [`discover_config`][discover_config] | Configures which process metadata to discover. | no       |

[discover_config]: #discover_config

### `discover_config`

The `discover_config` block describes which process metadata to discover.

The following arguments are supported:

| Name           | Type   | Description                                                      | Default | Required |
| -------------- | ------ | ---------------------------------------------------------------- | ------- | -------- |
| `exe`          | `bool` | A flag to enable discovering `__meta_process_exe` label.         | `true`  | no       |
| `cwd`          | `bool` | A flag to enable discovering `__meta_process_cwd` label.         | `true`  | no       |
| `commandline`  | `bool` | A flag to enable discovering `__meta_process_commandline` label. | `true`  | no       |
| `uid`          | `bool` | A flag to enable discovering `__meta_process_uid` label.         | `true`  | no       |
| `username`     | `bool` | A flag to enable discovering `__meta_process_username` label.    | `true`  | no       |
| `cgroup_path`  | `bool` | A flag to enable discovering `__meta_cgroup_path__` label.       | `false` | no       |
| `container_id` | `bool` | A flag to enable discovering `__container_id__` label.           | `true`  | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                            |
| --------- | ------------------- | ------------------------------------------------------ |
| `targets` | `list(map(string))` | The set of processes discovered on the local Linux OS. |

Each target includes the following labels:

* `__container_id__`: The container ID. Taken from `/proc/<pid>/cgroup`. If the process isn't running in a container, this label isn't set.
* `__meta_cgroup_path`: The cgroup path under which the process is running. In the case of cgroups v1, this label includes all the controllers paths delimited by `|`.
* `__meta_process_commandline`: The process command line. Taken from `/proc/<pid>/cmdline`.
* `__meta_process_cwd`: The process current working directory. Taken from `/proc/<pid>/cwd`.
* `__meta_process_exe`: The process executable path. Taken from `/proc/<pid>/exe`.
* `__meta_process_uid`: The process UID. Taken from `/proc/<pid>/status`.
* `__meta_process_username`: The process username. Taken from `__meta_process_uid` and `os/user/LookupID`.
* `__process_pid__`: The process PID.

## Component health

`discovery.process` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.process` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.process` doesn't expose any component-specific debug metrics.

## Examples

### Example discovering processes on the local host

```alloy
discovery.process "all" {
  refresh_interval = "60s"
  discover_config {
    cwd = true
    exe = true
    commandline = true
    username = true
    uid = true
    cgroup_path = true
    container_id = true
  }
}

```

### Example discovering processes on the local host and joining with `discovery.kubernetes`

```alloy
discovery.kubernetes "pyroscope_kubernetes" {
  selectors {
    field = "spec.nodeName=" + sys.env("HOSTNAME")
    role = "pod"
  }
  role = "pod"
}

discovery.process "all" {
  join = discovery.kubernetes.pyroscope_kubernetes.targets
  refresh_interval = "60s"
  discover_config {
    cwd = true
    exe = true
    commandline = true
    username = true
    uid = true
    container_id = true
  }
}
```

### Example discovering processes on the local host based on `cgroups` path

The following example configuration shows you how to discover processes running under systemd services on the local host.

```alloy
discovery.process "all" {
  refresh_interval = "60s"
  discover_config {
    cwd = true
    exe = true
    commandline = true
    username = true
    uid = true
    cgroup_path = true
    container_id = true
  }
}

discovery.relabel "systemd_services" {
  targets = discovery.process.all.targets
  // Only keep the targets that correspond to systemd services
  rule {
    action = "keep"
    regex = "^.*/([a-zA-Z0-9-_]+).service(?:.*$)"
    source_labels = ["__meta_cgroup_id"]
  }
}

```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.process` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)

`discovery.process` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
