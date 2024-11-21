---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.ssh/
aliases:
  - ../prometheus.exporter.ssh/ # /docs/alloy/latest/reference/components/prometheus.exporter.ssh/
description: Learn about prometheus.exporter.ssh
title: prometheus.exporter.ssh
---

# prometheus.exporter.ssh

The `prometheus.exporter.ssh` component embeds an SSH exporter for collecting metrics from remote servers over SSH and exporting them as Prometheus metrics.

## Usage

```alloy
prometheus.exporter.ssh "LABEL" {
  // Configuration options
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
All arguments are optional unless specified. Omitted fields take their default values.

| Name              | Type     | Description                                        | Default | Required |
| ----------------- | -------- | -------------------------------------------------- | ------- | -------- |
| `verbose_logging` | `bool`   | Enable verbose logging for debugging purposes.     | `false` | no       |
| `targets`         | `block`  | One or more target configurations for SSH metrics. |         | yes      |

## Blocks

The following blocks are supported inside the definition of `prometheus.exporter.ssh`:

| Block          | Description                                                 | Required |
| -------------- | ----------------------------------------------------------- | -------- |
| `targets`      | Configures an SSH target to collect metrics from.           | yes      |
| `custom_metrics` | Defines custom metrics to collect from the target server. | yes      |

### targets block

The `targets` block defines the remote servers to connect to and the metrics to collect. It supports the following arguments:

| Name              | Type                  | Description                                                            | Default | Required |
| ----------------- | --------------------- | ---------------------------------------------------------------------- | ------- | -------- |
| `address`         | `string`              | The IP address or hostname of the target server.                       |         | yes      |
| `port`            | `int`                 | SSH port number.                                                       | `22`    | no       |
| `username`        | `string`              | SSH username for authentication.                                       |         | yes      |
| `password`        | `secret`              | Password for password-based SSH authentication.                        |         | no       |
| `key_file`        | `string`              | Path to the private key file for key-based SSH authentication.         |         | no       |
| `command_timeout` | `int`                 | Timeout in seconds for each command execution over SSH.                | `30`    | no       |
| `custom_metrics`  | `block`               | One or more custom metrics to collect from the target server.          |         | yes      |

#### Authentication

You must provide either `password` or `key_file` for SSH authentication. If both are provided, `key_file` will be used.

### custom_metrics block

The `custom_metrics` block defines the metrics to collect from the target server. It supports the following arguments:

| Name           | Type                  | Description                                                                  | Default | Required |
| -------------- | --------------------- | ---------------------------------------------------------------------------- | ------- | -------- |
| `name`         | `string`              | The name of the metric.                                                      |         | yes      |
| `command`      | `string`              | The command to execute over SSH to collect the metric.                       |         | yes      |
| `type`         | `string`              | The type of the metric (`gauge` or `counter`).                               |         | yes      |
| `help`         | `string`              | Help text for the metric.                                                    |         | no       |
| `labels`       | `map(string, string)` | Key-value pairs of labels to associate with the metric.                      | `{}`    | no       |
| `parse_regex`  | `string`              | Regular expression to parse the command output and extract the metric value. |         | no       |

#### Metric Types

- `gauge`: Represents a numerical value that can go up or down.
- `counter`: Represents a cumulative value that only increases.

#### parse_regex

If the command output is not a simple numeric value, use `parse_regex` to extract the numeric value from the output.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.ssh` is only reported as unhealthy if given an invalid configuration. In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.ssh` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.ssh` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.ssh`:

```alloy
prometheus.exporter.ssh "example" {
  verbose_logging = true

  targets {
    address         = "192.168.1.10"
    port            = 22
    username        = "admin"
    password        = "password"
    command_timeout = 10

    custom_metrics {
      name    = "load_average"
      command = "cat /proc/loadavg | awk '{print $1}'"
      type    = "gauge"
      help    = "Load average over 1 minute"
    }
  }

  targets {
    address         = "192.168.1.11"
    port            = 22
    username        = "monitor"
    key_file        = "/path/to/private.key"
    command_timeout = 15

    custom_metrics {
      name        = "disk_usage"
      command     = "df / | tail -1 | awk '{print $5}'"
      type        = "gauge"
      help        = "Disk usage percentage"
      parse_regex = "(\\d+)%"
    }
  }
}

// Configure a prometheus.scrape component to collect SSH metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.ssh.example.targets
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

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.ssh` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
