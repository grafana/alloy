---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.windows/
aliases:
  - ../prometheus.exporter.windows/ # /docs/alloy/latest/reference/components/prometheus.exporter.windows/
description: Learn about prometheus.exporter.windows
labels:
  stage: general-availability
title: prometheus.exporter.windows
---

# `prometheus.exporter.windows`

The `prometheus.exporter.windows` component embeds the [`windows_exporter`][windows_exporter] which exposes a wide variety of hardware and OS metrics for Windows-based systems.

The `windows_exporter` itself comprises various _collectors_, which you can enable and disable as needed.
For more information on collectors, refer to the [`collectors-list`](#collectors-list) section.

{{< admonition type="note" >}}
The `blacklist` and `whitelist` configuration arguments are available for backwards compatibility but are deprecated.
The `include` and `exclude` arguments are preferred going forward.
{{< /admonition >}}

[windows_exporter]: https://github.com/prometheus-community/windows_exporter/tree/{{< param "PROM_WIN_EXP_VERSION" >}}

## Usage

```alloy
prometheus.exporter.windows "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.windows`:

| Name                 | Type           | Description                   | Default                                                     | Required |
| -------------------- | -------------- | ----------------------------- | ----------------------------------------------------------- | -------- |
| `enabled_collectors` | `list(string)` | List of collectors to enable. | `["cpu","cs","logical_disk","net","os","service","system"]` | no       |

`enabled_collectors` defines a hand-picked list of enabled-by-default collectors.
If set, anything not provided in that list is disabled by default.
Refer to the [Collectors list](#collectors-list) for the default set.

## Blocks

You can use the following blocks with `prometheus.exporter.windows`:

| Name                               | Description                                | Required |
| ---------------------------------- | ------------------------------------------ | -------- |
| [`dfsr`][dfsr]                     | Configures the `dfsr` collector.           | no       |
| [`exchange`][exchange]             | Configures the `exchange` collector.       | no       |
| [`iis`][iis]                       | Configures the `iis` collector.            | no       |
| [`logical_disk`][logical_disk]     | Configures the `logical_disk` collector.   | no       |
| [`msmq`][msmq]                     | Configures the `msmq` collector.           | no       |
| [`mssql`][mssql]                   | Configures the `mssql` collector.          | no       |
| [`network`][network]               | Configures the `network` collector.        | no       |
| [`physical_disk`][physical_disk]   | Configures the `physical_disk` collector.  | no       |
| [`printer`][printer]               | Configures the `printer` collector.        | no       |
| [`process`][process]               | Configures the `process` collector.        | no       |
| [`scheduled_task`][scheduled_task] | Configures the `scheduled_task` collector. | no       |
| [`service`][service]               | Configures the `service` collector.        | no       |
| [`smb_client`][smb_client]         | Configures the `smb_client` collector.     | no       |
| [`smb`][smb]                       | Configures the `smb` collector.            | no       |
| [`smtp`][smtp]                     | Configures the `smtp` collector.           | no       |
| [`text_file`][text_file]           | Configures the `text_file` collector.      | no       |

[dfsr]: #dfsr
[exchange]: #exchange
[iis]: #iis
[logical_disk]: #logical_disk
[msmq]: #msmq
[mssql]: #mssql
[network]: #network
[physical_disk]: #physical_disk
[printer]: #printer
[process]: #process
[scheduled_task]: #scheduled_task
[service]: #service
[smb_client]: #smb_client
[smb]: #smb
[smtp]: #smtp
[text_file]: #text_file

### `dfsr`

| Name             | Type           | Description                            | Default                            | Required |
| ---------------- | -------------- | -------------------------------------- | ---------------------------------- | -------- |
| `source_enabled` | `list(string)` | A list of DFSR Perflib sources to use. | `["connection","folder","volume"]` | no       |

### `exchange`

| Name           | Type           | Description                  | Default       | Required |
| ---------------|----------------|------------------------------|---------------|--------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["ADAccessProcesses", "TransportQueues", "HttpProxy", "ActiveSync", "AvailabilityService", "OutlookWebAccess", "Autodiscover", "WorkloadManagement", "RpcClientAccess", "MapiHttpEmsmdb"]` | no |

### `iis`

| Name           | Type     | Description                                      | Default  | Required |
| -------------- | -------- | ------------------------------------------------ | -------- | -------- |
| `app_exclude`  | `string` | Regular expression of applications to ignore.    | `"^$"`   | no       |
| `app_include`  | `string` | Regular expression of applications to report on. | `"^.+$"` | no       |
| `site_exclude` | `string` | Regular expression of sites to ignore.           | `"^$"`   | no       |
| `site_include` | `string` | Regular expression of sites to report on.        | `"^.+$"` | no       |

User-supplied `app_exclude`, `app_include`, `site_exclude` and `site_include` strings are [wrapped][wrap-regex] in a regular expression.

### `logical_disk`

| Name      | Type     | Description                               | Default  | Required |
| --------- | -------- | ----------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of volumes to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of volumes to include. | `"^.+$"` | no       |

Volume names must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `msmq`

| Name           | Type     | Description                                     | Default | Required |
| -------------- | -------- | ----------------------------------------------- | ------- | -------- |
| `where_clause` | `string` | WQL 'where' clause to use in WMI metrics query. | `""`    | no       |

Specifying `enabled_classes` is useful to limit the response to the MSMQs you specify, reducing the size of the response.

### `mssql`

| Name              | Type           | Description                         | Default | Required |
| ----------------- | -------------- | ----------------------------------- | ------- | -------- |
| `enabled_classes` | `list(string)` | A list of MSSQL WMI classes to use. | `["accessmethods", "availreplica", "bufman", "databases", "dbreplica", "genstats", "locks", "memmgr", "sqlstats", "sqlerrors", "transactions", "waitstats"]` | no       |

### `network`

| Name      | Type     | Description                            | Default  | Required |
| --------- | -------- | -------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of NICs to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of NICs to include. | `"^.+$"` | no       |

NIC names must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `physical_disk`

| Name      | Type     | Description                                     | Default  | Required |
| --------- | -------- | ----------------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of physical disk to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of physical disk to include. | `"^.+$"` | no       |

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `printer`

| Name      | Type     | Description                               | Default  | Required |
| --------- | -------- | ----------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of printer to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of printer to include. | `"^.+$"` | no       |

Printer must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `process`

| Name      | Type     | Description                                 | Default  | Required |
| --------- | -------- | ------------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of processes to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of processes to include. | `"^.+$"` | no       |

Processes must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `scheduled_task`

| Name      | Type     | Description                             | Default  | Required |
| --------- | -------- | --------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of tasks to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of tasks to include. | `"^.+$"` | no       |

For a server name to be included, it must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude`.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `service`

| Name                  | Type     | Description                                           | Default   | Required |
| --------------------- | -------- | ----------------------------------------------------- | --------- | -------- |
| `enable_v2_collector` | `string` | Enable V2 service collector.                          | `"false"` | no       |
| `use_api`             | `string` | Use API calls to collect service data instead of WMI. | `"false"` | no       |
| `where_clause`        | `string` | WQL 'where' clause to use in WMI metrics query.       | `""`      | no       |

The `where_clause` argument can be used to limit the response to the services you specify, reducing the size of the response.
If `use_api` is enabled, `where_clause` won't be effective.

The v2 collector can query service states much more efficiently, but can't provide general service information.

### `smb`

| Name           | Type           | Description                                      | Default | Required |
| -------------- | -------------- | ------------------------------------------------ | ------- | -------- |
| `enabled_list` | `list(string)` | Deprecated (no-op), a list of collectors to use. | `[]`    | no       |

The collectors specified by `enabled_list` can include the following:

* `ServerShares`

For example, `enabled_list` may be set to `["ServerShares"]`.

### `smb_client`

| Name           | Type           | Description                                      | Default | Required |
| -------------- | -------------- | ------------------------------------------------ | ------- | -------- |
| `enabled_list` | `list(string)` | Deprecated (no-op), a list of collectors to use. | `[]`    | no       |

The collectors specified by `enabled_list` can include the following:

* `ClientShares`

For example, `enabled_list` may be set to `"ClientShares"`.

### `smtp`

| Name      | Type     | Description                                       | Default  | Required |
| --------- | -------- | ------------------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of virtual servers to ignore.  | `"^$"`   | no       |
| `include` | `string` | Regular expression of virtual servers to include. | `"^.+$"` | no       |

For a server name to be included, it must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude`.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `text_file`

| Name                  | Type     | Description                                        | Default       | Required |
| --------------------- | -------- | -------------------------------------------------- | ------------- | -------- |
| `text_file_directory` | `string` | The directory containing the files to be ingested. | __see below__ | no       |

The default value for `text_file_directory` is relative to the location of the {{< param "PRODUCT_NAME" >}} executable.
By default, `text_file_directory` is set to the `textfile_inputs` directory in the installation directory of {{< param "PRODUCT_NAME" >}}.
For example, if {{< param "PRODUCT_NAME" >}} is installed in `C:\Program Files\GrafanaLabs\Alloy\`, the default is `C:\Program Files\GrafanaLabs\Alloy\textfile_inputs`.

When `text_file_directory` is set, only files with the extension `.prom` inside the specified directory are read.

{{< admonition type="note" >}}
The `.prom` files must end with an empty line feed for the component to recognize and read them.
{{< /admonition >}}

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.windows` is only reported as unhealthy if given an invalid configuration.
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

To find out if a particular regular expression argument will be wrapped, refer to the collector block documentation.

{{< admonition type="note" >}}
The wrapping may change the behaviour of your regular expression.
For example, the `e.*` regular expression would normally match both the "service" and "email" strings.
However, `^(?:e.*)$` would only match "email".
{{< /admonition >}}

## Collectors list

The following table lists the available collectors in `windows_exporter`.
Some collectors only work on specific operating systems, enabling a collector that's not supported by the host OS where {{< param "PRODUCT_NAME" >}} is running is a no-op.

Users can choose to enable a subset of collectors to limit the amount of metrics exposed by the `prometheus.exporter.windows` component, or disable collectors that are expensive to run.

| Name                                                                 | Description                                                          | Enabled by default |
| -------------------------------------------------------------------- | -------------------------------------------------------------------- | ------------------ |
| [`ad`][ad]                                                           | Active Directory Domain Services                                     |                    |
| [`adcs`][adcs]                                                       | Active Directory Certificate Services                                |                    |
| [`adfs`][adfs]                                                       | Active Directory Federation Services                                 |                    |
| [`cache`][cache]                                                     | Cache metrics                                                        |                    |
| [`cpu`][cpu]                                                         | CPU usage                                                            | Yes                |
| [`cpu_info`][cpu_info]                                               | CPU Information                                                      |                    |
| [`cs`][cs]                                                           | "Computer System" metrics (system properties, num cpus/total memory) | Yes                |
| [`container`][container]                                             | Container metrics                                                    |                    |
| [`dfsr`][dfsr]                                                       | DFSR metrics                                                         |                    |
| [`dhcp`][dhcp]                                                       | DHCP Server                                                          |                    |
| [`dns`][dns]                                                         | DNS Server                                                           |                    |
| [`exchange`][exchange]                                               | Exchange metrics                                                     |                    |
| [`fsrmquota`][fsrmquota]                                             | Microsoft File Server Resource Manager (FSRM) Quotas collector       |                    |
| [`hyperv`][hyperv]                                                   | Hyper-V hosts                                                        |                    |
| [`iis`][iis]                                                         | IIS sites and applications                                           |                    |
| [`logical_disk`][logical_disk]                                       | Logical disks, disk I/O                                              | Yes                |
| [`logon`][logon]                                                     | User logon sessions                                                  |                    |
| [`memory`][memory]                                                   | Memory usage metrics                                                 |                    |
| [`mscluster_cluster`][mscluster_cluster]                             | MSCluster cluster metrics                                            |                    |
| [`mscluster_network`][mscluster_network]                             | MSCluster network metrics                                            |                    |
| [`mscluster_node`][mscluster_node]                                   | MSCluster Node metrics                                               |                    |
| [`mscluster_resource`][mscluster_resource]                           | MSCluster Resource metrics                                           |                    |
| [`mscluster_resourcegroup`][mscluster_resourcegroup]                 | MSCluster ResourceGroup metrics                                      |                    |
| [`msmq`][msmq]                                                       | MSMQ queues                                                          |                    |
| [`mssql`][mssql]                                                     | [SQL Server Performance Objects][sql_server] metrics                 |                    |
| [`netframework_clrexceptions`][netframework_clrexceptions]           | .NET Framework CLR Exceptions                                        |                    |
| [`netframework_clrinterop`][netframework_clrinterop]                 | .NET Framework Interop Metrics                                       |                    |
| [`netframework_clrjit`][netframework_clrjit]                         | .NET Framework JIT metrics                                           |                    |
| [`netframework_clrloading`][netframework_clrloading]                 | .NET Framework CLR Loading metrics                                   |                    |
| [`netframework_clrlocksandthreads`][netframework_clrlocksandthreads] | .NET Framework locks and metrics threads                             |                    |
| [`netframework_clrmemory`][netframework_clrmemory]                   | .NET Framework Memory metrics                                        |                    |
| [`netframework_clrremoting`][netframework_clrremoting]               | .NET Framework Remoting metrics                                      |                    |
| [`netframework_clrsecurity`][netframework_clrsecurity]               | .NET Framework Security Check metrics                                |                    |
| [`net`][net]                                                         | Network interface I/O                                                | Yes                |
| [`os`][os]                                                           | OS metrics (memory, processes, users)                                | Yes                |
| [`physical_disk`][physical_disk]                                     | Physical disks                                                       | Yes                |
| [`printer`][printer]                                                 | Printer metrics                                                      |                    |
| [`process`][process]                                                 | Per-process metrics                                                  |                    |
| [`remote_fx`][remote_fx]                                             | RemoteFX protocol (RDP) metrics                                      |                    |
| [`scheduled_task`][scheduled_task]                                   | Scheduled Tasks metrics                                              |                    |
| [`service`][service]                                                 | Service state metrics                                                | Yes                |
| [`smb`][smb]                                                         | IIS SMTP Server                                                      |                    |
| [`smb_client`][smb_client]                                           | IIS SMTP Server                                                      |                    |
| [`smtp`][smtp]                                                       | IIS SMTP Server                                                      |                    |
| [`system`][system]                                                   | System calls                                                         | Yes                |
| [`tcp`][tcp]                                                         | TCP connections                                                      |                    |
| [`teradici_pcoip`][teradici_pcoip]                                   | [Teradici PCoIP][Teradici PCoIP] session metrics                     |                    |
| [`time`][time]                                                       | Windows Time Service                                                 |                    |
| [`thermalzone`][thermalzone]                                         | Thermal information                                                  |                    |
| [`terminal_services`][terminal_services]                             | Terminal services (RDS)                                              |                    |
| [`textfile`][textfile]                                               | Read Prometheus metrics from a text file                             |                    |
| [`vmware_blast`][vmware_blast]                                       | VMware Blast session metrics                                         |                    |
| [`vmware`][vmware]                                                   | Performance counters installed by the VMware Guest agent             |                    |

[ad]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.ad.md
[adcs]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.adcs.md
[adfs]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.adfs.md
[cache]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cache.md
[cpu]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cpu.md
[cpu_info]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cpu_info.md
[cs]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cs.md
[container]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.container.md
[dfsr]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dfsr.md
[dhcp]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dhcp.md
[dns]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dns.md
[exchange]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.exchange.md
[fsrmquota]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.fsrmquota.md
[hyperv]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.hyperv.md
[iis]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.iis.md
[logical_disk]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.logical_disk.md
[logon]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.logon.md
[memory]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.memory.md
[mscluster_cluster]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_cluster.md
[mscluster_network]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_network.md
[mscluster_node]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_node.md
[mscluster_resource]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_resource.md
[mscluster_resourcegroup]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_resourcegroup.md
[msmq]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.msmq.md
[mssql]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mssql.md
[netframework_clrexceptions]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrexceptions.md
[netframework_clrinterop]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrinterop.md
[netframework_clrjit]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrjit.md
[netframework_clrloading]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrloading.md
[netframework_clrlocksandthreads]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrlocksandthreads.md
[netframework_clrmemory]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrmemory.md
[netframework_clrremoting]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrremoting.md
[netframework_clrsecurity]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrsecurity.md
[net]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.net.md
[os]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.os.md
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
[teradici_pcoip]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.teradici_pcoip.md
[time]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.time.md
[thermalzone]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.thermalzone.md
[terminal_services]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.terminal_services.md
[textfile]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.textfile.md
[vmware_blast]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.vmware_blast.md
[vmware]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.vmware.md
[sql_server]: https://docs.microsoft.com/en-us/sql/relational-databases/performance-monitor/use-sql-server-objects#SQLServerPOs
[Teradici PCoIP]: https://www.teradici.com/web-help/pcoip_wmi_specs/

Refer to the linked documentation on each collector for more information on reported metrics, configuration settings and usage examples.

{{< admonition type="caution" >}}
Certain collectors cause {{< param "PRODUCT_NAME" >}} to crash if those collectors are used and the required infrastructure isn't installed.
These include but aren't limited to `mscluster_*`, `vmware`, `nps`, `dns`, `msmq`, `teradici_pcoip`, `ad`, `hyperv`, and `scheduled_task`.
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

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

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
