---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.loadbalancing/
aliases:
  - ../otelcol.exporter.loadbalancing/ # /docs/alloy/latest/reference/components/otelcol.exporter.loadbalancing/
description: Learn about otelcol.exporter.loadbalancing
title: otelcol.exporter.loadbalancing
---

# otelcol.exporter.loadbalancing

<!-- Include a picture of the LB architecture? -->

`otelcol.exporter.loadbalancing` accepts logs and traces from other `otelcol` components
and writes them over the network using the OpenTelemetry Protocol (OTLP) protocol.

{{< admonition type="note" >}}
`otelcol.exporter.loadbalancing` is a wrapper over the upstream OpenTelemetry Collector `loadbalancing` exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

Multiple `otelcol.exporter.loadbalancing` components can be specified by giving them
different labels.

The decision which backend to use depends on the trace ID or the service name.
The backend load doesn't influence the choice. Even though this load-balancer won't do
round-robin balancing of the batches, the load distribution should be very similar among backends,
with a standard deviation under 5% at the current configuration.

`otelcol.exporter.loadbalancing` is especially useful for backends configured with tail-based samplers
which choose a backend based on the view of the full trace.

When a list of backends is updated, some of the signals will be rerouted to different backends.
Around R/N of the "routes" will be rerouted differently, where:

* A "route" is either a trace ID or a service name mapped to a certain backend.
* "R" is the total number of routes.
* "N" is the total number of backends.

This should be stable enough for most cases, and the larger the number of backends, the less disruption it should cause.

## Usage

```alloy
otelcol.exporter.loadbalancing "LABEL" {
  resolver {
    ...
  }
  protocol {
    otlp {
      client {}
    }
  }
}
```

## Arguments

`otelcol.exporter.loadbalancing` supports the following arguments:

Name          | Type     | Description                          | Default     | Required
--------------|----------|--------------------------------------|-------------|---------
`routing_key` | `string` | Routing strategy for load balancing. | `"traceID"` | no

The `routing_key` attribute determines how to route signals across endpoints. Its value could be one of the following:
* `"service"`: spans with the same `service.name` will be exported to the same backend.
This is useful when using processors like the span metrics, so all spans for each service are sent to consistent {{< param "PRODUCT_NAME" >}} instances
for metric collection. Otherwise, metrics for the same services would be sent to different instances, making aggregations inaccurate.
* `"traceID"`: spans belonging to the same traceID will be exported to the same backend.

## Blocks

The following blocks are supported inside the definition of
`otelcol.exporter.loadbalancing`:

Hierarchy                            | Block             | Description                                                                | Required
-------------------------------------|-------------------|----------------------------------------------------------------------------|---------
resolver                             | [resolver][]      | Configures discovering the endpoints to export to.                         | yes
resolver > static                    | [static][]        | Static list of endpoints to export to.                                     | no
resolver > dns                       | [dns][]           | DNS-sourced list of endpoints to export to.                                | no
resolver > kubernetes                | [kubernetes][]    | Kubernetes-sourced list of endpoints to export to.                         | no
resolver > aws_cloud_map             | [aws_cloud_map][] | AWS CloudMap-sourced list of endpoints to export to.                       | no
protocol                             | [protocol][]      | Protocol settings. Only OTLP is supported at the moment.                   | no
protocol > otlp                      | [otlp][]          | Configures an OTLP exporter.                                               | no
protocol > otlp > client             | [client][]        | Configures the exporter gRPC client.                                       | no
protocol > otlp > client > tls       | [tls][]           | Configures TLS for the gRPC client.                                        | no
protocol > otlp > client > keepalive | [keepalive][]     | Configures keepalive settings for the gRPC client.                         | no
protocol > otlp > queue              | [queue][]         | Configures batching of data before sending.                                | no
protocol > otlp > retry              | [retry][]         | Configures retry mechanism for failed requests.                            | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

The `>` symbol indicates deeper levels of nesting. For example, `resolver > static`
refers to a `static` block defined inside a `resolver` block.

