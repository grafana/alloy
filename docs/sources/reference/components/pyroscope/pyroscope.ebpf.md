---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.ebpf/
aliases:
  - ../pyroscope.ebpf/ # /docs/alloy/latest/reference/components/pyroscope.ebpf/
description: Learn about pyroscope.ebpf
title: pyroscope.ebpf
---

# pyroscope.ebpf

`pyroscope.ebpf` configures an eBPF profiling job for the current host.
The collected performance profiles are forwarded to the list of receivers passed in `forward_to`.

{{< admonition type="note" >}}
To use the  `pyroscope.ebpf` component you must run {{< param "PRODUCT_NAME" >}} as root and inside the host PID namespace.
{{< /admonition >}}

You can specify multiple `pyroscope.ebpf` components by giving them different labels, however it's not recommended as it can lead to additional memory and CPU usage.

## Supported languages

This eBPF profiler only collects CPU profiles. Generally, natively compiled languages like C/C++, Go, and Rust are supported. Refer to [Troubleshooting unknown symbols][troubleshooting] for additional requirements.

Python is the only supported high-level language, as long as `python_enabled=true`.
Other high-level languages like Java, Ruby, PHP, and JavaScript require additional work to show stack traces of methods in these languages correctly.
Currently, the CPU usage for these languages is reported as belonging to the runtime's methods.

## Usage

```alloy
pyroscope.ebpf "LABEL" {
  targets    = TARGET_LIST
  forward_to = RECEIVER_LIST
}
```

## Arguments

The component configures and starts a new eBPF profiling job to collect performance profiles from the current host.

You can use the following arguments to configure a `pyroscope.ebpf`.
Only the `forward_to` and `targets` fields are required.
Omitted fields take their default values.

Name                      | Type                     | Description                                                                         | Default | Required
--------------------------|--------------------------|-------------------------------------------------------------------------------------|---------|---------
`targets`                 | `list(map(string))`      | List of targets to group profiles by container id                                   |         | yes
`forward_to`              | `list(ProfilesReceiver)` | List of receivers to send collected profiles to.                                    |         | yes
`collect_interval`        | `duration`               | How frequently to collect profiles                                                  | `15s`   | no
`sample_rate`             | `int`                    | How many times per second to collect profile samples                                | 97      | no
`pid_cache_size`          | `int`                    | The size of the pid -> proc symbols table LRU cache                                 | 32      | no
`build_id_cache_size`     | `int`                    | The size of the elf file build id -> symbols table LRU cache                        | 64      | no
`same_file_cache_size`    | `int`                    | The size of the elf file -> symbols table LRU cache                                 | 8       | no
`container_id_cache_size` | `int`                    | The size of the pid -> container ID table LRU cache                                 | 1024    | no
`collect_user_profile`    | `bool`                   | A flag to enable/disable collection of userspace profiles                           | true    | no
`collect_kernel_profile`  | `bool`                   | A flag to enable/disable collection of kernelspace profiles                         | true    | no
`demangle`                | `string`                 | C++ demangle mode. Available options are: `none`, `simplified`, `templates`, `full` | `none`  | no
`python_enabled`          | `bool`                   | A flag to enable/disable python profiling                                           | true    | no
`symbols_map_size`        | `int`                    | The size of eBPF symbols map                                                        | 16384   | no
`pid_map_size`            | `int`                    | The size of eBPF PID map                                                            | 2048    | no

## Exported fields

`pyroscope.ebpf` does not export any fields that can be referenced by other components.

## Component health

`pyroscope.ebpf` is only reported as unhealthy if given an invalid configuration.

## Debug information

* `targets` currently tracked active targets.
* `pid_cache` per process elf symbol tables and their sizes in symbols count.
* `elf_cache` per build id and per same file symbol tables and their sizes in symbols count.

## Debug metrics

* `pyroscope_fanout_latency` (histogram): Write latency for sending to direct and indirect components.
* `pyroscope_ebpf_active_targets` (gauge): Number of active targets the component tracks.
* `pyroscope_ebpf_profiling_sessions_total` (counter): Number of profiling sessions completed.
* `pyroscope_ebpf_profiling_sessions_failing_total` (counter): Number of profiling sessions failed.
* `pyroscope_ebpf_pprofs_total` (counter): Number of pprof profiles collected by the eBPF component.

## Profile collecting behavior

The `pyroscope.ebpf` component collects stack traces associated with a process running on the current host.
You can use the `sample_rate` argument to define the number of stack traces collected per second. The default is 97.

The following labels are automatically injected into the collected profiles if you haven't defined them.
These labels can help you pin down a profiling target.

Label              | Description
-------------------|---------------------------------------------------------------------------------------------------------------------------------
`service_name`     | Pyroscope service name. It's automatically selected from discovery meta labels if possible. Otherwise defaults to `unspecified`.
`__name__`         | pyroscope metric name. Defaults to `process_cpu`.
`__container_id__` | The container ID derived from target.

### Targets

One of the following special labels _must_ be included in each target of `targets` and the label must correspond to the container or process that is profiled:

* `__container_id__`: The container ID.
* `__meta_docker_container_id`: The ID of the Docker container.
* `__meta_kubernetes_pod_container_id`: The ID of the Kubernetes pod container.
* `__process_pid__` : The process ID.

Each process is then associated with a specified target from the targets list, determined by a container ID or process PID.

If a process's container ID matches a target's container ID label, the stack traces are aggregated per target based on the container ID.
If a process's PID matches a target's process PID label, the stack traces are aggregated per target based on the process PID.
Otherwise the process isn't profiled.

### Service name

The special label `service_name` is required and must always be present.
If it's not specified, it's attempted to be inferred from multiple sources:

