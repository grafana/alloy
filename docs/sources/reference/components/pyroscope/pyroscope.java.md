---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.java/
aliases:
  - ../pyroscope.java/ # /docs/alloy/latest/reference/components/pyroscope.java/
description: Learn about pyroscope.java
title: pyroscope.java
---

# pyroscope.java

`pyroscope.java` continuously profiles Java processes running on the local Linux OS using [async-profiler](https://github.com/async-profiler/async-profiler).

{{< admonition type="note" >}}
To use the  `pyroscope.java` component you must run {{< param "PRODUCT_NAME" >}} as root and inside host PID namespace.
{{< /admonition >}}

## Usage

```alloy
pyroscope.java "LABEL" {
  targets    = TARGET_LIST
  forward_to = RECEIVER_LIST
}
```

## Arguments

The following arguments are supported:

Name         | Type                     | Description                                      | Default | Required
-------------|--------------------------|--------------------------------------------------|---------|---------
`targets`    | `list(map(string))`      | List of java process targets to profile.         |         | yes
`forward_to` | `list(ProfilesReceiver)` | List of receivers to send collected profiles to. |         | yes
`tmp_dir`    | `string`                 | Temporary directory to store async-profiler.     | `/tmp`  | no

## Profiling behavior

The special label `__process_pid__` _must always_ be present in each target of `targets` and corresponds to the `PID` of the process to profile.

After component startup, `pyroscope.java` creates a temporary directory under `tmp_dir` and extracts the async-profiler binaries for both glibc and musl into the directory with the following layout.

```
/tmp/alloy-asprof-glibc-{SHA1}/bin/asprof
/tmp/alloy-asprof-glibc-{SHA1}/lib/libasyncProfiler.so
/tmp/alloy-asprof-musl-{SHA1}/bin/asprof
/tmp/alloy-asprof-musl-{SHA1}/lib/libasyncProfiler.so
```

After process profiling startup, the component detects libc type and copies according `libAsyncProfiler.so` into the target process file system at the exact same path.

{{< admonition type="note" >}}
The `asprof` binary runs with root permissions.
If you change the `tmp_dir` configuration to something other than `/tmp`, then you must ensure that the directory is only writable by root.
{{< /admonition >}}

#### `targets` argument

The special `__process_pid__` label _must always_ be present and corresponds to the process PID that's used for profiling.

Labels starting with a double underscore (`__`) are treated as _internal_, and are removed prior to scraping.

The special label `service_name` is required and must always be present.
If it's not specified, `pyroscope.scrape` will attempt to infer it from either of the following sources, in this order:
1. `__meta_kubernetes_pod_annotation_pyroscope_io_service_name` which is a `pyroscope.io/service_name` pod annotation.
2. `__meta_kubernetes_namespace` and `__meta_kubernetes_pod_container_name`
3. `__meta_docker_container_name`
4. `__meta_dockerswarm_container_label_service_name` or `__meta_dockerswarm_service_name`

If `service_name` isn't specified and couldn't be inferred, then it's set to `unspecified`.

## Blocks

The following blocks are supported inside the definition of `pyroscope.java`:

Hierarchy        | Block                | Description                             | Required
-----------------|----------------------|-----------------------------------------|---------
profiling_config | [profiling_config][] | Describes java profiling configuration. | no

[profiling_config]: #profiling_config-block

### profiling_config block

The `profiling_config` block describes how async-profiler is invoked.

The following arguments are supported:

Name          | Type       | Description                                                                                                 | Default  | Required
--------------|------------|-------------------------------------------------------------------------------------------------------------|----------|---------
`interval`    | `duration` | How frequently to collect profiles from the targets.                                                        | "60s"    | no
`cpu`         | `bool`     | A flag to enable cpu profiling, using `itimer` async-profiler event by default.                             | true     | no
`sample_rate` | `int`      | CPU profiling sample rate. It is converted from Hz to interval and passed as an `-i` arg to async-profiler. | 100      | no
`alloc`       | `string`   | Allocation profiling sampling configuration  It is passed as an `--alloc` arg to async-profiler.            | "512k"   | no
`lock`        | `string`   | Lock profiling sampling configuration. It is passed as an `--lock` arg to async-profiler.                   | "10ms"   | no
`event`       | `string`   | Sets the CPU profiling event. Can be one of `itimer`, `cpu` or `wall`.                                      | "itimer" | no
`per_thread`  | `bool`     | Sets per thread mode on async profiler. It is passed as an `-t` arg to async-profiler.                      | false    | no

Refer to [profiler-options](https://github.com/async-profiler/async-profiler?tab=readme-ov-file#profiler-options) for more information about async-profiler configuration.

#### `event` argument

Sets the CPU profiling event:
* `itimer` - Default. Uses the [`setitimer(ITIMER_PROF)`](http://man7.org/linux/man-pages/man2/setitimer.2.html) 
   syscall, which generates a signal every time a process consumes CPU. 
* `cpu` - Uses PMU-case sampling (like Intel PEBS or AMD IBS), can be more accurate than `itimer`, but it's not available on every platform.
* `wall` - This samples all threads equally every given period of time regardless of thread status: Running, Sleeping, or Blocked.
   For example, this can be helpful when profiling application start-up time or IO-intensive processes.

#### `per_thread` argument  
Sets per thread mode on async profiler. Threads are profiled separately and each stack trace will end with a frame that denotes a single thread.

The Wall-clock profiler (`event=wall`) is most useful in per-thread mode.

## Exported fields

`pyroscope.java` doesn't export any fields that can be referenced by other components.

## Component health

`pyroscope.java` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`pyroscope.java` doesn't expose any component-specific debug information.

## Debug metrics

`pyroscope.java` doesn't expose any component-specific debug metrics.

## Examples

### Profile every java process on the current host

```alloy
pyroscope.write "staging" {
  endpoint {
    url = "http://localhost:4040"
  }
}

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

discovery.relabel "java" {
  targets = discovery.process.all.targets
  rule {
    action = "keep"
    regex = ".*/java$"
    source_labels = ["__meta_process_exe"]
  }
}

pyroscope.java "java" {
  targets = discovery.relabel.java.output
  forward_to = [pyroscope.write.staging.receiver]
  profiling_config {
    interval = "60s"
    alloc = "512k"
    cpu = true
    sample_rate = 100
    lock = "1ms"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.java` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
