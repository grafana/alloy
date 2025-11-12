---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.ipmi/
aliases:
  - ../prometheus.exporter.ipmi/ # /docs/alloy/latest/reference/components/prometheus.exporter.ipmi/
description: Learn about prometheus.exporter.ipmi
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.ipmi
---

# `prometheus.exporter.ipmi`

The `prometheus.exporter.ipmi` component collects hardware metrics from IPMI-enabled devices.
It supports both local IPMI collection (from the machine running Alloy) and remote IPMI collection from network-accessible devices.

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

The `timeout` should be set high enough to allow IPMI sensor collection to complete.
IPMI operations can be slow, especially for devices with many sensors.
A timeout of 30 seconds or more is recommended for most hardware.

The `config_file` and `ipmi_config` arguments are mutually exclusive.
They allow advanced configuration of IPMI collection modules and command overrides.

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

| Name        | Type     | Description                                   | Default     | Required |
| ----------- | -------- | --------------------------------------------- | ----------- | -------- |
| `name`      | `string` | Name of the target (used in job label).      |             | yes      |
| `address`   | `string` | IP address or hostname of the IPMI device.    |             | yes      |
| `user`      | `string` | IPMI username for authentication.             |             | no       |
| `password`  | `secret` | IPMI password for authentication.             |             | no       |
| `driver`    | `string` | IPMI driver to use (`LAN_2_0` or `LAN`).      | `"LAN_2_0"` | no       |
| `privilege` | `string` | IPMI privilege level (`user` or `admin`).    | `"admin"`   | no       |
| `module`    | `string` | IPMI collector module to use.                 |             | no       |

The `target` block defines a remote IPMI device to monitor.
Multiple `target` blocks can be specified to monitor multiple devices.

The `driver` attribute specifies which IPMI protocol version to use:
- `LAN_2_0`: IPMI 2.0 protocol (recommended for most modern hardware)
- `LAN`: IPMI 1.5 protocol (for older hardware)

The `privilege` attribute sets the privilege level for IPMI operations:
- `admin`: Full administrative access (required for all sensor data on most hardware)
- `user`: User-level access (limited sensor access)

### `local`

| Name      | Type     | Description                              | Default | Required |
| --------- | -------- | ---------------------------------------- | ------- | -------- |
| `enabled` | `bool`   | Enable local IPMI collection.            | `false` | no       |
| `module`  | `string` | IPMI collector module to use.            |         | no       |

The `local` block enables collection from the local machine's IPMI interface.
This requires the machine to have IPMI hardware and appropriate permissions.
On Linux systems, this typically requires:
- FreeIPMI tools installed
- Kernel modules loaded (`ipmi_devintf`, `ipmi_si`)
- Appropriate device permissions (`/dev/ipmi0`)

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

| Metric                      | Type  | Description                                                       |
| --------------------------- | ----- | ----------------------------------------------------------------- |
| `ipmi_up`                   | Gauge | IPMI device reachability (1=up, 0=down)                           |
| `ipmi_temperature_celsius`  | Gauge | Temperature sensor reading in degrees Celsius                     |
| `ipmi_fan_speed_rpm`        | Gauge | Fan speed sensor reading in RPM                                   |
| `ipmi_voltage_volts`        | Gauge | Voltage sensor reading in volts                                   |
| `ipmi_power_watts`          | Gauge | Power sensor reading in watts                                     |
| `ipmi_current_amperes`      | Gauge | Current sensor reading in amperes                                 |
| `ipmi_sensor_state`         | Gauge | Sensor state (0=nominal, 1=warning, 2=critical, 3=not available)  |
| `ipmi_collector_info`       | Gauge | Information about the IPMI collector configuration               |

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

Metrics are available at:
```
http://localhost:12345/api/v0/component/prometheus.exporter.ipmi.test/metrics
```

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

- **LAN_2_0** (IPMI 2.0): Recommended for modern servers (Dell, HP, Supermicro from ~2010+)
- **LAN** (IPMI 1.5): For older hardware that doesn't support IPMI 2.0

If unsure, try `LAN_2_0` first. Connection failures may indicate the need to use `LAN`.

### Local IPMI requirements

For local IPMI collection, ensure:

1. **Hardware support**: Server has BMC/IPMI hardware
2. **Kernel modules** (Linux):
   ```bash
   modprobe ipmi_devintf
   modprobe ipmi_si
   ```
3. **Device permissions**:
   ```bash
   ls -l /dev/ipmi* /dev/ipmi0
   # Should be accessible by the user running Alloy
   ```
4. **FreeIPMI tools** (optional but recommended):
   ```bash
   # Debian/Ubuntu
   apt-get install freeipmi-tools

   # RHEL/CentOS
   yum install freeipmi
   ```

### Security considerations

IPMI credentials provide low-level hardware access:

- **Use dedicated monitoring users** with minimal required privileges
- **Store passwords securely** using `env()` or secrets management
- **Never commit passwords** to version control
- **Consider network isolation** for IPMI interfaces (dedicated management network)
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
