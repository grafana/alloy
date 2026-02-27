---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.smartctl/
description: Learn about prometheus.exporter.smartctl
labels:
  stage: general-availability
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: prometheus.exporter.smartctl
---

# `prometheus.exporter.smartctl`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.exporter.smartctl` component collects SMART disk health metrics from storage devices on the local system.
It uses the `smartctl` from the smartmontools package to query device health, temperature, power-on hours, and other attributes.

## Usage

```alloy
prometheus.exporter.smartctl "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.smartctl`:

| Name                | Type           | Description                                                | Default                | Required |
| ------------------- | -------------- | ---------------------------------------------------------- | ---------------------- | -------- |
| `device_exclude`    | `string`       | Regex pattern to exclude devices from automatic scanning.  | `""`                   | no       |
| `device_include`    | `string`       | Regex pattern to include only matching devices.            | `""`                   | no       |
| `devices`           | `list(string)` | List of specific devices to monitor.                       | `[]`                   | no       |
| `powermode_check`   | `string`       | Power mode threshold to skip checking devices.             | `"standby"`            | no       |
| `rescan_interval`   | `duration`     | How often to rescan for added or removed devices.          | `"10m"`                | no       |
| `scan_device_types` | `list(string)` | Device types to scan (for example, `sat`, `scsi`, `nvme`). | `[]`                   | no       |
| `scan_interval`     | `duration`     | How often to poll smartctl for device data.                | `"60s"`                | no       |
| `smartctl_path`     | `string`       | Path to the smartctl binary.                               | `"/usr/sbin/smartctl"` | no       |

The `smartctl_path` must point to a `smartctl` binary version 7.0 or later with JSON output support.

The `scan_interval` controls how frequently device metrics are collected.
Smartctl queries can be slow, especially with many drives, so a 60-second interval prevents system overload.

The `rescan_interval` controls how often the component rescans for added or removed devices.
This only applies when using automatic device discovery.
Set to `0` to disable automatic rescanning.

The `devices` argument specifies an explicit list of devices to monitor, for example `["/dev/sda", "/dev/nvme0n1"]`.
When specified, the component disables automatic device discovery.

The `device_exclude` and `device_include` arguments are mutually exclusive.
Use them to filter which devices the component monitors during automatic discovery:

- `device_exclude`: Exclude devices matching the regular expression, for example `"^(ram|loop|fd)\\d+$"` excludes RAM disks, loop devices, and floppy drives.
- `device_include`: Only include devices matching the regular expression, for example `"^(sd|nvme)"` includes only SATA and NVMe devices.

The `scan_device_types` argument controls which device types to scan.
Common values include:

- `sat`: SATA devices
- `scsi`: SAS and SCSI devices
- `nvme`: NVMe devices
- `auto`: Auto-detect device type, which is the default when not specified

The `powermode_check` argument determines when to skip checking devices based on their power state to avoid waking sleeping drives.
Valid values are:

- `never`: Always check devices regardless of power state
- `sleep`: Skip devices in sleep mode or deeper
- `standby`: Default, skip devices in standby mode or deeper
- `idle`: Skip devices in idle mode or deeper

## Blocks

The `prometheus.exporter.smartctl` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.smartctl` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

The `smartctl` command execution failures don't affect component health.
Instead, the component sets the `smartctl_device_scrape_success` metric to 0 for devices that fail to scrape.

## Debug information

`prometheus.exporter.smartctl` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.smartctl` doesn't expose any component-specific debug metrics.

## Collected metrics

The `prometheus.exporter.smartctl` component collects the following metrics:

### General metrics

| Metric name                            | Type  | Description                                          |
| -------------------------------------- | ----- | ---------------------------------------------------- |
| `smartctl_version`                     | gauge | `smartctl` version information                       |
| `smartctl_device`                      | gauge | Device information with labels                       |
| `smartctl_device_smart_status`         | gauge | Device SMART overall-health test result (1 = PASSED) |
| `smartctl_device_status`               | gauge | Device status (1 = available, 0 = unavailable)       |
| `smartctl_device_smartctl_exit_status` | gauge | Exit status from `smartctl`                          |

### Capacity and hardware metrics

| Metric name                       | Type  | Description                                   |
| --------------------------------- | ----- | --------------------------------------------- |
| `smartctl_device_capacity_blocks` | gauge | Device capacity in blocks                     |
| `smartctl_device_capacity_bytes`  | gauge | Device capacity in bytes                      |
| `smartctl_device_block_size`      | gauge | Device block size in bytes (logical/physical) |
| `smartctl_device_rotation_rate`   | gauge | Device rotation rate in RPM (0 for SSD)       |
| `smartctl_device_interface_speed` | gauge | Device interface speed (max/current)          |

### Temperature and health metrics

