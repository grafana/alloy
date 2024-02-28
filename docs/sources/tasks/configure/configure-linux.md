---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/configure/configure-linux/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/configure/configure-linux/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/configure/configure-linux/
- /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-linux/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/setup/configure/configure-linux/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/configure/configure-linux/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/configure/configure-linux/
- /docs/grafana-cloud/send-data/agent/flow/setup/configure/configure-linux/
- ../../setup/configure/configure-linux/ # /docs/agent/latest/flow/setup/configure/configure-linux/
canonical: https://grafana.com/docs/agent/latest/flow/tasks/configure/configure-linux/
description: Learn how to configure Grafana Agent Flow on Linux
menuTitle: Linux
title: Configure Grafana Agent Flow on Linux
weight: 300
---

# Configure {{% param "PRODUCT_NAME" %}} on Linux

To configure {{< param "PRODUCT_NAME" >}} on Linux, perform the following steps:

1. Edit the default configuration file at `/etc/grafana-agent-flow.river`.

1. Run the following command in a terminal to reload the configuration file:

   ```shell
   sudo systemctl reload grafana-agent-flow
   ```

To change the configuration file used by the service, perform the following steps:

1. Edit the environment file for the service:

   * Debian or Ubuntu: edit `/etc/default/grafana-agent-flow`
   * RHEL/Fedora or SUSE/openSUSE: edit `/etc/sysconfig/grafana-agent-flow`

1. Change the contents of the `CONFIG_FILE` environment variable to point to
   the new configuration file to use.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   sudo systemctl restart grafana-agent-flow
   ```

## Pass additional command-line flags

By default, the {{< param "PRODUCT_NAME" >}} service launches with the [run][]
command, passing the following flags:

* `--storage.path=/var/lib/grafana-agent-flow`

To pass additional command-line flags to the {{< param "PRODUCT_NAME" >}} binary, perform
the following steps:

1. Edit the environment file for the service:

   * Debian-based systems: edit `/etc/default/grafana-agent-flow`
   * RedHat or SUSE-based systems: edit `/etc/sysconfig/grafana-agent-flow`

1. Change the contents of the `CUSTOM_ARGS` environment variable to specify
   command-line flags to pass.

1. Restart the {{< param "PRODUCT_NAME" >}} service:

   ```shell
   sudo systemctl restart grafana-agent-flow
   ```

To see the list of valid command-line flags that can be passed to the service,
refer to the documentation for the [run][] command.

## Expose the UI to other machines

By default, {{< param "PRODUCT_NAME" >}} listens on the local network for its HTTP
server. This prevents other machines on the network from being able to access
the [UI for debugging][UI].

To expose the UI to other machines, complete the following steps:

1. Follow [Pass additional command-line flags](#pass-additional-command-line-flags)
   to edit command line flags passed to {{< param "PRODUCT_NAME" >}}, including the
   following customizations:

    1. Add the following command line argument to `CUSTOM_ARGS`:

       ```shell
       --server.http.listen-addr=LISTEN_ADDR:12345
       ```

       Replace `LISTEN_ADDR` with an address which other machines on the
       network have access to, like the network IP address of the machine
       {{< param "PRODUCT_NAME" >}} is running on.

       To listen on all interfaces, replace `LISTEN_ADDR` with `0.0.0.0`.

{{% docs/reference %}}
[run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/run.md"
[run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/run.md"
[UI]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md#grafana-agent-flow-ui"
[UI]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md#grafana-agent-flow-ui"
{{% /docs/reference %}}
