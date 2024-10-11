---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.windows/
aliases:
  - ../prometheus.exporter.windows/ # /docs/alloy/latest/reference/components/prometheus.exporter.windows/
description: Learn about prometheus.exporter.windows
title: prometheus.exporter.windows
---

# prometheus.exporter.windows
The `prometheus.exporter.windows` component embeds
[windows_exporter][] which exposes a
wide variety of hardware and OS metrics for Windows-based systems.

The `windows_exporter` itself comprises various _collectors_, which you can enable and disable as needed.
For more information on collectors, refer to the [`collectors-list`](#collectors-list) section.

{{< admonition type="note" >}}
The `blacklist` and `whitelist` configuration arguments arguments are available for backwards compatibility but are deprecated.
The `include` and `exclude` arguments are preferred going forward.
{{< /admonition >}}

[windows_exporter]: https://github.com/prometheus-community/windows_exporter/tree/{{< param "PROM_WIN_EXP_VERSION" >}}

## Usage

```alloy
prometheus.exporter.windows "LABEL" {
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
All arguments are optional. Omitted fields take their default values.

| Name                 | Type           | Description                   | Default                                                     | Required |
|----------------------|----------------|-------------------------------|-------------------------------------------------------------|----------|
| `enabled_collectors` | `list(string)` | List of collectors to enable. | `["cpu","cs","logical_disk","net","os","service","system"]` | no       |

`enabled_collectors` defines a hand-picked list of enabled-by-default collectors.
If set, anything not provided in that list is disabled by default.
Refer to the [Collectors list](#collectors-list) for the default set.

## Blocks

The following blocks are supported inside the definition of
`prometheus.exporter.windows` to configure collector-specific options:

Hierarchy      | Name               | Description                                       | Required
---------------|--------------------|---------------------------------------------------|---------
dfsr           | [dfsr][]           | Configures the dfsr collector.                    | no
exchange       | [exchange][]       | Configures the exchange collector.                | no
iis            | [iis][]            | Configures the iis collector.                     | no
logical_disk   | [logical_disk][]   | Configures the logical_disk collector.            | no
msmq           | [msmq][]           | Configures the msmq collector.                    | no
mssql          | [mssql][]          | Configures the mssql collector.                   | no
network        | [network][]        | Configures the network collector.                 | no
physical_disk  | [physical_disk][]  | Configures the physical_disk collector.           | no
printer        | [printer][]        | Configures the printer collector.                 | no
process        | [process][]        | Configures the process collector.                 | no
scheduled_task | [scheduled_task][] | Configures the scheduled_task collector.          | no
service        | [service][]        | Configures the service collector.                 | no
smb            | [smb][]            | Configures the smb collector.                     | no
smb_client     | [smb_client][]     | (Deprecated) Configures the smb_client collector. | no
smtp           | [smtp][]           | Configures the smtp collector.                    | no
text_file      | [text_file][]      | Configures the text_file collector.               | no

[dfsr]: #dfsr-block
[exchange]: #exchange-block
[iis]: #iis-block
[logical_disk]: #logical_disk-block
[msmq]: #msmq-block
[mssql]: #mssql-block
[network]: #network-block
[physical_disk]: #physical_disk-block
[printer]: #printer-block
[process]: #process-block
[scheduled_task]: #scheduled_task-block
[service]: #service-block
[smb]: #smb-block
[smb_client]: #smb_client-block
[smtp]: #smtp-block
[text_file]: #textfile-block

### dfsr block

Name             | Type           | Description                            | Default                            | Required
-----------------|----------------|----------------------------------------|------------------------------------|---------
`source_enabled` | `list(string)` | A list of DFSR Perflib sources to use. | `["connection","folder","volume"]` | no


### exchange block

Name           | Type           | Description                  | Default       | Required
---------------|----------------|------------------------------|---------------|---------
`enabled_list` | `list(string)` | A list of collectors to use. | `["ADAccessProcesses", "TransportQueues", "HttpProxy", "ActiveSync", "AvailabilityService", "OutlookWebAccess", "Autodiscover", "WorkloadManagement", "RpcClientAccess", "MapiHttpEmsmdb"]` | no

### iis block

Name           | Type     | Description                                      | Default   | Required
---------------|----------|--------------------------------------------------|-----------|---------
`app_exclude`  | `string` | Regular expression of applications to ignore.    | `"^$"`    | no
`app_include`  | `string` | Regular expression of applications to report on. | `"^.+$"`  | no
`site_exclude` | `string` | Regular expression of sites to ignore.           | `"^$"`    | no
`site_include` | `string` | Regular expression of sites to report on.        | `"^.+$"`  | no


### logical_disk block

Name      | Type     | Description                               | Default   | Required
----------|----------|-------------------------------------------|-----------|---------
`exclude` | `string` | Regular expression of volumes to exclude. | `"^$"`    | no
`include` | `string` | Regular expression of volumes to include. | `"^.+$"`  | no

Volume names must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.


### msmq block

Name           | Type     | Description                                     | Default | Required
---------------|----------|-------------------------------------------------|---------|---------
`where_clause` | `string` | WQL 'where' clause to use in WMI metrics query. | `""`    | no

Specifying `enabled_classes` is useful to limit the response to the MSMQs you specify, reducing the size of the response.


### mssql block

Name | Type     | Description | Default | Required
---- |----------| ----------- | ------- | --------
`enabled_classes` | `list(string)` | A list of MSSQL WMI classes to use. | `["accessmethods", "availreplica", "bufman", "databases", "dbreplica", "genstats", "locks", "memmgr", "sqlstats", "sqlerrors", "transactions", "waitstats"]` | no


### network block

Name      | Type     | Description                             | Default   | Required
----------|----------|-----------------------------------------|-----------|---------
`exclude` | `string` | Regular expression of NIC:s to exclude. | `"^$"`    | no
`include` | `string` | Regular expression of NIC:s to include. | `"^.+$"`  | no

NIC names must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.

### physical_disk block

Name      | Type     | Description                                     | Default   | Required
----------|----------|-------------------------------------------------|-----------|---------
`exclude` | `string` | Regular expression of physical disk to exclude. | `"^$"`    | no
`include` | `string` | Regular expression of physical disk to include. | `"^.+$"`  | no

### printer block

Name      | Type     | Description                               | Default   | Required
----------|----------|-------------------------------------------|-----------|---------
`exclude` | `string` | Regular expression of printer to exclude. | `"^$"`    | no
`include` | `string` | Regular expression of printer to include. | `"^.+$"`  | no

Printer must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.


### process block

Name      | Type     | Description                                 | Default   | Required
----------|----------|---------------------------------------------|-----------|---------
`exclude` | `string` | Regular expression of processes to exclude. | `"^$"`    | no
`include` | `string` | Regular expression of processes to include. | `"^.+$"`  | no

Processes must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude` to be included.


### scheduled_task block

Name      | Type     | Description                 | Default   | Required
----------|----------|-----------------------------|-----------|---------
`exclude` | `string` | Regexp of tasks to exclude. | `"^$"`    | no
`include` | `string` | Regexp of tasks to include. | `"^.+$"`  | no

For a server name to be included, it must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude`.


### service block

Name                  | Type     | Description                                           | Default   | Required
----------------------|----------|-------------------------------------------------------|-----------|---------
`enable_v2_collector` | `string` | Enable V2 service collector.                          | `"false"` | no
`use_api`             | `string` | Use API calls to collect service data instead of WMI. | `"false"` | no
`where_clause`        | `string` | WQL 'where' clause to use in WMI metrics query.       | `""`      | no

The `where_clause` argument can be used to limit the response to the services you specify, reducing the size of the response.
If `use_api` is enabled, 'where_clause' won't be effective.

The v2 collector can query service states much more efficiently, but can't provide general service information.

### smb block

Name           | Type           | Description                  | Default | Required
---------------|----------------|------------------------------|---------|---------
`enabled_list` | `list(string)` | A list of collectors to use. | `[]`    | no

The collectors specified by `enabled_list` can include the following:

- `ServerShares`

For example, `enabled_list` may be set to `["ServerShares"]`.

### smb_client block

Name           | Type           | Description                                      | Default | Required
---------------|----------------|--------------------------------------------------|---------|---------
`enabled_list` | `list(string)` | Deprecated (no-op), a list of collectors to use. | `[]`    | no

The collectors specified by `enabled_list` can include the following:

- `ClientShares`

For example, `enabled_list` may be set to `"ClientShares"`.


### smtp block

Name      | Type     | Description                           | Default   | Required
----------|----------|---------------------------------------|-----------|---------
`exclude` | `string` | Regexp of virtual servers to ignore.  | `"^$"`    | no
`include` | `string` | Regexp of virtual servers to include. | `"^.+$"`  | no

For a server name to be included, it must match the regular expression specified by `include` and must _not_ match the regular expression specified by `exclude`.


### text_file block

Name                  | Type     | Description                                        | Default       | Required
----------------------|----------|----------------------------------------------------|---------------|---------
`text_file_directory` | `string` | The directory containing the files to be ingested. | __see_below__ | no

The default value for `text_file_directory` is relative to the location of the {{< param "PRODUCT_NAME" >}} executable.
By default, `text_file_directory` is set to the `textfile_inputs` directory in the installation directory of {{< param "PRODUCT_NAME" >}}.
For example, if {{< param "PRODUCT_NAME" >}} is installed in `C:\Program Files\GrafanaLabs\Alloy\`,
the default will be `C:\Program Files\GrafanaLabs\Alloy\textfile_inputs`.

When `text_file_directory` is set, only files with the extension `.prom` inside the specified directory are read. Each `.prom` file found must end with an empty line feed to work properly.


## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.windows` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.windows` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.windows` does not expose any component-specific
debug metrics.

## Collectors list
The following table lists the available collectors that `windows_exporter` brings
bundled in. Some collectors only work on specific operating systems; enabling a
collector that is not supported by the host OS where {{< param "PRODUCT_NAME" >}} is running
is a no-op.

Users can choose to enable a subset of collectors to limit the amount of
metrics exposed by the `prometheus.exporter.windows` component,
or disable collectors that are expensive to run.


Name     | Description | Enabled by default
---------|-------------|--------------------
[ad](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.ad.md) | Active Directory Domain Services |
[adcs](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.adcs.md) | Active Directory Certificate Services |
[adfs](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.adfs.md) | Active Directory Federation Services |
[cache](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cache.md) | Cache metrics |
[cpu](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cpu.md) | CPU usage | &#10003;
[cpu_info](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cpu_info.md) | CPU Information |
[cs](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.cs.md) | "Computer System" metrics (system properties, num cpus/total memory) | &#10003;
[container](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.container.md) | Container metrics |
[dfsr](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dfsr.md) | DFSR metrics |
[dhcp](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dhcp.md) | DHCP Server |
[dns](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.dns.md) | DNS Server |
[exchange](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.exchange.md) | Exchange metrics |
[fsrmquota](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.fsrmquota.md) | Microsoft File Server Resource Manager (FSRM) Quotas collector |
[hyperv](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.hyperv.md) | Hyper-V hosts |
[iis](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.iis.md) | IIS sites and applications |
[logical_disk](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.logical_disk.md) | Logical disks, disk I/O | &#10003;
[logon](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.logon.md) | User logon sessions |
[memory](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.memory.md) | Memory usage metrics |
[mscluster_cluster](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_cluster.md) | MSCluster cluster metrics |
[mscluster_network](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_network.md) | MSCluster network metrics |
[mscluster_node](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_node.md) | MSCluster Node metrics |
[mscluster_resource](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_resource.md) | MSCluster Resource metrics |
[mscluster_resourcegroup](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mscluster_resourcegroup.md) | MSCluster ResourceGroup metrics |
[msmq](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.msmq.md) | MSMQ queues |
[mssql](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.mssql.md) | [SQL Server Performance Objects](https://docs.microsoft.com/en-us/sql/relational-databases/performance-monitor/use-sql-server-objects#SQLServerPOs) metrics  |
[netframework_clrexceptions](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrexceptions.md) | .NET Framework CLR Exceptions |
[netframework_clrinterop](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrinterop.md) | .NET Framework Interop Metrics |
[netframework_clrjit](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrjit.md) | .NET Framework JIT metrics |
[netframework_clrloading](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrloading.md) | .NET Framework CLR Loading metrics |
[netframework_clrlocksandthreads](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrlocksandthreads.md) | .NET Framework locks and metrics threads |
[netframework_clrmemory](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrmemory.md) |  .NET Framework Memory metrics |
[netframework_clrremoting](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrremoting.md) | .NET Framework Remoting metrics |
[netframework_clrsecurity](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.netframework_clrsecurity.md) | .NET Framework Security Check metrics |
[net](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.net.md) | Network interface I/O | &#10003;
[os](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.os.md) | OS metrics (memory, processes, users) | &#10003;
[physical_disk](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.physical_disk.md) | Physical disks | &#10003;
[printer](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.printer.md) | Printer metrics |
[process](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.process.md) | Per-process metrics |
[remote_fx](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.remote_fx.md) | RemoteFX protocol (RDP) metrics |
[scheduled_task](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.scheduled_task.md) | Scheduled Tasks metrics |
[service](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.service.md) | Service state metrics | &#10003;
[smb](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.smb.md) | IIS SMTP Server |
[smtp](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.smtp.md) | IIS SMTP Server |
[system](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.system.md) | System calls | &#10003;
[tcp](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.tcp.md) | TCP connections |
[teradici_pcoip](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.teradici_pcoip.md) | [Teradici PCoIP](https://www.teradici.com/web-help/pcoip_wmi_specs/) session metrics |
[time](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.time.md) | Windows Time Service |
[thermalzone](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.thermalzone.md) | Thermal information
[terminal_services](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.terminal_services.md) | Terminal services (RDS)
[textfile](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.textfile.md) | Read prometheus metrics from a text file |
[vmware_blast](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.vmware_blast.md) | VMware Blast session metrics |
[vmware](https://github.com/prometheus-community/windows_exporter/blob/{{< param "PROM_WIN_EXP_VERSION" >}}/docs/collector.vmware.md) | Performance counters installed by the Vmware Guest agent |

Refer to the linked documentation on each collector for more information on reported metrics, configuration settings and usage examples.

{{< admonition type="caution" >}}
Certain collectors will cause {{< param "PRODUCT_NAME" >}} to crash if those collectors are used and the required infrastructure isn't installed.
These include but aren't limited to mscluster_*, vmware, nps, dns, msmq, teradici_pcoip, ad, hyperv, and scheduled_task.
{{< /admonition >}}

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.windows`:

```alloy
prometheus.exporter.windows "default" { }

// Configure a prometheus.scrape component to collect windows metrics.
prometheus.scrape "example" {
  targets    = prometheus.exporter.windows.default.targets
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
  - `USERNAME`: The username to use for authentication to the remote_write API.
  - `PASSWORD`: The password to use for authentication to the remote_write API.

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
