---
canonical: https://grafana.com/docs/alloy/latest/monitor/monitor-linux/
description: Learn how to use Grafana Alloy to monitor Linux servers
menuTitle: Monitor Linux
title: Monitor Linux servers with Grafana Alloy
weight: 225
---

# Monitor Linux servers with {{% param "FULL_PRODUCT_NAME" %}}

The Linux operating system generates a wide range of metrics and logs that you can use to monitor the health and performance of your hardware and operating system.
With {{< param "PRODUCT_NAME" >}}, you can collect your metrics and logs, forward them to a Grafana stack, and create dashboards to monitor your Linux servers.

The [`alloy-scenarios`][scenarios] repository contains complete examples of {{< param "PRODUCT_NAME" >}} deployments.
Clone the repository and use the examples to understand how {{< param "PRODUCT_NAME" >}} collects, processes, and exports telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} collects Linux metrics and forwards them to a Loki destination.

[scenarios]: https://github.com/grafana/alloy-scenarios/

## Before you begin

Ensure you have the following:

* [Docker](https://www.docker.com/)
* [Git](https://git-scm.com/)
* A Linux host or Linux running in a Virtual Machine

{{< admonition type="note" >}}
You need administrator privileges to run `docker` commands.
{{< /admonition >}}

## Clone and deploy the example

Follow these steps to clone the repository and deploy the monitoring example:

1. Clone the {{< param "PRODUCT_NAME" >}} scenarios repository:

   ```shell
   git clone https://github.com/grafana/alloy-scenarios.git
   ```

1. Start Docker to deploy the Grafana stack:

   ```shell
   cd alloy-scenarios/linux
   docker compose up -d
   ```

   Verify the status of the Docker containers:

   ```shell
   docker ps
   ```

1. (Optional) Stop Docker to shut down the Grafana stack when you finish exploring this example:

   ```shell
   docker compose down
   ```

## Monitor and visualize your data

Use Grafana to monitor deployment health and visualize data.

### Monitor {{% param "PRODUCT_NAME" %}}

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and go to [http://localhost:12345](http://localhost:12345).

For more information about the {{< param "PRODUCT_NAME" >}} UI, refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/).

### Visualize your data

To explore metrics, open your browser and go to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).

To use the Grafana Logs Drilldown, open your browser and go to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) for visualizing metrics and logs:

