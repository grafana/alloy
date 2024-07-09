---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.blackbox/
aliases:
  - ../prometheus.exporter.blackbox/ # /docs/alloy/latest/reference/components/prometheus.exporter.blackbox/
description: Learn about prometheus.exporter.blackbox
title: prometheus.exporter.blackbox
---

# prometheus.exporter.blackbox

The `prometheus.exporter.blackbox` component embeds
[`blackbox_exporter`](https://github.com/prometheus/blackbox_exporter). `blackbox_exporter` lets you collect blackbox metrics (probes) and expose them as Prometheus metrics.

## Usage

```alloy
prometheus.exporter.blackbox "LABEL" {
  target {
    name    = "example"
    address = "EXAMPLE_ADDRESS"
  }
}
```

or 

```alloy
prometheus.exporter.blackbox "LABEL" {
  targets = TARGET_LIST
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.

| Name                   | Type                 | Description                                                      | Default  | Required |
| ---------------------- | -------------------- | ---------------------------------------------------------------- | -------- | -------- |
| `config_file`          | `string`             | `blackbox_exporter` configuration file path.                       |          | no       |
| `config`               | `string` or `secret` | `blackbox_exporter` configuration as inline string.                |          | no       |
| `probe_timeout_offset` | `duration`           | Offset in seconds to subtract from timeout when probing targets. | `"0.5s"` | no       |
| `targets`              | `list(map(string))`  | Blackbox targets.                                                |          | no       |

Either `config_file` or `config` must be specified.
The `config_file` argument points to a YAML file defining which `blackbox_exporter` modules to use.
The `config` argument must be a YAML document as string defining which `blackbox_exporter` modules to use.
`config` is typically loaded by using the exports of another component. For example,

- `local.file.LABEL.content`
- `remote.http.LABEL.content`
- `remote.s3.LABEL.content`

You can't use both the `targets` argument and the [target][] block in the same configuration file.
The `targets` argument must be used when blackbox targets cannot be passed as a target block because another component supplies them.

You can set the following labels to a target:
* `name`: The name of the target to probe (required).
* `address`: The address of the target to probe (required).
* `module`: The blackbox module to use to probe.

The component passes any additional labels to the exported target.

Refer to [`blackbox_exporter`](https://github.com/prometheus/blackbox_exporter/blob/master/example.yml) for more information about generating a configuration file.

## Blocks

The following blocks are supported inside the definition of
`prometheus.exporter.blackbox` to configure collector-specific options:

| Hierarchy | Name       | Description                   | Required |
| --------- | ---------- | ----------------------------- | -------- |
| target    | [target][] | Configures a blackbox target. | no       |

[target]: #target-block

### target block

The `target` block defines an individual blackbox target.
The `target` block may be specified multiple times to define multiple targets. `name` attribute is required and will be used in the target's `job` label.

| Name      | Type          | Description                         | Default | Required |
| --------- | ------------- | ----------------------------------- | ------- | -------- |
| `name`    | `string`      | The name of the target to probe.    |         | yes      |
| `address` | `string`      | The address of the target to probe. |         | yes      |
| `module`  | `string`      | Blackbox module to use to probe.    | `""`    | no       |
| `labels`  | `map(string)` | Labels to add to the target.        |         | no       |

Labels specified in the `labels` argument won't override labels set by `blackbox_exporter`.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.blackbox` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.blackbox` doesn't expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.blackbox` doesn't expose any component-specific
debug metrics.

## Examples

### Collect metrics using a blackbox exporter configuration file

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.blackbox`. 
It adds an extra label, `env="dev"`, to the metrics emitted by the `grafana` target. The `example` target doesn't have any added labels.

The `config_file` argument is used to define which `blackbox_exporter` modules to use. You can use the [blackbox example config file](https://github.com/prometheus/blackbox_exporter/blob/master/example.yml).

```alloy
prometheus.exporter.blackbox "example" {
  config_file = "blackbox_modules.yml"

  target {
    name    = "example"
    address = "https://example.com"
    module  = "http_2xx"
  }

  target {
    name    = "grafana"
    address = "https://grafana.com"
    module  = "http_2xx"
    labels = {
      "env" = "dev",
    }
  }
}

// Configure a prometheus.scrape component to collect blackbox metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.blackbox.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the `remote_write` API.
- `PASSWORD`: The password to use for authentication to the `remote_write` API.

### Collect metrics using an embedded configuration

This example uses an embedded configuration:

```alloy
prometheus.exporter.blackbox "example" {
  config = "{ modules: { http_2xx: { prober: http, timeout: 5s } } }"

  target {
    name    = "example"
    address = "https://example.com"
    module  = "http_2xx"
  }

  target {
    name    = "grafana"
    address = "https://grafana.com"
    module  = "http_2xx"
    labels = {
      "env" = "dev",
    }
  }
}

// Configure a prometheus.scrape component to collect blackbox metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.blackbox.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

### Collect metrics from a dynamic set of targets

This example is the same as above but the blackbox targets are discovered via a [`discovery.file` component][disc] and sent to the `prometheus.exporter.blackbox`:

```alloy
discovery.file "example" {
  files = ["targets.yml"]
}

prometheus.exporter.blackbox "example" {
  config = "{ modules: { http_2xx: { prober: http, timeout: 5s } } }"
  targets = discovery.file.example.targets
}

prometheus.scrape "example" {
  targets    = prometheus.exporter.blackbox.example.targets
  forward_to = [prometheus.remote_write.example.receiver]
}

prometheus.remote_write "example" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

The YAML file in this example looks like this:
```yaml
- targets:
  - localhost:9009
  labels:
    name: t1
    module: http_2xx
    other_label: example
- targets:
  - localhost:9009
  labels:
    name: t2
    module: http_2xx
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the `remote_write` API.
- `PASSWORD`: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/
[disc]: ../discovery.file/
[relabel]: ../discovery.relabel/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.blackbox` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