| Metric name                         | Type    | Description                     |
| ----------------------------------- | ------- | ------------------------------- |
| `smartctl_device_temperature`       | gauge   | Device temperature in Celsius   |
| `smartctl_device_power_on_seconds`  | counter | Device power-on time in seconds |
| `smartctl_device_power_cycle_count` | counter | Device power cycle count        |

### Data transfer metrics

| Metric name                     | Type    | Description                   |
| ------------------------------- | ------- | ----------------------------- |
| `smartctl_device_bytes_read`    | counter | Total bytes read from device  |
| `smartctl_device_bytes_written` | counter | Total bytes written to device |

### Error metrics

| Metric name                           | Type    | Description                 |
| ------------------------------------- | ------- | --------------------------- |
| `smartctl_device_media_errors`        | counter | Device media errors         |
| `smartctl_device_num_err_log_entries` | counter | Number of error log entries |

### NVMe-specific metrics

| Metric name                                 | Type    | Description                          |
| ------------------------------------------- | ------- | ------------------------------------ |
| `smartctl_device_percentage_used`           | counter | Percentage of device lifespan used   |
| `smartctl_device_available_spare`           | counter | Available spare capacity percentage  |
| `smartctl_device_available_spare_threshold` | counter | Available spare threshold percentage |
| `smartctl_device_critical_warning`          | counter | Critical warning status              |

### SMART attributes for ATA and SATA only

| Metric name                 | Type  | Description                                                |
| --------------------------- | ----- | ---------------------------------------------------------- |
| `smartctl_device_attribute` | gauge | SMART attribute values: normalized, raw, worst, threshold  |

### Scrape metrics

| Metric name                               | Type  | Description                                     |
| ----------------------------------------- | ----- | ----------------------------------------------- |
| `smartctl_device_scrape_success`          | gauge | Whether the scrape was successful (1 = success) |
| `smartctl_device_scrape_duration_seconds` | gauge | Duration of the `smartctl` scrape in seconds    |

All device metrics include a `device` label with the device path, for example `/dev/sda`.

The `smartctl_device` metric includes comprehensive device information labels:

- `device`: Device path
- `model_name`: Device model name
- `model_family`: Device model family
- `serial_number`: Device serial number
- `firmware_version`: Firmware version
- `interface`: Interface type, for example, SAT or NVMe
- `protocol`: Protocol, for example, ATA or NVMe
- `form_factor`: Form factor, for example, 2.5 inches or M.2
- `ata_version`: ATA version string
- `sata_version`: SATA version string

The `smartctl_device_attribute` metric includes labels:

- `device`: Device path
- `attribute_id`: SMART attribute ID, for example 5 for Reallocated Sectors
- `attribute_name`: SMART attribute name
- `attribute_value_type`: Type of value: normalized, raw, worst, threshold
- `attribute_flags_short`: Short flags string
- `attribute_flags_long`: Long flags string with comma-separated values

## Prerequisites

Before using `prometheus.exporter.smartctl`, ensure you meet the following requirements:

1. Install `smartmontools` version 7.0 or later:

   - Debian/Ubuntu: `sudo apt-get install smartmontools`
   - RHEL/CentOS: `sudo yum install smartmontools`
   - macOS: `brew install smartmontools`

1. Make sure the `smartctl` binary has the appropriate elevated privileges to access device data.
   {{< param "PRODUCT_NAME" >}} must run with one of the following:

   - Root permissions, not recommended for production
   - `CAP_SYS_RAWIO` capability on Linux
   - Appropriate device permissions

3. On Linux only, make sure the appropriate kernel modules are loaded for your devices:
   - SATA/SCSI: Usually enabled by default
   - NVMe: `nvme` kernel module

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.smartctl`:

```alloy
prometheus.exporter.smartctl "example" {
  smartctl_path   = "/usr/sbin/smartctl"
  scan_interval   = "60s"
  rescan_interval = "10m"

  // Exclude common virtual/pseudo devices
  device_exclude = "^(ram|loop|fd)\\d+$"

  // Skip devices in standby to avoid waking them
  powermode_check = "standby"
}

// Configure a prometheus.scrape component to collect smartctl metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.smartctl.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Explicit device list

This example monitors specific devices only:

```alloy
prometheus.exporter.smartctl "specific_devices" {
  smartctl_path = "/usr/sbin/smartctl"

  // Monitor only these devices
  devices = [
    "/dev/sda",
    "/dev/sdb",
    "/dev/nvme0n1",
  ]
}

prometheus.scrape "demo" {
  targets    = prometheus.exporter.smartctl.specific_devices.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}
```

### Device type filtering

This example monitors only NVMe devices:

```alloy
prometheus.exporter.smartctl "nvme_only" {
  smartctl_path = "/usr/sbin/smartctl"

  // Only include NVMe devices
  device_include = "^/dev/nvme"

  // Use NVMe device type
  scan_device_types = ["nvme"]
}

prometheus.scrape "demo" {
  targets    = prometheus.exporter.smartctl.nvme_only.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}
```

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.smartctl` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
