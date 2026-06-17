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

You can configure the HTTP server which exposes them with the [`http`](../config-blocks/http) block and the `--server.` [command line arguments](../cli/run).
For example, if you set the `--server.http.listen-addr` command line argument to `127.0.0.1:12345`, you can query the `127.0.0.1:12345/metrics` endpoint to see the internal metrics of {{< param "PRODUCT_NAME" >}}.

## `/metrics`

The `/metrics` endpoint returns the internal metrics of {{< param "PRODUCT_NAME" >}} in the Prometheus exposition format.

## `/-/ready`

An {{< param "PRODUCT_NAME" >}} instance is ready once it has loaded its initial configuration.
If the instance is ready, the `/-/ready` endpoint returns `HTTP 200 OK` and the message `Alloy is ready.`
Otherwise, if the instance isn't ready, the `/-/ready` endpoint returns `HTTP 503 Service Unavailable` and the message `Alloy is not ready.`

## `/-/healthy`

The `/-/healthy` endpoint returns `HTTP 200 OK` and the message `Alloy is healthy.` to indicate that the {{< param "PRODUCT_NAME" >}} instance is running.

```shell
curl localhost:12345/-/healthy
Alloy is healthy.
```

{{< admonition type="note" >}}
The `/-/healthy` endpoint is suitable for a [Kubernetes liveness probe][k8s-liveness] to verify that the {{< param "PRODUCT_NAME" >}} process is alive and responsive.

Note that this endpoint only reflects the health of the {{< param "PRODUCT_NAME" >}} process itself, not individual internal components. Individual components can be monitored through the {{< param "PRODUCT_NAME" >}} [UI](../../troubleshoot/debug#alloy-ui) or via the `/metrics` endpoint (using `alloy_component_controller_running_components` with the `health_type` label).

[k8s-liveness]: https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/
{{< /admonition >}}

## `/-/reload`

The `/-/reload` endpoint reloads the {{< param "PRODUCT_NAME" >}} configuration file.
If the configuration file can't be reloaded, the `/-/reload` endpoint returns `HTTP 400 Bad Request` and an error message.

```shell
curl localhost:12345/-/reload
config reloaded
```

```shell
curl localhost:12345/-/reload
error during the initial load: /Users/user1/Desktop/git.alloy:13:1: Failed to build component: loading custom component controller: custom component config not found in the registry, namespace: "math", componentName: "add"
```

## `/-/support`

The `/-/support` endpoint returns a [support bundle](../../troubleshoot/support_bundle) that contains information about your {{< param "PRODUCT_NAME" >}} instance. You can use this information as a baseline when debugging an issue.

## `/debug/pprof`

The `/debug/pprof` endpoint returns a pprof Go [profile](../../troubleshoot/profile) that you can use to visualize and analyze profiling data.

## `/graphql`

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `/graphql` endpoint exposes a [GraphQL API](./graphql/) for querying various aspects of {{< param "PRODUCT_NAME" >}}. It is disabled by default. To enable it, set the `--server.http.enable-graphql` flag to `true`.

You can also enable an interactive GraphQL playground at `/graphql/playground` by setting the `--server.http.enable-graphql-playground` flag to `true`.

Refer to the [GraphQL API](./graphql/) documentation for the full schema and example queries.
