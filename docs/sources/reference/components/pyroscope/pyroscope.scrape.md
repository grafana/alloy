---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.scrape/
aliases:
  - ../pyroscope.scrape/ # /docs/alloy/latest/reference/components/pyroscope.scrape/
description: Learn about pyroscope.scrape
labels:
  stage: general-availability
  products:
    - oss
title: pyroscope.scrape
---

# `pyroscope.scrape`

`pyroscope.scrape` collects [pprof] performance profiles for a given set of HTTP `targets`.

`pyroscope.scrape` mimics the scraping behavior of `prometheus.scrape`.
Similarly to how Prometheus scrapes metrics via HTTP, `pyroscope.scrape` collects profiles via HTTP requests.

Unlike Prometheus, which usually only scrapes one `/metrics` endpoint per target, `pyroscope.scrape` may need to scrape multiple endpoints for the same target.
This is because different types of profiles are scraped on different endpoints.
For example, mutex profiles may be scraped on a `/debug/pprof/delta_mutex` HTTP endpoint, whereas memory consumption may be scraped on a `/debug/pprof/allocs` HTTP endpoint.

The profile paths, protocol scheme, scrape interval, scrape timeout, query parameters, as well as any other settings can be configured within `pyroscope.scrape`.

The `pyroscope.scrape` component regards a scrape as successful if it responded with an HTTP `200 OK` status code and returned the body of a valid [pprof] profile.

If a scrape request fails, the [debug UI][] for `pyroscope.scrape` will show:

* Detailed information about the failure.
* The time of the last successful scrape.
* The labels last used for scraping.

The scraped performance profiles can be forwarded to components such as `pyroscope.write` via the `forward_to` argument.

Multiple `pyroscope.scrape` components can be specified by giving them different labels.

[debug UI]: ../../../../troubleshoot/debug/

## Usage

```alloy
pyroscope.scrape "<LABEL>" {
  targets    = <TARGET_LIST>
  forward_to = <RECEIVER_LIST>
}
```

## Arguments

`pyroscope.scrape` starts a new scrape job to scrape all of the input targets.
Multiple scrape jobs can be started for a single input target when scraping multiple profile types.

You can use the following arguments with `pyroscope.scrape`:

| Name                       | Type                     | Description                                                                                      | Default        | Required |
| -------------------------- | ------------------------ | ------------------------------------------------------------------------------------------------ | -------------- | -------- |
| `targets`                  | `list(map(string))`      | List of targets to scrape.                                                                       |                | yes      |
| `forward_to`               | `list(ProfilesReceiver)` | List of receivers to send scraped profiles to.                                                   |                | yes      |
| `job_name`                 | `string`                 | The job name to override the job label with.                                                     | component name | no       |
| `params`                   | `map(list(string))`      | A set of query parameters with which the target is scraped.                                      |                | no       |
| `scrape_interval`          | `duration`               | How frequently to scrape the targets of this scrape configuration.                               | `"15s"`        | no       |
| `scrape_timeout`           | `duration`               | The timeout for scraping targets of this configuration. Must be larger than `scrape_interval`.   | `"18s"`        | no       |
| `delta_profiling_duration` | `duration`               | The duration for a delta profiling to be scraped. Must be larger than 1 second.                  | `"14s"`        | no       |
| `scheme`                   | `string`                 | The URL scheme with which to fetch metrics from targets.                                         | `"http"`       | no       |
| `bearer_token_file`        | `string`                 | File containing a bearer token to authenticate with.                                             |                | no       |
| `bearer_token`             | `secret`                 | Bearer token to authenticate with.                                                               |                | no       |
| `enable_http2`             | `bool`                   | Whether HTTP2 is supported for requests.                                                         | `true`         | no       |
| `follow_redirects`         | `bool`                   | Whether redirects returned by the server should be followed.                                     | `true`         | no       |
| `http_headers`             | `map(list(secret))`      | Custom HTTP headers to be sent along with each request. The map key is the header name.          |                | no       |
| `proxy_url`                | `string`                 | HTTP proxy to send requests through.                                                             |                | no       |
| `no_proxy`                 | `string`                 | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                | no       |
| `proxy_from_environment`   | `bool`                   | Use the proxy URL indicated by environment variables.                                            | `false`        | no       |
| `proxy_connect_header`     | `map(list(secret))`      | Specifies headers to send to proxies during CONNECT requests.                                    |                | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

