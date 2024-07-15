---
canonical: https://grafana.com/docs/alloy/latest/set-up/migrate/from-static/
aliases:
  - ../../tasks/migrate/from-static/ # /docs/alloy/latest/tasks/migrate/from-static/
description: Learn how to migrate your configuration from Grafana Agent Static to Grafana Alloy
menuTitle: Migrate from Agent Static
title: Migrate Grafana Agent Static to Grafana Alloy
weight: 100
---

# Migrate from Grafana Agent Static to {{% param "FULL_PRODUCT_NAME" %}}

The built-in {{< param "PRODUCT_NAME" >}} convert command can migrate your [Grafana Agent Static][Static] configuration to an {{< param "PRODUCT_NAME" >}} configuration.

This topic describes how to:

* Convert a Grafana Agent Static configuration to an {{< param "PRODUCT_NAME" >}} configuration.
* Run a Grafana Agent Static configuration natively using {{< param "PRODUCT_NAME" >}}.

## Components used in this topic

* [prometheus.scrape][]
* [prometheus.remote_write][]
* [local.file_match][]
* [loki.process][]
* [loki.source.file][]
* [loki.write][]
* [otelcol.receiver.otlp][]
* [otelcol.processor.batch][]
* [otelcol.exporter.otlp][]

## Before you begin

* You must have a Grafana Agent Static configuration.
* You must be familiar with the [Components][] concept in {{< param "PRODUCT_NAME" >}}.

## Convert a Grafana Agent Static configuration

To fully migrate Grafana Agent Static to {{< param "PRODUCT_NAME" >}}, you must convert your Grafana Agent Static configuration into an {{< param "PRODUCT_NAME" >}} configuration.
This conversion allows you to take full advantage of the many additional features available in {{< param "PRODUCT_NAME" >}}.

> In this task, you use the [convert][] CLI command to output an {{< param "PRODUCT_NAME" >}} configuration from a Grafana Agent Static configuration.

1. Open a terminal window and run the following command.

   ```shell
   alloy convert --source-format=static --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   Replace the following:

    * _`<INPUT_CONFIG_PATH>`_: The full path to the configuration file for Grafana Agent Static.
    * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

1. [Run][run alloy] {{< param "PRODUCT_NAME" >}} using the new {{< param "PRODUCT_NAME" >}} configuration from _`<OUTPUT_CONFIG_PATH>`_:

### Debugging

1. If the convert command can't convert a Grafana Agent Static configuration, diagnostic information is sent to `stderr`.
   You can use the `--bypass-errors` flag to bypass any non-critical issues and output the {{< param "PRODUCT_NAME" >}} configuration using a best-effort conversion.

   {{< admonition type="caution" >}}
   If you bypass the errors, the behavior of the converted configuration may not match the original Grafana Agent Static configuration.
   Make sure you fully test the converted configuration before using it in a production environment.
   {{< /admonition >}}

   ```shell
   alloy convert --source-format=static --bypass-errors --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   Replace the following:

   * _`<INPUT_CONFIG_PATH>`_: The full path to the configuration file for Grafana Agent Static.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

1. You can use the `--report` flag to output a diagnostic report.

   ```shell
   alloy convert --source-format=static --report=<OUTPUT_REPORT_PATH> --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
    ```

   Replace the following:

   * _`<INPUT_CONFIG_PATH>`_: The full path to the configuration file for Grafana Agent Static.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.
   * _`<OUTPUT_REPORT_PATH>`_: The output path for the report.

   Using the [example][] Grafana Agent Static configuration below, the diagnostic report provides the following information.

    ```plaintext
    (Warning) Please review your agent command line flags and ensure they are set in your {{< param "PRODUCT_NAME" >}} configuration file where necessary.
    ```

## Run a Grafana Agent Static mode configuration

If you’re not ready to completely switch to an {{< param "PRODUCT_NAME" >}} configuration, you can run {{< param "PRODUCT_NAME" >}} using your Grafana Agent Static configuration.
The `--config.format=static` flag tells {{< param "PRODUCT_NAME" >}} to convert your Grafana Agent Static configuration to {{< param "PRODUCT_NAME" >}} and load it directly without saving the configuration.
This allows you to try {{< param "PRODUCT_NAME" >}} without modifying your Grafana Agent Static configuration infrastructure.

> In this task, you use the [run][] CLI command to run {{< param "PRODUCT_NAME" >}} using a Grafana Agent Static configuration.

[Run][] {{< param "PRODUCT_NAME" >}} and include the command line flag `--config.format=static`.
Your configuration file must be a valid Grafana Agent Static configuration file.

### Debugging

1. Follow the convert CLI command [debugging][] instructions to generate a diagnostic report.

1. Refer to the {{< param "PRODUCT_NAME" >}} [debugging UI][UI] for more information about running {{< param "PRODUCT_NAME" >}}.

