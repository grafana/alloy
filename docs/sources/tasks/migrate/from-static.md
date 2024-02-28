---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/migrate/from-static/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/migrate/from-static/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/migrate/from-static/
- /docs/grafana-cloud/send-data/agent/flow/tasks/migrate/from-static/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/getting-started/migrating-from-static/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/getting-started/migrating-from-static/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/getting-started/migrating-from-static/
- /docs/grafana-cloud/send-data/agent/flow/getting-started/migrating-from-static/
- ../../getting-started/migrating-from-static/ # /docs/agent/latest/flow/getting-started/migrating-from-static/
canonical: https://grafana.com/docs/agent/latest/flow/tasks/migrate/from-static/
description: Learn how to migrate your configuration from Grafana Agent Static to Grafana Agent Flow
menuTitle: Migrate from Static to Flow
title: Migrate Grafana Agent Static to Grafana Agent Flow
weight: 340
---

# Migrate from {{% param "PRODUCT_ROOT_NAME" %}} Static to {{% param "PRODUCT_NAME" %}}

The built-in {{< param "PRODUCT_ROOT_NAME" >}} convert command can migrate your [Static][] configuration to a {{< param "PRODUCT_NAME" >}} configuration.

This topic describes how to:

* Convert a Grafana Agent Static configuration to a {{< param "PRODUCT_NAME" >}} configuration.
* Run a Grafana Agent Static configuration natively using {{< param "PRODUCT_NAME" >}}.

## Components used in this topic

* [prometheus.scrape][]
* [prometheus.remote_write][]
* [local.file_match][]
* [loki.process][]
* [loki.source.file][]
* [loki.write][]

## Before you begin

* You must have an existing Grafana Agent Static configuration.
* You must be familiar with the [Components][] concept in {{< param "PRODUCT_NAME" >}}.

## Convert a Grafana Agent Static configuration

To fully migrate Grafana Agent [Static][] to {{< param "PRODUCT_NAME" >}}, you must convert your Static configuration into a {{< param "PRODUCT_NAME" >}} configuration.
This conversion will enable you to take full advantage of the many additional features available in {{< param "PRODUCT_NAME" >}}.

> In this task, you will use the [convert][] CLI command to output a {{< param "PRODUCT_NAME" >}}
> configuration from a Static configuration.

1. Open a terminal window and run the following command.

   {{< code >}}

   ```static-binary
   AGENT_MODE=flow grafana-agent convert --source-format=static --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   ```flow-binary
   grafana-agent-flow convert --source-format=static --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   {{< /code >}}

   Replace the following:

    * _`<INPUT_CONFIG_PATH>`_: The full path to the [Static][] configuration.
    * _`<OUTPUT_CONFIG_PATH>_`: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

1. [Run][] {{< param "PRODUCT_NAME" >}} using the new {{< param "PRODUCT_NAME" >}} configuration from _`<OUTPUT_CONFIG_PATH>`_:

### Debugging

1. If the convert command can't convert a [Static][] configuration, diagnostic information is sent to `stderr`.
   You can use the `--bypass-errors` flag to bypass any non-critical issues and output the {{< param "PRODUCT_NAME" >}} configuration using a best-effort conversion.

   {{< admonition type="caution" >}}
   If you bypass the errors, the behavior of the converted configuration may not match the original Grafana Agent Static configuration.
   Make sure you fully test the converted configuration before using it in a production environment.
   {{< /admonition >}}

   {{< code >}}

   ```static-binary
   AGENT_MODE=flow grafana-agent convert --source-format=static --bypass-errors --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   ```flow-binary
   grafana-agent-flow convert --source-format=static --bypass-errors --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   {{< /code >}}

   Replace the following:

   * _`<INPUT_CONFIG_PATH>`_: The full path to the [Static][] configuration.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

