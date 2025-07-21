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

| Name                                       | Description                                    | Required |
| ------------------------------------------ | ---------------------------------------------- | -------- |
| [`dfsr`][dfsr]                             | Configures the `dfsr` collector.               | no       |
| [`dns`][dns]                               | Configures the `dns` collector.                | no       |
| [`exchange`][exchange]                     | Configures the `exchange` collector.           | no       |
| [`filetime`][filetime]                     | Configures the `filetime` collector.           | no       |
| [`iis`][iis]                               | Configures the `iis` collector.                | no       |
| [`logical_disk`][logical_disk]             | Configures the `logical_disk` collector.       | no       |
| [`mscluster`][mscluster]                   | Configures the `mscluster` collector.          | no       |
| [`mssql`][mssql]                           | Configures the `mssql` collector.              | no       |
| [`netframework`][netframework]             | Configures the `netframework` collector.       | no       |
| [`network`][network]                       | Configures the `network` collector.            | no       |
| [`performancecounter`][performancecounter] | Configures the `performancecounter` collector. | no       |
| [`physical_disk`][physical_disk]           | Configures the `physical_disk` collector.      | no       |
| [`printer`][printer]                       | Configures the `printer` collector.            | no       |
| [`process`][process]                       | Configures the `process` collector.            | no       |
| [`scheduled_task`][scheduled_task]         | Configures the `scheduled_task` collector.     | no       |
| [`service`][service]                       | Configures the `service` collector.            | no       |
| [`smb_client`][smb_client]                 | Configures the `smb_client` collector.         | no       |
| [`smb`][smb]                               | Configures the `smb` collector.                | no       |
| [`smtp`][smtp]                             | Configures the `smtp` collector.               | no       |
| [`tcp`][tcp]                               | Configures the `tcp` collector.                | no       |
| [`text_file`][text_file]                   | Configures the `text_file` collector.          | no       |
| [`update`][update]                         | Configures the `update` collector.          | no       |

{{< admonition type="note" >}}
Starting with release 1.9.0, the `msmq` block is deprecated.
It will be removed in a future release.
You can still include this block in your configuration files. However, its usage is now a no-op.
{{< /admonition >}}

[dfsr]: #dfsr
[dns]: #dns
[exchange]: #exchange
[filetime]: #filetime
[iis]: #iis
[logical_disk]: #logical_disk
[mscluster]: #mscluster
[mssql]: #mssql
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
[text_file]: #text_file
[tcp]: #tcp
[update]: #update

### `dfsr`

| Name             | Type           | Description                            | Default                            | Required |
| ---------------- | -------------- | -------------------------------------- | ---------------------------------- | -------- |
| `source_enabled` | `list(string)` | A list of DFSR Perflib sources to use. | `["connection","folder","volume"]` | no       |

### `dns`

| Name           | Type           | Description                  | Default                      | Required |
|----------------|----------------|------------------------------|------------------------------|----------|
| `enabled_list` | `list(string)` | A list of collectors to use. | `["metrics", "wmi_stats"]` | no       |

### `exchange`

| Name           | Type           | Description                  | Default       | Required |
| ---------------|----------------|------------------------------|---------------|--------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["ADAccessProcesses", "TransportQueues", "HttpProxy", "ActiveSync", "AvailabilityService", "OutlookWebAccess", "Autodiscover", "WorkloadManagement", "RpcClientAccess", "MapiHttpEmsmdb"]` | no |

### `filetime`

| Name            | Type           | Description                                             | Default | Required |
|-----------------|----------------|---------------------------------------------------------|---------|----------|
| `file_patterns` | `list(string)` | A list of glob patterns matching files to be monitored. | `[]`    | no       |

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

### `mscluster`

| Name           | Type           | Description                  | Default                                                   | Required |
| -------------- | -------------- | ---------------------------- | --------------------------------------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["cluster","network","node","resource","resourcegroup"]` | no       |

The collectors specified by `enabled_list` can include the following:

* `cluster`
* `network`
* `node`
* `resource`
* `resouregroup`

For example, you can set `enabled_list` to `["cluster"]`.

### `mssql`

| Name              | Type           | Description                         | Default                                                                                                                                                              | Required |
| ----------------- | -------------- | ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| `enabled_classes` | `list(string)` | A list of MSSQL WMI classes to use. | `["accessmethods", "availreplica", "bufman", "databases", "dbreplica", "genstats", "info", "locks", "memmgr", "sqlerrors", "sqlstats", "transactions", "waitstats"]` | no       |

### `network`

| Name      | Type     | Description                            | Default  | Required |
| --------- | -------- | -------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of NICs to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of NICs to include. | `"^.+$"` | no       |

NIC names must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `netframework`

