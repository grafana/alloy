---
canonical: https://grafana.com/docs/alloy/latest/get-started/quickstart/linux/
description: Get Linux server metrics into Grafana quickly with Grafana Alloy
menuTitle: Quickstart Linux monitoring
title: Quickstart Linux monitoring with Grafana Alloy
weight: 200
---

# Quickstart Linux monitoring with {{% param "FULL_PRODUCT_NAME" %}}

Get your Linux server metrics flowing to Grafana quickly.
This guide shows you how to install {{< param "PRODUCT_NAME" >}}, configure it to collect essential system metrics (CPU, memory, disk, network), and visualize them in Grafana Cloud.

This quickstart is for local installation in Linux and sending metrics to Grafana Cloud.
For a more in-depth guide, or if you want to run {{< param "PRODUCT_NAME" >}} in Docker, refer to [Monitor Linux servers with {{< param "FULL_PRODUCT_NAME" >}}](../monitor/monitor-linux/).

## Before you begin

Before you begin, ensure you have the following:

- A Linux server with administrative privileges
- A Grafana Cloud account with a Prometheus data source configured

  If you don't have a Grafana instance yet, you can [Set up Grafana Cloud](https://grafana.com/docs/grafana-cloud/get-started/).

  To configure a Prometheus data source in Grafana, refer to [Add a Prometheus data source](https://grafana.com/docs/grafana/latest/datasources/prometheus/configure/).

## Step 1: Install {{% param "PRODUCT_NAME" %}}

Choose the installation method for your Linux distribution:

{{< tabs >}}
{{< tab-content name="Debian and Ubuntu" >}}

1. Import the GPG key and add the Grafana package repository:

```shell
sudo mkdir -p /etc/apt/keyrings/
wget -q -O - https://apt.grafana.com/gpg.key | gpg --dearmor | sudo tee /etc/apt/keyrings/grafana.gpg > /dev/null
echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" | sudo tee /etc/apt/sources.list.d/grafana.list
sudo apt-get update
```

1. Install Alloy:

```shell
sudo apt-get install alloy
```

{{< /tab-content >}}
{{< tab-content name="RHEL and Fedora" >}}

1. Import the GPG key and add the Grafana YUM repository:

```shell
wget -q -O gpg.key https://rpm.grafana.com/gpg.key
sudo rpm --import gpg.key
echo -e '[grafana]\nname=grafana\nbaseurl=https://rpm.grafana.com\nrepo_gpgcheck=1\nenabled=1\ngpgcheck=1\ngpgkey=https://rpm.grafana.com/gpg.key\nsslverify=1\nsslcacert=/etc/pki/tls/certs/ca-bundle.crt' | sudo tee /etc/yum.repos.d/grafana.repo
```

1. Update the repository:

```shell
yum update
```

1. Install Alloy:

```shell
sudo dnf install alloy
```

{{< /tab-content >}}
{{< /tabs >}}

{{< admonition type="note" >}}
Repository-based installation is recommended for automatic updates and easier management.
For manual installation refer to the [Alloy releases page](https://github.com/grafana/alloy/releases/latest).
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

   - _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The `remote_write` endpoint URL from your Grafana Cloud account.
   - _`<USERNAME>`_: The username for your Grafana Cloud Prometheus `remote_write` endpoint.
   - _`<PASSWORD>`_: The API key or password for your Grafana Cloud Prometheus `remote_write` endpoint.

  {{< admonition type="tip" >}}
  To find your `remote_write` connection details:

  1. Log in to [Grafana Cloud](https://grafana.com/).
  1. Navigate to **Connections** and select **Data sources**.
  1. Find your **Prometheus** connection in the list.
  1. Click on the Prometheus connection to view its configuration.
  1. Copy the **URL**, **Username**, and **API Key** from the configuration.
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

Within a few minutes of starting {{< param "PRODUCT_NAME" >}}, your Linux metrics should appear in Grafana Cloud.

1. Log in to your [Grafana Cloud](https://grafana.com/) instance.
1. Navigate to **Connections** > **Infrastructure** > **Linux Node**.
1. Click **Install Integration** if not already installed.
1. Go to **Dashboards** and look for the **Node Exporter / USE Method / Node** dashboard.
1. Alternatively, import a community dashboard:

   - Go to **Dashboards** > **New** > **Import**.
   - Enter dashboard ID: `1860` (Node Exporter Full).
   - Click **Load**.
   - Select your Prometheus data source and click **Import**.

### What you should see

The dashboard displays comprehensive Linux system metrics:

- **CPU Usage**: Real-time CPU utilization across all cores
- **Memory Usage**: Available, used, and cached memory
- **Disk Usage**: Disk space utilization and I/O statistics
- **Network Traffic**: Network interface throughput and errors
- **System Load**: Load average and running processes

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
