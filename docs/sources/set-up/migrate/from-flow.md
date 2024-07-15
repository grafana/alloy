---
canonical: https://grafana.com/docs/alloy/latest/set-up/migrate/from-flow/
aliases:
  - ../../tasks/migrate/from-flow/ # /docs/alloy/latest/tasks/migrate/from-flow/
description: Learn how to migrate your configuration from Grafana Agent Flow to Grafana Alloy
menuTitle: Migrate from Agent Flow
title: Migrate Grafana Agent Flow to Grafana Alloy
weight: 140
---

# Migrate from Grafana Agent Flow to {{% param "FULL_PRODUCT_NAME" %}}

This topic describes how to perform a live migration from Grafana Agent Flow to {{< param "FULL_PRODUCT_NAME" >}} with minimal downtime.

{{< admonition type="note" >}}
This procedure is only required for live migrations from a Grafana Agent Flow install to {{< param "PRODUCT_NAME" >}}.

If you want a fresh start with {{< param "PRODUCT_NAME" >}}, you can [uninstall Grafana Agent Flow][uninstall] and [install {{< param "PRODUCT_NAME" >}}][install].

[uninstall]: https://grafana.com/docs/agent/latest/flow/get-started/install/
[install]: ../../../set-up/install/
{{< /admonition >}}

## Before you begin

* You must have a Grafana Agent Flow configuration to migrate.
* You must be running Grafana Agent Flow version v0.40 or later.
* If you use auto-scaling make sure you disable auto-scaling for your Grafana Agent Flow deployment to prevent it from scaling during the migration.

## Differences between Grafana Agent Flow and {{% param "PRODUCT_NAME" %}}

* Only functionality marked _Generally available_ may be used by default.
You can enable functionality in _Experimental_ and _Public preview_ by setting the `--stability.level` flag in [run].
* The default value of `--storage.path` has changed from `data-agent/` to `data-alloy/`.
* The default value of `--server.http.memory-addr` has changed from `agent.internal:12345` to `alloy.internal:12345`.
* Debug metrics reported by {{% param "PRODUCT_NAME" %}} are prefixed with `alloy_` instead of `agent_`.
* The "classic modules", `module.file`, `module.git`, `module.http`, and `module.string` have been removed in favor of import configuration blocks.
* The `prometheus.exporter.vsphere` component has been replaced by the `otelcol.receiver.vcenter` component.

[run]: ../../../reference/cli/run

## Steps

### Prepare your Grafana Agent Flow configuration

{{< param "PRODUCT_NAME" >}} uses the same configuration format as Grafana Agent Flow, but some functionality has been removed.

Before migrating, modify your Grafana Agent Flow configuration to remove or replace any unsupported components:

* The "classic modules" in Grafana Agent Flow have been removed in favor of the modules introduced in v0.40:
    * `module.file` is replaced by the [import.file] configuration block.
    * `module.git` is replaced by the [import.git] configuration block.
    * `module.http` is replaced by the [import.http] configuration block.
    * `module.string` is replaced by the [import.string] configuration block.
* `prometheus.exporter.vsphere` is replaced by the [otelcol.receiver.vcenter] component.

[import.file]: ../../../reference/config-blocks/import.file/
[import.git]: ../../../reference/config-blocks/import.git/
[import.http]: ../../../reference/config-blocks/import.http/
[import.string]: ../../../reference/config-blocks/import.string/
[otelcol.receiver.vcenter]: ../../../reference/components/otelcol/otelcol.receiver.vcenter/

### Deploy {{% param "PRODUCT_NAME" %}} with a default configuration

Follow the [installation instructions][install] for {{< param "PRODUCT_NAME" >}}, using the default configuration file. The configuration file is customized in the following steps.

When deploying {{< param "PRODUCT_NAME" >}}, be aware of the following settings:

- {{< param "PRODUCT_NAME" >}} should be deployed with topology that's the same as Grafana Agent Flow.
  The CPU, and storage limits should match.
