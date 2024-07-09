---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.unix/
aliases:
  - ../prometheus.exporter.unix/ # /docs/alloy/latest/reference/components/prometheus.exporter.unix/
description: Learn about prometheus.exporter.unix
title: prometheus.exporter.unix
---

# prometheus.exporter.unix

The `prometheus.exporter.unix` component uses [node_exporter](https://github.com/prometheus/node_exporter) to expose a wide variety of hardware and OS metrics for \*nix-based systems.

The `node_exporter` itself is comprised of various _collectors_, which can be enabled and disabled at will.
For more information on collectors, refer to the [`collectors-list`](#collectors-list) section.


You can specify multiple `prometheus.exporter.unix` components by giving them different labels.

## Usage

```alloy
prometheus.exporter.unix "LABEL" {
}
```

## Arguments

You can use the following arguments to configure the exporter.
All arguments are optional.
Omitted fields take their default values.

| Name                       | Type           | Description                                                                 | Default          | Required |
| -------------------------- | -------------- | --------------------------------------------------------------------------- | ---------------- | -------- |
| `set_collectors`           | `list(string)` | Overrides the default set of enabled collectors with the collectors listed. |                  | no       |
| `enable_collectors`        | `list(string)` | Collectors to mark as enabled.                                              |                  | no       |
| `disable_collectors`       | `list(string)` | Collectors to mark as disabled.                                             |                  | no       |
| `include_exporter_metrics` | `boolean`      | Whether metrics about the exporter itself should be reported.               | false            | no       |
| `procfs_path`              | `string`       | The procfs mount point.                                                     | `/proc`          | no       |
| `sysfs_path`               | `string`       | The sysfs mount point.                                                      | `/sys`           | no       |
| `rootfs_path`              | `string`       | Specify a prefix for accessing the host filesystem.                         | `/`              | no       |
| `udev_data_path`           | `string`       | The udev data path.                                                         | `/run/udev/data` | no       |

`set_collectors` defines a hand-picked list of enabled-by-default collectors.
If set, anything not provided in that list is disabled by default.
Refer to the [Collectors list](#collectors-list) for the default set of enabled collectors for each supported operating system.

`enable_collectors` enables more collectors over the default set, or on top of the ones provided in `set_collectors`.

`disable_collectors` extends the default set of disabled collectors.
If there are conflicts, it takes precedence over `enable_collectors`.

## Blocks

The following blocks are supported inside the definition of `prometheus.exporter.unix` to configure collector-specific options:

| Hierarchy   | Name            | Description                           | Required |
| ----------- | --------------- | ------------------------------------- | -------- |
| bcache      | [bcache][]      | Configures the bcache collector.      | no       |
| cpu         | [cpu][]         | Configures the cpu collector.         | no       |
| disk        | [disk][]        | Configures the diskstats collector.   | no       |
| ethtool     | [ethtool][]     | Configures the ethtool collector.     | no       |
| filesystem  | [filesystem][]  | Configures the filesystem collector.  | no       |
| ipvs        | [ipvs][]        | Configures the ipvs collector.        | no       |
| ntp         | [ntp][]         | Configures the ntp collector.         | no       |
| netclass    | [netclass][]    | Configures the netclass collector.    | no       |
| netdev      | [netdev][]      | Configures the netdev collector.      | no       |
| netstat     | [netstat][]     | Configures the netstat collector.     | no       |
| perf        | [perf][]        | Configures the perf collector.        | no       |
| powersupply | [powersupply][] | Configures the powersupply collector. | no       |
| runit       | [runit][]       | Configures the runit collector.       | no       |
| supervisord | [supervisord][] | Configures the supervisord collector. | no       |
| sysctl      | [sysctl][]      | Configures the sysctl collector.      | no       |
| systemd     | [systemd][]     | Configures the systemd collector.     | no       |
| tapestats   | [tapestats][]   | Configures the tapestats collector.   | no       |
| textfile    | [textfile][]    | Configures the textfile collector.    | no       |
| vmstat      | [vmstat][]      | Configures the vmstat collector.      | no       |

[bcache]: #bcache-block
[cpu]: #cpu-block
[disk]: #disk-block
[ethtool]: #ethtool-block
[filesystem]: #filesystem-block
[ipvs]: #ipvs-block
[ntp]: #ntp-block
[netclass]: #netclass-block
[netdev]: #netdev-block
[netstat]: #netstat-block
[perf]: #perf-block
[powersupply]: #powersupply-block
[runit]: #runit-block
[supervisord]: #supervisord-block
[sysctl]: #sysctl-block
[systemd]: #systemd-block
[tapestats]: #tapestats-block
[textfile]: #textfile-block
[vmstat]: #vmstat-block

### bcache block

| Name             | Type      | Description                                         | Default | Required |
| ---------------- | --------- | --------------------------------------------------- | ------- | -------- |
| `priority_stats` | `boolean` | Enable exposing of expensive bcache priority stats. | false   | no       |

### cpu block

| Name            | Type      | Description                                         | Default | Required |
| --------------- | --------- | --------------------------------------------------- | ------- | -------- |
| `guest`         | `boolean` | Enable the `node_cpu_guest_seconds_total` metric.   | true    | no       |
| `info`          | `boolean` | Enable the `cpu_info metric` for the cpu collector. | true    | no       |
| `bugs_include`  | `string`  | Regexp of `bugs` field in cpu info to filter.       |         | no       |
| `flags_include` | `string`  | Regexp of `flags` field in cpu info to filter.      |         | no       |

### disk block

| Name             | Type     | Description                                                                      | Default                                                        | Required |
| ---------------- | -------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------- | -------- |
| `device_exclude` | `string` | Regexp of devices to exclude for diskstats.                                      | `"^(ram\|loop\|fd\|(h\|s\|v\|xv)d[a-z]\|nvme\\d+n\\d+p)\\d+$"` | no       |
| `device_include` | `string` | Regexp of devices to include for diskstats. If set, `device_exclude` is ignored. |                                                                | no       |

### ethtool block

| Name              | Type     | Description                                                                      | Default | Required |
| ----------------- | -------- | ------------------------------------------------------------------------------- | ------- | -------- |
| `device_exclude`  | `string` | Regexp of ethtool devices to exclude. Mutually exclusive with `device_include`. |         | no       |
| `device_include`  | `string` | Regexp of ethtool devices to include. Mutually exclusive with `device_exclude`. |         | no       |
| `metrics_include` | `string` | Regexp of ethtool stats to include.                                             | `.*`    | no       |

### filesystem block

The default values vary by the operating system {{< param "PRODUCT_NAME" >}} runs on.

| Name                   | Type       | Description                                                         | Default        | Required |
|------------------------|------------|---------------------------------------------------------------------|----------------|----------|
| `fs_types_exclude`     | `string`   | Regexp of filesystem types to ignore for filesystem collector.      | (_see below_ ) | no       |
| `mount_points_exclude` | `string`   | Regexp of mount points to ignore for filesystem collector.          | (_see below_ ) | no       |
| `mount_timeout`        | `duration` | How long to wait for a mount to respond before marking it as stale. | `"5s"`         | no       |

`fs_types_exclude` defaults to the following regular expression string:

{{< code >}}
```linux
^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs)$
```
```osx
^(autofs|devfs)$
```
```bsd
^devfs$
```
{{< /code >}}

`mount_points_exclude` defaults to the following regular expression string:

{{< code >}}
```linux
^/(dev|proc|run/credentials/.+|sys|var/lib/docker/.+)($|/)
```
```osx
^/(dev)($|/)
```
```bsd
^/(dev)($|/)
```
{{< /code >}}

### ipvs block

| Name             | Type           | Description                         | Default                                                                       | Required |
| ---------------- | -------------- | ----------------------------------- | ----------------------------------------------------------------------------- | -------- |
| `backend_labels` | `list(string)` | Array of IPVS backend stats labels. | `[local_address, local_port, remote_address, remote_port, proto, local_mark]` | no       |

### ntp block

| name                     | type       | description                                                   | default       | required |
| ------------------------ | ---------- | ------------------------------------------------------------ | ------------- | -------- |
| `server`                 | `string`   | NTP server to use for the collector.                         | `"127.0.0.1"` | no       |
| `server_is_local`        | `boolean`  | Certifies that the server address isn't a public NTP server. | false         | no       |
| `ip_ttl`                 | `int`      | TTL to use while sending NTP query.                          | 1             | no       |
| `local_offset_tolerance` | `duration` | Offset between local clock and local NTPD time to tolerate.  | `"1ms"`       | no       |
| `max_distance`           | `duration` | Max accumulated distance to the root.                        | `"3466080us"` | no       |
| `protocol_version`       | `int`      | NTP protocol version.                                        | 4             | no       |

### netclass block

| name                          | type      | description                                             | default | required |
| ----------------------------- | --------- | ------------------------------------------------------- | ------- | -------- |
| `ignore_invalid_speed_device` | `boolean` | Ignore net devices with invalid speed values.           | false   | no       |
| `ignored_devices`             | `string`  | Regexp of net devices to ignore for netclass collector. | `"^$"`  | no       |

### netdev block

| name             | type      | description                                                                 | default | required |
| ---------------- | --------- | --------------------------------------------------------------------------- | ------- | -------- |
| `address_info`   | `boolean` | Enable collecting address-info for every device.                            | false   | no       |
| `device_exclude` | `string`  | Regexp of net devices to exclude. Mutually exclusive with `device_include`. |         | no       |
| `device_include` | `string`  | Regexp of net devices to include. Mutually exclusive with `device_exclude`. |         | no       |

### netstat block

| name     | type     | description                                       | default       | required |
| -------- | -------- | ------------------------------------------------- | ------------- | -------- |
| `fields` | `string` | Regexp of fields to return for netstat collector. | _(see below)_ | no       |

`fields` defaults to the following regular expression string:

```
"^(.*_(InErrors|InErrs)|Ip_Forwarding|Ip(6|Ext)_(InOctets|OutOctets)|Icmp6?_(InMsgs|OutMsgs)|TcpExt_(Listen.*|Syncookies.*|TCPSynRetrans|TCPTimeouts)|Tcp_(ActiveOpens|InSegs|OutSegs|OutRsts|PassiveOpens|RetransSegs|CurrEstab)|Udp6?_(InDatagrams|OutDatagrams|NoPorts|RcvbufErrors|SndbufErrors))$"
```

### perf block

| name                         | type           | description                                               | default | required |
| ---------------------------- | -------------- | --------------------------------------------------------- | ------- | -------- |
| `cpus`                       | `string`       | List of CPUs from which perf metrics should be collected. |         | no       |
| `tracepoint`                 | `list(string)` | Array of perf tracepoints that should be collected.       |         | no       |
| `disable_hardware_profilers` | `boolean`      | Disable perf hardware profilers.                          | false   | no       |
| `hardware_profilers`         | `list(string)` | Perf hardware profilers that should be collected.         |         | no       |
| `disable_software_profilers` | `boolean`      | Disable perf software profilers.                          | false   | no       |
| `software_profilers`         | `list(string)` | Perf software profilers that should be collected.         |         | no       |
| `disable_cache_profilers`    | `boolean`      | Disable perf cache profilers.                             | false   | no       |
| `cache_profilers`            | `list(string)` | Perf cache profilers that should be collected.            |         | no       |

### powersupply block

| name               | type     | description                                                            | default | required |
| ------------------ | -------- | ---------------------------------------------------------------------- | ------- | -------- |
| `ignored_supplies` | `string` | Regexp of power supplies to ignore for the powersupplyclass collector. | `"^$"`  | no       |

### runit block

| name          | type     | description                      | default          | required |
| ------------- | -------- | -------------------------------- | ---------------- | -------- |
| `service_dir` | `string` | Path to runit service directory. | `"/etc/service"` | no       |

### supervisord block

| name  | type     | description                                     | default                        | required |
| ----- | -------- | ----------------------------------------------- | ------------------------------ | -------- |
| `url` | `string` | XML RPC endpoint for the supervisord collector. | `"http://localhost:9001/RPC2"` | no       |

Setting `SUPERVISORD_URL` in the environment overrides the default value.
An explicit value in the block takes precedence over the environment variable.

### sysctl block

| name           | type           | description                      | default | required |
| -------------- | -------------- | -------------------------------- | ------- | -------- |
| `include`      | `list(string)` | Numeric sysctl values to expose. | `[]`    | no       |
| `include_info` | `list(string)` | String sysctl values to expose.  | `[]`    | no       |

### systemd block

| name              | type      | description                                                                                              | default                                           | required |
| ----------------- | --------- | -------------------------------------------------------------------------------------------------------- | ------------------------------------------------- | -------- |
| `enable_restarts` | `boolean` | Enables service unit metric `service_restart_total`                                                      | false                                             | no       |
| `start_time`      | `boolean` | Enables service unit metric `unit_start_time_seconds`                                                    | false                                             | no       |
| `task_metrics`    | `boolean` | Enables service unit task metrics `unit_tasks_current` and `unit_tasks_max.`                             | false                                             | no       |
| `unit_exclude`    | `string`  | Regexp of systemd units to exclude. Units must both match include and not match exclude to be collected. | `".+\\.(automount\|device\|mount\|scope\|slice)"` | no       |
| `unit_include`    | `string`  | Regexp of systemd units to include. Units must both match include and not match exclude to be collected. | `".+"`                                            | no       |

### tapestats block

| name              | type     | description                            | default | required |
| ----------------- | -------- | -------------------------------------- | ------- | -------- |
| `ignored_devices` | `string` | Regexp of tapestats devices to ignore. | `"^$"`  | no       |

### textfile block

| name        | type     | description                                                       | default | required |
| ----------- | -------- | ----------------------------------------------------------------- | ------- | -------- |
| `directory` | `string` | Directory to read `*.prom` files from for the textfile collector. |         | no       |

### vmstat block

| name     | type     | description                                          | default                                  | required |
| -------- | -------- | ---------------------------------------------------- | ---------------------------------------- | -------- |
| `fields` | `string` | Regexp of fields to return for the vmstat collector. | `"^(oom_kill\|pgpg\|pswp\|pg.*fault).*"` | no       |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.unix` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.unix` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.unix` does not expose any component-specific
debug metrics.

## Collectors list

The following table lists the available collectors that `node_exporter` brings bundled in.
Some collectors only work on specific operating systems.
Enabling a collector that's not supported by the host operating system where {{< param "PRODUCT_NAME" >}} is running is a no-op.

Users can choose to enable a subset of collectors to limit the amount of metrics exposed by the `prometheus.exporter.unix` component, or disable collectors that are expensive to run.

| Name               | Description                                                                                                   | OS                                                                 | Enabled by default |
|--------------------|---------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------|--------------------|
| `arp`              | Exposes ARP statistics from `/proc/net/arp`.                                                                  | Linux                                                              | yes                |
| `bcache`           | Exposes bcache statistics from `/sys/fs/bcache`.                                                              | Linux                                                              | yes                |
| `bonding`          | Exposes the number of configured and active slaves of Linux bonding interfaces.                               | Linux                                                              | yes                |
| `boottime`         | Exposes system boot time derived from the `kern.boottime sysctl`.                                             | Darwin, Dragonfly, FreeBSD, NetBSD, OpenBSD, Oracle Solaris        | yes                |
| `btrfs`            | Exposes statistics on btrfs.                                                                                  | Linux                                                              | yes                |
| `buddyinfo`        | Exposes statistics of memory fragments as reported by `/proc/buddyinfo`.                                      | Linux                                                              | no                 |
| `conntrack`        | Shows conntrack statistics. Does nothing if no `/proc/sys/net/netfilter/` is present.                         | Linux                                                              | yes                |
| `cpu`              | Exposes CPU statistics.                                                                                       | Darwin, Dragonfly, FreeBSD, Linux, Oracle Solaris, NetBSD          | yes                |
| `cpufreq`          | Exposes CPU frequency statistics.                                                                             | Linux, Oracle Solaris                                              | yes                |
| `devstat`          | Exposes device statistics.                                                                                    | Dragonfly, FreeBSD                                                 | no                 |
| `diskstats`        | Exposes disk I/O statistics.                                                                                  | Darwin, Linux, OpenBSD                                             | yes                |
| `dmi`              | Exposes DMI information.                                                                                      | Linux                                                              | yes                |
| `drbd`             | Exposes Distributed Replicated Block Device statistics (to version 8.4).                                      | Linux                                                              | no                 |
| `drm`              | Exposes GPU card info from `/sys/class/drm/card?/device`.                                                     | Linux                                                              | no                 |
| `edac`             | Exposes error detection and correction statistics.                                                            | Linux                                                              | yes                |
| `entropy`          | Exposes available entropy.                                                                                    | Linux                                                              | yes                |
| `ethtool`          | Exposes ethtool stats.                                                                                        | Linux                                                              | no                 |
| `exec`             | Exposes execution statistics.                                                                                 | Dragonfly, FreeBSD                                                 | yes                |
| `fibrechannel`     | Exposes FibreChannel statistics.                                                                              | Linux                                                              | yes                |
| `filefd`           | Exposes file descriptor statistics from `/proc/sys/fs/file-nr`.                                               | Linux                                                              | yes                |
| `filesystem`       | Exposes filesystem statistics, such as disk space used.                                                       | Darwin, Dragonfly, FreeBSD, Linux, OpenBSD                         | yes                |
| `hwmon`            | Exposes hardware monitoring and sensor data from `/sys/class/hwmon`.                                          | Linux                                                              | yes                |
| `infiniband`       | Exposes network statistics specific to InfiniBand and Intel OmniPath configurations.                          | Linux                                                              | yes                |
| `interrupts`       | Exposes detailed interrupts statistics.                                                                       | Linux, OpenBSD                                                     | no                 |
| `ipvs`             | Exposes IPVS status from `/proc/net/ip_vs` and stats from `/proc/net/ip_vs_stats`.                            | Linux                                                              | yes                |
| `ksmd`             | Exposes kernel and system statistics from `/sys/kernel/mm/ksm`.                                               | Linux                                                              | no                 |
| `lnstat`           | Exposes Linux network cache stats.                                                                            | Linux                                                              | no                 |
| `loadavg`          | Exposes load average.                                                                                         | Darwin, Dragonfly, FreeBSD, Linux, NetBSD, OpenBSD, Oracle Solaris | yes                |
| `logind`           | Exposes session counts from logind.                                                                           | Linux                                                              | no                 |
| `mdadm`            | Exposes statistics about devices in `/proc/mdstat`. Does nothing if no `/proc/mdstat` is present.             | Linux                                                              | yes                |
| `meminfo`          | Exposes memory statistics.                                                                                    | Darwin, Dragonfly, FreeBSD, Linux, OpenBSD, NetBSD                 | yes                |
| `meminfo_numa`     | Exposes memory statistics from `/proc/meminfo_numa`.                                                          | Linux                                                              | no                 |
| `mountstats`       | Exposes filesystem statistics from `/proc/self/mountstats`. Exposes detailed NFS client statistics.           | Linux                                                              | no                 |
| `netclass`         | Exposes network interface info from `/sys/class/net`.                                                         | Linux                                                              | yes                |
| `netdev`           | Exposes network interface statistics such as bytes transferred.                                               | Darwin, Dragonfly, FreeBSD, Linux, OpenBSD                         | yes                |
| `netisr`           | Exposes netisr statistics.                                                                                    | FreeBSD                                                            | yes                |
| `netstat`          | Exposes network statistics from `/proc/net/netstat`. This is the same information as `netstat -s`.            | Linux                                                              | yes                |
| `network_route`    | Exposes network route statistics.                                                                             | Linux                                                              | no                 |
| `nfs`              | Exposes NFS client statistics from `/proc/net/rpc/nfs`. This is the same information as `nfsstat -c`.         | Linux                                                              | yes                |
| `nfsd`             | Exposes NFS kernel server statistics from `/proc/net/rpc/nfsd`. This is the same information as `nfsstat -s`. | Linux                                                              | yes                |
| `ntp`              | Exposes local NTP daemon health to check time.                                                                | any                                                                | no                 |
| `nvme`             | Exposes NVMe statistics.                                                                                      | Linux                                                              | yes                |
| `os`               | Exposes os-release information.                                                                               | Linux                                                              | yes                |
| `perf`             | Exposes perf based metric. **Warning**: Metrics are dependent on kernel configuration and settings.           | Linux                                                              | no                 |
| `powersupplyclass` | Collects information on power supplies.                                                                       | any                                                                | yes                |
| `pressure`         | Exposes pressure stall statistics from `/proc/pressure/`.                                                     | Linux kernel 4.20+ or CONFIG_PSI                                   | yes                |
| `processes`        | Exposes aggregate process statistics from /proc.                                                              | Linux                                                              | no                 |
| `qdisc`            | Exposes queuing discipline statistics.                                                                        | Linux                                                              | no                 |
| `rapl`             | Exposes various statistics from `/sys/class/powercap`.                                                        | Linux                                                              | yes                |
| `runit`            | Exposes service status from runit.                                                                            | any                                                                | no                 |
| `schedstat`        | Exposes task scheduler statistics from `/proc/schedstat`.                                                     | Linux                                                              | yes                |
| `sockstat`         | Exposes various statistics from `/proc/net/sockstat`.                                                         | Linux                                                              | yes                |
| `softirqs`         | Exposes detailed softirq statistics from `/proc/softirqs`.                                                    | Linux                                                              | no                 |
| `softnet`          | Exposes statistics from `/proc/net/softnet_stat`.                                                             | Linux                                                              | yes                |
| `stat`             | Exposes various statistics from `/proc/stat`. This includes boot time, forks and interrupts.                  | Linux                                                              | yes                |
| `supervisord`      | Exposes service status from supervisord.                                                                      | any                                                                | no                 |
| `sysctl`           | Expose sysctl values from `/proc/sys`.                                                                        | Linux                                                              | no                 |
| `systemd`          | Exposes service and system status from systemd.                                                               | Linux                                                              | no                 |
| `tapestats`        | Exposes tape device stats.                                                                                    | Linux                                                              | yes                |
| `tcpstat`          | Exposes TCP connection status information from `/proc/net/tcp` and `/proc/net/tcp6`. **Warning**: The current version has potential performance issues in high load situations. | Linux | no            |
| `textfile`         | Collects metrics from files in a directory matching the filename pattern `*.prom`. The files must use [text-based exposition formats][formats].                                 | any   | yes           |
| `thermal`          | Exposes thermal statistics.                                                                                   | Darwin                                                             | yes                |
| `thermal_zone`     | Exposes thermal zone & cooling device statistics from `/sys/class/thermal`.                                   | Linux                                                              | yes                |
| `time`             | Exposes the current system time.                                                                              | any                                                                | yes                |
| `timex`            | Exposes selected `adjtimex(2)` system call stats.                                                             | Linux                                                              | yes                |
| `udp_queues`       | Exposes UDP total lengths of the `rx_queue` and `tx_queue` from `/proc/net/udp` and `/proc/net/udp6`.         | Linux                                                              | yes                |
| `uname`            | Exposes system information as provided by the uname system call.                                              | Darwin, FreeBSD, Linux, OpenBSD, NetBSD                            | yes                |
| `vmstat`           | Exposes statistics from `/proc/vmstat`.                                                                       | Linux                                                              | yes                |
| `wifi`             | Exposes WiFi device and station statistics.                                                                   | Linux                                                              | no                 |
| `xfs`              | Exposes XFS runtime statistics.                                                                               | Linux kernel 4.4+                                                  | yes                |
| `zfs`              | Exposes ZFS performance statistics.                                                                           | Linux, Oracle Solaris                                              | yes                |
| `zoneinfo`         | Exposes zone stats.                                                                                           | Linux                                                              | no                 |

## Run on Docker/Kubernetes

When running {{< param "PRODUCT_NAME" >}} in a Docker container, you need to bind mount the filesystem, procfs, and sysfs from the host machine, as well as set the corresponding arguments for the component to work.

You may also need to add capabilities such as `SYS_TIME` and make sure that {{< param "PRODUCT_NAME" >}} is running with elevated privileges for some of the collectors to work properly.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.unix`:

```alloy
prometheus.exporter.unix "demo" { }

// Configure a prometheus.scrape component to collect unix metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.unix.demo.targets
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

[formats]: https://prometheus.io/docs/instrumenting/exposition_formats/
[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.unix` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