1. If your Grafana Agent Static configuration can't be converted and loaded directly into {{< param "PRODUCT_NAME" >}}, diagnostic information is sent to `stderr`.
   You can use the `--config.bypass-conversion-errors` flag with `--config.format=static` to bypass any non-critical issues and start {{< param "PRODUCT_NAME" >}}.

   {{< admonition type="caution" >}}
   If you bypass the errors, the behavior of the converted configuration may not match the original Grafana Agent Static configuration.
   Don't use this flag in a production environment.
   {{< /admonition >}}

## Example

This example demonstrates converting a Grafana Agent Static configuration file to an {{< param "PRODUCT_NAME" >}} configuration file.

The following Grafana Agent Static configuration file provides the input for the conversion.

```yaml
server:
  log_level: info

metrics:
  global:
    scrape_interval: 15s
    remote_write:
      - url: https://prometheus-us-central1.grafana.net/api/prom/push
        basic_auth:
          username: USERNAME
          password: PASSWORD
  configs:
    - name: test
      host_filter: false
      scrape_configs:
        - job_name: local-agent
          static_configs:
            - targets: ['127.0.0.1:12345']
              labels:
                cluster: 'localhost'

logs:
  global:
    file_watch_config:
      min_poll_frequency: 1s
      max_poll_frequency: 5s
  positions_directory: /var/lib/agent/data-agent
  configs:
    - name: varlogs
      scrape_configs:
        - job_name: varlogs
          static_configs:
            - targets:
              - localhost
              labels:
                job: varlogs
                host: mylocalhost
                __path__: /var/log/*.log
          pipeline_stages:
            - match:
                selector: '{filename="/var/log/*.log"}'
                stages:
                - drop:
                    expression: '^[^0-9]{4}'
                - regex:
                    expression: '^(?P<timestamp>\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[(?P<level>[[:alpha:]]+)\] (?:\d+)\#(?:\d+): \*(?:\d+) (?P<message>.+)$'
                - pack:
                    labels:
                      - level
      clients:
        - url: https://USER_ID:API_KEY@logs-prod3.grafana.net/loki/api/v1/push

traces:
  configs:
    - name: tempo
      receivers:
        otlp:
          protocols:
            grpc:
            http:
      batch:
        send_batch_size: 10000
        timeout: 20s
      remote_write:
        - endpoint: tempo-us-central1.grafana.net:443
          basic_auth:
            username: USERNAME
            password: PASSWORD
```

The convert command takes the YAML file as input and outputs a [{{< param "PRODUCT_NAME" >}} configuration][configuration] file.

```shell
alloy convert --source-format=static --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
```

Replace the following:

* _`<INPUT_CONFIG_PATH>`_: The full path to the configuration file for Grafana Agent Static.
* _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

The {{< param "PRODUCT_NAME" >}} configuration file looks like this:

```alloy
prometheus.scrape "metrics_test_local_agent" {
    targets = [{
        __address__ = "127.0.0.1:12345",
        cluster     = "localhost",
    }]
    forward_to      = [prometheus.remote_write.metrics_test.receiver]
    job_name        = "local-agent"
    scrape_interval = "15s"
}

prometheus.remote_write "metrics_test" {
    endpoint {
        name = "test-4dec64"
        url  = "https://prometheus-us-central1.grafana.net/api/prom/push"

        basic_auth {
            username = "<USERNAME>"
            password = "<PASSWORD>"
        }

        queue_config { }

        metadata_config { }
    }
}

local.file_match "logs_varlogs_varlogs" {
    path_targets = [{
        __address__ = "localhost",
        __path__    = "/var/log/*.log",
        host        = "mylocalhost",
        job         = "varlogs",
    }]
}

loki.process "logs_varlogs_varlogs" {
    forward_to = [loki.write.logs_varlogs.receiver]

    stage.match {
        selector = "{filename=\"/var/log/*.log\"}"

        stage.drop {
            expression = "^[^0-9]{4}"
        }

        stage.regex {
            expression = "^(?P<timestamp>\\d{4}/\\d{2}/\\d{2} \\d{2}:\\d{2}:\\d{2}) \\[(?P<level>[[:alpha:]]+)\\] (?:\\d+)\\#(?:\\d+): \\*(?:\\d+) (?P<message>.+)$"
        }

        stage.pack {
            labels           = ["level"]
            ingest_timestamp = false
        }
    }
}

loki.source.file "logs_varlogs_varlogs" {
    targets    = local.file_match.logs_varlogs_varlogs.targets
    forward_to = [loki.process.logs_varlogs_varlogs.receiver]

    file_watch {
        min_poll_frequency = "1s"
        max_poll_frequency = "5s"
    }
    legacy_positions_file = "/var/lib/agent/data-agent/varlogs.yml"
}

loki.write "logs_varlogs" {
    endpoint {
        url = "https://USER_ID:API_KEY@logs-prod3.grafana.net/loki/api/v1/push"
    }
    external_labels = {}
}

otelcol.receiver.otlp "default" {
    grpc {
        include_metadata = true
    }

    http {
        include_metadata = true
    }

    output {
        metrics = []
        logs    = []
        traces  = [otelcol.processor.batch.default.input]
    }
}

otelcol.processor.batch "default" {
    timeout         = "20s"
    send_batch_size = 10000

    output {
        metrics = []
        logs    = []
        traces  = [otelcol.exporter.otlp.default_0.input]
    }
}

otelcol.exporter.otlp "default_0" {
    retry_on_failure {
        max_elapsed_time = "1m0s"
    }

    client {
        endpoint = "tempo-us-central1.grafana.net:443"
        headers  = {
            authorization = "Basic VVNFUk5BTUU6UEFTU1dPUkQ=",
        }
    }
}
```

