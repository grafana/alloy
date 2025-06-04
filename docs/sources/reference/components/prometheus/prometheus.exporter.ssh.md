---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.ssh/
description: Learn about prometheus.exporter.ssh
labels:
  stage: experimental
  products:
    - oss
title: prometheus.exporter.ssh
---

# `prometheus.exporter.ssh`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.exporter.ssh` component embeds an SSH exporter for collecting metrics from remote servers over SSH and exporting them as Prometheus metrics.

## Usage

```alloy
prometheus.exporter.ssh "example" {
  verbose_logging  = true        // optional: enable debug-level logs
  command_timeout  = "30s"       // optional: timeout for SSH commands

  targets {
    address         = "192.168.1.10"  // required: host or IP
    username        = "admin"         // required: SSH username
    password        = "password"      // required if key_file is unset
    // key_file     = "path/to/key.pem" // alternative to password-based auth

    custom_metrics {
      name    = "load_average"        // required: metric name
      command = "cat /proc/loadavg | awk '{print $1}'"  // required: command
      type    = "gauge"               // required: gauge or counter
      help    = "Load average over 1 minute" // optional help text
    }
  }
}
```

## Arguments

You can use the following argument with `prometheus.exporter.ssh`:

| Name              | Type   | Description                                    | Default | Required |
|-------------------|--------|------------------------------------------------|---------|----------|
| `verbose_logging` | `bool` | Enable verbose logging for debugging purposes. | `false` | no       |

## Blocks

You can use the following blocks with `prometheus.exporter.ssh`:

| Block                                          | Description                                     | Required |
| ---------------------------------------------- | ----------------------------------------------- | -------- |
| [`targets`][targets]                           | Configures SSH targets to collect metrics from. | yes      |
| `targets` > [`custom_metrics`][custom_metrics] | Defines metrics to collect from a server.       | yes      |

The `>` symbol indicates deeper levels of nesting.
For example, `targets` > `custom_metrics` refers to a `custom_metrics` block defined inside a `target` block.

[targets]: #targets
[custom_metrics]: #custom_metrics

### `targets` block

Configures SSH targets to collect metrics from.

| Name              | Type                  | Description                                                            | Default | Required |
|-------------------|-----------------------|------------------------------------------------------------------------|---------|----------|
| `address`         | `string`              | The IP or hostname of the target server.                               |         | yes      |
| `port`            | `int`                 | SSH port number.                                                       | `22`    | no       |
| `username`        | `string`              | SSH username.                                                          |         | yes      |
| `password`        | `secret`              | Password for SSH login.                                                |         | no       |
| `key_file`        | `string`              | Private key file path for key-based auth.                              |         | no       |
| `command_timeout` | `duration`            | Timeout for each SSH command.                                          | `30s`   | no       |
| `custom_metrics`  | `block`               | One or more metrics to collect via SSH.                                |         | yes      |

> Either `password` or `key_file` must be set. If both are provided, `key_file` is used.

### `custom_metrics` block

Defines metrics to collect from a server.

| Name           | Type                  | Description                                                                  | Default | Required |
|----------------|-----------------------|------------------------------------------------------------------------------|---------|----------|
| `name`         | `string`              | Name of the exported metric.                                                |         | yes      |
| `command`      | `string`              | Command to run remotely to get the metric value.                            |         | yes      |
| `type`         | `string`              | Metric type: `gauge` or `counter`.                                          |         | yes      |
| `help`         | `string`              | Help text for the metric.                                                   |         | no       |
| `labels`       | `map(string, string)` | Additional labels to attach to the metric.                                  | `{}`    | no       |
| `parse_regex`  | `string`              | Regex to extract value from command output.                                 |         | no       |

## Example: Curated Targets via Discovery

```alloy
locals {
  ssh_keys = {
    for path in filesystem.glob("${path.module}/keys/*.pem") : basename(path, ".pem") => path
  }
}

prometheus.exporter.ssh "curated" {
  # Iterate only over discovered hosts with a matching key
  for_each = data.discovery.hosts.byLabel("app=backend")

  targets {
    address    = each.value             // host or IP from discovery
    username   = "monitor"             // required: SSH user
    key_file   = ssh_keys[each.value]    // only hosts with key files

    custom_metrics {
      name    = "uptime"               // required: metric name
      command = "cat /proc/uptime | awk '{print $1}'"  // required: command
      type    = "gauge"                // required: gauge or counter
    }
  }
}
```

## Secure Known Hosts Setup

### How It Works

1. **First Run**: If `~/.ssh/known_hosts` is missing, a new one is created using `ssh-keyscan`.
2. **Validation**: Host keys are validated on every connection attempt.
3. **Changes**: Key mismatches raise an error requiring manual review.
4. **New Targets**: Automatically scanned and added, but mismatches block the connection.

### Manual resolution

Use `ssh-keyscan` or another secure method to update `known_hosts` when a host key legitimately changes.

## Security considerations

- Only valid IPs/hostnames accepted.
- Backticks and semicolons are disallowed in commands to prevent injection.
- SSH key files must be `0600`.
- Known-hosts entries are bootstrapped and never overwritten.
- On Windows, host-key checks are skipped.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.ssh` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.ssh` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.ssh` doesn't expose any component-specific debug metrics.

## Example: Prometheus Scrape

```alloy
prometheus.exporter.ssh "example" {
  verbose_logging = true

  targets {
    address         = "192.168.1.10"
    port            = 22
    username        = "admin"
    password        = "password"
    command_timeout = "10s"

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
    command_timeout = "15s"

    custom_metrics {
      name        = "disk_usage"
      command     = "df / | tail -1 | awk '{print $5}'"
      type        = "gauge"
      help        = "Disk usage percentage"
      parse_regex = "(\d+)%"
    }
  }
}

prometheus.scrape "demo" {
  targets    = prometheus.exporter.ssh.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = <PROMETHEUS_REMOTE_WRITE_URL>

    basic_auth {
      username = <USERNAME>
      password = <PASSWORD>
    }
  }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: Remote write-compatible server URL.
* _`<USERNAME>`_: Auth username.
* _`<PASSWORD>`_: Auth password.

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
