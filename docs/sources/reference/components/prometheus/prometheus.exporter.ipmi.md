---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.ipmi/
description: Learn about prometheus.exporter.ipmi
labels:
  stage: general-availability
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: prometheus.exporter.ipmi
---

# `prometheus.exporter.ipmi`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.exporter.ipmi` component collects hardware metrics from IPMI-enabled devices.
It supports both local IPMI collection from the machine running {{< param "PRODUCT_NAME" >}}, and remote IPMI collection from network-accessible devices.

## Usage

```alloy
prometheus.exporter.ipmi "<LABEL>" {
  target {
    name    = "<NAME>"
    address = "<IPMI_ADDRESS>"
    user    = "<USERNAME>"
    password = "<PASSWORD>"
  }
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.ipmi`:

| Name          | Type       | Description                                           | Default | Required |
| ------------- | ---------- | ----------------------------------------------------- | ------- | -------- |
| `timeout`     | `duration` | Timeout for IPMI requests.                            | `"30s"` | no       |
| `config_file` | `string`   | Path to IPMI exporter configuration file.            |         | no       |
| `ipmi_config` | `string`   | IPMI exporter configuration as inline YAML string.    |         | no       |

Set the `timeout` high enough to allow IPMI sensor collection to complete.
IPMI operations can be slow, especially for devices with many sensors.
A timeout of 30 seconds or more is recommended for most hardware.

The `config_file` and `ipmi_config` arguments are mutually exclusive.
These arguments enable advanced configuration of IPMI collection modules and command overrides.

## Blocks

You can use the following blocks with `prometheus.exporter.ipmi`:

| Name             | Description                          | Required |
| ---------------- | ------------------------------------ | -------- |
| [`target`][target] | Configures a remote IPMI target.     | no       |
| [`local`][local]   | Configures local IPMI collection.    | no       |

At least one `target` block or the `local` block must be configured.

[target]: #target
[local]: #local

### `target`

| Name        | Type     | Description                                | Default     | Required |
| ----------- | -------- | ------------------------------------------ | ----------- | -------- |
| `address`   | `string` | IP address or hostname of the IPMI device. |             | yes      |
| `name`      | `string` | Name of the target used in job label.      |             | yes      |
| `driver`    | `string` | IPMI driver to use, `LAN_2_0` or `LAN`.    | `"LAN_2_0"` | no       |
| `module`    | `string` | IPMI collector module to use.              |             | no       |
| `password`  | `secret` | IPMI password for authentication.          |             | no       |
| `privilege` | `string` | IPMI privilege level, `user` or `admin`.   | `"admin"`   | no       |
| `user`      | `string` | IPMI username for authentication.          |             | no       |

The `target` block defines a remote IPMI device to monitor.
You can specify multiple `target` blocks to monitor multiple devices.

The `driver` attribute specifies which IPMI protocol version to use:

- `LAN_2_0`: IPMI 2.0 protocol, recommended for most modern hardware
- `LAN`: IPMI 1.5 protocol, for legacy hardware

The `privilege` attribute sets the privilege level for IPMI operations:

- `admin`: Full administrative access, required for all sensor data on most hardware
- `user`: User-level access, limited sensor access

### `local`

| Name      | Type     | Description                              | Default | Required |
| --------- | -------- | ---------------------------------------- | ------- | -------- |
| `enabled` | `bool`   | Enable local IPMI collection.            | `false` | no       |
| `module`  | `string` | IPMI collector module to use.            |         | no       |

The `local` block enables collection from the local machine's IPMI interface.
This requires the machine to have IPMI hardware and appropriate permissions.
On Linux systems, this typically requires:

- FreeIPMI tools installed
- Kernel modules for `ipmi_devintf` and `ipmi_si` loaded
- Appropriate device permissions for `/dev/ipmi0`

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.ipmi` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

IPMI connection failures or timeouts don't affect component health.
Instead, the `ipmi_up` metric is set to 0 for unreachable targets.

## Debug information

`prometheus.exporter.ipmi` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.ipmi` doesn't expose any component-specific debug metrics.

## Collected metrics

The IPMI exporter collects the following metrics:

| Metric                     | Type  | Description                                                            |
| -------------------------- | ----- | ---------------------------------------------------------------------- |
| `ipmi_collector_info`      | Gauge | Information about the IPMI collector configuration                     |
| `ipmi_current_amperes`     | Gauge | Current sensor reading in amperes                                      |
| `ipmi_fan_speed_rpm`       | Gauge | Fan speed sensor reading in RPM                                        |
| `ipmi_power_watts`         | Gauge | Power sensor reading in watts                                          |
| `ipmi_sensor_state`        | Gauge | Sensor state `0`=nominal, `1`=warning, `2`=critical, `3`=not available |
| `ipmi_temperature_celsius` | Gauge | Temperature sensor reading in degrees Celsius                          |
| `ipmi_up`                  | Gauge | IPMI device reachability, `1`=up and `0`=down                          |
| `ipmi_voltage_volts`       | Gauge | Voltage sensor reading in volts                                        |

