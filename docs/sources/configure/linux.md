---
canonical: https://grafana.com/docs/alloy/latest/configure/linux/
aliases:
  - ../tasks/configure/configure-linux/ # /docs/alloy/latest/tasks/configure/configure-linux/
description: Learn how to configure Grafana Alloy on Linux
menuTitle: Linux
title: Configure Grafana Alloy on Linux
weight: 300
---

# Configure {{% param "FULL_PRODUCT_NAME" %}} on Linux

To configure {{< param "PRODUCT_NAME" >}} on Linux, perform the following steps:

1. Edit the default configuration file at `/etc/alloy/config.alloy`.

1. Run the following command in a terminal to reload the configuration file:

   ```shell
   sudo systemctl reload alloy
   ```

To change the configuration file used by the service, perform the following steps:

1. Edit the environment file for the service:

   * Debian or Ubuntu: edit `/etc/default/alloy`
   * RHEL/Fedora or SUSE/openSUSE: edit `/etc/sysconfig/alloy`

1. Change the contents of the `CONFIG_FILE` environment variable to point at the new configuration file to use.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   sudo systemctl restart alloy
   ```

## Pass additional command-line flags

By default, the {{< param "PRODUCT_NAME" >}} service launches with the [run][] command, passing the following flags:

* `--storage.path=/var/lib/alloy`

To pass additional command-line flags to the {{< param "PRODUCT_NAME" >}} binary, perform the following steps:

1. Edit the environment file for the service:

   * Debian-based systems: edit `/etc/default/alloy`
   * RedHat or SUSE-based systems: edit `/etc/sysconfig/alloy`

1. Change the contents of the `CUSTOM_ARGS` environment variable to specify
   command-line flags to pass.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   sudo systemctl restart alloy
   ```

To see the list of valid command-line flags that can be passed to the service, refer to the documentation for the [run][] command.

## Expose the UI to other machines

By default, {{< param "PRODUCT_NAME" >}} listens on the local network for its HTTP server.
This prevents other machines on the network from being able to access the [UI for debugging][UI].

To expose the UI to other machines, complete the following steps:

1. Follow [Pass additional command-line flags](#pass-additional-command-line-flags)
   to edit command line flags passed to {{< param "PRODUCT_NAME" >}}

1. Add the following command line argument to `CUSTOM_ARGS`:

   ```shell
   --server.http.listen-addr=<LISTEN_ADDR>:12345
   ```

   Replace the following:

   * _`<LISTEN_ADDR>`_: An IP address which other machines on the network have access to.
     For example, the IP address of the machine {{< param "PRODUCT_NAME" >}} is running on.

     To listen on all interfaces, replace _`<LISTEN_ADDR>`_ with `0.0.0.0`.

[run]:../../reference/cli/run/
[UI]: ../../troubleshoot/debug/#alloy-ui