1. You can use the `--report` flag to output a diagnostic report.

   {{< code >}}

   ```static-binary
   AGENT_MODE=flow grafana-agent convert --source-format=static --report=<OUTPUT_REPORT_PATH> --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   ```flow-binary
   grafana-agent-flow convert --source-format=static --report=<OUTPUT_REPORT_PATH> --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
    ```

   {{< /code >}}

   Replace the following:

   * _`<INPUT_CONFIG_PATH>`_: The full path to the [Static][] configuration.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.
   * _`<OUTPUT_REPORT_PATH>`_: The output path for the report.

   Using the [example][] Grafana Agent Static configuration below, the diagnostic report provides the following information.

    ```plaintext
    (Warning) Please review your agent command line flags and ensure they are set in your {{< param "PRODUCT_NAME" >}} configuration file where necessary.
    ```

## Run a Static mode configuration

If you’re not ready to completely switch to a {{< param "PRODUCT_NAME" >}} configuration, you can run {{< param "PRODUCT_ROOT_NAME" >}} using your existing Grafana Agent Static configuration.
The `--config.format=static` flag tells {{< param "PRODUCT_ROOT_NAME" >}} to convert your [Static] configuration to {{< param "PRODUCT_NAME" >}} and load it directly without saving the new configuration.
This allows you to try {{< param "PRODUCT_NAME" >}} without modifying your existing Grafana Agent Static configuration infrastructure.

> In this task, you will use the [run][] CLI command to run {{< param "PRODUCT_NAME" >}} using a Static configuration.

[Run][] {{< param "PRODUCT_NAME" >}} and include the command line flag `--config.format=static`.
Your configuration file must be a valid [Static] configuration file.

### Debugging

1. You can follow the convert CLI command [debugging][] instructions to generate a diagnostic report.

1. Refer to the {{< param "PRODUCT_NAME" >}} [debugging UI][DebuggingUI] for more information about running {{< param "PRODUCT_NAME" >}}.

1. If your [Static] configuration can't be converted and loaded directly into {{< param "PRODUCT_NAME" >}}, diagnostic information is sent to `stderr`.
   You can use the `--config.bypass-conversion-errors` flag with `--config.format=static` to bypass any non-critical issues and start {{< param "PRODUCT_NAME" >}}.

   {{< admonition type="caution" >}}
   If you bypass the errors, the behavior of the converted configuration may not match the original Grafana Agent Static configuration.
   Do not use this flag in a production environment.
   {{< /admonition >}}

## Example

This example demonstrates converting a [Static] configuration file to a {{< param "PRODUCT_NAME" >}} configuration file.

The following [Static] configuration file provides the input for the conversion.

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
```

The convert command takes the YAML file as input and outputs a [River][] file.

{{< code >}}

```static-binary
AGENT_MODE=flow grafana-agent convert --source-format=static --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
```

```flow-binary
grafana-agent-flow convert --source-format=static --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
```

{{< /code >}}

Replace the following:

* _`<INPUT_CONFIG_PATH>`_: The full path to the [Static][] configuration.
* _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

The new {{< param "PRODUCT_NAME" >}} configuration file looks like this:

```river
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
		name = "test-3a2a1b"
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
}

loki.write "logs_varlogs" {
	endpoint {
		url = "https://USER_ID:API_KEY@logs-prod3.grafana.net/loki/api/v1/push"
	}
	external_labels = {}
}
```

## Integrations Next

You can convert [integrations next][] configurations by adding the `extra-args` flag for [convert][] or `config.extra-args` for [run][].

{{< code >}}

```static-binary
AGENT_MODE=flow grafana-agent convert --source-format=static --extra-args="-enable-features=integrations-next" --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
```

```flow-binary
grafana-agent-flow convert --source-format=static --extra-args="-enable-features=integrations-next" --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
```

{{< /code >}}

 Replace the following:
   * _`<INPUT_CONFIG_PATH>`_: The full path to the [Static][] configuration.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.
   
## Environment Vars