All sensor metrics include labels:

- `target`: IP address or "localhost" for local collection
- `sensor`: Sensor name from IPMI hardware
- `id`: Sensor ID number

## Examples

### Monitor remote IPMI devices

This example monitors two remote servers via IPMI and forwards metrics to Prometheus:

```alloy
prometheus.exporter.ipmi "servers" {
  target {
    name      = "server-01"
    address   = "192.168.1.10"
    user      = "monitoring"
    password  = env("IPMI_PASSWORD")
    driver    = "LAN_2_0"
    privilege = "admin"
  }

  target {
    name      = "server-02"
    address   = "192.168.1.11"
    user      = "monitoring"
    password  = env("IPMI_PASSWORD")
    driver    = "LAN_2_0"
    privilege = "admin"
  }

  timeout = "30s"
}

prometheus.scrape "ipmi" {
  targets         = prometheus.exporter.ipmi.servers.targets
  forward_to      = [prometheus.remote_write.default.receiver]
  scrape_interval = "60s"
  scrape_timeout  = "45s"
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://prometheus:9090/api/v1/write"
  }
}
```

### Monitor local IPMI device

This example monitors the local server's IPMI interface:

```alloy
prometheus.exporter.ipmi "local_server" {
  local {
    enabled = true
    module  = "default"
  }
}

prometheus.scrape "ipmi_local" {
  targets    = prometheus.exporter.ipmi.local_server.targets
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://prometheus:9090/api/v1/write"
  }
}
```

### Monitor both local and remote devices

This example combines local and remote IPMI monitoring:

```alloy
prometheus.exporter.ipmi "infrastructure" {
  // Monitor local machine
  local {
    enabled = true
  }

  // Monitor remote servers
  target {
    name     = "compute-01"
    address  = "10.0.1.10"
    user     = "admin"
    password = env("IPMI_PASSWORD")
  }

  target {
    name     = "compute-02"
    address  = "10.0.1.11"
    user     = "admin"
    password = env("IPMI_PASSWORD")
  }

  timeout = "30s"
}

prometheus.scrape "all_ipmi" {
  targets         = prometheus.exporter.ipmi.infrastructure.targets
  forward_to      = [prometheus.remote_write.default.receiver]
  scrape_interval = "60s"
  scrape_timeout  = "45s"
}

prometheus.remote_write "default" {
  endpoint {
    url = env("PROMETHEUS_URL")
  }
}
```

### Access metrics directly without scraping

For testing or debugging, you can access IPMI metrics directly via HTTP:

```alloy
prometheus.exporter.ipmi "test" {
  target {
    name     = "test-server"
    address  = "192.168.1.100"
    user     = "monitoring"
    password = env("IPMI_PASSWORD")
  }
}
```

Metrics are available at `http://localhost:12345/api/v0/component/prometheus.exporter.ipmi.test/metrics`

## Technical notes

### IPMI timeout configuration

IPMI sensor collection can be slow, especially for servers with many sensors.
Configure timeouts appropriately:

- **Component timeout**: Set to 30-60 seconds to allow sensor collection to complete
- **Scrape timeout**: Must be longer than the component timeout
- **Scrape interval**: Recommended 60 seconds or more to avoid overlapping collections

Example configuration:

```alloy
prometheus.exporter.ipmi "servers" {
  timeout = "30s"  // Allow 30 seconds for IPMI operations
  // ... targets ...
}

prometheus.scrape "ipmi" {
  scrape_interval = "60s"  // Scrape every minute
  scrape_timeout  = "45s"  // Allow 45 seconds (> component timeout)
  targets         = prometheus.exporter.ipmi.servers.targets
  // ... forward_to ...
}
```

### IPMI driver selection

Choose the appropriate IPMI driver based on your hardware:

- **LAN_2_0** (IPMI 2.0): Recommended for modern servers such as Dell, HP, and Supermicro, from approximately onward
- **LAN** (IPMI 1.5): For legacy hardware that doesn't support IPMI 2.0

If unsure, try `LAN_2_0` first. Connection failures may indicate the need to use `LAN`.

### Local IPMI requirements

For local IPMI collection, ensure the following minimum requirements are met:

- **Hardware support**: Server has BMC/IPMI hardware
- **Kernel modules**: `ipmi_devintf` and `ipmi_si`
- **Device permissions**: `/dev/ipmi0` is accessible by the user running {{< param "PRODUCT_NAME" >}}.
- **FreeIPMI tools**: Optional but recommended.

### Security considerations

IPMI credentials provide low-level hardware access:

- **Use dedicated monitoring users** with minimal required privileges
- **Store passwords securely** using `env()` or secrets management
- **Never commit passwords** to version control
- **Consider network isolation** for IPMI interfaces on dedicated management networks
- **Use IPMI 2.0** (`LAN_2_0`) when possible for better security

Example with environment variables:

```alloy
prometheus.exporter.ipmi "secure" {
  target {
    name     = "production-server"
    address  = env("IPMI_HOST")
    user     = env("IPMI_USER")
    password = env("IPMI_PASSWORD")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.ipmi` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
