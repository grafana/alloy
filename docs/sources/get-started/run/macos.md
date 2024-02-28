---
aliases:
  - /docs/grafana-cloud/agent/flow/get-started/run/macos/
  - /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/run/macos/
  - /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/run/macos/
  - /docs/grafana-cloud/send-data/agent/flow/get-started/run/macos/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/run/macos/
description: Learn how to run Grafana Agent Flow on macOS
menuTitle: macOS
title: Run Grafana Agent Flow on macOS
weight: 400
---

# Run {{% param "PRODUCT_NAME" %}} on macOS

{{< param "PRODUCT_NAME" >}} is [installed][InstallMacOS] as a launchd service on macOS.

## Start {{% param "PRODUCT_NAME" %}}

To start {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
brew services start grafana-agent-flow
```

{{< param "PRODUCT_NAME" >}} automatically runs when the system starts.

(Optional) To verify that the service is running, run the following command in a terminal window:

```shell
brew services info grafana-agent-flow
```

## Restart {{% param "PRODUCT_NAME" %}}

To restart {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
brew services restart grafana-agent-flow
```

## Stop {{% param "PRODUCT_NAME" %}}

To stop {{< param "PRODUCT_NAME" >}}, run the following command in a terminal window:

```shell
brew services stop grafana-agent-flow
```

## View {{% param "PRODUCT_NAME" %}} logs on macOS

By default, logs are written to `$(brew --prefix)/var/log/grafana-agent-flow.log` and
`$(brew --prefix)/var/log/grafana-agent-flow.err.log`.

If you followed [Configure the {{< param "PRODUCT_NAME" >}} service][ConfigureService] and changed the path where logs are written,
refer to your current copy of the {{< param "PRODUCT_NAME" >}} formula to locate your log files.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][ConfigureMacOS]

{{% docs/reference %}}
[InstallMacOS]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/install/macos.md"
[InstallMacOS]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/flow/get-started/install/macos.md"
[ConfigureMacOS]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure/configure-macos.md"
[ConfigureMacOS]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-macos.md"
[ConfigureService]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/configure/configure-macos.md#configure-the-grafana-agent-flow-service"
[ConfigureService]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-macos.md#configure-the-grafana-agent-flow-service"
{{% /docs/reference %}}
