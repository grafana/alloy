---
aliases:
- /docs/grafana-cloud/agent/flow/get-started/install/binary/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/get-started/install/binary/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/get-started/install/binary/
- /docs/grafana-cloud/send-data/agent/flow/get-started/install/binary/
# Previous docs aliases for backwards compatibility:
- ../../install/binary/ # /docs/agent/latest/flow/install/binary/
- /docs/grafana-cloud/agent/flow/setup/install/binary/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/install/binary/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/install/binary/
- /docs/grafana-cloud/send-data/agent/flow/setup/install/binary/
- ../../setup/install/binary/ # /docs/agent/latest/flow/setup/install/binary/
canonical: https://grafana.com/docs/agent/latest/flow/get-started/install/binary/
description: Learn how to install Grafana Agent Flow as a standalone binary
menuTitle: Standalone
title: Install Grafana Agent Flow as a standalone binary
weight: 600
---

# Install {{% param "PRODUCT_NAME" %}} as a standalone binary

{{< param "PRODUCT_NAME" >}} is distributed as a standalone binary for the following operating systems and architectures:

* Linux: AMD64, ARM64
* Windows: AMD64
* macOS: AMD64 (Intel), ARM64 (Apple Silicon)
* FreeBSD: AMD64

## Download {{% param "PRODUCT_ROOT_NAME" %}}

To download {{< param "PRODUCT_NAME" >}} as a standalone binary, perform the following steps.

1. Navigate to the current {{< param "PRODUCT_ROOT_NAME" >}} [release](https://github.com/grafana/agent/releases) page.

1. Scroll down to the **Assets** section.

1. Download the `grafana-agent` zip file that matches your operating system and machine's architecture.

1. Extract the package contents into a directory.

1. If you are installing {{< param "PRODUCT_NAME" >}} on Linux, macOS, or FreeBSD, run the following command in a terminal:

   ```shell
   chmod +x <BINARY_PATH>
   ```

   Replace the following:

   - _`<BINARY_PATH>`_: The path to the extracted binary.

## Next steps

- [Run {{< param "PRODUCT_NAME" >}}][Run]

{{% docs/reference %}}
[Run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/run/binary.md"
[Run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/get-started/run/binary.md"
{{% /docs/reference %}}
