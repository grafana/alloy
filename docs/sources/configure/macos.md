---
canonical: https://grafana.com/docs/alloy/latest/configure/macos/
aliases:
  - ../tasks/configure/configure-macos/ # /docs/alloy/latest/tasks/configure/configure-macos/
description: Learn how to configure Grafana Alloy on macOS
menuTitle: macOS
title: Configure Grafana Alloy on macOS
weight: 400
---

# Configure {{% param "FULL_PRODUCT_NAME" %}} on macOS

To configure {{< param "PRODUCT_NAME" >}} on macOS, perform the following steps:

1. Edit the default configuration file at `$(brew --prefix)/etc/alloy/config.alloy`.

1. Run the following command in a terminal to restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   brew services restart alloy
   ```

## Configure the {{% param "PRODUCT_NAME" %}} service

{{< admonition type="note" >}}
Due to limitations in Homebrew, customizing the service used by {{< param "PRODUCT_NAME" >}} on macOS requires changing the Homebrew formula and reinstalling {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

To customize the {{< param "PRODUCT_NAME" >}} service on macOS, perform the following steps:

1. Run the following command in a terminal:

   ```shell
   brew edit alloy
   ```

   This will open the {{< param "PRODUCT_NAME" >}} Homebrew Formula in an editor.

1. Modify the `service` section as desired to change things such as:

   * The configuration file used by {{< param "PRODUCT_NAME" >}}.
   * Flags passed to the {{< param "PRODUCT_NAME" >}} binary.
   * Location of log files.

   When you are done, save the file.

1. Reinstall the {{< param "PRODUCT_NAME" >}} Formula by running the following command in a terminal:

   ```shell
   brew reinstall alloy
   ```

1. Restart the {{< param "PRODUCT_NAME" >}} service by running the command in a terminal:

   ```shell
   brew services restart alloy
   ```

## Expose the UI to other machines

By default, {{< param "PRODUCT_NAME" >}} listens on the local network for its HTTP server.
This prevents other machines on the network from being able to access the [UI for debugging][UI].

To expose the UI to other machines, complete the following steps:

1. Follow [Configure the {{< param "PRODUCT_NAME" >}} service](#configure-the-alloy-service)
   to edit command line flags passed to {{< param "PRODUCT_NAME" >}}.

1. Modify the line inside the `service` block containing `--server.http.listen-addr=127.0.0.1:12345`, replacing `127.0.0.1` with the IP address which other machines on the network have access to.
   For example, the IP address of the machine {{< param "PRODUCT_NAME" >}} is running on.

   To listen on all interfaces, replace `127.0.0.1` with `0.0.0.0`.

[UI]: ../../troubleshoot/debug/#alloy-ui