Any omitted arguments take on their default values.
If conflicting arguments are being passed, for example, configuring both `bearer_token` and `bearer_token_file`, then `pyroscope.scrape` will fail to start and will report an error.

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `job_name`

The `job_name` argument defaults to the component's unique identifier.

For example, the `job_name` of `pyroscope.scrape "local" { ... }` will be `"pyroscope.scrape.local"`.

### `targets`

The list of `targets` can be provided [statically][example_static_targets], [dynamically][example_dynamic_targets], or a [combination of both][example_static_and_dynamic_targets].

The following special labels can change the behavior of `pyroscope.scrape`:

* `__address__` is the special label that _must always_ be present and corresponds to the `<host>:<port>` that is used for the scrape request.
* `__name__` is the special label that indicates the profile type being collected.
* `__profile_path__` is the special label that holds the path to the profile endpoint on the target (e.g. "/debug/pprof/allocs").
* `__profile_path_prefix__` is the special label that holds an optional prefix to prepend to the profile path (e.g. "/mimir-prometheus").
* `service_name` is a required label that identifies the service being profiled.

Labels starting with a double underscore (`__`) are treated as _internal_, and are removed prior to scraping.

The special label `service_name` is required and must always be present.
If it's not specified, `pyroscope.scrape` will attempt to infer it from either of the following sources, in this order:

1. `__meta_kubernetes_pod_annotation_pyroscope_io_service_name` which is a `pyroscope.io/service_name` pod annotation.
1. `__meta_kubernetes_namespace` and `__meta_kubernetes_pod_container_name`
1. `__meta_docker_container_name`
1. `__meta_dockerswarm_container_label_service_name` or `__meta_dockerswarm_service_name`

If `service_name` isn't specified and couldn't be inferred, then it's set to `unspecified`.

The following labels are automatically injected to the scraped profiles so that they can be linked to a scrape target:

| Label            | Description                                                      |
| ---------------- | ---------------------------------------------------------------- |
| `"job"`          | The `job_name` that the target belongs to.                       |
| `"instance"`     | The `__address__` or `<host>:<port>` of the scrape target's URL. |
| `"service_name"` | The inferred Pyroscope service name.                             |

#### `scrape_interval`

The `scrape_interval` typically refers to the frequency with which {{< param "PRODUCT_NAME" >}} collects performance profiles from the monitored targets.
It represents the time interval between consecutive scrapes or data collection events.
This parameter is important for controlling the trade-off between resource usage and the freshness of the collected data.

If `scrape_interval` is short:

* Advantages:
  * Fewer profiles may be lost if the application being scraped crashes.
* Disadvantages:
  * Greater consumption of CPU, memory, and network resources during scrapes and remote writes.
  * The backend database (Pyroscope) consumes more storage space.

If `scrape_interval` is long:

* Advantages:
  * Lower resource consumption.
* Disadvantages:
  * More profiles may be lost if the application being scraped crashes.
  * If the [delta argument][] is set to `true`, the batch size of each remote write to Pyroscope may be bigger.
    The Pyroscope database may need to be tuned with higher limits.
  * If the [delta argument][] is set to `true`, there is a larger risk of reaching the HTTP server timeouts of the application being scraped.

For example, consider this situation:

* `pyroscope.scrape` is configured with a `scrape_interval` of `"60s"`.
* The application being scraped is running an HTTP server with a timeout of 30 seconds.
* Any scrape HTTP requests where the [delta argument][] is set to `true` will fail, because they will attempt to run for 59 seconds.

## Blocks

You can use the following blocks with `pyroscope.scrape`:

