---
canonical: https://grafana.com/docs/alloy/latest/collect/opentelemetry-data/
aliases:
  - ../tasks/collect-opentelemetry-data/ # /docs/alloy/latest/tasks/collect-opentelemetry-data/
description: Learn how to collect OpenTelemetry data
menuTitle: Collect OpenTelemetry data
title: Collect OpenTelemetry data from instrumented applications
weight: 400
---

# Collect OpenTelemetry data from instrumented applications

{{< param "PRODUCT_NAME" >}} can receive OpenTelemetry metrics, logs, and traces and forward them to any OTLP-compatible endpoint.

## Configuration

<!-- TODO: Find a way to import this without listing the SVG code like that -->

<svg xmlns="http://www.w3.org/2000/svg" width="100%" viewBox="-2 -2 1351 116" role="img" aria-label="Pipeline diagram: otelcol.receiver.otlp forwards data to otelcol.exporter.otlphttp, which uses otelcol.auth.basic for authentication">
<defs>
  <marker id="arrow-2563EB" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto"><polygon points="0 0, 10 3.5, 0 7" fill="#2563EB"/></marker>
</defs>
<path d="M394.0,56.0 C419.0,56.0 419.0,56.0 444.0,56.0" fill="none" stroke="#2563EB" stroke-width="1.5" marker-end="url(#arrow-2563EB)"/>
<path d="M890.0,56.0 C915.0,56.0 915.0,56.0 940.0,56.0" fill="none" stroke="#2563EB" stroke-width="1.5" marker-end="url(#arrow-2563EB)"/>
<rect x="0.0" y="0.0" width="394.0" height="112" rx="8" ry="8" fill="#DBEAFE" stroke="#2563EB" stroke-width="1.5"/>
<text x="197.0" y="42.6" text-anchor="middle" dominant-baseline="middle" font-family="ui-monospace,monospace" font-size="21" font-weight="bold" fill="#2563EB">otelcol.receiver.otlp</text>
<text x="197.0" y="78.4" text-anchor="middle" dominant-baseline="middle" font-family="ui-monospace,monospace" font-size="20" fill="#64748B">"default"</text>
<rect x="444.0" y="0.0" width="446.0" height="112" rx="8" ry="8" fill="#DBEAFE" stroke="#2563EB" stroke-width="1.5"/>
<text x="667.0" y="42.6" text-anchor="middle" dominant-baseline="middle" font-family="ui-monospace,monospace" font-size="21" font-weight="bold" fill="#2563EB">otelcol.exporter.otlphttp</text>
<text x="667.0" y="78.4" text-anchor="middle" dominant-baseline="middle" font-family="ui-monospace,monospace" font-size="20" fill="#64748B">"default"</text>
<rect x="940.0" y="0.0" width="407.0" height="112" rx="8" ry="8" fill="#DBEAFE" stroke="#2563EB" stroke-width="1.5"/>
<text x="1143.5" y="42.6" text-anchor="middle" dominant-baseline="middle" font-family="ui-monospace,monospace" font-size="21" font-weight="bold" fill="#2563EB">otelcol.auth.basic</text>
<text x="1143.5" y="78.4" text-anchor="middle" dominant-baseline="middle" font-family="ui-monospace,monospace" font-size="20" fill="#64748B">"credentials"</text>
</svg>

The following configuration receives OTLP data from your applications, batches it, and exports it over HTTP.

```alloy
otelcol.receiver.otlp "default" {
	// Configures the gRPC server to receive telemetry data.
	grpc {
		// host:port to listen for traffic on.
		endpoint = "0.0.0.0:4317"
	}

	// Configures the HTTP server to receive telemetry data.
	http {
		// host:port to listen for traffic on.
		endpoint = "0.0.0.0:4318"
	}

	// Configures where to send received telemetry data.
	output {
		metrics = [otelcol.exporter.otlphttp.default.input]
		logs    = [otelcol.exporter.otlphttp.default.input]
		traces  = [otelcol.exporter.otlphttp.default.input]
	}
}

// Grafana databases support basic authentication
otelcol.auth.basic "credentials" {
	// Username to use for basic authentication requests.
	username = sys.env("OTLP_USERNAME")

	// Password to use for basic authentication requests.
	password = sys.env("OTLP_API_KEY")
}

// Export data over HTTP to any OTLP-compatible endpoint
otelcol.exporter.otlphttp "default" {
	// Configures the HTTP client to send telemetry data to.
	client {
		// The target URL to send telemetry data to.
		endpoint = "https://<OTLP_ENDPOINT>"

		// Handler from an otelcol.auth component to use for authenticating requests.
		auth = otelcol.auth.basic.credentials.handler
	}

	// Configures queueing and batching for the exporter.
	sending_queue {
		// Configures batching requests based on a timeout and a minimum number of items.
		batch { }
	}
}
```

## How it works

Data flows through the pipeline in this order:

1. [`otelcol.receiver.otlp`][otelcol.receiver.otlp] receives OTLP data from your applications over gRPC or HTTP.
1. [`otelcol.auth.basic`][otelcol.auth.basic] provides credentials for authenticating with your OTLP endpoint.
1. [`otelcol.exporter.otlphttp`][otelcol.exporter.otlphttp] batches and sends the data to your OTLP endpoint over HTTP.

Refer to the [full list of components][Components] for other receiver, processor, and exporter options.

## Grafana database OTLP endpoints

Grafana [Mimir][mimir-otlp], [Loki][loki-otlp], and [Tempo][tempo-otlp] all support OTLP ingestion over HTTP.
Refer to each database's API documentation for the OTLP endpoint URL and any authentication requirements.

**Grafana Cloud** provides a single managed OTLP endpoint that accepts metrics, logs, and traces in one place.
<!-- TODO: link to Grafana Cloud docs explaining how to use the OpenTelemetry tile to get the endpoint URL and credentials -->

{{< admonition type="note" >}}
Tempo also accepts OTLP over gRPC. To use gRPC, replace `otelcol.exporter.otlphttp` with [`otelcol.exporter.otlp`][otelcol.exporter.otlp].
{{< /admonition >}}

[mimir-otlp]: https://grafana.com/docs/mimir/latest/references/http-api/#otlp
[loki-otlp]: https://grafana.com/docs/loki/latest/reference/loki-http-api/#ingest-endpoints
[tempo-otlp]: https://grafana.com/docs/tempo/latest/api_docs/#ingest
[otelcol.auth.basic]: ../../reference/components/otelcol/otelcol.auth.basic/
[otelcol.exporter.otlp]: ../../reference/components/otelcol/otelcol.exporter.otlp/
[otelcol.exporter.otlphttp]: ../../reference/components/otelcol/otelcol.exporter.otlphttp/
[otelcol.receiver.otlp]: ../../reference/components/otelcol/otelcol.receiver.otlp/
[Components]: ../../get-started/components/
