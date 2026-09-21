---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/helm/
description: Learn how to run the OpenTelemetry Engine with the OpenTelemetry Collector Helm chart
menuTitle: Helm chart
title: Run the OpenTelemetry Engine with the OpenTelemetry Collector Helm chart
weight: 300
---

# Run the {{% param "OTEL_ENGINE" %}} with the OpenTelemetry Collector Helm chart

Use the upstream [OpenTelemetry Collector Helm chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector) to run the {{< param "OTEL_ENGINE" >}}.
This approach delivers an identical upstream collector experience.
It also ensures you get improvements, bug fixes, and security updates as they're released.

The following example Helm `values.yaml` incorporates the same configuration used in the [CLI example](../cli/) into a Kubernetes Deployment.

{{< admonition type="note" >}}
In this configuration, binding port `8888` to `0.0.0.0` makes the metrics endpoint listen on all interfaces inside the Pod.
This lets other Pods in the cluster reach it without using `kubectl port-forward`.

The configuration also sets the `command.name` key to `bin/otelcol`.
This is the binary that runs the `alloy otel` sub-command.
The Helm chart doesn't expose custom commands, so this setting is necessary.
{{< /admonition >}}

```yaml
image:
  repository: grafana/alloy
  tag: latest

command: 
  name: "bin/otelcol"

mode: deployment

ports:
  metrics:
    enabled: true

alternateConfig:
  extensions:
    health_check:
      endpoint: 0.0.0.0:13133 # This is necessary for the Kubernetes liveness check
    basicauth/my_auth:
      client_auth:
        username: <USERNAME>
        password: <PASSWORD>

  receivers:
    otlp:
      protocols:
        grpc: {}
        http: {}

  processors:
    batch:
      timeout: 1s
      send_batch_size: 512

  exporters:
    otlphttp/my_backend:
      endpoint: <URL>
      auth:
        authenticator: basicauth/my_auth

  service:
    telemetry:
      metrics:
        readers:
          - pull:
              exporter:
                prometheus:
                  host: 0.0.0.0 
                  port: 8888
    extensions: [basicauth/my_auth, health_check]
    pipelines:
      traces:
        receivers: [otlp]
        processors: [batch]
        exporters: [otlphttp/my_backend]
```

Replace the following:

- _`<USERNAME>`_: Your username. If you're using Grafana Cloud, this is your Grafana Cloud instance ID.
- _`<PASSWORD>`_: Your password. If you're using Grafana Cloud, this is your Grafana Cloud API token.
- _`<URL>`_: The URL to export data to. If you're using Grafana Cloud, this is your Grafana Cloud OTLP endpoint URL.

The Helm chart ships with a default OpenTelemetry Collector configuration in the `config` field.
The upstream Helm chart [documentation](https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/#configuration) describes this field.
If you want to completely override that default configuration, use the `alternateConfig` field.
In the example above, the `alternateConfig` field ensures the configuration matches the other {{< param "OTEL_ENGINE" >}} examples and doesn't inherit any of the chart's defaults.
Alternatively, you can omit both `config` and `alternateConfig` to use the default configuration as-is.
You can also provide your own `config` block that merges with the chart's default configuration.

Refer to the [upstream documentation](https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/) for more information about how to configure the helm chart to work for your use case.
