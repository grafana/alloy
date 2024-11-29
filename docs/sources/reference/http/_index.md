---
canonical: https://grafana.com/docs/alloy/latest/reference/http/
description: Learn about HTTP endpoints exposed by Grafana Alloy
title: HTTP endpoints
weight: 700
---

# The {{% param "FULL_PRODUCT_NAME" %}} HTTP endpoints

There are HTTP endpoints which are enabled by default on every instance of {{% param "FULL_PRODUCT_NAME" %}}, 
regardless which components are configured.
They can be used for monitoring, health checking, and troubleshooting.

The HTTP server which exposes them is configured via the [http block](../config-blocks/http)
and the `--server.` [command line arguments](../cli/run).
For example, if the `--server.http.listen-addr` command line argument is set to `127.0.0.1:12345`, 
you can query the `127.0.0.1:12345/metrics` endpoint to see the internal metrics of {{% param "FULL_PRODUCT_NAME" %}}.

### /metrics

Displays the internal metrics of {{% param "FULL_PRODUCT_NAME" %}} in the Prometheus exposition format.

### /-/ready

A {{% param "FULL_PRODUCT_NAME" %}} instance is "ready" once it has loaded its initial configuration.
If it is ready, HTTP 200 and the message `Alloy is ready.` are returned.
Otherwise, HTTP 503 and the message `Alloy is not ready.` are returned.

### /-/healthy

If all components are healthy, HTTP 200 and the message "Alloy is healthy." will be returned.
Otherwise, {{% param "FULL_PRODUCT_NAME" %}} will return HTTP 500 and an error message.
You can also monitor component health through the [UI](../../troubleshoot/debug#alloy-ui).

```
$ curl localhost:12345/-/healthy
Alloy is healthy.
```

```
$ curl localhost:12345/-/healthy
unhealthy components: math.add
```

### /-/reload

Reloads the {{% param "FULL_PRODUCT_NAME" %}} configuration file. Returns HTTP 400 and an error message if an issue with the reload was encountered.

```
$ curl localhost:12345/-/reload
config reloaded
```

```
$ curl localhost:12345/-/reload
error during the initial load: /Users/user1/Desktop/git.alloy:13:1: Failed to build component: loading custom component controller: custom component config not found in the registry, namespace: "math", componentName: "add"
```

### /-/support

Generates a [support bundle](../../troubleshoot/support_bundle).

### /debug/pprof

Generates a [profile](../../troubleshoot/profile).