1. Open your browser and go to [http://localhost:3000/dashboards](http://localhost:3000/dashboards).
1. Download the JSON file for the preconfigured [Linux node dashboard](https://grafana.com/api/dashboards/1860/revisions/37/download).
1. Go to **Dashboards** > **Import**
1. Upload the JSON file.
1. Select the Prometheus data source and click **Import**

This community dashboard provides comprehensive system metrics including CPU, memory, disk, and network usage.

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example uses a `config.alloy` file to configure {{< param "PRODUCT_NAME" >}} components for metrics and logging.
You can find this file in the cloned repository at `alloy-scenarios/linux/`.

### Configure metrics

The metrics configuration in this example requires four components:

* `prometheus.exporter.unix`
* `discovery.relabel`
* `prometheus.scrape`
* `prometheus.remote_write`

#### `prometheus.exporter.unix`

The [`prometheus.exporter.unix`][prometheus.exporter.unix] component exposes hardware and Linux kernel metrics.
This is the primary component that you configure to collect your Linux system metrics.

In this example, this component requires the following arguments:

* `disable_collectors`: Disable specific collectors to reduce unnecessary overhead.
* `enable_collectors`: Enable the `meminfo` collector.
* `fs_types_exclude`: A regular expression of filesystem types to ignore.
* `mount_points_exclude`: A regular expression of mount types to ignore.
* `mount_timeout`: How long to wait for a mount to respond before marking it as stale.
* `ignored_devices`: Regular expression of virtual and container network interfaces to ignore.
* `device_exclude`: Regular expression of virtual and container network interfaces to exclude.

```alloy
prometheus.exporter.unix "integrations_node_exporter" {
  disable_collectors = ["ipvs", "btrfs", "infiniband", "xfs", "zfs"]
  enable_collectors = ["meminfo"]

  filesystem {
    fs_types_exclude     = "^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|tmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs)$"
    mount_points_exclude = "^/(dev|proc|run/credentials/.+|sys|var/lib/docker/.+)($|/)"
    mount_timeout        = "5s"
  }

  netclass {
    ignored_devices = "^(veth.*|cali.*|[a-f0-9]{15})$"
  }

  netdev {
    device_exclude = "^(veth.*|cali.*|[a-f0-9]{15})$"
  }
}
```

This component provides the `prometheus.exporter.unix.integrations_node_exporter.output` target for `prometheus.scrape`.

#### `discovery.relabel` instance and job labels

There are two `discovery.relabel` components in this configuration.
This [`discovery.relabel`][discovery.relabel] component replaces the instance and job labels that come in from the `node_exporter` with the hostname of the machine and a standard job name for all metrics.

In this example, this component requires the following arguments:

* `targets`: The targets to relabel.
* `source_labels`: The list of labels to select for relabeling. The rules extract the instance and job labels.
* `replacement`: The value that replaces the source label. The rules set the target labels to `constants.hostname`, and `integrations/node_exporter`.

```alloy
discovery.relabel "integrations_node_exporter" {
  targets = prometheus.exporter.unix.integrations_node_exporter.targets

  rule {
    target_label = "instance"
    replacement  = constants.hostname
  }

  rule {
    target_label = "job"
    replacement = "integrations/node_exporter"
  }
}
```

#### `discovery.relabel` for systemd journal logs

This [`discovery.relabel`][discovery.relabel] component defines the relabeling rules for the systemd journal logs.
In this example, this component requires the following arguments:

* `targets`: The targets to relabel.
  No targets are modified, so the `targets` argument is an empty array.
* `source_labels`: The list of labels to select for relabeling. The rules extract the systemd unit, ID, transport, and log priority.
* `target_label`: The label written to the target. The rules set the target labels to `unit`, `boot_id`, `transport`, and `level`.

```alloy
discovery.relabel "logs_integrations_integrations_node_exporter_journal_scrape" {
  targets = []

  rule {
    source_labels = ["__journal__systemd_unit"]
    target_label  = "unit"
  }

  rule {
    source_labels = ["__journal__boot_id"]
    target_label  = "boot_id"
  }

  rule {
    source_labels = ["__journal__transport"]
    target_label  = "transport"
  }

  rule {
    source_labels = ["__journal_priority_keyword"]
    target_label  = "level"
  }
}
```

#### `prometheus.scrape`

The [`prometheus.scrape`][prometheus.scrape] component scrapes `node_exporter` metrics and forwards them to a receiver.
In this example, the component requires the following arguments:

* `targets`: The target to scrape metrics from. Use the targets with labels from the `discovery.relabel` component.
* `forward_to`: The destination to forward metrics to. Send the scraped metrics to the relabeling component.
* `scrape_interval`: The frequency of scraping the target.

```alloy
prometheus.scrape "integrations_node_exporter" {
  scrape_interval = "15s"
  targets    = discovery.relabel.integrations_node_exporter.output
  forward_to = [prometheus.remote_write.local.receiver]
}
```

#### `prometheus.remote_write`

The [`prometheus.remote_write`][prometheus.remote_write] component sends metrics to a Prometheus server.
In this example, the component requires the following argument:

* `url`: Defines the full URL endpoint to send metrics to.

This component provides the `prometheus.remote_write.local.receiver` destination for `prometheus.scrape`.

```alloy
prometheus.remote_write "local" {
  endpoint {
    url = "http://prometheus:9090/api/v1/write"
  }
}
```

This component provides the `prometheus.remote_write.local.receiver` destination for `prometheus.scrape`.

### Configure logging

The logging configuration in this example requires four components:

* `loki.source.journal`
* `local.file_match`
* `loki.source.file`
* `loki.write`

#### `loki.source.journal`

The [`loki.source.journal`][loki.source.journal] component collects logs from the systemd journal and forwards them to a Loki receiver.
In this example, the component requires the following arguments:

* `max_age`: Only collect logs from the last 24 hours.
* `relabel_rules`: Relabeling rules to apply on log entries.
* `forward_to`: Send logs to the local Loki instance.

```alloy
loki.source.journal "logs_integrations_integrations_node_exporter_journal_scrape" {
  max_age       = "24h0m0s"
  relabel_rules = discovery.relabel.logs_integrations_integrations_node_exporter_journal_scrape.rules
  forward_to    = [loki.write.local.receiver]
}
```

#### `local.file_match`

The [`local.file_match`][local.file_match] component discovers files on the local filesystem using glob patterns.
In this example, the component requires the following arguments:

* `path_targets`: Targets to expand:
  * `__address__`: Targets the localhost for log collection.
  * `__path__`: Collect standard system logs.
  * `instance`: Add an instance label with hostname.
  * `job`: Add a job label for logs.

```alloy
local.file_match "logs_integrations_integrations_node_exporter_direct_scrape" {
  path_targets = [{
    __address__ = "localhost",
    __path__    = "/var/log/{syslog,messages,*.log}",
    instance    = constants.hostname,
    job         = "integrations/node_exporter",
  }]
}
```

#### `loki.source.file`

The [`loki.source.file`][loki.source.file] component reads log entries from files and forwards them to other Loki components.
In this example, the component requires the following arguments:

* `targets`: The list of files to read logs from.
* `forward_to`: The list of receivers to send log entries to.

```alloy
loki.source.file "logs_integrations_integrations_node_exporter_direct_scrape" {
  targets    = local.file_match.logs_integrations_integrations_node_exporter_direct_scrape.targets
  forward_to = [loki.write.local.receiver]
}
```

#### `loki.write`

The [`loki.write`][loki.write] component writes logs to a Loki destination.
In this example, the component requires the following argument:

* `url`: Defines the full URL endpoint in Loki to send logs to.

```alloy
loki.write "local" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

### Configure `livedebugging`

`livedebugging` streams real-time data from your components directly to the {{< param "PRODUCT_NAME" >}} UI.
Refer to the [Troubleshooting documentation][troubleshooting] for more details about this feature.

[troubleshooting]: https://grafana.com/docs/alloy/latest/troubleshoot/debug/#live-debugging-page

#### `livedebugging`

`livedebugging` is disabled by default.
Enable it explicitly through the `livedebugging` configuration block to make debugging data visible in the {{< param "PRODUCT_NAME" >}} UI.
You can use an empty configuration for this block and {{< param "PRODUCT_NAME" >}} uses the default values.

```alloy
livedebugging{}
```

[prometheus.scrape]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.remote_write/
[discovery.relabel]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/discovery/discovery.relabel/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.write/
[loki.source.file]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.file/
[local.file_match]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/local/local.file_match/
[loki.source.journal]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.journal/
[prometheus.exporter.unix]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.exporter.unix/