| Block                                                                           | Description                                                                                 | Required |
| ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- | -------- |
| [`authorization`][authorization]                                                | Configure generic authorization to targets.                                                 | no       |
| [`basic_auth`][basic_auth]                                                      | Configure `basic_auth` for authenticating to targets.                                       | no       |
| [`clustering`][clustering]                                                      | Configure the component for when {{< param "PRODUCT_NAME" >}} is running in clustered mode. | no       |
| [`oauth2`][oauth2]                                                              | Configure OAuth 2.0 for authenticating to targets.                                          | no       |
| `oauth2` > [`tls_config`][tls_config]                                           | Configure TLS settings for connecting to targets via OAuth2.                                | no       |
| [`profiling_config`][profiling_config]                                          | Configure profiling settings for the scrape job.                                            | no       |
| `profiling_config` > [`profile.block`][profile.block]                           | Collect profiles on blocks.                                                                 | no       |
| `profiling_config` > [`profile.custom`][profile.custom]                         | Collect custom profiles.                                                                    | no       |
| `profiling_config` > [`profile.fgprof`][profile.fgprof]                         | Collect [`fgprof`][fgprof] profiles.                                                        | no       |
| `profiling_config` > [`profile.godeltaprof_block`][profile.godeltaprof_block]   | Collect [`godeltaprof`][godeltaprof] block profiles.                                        | no       |
| `profiling_config` > [`profile.godeltaprof_memory`][profile.godeltaprof_memory] | Collect [`godeltaprof`][godeltaprof] memory profiles.                                       | no       |
| `profiling_config` > [`profile.godeltaprof_mutex`][profile.godeltaprof_mutex]   | Collect [`godeltaprof`][godeltaprof] mutex profiles.                                        | no       |
| `profiling_config` > [`profile.goroutine`][profile.goroutine]                   | Collect goroutine profiles.                                                                 | no       |
| `profiling_config` > [`profile.memory`][profile.memory]                         | Collect memory profiles.                                                                    | no       |
| `profiling_config` > [`profile.mutex`][profile.mutex]                           | Collect mutex profiles.                                                                     | no       |
| `profiling_config` > [`profile.process_cpu`][profile.process_cpu]               | Collect CPU profiles.                                                                       | no       |
| [`tls_config`][tls_config]                                                      | Configure TLS settings for connecting to targets.                                           | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

Any omitted blocks take on their default values.
For example, if `profile.mutex` isn't specified in the configuration, the defaults documented in [profile.mutex][] are used.

[authorization]: #authorization
[basic_auth]: #basic_auth
[clustering]: #clustering
[oauth2]: #oauth2
[pprof]: https://github.com/google/pprof/blob/main/doc/README.md
[profile.block]: #profileblock
[profile.custom]: #profilecustom
[profile.fgprof]: #profilefgprof
[profile.godeltaprof_block]: #profilegodeltaprof_block
[profile.godeltaprof_memory]: #profilegodeltaprof_memory
[profile.godeltaprof_mutex]: #profilegodeltaprof_mutex
[profile.goroutine]: #profilegoroutine
[profile.memory]: #profilememory
[profile.mutex]: #profilemutex
[profile.process_cpu]: #profileprocess_cpu
[profiling_config]: #profiling_config
[tls_config]: #tls_config

[fgprof]: https://github.com/felixge/fgprof
[godeltaprof]: https://github.com/grafana/pyroscope-go/tree/main/godeltaprof

[delta argument]: #delta-argument

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `clustering`

| Name      | Type   | Description                                       | Default | Required |
| --------- | ------ | ------------------------------------------------- | ------- | -------- |
| `enabled` | `bool` | Enables sharing targets with other cluster nodes. | `false` | yes      |

When {{< param "PRODUCT_NAME" >}} is [using clustering][], and `enabled` is set to true, then this `pyroscope.scrape` component instance opts-in to participating in the cluster to distribute scrape load between all cluster nodes.

Clustering causes the set of targets to be locally filtered down to a unique subset per node, where each node is roughly assigned the same number of targets.
If the state of the cluster changes, such as a new node joins, then the subset of targets to scrape per node is recalculated.

When clustering mode is enabled, all {{< param "PRODUCT_NAME" >}} instances participating in the cluster must use the same configuration file and have access to the same service discovery APIs.

If {{< param "PRODUCT_NAME" >}} is _not_ running in clustered mode, this block is a no-op.

[using clustering]: ../../../../get-started/clustering/

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `profiling_config`

The `profiling_config` block configures the profiling settings when scraping targets.

The following arguments are supported:

| Name          | Type     | Description                                   | Default | Required |
| ------------- | -------- | --------------------------------------------- | ------- | -------- |
| `path_prefix` | `string` | The path prefix to use when scraping targets. |         | no       |

### `profile.block`

The `profile.block` block collects profiles on process blocking.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                | Required |
| --------- | --------- | ------------------------------------------- | ---------------------- | -------- |
| `delta`   | `boolean` | Whether to scrape the profile as a delta.   | `false`                | no       |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `true`                 | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/block"` | no       |

For more information about the `delta` argument, see the [delta argument][] section.

### `profile.custom`

The `profile.custom` block allows for collecting profiles from custom endpoints.
Blocks must be specified with a label:

```alloy
profile.custom "<PROFILE_TYPE>" {
  enabled = true
  path    = "<PROFILE_PATH>"
}
```

