---
aliases:
- ./upgrade-guide/
- /docs/grafana-cloud/agent/flow/release-notes/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/release-notes/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/release-notes/
- /docs/grafana-cloud/send-data/agent/flow/release-notes/
canonical: https://grafana.com/docs/agent/latest/flow/release-notes/
description: Release notes for Grafana Agent Flow
menuTitle: Release notes
title: Release notes for Grafana Agent Flow
weight: 999
---

# Release notes for {{% param "PRODUCT_NAME" %}}

The release notes provide information about deprecations and breaking changes in {{< param "PRODUCT_NAME" >}}.

For a complete list of changes to {{< param "PRODUCT_ROOT_NAME" >}}, with links to pull requests and related issues when available, refer to the [Changelog](https://github.com/grafana/agent/blob/main/CHANGELOG.md).

{{< admonition type="note" >}}
These release notes are specific to {{< param "PRODUCT_NAME" >}}.
Other release notes for the different {{< param "PRODUCT_ROOT_NAME" >}} variants are contained on separate pages:

* [Static mode release notes][release-notes-static]
* [Static mode Kubernetes operator release notes][release-notes-operator]

[release-notes-static]: {{< relref "../static/release-notes.md" >}}
[release-notes-operator]: {{< relref "../operator/release-notes.md" >}}
{{< /admonition >}}

## v0.40

### Breaking change: Prohibit the configuration of services within modules.

Previously it was possible to configure the HTTP service via the [HTTP config block](https://grafana.com/docs/agent/v0.39/flow/reference/config-blocks/http/) inside of a module.
This functionality is now only available in the main configuration.

### Breaking change: Change the default value of `disable_high_cardinality_metrics` to `true`.

The `disable_high_cardinality_metrics` configuration argument is used by `otelcol.exporter` components such as `otelcol.exporter.otlp`.
If you need to see high cardinality metrics containing labels such as IP addresses and port numbers, you now have to explicitly set `disable_high_cardinality_metrics` to `false`.

### Breaking change: Rename component `prometheus.exporter.agent` to `prometheus.exporter.self`

The name `prometheus.exporter.agent` is potentially ambiguous and can be misinterpreted as an exporter for Prometheus Agent.
The new name reflects the component's true purpose as an exporter of the process's own metrics.

## v0.39

### Breaking change: `otelcol.receiver.prometheus` will drop all `otel_scope_info` metrics when converting them to OTLP

* If the `otel_scope_info` metric has the `otel_scope_name` and `otel_scope_version` labels,
  their values are used to set the OTLP Instrumentation Scope name and  version, respectively. 
* Labels for `otel_scope_info` metrics other than `otel_scope_name` and `otel_scope_version` 
  are added as scope attributes with the matching name and version.

### Breaking change: label for `target` block in `prometheus.exporter.blackbox` is removed

Previously in `prometheus.exporter.blackbox`, the `target` block requires a label which is used in job's name. 
In this version, user needs to be specify `name` attribute instead, which allow less restrictive naming.

Old configuration example:

```river
prometheus.exporter.blackbox "example" {
  config_file = "blackbox_modules.yml"

  target "grafana" {
    address = "http://grafana.com"
    module  = "http_2xx"
    labels = {
      "env": "dev",
    }
  }
}
```

New configuration example:

```river
prometheus.exporter.blackbox "example" {
  config_file = "blackbox_modules.yml"

  target {
    name     = "grafana"
    address = "http://grafana.com"
    module  = "http_2xx"
    labels = {
      "env": "dev",
    }
  }
}
```

## v0.38

### Breaking change: `otelcol.exporter.jaeger` component removed

The deprecated `otelcol.exporter.jaeger` component has been removed. To send
traces to Jaeger, use `otelcol.exporter.otlp` and a version of Jaeger that
supports OTLP.

## v0.37

### Breaking change: Renamed `non_indexed_labels` Loki processing stage to `structured_metadata`.

If you use the Loki processing stage in your {{< param "PRODUCT_NAME" >}} configuration, you must rename the `non_indexed_labels` pipeline stage definition to `structured_metadata`.

Old configuration example:

```river
stage.non_indexed_labels {
    values = {"app" = ""}
}
```

New configuration example:
```river
stage.structured_metadata {
    values = {"app" = ""}
}
```

### Breaking change: `otelcol.exporter.prometheus` scope labels updated

There are 2 changes to the way scope labels work for this component.

* Previously, the `include_scope_info` argument would trigger including
`otel_scope_name` and `otel_scope_version` in metrics. This is now defaulted
to `true` and controlled via the `include_scope_labels` argument.

* A bugfix was made to rename `otel_scope_info` metric labels from
`name` to `otel_scope_name` and `version` to `otel_scope_version`. This is
now correct with the OTLP Instrumentation Scope specification.

### Breaking change: `prometheus.exporter.unix` now requires a label.

Previously the exporter was a singleton and did not require a label. The exporter now can be used multiple times and
needs a label.

Old configuration example:

```river
prometheus.exporter.unix { /* ... */ }
```

New configuration example:

```river
prometheus.exporter.unix "example" { /* ... */ }
```

## v0.36

### Breaking change: The default value of `retry_on_http_429` is changed to `true` for the `queue_config` in `prometheus.remote_write`

The default value of `retry_on_http_429` is changed from `false` to `true` for the `queue_config` block in `prometheus.remote_write`
so that {{< param "PRODUCT_ROOT_NAME" >}} can retry sending and avoid data being lost for metric pipelines by default.

* If you set the `retry_on_http_429` explicitly - no action is required.
* If you do not set `retry_on_http_429` explicitly and you do *not* want to retry on HTTP 429, make sure you set it to `false` as you upgrade to this new version.

### Breaking change: `loki.source.file` no longer automatically extracts logs from compressed files

`loki.source.file` component will no longer automatically detect and decompress
logs from compressed files (this was an undocumented behaviour).

This file-extension-based detection of compressed files has been replaced by a
new configuration block that explicitly enables and specifies the compression
format. By default, the decompression of files is entirely disabled.

How to migrate:

* If {{< param "PRODUCT_NAME" >}} never reads logs from files with
  extensions `.gz`, `.tar.gz`, `.z` or `.bz2` then no action is required.
  > You can check what are the file extensions {{< param "PRODUCT_NAME" >}} reads from by looking
  at the `path` label on `loki_source_file_file_bytes_total` metric.

* If {{< param "PRODUCT_NAME" >}} extracts data from compressed files, please add the following
  configuration block to your `loki.source.file` component:

    ```river
    loki.source.file "example" {
      ...
      decompression {
        enabled       = true
        format        = "<compression format>"
      }
    }
    ```

    where the `<compression format>` is the appropriate compression format -
    see [`loki.source.file` documentation][loki-source-file-docs] for details.

    [loki-source-file-docs]: {{< relref "./reference/components/loki.source.file.md" >}}

## v0.35

### Breaking change: `auth` and `version` attributes from `walk_params` block of `prometheus.exporter.snmp` have been removed

The `prometheus.exporter.snmp` flow component wraps a new version of SNMP exporter which introduces a new configuration file format.
This new format separates the walk and metric mappings from the connection and authentication settings. This allows for easier configuration of different
auth params without having to duplicate the full walk and metric mapping.

Old configuration example:

```river
prometheus.exporter.snmp "example" {
   config_file = "snmp_modules.yml"

    target "network_switch_1" {
        address     = "192.168.1.2"
        module      = "if_mib"
        walk_params = "public"
    }

    walk_param "public" {
        retries = "2"
        version = "2"
        auth {
            community = "public"
        }
    }
}
```

New configuration example:

```river
prometheus.exporter.snmp "example" {
   config_file = "snmp_modules.yml"

    target "network_switch_1" {
        address     = "192.168.1.2"
        module      = "if_mib"
        walk_params = "public"
        auth        = "public_v2"
    }

    walk_param "public" {
        retries = "2"
    }
}
```

See [Module and Auth Split Migration](https://github.com/prometheus/snmp_exporter/blob/main/auth-split-migration.md) for more details.

### Breaking change: `discovery.file` has been renamed to `local.file_match`

The `discovery.file` component has been renamed to `local.file_match` to make
its purpose more clear: to find files on the local filesystem matching a
pattern.

Renaming `discovery.file` to `local.file_match` also resolves a point of
confusion where `discovery.file` was thought to implement Prometheus' file
service discovery.

Old configuration example:

```river
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

discovery.file "pods" {
  path_targets = discovery.relabel.k8s.output
}
```

New configuration example:

```river
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
```

### Breaking change: `discovery_target_decode` has been removed from the River standard library

The `discovery_target_decode` function was initially added to the River
standard library as an equivalent to Prometheus' file-based discovery and
HTTP-based discovery methods.

However, the Prometheus discovery mechanisms have more functionality than
`discovery_target_decode`:

* Prometheus' `file_sd_configs` can use many files based on pattern matching.
* Prometheus' `http_sd_configs` also support YAML files.

Additionally, it is no longer an accepted pattern to have component-specific
functions to the River standard library.

As a result, `discovery_target_decode` has been removed in favor of using
components.

Old configuration example:

```river
remote.http "example" {
    url = URL_CONTAINING_TARGETS
}

prometehus.scrape "example" {
    targets    = discovery_target_decode(remote.http.example.content)
    forward_to = FORWARD_LIST
}
```

New configuration example:

```river
discovery.http "example" {
    url = URL_CONTAINING_TARGETS
}

prometehus.scrape "example" {
    targets    = discovery.http.example.targets
    forward_to = FORWARD_LIST
}
```

### Breaking change: The algorithm for the "hash" action of `otelcol.processor.attributes` has changed
The hash produced when using `action = "hash"` in the `otelcol.processor.attributes` flow component is now using the more secure SHA-256 algorithm.
The change was made in PR [#22831](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/22831) of opentelemetry-collector-contrib.

### Breaking change: `otelcol.exporter.loki` now includes instrumentation scope in its output

Additional `instrumentation_scope` information will be added to the OTLP log signal, like this:
```
{
    "body": "Example log",
    "traceid": "01020304000000000000000000000000",
    "spanid": "0506070800000000",
    "severity": "error",
    "attributes": {
        "attr1": "1",
        "attr2": "2"
    },
    "resources": {
        "host.name": "something"
    },
    "instrumentation_scope": {
        "name": "example-logger-name",
        "version": "v1"
    }
}
```

### Breaking change: `otelcol.extension.jaeger_remote_sampling` removes the `/` HTTP endpoint

The `/` HTTP endpoint was the same as the `/sampling` endpoint. The `/sampling` endpoint is still functional.
The change was made in PR [#18070](https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/18070) of opentelemetry-collector-contrib.

### Breaking change: The `remote_sampling` block has been removed from `otelcol.receiver.jaeger`

The `remote_sampling` block in `otelcol.receiver.jaeger` has been an undocumented no-op configuration for some time, and has now been removed.
Customers are advised to use `otelcol.extension.jaeger_remote_sampling` instead.

### Deprecation: `otelcol.exporter.jaeger` has been deprecated and will be removed in {{% param "PRODUCT_NAME" %}} v0.38.0.

This is because Jaeger supports OTLP directly and OpenTelemetry Collector is also removing its
[Jaeger receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/jaegerexporter).

## v0.34

### Breaking change: `phlare.scrape` and `phlare.write` have been renamed to `pyroscope.scrape` and `pyroscope.scrape`

Old configuration example:

```river
phlare.write "staging" {
  endpoint {
    url = "http://phlare:4100"
  }
}

phlare.scrape "default" {
  targets = [
    {"__address__" = "agent:12345", "app"="agent"},
  ]
  forward_to = [phlare.write.staging.receiver]
}
```

New configuration example:

```river
pyroscope.write "staging" {
  endpoint {
    url = "http://pyroscope:4100"
  }
}

pyroscope.scrape "default" {
  targets = [
    {"__address__" = "agent:12345", "app"="agent"},
  ]
  forward_to = [pyroscope.write.staging.receiver]
}
```

## v0.33

### Symbolic links in Docker containers removed

We've removed the deprecated symbolic links to `/bin/agent*` in Docker
containers, as planned in v0.31. In case you're setting a custom entrypoint,
use the new binaries that are prefixed with `/bin/grafana*`.

## v0.32

### Breaking change: `http_client_config` Flow blocks merged with parent blocks

To reduce the amount of typing required to write Flow components, the arguments
and subblocks found in `http_client_config` have been merged with their parent
blocks:

- `discovery.docker > http_client_config` is merged into the `discovery.docker` block.
- `discovery.kubernetes > http_client_config` is merged into the `discovery.kubernetes` block.
- `loki.source.kubernetes > client > http_client_config` is merged into the `client` block.
- `loki.source.podlogs > client > http_client_config` is merged into the `client` block.
- `loki.write > endpoint > http_client_config` is merged into the `endpoint` block.
- `mimir.rules.kubernetes > http_client_config` is merged into the `mimir.rules.kubernetes` block.
- `otelcol.receiver.opencensus > grpc` is merged into the `otelcol.receiver.opencensus` block.
- `otelcol.receiver.zipkin > http` is merged into the `otelcol.receiver.zipkin` block.
- `phlare.scrape > http_client_config` is merged into the `phlare.scrape` block.
- `phlare.write > endpoint > http_client_config` is merged into the `endpoint` block.
- `prometheus.remote_write > endpoint > http_client_config` is merged into the `endpoint` block.
- `prometheus.scrape > http_client_config` is merged into the `prometheus.scrape` block.

Old configuration example:

```river
prometheus.remote_write "example" {
  endpoint {
    url = URL

    http_client_config {
      basic_auth {
        username = BASIC_AUTH_USERNAME
        password = BASIC_AUTH_PASSWORD
      }
    }
  }
}
```

New configuration example:

```river
prometheus.remote_write "example" {
  endpoint {
    url = URL

    basic_auth {
      username = BASIC_AUTH_USERNAME
      password = BASIC_AUTH_PASSWORD
    }
  }
}
```

### Breaking change: `loki.process` stage blocks combined into new blocks

Previously, to add a stage to `loki.process`, two blocks were needed: a block
called `stage`, then an inner block for the stage being written. Stage blocks
are now a single block called `stage.STAGENAME`.

Old configuration example:

```river
loki.process "example" {
  forward_to = RECEIVER_LIST

  stage {
    docker {}
  }

  stage {
    json {
      expressions = { output = "log", extra = "" }
    }
  }
}
```

New configuration example:

```river
loki.process "example" {
  forward_to = RECEIVER_LIST

  stage.docker {}

  stage.json {
    expressions = { output = "log", extra = "" }
  }
}
```

### Breaking change: `client_options` block renamed in `remote.s3` component

To synchronize naming conventions between `remote.s3` and `remote.http`, the
`client_options` block has been renamed `client`.

Old configuration example:

```river
remote.s3 "example" {
  path = S3_PATH

  client_options {
    key    = ACCESS_KEY
    secret = KEY_SECRET
  }
}
```

New configuration example:

```river
remote.s3 "example" {
  path = S3_PATH

  client {
    key    = ACCESS_KEY
    secret = KEY_SECRET
  }
}
```

### Breaking change: `prometheus.integration.node_exporter` component name changed

The `prometheus.integration.node_exporter` component has been renamed to
`prometheus.exporter.unix`. `unix` was chosen as a name to approximate the
\*nix-like systems the exporter supports.

Old configuration example:

```river
prometheus.integration.node_exporter { }
```

New configuration example:

```river
prometheus.exporter.unix { }
```

### Breaking change: support for `EXPERIMENTAL_ENABLE_FLOW` environment variable removed

As first announced in v0.30.0, support for using the `EXPERIMENTAL_ENABLE_FLOW`
environment variable to enable Flow mode has been removed.

To enable {{< param "PRODUCT_NAME" >}}, set the `AGENT_MODE` environment variable to `flow`.

## v0.31

### Breaking change: binary names are now prefixed with `grafana-`

As first announced in v0.29, the `agent` release binary name is now prefixed
with `grafana-`:

- `agent` is now `grafana-agent`.

For the `grafana/agent` Docker container, the entrypoint is now
`/bin/grafana-agent`. A symbolic link from `/bin/agent` to the new binary has
been added.

Symbolic links will be removed in v0.33. Custom entrypoints must be
updated prior to v0.33 to use the new binaries before the symbolic links get
removed.

## v0.30

### Deprecation: `EXPERIMENTAL_ENABLE_FLOW` environment variable changed

As part of graduating {{< param "PRODUCT_NAME" >}} to beta, the
`EXPERIMENTAL_ENABLE_FLOW` environment variable is replaced by setting
`AGENT_MODE` to `flow`.

Setting `EXPERIMENTAL_ENABLE_FLOW` to `1` or `true` is now deprecated and
support for it will be removed for the v0.32 release.

## v0.29

### Deprecation: binary names will be prefixed with `grafana-` in v0.31.0

The binary name `agent` has been deprecated and will be renamed to
`grafana-agent` in the v0.31.0 release.

As part of this change, the Docker containers for the v0.31.0 release will
include symbolic links from the old binary names to the new binary names.

There is no action to take at this time.