You can use the `-config.expand-env` command line flag to interpret environment variables in your Grafana Agent Static configuration.
You can pass these flags to [convert][] with `--extra-args="-config.expand-env"` or to [run][] with `--config.extra-args="-config.expand-env"`.

> It's possible to combine `integrations-next` with `expand-env`.
> For [convert][], you can use `--extra-args="-enable-features=integrations-next -config.expand-env"`

## Limitations

Configuration conversion is done on a best-effort basis. {{< param "PRODUCT_ROOT_NAME" >}} will issue warnings or errors where the conversion can't be performed.

After the configuration is converted, review the {{< param "PRODUCT_NAME" >}} configuration file and verify that it's correct before starting to use it in a production environment.

The following list is specific to the convert command and not {{< param "PRODUCT_NAME" >}}:

* The  [Traces][] and [Agent Management][] configuration options can't be automatically converted to {{< param "PRODUCT_NAME" >}}. However, traces are fully supported in {{< param "PRODUCT_NAME" >}} and you can build your configuration manually.
  Any additional unsupported features are returned as errors during conversion.
* There is no gRPC server to configure for {{< param "PRODUCT_NAME" >}}, as any non-default configuration will show as unsupported during the conversion.
* Check if you are using any extra command line arguments with Static that aren't present in your configuration file. For example, `-server.http.address`.
* Check if you are using any environment variables in your [Static][] configuration.
  These will be evaluated during conversion and you may want to replace them with the {{< param "PRODUCT_NAME" >}} Standard library [env][] function after conversion.
* Review additional [Prometheus Limitations][] for limitations specific to your [Metrics][] configuration.
* Review additional [Promtail Limitations][] for limitations specific to your [Logs][] configuration.
* The logs produced by {{< param "PRODUCT_NAME" >}} mode will differ from those produced by Static.
* {{< param "PRODUCT_ROOT_NAME" >}} exposes the {{< param "PRODUCT_NAME" >}} [UI][].

[debugging]: #debugging
[example]: #example

{{% docs/reference %}}
[Static]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static"
[Static]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/static"
[prometheus.scrape]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.scrape.md"
[prometheus.scrape]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.scrape.md"
[prometheus.remote_write]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.remote_write.md"
[prometheus.remote_write]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.remote_write.md"
[local.file_match]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/local.file_match.md"
[local.file_match]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/local.file_match.md"
[loki.process]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/loki.process.md"
[loki.process]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/loki.process.md"
[loki.source.file]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/loki.source.file.md"
[loki.source.file]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/loki.source.file.md"
[loki.write]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/loki.write.md"
[loki.write]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/loki.write.md"
[Components]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/components.md"
[Components]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/components.md"
[convert]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/convert.md"
[convert]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/convert.md"
[run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/run.md"
[run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/run.md"
[Run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/run/"
[Run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/get-started/run/"
[DebuggingUI]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md"
[DebuggingUI]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md"
[River]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/config-language/"
[River]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/"
[Integrations next]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/integrations/integrations-next/_index.md"
[Integrations next]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/static/configuration/traces-config.md
[Traces]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/traces-config.md"
[Traces]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/static/configuration/traces-config.md"
[Agent Management]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/agent-management.md"
[Agent Management]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/static/configuration/agent-management.md"
[env]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/stdlib/env.md"
[env]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/env.md"
[Prometheus Limitations]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/migrate/from-prometheus.md#limitations"
[Prometheus Limitations]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/migrate/from-prometheus.md#limitations"
[Promtail Limitations]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/migrate/from-promtail.md#limitations"
[Promtail Limitations]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/migrate/from-promtail.md#limitations"
[Metrics]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/metrics-config.md"
[Metrics]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/static/configuration/metrics-config.md"
[Logs]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/logs-config.md"
[Logs]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/static/logs-config.md"
[UI]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug#grafana-agent-flow-ui"
[UI]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug#grafana-agent-flow-ui"
{{% /docs/reference %}}