[resolver]: #resolver-block
[static]: #static-block
[dns]: #dns-block
[kubernetes]: #kubernetes-block
[aws_cloud_map]: #aws_cloud_map-block
[protocol]: #protocol-block
[otlp]: #otlp-block
[client]: #client-block
[tls]: #tls-block
[keepalive]: #keepalive-block
[queue]: #queue-block
[retry]: #retry-block
[debug_metrics]: #debug_metrics-block

### resolver block

The `resolver` block configures how to retrieve the endpoint to which this exporter will send data.

Inside the `resolver` block, either the [dns][] block or the [static][] block should be specified.
If both `dns` and `static` are specified, `dns` takes precedence.

### static block

The `static` block configures a list of endpoints which this exporter will send data to.

The following arguments are supported:

Name        | Type           | Description                     | Default | Required
------------|----------------|---------------------------------|---------|---------
`hostnames` | `list(string)` | List of endpoints to export to. |         | yes

### dns block

The `dns` block periodically resolves an IP address via the DNS `hostname` attribute. This IP address
and the port specified via the `port` attribute will then be used by the gRPC exporter
as the endpoint to which to export data to.

The following arguments are supported:

Name       | Type       | Description                                                           | Default  | Required
-----------|------------|-----------------------------------------------------------------------|----------|---------
`hostname` | `string`   | DNS hostname to resolve.                                              |          | yes
`interval` | `duration` | Resolver interval.                                                    | `"5s"`   | no
`timeout`  | `duration` | Resolver timeout.                                                     | `"1s"`   | no
`port`     | `string`   | Port to be used with the IP addresses resolved from the DNS hostname. | `"4317"` | no

### kubernetes block

You can use the `kubernetes` block to load balance across the pods of a Kubernetes service.
The Kubernetes API notifies {{< param "PRODUCT_NAME" >}} whenever a new pod is added or removed from the service.
The `kubernetes` resolver has a much faster response time than the `dns` resolver because it doesn't require polling.

The following arguments are supported:

Name      | Type           | Description                                                 | Default  | Required
----------|----------------|-------------------------------------------------------------|----------|---------
`service` | `string`       | Kubernetes service to resolve.                              |          | yes
`ports`   | `list(number)` | Ports to use with the IP addresses resolved from `service`. | `[4317]` | no
`timeout` | `duration`     | Resolver timeout.                                           | `"1s"`   | no

If no namespace is specified inside `service`, an attempt will be made to infer the namespace for this {{< param "PRODUCT_NAME" >}}.
If this fails, the `default` namespace will be used.

Each of the ports listed in `ports` will be used with each of the IPs resolved from `service`.

The "get", "list", and "watch" [roles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-example)
must be granted in Kubernetes for the resolver to work.

### aws_cloud_map block

The `aws_cloud_map` block allows users to use `otelcol.exporter.loadbalancing` when using ECS over EKS in an AWS infrastructure.

The following arguments are supported:

Name            | Type       | Description                                                                        | Default     | Required
----------------|------------|------------------------------------------------------------------------------------|-------------|---------
`namespace`     | `string`   | The CloudMap namespace where the service is registered.                            |             | yes
`service_name`  | `string`   | The name of the service which was specified when registering the instance.         |             | yes
`interval`      | `duration` | Resolver interval.                                                                 | `"30s"`     | no
`timeout`       | `duration` | Resolver timeout.                                                                  | `"5s"`      | no
`health_status` | `string`   | Ports to use with the IP addresses resolved from `service`.                        | `"HEALTHY"` | no
`port`          | `number`   | Port to be used for exporting the traces to the addresses resolved from `service`. | `null`      | no

`health_status` can be set to either of:
* `HEALTHY`: Only return instances that are healthy.
* `UNHEALTHY`: Only return instances that are unhealthy.
* `ALL`: Return all instances, regardless of their health status.
* `HEALTHY_OR_ELSE_ALL`: Returns healthy instances, unless none are reporting a healthy state. 
  In that case, return all instances. This is also called failing open.

