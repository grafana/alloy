---
canonical: https://grafana.com/docs/alloy/latest/get-started/quickstart/windows/
description: Get Windows server metrics into Grafana quickly with Grafana Alloy
menuTitle: Quick start Windows monitoring
title: Quick start Windows monitoring with Grafana Alloy
weight: 200
---

# Quick start Windows monitoring with {{% param "FULL_PRODUCT_NAME" %}}

Get your Windows server metrics flowing to Grafana quickly.
This guide shows you how to install {{< param "PRODUCT_NAME" >}}, configure it to collect essential system metrics (CPU, memory, disk, network), and visualize them in Grafana.

## Before you begin

Before you begin, ensure you have the following:

- A Windows server with administrator privileges
- A Grafana instance with Prometheus data source

## Step 1: Install {{% param "PRODUCT_NAME" %}}

{{< admonition type="note" >}}
This quickstart uses the installer executable for a fast setup.
For detailed installation options, refer to [Install {{< param "PRODUCT_NAME" >}} on Windows](../../set-up/install/windows/).
{{< /admonition >}}

1. Navigate to the [latest release](https://github.com/grafana/alloy/releases/latest) on GitHub.
1. Scroll down to the **Assets** section.
1. Download the file called `alloy-installer-windows-amd64.exe.zip`.
1. Extract the downloaded file.
1. Right-click on `alloy-installer-windows-amd64.exe` and select **Run as administrator**.
1. Follow the installation wizard to complete the setup.

{{< param "PRODUCT_NAME" >}} is installed into the default directory `%PROGRAMFILES%\GrafanaLabs\Alloy` and configured as a Windows service that starts automatically.

{{< admonition type="note" >}}
If the installation fails, check that you have administrator privileges and that your system architecture is 64-bit.
{{< /admonition >}}

## Step 2: Edit the {{% param "PRODUCT_NAME" %}} configuration file

{{< admonition type="note" >}}
This configuration collects essential Windows metrics including CPU usage, memory utilization, disk space, network statistics, and key system services.
The comments explain what each section does to help you understand and customize the configuration.
{{< /admonition >}}

Edit the default configuration file at `%PROGRAMFILES%\GrafanaLabs\Alloy\config.alloy`.

```powershell
notepad "%PROGRAMFILES%\GrafanaLabs\Alloy\config.alloy"
```

Copy and paste the following configuration:

```alloy
// This block runs a built-in Windows exporter to collect CPU, memory, disk, and network metrics
prometheus.exporter.windows "default" {
  // Enable essential Windows performance counters
  enabled_collectors = [
    "cpu",
    "cs",
    "logical_disk",
    "memory",
    "net",
    "os",
    "pagefile",
    "physical_disk",
    "process",
    "service",
    "system",
    "textfile"
  ]

  // Configure disk monitoring
  logical_disk {
    // Exclude unmounted system volumes (HarddiskVolume* without drive letters)
    // Note: System files like pagefile.sys and hiberfil.sys are included in disk usage metrics,
    // which is normal and expected for monitoring purposes
    volume_exclude = "^HarddiskVolume[0-9]+$"
  }

  // Configure network monitoring
  net {
    // Exclude virtual network interfaces (Hyper-V, VMware, etc.)
    nic_exclude = "^(Teredo|isatap|Local Area Connection.*[0-9]+|Bluetooth).*$"
  }

  // Configure physical disk monitoring
  physical_disk {
    // Exclude virtual disks
    disk_exclude = "^(\\\\?\\Volume|Harddisk|_Total).*$"
  }

  // Configure process monitoring (limit to reduce overhead)
  process {
    // Include only essential processes to reduce metric volume
    process_include = "^(dwm|explorer|svchost|System|Registry|smss|csrss|wininit|winlogon|services|lsass|spoolsv)$"
  }

  // Configure service monitoring for critical Windows services
  service {
    service_include = "^(BITS|Browser|Dhcp|Dnscache|EventLog|Netlogon|PlugPlay|Spooler|TrkWks|W32Time|Winmgmt|Workstation)$"
  }
}

// This block adds standard labels to our metrics for better organization in Grafana
discovery.relabel "default" {
  targets = prometheus.exporter.windows.default.targets

  // Set the instance label to this server's hostname
  rule {
    target_label = "instance"
    replacement  = constants.hostname
  }

  // Set a job label to identify this as Windows node metrics
  rule {
    target_label = "job"
    replacement  = "integrations/windows_exporter"
  }
}

// This block collects the metrics from the Windows exporter every 15 seconds
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

{{< param "PRODUCT_NAME" >}} is automatically installed and configured as a Windows service that starts on system boot.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   In the Services manager, right-click on the **{{< param "PRODUCT_NAME" >}}** service and click **All Tasks > Restart**.

   Alternatively, use PowerShell:

   ```powershell
   Restart-Service -Name "Grafana Alloy"
   ```

1. (Optional) Verify that {{< param "PRODUCT_NAME" >}} is running:

   Open the Windows Services manager:

   1. Right-click on the Start Menu and select **Run**.
   1. Type `services.msc` and press **Enter**.
   1. Scroll down to find the **{{< param "PRODUCT_NAME" >}}** service and verify that the **Status** is **Running**.

### Troubleshoot the service

If the service fails to start, check the Windows Event Log:

1. Right-click on the Start Menu and select **Run**.
1. Type `eventvwr` and press **Enter**.
1. Navigate to **Windows Logs > Application**.
1. Look for events with the source **{{< param "FULL_PRODUCT_NAME" >}}**.

Common issues:

- **Configuration syntax errors**: Check your configuration file for typos or missing values
- **Network connectivity**: Verify your Grafana Cloud credentials and network access
- **Permission errors**: Ensure the service account has proper permissions to read system metrics
- **Empty configuration**: An empty or invalid configuration file can cause startup failures
- **Firewall blocking**: Check Windows Firewall settings for outbound HTTPS connections

## Step 4: Visualize your metrics in Grafana

Within a few minutes of starting {{< param "PRODUCT_NAME" >}}, your Windows metrics should appear in Grafana.

### Visualize in Grafana Cloud

1. Log in to your [Grafana Cloud](https://grafana.com/) instance.
1. Navigate to **Connections** > **Infrastructure** > **Windows Node**.
1. Click **Install Integration** if not already installed.
1. Go to **Dashboards** and look for the **Windows Node** dashboard.

Alternatively, import a community dashboard:

1. Go to **Dashboards** > **New** > **Import**.
1. Enter dashboard ID: `14694` (Windows Exporter Dashboard).
1. Click **Load**.
1. Select your Prometheus data source and click **Import**.

### Visualize in self-managed Grafana

1. Open your Grafana instance.
1. Go to **Dashboards** > **New** > **Import**.
1. Enter dashboard ID `14694` or download the JSON from the [Grafana dashboard library](https://grafana.com/grafana/dashboards/14694-windows-exporter-dashboard/).
1. Click **Load**.
1. Select your Prometheus data source and click **Import**.

### What you should see

The dashboard displays comprehensive Windows system metrics:

- **CPU Usage**: Real-time CPU utilization across all cores
- **Memory Usage**: Available, committed, and cached memory
- **Disk Usage**: Disk space utilization and I/O statistics
- **Network Traffic**: Network interface throughput and errors
- **System Services**: Status of critical Windows services
- **Process Information**: Resource usage of key system processes

{{< admonition type="note" >}}
Metrics should appear in Grafana within a few minutes of starting {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Troubleshoot

If metrics don't appear in Grafana after several minutes, check these common issues:

### Verify {{< param "PRODUCT_NAME" >}} is running

```powershell
Get-Service -Name "Grafana Alloy"
```

You can also check the Event Log for recent entries from {{< param "PRODUCT_NAME" >}}.

Look for error messages about configuration parsing, network connectivity, or authentication.

### Check configuration syntax

Validate your configuration file:

```powershell
& "%PROGRAMFILES%\GrafanaLabs\Alloy\alloy.exe" fmt "%PROGRAMFILES%\GrafanaLabs\Alloy\config.alloy"
```

This command checks for syntax errors and formats the file.

### Test network connectivity

Verify that {{< param "PRODUCT_NAME" >}} can reach your Prometheus endpoint:

```powershell
Test-NetConnection -ComputerName "<PROMETHEUS_HOSTNAME>" -Port 443
```

Replace the following:

- _`<PROMETHEUS_HOSTNAME>`_: Your actual endpoint hostname from the URL.

### Verify credentials

Double-check your Grafana Cloud or Prometheus credentials:

```powershell
Get-Content "%PROGRAMFILES%\GrafanaLabs\Alloy\config.alloy" | Select-String -Pattern "prometheus.remote_write" -Context 5
```

### Check the {{< param "PRODUCT_NAME" >}} UI

{{< param "PRODUCT_NAME" >}} provides a web UI for debugging:

1. Open your browser and go to `http://localhost:12345`.
1. Check the **Graph** tab to see component connections.
1. Look at component health indicators for any errors.

For more information about the UI, refer to [Debug {{< param "FULL_PRODUCT_NAME" >}}](https://grafana.com/docs/alloy/latest/troubleshoot/debug/).

### Common solutions

- **Service won't start**: Restart the service using the Services manager or PowerShell: `Restart-Service -Name "Grafana Alloy"`
- **Permission denied**: Ensure you're running PowerShell as Administrator and check service account permissions
- **Network timeout**: Verify firewall settings and internet connectivity
- **Authentication failed**: Regenerate your API token in Grafana Cloud
- **No metrics in Grafana**: Wait a few minutes for the first scrape cycle to complete
- **Windows Firewall blocking**: Add exception for outbound HTTPS traffic

### Windows-specific troubleshooting

- **Performance counter access**: Ensure the {{< param "PRODUCT_NAME" >}} service account has "Log on as a service" rights
- **WMI access issues**: Verify WMI service is running: `Get-Service -Name "Winmgmt"`
- **Registry access**: Check that the service can read performance counter registry keys
- **Antivirus interference**: Add {{< param "PRODUCT_NAME" >}} to antivirus exclusions if necessary

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