- Custom command-line flags configured in Grafana Agent Flow should be reflected in your {{< param "PRODUCT_NAME" >}} installation.
- {{< param "PRODUCT_NAME" >}} may need to be deployed with the `--stability.level` flag in [run] to enable non-stable components:
    - Set `--stability.level` to `experimental` if you are using the following component:
        - [otelcol.receiver.vcenter]
    - Otherwise, `--stability.level` may be omitted or set to the default value (`generally-available`).
- When installing on Kubernetes, update your `values.yaml` file to rename the `agent` key to `alloy`.
- If you are deploying {{< param "PRODUCT_NAME" >}} as a cluster:
    - Set the number of instances to match the number of instances in your Grafana Agent Flow cluster.
    - Don't enable auto-scaling until the migration is complete.

[install]: ../../../set-up/install
[run]: ../../../reference/cli/run
[discovery.process]: ../../../reference/components/discovery.process/
[pyroscope.ebpf]: ../../../reference/components/pyroscope.ebpf/
[pyroscope.java]: ../../../reference/components/pyroscope.java/
[pyroscope.scrape]: ../../../reference/components/pyroscope.scrape/
[pyroscope.write]: ../../../reference/components/pyroscope.write/
[otelcol.receiver.vcenter]: ../../../reference/components/otelcol/otelcol.receiver.vcenter/

### Migrate Grafana Agent Flow data to {{% param "PRODUCT_NAME" %}}

Migrate your Grafana Agent Flow data to {{< param "PRODUCT_NAME" >}} by copying the contents of the Grafana Agent Flow data directory to the {{< param "PRODUCT_NAME" >}} data directory.

* Linux installations: copy the _contents_ of `/var/lib/grafana-agent-flow` to `/var/lib/alloy/data`.
* macOS installations: copy the _contents_ of `$(brew --prefix)/etc/grafana-agent-flow/data` to `$(brew --prefix)/etc/alloy/data`.
* Windows installations: copy the _contents_ of `%ProgramData%\Grafana Agent Flow\data` to `%ProgramData%\GrafanaLabs\Alloy\data`.
* Docker: copy the contents of mounted volumes to a new directory, and then mount the new directory when running {{% param "PRODUCT_NAME" %}}.
* Kubernetes: use `kubectl cp` to copy the _contents_ of the data directory on Flow pods to the data directory on {{% param "PRODUCT_NAME" %}} pods.
    * The data directory is determined by the `agent.storagePath` (default `/tmp/agent`) and `alloy.storagePath` (default `/tmp/alloy`) fields in `values.yaml`.

### Migrate pipelines that receive data over the network

Telemetry pipelines which receive data over the network (for example, pipelines using `otelcol.receiver.otlp` or `prometheus.receive_http`) should be configured in {{< param "PRODUCT_NAME" >}} first:

1. Configure {{< param "PRODUCT_NAME" >}} with all pipelines where telemetry data is received over the network.
1. Reconfigure applications to send telemetry data to {{< param "PRODUCT_NAME" >}} instead of Grafana Agent Flow.

### Migrate the remaining pipelines

Migrate remaining pipelines from Grafana Agent Flow to {{% param "PRODUCT_NAME" %}}:

1. Disable remaining pipelines in Grafana Agent Flow to prevent Flow and {{< param "PRODUCT_NAME" >}} from processing the same data.
2. Configure {{< param "PRODUCT_NAME" >}} with the remaining pipelines.

{{< admonition type="note" >}}
This process results in minimal downtime as remaining pipelines are moved from Grafana Agent Flow to {{< param "PRODUCT_NAME" >}}.

To completely eliminate downtime, configure remaining pipelines in {{< param "PRODUCT_NAME" >}} before disabling them in Grafana Agent Flow.
This alternative approach results in some duplicate data being sent to backends during the migration period.
{{< /admonition >}}

### Uninstall Grafana Agent Flow

After you have completed the migration, you can uninstall Grafana Agent Flow.

### Cleanup temporary changes

You can enable auto-scaling in your {{< param "PRODUCT_NAME" >}} deployment if you disabled it during the migration process.