If `port` is not set, a default port defined in CloudMap will be used.

{{< admonition type="note" >}}
The `aws_cloud_map` resolver returns a maximum of 100 hosts.
A [feature request](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29771) 
aims cover pagination for this scenario.
{{< /admonition >}}

### protocol block

The `protocol` block configures protocol-related settings for exporting.
At the moment only the OTLP protocol is supported.

### otlp block

The `otlp` block configures OTLP-related settings for exporting.

### client block

The `client` block configures the gRPC client used by the component.
The endpoints used by the client block are the ones from the `resolver` block

The following arguments are supported:

Name                | Type                       | Description                                                                      | Default      | Required
--------------------|----------------------------|----------------------------------------------------------------------------------|--------------|---------
`compression`       | `string`                   | Compression mechanism to use for requests.                                       | `"gzip"`     | no
`read_buffer_size`  | `string`                   | Size of the read buffer the gRPC client to use for reading server responses.     |              | no
`write_buffer_size` | `string`                   | Size of the write buffer the gRPC client to use for writing requests.            | `"512KiB"`   | no
`wait_for_ready`    | `boolean`                  | Waits for gRPC connection to be in the `READY` state before sending data.        | `false`      | no
`headers`           | `map(string)`              | Additional headers to send with the request.                                     | `{}`         | no
`balancer_name`     | `string`                   | Which gRPC client-side load balancer to use for requests.                        | `pick_first` | no
`authority`         | `string`                   | Overrides the default `:authority` header in gRPC requests from the gRPC client. |              | no
`auth`              | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests.     |              | no

{{< docs/shared lookup="reference/components/otelcol-compression-field.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< docs/shared lookup="reference/components/otelcol-grpc-balancer-name.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< docs/shared lookup="reference/components/otelcol-grpc-authority.md" source="alloy" version="<ALLOY_VERSION>" >}}

You can configure an HTTP proxy with the following environment variables:

* `HTTPS_PROXY`
* `NO_PROXY`

The `HTTPS_PROXY` environment variable specifies a URL to use for proxying
requests. Connections to the proxy are established via [the `HTTP CONNECT`
method][HTTP CONNECT].

The `NO_PROXY` environment variable is an optional list of comma-separated
hostnames for which the HTTPS proxy should _not_ be used. Each hostname can be
provided as an IP address (`1.2.3.4`), an IP address in CIDR notation
(`1.2.3.4/8`), a domain name (`example.com`), or `*`. A domain name matches
that domain and all subdomains. A domain name with a leading "."
(`.example.com`) matches subdomains only. `NO_PROXY` is only read when
`HTTPS_PROXY` is set.

Because `otelcol.exporter.loadbalancing` uses gRPC, the configured proxy server must be
able to handle and proxy HTTP/2 traffic.

[HTTP CONNECT]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/CONNECT

### tls block

The `tls` block configures TLS settings used for the connection to the gRPC server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### keepalive block

The `keepalive` block configures keepalive settings for gRPC client
connections.

The following arguments are supported:

Name                    | Type       | Description                                                                                | Default | Required
------------------------|------------|--------------------------------------------------------------------------------------------|---------|---------
`ping_wait`             | `duration` | How often to ping the server after no activity.                                            |         | no
`ping_response_timeout` | `duration` | Time to wait before closing inactive connections if the server does not respond to a ping. |         | no
`ping_without_stream`   | `boolean`  | Send pings even if there is no active stream request.                                      |         | no

### queue block

The `queue` block configures an in-memory buffer of batches before data is sent to the gRPC server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### retry block

