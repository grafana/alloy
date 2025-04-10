---
canonical: https://grafana.com/docs/alloy/latest/monitor/monitor-linux/
description: Learn how to use Grafana Alloy to monitor Linux servers
menuTitle: Monitor Linux
title: Monitor Linux servers with Grafana Alloy
weight: 225
---

# Monitor Linux servers with {{% param "FULL_PRODUCT_NAME" %}}

The Prometheus Node Exporter exposes hardware and Linux kernel metrics.
With {{< param "PRODUCT_NAME" >}}, you can collect your metrics, forward them to a Grafana stack, and create dashboards to monitor your Docker containers.

The [`alloy-scenarios`][scenarios] repository contains complete examples of {{< param "PRODUCT_NAME" >}} deployments.
Clone the repository and use the examples to understand how {{< param "PRODUCT_NAME" >}} collects, processes, and exports telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} collects Linux metrics and forwards them to a Loki destination.

[scenarios]: https://github.com/grafana/alloy-scenarios/

## Before you begin

Ensure you have the following:

* [Docker](https://www.docker.com/)
* [Git](https://git-scm.com/)
* A Linux host or Linux running in e Virtual Machine

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

The metrics configuration in this example uses nine components:

* `discovery.relabel`
* `prometheus.exporter.unix`
* `prometheus.scrape`
* `prometheus.remote_write`
* `loki.source.journal`
* `local.file_match`
* `loki.source.file`
* `loki.write`
* `livedebugging`

#### `discovery.relabel`

```alloy
// This block relabels metrics coming from node_exporter to add standard labels
discovery.relabel "integrations_node_exporter" {
  targets = prometheus.exporter.unix.integrations_node_exporter.targets

  rule {
    // Set the instance label to the hostname of the machine
    target_label = "instance"
    replacement  = constants.hostname
  }

  rule {
    // Set a standard job name for all node_exporter metrics
    target_label = "job"
    replacement = "integrations/node_exporter"
  }
}
```

#### `prometheus.exporter.unix`

```alloy
// Configure the node_exporter integration to collect system metrics
prometheus.exporter.unix "integrations_node_exporter" {
  // Disable unnecessary collectors to reduce overhead
  disable_collectors = ["ipvs", "btrfs", "infiniband", "xfs", "zfs"]
  enable_collectors = ["meminfo"]

  filesystem {
    // Exclude filesystem types that aren't relevant for monitoring
    fs_types_exclude     = "^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|tmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs)$"
    // Exclude mount points that aren't relevant for monitoring
    mount_points_exclude = "^/(dev|proc|run/credentials/.+|sys|var/lib/docker/.+)($|/)"
    // Timeout for filesystem operations
    mount_timeout        = "5s"
  }

  netclass {
    // Ignore virtual and container network interfaces
    ignored_devices = "^(veth.*|cali.*|[a-f0-9]{15})$"
  }

  netdev {
    // Exclude virtual and container network interfaces from device metrics
    device_exclude = "^(veth.*|cali.*|[a-f0-9]{15})$"
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

#### `loki.source.journal`

```alloy
// Collect logs from systemd journal for node_exporter integration
loki.source.journal "logs_integrations_integrations_node_exporter_journal_scrape" {
  // Only collect logs from the last 24 hours
  max_age       = "24h0m0s"
  // Apply relabeling rules to the logs
  relabel_rules = discovery.relabel.logs_integrations_integrations_node_exporter_journal_scrape.rules
  // Send logs to the local Loki instance
  forward_to    = [loki.write.local.receiver]
}
```

#### `local.file_match`

```alloy
// Define which log files to collect for node_exporter
local.file_match "logs_integrations_integrations_node_exporter_direct_scrape" {
  path_targets = [{
    // Target localhost for log collection
    __address__ = "localhost",
    // Collect standard system logs
    __path__    = "/var/log/{syslog,messages,*.log}",
    // Add instance label with hostname
    instance    = constants.hostname,
    // Add job label for logs
    job         = "integrations/node_exporter",
  }]
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

#### `loki.source.file`

```alloy
// Collect logs from files for node_exporter
loki.source.file "logs_integrations_integrations_node_exporter_direct_scrape" {
  // Use targets defined in local.file_match
  targets    = local.file_match.logs_integrations_integrations_node_exporter_direct_scrape.targets
  // Send logs to the local Loki instance
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

#### `livedebugging`

`livedebugging` is disabled by default.
Enable it explicitly through the `livedebugging` configuration block to make debugging data visible in the {{< param "PRODUCT_NAME" >}} UI. You can use an empty configuration for this block and Alloy uses the default values.

```alloy
livedebugging{}
```

[prometheus.scrape]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.remote_write/
[discovery.relabel]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/discovery/discovery.relabel/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.write/
