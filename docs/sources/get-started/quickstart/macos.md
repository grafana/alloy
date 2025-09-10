---
canonical: https://grafana.com/docs/alloy/latest/get-started/quickstart/macos/
description: Get macOS server metrics into Grafana quickly with Grafana Alloy
menuTitle: Quickstart macOS monitoring
title: Quickstart macOS monitoring with Grafana Alloy
weight: 300
---

# Quickstart macOS monitoring with {{% param "FULL_PRODUCT_NAME" %}}

Get your macOS server metrics flowing to Grafana quickly.
This guide shows you how to install {{< param "PRODUCT_NAME" >}}, configure it to collect essential system metrics (CPU, memory, disk, network), and visualize them in Grafana.

## Before you begin

Before you begin, ensure you have the following:

- A macOS system with administrator privileges
- A Grafana instance with Prometheus data source configured

If you don't have a Grafana instance yet, you can:

  - [Set up Grafana Cloud](https://grafana.com/docs/grafana-cloud/get-started/) for a managed solution, or
  - [Install Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/installation/) on your own infrastructure

  To configure a Prometheus data source in Grafana, refer to [Add a Prometheus data source](https://grafana.com/docs/grafana/latest/datasources/prometheus/configure/).

## Step 1: Install {{% param "PRODUCT_NAME" %}}

Install {{< param "PRODUCT_NAME" >}} using Homebrew:

```shell
brew install grafana/grafana/alloy
```

## Step 2: Configure {{% param "PRODUCT_NAME" %}}

1. Open the default configuration file in your text editor:

   ```shell
   open $(brew --prefix)/etc/alloy/config.alloy
   ```

1. Replace the contents with the following configuration:

   ```alloy
   // This block runs a built-in node exporter to collect CPU, memory, disk, and network metrics
   prometheus.exporter.unix "default" {
     // Disable collectors we don't need to reduce overhead
     disable_collectors = ["ipvs", "btrfs", "infiniband", "xfs", "zfs"]

     // Enable memory info collector for detailed memory metrics
     enable_collectors = ["meminfo"]

     // Configure filesystem monitoring
     filesystem {
       // Exclude virtual filesystems that aren't useful for monitoring
       fs_types_exclude = "^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|tmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs)$"

       // Exclude system mount points that aren't relevant
       mount_points_exclude = "^/(dev|proc|run/credentials/.+|sys|var/lib/docker/.+)($|/)"

       // Timeout for filesystem operations
       mount_timeout = "5s"
     }

     // Network monitoring configuration
     netclass {
       // Ignore virtual network interfaces (Docker, VMware, etc.)
       ignored_devices = "^(veth.*|vmnet.*|bridge[0-9]+|utun[0-9]+|[a-f0-9]{15})$"
     }

     netdev {
       // Exclude virtual network interfaces from device metrics
       device_exclude = "^(veth.*|vmnet.*|bridge[0-9]+|utun[0-9]+|[a-f0-9]{15})$"
     }
   }

   // This block adds standard labels to our metrics for better organization in Grafana
   discovery.relabel "default" {
     targets = prometheus.exporter.unix.default.targets

     // Set the instance label to this server's hostname
     rule {
       target_label = "instance"
       replacement  = constants.hostname
     }

     // Set a job label to identify this as macOS node metrics
     rule {
       target_label = "job"
       replacement  = "integrations/node_exporter"
     }
   }

   // This block collects the metrics from the node exporter every 15 seconds
   prometheus.scrape "default" {
     targets    = discovery.relabel.default.output
     forward_to = [prometheus.remote_write.grafana_cloud.receiver]
     scrape_interval = "15s"
   }

   // This block sends your metrics to Grafana Cloud
   // Replace the placeholders with your actual Grafana Cloud values
   prometheus.remote_write "grafana_cloud" {
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

   - _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
   - _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
   - _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

   {{< admonition type="tip" >}}
   To find your `remote_write` connection details if you are using Grafana Cloud:

   1. Log in to [Grafana Cloud](https://grafana.com/).
   1. Navigate to **Connections** and select **Data sources**.
   1. Find your **Prometheus** connection in the list.
   1. Click on the Prometheus connection to view its configuration.
   1. Copy the following details from the configuration:
      - **URL** (Remote Write Endpoint) - found in the HTTP section
      - **Username** - found in the Authentication section  
      - **Password/API Key** - this is the API token you generated previously

   If you are using a self-managed Grafana connection, the _`<PROMETHEUS_REMOTE_WRITE_URL>`_ should be `"http://<YOUR-PROMETHEUS-SERVER-URL>:9090/api/v1/write"`.
   The _`<USERNAME>`_ and _`<PASSWORD>`_ are what you set up when you installed Grafana and Prometheus.
   {{< /admonition >}}

## Step 3: Start {{% param "PRODUCT_NAME" %}}

1. Start the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   brew services start alloy
   ```

1. (Optional) Verify that {{< param "PRODUCT_NAME" >}} is running:

   ```shell
   brew services list | grep alloy
   ```

   You should see {{< param "PRODUCT_NAME" >}} listed as "started" in green.

### Troubleshoot the service

If {{< param "PRODUCT_NAME" >}} fails to start, check the logs for error messages:

```shell
brew services info alloy
```

Common issues:

- **Configuration syntax errors**: Check your configuration file for typos or missing values
- **Network connectivity**: Verify your Grafana Cloud credentials and network access
- **Permission errors**: Ensure {{< param "PRODUCT_NAME" >}} has read access to system metrics
- **Homebrew service issues**: Try restarting with `brew services restart alloy`
- **Port conflicts**: Ensure port 12345 is not in use by another service

## Step 4: Visualize your metrics in Grafana

Within a few minutes of starting {{< param "PRODUCT_NAME" >}}, your macOS metrics should appear in Grafana.

### Visualize in Grafana Cloud

1. Log in to your [Grafana Cloud](https://grafana.com/) instance.
1. Navigate to **Connections** > **Infrastructure** > **Linux** (the dashboards work for macOS too).
1. Click **Install Integration** if not already installed.
1. Go to **Dashboards** and look for Linux-related dashboards such as:
   - **Node Exporter / Unix Host**
   - **Node Exporter / Use Method / Cluster**
   - **Node Exporter / MacOS**

Alternatively, import a community dashboard:

1. Go to **Dashboards** > **New** > **Import**.
1. Enter dashboard ID: `1860` (Node Exporter Full).
1. Click **Load**.
1. Select your Prometheus data source and click **Import**.

### Visualize in self-managed Grafana

1. Open your Grafana instance.
1. Go to **Dashboards** > **New** > **Import**.
1. Enter dashboard ID `1860` or download the JSON from the [Grafana dashboard library](https://grafana.com/grafana/dashboards/1860-node-exporter-full/).
1. Click **Load**.
1. Select your Prometheus data source and click **Import**.

### What you should see

The dashboard displays comprehensive macOS system metrics:

- **System Overview**: CPU usage, memory utilization, disk space, and network activity
- **CPU Metrics**: Per-core usage, load averages, and context switches
- **Memory Metrics**: Available memory, swap usage, and buffer/cache details
- **Disk Metrics**: Disk I/O, free space per mount point, and inode usage
- **Network Metrics**: Network I/O, packet rates, and error counts for each interface

{{< admonition type="note" >}}
Metrics should appear in Grafana within a few minutes of starting {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Troubleshoot

If metrics don't appear in Grafana after several minutes, check these common issues:

### Verify {{< param "PRODUCT_NAME" >}} is running

```shell
brew services list | grep alloy
ps aux | grep alloy
```

### Check {{< param "PRODUCT_NAME" >}} logs

```shell
brew services info alloy
tail -f /opt/homebrew/var/log/alloy.log
```

Look for error messages about configuration parsing, network connectivity, or authentication.

### Test network connectivity

Verify that {{< param "PRODUCT_NAME" >}} can reach your Prometheus endpoint:

```shell
curl -I "<PROMETHEUS_REMOTE_WRITE_URL>"
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.

### Check configuration syntax

Validate your configuration by running {{< param "PRODUCT_NAME" >}} in the foreground:

```shell
alloy run /opt/homebrew/etc/alloy/config.alloy --server.http.listen-addr=127.0.0.1:12345
```

### Access the {{< param "PRODUCT_NAME" >}} UI

Open your browser and navigate to `http://localhost:12345` to inspect:

1. **Graph** tab for component connections
2. Component health indicators for any errors

### Common solutions

- **Permission denied**: Ensure {{< param "PRODUCT_NAME" >}} has the necessary permissions to read system metrics
- **Network timeout**: Check firewall settings and network connectivity to your Prometheus endpoint
- **Authentication failed**: Verify your Grafana Cloud credentials are correct
- **Configuration errors**: Check the syntax of your configuration file
- **Port conflicts**: Ensure port 12345 is not in use by another service

### macOS-specific troubleshooting

- **System permissions**: macOS may require additional permissions for system monitoring
- **Firewall blocking**: Check macOS firewall settings for outbound connections
- **Virtual interfaces**: Adjust network interface exclusions for your specific setup
- **Homebrew path issues**: Ensure `/opt/homebrew/bin` (Apple Silicon) or `/usr/local/bin` (Intel) is in your PATH

## Next steps

- [Set up alerting rules](https://grafana.com/docs/grafana/latest/alerting/) to get notified when system metrics exceed thresholds
- [Configure log collection](https://grafana.com/docs/alloy/latest/reference/components/loki/) from macOS system logs
- [Add custom dashboards](https://grafana.com/docs/grafana/latest/dashboards/) tailored to your specific monitoring needs
- [Monitor applications](https://grafana.com/docs/alloy/latest/reference/components/prometheus/) running on your macOS system
- [Explore advanced configurations](https://grafana.com/docs/alloy/latest/configure/) for production deployments

### Learn more

- [{{< param "FULL_PRODUCT_NAME" >}} documentation](https://grafana.com/docs/alloy/latest/)
- [macOS monitoring best practices](https://grafana.com/docs/grafana/latest/fundamentals/intro-prometheus/)
- [Grafana dashboard best practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)
- [Observability with Grafana](https://grafana.com/docs/grafana/latest/fundamentals/)