The `retry` block configures how failed requests to the gRPC server are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` OTLP-formatted data for telemetry signals of these types:
* logs
* traces

## Choose a load balancing strategy

<!-- TODO: Mention gropubytrace processor when Alloy supports it -->
<!-- TODO: Should we run more than 1 LB instance for better resiliency and spreading out the load? -->

Different {{< param "PRODUCT_NAME" >}} components require different load-balancing strategies.
The use of `otelcol.exporter.loadbalancing` is only necessary for [stateful components][stateful-and-stateless-components].

[stateful-and-stateless-components]: ../../../../set-up/deploy/#stateful-and-stateless-components

### otelcol.processor.tail_sampling
<!-- TODO: Add a picture of the architecture?  -->
All spans for a given trace ID must go to the same tail sampling {{< param "PRODUCT_NAME" >}} instance.
* This can be done by configuring `otelcol.exporter.loadbalancing` with `routing_key = "traceID"`.
* If you do not configure `routing_key = "traceID"`, the sampling decision may be incorrect.
  The tail sampler must have a full view of the trace when making a sampling decision.
  For example, a `rate_limiting` tail sampling strategy may incorrectly pass through
  more spans than expected if the spans for the same trace are spread out to more than
  one {{< param "PRODUCT_NAME" >}} instance.

<!-- Make "rate limiting" a URL to the tail sampler doc -->

### otelcol.connector.spanmetrics
All spans for a given `service.name` must go to the same spanmetrics {{< param "PRODUCT_NAME" >}}.
* This can be done by configuring `otelcol.exporter.loadbalancing` with `routing_key = "service"`.
* If you do not configure `routing_key = "service"`, metrics generated from spans might be incorrect.
For example, if similar spans for the same `service.name` end up on different {{< param "PRODUCT_NAME" >}} instances, the two {{< param "PRODUCT_NAME" >}}s will have identical metric series for calculating span latency, errors, and number of requests.
When both {{< param "PRODUCT_NAME" >}} instances attempt to write the metrics to a database such as Mimir, the series may clash with each other.
At best, this will lead to an error in {{< param "PRODUCT_NAME" >}} and a rejected write to the metrics database.
At worst, it could lead to inaccurate data due to overlapping samples for the metric series.

However, there are ways to scale `otelcol.connector.spanmetrics` without the need for a load balancer:
1. Each {{< param "PRODUCT_NAME" >}} could add an attribute such as `collector.id` in order to make its series unique.
   Then, for example, you could use a `sum by` PromQL query to aggregate the metrics from different {{< param "PRODUCT_NAME" >}}s.
   Unfortunately, an extra `collector.id` attribute has a downside that the metrics stored in the database will have higher {{< term "cardinality" >}}cardinality{{< /term >}}.
2. Spanmetrics could be generated in the backend database instead of in {{< param "PRODUCT_NAME" >}}.
    For example, span metrics can be [generated][tempo-spanmetrics] in Grafana Cloud by the Tempo traces database.

[tempo-spanmetrics]: https://grafana.com/docs/tempo/latest/metrics-generator/span_metrics/

### otelcol.connector.servicegraph
It is challenging to scale `otelcol.connector.servicegraph` over multiple {{< param "PRODUCT_NAME" >}} instances.
For `otelcol.connector.servicegraph` to work correctly, each "client" span must be paired with a "server" span to calculate metrics such as span duration.
If a "client" span goes to one {{< param "PRODUCT_NAME" >}}, but a "server" span goes to another {{< param "PRODUCT_NAME" >}},  then no single {{< param "PRODUCT_NAME" >}} will be able to pair the spans and a metric won't be generated.

`otelcol.exporter.loadbalancing` can solve this problem partially if it is configured with `routing_key = "traceID"`.
Each {{< param "PRODUCT_NAME" >}} will then be able to calculate a service graph for each "client"/"server" pair in a trace.
It is possible to have a span with similar "server"/"client" values in a different trace, processed by another {{< param "PRODUCT_NAME" >}}.
If two different {{< param "PRODUCT_NAME" >}} instances process similar "server"/"client" spans, they will generate the same service graph metric series.
If the series from two {{< param "PRODUCT_NAME" >}} are the same, this will lead to issues when writing them to the backend database.
You could differentiate the series by adding an attribute such as `"collector.id"`.
The series from different {{< param "PRODUCT_NAME" >}}s can be aggregated using PromQL queries on the backed metrics database.
If the metrics are stored in Grafana Mimir, cardinality issues due to `"collector.id"` labels can be solved using [Adaptive Metrics][adaptive-metrics].

A simpler, more scalable alternative to generating service graph metrics in {{< param "PRODUCT_NAME" >}} is to generate them entirely in the backend database.
For example, service graphs can be [generated][tempo-servicegraphs] in Grafana Cloud by the Tempo traces database.

[tempo-servicegraphs]: https://grafana.com/docs/tempo/latest/metrics-generator/service_graphs/
[adaptive-metrics]: https://grafana.com/docs/grafana-cloud/cost-management-and-billing/reduce-costs/metrics-costs/control-metrics-usage-via-adaptive-metrics/

### Mixing stateful components
<!-- TODO: Add a picture of the architecture?  -->
Different {{< param "PRODUCT_NAME" >}} components may require a different `routing_key` for `otelcol.exporter.loadbalancing`.
For example, `otelcol.processor.tail_sampling` requires `routing_key = "traceID"` whereas `otelcol.connector.spanmetrics` requires `routing_key = "service"`.
To load balance both types of components, two different sets of load balancers have to be set up:

* One set of `otelcol.exporter.loadbalancing` with `routing_key = "traceID"`, sending spans to {{< param "PRODUCT_NAME" >}}s doing tail sampling and no span metrics.
* Another set of `otelcol.exporter.loadbalancing` with `routing_key = "service"`, sending spans to {{< param "PRODUCT_NAME" >}}s doing span metrics and no service graphs.

Unfortunately, this can also lead to side effects.
For example, if `otelcol.connector.spanmetrics` is configured to generate exemplars, the tail sampling {{< param "PRODUCT_NAME" >}}s might drop the trace that the exemplar points to.
There is no coordination between the tail sampling {{< param "PRODUCT_NAME" >}}s and the span metrics {{< param "PRODUCT_NAME" >}}s to make sure trace IDs for exemplars are kept.

<!--
TODO: Add a troubleshooting section?
1. Use GODEBUG for DNS resolver logging
2. Enable debug logging on Alloy
3. gRPC debug env variables?
 -->

## Component health

`otelcol.exporter.loadbalancing` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.loadbalancing` does not expose any component-specific debug
information.

