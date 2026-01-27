---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.windows/
aliases:
  - ../prometheus.exporter.windows/ # /docs/alloy/latest/reference/components/prometheus.exporter.windows/
description: Learn about prometheus.exporter.windows
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.windows
---

# `prometheus.exporter.windows`

The `prometheus.exporter.windows` component embeds the [`windows_exporter`][windows_exporter] which exposes a wide variety of hardware and OS metrics for Windows-based systems.

The `windows_exporter` itself comprises various _collectors_, which you can enable and disable as needed.
For more information on collectors, refer to the [`collectors-list`](#collectors-list) section.

{{< admonition type="note" >}}
The `blacklist` and `whitelist` configuration arguments are deprecated but remain available for backwards compatibility.
Use the `include` and `exclude` arguments instead.
{{< /admonition >}}

[windows_exporter]: https://github.com/prometheus-community/windows_exporter/tree/{{< param "PROM_WIN_EXP_VERSION" >}}

{{< docs/shared lookup="reference/components/exporter-clustering-warning.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Usage

```alloy
prometheus.exporter.windows "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.windows`:

| Name                 | Type           | Description                   | Default                                                | Required |
| -------------------- | -------------- | ----------------------------- | ------------------------------------------------------ | -------- |
| `enabled_collectors` | `list(string)` | List of collectors to enable. | `["cpu","logical_disk","net","os","service","system"]` | no       |

`enabled_collectors` defines a hand-picked list of collectors to enable by default.
If you set this argument, the component disables any collectors not in the list.
Refer to the [Collectors list](#collectors-list) for the default set.

{{< admonition type="caution" >}}
To use any of the configuration blocks below, you must add the corresponding collector name to the `enabled_collectors` list.
For example, to use the `dns` block, you must include `"dns"` in your `enabled_collectors` list.
A block has no effect unless you enable its collector.
{{< /admonition >}}

## Blocks

You can use the following blocks with `prometheus.exporter.windows`.
Each block only takes effect if you include its corresponding collector in `enabled_collectors`.

| Name                                       | Description                                                               | Required |
| ------------------------------------------ | ------------------------------------------------------------------------- | -------- |
| [`dfsr`][dfsr]                             | Configures the `dfsr` collector.                                          | no       |
| [`dns`][dns]                               | Configures the `dns` collector.                                           | no       |
| [`exchange`][exchange]                     | Configures the `exchange` collector.                                      | no       |
| [`filetime`][filetime]                     | Configures the `filetime` collector.                                      | no       |
| [`iis`][iis]                               | Configures the `iis` collector.                                           | no       |
| [`logical_disk`][logical_disk]             | Configures the `logical_disk` collector.                                  | no       |
| [`mscluster`][mscluster]                   | Configures the `mscluster` collector.                                     | no       |
| [`mssql`][mssql]                           | Configures the `mssql` collector.                                         | no       |
| [`netframework`][netframework]             | Configures the `netframework` collector.                                  | no       |
| [`net`][net]                               | Configures the `net` collector.                                           | no       |
| [`network`][network]                       | Configures the `network` collector.                                       | no       |
| [`performancecounter`][performancecounter] | Configures the `performancecounter` collector.                            | no       |
| [`physical_disk`][physical_disk]           | Configures the `physical_disk` collector.                                 | no       |
| [`printer`][printer]                       | Configures the `printer` collector.                                       | no       |
| [`process`][process]                       | Configures the `process` collector.                                       | no       |
| [`scheduled_task`][scheduled_task]         | Configures the `scheduled_task` collector.                                | no       |
| [`service`][service]                       | Configures the `service` collector.                                       | no       |
| [`smb_client`][smb_client]                 | Configures the `smb_client` collector.                                    | no       |
| [`smb`][smb]                               | Configures the `smb` collector.                                           | no       |
| [`smtp`][smtp]                             | Configures the `smtp` collector.                                          | no       |
| [`tcp`][tcp]                               | Configures the `tcp` collector.                                           | no       |
| [`textfile`][textfile]                     | Configures the `textfile` collector.                                      | no       |
| [`text_file`][text_file]                   | (Deprecated: use `textfile` instead) Configures the `textfile` collector. | no       |
| [`update`][update]                         | Configures the `update` collector.                                        | no       |

{{< admonition type="caution" >}}
The `text_file` block is deprecated as of {{< param "PRODUCT_NAME" >}} v1.11.0.
Use the `textfile` block to configure the `textfile` collector.
{{< /admonition >}}

{{< admonition type="note" >}}
The `msmq` block is deprecated as of {{< param "PRODUCT_NAME" >}} v1.9.0.
You can still include this block in your configuration files, but it has no effect.
{{< /admonition >}}

[dfsr]: #dfsr
[dns]: #dns
[exchange]: #exchange
[filetime]: #filetime
[iis]: #iis
[logical_disk]: #logical_disk
[mscluster]: #mscluster
[mssql]: #mssql
[net]: #net
[netframework]: #netframework
[network]: #network
[performancecounter]: #performancecounter
[physical_disk]: #physical_disk
[printer]: #printer
[process]: #process
[scheduled_task]: #scheduled_task
[service]: #service
[smb_client]: #smb_client
[smb]: #smb
[smtp]: #smtp
[textfile]: #textfile
[text_file]: #text_file-deprecated-use-textfile-instead
[tcp]: #tcp
[update]: #update

### `dfsr`

| Name              | Type           | Description                            | Default                            | Required |
| ----------------- | -------------- | -------------------------------------- | ---------------------------------- | -------- |
| `sources_enabled` | `list(string)` | A list of DFSR `Perflib` sources to use. | `["connection","folder","volume"]` | no       |

### `dns`

| Name           | Type           | Description                  | Default                    | Required |
| -------------- | -------------- | ---------------------------- | -------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["metrics", "wmi_stats"]` | no       |

### `exchange`

| Name           | Type           | Description                  | Default                                                                                                                                                                                     | Required |
| -------------- | -------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["ADAccessProcesses", "TransportQueues", "HttpProxy", "ActiveSync", "AvailabilityService", "OutlookWebAccess", "Autodiscover", "WorkloadManagement", "RpcClientAccess", "MapiHttpEmsmdb"]` | no       |

### `filetime`

| Name            | Type           | Description                                             | Default | Required |
| --------------- | -------------- | ------------------------------------------------------- | ------- | -------- |
| `file_patterns` | `list(string)` | A list of glob patterns that match files to monitor.    | `[]`    | no       |

### `iis`

| Name           | Type     | Description                                      | Default  | Required |
| -------------- | -------- | ------------------------------------------------ | -------- | -------- |
| `app_exclude`  | `string` | Regular expression of applications to ignore.    | `"^$"`   | no       |
| `app_include`  | `string` | Regular expression of applications to report on. | `"^.+$"` | no       |
| `site_exclude` | `string` | Regular expression of sites to ignore.           | `"^$"`   | no       |
| `site_include` | `string` | Regular expression of sites to report on.        | `"^.+$"` | no       |

The component [wraps][wrap-regex] user-supplied `app_exclude`, `app_include`, `site_exclude`, and `site_include` strings in a regular expression.

### `logical_disk`

| Name           | Type           | Description                               | Default       | Required |
| -------------- | -------------- | ----------------------------------------- | ------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use.              | `["metrics"]` | no       |
| `exclude`      | `string`       | Regular expression of volumes to exclude. | `"^$"`        | no       |
| `include`      | `string`       | Regular expression of volumes to include. | `"^.+$"`      | no       |

The collectors specified by `enabled_list` can include the following:

- `metrics`
- `bitlocker_status`

The component includes volume names that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

### `mscluster`

| Name           | Type           | Description                  | Default                                                   | Required |
| -------------- | -------------- | ---------------------------- | --------------------------------------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["cluster","network","node","resource","resourcegroup"]` | no       |

The collectors specified by `enabled_list` can include the following:

- `cluster`
- `network`
- `node`
- `resource`
- `resourcegroup`

For example, you can set `enabled_list` to `["cluster"]`.

### `mssql`

| Name              | Type           | Description                         | Default                                                                                                                                                              | Required |
| ----------------- | -------------- | ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| `enabled_classes` | `list(string)` | A list of MSSQL WMI classes to use. | `["accessmethods", "availreplica", "bufman", "databases", "dbreplica", "genstats", "info", "locks", "memmgr", "sqlerrors", "sqlstats", "transactions", "waitstats"]` | no       |

### `net`

| Name           | Type           | Description                            | Default                   | Required |
| -------------- | -------------- | -------------------------------------- | ------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use.           | `["metrics", "nic_info"]` | no       |
| `exclude`      | `string`       | Regular expression of NICs to exclude. | `"^$"`                    | no       |
| `include`      | `string`       | Regular expression of NICs to include. | `"^.+$"`                  | no       |

The collectors specified by `enabled_list` can include the following:

- `metrics`
- `nic_info`

The component includes NIC names that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

### `network`

| Name      | Type     | Description                            | Default  | Required |
| --------- | -------- | -------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of NICs to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of NICs to include. | `"^.+$"` | no       |

The component includes NIC names that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

### `netframework`

| Name           | Type           | Description                  | Default                                                                                                             | Required |
| -------------- | -------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["clrexceptions","clrinterop","clrjit","clrloading","clrlocksandthreads","clrmemory","clrremoting","clrsecurity"]` | no       |

The collectors specified by `enabled_list` can include the following:

- `clrexceptions`
- `clrinterop`
- `clrjit`
- `clrloading`
- `clrlocksandthreads`
- `clrmemory`
- `clrremoting`
- `clrsecurity`

For example, you can set `enabled_list` to `["clrjit"]`.

### `performancecounter`

| Name      | Type     | Description                                       | Default | Required |
| --------- | -------- | ------------------------------------------------- | ------- | -------- |
| `objects` | `string` | YAML string representing the counters to monitor. | `""`    | no       |

The `objects` field accepts a YAML string that satisfies the schema in the exporter's [documentation] for the `performancecounter` collector.
You can construct this directly in {{< param "PRODUCT_NAME" >}} syntax with [raw {{< param "PRODUCT_NAME" >}} syntax strings][raw-strings], but the best way to configure this collector is to use a `local.file` component.

```alloy
local.file "counters" {
  filename = "/etc/alloy/performance_counters.yaml"
}

prometheus.exporter.windows "default" {
  ...

  performancecounter {
    objects = local.file.counters.content
  }

  ...
}
```

The `performance_counters.yaml` file should be a YAML file that represents an array of objects matching the schema in the documentation, like the example below.

```yaml
# Monitor Memory performance counters
- name: memory
  object: "Memory"
  counters:
    - name: "Cache Faults/sec"
      type: "counter"  # Use 'counter' for cumulative/rate metrics
    - name: "Available Bytes"
      type: "gauge"    # Use 'gauge' for point-in-time values

# Monitor Processor performance counters
- name: processor
  object: "Processor"
  instances: ["_Total"]  # Optional: filter to specific instances
  counters:
    - name: "% Processor Time"
      type: "gauge"
    - name: "Interrupts/sec"
      type: "counter"
```

[documentation]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.performancecounter.md
[raw-strings]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#raw-strings

### `physical_disk`

| Name      | Type     | Description                                     | Default  | Required |
| --------- | -------- | ----------------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of physical disk to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of physical disk to include. | `"^.+$"` | no       |

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

### `printer`

| Name      | Type     | Description                               | Default  | Required |
| --------- | -------- | ----------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of printer to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of printer to include. | `"^.+$"` | no       |

The component includes printers that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

### `process`

| Name                        | Type     | Description                                 | Default  | Required |
| --------------------------- | -------- | ------------------------------------------- | -------- | -------- |
| `counter_version`           | `int`    | Version of the process collector to use.    | `0`      | no       |
| `enable_iis_worker_process` | `bool`   | Enable IIS worker process name queries.     | `false`  | no       |
| `exclude`                   | `string` | Regular expression of processes to exclude. | `"^$"`   | no       |
| `include`                   | `string` | Regular expression of processes to include. | `"^.+$"` | no       |

The `counter_version` may be `0`, `1`, or `2`.

- A value of `1` uses the Windows `Process` performance counters through the [registry][] API.
- A value of `2` uses the Windows `Process V2` performance counters through the [Performance Data Helper (PDH)][pdh] API. These counters are available starting in Windows 11.
- A value of `0` checks if `Process V2` counters are available and falls back to `Process` counters if they aren't.

The component includes processes that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

The upstream collector warns that `enable_iis_worker_process` may leak memory. Use with caution.

[pdh]: https://learn.microsoft.com/en-us/windows/win32/perfctrs/collecting-performance-data
[registry]: https://learn.microsoft.com/en-us/windows/win32/perfctrs/using-the-registry-functions-to-consume-counter-data

### `scheduled_task`

| Name      | Type     | Description                             | Default  | Required |
| --------- | -------- | --------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of tasks to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of tasks to include. | `"^.+$"` | no       |

The component includes tasks that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

### `service`

| Name      | Type     | Description                                | Default  | Required |
| --------- | -------- | ------------------------------------------ | -------- | -------- |
| `exclude` | `string` | Regular expression of services to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of services to include. | `"^.+$"` | no       |

The component includes services that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

{{< admonition type="note" >}}
The `use_api`, `where_clause`, and `enable_v2_collector` attributes are deprecated as of {{< param "PRODUCT_NAME" >}} v1.9.0.
You can still include these attributes in your configuration files, but they have no effect.
{{< /admonition >}}

### `smb`

| Name           | Type           | Description                                      | Default | Required |
| -------------- | -------------- | ------------------------------------------------ | ------- | -------- |
| `enabled_list` | `list(string)` | Deprecated (no-op), a list of collectors to use. | `[]`    | no       |

The collectors specified by `enabled_list` can include the following:

- `ServerShares`

For example, you can set `enabled_list` to `["ServerShares"]`.

### `smb_client`

| Name           | Type           | Description                                      | Default | Required |
| -------------- | -------------- | ------------------------------------------------ | ------- | -------- |
| `enabled_list` | `list(string)` | Deprecated (no-op), a list of collectors to use. | `[]`    | no       |

The collectors specified by `enabled_list` can include the following:

- `ClientShares`

For example, you can set `enabled_list` to `["ClientShares"]`.

### `smtp`

| Name      | Type     | Description                                       | Default  | Required |
| --------- | -------- | ------------------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of virtual servers to ignore.  | `"^$"`   | no       |
| `include` | `string` | Regular expression of virtual servers to include. | `"^.+$"` | no       |

The component includes server names that match the regular expression specified by `include` and don't match the regular expression specified by `exclude`.

The component [wraps][wrap-regex] user-supplied `exclude` and `include` strings in a regular expression.

### `tcp`

| Name           | Type           | Description                  | Default                           | Required |
| -------------- | -------------- | ---------------------------- | --------------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["metrics","connections_state"]` | no       |

The collectors specified by `enabled_list` can include the following:

- `connections_state`
- `metrics`

For example, you can set `enabled_list` to `["metrics"]`.

### `textfile`

| Name                  | Type           | Description                                           | Default     | Required |
| --------------------- | -------------- | ----------------------------------------------------- | ----------- | -------- |
| `directories`         | `list(string)` | The list of directories containing files to ingest.   | _see below_ | no       |
| `text_file_directory` | `string`       | Deprecated. The directory containing files to ingest. |             | no       |

By default, `directories` contains the `textfile_inputs` directory in the {{< param "PRODUCT_NAME" >}} installation directory.
For example, if you install {{< param "PRODUCT_NAME" >}} in `C:\Program Files\GrafanaLabs\Alloy\`, the default is `["C:\Program Files\GrafanaLabs\Alloy\textfile_inputs"]`.

The deprecated `text_file_directory` attribute accepts a comma-separated string of directories.
If you set both `text_file_directory` and `directories`, the component combines them into a single list.

For backwards compatibility, you can also use the deprecated `text_file` block to configure the `textfile` collector.
If you configure both blocks, the component combines the distinct directory values from each.

The component only reads files with the `.prom` extension inside the specified directories.

{{< admonition type="note" >}}
The `.prom` files must end with an empty line feed for the component to recognize and read them.
{{< /admonition >}}

### `text_file` (Deprecated: use `textfile` instead)

| Name                  | Type           | Description                                           | Default     | Required |
| --------------------- | -------------- | ----------------------------------------------------- | ----------- | -------- |
| `directories`         | `list(string)` | The list of directories containing files to ingest.   | _see below_ | no       |
| `text_file_directory` | `string`       | Deprecated. The directory containing files to ingest. |             | no       |

By default, `directories` contains the `textfile_inputs` directory in the {{< param "PRODUCT_NAME" >}} installation directory.
For example, if you install {{< param "PRODUCT_NAME" >}} in `C:\Program Files\GrafanaLabs\Alloy\`, the default is `["C:\Program Files\GrafanaLabs\Alloy\textfile_inputs"]`.

The deprecated `text_file_directory` attribute accepts a comma-separated string of directories.
If you set both `text_file_directory` and `directories`, the component combines them into a single list.

For backwards compatibility, you can also use the `textfile` block to configure the `textfile` collector.
If you configure both blocks, the component combines the distinct directory values from each.

The component only reads files with the `.prom` extension inside the specified directories.

{{< admonition type="note" >}}
The `.prom` files must end with an empty line feed for the component to recognize and read them.
{{< /admonition >}}

### `update`

| Name              | Type       | Description                                          | Default | Required |
| ----------------- | ---------- | ---------------------------------------------------- | ------- | -------- |
| `online`          | `bool`     | Whether to search for updates online.                | `false` | no       |
| `scrape_interval` | `duration` | How frequently to scrape Windows Update information. | `6h`    | no       |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.windows` reports as unhealthy only when you provide an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.windows` doesn't expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.windows` doesn't expose any component-specific
debug metrics.

[wrap-regex]: #wrap-regular-expression-strings

## Wrap regular expression strings

Some collector blocks such as [`scheduled_task`][scheduled_task] accept a regular expression as a string argument.
`prometheus.exporter.windows` prefixes some regular expression string arguments with `^(?:` and suffixes them with `)$`.
For example, if a user sets an `exclude` argument to `".*"`, Alloy sets it to `"^(?:.*)$"`.

To find out if the component wraps a particular regular expression argument, refer to the collector block documentation.

{{< admonition type="note" >}}
The wrapping may change the behaviour of your regular expression.
For example, the `e.*` regular expression would normally match both the "service" and "email" strings.
However, `^(?:e.*)$` would only match "email".
{{< /admonition >}}

## Collectors list

The following table lists the available collectors in `windows_exporter`.
Some collectors only work on specific operating systems. If you enable a collector that the host OS doesn't support, it has no effect.

You can enable a subset of collectors to limit the amount of metrics that the `prometheus.exporter.windows` component exposes, or disable collectors that are expensive to run.

| Name                                       | Description                                                    | Enabled by default |
| ------------------------------------------ | -------------------------------------------------------------- | ------------------ |
| [`ad`][ad]                                 | Active Directory Domain Services                               |                    |
| [`adcs`][adcs]                             | Active Directory Certificate Services                          |                    |
| [`adfs`][adfs]                             | Active Directory Federation Services                           |                    |
| [`cache`][cache]                           | Cache metrics                                                  |                    |
| [`cpu`][cpu]                               | CPU usage                                                      | Yes                |
| [`cpu_info`][cpu_info]                     | CPU Information                                                |                    |
| [`container`][container]                   | Container metrics                                              |                    |
| [`dfsr`][dfsr]                             | DFSR metrics                                                   |                    |
| [`dhcp`][dhcp]                             | DHCP Server                                                    |                    |
| [`dns`][dns]                               | DNS Server                                                     |                    |
| [`exchange`][exchange]                     | Exchange metrics                                               |                    |
| [`filetime`][filetime]                     | File modification time metrics                                 |                    |
| [`fsrmquota`][fsrmquota]                   | Microsoft File Server Resource Manager (FSRM) Quotas collector |                    |
| [`gpu`][gpu]                               | GPU usage and memory consumption                               |                    |
| [`hyperv`][hyperv]                         | Hyper-V hosts                                                  |                    |
| [`iis`][iis]                               | IIS sites and applications                                     |                    |
| [`logical_disk`][logical_disk]             | Logical disks, disk I/O                                        | Yes                |
| [`memory`][memory]                         | Memory usage metrics                                           |                    |
| [`mscluster`][mscluster]                   | MSCluster metrics                                              |                    |
| [`msmq`][msmq]                             | MSMQ queues                                                    |                    |
| [`mssql`][mssql]                           | [SQL Server Performance Objects][sql_server] metrics           |                    |
| [`netframework`][netframework]             | .NET Framework metrics                                         |                    |
| [`net`][net]                               | Network interface I/O                                          | Yes                |
| [`os`][os]                                 | OS metrics (memory, processes, users)                          | Yes                |
| [`pagefile`][pagefile]                     | Pagefile metrics                                               |                    |
| [`performancecounter`][performancecounter] | Performance Counter metrics                                    |                    |
| [`physical_disk`][physical_disk]           | Physical disks                                                 |                    |
| [`printer`][printer]                       | Printer metrics                                                |                    |
| [`process`][process]                       | Per-process metrics                                            |                    |
| [`remote_fx`][remote_fx]                   | RemoteFX protocol (RDP) metrics                                |                    |
| [`scheduled_task`][scheduled_task]         | Scheduled Tasks metrics                                        |                    |
| [`service`][service]                       | Service state metrics                                          | Yes                |
| [`smb`][smb]                               | SMB Server shares                                              |                    |
| [`smb_client`][smb_client]                 | SMB Client shares                                              |                    |
| [`smtp`][smtp]                             | IIS SMTP Server                                                |                    |
| [`system`][system]                         | System calls                                                   | Yes                |
| [`tcp`][tcp]                               | TCP connections                                                |                    |
| [`time`][time]                             | Windows Time Service                                           |                    |
| [`thermalzone`][thermalzone]               | Thermal information                                            |                    |
| [`terminal_services`][terminal_services]   | Terminal services (RDS)                                        |                    |
| [`textfile`][textfile]                     | Read Prometheus metrics from a text file                       |                    |
| [`udp`][udp]                               | UDP connections                                                |                    |
| [`update`][update]                         | Windows Update service metrics                                 |                    |
| [`vmware`][vmware]                         | Performance counters installed by the VMware Guest agent       |                    |

[ad]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.ad.md
[adcs]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.adcs.md
[adfs]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.adfs.md
[cache]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cache.md
[cpu]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cpu.md
[cpu_info]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cpu_info.md
[container]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.container.md
[dfsr]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dfsr.md
[dhcp]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dhcp.md
[dns]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dns.md
[exchange]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.exchange.md
[filetime]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.filetime.md
[fsrmquota]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.fsrmquota.md
[gpu]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.gpu.md
[hyperv]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.hyperv.md
[iis]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.iis.md
[logical_disk]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.logical_disk.md
[memory]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.memory.md
[mscluster]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster.md
[msmq]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.msmq.md
[mssql]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mssql.md
[netframework]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework.md
[net]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.net.md
[os]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.os.md
[pagefile]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.pagefile.md
[performancecounter]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.performancecounter.md
[physical_disk]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.physical_disk.md
[printer]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.printer.md
[process]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.process.md
[remote_fx]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.remote_fx.md
[scheduled_task]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.scheduled_task.md
[service]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.service.md
[smb]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.smb.md
[smb_client]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.smbclient.md
[smtp]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.smtp.md
[system]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.system.md
[tcp]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.tcp.md
[time]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.time.md
[thermalzone]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.thermalzone.md
[terminal_services]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.terminal_services.md
[textfile]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.textfile.md
[udp]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.udp.md
[update]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.update.md
[vmware]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.vmware.md
[sql_server]: https://docs.microsoft.com/en-us/sql/relational-databases/performance-monitor/use-sql-server-objects#SQLServerPOs

Refer to the linked documentation on each collector for more information on reported metrics, configuration settings and usage examples.

{{< admonition type="caution" >}}
Certain collectors cause {{< param "PRODUCT_NAME" >}} to crash if you use them without the required infrastructure.
These include but aren't limited to `mscluster`, `VMware`, `nps`, `dns`, `msmq`, `ad`, `hyperv`, and `scheduled_task`.

The `cs` and `logon` collectors are deprecated and removed from the exporter.
You can still configure these collectors, but they have no effect.
{{< /admonition >}}

## Example

The following example uses a [`prometheus.scrape`][scrape] component to collect metrics from `prometheus.exporter.windows`:

```alloy
prometheus.exporter.windows "default" { }

// Configure a prometheus.scrape component to collect windows metrics.
prometheus.scrape "example" {
  targets    = prometheus.exporter.windows.default.targets
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

The following example shows you how to enable additional collectors and configure them:

```alloy
prometheus.exporter.windows "advanced" {
  // Enable additional collectors beyond the default set
  enabled_collectors = [
    "cpu", "logical_disk", "net", "os", "service", "system",  // defaults
    "dns", "iis", "process", "scheduled_task"                 // additional
  ]

  // Configure DNS collector settings
  dns {
    enabled_list = ["metrics", "wmi_stats"]
  }

  // Configure IIS collector settings
  iis {
    site_include = "^(Default Web Site|Production)$"
    app_exclude  = "^$"
  }

  // Configure process collector settings
  process {
    include = "^(chrome|firefox|notepad).*"
    exclude = "^$"
  }
}

prometheus.scrape "advanced_example" {
  targets    = prometheus.exporter.windows.advanced.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.windows` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
