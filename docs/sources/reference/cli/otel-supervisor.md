---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/otel-supervisor/
description: Learn about the otel-supervisor command
labels:
  stage: experimental
  products:
    - oss
title: otel-supervisor
weight: 360
---

# `otel-supervisor`

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `otel-supervisor` command runs {{< param "PRODUCT_NAME" >}} with the {{< param "OTEL_ENGINE" >}} under an embedded OpAMP supervisor.
The supervisor connects to Grafana Fleet Management and receives the {{< param "OTEL_ENGINE" >}} configuration through OpAMP.

## Usage

The `otel-supervisor` command supports simple and manual modes.

Simple mode helps you get started quickly with Grafana Fleet Management.
Use manual mode to connect to an OpAMP server other than Grafana Fleet Management.

## Run in simple mode

Simple mode uses environment variables to configure the supervisor.
To run it, use the following command:

```shell
alloy otel-supervisor
```

To configure simple mode, set the following environment variables:

* `GCLOUD_FM_URL`: The Grafana Fleet Management base URL. The command adds `/v1/opamp` when the URL doesn't include it.
* `GCLOUD_INSTANCE_ID`: Your Grafana Cloud instance ID.
* `GCLOUD_RW_API_KEY`: Your Grafana Cloud API token.
* `STORAGE_DIR`: A directory where the supervisor stores its data.

Set the variables before you start the supervisor. The following shell example uses placeholder values:

```shell
export GCLOUD_FM_URL="<FLEET_MANAGEMENT_URL>"
export GCLOUD_INSTANCE_ID="<GRAFANA_CLOUD_INSTANCE_ID>"
export GCLOUD_RW_API_KEY="<GRAFANA_CLOUD_API_TOKEN>"
export STORAGE_DIR="<SUPERVISOR_STORAGE_DIRECTORY>"

alloy otel-supervisor
```

For information about setting up Fleet Management, refer to the [Grafana Fleet Management documentation](https://grafana.com/docs/grafana-cloud/send-data/fleet-management/).

## Run in manual mode

Manual mode uses a supervisor configuration file.
To run it, use the following command:

```shell
alloy otel-supervisor --config=<SUPERVISOR_CONFIG_FILE>
```

Replace _`<SUPERVISOR_CONFIG_FILE>`_ with the path to an [OpenTelemetry Collector OpAMP supervisor configuration file](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/cmd/opampsupervisor/specification/README.md).

Alloy always sets `agent.executable` to the running Alloy binary.
It ignores any `agent.executable` value in the supervisor configuration file.
You can set `agent.args` to change the arguments passed to the Alloy binary.

### Configure an OpAMP connection

The following configuration connects the supervisor to Grafana Fleet Management:

```yaml
server:
  endpoint: "${env:GCLOUD_FM_URL}/v1/opamp"
  headers:
    Authorization: "Basic ${env:GCLOUD_BASIC_AUTH_BASE64}"
capabilities:
  accepts_remote_config: true
  reports_remote_config: true
storage:
  directory: ${env:STORAGE_DIR}
```

Set `GCLOUD_BASIC_AUTH_BASE64` to the base64-encoded Grafana Cloud instance ID and API token, separated by a colon.

### Override Alloy arguments

To pass different arguments to the Alloy binary, add an `agent` block to the supervisor configuration file:

```yaml
agent:
  args: [otel, --feature-gates, +service.profilesSupport]
```