## Examples

### Static resolver

This example accepts OTLP logs and traces over gRPC.
It then sends them in a load-balanced way to "localhost:55690" or "localhost:55700".

```alloy
otelcol.receiver.otlp "default" {
    grpc {}
    output {
        traces  = [otelcol.exporter.loadbalancing.default.input]
        logs    = [otelcol.exporter.loadbalancing.default.input]
    }
}

otelcol.exporter.loadbalancing "default" {
    resolver {
        static {
            hostnames = ["localhost:55690", "localhost:55700"]
        }
    }
    protocol {
        otlp {
            client {}
        }
    }
}
```

### DNS resolver

When configured with a `dns` resolver, `otelcol.exporter.loadbalancing` will do a DNS lookup
on regular intervals. Spans are exported to the addresses the DNS lookup returned.

```alloy
otelcol.exporter.loadbalancing "default" {
    resolver {
        dns {
            hostname = "alloy-traces-sampling.grafana-cloud-monitoring.svc.cluster.local"
            port     = "34621"
            interval = "5s"
            timeout  = "1s"
        }
    }
    protocol {
        otlp {
            client {}
        }
    }
}
```

The following example shows a Kubernetes configuration that configures two sets of {{< param "PRODUCT_NAME" >}}s:
* A pool of load-balancer {{< param "PRODUCT_NAME" >}}s:
  * Spans are received from instrumented applications via `otelcol.receiver.otlp`
  * Spans are exported via `otelcol.exporter.loadbalancing`.
