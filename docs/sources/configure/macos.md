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
   brew services restart  grafana/grafana/alloy
   ```

## Configure the {{% param "PRODUCT_NAME" %}} service

{{< admonition type="note" >}}
Due to limitations in Homebrew, customizing the service used by {{< param "PRODUCT_NAME" >}} on macOS requires changing the Homebrew formula and reinstalling {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

To customize the {{< param "PRODUCT_NAME" >}} service on macOS, perform the following steps:

1. Run the following command in a terminal:

   ```shell
   brew edit grafana/grafana/alloy
   ```

   This opens the {{< param "PRODUCT_NAME" >}} Homebrew Formula in an editor.

1. Modify the `service` section as desired to change things such as:

   * Location of log files.

1. Modify the `COMMAND` in the `install` section as desired to change things such as:

   * The configuration file used by {{< param "PRODUCT_NAME" >}}.
   * Flags passed to the {{< param "PRODUCT_NAME" >}} binary.

1. Save the modified file.

1. Reinstall the {{< param "PRODUCT_NAME" >}} Formula by running the following command in a terminal:

   ```shell
   brew reinstall --formula  grafana/grafana/alloy
   ```

1. Restart the {{< param "PRODUCT_NAME" >}} service by running the command in a terminal:

   ```shell
   brew services restart  grafana/grafana/alloy
   ```

## Configure environment variables

You can use [environment variables][env_vars] to control the run-time behavior of {{< param "PRODUCT_NAME" >}}.
These environment variables are set in `$(brew --prefix)/etc/alloy/config.env`

To add the environment variables:

1. Edit the file at `$(brew --prefix)/etc/alloy/config.env`.
1. Add the specific environment variables you need.
1. Restart {{< param "PRODUCT_NAME" >}}.

For example, you can add the following environment variables to `$(brew --prefix)/etc/alloy/config.env`:

```shell
export GCLOUD_RW_API_KEY="glc_xxx"
export GCLOUD_FM_COLLECTOR_ID="my-collector"
export GCLOUD_FM_LOG_PATH="/opt/homebrew/var/log/alloy.err.log"
```

## Configure command line flags

You can use the file at `$(brew --prefix)/etc/alloy/extra-args.txt` to pass multiple [command line flags][flags] to {{< param "PRODUCT_NAME" >}}.

To add the command line flags:

1. Edit the file at `$(brew --prefix)/etc/alloy/extra-args.txt`.
1. Add the specific flags you need.
1. Restart {{< param "PRODUCT_NAME" >}}.

For example, you can add the following command line flag in `$(brew --prefix)/etc/alloy/extra-args.txt` to enable the experimental components in {{< param "PRODUCT_NAME" >}}.

```shell
--stability.level=experimental
```

## Expose the UI to other machines

By default, {{< param "PRODUCT_NAME" >}} listens on the local network for its HTTP server.
This prevents other machines on the network from being able to access the [UI for debugging][UI].

To expose the UI to other machines, complete the following steps:

1. Follow [Configure the {{< param "PRODUCT_NAME" >}} service](#configure-the-alloy-service) steps to edit command line flags passed to {{< param "PRODUCT_NAME" >}}.

1. Modify the `COMMAND` line in the `install` section containing `--server.http.listen-addr=127.0.0.1:12345`, and replace `127.0.0.1` with the IP address that other machines on the network have access to.
   For example, the IP address of the machine {{< param "PRODUCT_NAME" >}} is running on.

   To listen on all interfaces, replace `127.0.0.1` with `0.0.0.0`.

[UI]: ../../troubleshoot/debug/#alloy-ui
[env_vars]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/environment-variables/
[flags]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/