You can specify multiple `profile.custom` blocks.
Labels assigned to `profile.custom` blocks must be unique across the component.

The following arguments are supported:

| Name      | Type      | Description                                 | Default | Required |
| --------- | --------- | ------------------------------------------- | ------- | -------- |
| `delta`   | `boolean` | Whether to scrape the profile as a delta.   | `false` | no       |
| `enabled` | `boolean` | Enable this profile type to be scraped.     |         | yes      |
| `path`    | `string`  | The path to the profile type on the target. |         | yes      |

When the `delta` argument is `true`, a `seconds` query parameter is automatically added to requests.
The `seconds` used will be equal to `scrape_interval - 1`.

### `profile.fgprof`

The `profile.fgprof` block collects profiles from an [fgprof][] endpoint.

The following arguments are supported:

| Name      | Type      | Description                                 | Default           | Required |
| --------- | --------- | ------------------------------------------- | ----------------- | -------- |
| `delta`   | `boolean` | Whether to scrape the profile as a delta.   | `true`            | no       |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `false`           | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/fgprof"` | no       |

For more information about the `delta` argument, see the [delta argument][] section.

### `profile.godeltaprof_block`

The `profile.godeltaprof_block` block collects profiles from [godeltaprof][] block endpoint. The delta is computed on the target.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                      | Required |
| --------- | --------- | ------------------------------------------- | ---------------------------- | -------- |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `false`                      | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/delta_block"` | no       |

### `profile.godeltaprof_memory`

The `profile.godeltaprof_memory` block collects profiles from [godeltaprof][] memory endpoint. The delta is computed on the target.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                     | Required |
| --------- | --------- | ------------------------------------------- | --------------------------- | -------- |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `false`                     | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/delta_heap"` | no       |

### `profile.godeltaprof_mutex`

The `profile.godeltaprof_mutex` block collects profiles from [godeltaprof][] mutex endpoint.
The delta is computed on the target.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                      | Required |
| --------- | --------- | ------------------------------------------- | ---------------------------- | -------- |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `false`                      | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/delta_mutex"` | no       |

### `profile.goroutine`

The `profile.goroutine` block collects profiles on the number of goroutines.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                    | Required |
| --------- | --------- | ------------------------------------------- | -------------------------- | -------- |
| `delta`   | `boolean` | Whether to scrape the profile as a delta.   | `false`                    | no       |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `true`                     | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/goroutine"` | no       |

Refer to [delta argument][] for more information about the `delta` argument.

### `profile.memory`

The `profile.memory` block collects profiles on memory consumption.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                 | Required |
| --------- | --------- | ------------------------------------------- | ----------------------- | -------- |
| `delta`   | `boolean` | Whether to scrape the profile as a delta.   | `false`                 | no       |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `true`                  | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/allocs"` | no       |

Refer to [delta argument][] for more information about the `delta` argument.

### `profile.mutex`

The `profile.mutex` block collects profiles on mutexes.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                | Required |
| --------- | --------- | ------------------------------------------- | ---------------------- | -------- |
| `delta`   | `boolean` | Whether to scrape the profile as a delta.   | `false`                | no       |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `true`                 | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/mutex"` | no       |

Refer to [delta argument][] for more information about the `delta` argument.

### `profile.process_cpu`

The `profile.process_cpu` block collects profiles on CPU consumption for the process.

The following arguments are supported:

| Name      | Type      | Description                                 | Default                  | Required |
| --------- | --------- | ------------------------------------------- | ------------------------ | -------- |
| `delta`   | `boolean` | Whether to scrape the profile as a delta.   | `true`                   | no       |
| `enabled` | `boolean` | Enable this profile type to be scraped.     | `true`                   | no       |
| `path`    | `string`  | The path to the profile type on the target. | `"/debug/pprof/profile"` | no       |

For more information about the `delta` argument, see the [delta argument][] section.

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Common configuration

### `delta` argument

When the `delta` argument is `false`, the [pprof][] HTTP query will be instantaneous.

When the `delta` argument is `true`:

* The [pprof][] HTTP query runs for a certain amount of time.
* A `seconds` parameter is automatically added to the HTTP request.
* The default value for the `seconds` query parameter is `scrape_interval - 1`.
  If you set `delta_profiling_duration`, then `seconds` is assigned the same value as `delta_profiling_duration`.
  However, the `delta_profiling_duration` can't be larger than `scrape_interval`.
  For example, if you set `scrape_interval` to `"15s"`, then `seconds` defaults to `14s`
  If you set `delta_profiling_duration` to `16s`, then `scrape_interval` must be set to at least `17s`.
  If the HTTP endpoint is `/debug/pprof/profile`, then the HTTP query becomes `/debug/pprof/profile?seconds=14`