* A pool of sampling {{< param "PRODUCT_NAME" >}}s:
  * The sampling {{< param "PRODUCT_NAME" >}}s run behind a headless service to enable the load-balancer {{< param "PRODUCT_NAME" >}}s to discover them.
  * Spans are received from the load-balancer {{< param "PRODUCT_NAME" >}}s via `otelcol.receiver.otlp`
  * Traces are sampled via `otelcol.processor.tail_sampling`.
  * The traces are exported via `otelcol.exporter.otlp` to an OTLP-compatible database such as Tempo.

{{< collapse title="Example Kubernetes configuration" >}}

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: grafana-cloud-monitoring
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k6-trace-generator
  namespace: grafana-cloud-monitoring
spec:
  minReadySeconds: 10
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      name: k6-trace-generator
  template:
    metadata:
      labels:
        name: k6-trace-generator
    spec:
      containers:
      - env:
        - name: ENDPOINT
          value: alloy-traces-lb.grafana-cloud-monitoring.svc.cluster.local:9411
        image: ghcr.io/grafana/xk6-client-tracing:v0.0.2
        imagePullPolicy: IfNotPresent
        name: k6-trace-generator
---
apiVersion: v1
kind: Service
metadata:
  name: alloy-traces-lb
  namespace: grafana-cloud-monitoring
spec:
  clusterIP: None
  ports:
  - name: alloy-traces-otlp-grpc
    port: 9411
    protocol: TCP
    targetPort: 9411
  selector:
    name: alloy-traces-lb
  type: ClusterIP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alloy-traces-lb
  namespace: grafana-cloud-monitoring
spec:
  minReadySeconds: 10
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      name: alloy-traces-lb
  template:
    metadata:
      labels:
        name: alloy-traces-lb
    spec:
      containers:
      - args:
        - run
        - /etc/alloy/alloy_lb.alloy
        command:
        - /bin/alloy
        image: grafana/alloy:v1.0
        imagePullPolicy: IfNotPresent
        name: alloy-traces
        ports:
        - containerPort: 9411
          name: otlp-grpc
          protocol: TCP
        - containerPort: 34621
          name: alloy-lb
          protocol: TCP
        volumeMounts:
        - mountPath: /etc/alloy
          name: alloy-traces
      volumes:
      - configMap:
          name: alloy-traces
        name: alloy-traces
---
apiVersion: v1
kind: Service
metadata:
  name: alloy-traces-sampling
  namespace: grafana-cloud-monitoring
spec:
  clusterIP: None
  ports:
  - name: alloy-lb
    port: 34621
    protocol: TCP
    targetPort: alloy-lb
  selector:
    name: alloy-traces-sampling
  type: ClusterIP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alloy-traces-sampling
  namespace: grafana-cloud-monitoring
spec:
  minReadySeconds: 10
  replicas: 3
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      name: alloy-traces-sampling
  template:
    metadata:
      labels:
        name: alloy-traces-sampling
    spec:
      containers:
      - args:
        - run
        - /etc/alloy/alloy_sampling.alloy
        command:
        - /bin/alloy
        image: grafana/alloy:v1.0
        imagePullPolicy: IfNotPresent
        name: alloy-traces
        ports:
        - containerPort: 9411
          name: otlp-grpc
          protocol: TCP
        - containerPort: 34621
          name: alloy-lb
          protocol: TCP
        volumeMounts:
        - mountPath: /etc/alloy
          name: alloy-traces
      volumes:
      - configMap:
          name: alloy-traces
        name: alloy-traces
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: alloy-traces
  namespace: grafana-cloud-monitoring
