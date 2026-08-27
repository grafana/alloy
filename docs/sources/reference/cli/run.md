---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/run/
description: Learn about the run command
labels:
  stage: general-availability
  products:
    - oss
title: run
weight: 300
---

# `run`

The `run` command runs the {{< param "PRODUCT_NAME" >}} Default Engine in the foreground until an interrupt is received.

## Usage

```shell
alloy run [<FLAG> ...] <PATH_NAME>
```

Replace the following:

* _`<FLAG>`_: One or more flags that define the input and output of the command.
* _`<PATH_NAME>`_: Required. The {{< param "PRODUCT_NAME" >}} configuration file or directory path.

If the _`<PATH_NAME>`_ argument isn't provided, or if the configuration path can't be loaded or contains errors during the initial load, the `run` command immediately exits and shows an error message.

If you provide a directory path for the _`<PATH_NAME>`_, {{< param "PRODUCT_NAME" >}} finds `*.alloy` files, ignoring nested directories, and loads them as a single configuration source.
However, component names must be **unique** across all {{< param "PRODUCT_NAME" >}} configuration files, and configuration blocks must not be repeated.

{{< param "PRODUCT_NAME" >}} continues to run if subsequent reloads of the configuration file fail, potentially marking components as unhealthy depending on the nature of the failure.
When this happens, {{< param "PRODUCT_NAME" >}} continues functioning in the last valid state.

`run` launches an HTTP server that exposes metrics about itself and its components.
The HTTP server exposes a UI at `/` for debugging running components.

The following flags are supported:

