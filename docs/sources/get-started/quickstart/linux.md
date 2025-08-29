---
canonical: https://grafana.com/docs/alloy/latest/get-started/quickstart/linux/
description: Get Linux server metrics into Grafana quickly with Grafana Alloy
menuTitle: Quick start Linux monitoring
title: Quick start Linux monitoring with Grafana Alloy
weight: 100
---

# Quick start Linux monitoring with {{% param "FULL_PRODUCT_NAME" %}}

Get your Linux server metrics flowing to Grafana quickly.
This guide shows you how to install {{< param "PRODUCT_NAME" >}}, configure it to collect essential system metrics (CPU, memory, disk, network), and visualize them in Grafana.

## Before you begin

Before you begin, ensure you have the following:

- A Linux server with administrator privileges
- A Grafana instance (Grafana Cloud or self-managed Grafana with Prometheus)
- Basic command line familiarity

## Step 1: Install {{% param "PRODUCT_NAME" %}}

{{< admonition type="note" >}}
For production environments, consider using the [repository-based installation method](../../set-up/install/linux/) instead.
This enables automatic updates and better package management integration.
{{< /admonition >}}

Choose the installation method for your Linux distribution:

{{< tabs >}}
{{< tab-content name="Ubuntu and Debian" >}}

```shell
curl -fsSL https://github.com/grafana/alloy/releases/latest/download/alloy-linux-amd64.deb -o alloy.deb
sudo dpkg -i alloy.deb
```

{{< /tab-content >}}
{{< tab-content name= "RHEL, CentOS, and Fedora" >}}

```shell
curl -fsSL https://github.com/grafana/alloy/releases/latest/download/alloy-linux-amd64.rpm -o alloy.rpm
sudo rpm -i alloy.rpm
```

{{< /tab-content >}}
{{< tab-content name= "Generic Linux (binary)" >}}

```shell
curl -fsSL https://github.com/grafana/alloy/releases/latest/download/alloy-linux-amd64.zip -o alloy.zip
unzip alloy.zip
sudo mv alloy-linux-amd64 /usr/local/bin/alloy
sudo chmod +x /usr/local/bin/alloy
```

{{< /tab-content >}}
{{< /tabs >}}

{{< admonition type="note" >}}
If the installation fails, check that you have the required permissions and that your system architecture matches the download (amd64).
For other architectures, visit the [Alloy releases page](https://github.com/grafana/alloy/releases/latest).
{{< /admonition >}}

## Step 2: Create the configuration file

Create a configuration file that collects essential Linux metrics and sends them to Grafana.

{{< admonition type="note" >}}
If you installed using the DEB or RPM packages, the `/etc/alloy` directory and a default configuration file are already created.
You can edit the existing file or replace it with the configuration below.
{{< /admonition >}}

For binary installations, create the configuration directory:

```shell
sudo mkdir -p /etc/alloy
```

Create or edit the configuration file with your preferred text editor:

```shell
sudo nano /etc/alloy/config.alloy
```

Copy and paste this configuration:

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
    url = "YOUR_PROMETHEUS_URL"

    basic_auth {
      username = "YOUR_PROMETHEUS_USERNAME"
      password = "YOUR_API_TOKEN"
    }
  }
}
```

{{< admonition type="note" >}}
This configuration collects essential system metrics including CPU usage, memory utilization, disk space, and network statistics.
The comments explain what each section does to help you understand and customize the configuration.
{{< /admonition >}}

## Step 3: Configure your Grafana connection

Update the configuration with your Grafana connection details.

### Configure Grafana Cloud connection

1. Log in to [Grafana Cloud](https://grafana.com/).
1. Navigate to **Connections** > **Add new connection** > **Hosted Prometheus metrics**.
1. Copy the following details:
   - **URL** (Remote Write Endpoint)
   - **Username**
   - **Password/API Key**

### Configure self-managed Grafana connection

Replace the `prometheus.remote_write` block with your Prometheus server details:

```alloy
prometheus.remote_write "prometheus" {
  endpoint {
    url = "http://your-prometheus-server:9090/api/v1/write"
  }
}
```

### Update the configuration file

Edit the configuration file:

```shell
sudo nano /etc/alloy/config.alloy
```

Replace these placeholders with your actual values:

- `YOUR_PROMETHEUS_URL` - paste your Remote Write Endpoint
- `YOUR_PROMETHEUS_USERNAME` - paste your Username
- `YOUR_API_TOKEN` - paste your Password/API Key

Save and exit the file.

## Step 4: Start {{% param "PRODUCT_NAME" %}}

Start the {{< param "PRODUCT_NAME" >}} service:

```shell
sudo systemctl enable alloy
sudo systemctl start alloy
```

Verify that {{< param "PRODUCT_NAME" >}} is running:

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

## Step 5: Visualize your metrics in Grafana

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
**Expected timeline**: Metrics should appear in Grafana within a few minutes of starting {{< param "PRODUCT_NAME" >}}.
If you don't see data after several minutes, check the troubleshooting section below.
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
curl -v "YOUR_PROMETHEUS_URL"
```

Replace `YOUR_PROMETHEUS_URL` with your actual endpoint.

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

Congratulations. You now have {{< param "PRODUCT_NAME" >}} collecting Linux metrics and displaying them in Grafana.

Here's what you can do next:

- [Set up alerting rules](https://grafana.com/docs/grafana/latest/alerting/) to get notified when metrics exceed thresholds
- [Configure application metrics collection](https://grafana.com/docs/alloy/latest/reference/components/prometheus/) from services running on your servers
- [Add log collection](https://grafana.com/docs/alloy/latest/reference/components/loki/) to complement your metrics
- [Monitor multiple servers](https://grafana.com/docs/alloy/latest/configure/) with centralized {{< param "PRODUCT_NAME" >}} configuration
- [Explore the alloy-scenarios repository](https://github.com/grafana/alloy-scenarios) for more advanced configurations

### Production considerations

For production deployments, consider:

- [Installing {{< param "PRODUCT_NAME" >}} using the package manager](https://grafana.com/docs/alloy/latest/set-up/install/) for automatic updates
- [Configuring {{< param "PRODUCT_NAME" >}} as a system service](https://grafana.com/docs/alloy/latest/set-up/deploy/) with automatic startup
- [Setting up log rotation](https://grafana.com/docs/alloy/latest/configure/configure-logging/) and monitoring {{< param "PRODUCT_NAME" >}} itself
- [Using configuration management tools](https://grafana.com/docs/alloy/latest/configure/) to deploy {{< param "PRODUCT_NAME" >}} across multiple servers
- [Implementing security best practices](https://grafana.com/docs/alloy/latest/configure/configure-security/) for credential management

### Learn more

- [{{< param "FULL_PRODUCT_NAME" >}} documentation](https://grafana.com/docs/alloy/latest/)
- [Prometheus monitoring concepts](https://grafana.com/docs/grafana/latest/fundamentals/intro-prometheus/)
- [Grafana dashboard best practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)
- [Observability with Grafana](https://grafana.com/docs/grafana/latest/fundamentals/)