data:
  alloy_lb.alloy: |
    otelcol.receiver.otlp "default" {
      grpc {
        endpoint = "0.0.0.0:9411"
      }
      output {
        traces = [otelcol.exporter.loadbalancing.default.input,otelcol.exporter.logging.default.input]
      }
    }

    otelcol.exporter.logging "default" {
      verbosity = "detailed"
    }

    otelcol.exporter.loadbalancing "default" {
      resolver {
        dns {
          hostname = "alloy-traces-sampling.grafana-cloud-monitoring.svc.cluster.local"
          port = "34621"
        }
      }
      protocol {
        otlp {
          client {
            tls {
              insecure = true
            }
          }
        }
      }
    }

  alloy_sampling.alloy: |
    otelcol.receiver.otlp "default" {
      grpc {
        endpoint = "0.0.0.0:34621"
      }
      output {
        traces = [otelcol.exporter.otlp.default.input,otelcol.exporter.logging.default.input]
      }
    }

    otelcol.exporter.logging "default" {
      verbosity = "detailed"
    }

    otelcol.exporter.otlp "default" {
      client {
        endpoint = "tempo-prod-06-prod-gb-south-0.grafana.net:443"
        auth     = otelcol.auth.basic.creds.handler
      }
    }

    otelcol.auth.basic "creds" {
      username = "111111"
      password = "pass"
    }
