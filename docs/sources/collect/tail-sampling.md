---
canonical: https://grafana.com/docs/alloy/latest/collect/tail-sampling/
description: Learn how to configure tail sampling for traces in Grafana Alloy
menuTitle: Tail sampling
title: Collect and sample traces with tail sampling
weight: 475
---

# Collect and sample traces with tail sampling

Tail sampling lets you make sampling decisions based on the full trace, after all spans arrive.
This gives you more control than head sampling, because you can keep traces that contain errors, high latency, or other important attributes.

## Components used in this topic

- [`otelcol.exporter.loadbalancing`][otelcol.exporter.loadbalancing]
- [`otelcol.exporter.otlp`][otelcol.exporter.otlp]
- [`otelcol.processor.tail_sampling`][otelcol.processor.tail_sampling]
- [`otelcol.receiver.otlp`][otelcol.receiver.otlp]

## Before you begin

- Ensure you have a working {{< param "PRODUCT_NAME" >}} installation.
- Have an OTLP-compatible backend, such as Grafana Tempo, ready to receive traces.
- Be familiar with [OpenTelemetry concepts][opentelemetry] and the [`otelcol` components][components] in {{< param "PRODUCT_NAME" >}}.

## What is tail sampling

With head sampling, the decision to keep or drop a trace happens at the start, before all spans arrive.
Tail sampling waits until the trace is complete, then evaluates it against your policies.

The [`otelcol.processor.tail_sampling`][otelcol.processor.tail_sampling] component buffers spans in memory.
After the `decision_wait` period elapses, it evaluates the trace against each configured policy and either keeps or drops it.

Tail sampling is stateful: all spans for a given trace ID must arrive at the same {{< param "PRODUCT_NAME" >}} instance.
If spans scatter across instances, policies that depend on full trace context—such as `latency` or `rate_limiting`—produce incorrect results.

## When to use tail sampling

Use tail sampling when you need to:

- Keep traces that contain errors or spans with high latency.
- Reduce trace volume while you preserve the signals you care about.
- Base decisions on trace-level attributes, not just the first span.

If you want low-cost, stateless sampling, use [`otelcol.processor.probabilistic_sampler`][otelcol.processor.probabilistic_sampler] instead.
It makes a random sampling decision at ingestion and doesn't require buffering.

| Use case                            | Recommendation                            |
| ----------------------------------- | ----------------------------------------- |
| Random percentage of all traces     | `otelcol.processor.probabilistic_sampler` |
| Keep errors and slow traces         | `otelcol.processor.tail_sampling`         |
| Centralized control across backends | Backend or adaptive sampling in Tempo     |

