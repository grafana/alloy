---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/service-installation/
description: Learn how to run the OpenTelemetry Engine from an Alloy service installation on Linux, macOS, or Windows
menuTitle: Service installation
title: Run the OpenTelemetry Engine with a service installation
weight: 400
---

# Run the {{% param "OTEL_ENGINE" %}} with a service installation

The {{< param "PRODUCT_NAME" >}} service installations for Linux, macOS, and Windows can run the {{< param "OTEL_ENGINE" >}} instead of the {{< param "DEFAULT_ENGINE" >}}.
Set `ALLOY_OTEL_MODE` to `1` or `true` (case-insensitive) to run the {{< param "OTEL_ENGINE" >}}.
Leaving it unset keeps the {{< param "DEFAULT_ENGINE" >}} running.

None of the service installers currently expose an install-time flag for this setting.
Set it using the platform-specific mechanism below, then restart the service.

## Linux

{{< param "PRODUCT_NAME" >}} is [installed][RunLinux] as a systemd service on Linux.
It reads `ALLOY_OTEL_MODE` from the systemd environment file.

To run the {{< param "OTEL_ENGINE" >}} on Linux:

1. Edit the environment file for the service:

   - Debian or Ubuntu: edit `/etc/default/alloy`
   - RHEL/Fedora or SUSE/openSUSE: edit `/etc/sysconfig/alloy`

1. Set `ALLOY_OTEL_MODE=1`.

1. Edit the sample OpenTelemetry Collector configuration the package installs at `/etc/alloy/config.yaml`, or set `OTEL_CONFIG_FILE` to point at a different file.

1. Optional: Set `OTEL_CUSTOM_ARGS` to pass additional command-line flags to the {{< param "OTEL_ENGINE" >}}.
   This setting works the same way as `CUSTOM_ARGS` does for the {{< param "DEFAULT_ENGINE" >}}.

1. Restart the service:

   ```shell
   sudo systemctl restart alloy
   ```

Refer to [Pass additional command-line flags][ConfigureLinux] for more information about the environment file.

## macOS

{{< param "PRODUCT_NAME" >}} is [installed][RunMacOS] as a launchd service on macOS.
It reads `ALLOY_OTEL_MODE` from the Homebrew environment file.

To run the {{< param "OTEL_ENGINE" >}} on macOS:

1. Edit the file at `$(brew --prefix)/etc/alloy/config.env`.

1. Set `ALLOY_OTEL_MODE=1`.

1. Create your OpenTelemetry Collector configuration at `$(brew --prefix)/etc/alloy/config.yaml`.

1. Optional: Add command-line flags for the {{< param "OTEL_ENGINE" >}} to `$(brew --prefix)/etc/alloy/otel-extra-args.txt`.
   This file works the same way as `extra-args.txt` does for the {{< param "DEFAULT_ENGINE" >}}.

1. Restart the service:

   ```shell
   brew services restart grafana/grafana/alloy
   ```

Refer to [Configure environment variables][ConfigureMacOS] for more information about the environment file.

## Windows

{{< param "PRODUCT_NAME" >}} is [installed][RunWindows] as a Windows Service.
It reads `ALLOY_OTEL_MODE` from the Windows registry at `HKLM\Software\GrafanaLabs\Alloy`.

To run the {{< param "OTEL_ENGINE" >}} on Windows:

1. Open the Registry Editor and navigate to `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy`.

1. Set the string value `ALLOY_OTEL_MODE` to `1`.

1. Edit the sample OpenTelemetry Collector configuration the installer creates at `%PROGRAMFILES%\GrafanaLabs\Alloy\config.yaml`.

1. Optional: Add command-line flags for the {{< param "OTEL_ENGINE" >}} to the multi-string value `OTelArguments`.
   This value works the same way as `Arguments` does for the {{< param "DEFAULT_ENGINE" >}}.

1. Restart the **{{< param "PRODUCT_NAME" >}}** service from the Windows Services manager.

Refer to [Service configuration][ConfigureWindows] for more information about the registry values {{< param "PRODUCT_NAME" >}} uses.

[RunLinux]: ../../run/linux/
[RunMacOS]: ../../run/macos/
[RunWindows]: ../../run/windows/
[ConfigureLinux]: ../../../configure/linux/#pass-additional-command-line-flags
[ConfigureMacOS]: ../../../configure/macos/#configure-environment-variables
[ConfigureWindows]: ../../install/windows/#service-configuration