```
{{< /collapse >}}

You must fill in the correct OTLP credentials prior to running the example.
You can use [k3d][] to start the example:

<!-- TODO: Link to the k3d page -->
```bash
k3d cluster create alloy-lb-test
kubectl apply -f kubernetes_config.yaml
```

To delete the cluster, run:

```bash
k3d cluster delete alloy-lb-test
```

[k3d]: https://k3d.io/v5.6.0/

### Kubernetes resolver

When you configure `otelcol.exporter.loadbalancing`  with a `kubernetes` resolver, the Kubernetes API notifies {{< param "PRODUCT_NAME" >}} whenever a new pod is added or removed from the service.
Spans are exported to the addresses from the Kubernetes API, combined with all the possible `ports`.

```alloy
otelcol.exporter.loadbalancing "default" {
    resolver {
        kubernetes {
            service = "alloy-traces-headless"
            ports   = [ 34621 ]
        }
    }
    protocol {
        otlp {
            client {}
        }
    }
}
```

The following example shows a Kubernetes configuration that sets up two sets of {{< param "PRODUCT_NAME" >}}s:
* A pool of load-balancer {{< param "PRODUCT_NAME" >}}s:
  * Spans are received from instrumented applications via `otelcol.receiver.otlp`
  * Spans are exported via `otelcol.exporter.loadbalancing`.
  * The load-balancer {{< param "PRODUCT_NAME" >}}s will get notified by the Kubernetes API any time a pod
    is added or removed from the pool of sampling {{< param "PRODUCT_NAME" >}}s.
* A pool of sampling {{< param "PRODUCT_NAME" >}}s:
  * The sampling {{< param "PRODUCT_NAME" >}}s do not need to run behind a headless service.
  * Spans are received from the load-balancer {{< param "PRODUCT_NAME" >}}s via `otelcol.receiver.otlp`
  * Traces are sampled via `otelcol.processor.tail_sampling`.
  * The traces are exported via `otelcol.exporter.otlp` to a an OTLP-compatible database such as Tempo.

<!-- TODO: In the k8s config, why does the LB service has to be headless? -->
{{< collapse title="Example Kubernetes configuration" >}}

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: grafana-cloud-monitoring
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: alloy-traces
  namespace: grafana-cloud-monitoring
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: alloy-traces-role
  namespace: grafana-cloud-monitoring
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - list
  - watch
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: alloy-traces-rolebinding
  namespace: grafana-cloud-monitoring
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: alloy-traces-role
subjects:
- kind: ServiceAccount
  name: alloy-traces
  namespace: grafana-cloud-monitoring
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k6-trace-generator
  namespace: grafana-cloud-monitoring
spec:
  minReadySeconds: 10
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      name: k6-trace-generator
  template:
    metadata:
      labels:
        name: k6-trace-generator
    spec:
      containers:
      - env:
        - name: ENDPOINT
          value: alloy-traces-lb.grafana-cloud-monitoring.svc.cluster.local:9411
        image: ghcr.io/grafana/xk6-client-tracing:v0.0.2
        imagePullPolicy: IfNotPresent
        name: k6-trace-generator
---
apiVersion: v1
kind: Service
metadata:
  name: alloy-traces-lb
  namespace: grafana-cloud-monitoring
spec:
  clusterIP: None
  ports:
  - name: alloy-traces-otlp-grpc
    port: 9411
    protocol: TCP
    targetPort: 9411
  selector:
    name: alloy-traces-lb
  type: ClusterIP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alloy-traces-lb
  namespace: grafana-cloud-monitoring
spec:
  minReadySeconds: 10
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      name: alloy-traces-lb
  template:
    metadata:
      labels:
        name: alloy-traces-lb
    spec:
      containers:
      - args:
        - run
        - /etc/alloy/alloy_lb.alloy
        command:
        - /bin/alloy
        image: grafana/alloy:v1.0
        imagePullPolicy: IfNotPresent
        name: alloy-traces
        ports:
        - containerPort: 9411
          name: otlp-grpc
          protocol: TCP
        volumeMounts:
        - mountPath: /etc/alloy
          name: alloy-traces
      serviceAccount: alloy-traces
      volumes:
      - configMap:
          name: alloy-traces
        name: alloy-traces
---
apiVersion: v1
kind: Service
metadata:
  name: alloy-traces-sampling
  namespace: grafana-cloud-monitoring
spec:
  ports:
  - name: alloy-lb
    port: 34621
    protocol: TCP
    targetPort: alloy-lb
  selector:
    name: alloy-traces-sampling
  type: ClusterIP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alloy-traces-sampling
  namespace: grafana-cloud-monitoring
spec:
  minReadySeconds: 10
  replicas: 3
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      name: alloy-traces-sampling
  template:
    metadata:
      labels:
        name: alloy-traces-sampling
    spec:
      containers:
      - args:
        - run
        - /etc/alloy/alloy_sampling.alloy
        command:
        - /bin/alloy
        image: grafana/alloy:v1.0
        imagePullPolicy: IfNotPresent
        name: alloy-traces
        ports:
        - containerPort: 34621
          name: alloy-lb
          protocol: TCP
        volumeMounts:
        - mountPath: /etc/alloy
          name: alloy-traces
      volumes:
      - configMap:
          name: alloy-traces
        name: alloy-traces
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: alloy-traces
  namespace: grafana-cloud-monitoring
data:
  alloy_lb.alloy: |
    otelcol.receiver.otlp "default" {
      grpc {
        endpoint = "0.0.0.0:9411"
      }
      output {
        traces = [otelcol.exporter.loadbalancing.default.input,otelcol.exporter.logging.default.input]
      }
    }

    otelcol.exporter.logging "default" {
      verbosity = "detailed"
    }

    otelcol.exporter.loadbalancing "default" {
      resolver {
        kubernetes {
          service = "alloy-traces-sampling"
          ports = ["34621"]
        }
      }
      protocol {
        otlp {
          client {
            tls {
              insecure = true
            }
          }
        }
      }
    }

  alloy_sampling.alloy: |
    otelcol.receiver.otlp "default" {
      grpc {
        endpoint = "0.0.0.0:34621"
      }
      output {
        traces = [otelcol.exporter.otlp.default.input,otelcol.exporter.logging.default.input]
      }
    }

    otelcol.exporter.logging "default" {
      verbosity = "detailed"
    }

    otelcol.exporter.otlp "default" {
      client {
        endpoint = "tempo-prod-06-prod-gb-south-0.grafana.net:443"
        auth     = otelcol.auth.basic.creds.handler
      }
    }

    otelcol.auth.basic "creds" {
      username = "111111"
      password = "pass"
    }
```

{{< /collapse >}}

You must fill in the correct OTLP credentials prior to running the example.
You can use [k3d][] to start the example:

```bash
k3d cluster create alloy-lb-test
kubectl apply -f kubernetes_config.yaml
```

To delete the cluster, run:

```bash
k3d cluster delete alloy-lb-test
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.loadbalancing` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