{{< admonition type="note" >}}
Sampling decisions affect signal correlation in Grafana.
When you drop a trace, any log lines, metric exemplars, or profiles that reference that trace ID produce broken links in Grafana dashboards.
Choose policies that target non-normative behavior—such as errors or high latency—so you're less likely to drop a trace that other signals reference.
For more context, refer to [Sampling and telemetry correlation](https://grafana.com/docs/tempo/latest/set-up-for-tracing/instrument-send/set-up-collector/tail-sampling/#sampling-and-telemetry-correlation) in the Grafana Tempo documentation.
{{< /admonition >}}

## Configure tail sampling

The steps below configure a basic tail sampling pipeline that receives traces over OTLP, keeps traces with errors or high latency, and forwards them to a Tempo backend.

1. Add an `otelcol.receiver.otlp` component to receive trace data:

   ```alloy
   otelcol.receiver.otlp "default" {
     grpc {
       endpoint = "0.0.0.0:4317"
     }
     http {
       endpoint = "0.0.0.0:4318"
     }

     output {
       traces = [otelcol.processor.tail_sampling.default.input]
     }
   }
   ```

1. Add an `otelcol.processor.tail_sampling` component and configure your policies:

   ```alloy
   otelcol.processor.tail_sampling "default" {
     decision_wait = "10s"

     policy {
       name = "keep-errors"
       type = "status_code"

       status_code {
         status_codes = ["ERROR"]
       }
     }

     policy {
       name = "keep-slow-traces"
       type = "latency"

       latency {
         threshold_ms = 1000
       }
     }

     output {
       traces = [otelcol.exporter.otlp.tempo.input]
     }
   }
   ```

   `decision_wait` sets how long {{< param "PRODUCT_NAME" >}} waits after the first span arrives before it makes a sampling decision. The default is `"30s"`. A shorter value reduces memory usage but may cause {{< param "PRODUCT_NAME" >}} to drop spans that arrive after the decision is made.

   If a trace runs longer than `decision_wait`, {{< param "PRODUCT_NAME" >}} splits it across multiple decision windows. This can produce a fragmented trace in your backend, where only the spans from the final window are stored. You can reduce this risk by configuring `decision_cache`, which tells {{< param "PRODUCT_NAME" >}} to honor previous sampling decisions for a trace ID even after the decision window closes. For a detailed explanation of decision windows, cache trade-offs, and configuration guidance, refer to [Decision periods and caches](https://grafana.com/docs/tempo/latest/set-up-for-tracing/instrument-send/set-up-collector/tail-sampling/#decision-periods) in the Grafana Tempo documentation.

1. Add an `otelcol.exporter.otlp` component to forward kept traces to Tempo:

   ```alloy
   otelcol.exporter.otlp "tempo" {
     client {
       endpoint = "<TEMPO_ENDPOINT>"
     }
   }
   ```

   Replace the following:

   - _`<TEMPO_ENDPOINT>`_: The address of your Tempo instance, such as `tempo:4317`.

The following example shows the complete pipeline:

```alloy
otelcol.receiver.otlp "default" {
  grpc {
    endpoint = "0.0.0.0:4317"
  }
  http {
    endpoint = "0.0.0.0:4318"
  }

  output {
    traces = [otelcol.processor.tail_sampling.default.input]
  }
}

otelcol.processor.tail_sampling "default" {
  decision_wait = "10s"

  policy {
    name = "keep-errors"
    type = "status_code"

    status_code {
      status_codes = ["ERROR"]
    }
  }

  policy {
    name = "keep-slow-traces"
    type = "latency"

    latency {
      threshold_ms = 1000
    }
  }

  output {
    traces = [otelcol.exporter.otlp.tempo.input]
  }
}

otelcol.exporter.otlp "tempo" {
  client {
    endpoint = "<TEMPO_ENDPOINT>"
  }
}
```

## Scale tail sampling across multiple instances

Because tail sampling is stateful, you can't spread traces randomly across multiple {{< param "PRODUCT_NAME" >}} instances.
Each trace ID must always route to the same instance, or the processor won't have a complete view of the trace.

Use [`otelcol.exporter.loadbalancing`][otelcol.exporter.loadbalancing] in front of your tail sampling instances and set `routing_key = "traceID"`.
This ensures all spans for each trace ID route to the same downstream instance:

```alloy
otelcol.exporter.loadbalancing "front" {
  routing_key = "traceID"

  resolver {
    static {
      hostnames = ["alloy-sampler-1:4317", "alloy-sampler-2:4317"]
    }
  }

  protocol {
    otlp {
      client {}
    }
  }
}
```

{{< admonition type="note" >}}
Don't run tail sampling and span metrics on the same {{< param "PRODUCT_NAME" >}} instances.
Each processor needs a different `routing_key` for the load balancer—`"traceID"` for tail sampling and `"service"` for span metrics.
Run them as separate sets of instances with separate load balancers.
{{< /admonition >}}

If you also run [`otelcol.connector.spanmetrics`][otelcol.connector.spanmetrics] or [`otelcol.connector.servicegraph`][otelcol.connector.servicegraph], connect them to the receiver output before the tail sampling processor.
Both connectors need the full span stream, not the already-filtered output from the sampler.

For a full explanation of load balancing strategies and stateful components, refer to [`otelcol.exporter.loadbalancing`][otelcol.exporter.loadbalancing].
For a recommended production pipeline shape that combines tail sampling, span metrics, and service graphs, refer to [Pipeline workflows](https://grafana.com/docs/tempo/latest/set-up-for-tracing/instrument-send/set-up-collector/tail-sampling/#pipeline-workflows) in the Grafana Tempo documentation.

## Learn more

- [Tail sampling in Tempo](https://grafana.com/docs/tempo/latest/set-up-for-tracing/instrument-send/set-up-collector/tail-sampling/)
- [Tail-based sampling architecture and configuration](https://grafana.com/docs/tempo/latest/configuration/grafana-agent/tail-based-sampling/)
- [`otelcol.processor.tail_sampling`][otelcol.processor.tail_sampling] reference
- [`otelcol.exporter.loadbalancing`][otelcol.exporter.loadbalancing] reference

[components]: ../../get-started/components/
[opentelemetry]: https://opentelemetry.io
[otelcol.connector.servicegraph]: ../../reference/components/otelcol/otelcol.connector.servicegraph/
[otelcol.connector.spanmetrics]: ../../reference/components/otelcol/otelcol.connector.spanmetrics/
[otelcol.exporter.loadbalancing]: ../../reference/components/otelcol/otelcol.exporter.loadbalancing/
[otelcol.exporter.otlp]: ../../reference/components/otelcol/otelcol.exporter.otlp/
[otelcol.processor.probabilistic_sampler]: ../../reference/components/otelcol/otelcol.processor.probabilistic_sampler/
[otelcol.processor.tail_sampling]: ../../reference/components/otelcol/otelcol.processor.tail_sampling/
[otelcol.receiver.otlp]: ../../reference/components/otelcol/otelcol.receiver.otlp/
