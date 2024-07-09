---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.process/
aliases:
  - ../discovery.process/ # /docs/alloy/latest/reference/components/discovery.process/
description: Learn about discovery.process
title: discovery.process
---

# discovery.process

`discovery.process` discovers processes running on the local Linux OS.

{{< admonition type="note" >}}
To use the `discovery.process` component you must run {{< param "PRODUCT_NAME" >}} as root and inside host PID namespace.
{{< /admonition >}}

## Usage

```alloy
discovery.process "LABEL" {

}
```

## Arguments

The following arguments are supported:

| Name               | Type                | Description                                                                              | Default | Required |
|--------------------|---------------------|------------------------------------------------------------------------------------------|---------|----------|
| `join`             | `list(map(string))` | Join external targets to discovered processes targets based on `__container_id__` label. |         | no       |
| `refresh_interval` | `duration`          | How often to sync targets.                                                               | "60s"   | no       |

### Targets joining

If `join` is specified, `discovery.process` will join the discovered processes based on the `__container_id__` label.

For example, if `join` is specified as follows:

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

And the discovered processes are:

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

## Blocks

The following blocks are supported inside the definition of
`discovery.process`:

| Hierarchy       | Block               | Description                                    | Required |
|-----------------|---------------------|------------------------------------------------|----------|
| discover_config | [discover_config][] | Configures which process metadata to discover. | no       |

[discover_config]: #discover_config-block

### discover_config block

The `discover_config` block describes which process metadata to discover.

The following arguments are supported:

| Name           | Type   | Description                                                      | Default | Required |
|----------------|--------|------------------------------------------------------------------|---------|----------|
| `exe`          | `bool` | A flag to enable discovering `__meta_process_exe` label.         | true    | no       |
| `cwd`          | `bool` | A flag to enable discovering `__meta_process_cwd` label.         | true    | no       |
| `commandline`  | `bool` | A flag to enable discovering `__meta_process_commandline` label. | true    | no       |
| `uid`          | `bool` | A flag to enable discovering `__meta_process_uid`: label.        | true    | no       |
| `username`     | `bool` | A flag to enable discovering `__meta_process_username`: label.   | true    | no       |
| `container_id` | `bool` | A flag to enable discovering `__container_id__` label.           | true    | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                            |
|-----------|---------------------|--------------------------------------------------------|
| `targets` | `list(map(string))` | The set of processes discovered on the local Linux OS. |

Each target includes the following labels:

* `__process_pid__`: The process PID.
* `__meta_process_exe`: The process executable path. Taken from `/proc/<pid>/exe`.
* `__meta_process_cwd`: The process current working directory. Taken from `/proc/<pid>/cwd`.
* `__meta_process_commandline`: The process command line. Taken from `/proc/<pid>/cmdline`.
* `__meta_process_uid`: The process UID. Taken from `/proc/<pid>/status`.
* `__meta_process_username`: The process username. Taken from `__meta_process_uid` and `os/user/LookupID`.
* `__container_id__`: The container ID. Taken from `/proc/<pid>/cgroup`. If the process is not running in a container, this label is not set.

## Component health

`discovery.process` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.process` does not expose any component-specific debug information.

## Debug metrics

`discovery.process` does not expose any component-specific debug metrics.

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
    container_id = true
  }
}

```

### Example discovering processes on the local host and joining with `discovery.kubernetes`

```alloy
discovery.kubernetes "pyroscope_kubernetes" {
  selectors {
    field = "spec.nodeName=" + env("HOSTNAME")
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
