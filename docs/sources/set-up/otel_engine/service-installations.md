---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/service-installations/
description: Learn how to run the Alloy OpenTelemetry Engine from an Alloy service installation on Linux, macOS, or Windows
menuTitle: Service installations
review_date: 2026-09-23
title: Run the Alloy OpenTelemetry Engine with service installations
weight: 400
---

# Run the {{% param "FULL_OTEL_ENGINE" %}} with service installations

The {{< param "PRODUCT_NAME" >}} service installations for Linux, macOS, and Windows can run the {{< param "OTEL_ENGINE" >}} instead of the {{< param "DEFAULT_ENGINE" >}}.
Set `ALLOY_OTEL_MODE` to `1` or `true` to run the {{< param "OTEL_ENGINE" >}}.
Matching isn't case-sensitive.
If you don't set `ALLOY_OTEL_MODE`, or you set it to any other value, {{< param "PRODUCT_NAME" >}} runs the {{< param "DEFAULT_ENGINE" >}}.

No service installer exposes an install-time flag for this setting.
Set it with the platform-specific mechanism below, then restart the service.

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Before you begin

Make sure you have {{< param "PRODUCT_NAME" >}} installed as a service.
Refer to [Install {{< param "FULL_PRODUCT_NAME" >}}][Install] for more information.

## Linux

{{< param "PRODUCT_NAME" >}} is [installed][RunLinux] as a systemd service on Linux.
It reads `ALLOY_OTEL_MODE` from the systemd environment file.

To run the {{< param "OTEL_ENGINE" >}} on Linux:

1. Edit the environment file for the service:

   - Debian or Ubuntu: Edit `/etc/default/alloy`
   - RHEL/Fedora or SUSE/openSUSE: Edit `/etc/sysconfig/alloy`

1. Set `ALLOY_OTEL_MODE=1`.

1. Edit the sample OpenTelemetry Collector configuration the package installs at `/etc/alloy/config.yaml`.
   To use a different file, set `OTEL_CONFIG_FILE` to its path.

1. Optional: Set `OTEL_CUSTOM_ARGS` to pass additional command-line flags to the {{< param "OTEL_ENGINE" >}}.
   This setting works the same way as `CUSTOM_ARGS` does for the {{< param "DEFAULT_ENGINE" >}}.

1. Restart the service:

   ```shell
   sudo systemctl restart alloy
   ```

1. Confirm the {{< param "OTEL_ENGINE" >}} is running:

   ```shell
   curl http://localhost:8888/metrics
   ```

Refer to [Pass additional command-line flags][ConfigureLinux] for more information about the environment file.

## macOS

{{< param "PRODUCT_NAME" >}} is [installed][RunMacOS] as a launchd service on macOS.
It reads `ALLOY_OTEL_MODE` from the Homebrew environment file.

To run the {{< param "OTEL_ENGINE" >}} on macOS:

1. Edit the file at `$(brew --prefix)/etc/alloy/config.env`.

1. Set `ALLOY_OTEL_MODE=1`.

1. Create your OpenTelemetry Collector configuration at `$(brew --prefix)/etc/alloy/config.yaml`.
   To use a different file, add `--config=<PATH>` to `$(brew --prefix)/etc/alloy/otel-extra-args.txt`.
   {{< param "PRODUCT_NAME" >}} applies this flag after the default, so its values override the installer's default where they overlap.

1. Optional: Add command-line flags for the {{< param "OTEL_ENGINE" >}} to `$(brew --prefix)/etc/alloy/otel-extra-args.txt`.
   This file works the same way as `extra-args.txt` does for the {{< param "DEFAULT_ENGINE" >}}.

1. Restart the service:

   ```shell
   brew services restart grafana/grafana/alloy
   ```

1. Confirm the {{< param "OTEL_ENGINE" >}} is running:

   ```shell
   curl http://localhost:8888/metrics
   ```

Refer to [Configure environment variables][ConfigureMacOS] for more information about the environment file.

## Windows

{{< param "PRODUCT_NAME" >}} is [installed][RunWindows] as a Windows Service.
It reads `ALLOY_OTEL_MODE` from the Windows registry at `HKLM\Software\GrafanaLabs\Alloy`.

To run the {{< param "OTEL_ENGINE" >}} on Windows:

1. Open the Registry Editor and navigate to `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy`.

1. Set the string value `ALLOY_OTEL_MODE` to `1`.

1. Edit the sample OpenTelemetry Collector configuration the installer creates at `%PROGRAMFILES%\GrafanaLabs\Alloy\config.yaml`.
   To use a different file, add `--config=<PATH>` to the multi-string value `OTelArguments`.
   {{< param "PRODUCT_NAME" >}} applies this flag after the default, so its values override the installer's default where they overlap.

1. Optional: Add command-line flags for the {{< param "OTEL_ENGINE" >}} to the multi-string value `OTelArguments`.
   This value works the same way as `Arguments` does for the {{< param "DEFAULT_ENGINE" >}}.

1. Restart the **{{< param "PRODUCT_NAME" >}}** service from the Windows Services manager.

1. Confirm the {{< param "OTEL_ENGINE" >}} is running:

   ```powershell
   Invoke-WebRequest http://localhost:8888/metrics -UseBasicParsing
   ```

Refer to [Service configuration][ConfigureWindows] for more information about the registry values {{< param "PRODUCT_NAME" >}} uses.

[RunLinux]: ../../run/linux/
[RunMacOS]: ../../run/macos/
[RunWindows]: ../../run/windows/
[ConfigureLinux]: ../../../configure/linux/#pass-additional-command-line-flags
[ConfigureMacOS]: ../../../configure/macos/#configure-environment-variables
[ConfigureWindows]: ../../install/windows/#service-configuration
[Install]: ../../install/