- `__meta_kubernetes_pod_annotation_pyroscope_io_service_name` which is a `pyroscope.io/service_name` pod annotation.
- `__meta_kubernetes_namespace` and `__meta_kubernetes_pod_container_name`
- `__meta_docker_container_name`

If `service_name` isn't specified and couldn't be inferred, it's set to `unspecified`.

## Troubleshooting unknown symbols

Symbols are extracted from various sources, including:

- The `.symtab` and `.dynsym` sections in the ELF file.
- The `.symtab` and `.dynsym` sections in the debug ELF file.
- The `.gopclntab` section in Go language ELF files.

The search for debug files follows [gdb algorithm][].
For example, if the profiler wants to find the debug file for `/lib/x86_64-linux-gnu/libc.so.6` with a `.gnu_debuglink` set to `libc.so.6.debug` and a build ID `0123456789abcdef`.
The following paths are examined:

- `/usr/lib/debug/.build-id/01/0123456789abcdef.debug`
- `/lib/x86_64-linux-gnu/libc.so.6.debug`
- `/lib/x86_64-linux-gnu/.debug/libc.so.6.debug`
- `/usr/lib/debug/lib/x86_64-linux-gnu/libc.so.6.debug`

### Dealing with unknown symbols

Unknown symbols in the profiles you’ve collected indicate that the profiler couldn't access an ELF file associated with a given address in the trace.

This can occur for several reasons:

- The process has terminated, making the ELF file inaccessible.
- The ELF file is either corrupted or not recognized as an ELF file.
- There is no corresponding ELF file entry in `/proc/pid/maps` for the address in the stack trace.

### Addressing unresolved symbols

If you only see module names (e.g., `/lib/x86_64-linux-gnu/libc.so.6`) without corresponding function names, this indicates that the symbols couldn't be mapped to their respective function names.

This can occur for several reasons:

- The binary has been stripped, leaving no .symtab, .dynsym, or .gopclntab sections in the ELF file.
- The debug file is missing or could not be located.

To fix this for your binaries, ensure that they are either not stripped or that you have separate debug files available.
You can achieve this by running:

```bash
objcopy --only-keep-debug elf elf.debug
strip elf -o elf.stripped
objcopy --add-gnu-debuglink=elf.debug elf.stripped elf.debuglink
```

For system libraries, ensure that debug symbols are installed.
On Ubuntu, for example, you can install debug symbols for `libc` by executing:

```bash
apt install libc6-dbg
```

### Understanding flat stack traces

If your profiles show many shallow stack traces, typically 1-2 frames deep, your binary might have been compiled without frame pointers.

To compile your code with frame pointers, include the `-fno-omit-frame-pointer` flag in your compiler options.


## Example

### Kubernetes discovery

In the following example, performance profiles are collected from pods on the same node, discovered using `discovery.kubernetes`.
Pod selection relies on the `HOSTNAME` environment variable, which is a pod name if {{< param "PRODUCT_NAME" >}} is used as an {{< param "PRODUCT_NAME" >}} Helm chart.
The `service_name` label is set to `{__meta_kubernetes_namespace}/{__meta_kubernetes_pod_container_name}` from Kubernetes meta labels.

```alloy
discovery.kubernetes "all_pods" {
  role = "pod"
  selectors {
    field = "spec.nodeName=" + env("HOSTNAME")
    role = "pod"
  }
}

discovery.relabel "local_pods" {
  targets = discovery.kubernetes.all_pods.targets
  rule {
    action = "drop"
    regex = "Succeeded|Failed"
    source_labels = ["__meta_kubernetes_pod_phase"]
  }
  rule {
    action = "replace"
    regex = "(.*)@(.*)"
    replacement = "ebpf/${1}/${2}"
    separator = "@"
    source_labels = ["__meta_kubernetes_namespace", "__meta_kubernetes_pod_container_name"]
    target_label = "service_name"
  }
  rule {
    action = "labelmap"
    regex = "__meta_kubernetes_pod_label_(.+)"
  }
  rule {
    action = "replace"
    source_labels = ["__meta_kubernetes_namespace"]
    target_label = "namespace"
  }
  rule {
    action = "replace"
    source_labels = ["__meta_kubernetes_pod_name"]
    target_label = "pod"
  }
  rule {
    action = "replace"
    source_labels = ["__meta_kubernetes_node_name"]
    target_label = "node"
  }
  rule {
    action = "replace"
    source_labels = ["__meta_kubernetes_pod_container_name"]
    target_label = "container"
  }
}
pyroscope.ebpf "local_pods" {
  forward_to = [ pyroscope.write.endpoint.receiver ]
  targets = discovery.relabel.local_pods.output
}

pyroscope.write "endpoint" {
  endpoint {
    url = "http://pyroscope:4100"
  }
}
```

### Docker discovery

The following example collects performance profiles from containers discovered by `discovery.docker` and ignores all other profiles collected from outside any docker container.
The `service_name` label is set to the `__meta_docker_container_name` label.

```alloy
discovery.docker "linux" {
  host = "unix:///var/run/docker.sock"
}

discovery.relabel "local_containers" {
  targets = discovery.docker.linux.targets
  rule {
    action = "replace"
    source_labels = ["__meta_docker_container_name"]
    target_label = "service_name"
  }
}

pyroscope.write "staging" {
  endpoint {
    url = "http://pyroscope:4100"
  }
}

pyroscope.ebpf "default" {
  forward_to   = [ pyroscope.write.staging.receiver ]
  targets      = discovery.relabel.local_containers.output
}
```

[troubleshooting]: #troubleshooting-unknown-symbols
[gdb algorithm]: https://sourceware.org/gdb/onlinedocs/gdb/Separate-Debug-Files.html

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.ebpf` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