| Name           | Type           | Description                  | Default                                                                                                             | Required |
| -------------- | -------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["clrexceptions","clrinterop","clrjit","clrloading","clrlocksandthreads","clrmemory","clrremoting","clrsecurity"]` | no       |

The collectors specified by `enabled_list` can include the following:

* `clrexceptions`
* `clrinterop`
* `clrjit`
* `clrloading`
* `clrlocksandthreads`
* `clrmemory`
* `clrremoting`
* `clrsecurity`

For example, you can set `enabled_list` to `["clrjit"]`.

### `performancecounter`

| Name      | Type     | Description                                       | Default  | Required |
| --------- | -------- | ------------------------------------------------- | -------- | -------- |
| `objects` | `string` | YAML string representing the counters to monitor. | `""`     | no       |

The `objects` field should contain a YAML file as a string that satisfies the schema shown in the exporter's [documentation] for the `performancecounter` collector.
While there are ways to construct this directly in {{< param "PRODUCT_NAME" >}} syntax using [raw {{< param "PRODUCT_NAME" >}} syntax strings][raw-strings] for example, the best way to configure
this collector will be using a `local.file` component.

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

[documentation]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.performancecounter.md
[raw-strings]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#raw-strings

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

| Name                        | Type     | Description                                   | Default  | Required |
|-----------------------------|----------|-----------------------------------------------|----------|----------|
| `exclude`                   | `string` | Regular expression of processes to exclude.   | `"^$"`   | no       |
| `include`                   | `string` | Regular expression of processes to include.   | `"^.+$"` | no       |
| `enable_iis_worker_process` | `string` | Enable IIS collectWorker process name queries | `false`  | no       |

Processes must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

There is a warning in the upstream collector that use of `enable_iis_worker_process` may leak memory. Use with caution.

### `scheduled_task`

| Name      | Type     | Description                             | Default  | Required |
| --------- | -------- | --------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of tasks to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of tasks to include. | `"^.+$"` | no       |

For a task to be included, it must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude`.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `service`

| Name      | Type     | Description                                | Default  | Required |
| --------- | -------- | ------------------------------------------ | -------- | -------- |
| `exclude` | `string` | Regular expression of services to exclude. | `"^$"`   | no       |
| `include` | `string` | Regular expression of services to include. | `"^.+$"` | no       |

For a service to be included, it must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude`.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

{{< admonition type="note" >}}
Starting with release 1.9.0, the `use_api`, `where_clause`, and `enable_v2_collector` attributes are deprecated.
They will be removed in a future release.
You can still include these attributes in your configuration files. However, their usage is now a no-op.
{{< /admonition >}}

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

For example, `enabled_list` may be set to `["ClientShares"]`.

### `smtp`

| Name      | Type     | Description                                       | Default  | Required |
| --------- | -------- | ------------------------------------------------- | -------- | -------- |
| `exclude` | `string` | Regular expression of virtual servers to ignore.  | `"^$"`   | no       |
| `include` | `string` | Regular expression of virtual servers to include. | `"^.+$"` | no       |

For a server name to be included, it must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude`.

User-supplied `exclude` and `include` strings are [wrapped][wrap-regex] in a regular expression.

### `tcp`

| Name           | Type           | Description                  | Default                           | Required |
| -------------- | -------------- | ---------------------------- | --------------------------------- | -------- |
| `enabled_list` | `list(string)` | A list of collectors to use. | `["metrics","connections_state"]` | no       |

The collectors specified by `enabled_list` can include the following:

* `connections_state`
* `metrics`

For example, you can set `enabled_list` to `["metrics"]`.

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

### `update`

| Name              | Type       | Description                                          | Default | Required |
|-------------------|------------|------------------------------------------------------|---------|----------|
| `online`          | `bool`     | Whether to search for updates online.                | `false` | no       |
| `scrape_interval` | `duration` | How frequently to scrape Windows Update information. | `6h`    | no       |

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
| [`filetime`][filetime]                                               | File modification time metrics                                       |                    |
| [`fsrmquota`][fsrmquota]                                             | Microsoft File Server Resource Manager (FSRM) Quotas collector       |                    |
| [`hyperv`][hyperv]                                                   | Hyper-V hosts                                                        |                    |
| [`iis`][iis]                                                         | IIS sites and applications                                           |                    |
| [`logical_disk`][logical_disk]                                       | Logical disks, disk I/O                                              | Yes                |
| [`logon`][logon]                                                     | User logon sessions                                                  |                    |
| [`memory`][memory]                                                   | Memory usage metrics                                                 |                    |
| [`mscluster`][mscluster]                                             | MSCluster metrics                                                    |                    |
| [`msmq`][msmq]                                                       | MSMQ queues                                                          |                    |
| [`mssql`][mssql]                                                     | [SQL Server Performance Objects][sql_server] metrics                 |                    |
| [`netframework`][netframework]                                       | .NET Framework metrics                                               |                    |
| [`net`][net]                                                         | Network interface I/O                                                | Yes                |
| [`os`][os]                                                           | OS metrics (memory, processes, users)                                | Yes                |
| [`pagefile`][pagefile]                                               | Pagefile metrics                                                     |                    |
| [`performancecounter`][performancecounter]                           | Performance Counter metrics                                          |                    |
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
| [`time`][time]                                                       | Windows Time Service                                                 |                    |
| [`thermalzone`][thermalzone]                                         | Thermal information                                                  |                    |
| [`terminal_services`][terminal_services]                             | Terminal services (RDS)                                              |                    |
| [`textfile`][textfile]                                               | Read Prometheus metrics from a text file                             |                    |
| [`udp`][udp]                                                         | UDP connections                                                      |                    |
| [`update`][update]                                                   | Windows Update service metrics                                       |                    |
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
[filetime]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.filetime.md
[fsrmquota]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.fsrmquota.md
[hyperv]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.hyperv.md
[iis]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.iis.md
[logical_disk]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.logical_disk.md
[logon]: https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.logon.md
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
Certain collectors cause {{< param "PRODUCT_NAME" >}} to crash if those collectors are used and the required infrastructure isn't installed.
These include but aren't limited to `mscluster`, `vmware`, `nps`, `dns`, `msmq`, `ad`, `hyperv`, and `scheduled_task`.

The `cs` collector has been deprecated and may be removed in future versions of the exporter.
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
