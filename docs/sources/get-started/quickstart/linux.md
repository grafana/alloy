---
canonical: https://grafana.com/docs/alloy/latest/get-started/quickstart/linux/
description: Get Linux server metrics into Grafana quickly with Grafana Alloy
menuTitle: Quickstart Linux monitoring
title: Quickstart Linux monitoring with Grafana Alloy
weight: 200
---

# Quickstart Linux monitoring with {{% param "FULL_PRODUCT_NAME" %}}

Get your Linux server metrics flowing to Grafana quickly.
This guide shows you how to install {{< param "PRODUCT_NAME" >}}, configure it to collect essential system metrics (CPU, memory, disk, network), and visualize them in Grafana.

## Before you begin

Before you begin, ensure you have the following:

- A Linux server with administrative privileges
- A Grafana instance with Prometheus data source configured

  If you don't have a Grafana instance yet, you can:

  - [Set up Grafana Cloud](https://grafana.com/docs/grafana-cloud/account-management/create-account/) for a managed solution, or
  - [Install Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/installation/) on your own infrastructure

  To configure a Prometheus data source in Grafana, refer to [Add a Prometheus data source](https://grafana.com/docs/grafana/latest/datasources/prometheus/configure-prometheus-data-source/).

## Step 1: Install {{% param "PRODUCT_NAME" %}}

Choose the installation method for your Linux distribution:

{{< tabs >}}
{{< tab-content name="Ubuntu and Debian" >}}

```shell
curl -fsSL https://github.com/grafana/alloy/releases/latest/download/alloy-linux-amd64.deb -o alloy.deb
sudo dpkg -i alloy.deb
```

{{< /tab-content >}}
{{< tab-content name="RHEL, CentOS, and Fedora" >}}

```shell
curl -fsSL https://github.com/grafana/alloy/releases/latest/download/alloy-linux-amd64.rpm -o alloy.rpm
sudo rpm -i alloy.rpm
```

{{< /tab-content >}}
{{< /tabs >}}

{{< admonition type="note" >}}
If the installation fails, verify that you have the required permissions and that your system architecture matches the download.
{{< /admonition >}}

## Step 2: Edit the {{% param "PRODUCT_NAME" %}} configuration file

{{< admonition type="note" >}}
This configuration collects essential system metrics including CPU usage, memory utilization, disk space, and network statistics.
The comments explain what each section does to help you understand and customize the configuration.
{{< /admonition >}}

1. Edit the default configuration file at `/etc/alloy/config.alloy`.

   ```shell
   sudo nano /etc/alloy/config.alloy
   ```

1. Copy and paste the following configuration:

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
       // Ignore virtual network interfaces (Docker, Kubernetes, etc.)
       ignored_devices = "^(veth.*|cali.*|[a-f0-9]{15})$"
     }

     netdev {
       // Exclude virtual network interfaces from device metrics
       device_exclude = "^(veth.*|cali.*|[a-f0-9]{15})$"
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

     // Set a job label to identify this as Linux node metrics
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
   To find your `remote_write` connection details if you are using a Grafana Cloud connection:

   1. Log in to [Grafana Cloud](https://grafana.com/).
   1. Navigate to **Connections** > **Add new connection** > **Hosted Prometheus metrics**.
   1. Copy the following details:
      - **URL** (Remote Write Endpoint)
      - **Username**
      - **Password/API Key**

   If you are using a self-managed Grafana connection, the _`<PROMETHEUS_REMOTE_WRITE_URL>`_ should be `"http://<YOUR-PROMETHEUS-SERVER-URL>:9090/api/v1/write"`.
   The _`<USERNAME>`_ and _`<PASSWORD>`_ are what you set up when you installed Grafana and Prometheus.
   {{< /admonition >}}

## Step 3: Restart {{% param "PRODUCT_NAME" %}}

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   sudo systemctl restart alloy
   ```

1. (Optional) Verify that {{< param "PRODUCT_NAME" >}} is running:

   ```shell
   sudo systemctl status alloy
   ```

   You should see output indicating the service is `active (running)`.

### Troubleshoot the service

If the service fails to start, check the logs:

```shell
sudo journalctl -u alloy -f
```

Common issues:

- **Configuration syntax errors**: Check your configuration file for typos or missing values
- **Network connectivity**: Verify your Grafana Cloud credentials and network access
- **Permission errors**: Ensure {{< param "PRODUCT_NAME" >}} has read access to system metrics
- **Empty configuration**: An empty or invalid configuration file can cause startup failures

## Step 4: Visualize your metrics in Grafana

Within a few minutes of starting {{< param "PRODUCT_NAME" >}}, your Linux metrics should appear in Grafana.

### Visualize in Grafana Cloud

1. Log in to your [Grafana Cloud](https://grafana.com/) instance.
1. Navigate to **Connections** > **Infrastructure** > **Linux Node**.
1. Click **Install Integration** if not already installed.
1. Go to **Dashboards** and look for the **Node Exporter / USE Method / Node** dashboard.

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

The dashboard displays comprehensive Linux system metrics:

- **CPU Usage**: Real-time CPU utilization across all cores
- **Memory Usage**: Available, used, and cached memory
- **Disk Usage**: Disk space utilization and I/O statistics
- **Network Traffic**: Network interface throughput and errors
- **System Load**: Load average and running processes

{{< admonition type="note" >}}
Metrics should appear in Grafana within a few minutes of starting {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Troubleshoot

If metrics don't appear in Grafana after several minutes, check these common issues:

### Verify {{< param "PRODUCT_NAME" >}} is running

```shell
sudo systemctl status alloy
sudo journalctl -u alloy -n 20
```

Look for error messages about configuration parsing, network connectivity, or authentication.

### Check configuration syntax

Validate your configuration file:

```shell
sudo alloy fmt /etc/alloy/config.alloy
```

This command checks for syntax errors and formats the file.

### Test network connectivity

Verify that {{< param "PRODUCT_NAME" >}} can reach your Prometheus endpoint:

```shell
curl -v "<PROMETHEUS_REMOTE_WRITE_URL>"
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.

### Verify credentials

Double-check your Grafana Cloud or Prometheus credentials:

```shell
sudo cat /etc/alloy/config.alloy | grep -A 5 "prometheus.remote_write"
```

### Check the {{< param "PRODUCT_NAME" >}} UI

{{< param "PRODUCT_NAME" >}} provides a web UI for debugging:

1. Open your browser and go to `http://localhost:12345`.
1. Check the **Graph** tab to see component connections.
1. Look at component health indicators for any errors.

For more information about the UI, refer to [Debug {{< param "FULL_PRODUCT_NAME" >}}](https://grafana.com/docs/alloy/latest/troubleshoot/debug/).

### Common solutions

- **Service won't start**: Restart the service: `sudo systemctl restart alloy`
- **Permission denied**: Check file ownership: `sudo chown alloy:alloy /etc/alloy/config.alloy`
- **Network timeout**: Verify firewall settings and internet connectivity
- **Authentication failed**: Regenerate your API token in Grafana Cloud
- **No metrics in Grafana**: Wait a few minutes for the first scrape cycle to complete

## Next steps

- [Set up alerting rules](https://grafana.com/docs/grafana/latest/alerting/) to get notified when metrics exceed thresholds
- [Configure application metrics collection](https://grafana.com/docs/alloy/latest/reference/components/prometheus/) from services running on your servers
- [Add log collection](https://grafana.com/docs/alloy/latest/reference/components/loki/) to complement your metrics
- [Monitor multiple servers](https://grafana.com/docs/alloy/latest/configure/) with centralized {{< param "PRODUCT_NAME" >}} configuration
- [Explore the alloy-scenarios repository](https://github.com/grafana/alloy-scenarios) for more advanced configurations

### Learn more

- [{{< param "FULL_PRODUCT_NAME" >}} documentation](https://grafana.com/docs/alloy/latest/)
- [Prometheus monitoring concepts](https://grafana.com/docs/grafana/latest/fundamentals/intro-prometheus/)
- [Grafana dashboard best practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)
- [Observability with Grafana](https://grafana.com/docs/grafana/latest/fundamentals/)
