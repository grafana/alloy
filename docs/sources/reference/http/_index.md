---
canonical: https://grafana.com/docs/alloy/latest/reference/http/
description: Learn about HTTP endpoints exposed by Grafana Alloy
title: The Grafana Alloy HTTP endpoints
menuTitle: HTTP endpoints
weight: 700
---

# The {{% param "FULL_PRODUCT_NAME" %}} HTTP endpoints

{{< param "FULL_PRODUCT_NAME" >}} has several default HTTP endpoints that are available by default regardless of which components you have configured.
You can use these HTTP endpoints to monitor, health check, and troubleshoot {{< param "PRODUCT_NAME" >}}.

The HTTP server which exposes them is configured via the [http block](../config-blocks/http)
and the `--server.` [command line arguments](../cli/run).
For example, if you set the `--server.http.listen-addr` command line argument to `127.0.0.1:12345`, 
you can query the `127.0.0.1:12345/metrics` endpoint to see the internal metrics of {{< param "PRODUCT_NAME" >}}.

### /metrics

The `/metrics` endpoint returns the internal metrics of {{< param "PRODUCT_NAME" >}} in the Prometheus exposition format.

### /-/ready

An {{< param "PRODUCT_NAME" >}} instance is ready once it has loaded its initial configuration.
If the instance is ready, the `/-/ready` endpoint returns `HTTP 200 OK` and the message `Alloy is ready.`
Otherwise, if the instance is not ready, the `/-/ready` endpoint returns `HTTP 503 Service Unavailable` and the message `Alloy is not ready.`

### /-/healthy

When all {{< param "PRODUCT_NAME" >}} components are working correctly, all components are considered healthy.
If all components are healthy, the `/-/healthy` endpoint returns `HTTP 200 OK` and the message `All Alloy components are healthy.`.
Otherwise, if any of the components are not working correctly, the `/-/healthy` endpoint returns `HTTP 500 Internal Server Error` and an error message.
You can also monitor component health through the {{< param "PRODUCT_NAME" >}} [UI](../../troubleshoot/debug#alloy-ui).

```shell
$ curl localhost:12345/-/healthy
All Alloy components are healthy.
```

```shell
$ curl localhost:12345/-/healthy
unhealthy components: math.add
```

{{< admonition type="note" >}}
The `/-/healthy` endpoint isn't suitable for a [Kubernetes liveness probe][k8s-liveness].

An {{< param "PRODUCT_NAME" >}} instance that reports as unhealthy should not necessarily be restarted.
For example, a component may be unhealthy due to an invalid configuration or an unavailable external resource.
In this case, restarting {{< param "PRODUCT_NAME" >}} would not fix the problem.
A restart may make it worse, because it would could stop the flow of telemetry in healthy pipelines.

[k8s-liveness]: https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/
{{< /admonition >}}

### /-/reload

The `/-/reload` endpoint reloads the {{< param "PRODUCT_NAME" >}} configuration file.
If the configuration file can't be reloaded, the `/-/reload` endpoint returns `HTTP 400 Bad Request` and an error message.

```shell
$ curl localhost:12345/-/reload
config reloaded
```

```shell
$ curl localhost:12345/-/reload
error during the initial load: /Users/user1/Desktop/git.alloy:13:1: Failed to build component: loading custom component controller: custom component config not found in the registry, namespace: "math", componentName: "add"
```

### /-/support

The `/-/support` endpoint returns a [support bundle](../../troubleshoot/support_bundle) that contains information about your {{< param "PRODUCT_NAME" >}} instance. You can use this information as a baseline when debugging an issue.

### /debug/pprof

The `/debug/pprof` endpoint returns a pprof Go [profile](../../troubleshoot/profile) that you can use to visualize and analyze profiling data.
