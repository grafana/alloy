---
aliases:
- /docs/grafana-cloud/agent/flow/get-started/install/docker/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/install/docker/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/install/docker/
- /docs/grafana-cloud/send-data/agent/flow/get-started/install/docker/
# Previous docs aliases for backwards compatibility:
- ../../install/docker/ # /docs/agent/latest/flow/install/docker/
- /docs/grafana-cloud/agent/flow/setup/install/docker/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/install/docker/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/install/docker/
- /docs/grafana-cloud/send-data/agent/flow/setup/install/docker/
- ../../setup/install/docker/ # /docs/agent/latest/flow/setup/install/docker/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/install/docker/
description: Learn how to install Grafana Agent Flow on Docker
menuTitle: Docker
title: Run Grafana Agent Flow in a Docker container
weight: 100
---

# Run {{% param "PRODUCT_NAME" %}} in a Docker container

{{< param "PRODUCT_NAME" >}} is available as a Docker container image on the following platforms:

* [Linux containers][] for AMD64 and ARM64.
* [Windows containers][] for AMD64.

## Before you begin

* Install [Docker][] on your computer.
* Create and save a {{< param "PRODUCT_NAME" >}} River configuration file on your computer, for example:

  ```river
  logging {
    level  = "info"
    format = "logfmt"
  }
  ```

## Run a Linux Docker container

To run {{< param "PRODUCT_NAME" >}} as a Linux Docker container, run the following command in a terminal window:

```shell
docker run \
  -e AGENT_MODE=flow \
  -v <CONFIG_FILE_PATH>:/etc/agent/config.river \
  -p 12345:12345 \
  grafana/agent:latest \
    run --server.http.listen-addr=0.0.0.0:12345 /etc/agent/config.river
```

Replace the following:

- _`<CONFIG_FILE_PATH>`_: The path of the configuration file on your host system.

You can modify the last line to change the arguments passed to the {{< param "PRODUCT_NAME" >}} binary.
Refer to the documentation for [run][] for more information about the options available to the `run` command.

{{< admonition type="note" >}}
Make sure you pass `--server.http.listen-addr=0.0.0.0:12345` as an argument as shown in the example above.
If you don't pass this argument, the [debugging UI][UI] won't be available outside of the Docker container.
{{< /admonition >}}

## Run a Windows Docker container

To run {{< param "PRODUCT_NAME" >}} as a Windows Docker container, run the following command in a terminal window:

```shell
docker run \
  -e AGENT_MODE=flow \
  -v <CONFIG_FILE_PATH>:C:\etc\grafana-agent\config.river \
  -p 12345:12345 \
  grafana/agent:latest-windows \
    run --server.http.listen-addr=0.0.0.0:12345 C:\etc\grafana-agent\config.river
```

Replace the following:

- _`<CONFIG_FILE_PATH>`_: The path of the configuration file on your host system.

You can modify the last line to change the arguments passed to the {{< param "PRODUCT_NAME" >}} binary.
Refer to the documentation for [run][] for more information about the options available to the `run` command.

{{< admonition type="note" >}}
Make sure you pass `--server.http.listen-addr=0.0.0.0:12345` as an argument as shown in the example above.
If you don't pass this argument, the [debugging UI][UI] won't be available outside of the Docker container.
{{< /admonition >}}

## Verify

To verify that {{< param "PRODUCT_NAME" >}} is running successfully, navigate to <http://localhost:12345> and make sure the {{< param "PRODUCT_NAME" >}} [UI][] loads without error.

[Linux containers]: #run-a-linux-docker-container
[Windows containers]: #run-a-windows-docker-container
[Docker]: https://docker.io

{{% docs/reference %}}
[run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/run.md"
[run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/run.md"
[UI]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md#grafana-agent-flow-ui"
[UI]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md#grafana-agent-flow-ui"
{{% /docs/reference %}}