## Exported fields

`pyroscope.scrape` doesn't export any fields that can be referenced by other components.

## Component health

`pyroscope.scrape` is only reported as unhealthy if given an invalid configuration.

## Debug information

`pyroscope.scrape` reports the status of the last scrape for each configured scrape job on the component's debug endpoint.

## Debug metrics

* `pyroscope_fanout_latency` (histogram): Write latency for sending to direct and indirect components.

## Examples

[example_static_targets]: #default-endpoints-of-static-targets

### Default endpoints of static targets

The following example sets up a scrape job of a statically configured list of targets - {{< param "PRODUCT_NAME" >}} itself and Pyroscope.
The scraped profiles are sent to `pyroscope.write` which remote writes them to a Pyroscope database.

```alloy
pyroscope.scrape "local" {
  targets = [
    {"__address__" = "localhost:4040", "service_name"="pyroscope"},
    {"__address__" = "localhost:12345", "service_name"="alloy"},
  ]

  forward_to = [pyroscope.write.local.receiver]
}

pyroscope.write "local" {
  endpoint {
    url = "http://pyroscope:4040"
  }
}
```

These endpoints will be scraped every 15 seconds:

```text
http://localhost:4040/debug/pprof/allocs
http://localhost:4040/debug/pprof/block
http://localhost:4040/debug/pprof/goroutine
http://localhost:4040/debug/pprof/mutex
http://localhost:4040/debug/pprof/profile?seconds=14

http://localhost:12345/debug/pprof/allocs
http://localhost:12345/debug/pprof/block
http://localhost:12345/debug/pprof/goroutine
http://localhost:12345/debug/pprof/mutex
http://localhost:12345/debug/pprof/profile?seconds=14
```

`seconds=14` is added to the `/debug/pprof/profile` endpoint, because:

* The `delta` argument of the `profile.process_cpu` block is `true` by default.
* `scrape_interval` is `"15s"` by default.

The `/debug/fgprof` endpoint won't be scraped, because the `enabled` argument of the `profile.fgprof` block is `false` by default.

[example_dynamic_targets]: #default-endpoints-of-dynamic-targets

### Default endpoints of dynamic targets

```alloy
discovery.http "dynamic_targets" {
  url = "https://example.com/scrape_targets"
  refresh_interval = "15s"
}

pyroscope.scrape "local" {
  targets = [discovery.http.dynamic_targets.targets]

  forward_to = [pyroscope.write.local.receiver]
}

pyroscope.write "local" {
  endpoint {
    url = "http://pyroscope:4040"
  }
}
```

[example_static_and_dynamic_targets]: #default-endpoints-of-static-and-dynamic-targets

### Default endpoints of static and dynamic targets

```alloy
discovery.http "dynamic_targets" {
  url = "https://example.com/scrape_targets"
  refresh_interval = "15s"
}

pyroscope.scrape "local" {
  targets = array.concat([
    {"__address__" = "localhost:4040", "service_name"="pyroscope"},
    {"__address__" = "localhost:12345", "service_name"="alloy"},
  ], discovery.http.dynamic_targets.targets)

  forward_to = [pyroscope.write.local.receiver]
}

pyroscope.write "local" {
  endpoint {
    url = "http://pyroscope:4040"
  }
}
```

### Enable and disable profiles

```alloy
pyroscope.scrape "local" {
  targets = [
    {"__address__" = "localhost:12345", "service_name"="alloy"},
  ]

  profiling_config {
    profile.fgprof {
      enabled = true
    }
    profile.block {
      enabled = false
    }
    profile.mutex {
      enabled = false
    }
  }

  forward_to = [pyroscope.write.local.receiver]
}
```

These endpoints will be scraped every 15 seconds:

```text
http://localhost:12345/debug/pprof/allocs
http://localhost:12345/debug/pprof/goroutine
http://localhost:12345/debug/pprof/profile?seconds=14
http://localhost:12345/debug/fgprof?seconds=14
```

These endpoints will **NOT** be scraped because they are explicitly disabled:

```text
http://localhost:12345/debug/pprof/block
http://localhost:12345/debug/pprof/mutex
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.scrape` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