## Integrations Next

You can convert [integrations next][] configurations by adding the `extra-args` flag for [convert][] or `config.extra-args` for [run][].

```shell
alloy convert --source-format=static --extra-args="-enable-features=integrations-next" --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
```

 Replace the following:
   * _`<INPUT_CONFIG_PATH>`_: The full path to the configuration file for Grafana Agent Static.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

## Environment variables

You can use the `-config.expand-env` command line flag to interpret environment variables in your Grafana Agent Static configuration.
You can pass these flags to [convert][] with `--extra-args="-config.expand-env"` or to [run][] with `--config.extra-args="-config.expand-env"`.

> It's possible to combine `integrations-next` with `expand-env`.
> For [convert][], you can use `--extra-args="-enable-features=integrations-next -config.expand-env"`

## Limitations

Configuration conversion is done on a best-effort basis. {{< param "PRODUCT_NAME" >}} issues warnings or errors if the conversion can't be done.

After the configuration is converted, review the {{< param "PRODUCT_NAME" >}} configuration file and verify that it's correct before starting to use it in a production environment.

The following list is specific to the convert command and not {{< param "PRODUCT_NAME" >}}:

* The [Agent Management][] configuration options can't be automatically converted to {{< param "PRODUCT_NAME" >}}.
  Any additional unsupported features are returned as errors during conversion.
* There is no gRPC server to configure for {{< param "PRODUCT_NAME" >}}. Any non-default configuration shows as unsupported during the conversion.
* Check if you are using any extra command line arguments with Grafana Agent Static that aren't present in your configuration file. For example, `-server.http.address`.
* Check if you are using any environment variables in your Grafana Agent Static configuration.
  These are evaluated during conversion, and you may want to replace them with the {{< param "PRODUCT_NAME" >}} Standard library [env][] function after conversion.
* Review additional [Prometheus Limitations][] for limitations specific to your [Metrics][] configuration.
* Review additional [Promtail Limitations][] for limitations specific to your [Logs][] configuration.
* The logs produced by {{< param "PRODUCT_NAME" >}} differ from those produced by Grafana Agent Static.
* {{< param "PRODUCT_NAME" >}} exposes the {{< param "PRODUCT_NAME" >}} [UI][].

[debugging]: #debugging
[example]: #example
[Static]: https://grafana.com/docs/agent/latest/static
[prometheus.scrape]: ../../../reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: ../../../reference/components/prometheus/prometheus.remote_write/
[local.file_match]: ../../../reference/components/local/local.file_match/
[loki.process]: ../../../reference/components/loki/loki.process/
[loki.source.file]: ../../../reference/components/loki/loki.source.file/
[loki.write]: ../../../reference/components/loki/loki.write/
[Components]: ../../../get-started/components/
[convert]: ../../../reference/cli/convert/
[run]: ../../../reference/cli/run/
[run alloy]: ../../../set-up/run/
[UI]: ../../../troubleshoot/debug/
[configuration]: ../../../get-started/configuration-syntax/
[Integrations next]: https://grafana.com/docs/agent/latest/static/configuration/integrations/integrations-next/
[Agent Management]: https://grafana.com/docs/agent/latest/static/configuration/agent-management/
[env]: ../../../reference/stdlib/env/
[Prometheus Limitations]: ../from-prometheus/#limitations
[Promtail Limitations]: ../from-promtail/#limitations
[Metrics]: https://grafana.com/docs/agent/latest/static/configuration/metrics-config/
[Logs]: https://grafana.com/docs/agent/latest/static/configuration/logs-config/
[UI]: ../../../debug/#alloy-ui
[otelcol.receiver.otlp]: ../../../reference/components/otelcol/otelcol.receiver.otlp/
[otelcol.processor.batch]: ../../../reference/components/otelcol/otelcol.processor.batch/
[otelcol.exporter.otlp]:../../../reference/components/otelcol/otelcol.exporter.otlp/