* `--server.http.enable-pprof`: Enable [`/debug/pprof`][] profiling endpoints. (default `true`).
* `--server.http.memory-addr`: Address to listen for [in-memory HTTP traffic][] on (default `"alloy.internal:12345"`).
* `--server.http.listen-addr`: Address to listen for HTTP traffic on (default `"127.0.0.1:12345"`).
* `--server.http.ui-path-prefix`: Base path where the UI is exposed (default `"/"`).
* `--server.http.disable-support-bundle`: Disable [support bundle][] endpoint (default `false`).
* `--storage.path`: Base directory where components can store data (default `"data-alloy/"`).
* `--disable-reporting`: Disable [data collection][] (default `false`).
* `--cluster.enabled`: Start {{< param "PRODUCT_NAME" >}} in clustered mode (default `false`).
* `--cluster.node-name`: The name to use for this node (defaults to the environment's hostname).
* `--cluster.join-addresses`: Comma-separated list of addresses to join the cluster at (default `""`). Mutually exclusive with `--cluster.discover-peers`.
* `--cluster.discover-peers`: List of key-value tuples for discovering peers (default `""`). Mutually exclusive with `--cluster.join-addresses`.
* `--cluster.rejoin-interval`: How often to rejoin the list of peers (default `"60s"`).
* `--cluster.advertise-address`: Address to advertise to other cluster nodes (default `""`).
* `--cluster.advertise-interfaces`: List of interfaces used to infer an address to advertise. Set to `all` to use all available network interfaces on the system. (default `"eth0,en0"`).
* `--cluster.max-join-peers`: Number of peers to join from the discovered set (default `5`).
* `--cluster.name`: Name to prevent nodes without this identifier from joining the cluster (default `""`).
* `--cluster.enable-tls`: Specifies whether TLS should be used for communication between peers (default `false`).
* `--cluster.tls-ca-path`: Path to the CA certificate file used for peer communication over TLS.
* `--cluster.tls-cert-path`: Path to the certificate file used for peer communication over TLS.
* `--cluster.tls-key-path`: Path to the key file used for peer communication over TLS.
* `--cluster.tls-server-name`: Server name used for peer communication over TLS.
* `--cluster.wait-for-size`: Wait for the cluster to reach the specified number of instances before allowing components that use clustering to begin processing. Zero means disabled (default `0`).
* `--cluster.wait-timeout`: Maximum duration to wait for minimum cluster size before proceeding with available nodes. Zero means wait forever, no timeout (default `0`).
* `--config.format`: Specifies the source file format. Supported formats: `alloy`, `otelcol`, `prometheus`, `promtail`, and `static` (default `"alloy"`).
* `--config.bypass-conversion-errors`: Enable bypassing errors during conversion (default `false`).
* `--config.extra-args`: Extra arguments from the original format used by the converter.
* `--stability.level`: The minimum permitted stability level of functionality. Supported values: `experimental`, `public-preview`, and `generally-available` (default `"generally-available"`).
* `--feature.community-components.enabled`: Enable community components (default `false`).
* `--feature.component-shutdown-deadline`: Maximum duration to wait for a component to shut down before giving up and logging an error (default `"10m"`).
* `--feature.prometheus.direct-fanout.enabled`: Experimental. Enable direct fan-out for metric forwarding without a global label store.
* `--server.http.enable-graphql`: Experimental. Enable the [GraphQL API][] (default `false`).
* `--server.http.enable-graphql-playground`: Experimental. Enable the [GraphQL playground][] UI at `/graphql/playground` (default `false`). Requires `--server.http.enable-graphql`.
* `--windows.priority`: The priority to set for the {{< param "PRODUCT_NAME" >}} process when running on Windows. This is only available on Windows. Supported values: `above_normal`, `below_normal`, `normal`, `high`, `idle`, or `realtime` (default `"normal"`).

{{< admonition type="note" >}}
The `--feature.prometheus.direct-fanout.enabled`, `--server.http.enable-graphql`, and `--server.http.enable-graphql-playground` flags are [experimental][] features.
Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
To enable and use an experimental feature, you must set the `stability.level` [flag](#permitted-stability-levels) to `experimental`.

[experimental]: https://grafana.com/docs/release-life-cycle/
{{< /admonition >}}

{{< admonition type="note" >}}
The `--windows.priority` flag is in [Public preview][] and is not covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.

[Public preview]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../../../introduction/backward-compatibility/
{{< /admonition >}}

### Deprecated flags

* `--cluster.use-discovery-v1`: This flag is deprecated and has no effect.
* `--feature.prometheus.metric-validation-scheme`: This flag is deprecated and has no effect. You can configure the metric validation scheme individually for each `prometheus.scrape` component in your {{< param "PRODUCT_NAME" >}} configuration file.

## Update the configuration file

The configuration file can be reloaded from disk by either:

* Sending an HTTP POST request to the `/-/reload` endpoint.
* Sending a `SIGHUP` signal to the {{< param "PRODUCT_NAME" >}} process.

When this happens, the [component controller][] synchronizes the set of running components with the latest set of components specified in the configuration file.
Components that are no longer defined in the configuration file after reloading are shut down, and components that have been added to the configuration file since the previous reload are created.

All components managed by the component controller are reevaluated after reloading.

## Permitted stability levels

By default, {{< param "PRODUCT_NAME" >}} only allows you to use functionality that is marked _Generally available_.

To use [Experimental][stability] or [Public preview][stability] functionality, set the `--stability.level` flag to the level you want to use:

* `--stability.level=experimental`: Use functionality marked as _Experimental_ and above.
* `--stability.level=public-preview`: Use functionality marked as _Public preview_ and above.
* `--stability.level=generally-available`: Use functionality marked as _Generally available_.

{{< admonition type="caution" >}}
Setting `--stability.level` to `experimental` or `public-preview` may enable _Experimental_ or _Public preview_ behavior for items otherwise marked _Generally available_, such as:

* The component controller
* Components in the main configuration or in imported modules
* Configuration blocks in the main configuration
{{< /admonition >}}

[stability]: https://grafana.com/docs/release-life-cycle/

Refer to [Release life cycle for Grafana Labs](https://grafana.com/docs/release-life-cycle/) for the definition of each stability level.

## Clustering

The `--cluster.enabled` flag starts {{< param "PRODUCT_NAME" >}} in [clustering][] mode.
To configure clustering, refer to [Configure {{< param "PRODUCT_NAME" >}} for clustering][configure-clustering].

Use the other `--cluster.*` flags to control peer discovery, advertised addresses, TLS, rejoining, cluster readiness, and cluster naming.

### Join address format

The comma-separated list of addresses provided in `--cluster.join-addresses` can include IP addresses or DNS names.
In both cases, you can specify the port number with a `:<port>` suffix.
If you don't provide a port, {{< param "PRODUCT_NAME" >}} uses the port from the HTTP listener.
If you don't provide the port number explicitly, you must ensure that all instances use the same port for the HTTP listener.
To select a DNS query type, add one of the following prefixes to the address:

* **`dns+`**\
The domain name after the prefix is looked up as an A/AAAA query.\
For example: `dns+alloy.local:11211`.
* **`dnssrv+`**\
The domain name after the prefix is looked up as a SRV query, and then each SRV record is resolved as an A/AAAA record.\
For example: `dnssrv+_alloy._tcp.alloy.namespace.svc.cluster.local`.
* **`dnssrvnoa+`**\
The domain name after the prefix is looked up as a SRV query, with no A/AAAA lookup made after that.\
For example: `dnssrvnoa+_alloy-memberlist._tcp.service.consul`

If no prefix is provided, {{< param "PRODUCT_NAME" >}} attempts to resolve the name using both A/AAAA and DNSSRV queries.

The `--cluster.discover-peers` flag expects a list of tuples in the form of `provider=XXX key=val key=val ...`.
If a key or value contains a space, a backslash, or double quotes, quote it with double quotes.
Within quoted strings, use a backslash to escape double quotes or the backslash itself.

### Clustering states

Clustered {{< param "PRODUCT_NAME" >}} instances are in one of three states:

* **Viewer**: {{< param "PRODUCT_NAME" >}} has a read-only view of the cluster and isn't participating in workload distribution.
* **Participant**: {{< param "PRODUCT_NAME" >}} is participating in workload distribution for components that have clustering enabled.
* **Terminating**: {{< param "PRODUCT_NAME" >}} is shutting down and no longer assigning new work to itself.

Each {{< param "PRODUCT_NAME" >}} initially joins the cluster in the viewer state and then transitions to the participant state after the process startup completes.
Each {{< param "PRODUCT_NAME" >}} then transitions to the terminating state when shutting down.

The current state of a clustered {{< param "PRODUCT_NAME" >}} is shown on the clustering page in the [UI][].

## Configuration conversion

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

When you use the `--config.format` command-line argument with a value other than `alloy`, {{< param "PRODUCT_NAME" >}} converts the configuration file from the source format to {{< param "PRODUCT_NAME" >}} and immediately starts running with the new configuration.
This conversion uses the converter API described in the [alloy convert][] docs.

If you include the `--config.bypass-conversion-errors` command-line argument, {{< param "PRODUCT_NAME" >}} ignores errors from the converter.
Use this argument with caution because the resulting conversion may not be equivalent to the original configuration.

Include `--config.extra-args` to pass additional command line flags from the original format to the converter.
Refer to [alloy convert][] for more details on how `extra-args` work.

[alloy convert]: ../convert/
[clustering]:  ../../../get-started/clustering/
[configure-clustering]: ../../../configure/clustering/configure-alloy/
[in-memory HTTP traffic]: ../../../get-started/component_controller/#in-memory-traffic
[data collection]: ../../../data-collection/
[support bundle]: ../../../troubleshoot/support_bundle/
[component controller]: ../../../get-started/component_controller/
[UI]: ../../../troubleshoot/debug/#clustering-page
[`/debug/pprof`]: http://pkg.go.dev/net/http/pprof
[GraphQL API]: ../../http/graphql/
[GraphQL playground]: ../../http/graphql/#graphql-playground
