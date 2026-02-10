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

If you provide a directory path for  the _`<PATH_NAME>`_, {{< param "PRODUCT_NAME" >}} finds `*.alloy` files, ignoring nested directories, and loads them as a single configuration source.
However, component names must be **unique** across all {{< param "PRODUCT_NAME" >}} configuration files, and configuration blocks must not be repeated.

{{< param "PRODUCT_NAME" >}} continues to run if subsequent reloads of the configuration file fail, potentially marking components as unhealthy depending on the nature of the failure.
When this happens, {{< param "PRODUCT_NAME" >}} continues functioning in the last valid state.

`run` launches an HTTP server that exposes metrics about itself and its components.
The HTTP server is also exposes a UI at `/` for debugging running components.

The following flags are supported:

* `--server.http.enable-pprof`: Enable [`/debug/pprof`][] profiling endpoints. (default `true`).
* `--server.http.memory-addr`: Address to listen for [in-memory HTTP traffic][] on (default `"alloy.internal:12345"`).
* `--server.http.listen-addr`: Address to listen for HTTP traffic on (default `"127.0.0.1:12345"`).
* `--server.http.ui-path-prefix`: Base path where the UI is exposed (default `"/"`).
* `--storage.path`: Base directory where components can store data (default `"data-alloy/"`).
* `--disable-reporting`: Disable [data collection][] (default `false`).
* `--disable-support-bundle`: Disable [support bundle][] endpoint (default `false`).
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
* `--windows.priority`: The priority to set for the {{< param "PRODUCT_NAME" >}} process when running on Windows. This is only available on Windows. Supported values: `above_normal`, `below_normal`, `normal`, `high`, `idle`, or `realtime` (default `"normal"`).

{{< admonition type="note" >}}
The `--windows.priority` flag is in [Public preview][] and is not covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.

### Deprecated flags

* `--feature.prometheus.metric-validation-scheme`: This flag is deprecated and has no effect. You can configure the metric validation scheme individually for each `prometheus.scrape` component in your {{< param "PRODUCT_NAME" >}} configuration file.

[Public preview]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../../../introduction/backward-compatibility/
{{< /admonition >}}

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

The `--cluster.enabled` command-line argument starts {{< param "PRODUCT_NAME" >}} in [clustering][] mode.
The rest of the `--cluster.*` command-line flags can be used to configure how nodes discover and connect to one another.

Each cluster member's name must be unique within the cluster.
Nodes which try to join with a conflicting name are rejected and fall back to bootstrapping a new cluster of their own.

Peers communicate over HTTP/2 on the built-in HTTP server.
Each node must be configured to accept connections on `--server.http.listen-addr` and the address defined or inferred in `--cluster.advertise-address`.

If the `--cluster.advertise-address` flag isn't explicitly set, {{< param "PRODUCT_NAME" >}} tries to infer a suitable one from `--cluster.advertise-interfaces`.
If `--cluster.advertise-interfaces` isn't explicitly set, {{< param "PRODUCT_NAME" >}} infers one from the `eth0` and `en0` local network interfaces.
{{< param "PRODUCT_NAME" >}} will fail to start if it can't determine the advertised address.
Since Windows doesn't use the interface names `eth0` or `en0`, Windows users must explicitly pass at least one valid network interface for `--cluster.advertise-interfaces` or a value for `--cluster.advertise-address`.

The comma-separated list of addresses provided in `--cluster.join-addresses` can either be IP addresses or DNS names to lookup (supports SRV and A/AAAA records).
In both cases, the port number can be specified with a `:<port>` suffix. If ports aren't provided, default of the port used for the HTTP listener is used.
If you don't provide the port number explicitly, you must ensure that all instances use the same port for the HTTP listener.
Optionally, you may specify a DNS query type as a prefix for each address. See [join addresses format](#join-address-format) for more information.

The `--cluster.enable-tls` flag can be set to enable TLS for peer-to-peer communications.
Additional arguments are required to configure the TLS client, including the CA certificate, the TLS certificate, the key, and the server name.

The `--cluster.discover-peers` command-line flag expects a list of tuples in the form of `provider=XXX key=val key=val ...`.
Clustering uses the [go-discover] package to discover peers and fetch their IP addresses, based on the chosen provider and the filtering key-values it supports.
Clustering supports the default set of providers available in go-discover and registers the `k8s` provider on top.

If either the key or the value in a tuple pair contains a space, a backslash, or double quotes, then it must be quoted with double quotes.
Within this quoted string, the backslash can be used to escape double quotes or the backslash itself.

The `--cluster.rejoin-interval` flag defines how often each node should rediscover peers based on the contents of the `--cluster.join-addresses` and `--cluster.discover-peers` flags and try to rejoin them.
This operation is useful for addressing split-brain issues if the initial bootstrap is unsuccessful and for making clustering easier to manage in dynamic environments.
To disable this behavior, set the `--cluster.rejoin-interval` flag to `"0s"`.

If `--cluster.rejoin-interval` is set to `0s`, then discovering peers using the `--cluster.join-addresses` and `--cluster.discover-peers` flags only happens at startup. After that, cluster nodes depend on gossiping messages with each other to converge on the cluster's state.

The first node that's used to bootstrap a new cluster (also known as the "seed node") can either omit the flags that specify peers to join or can try to connect to itself.

To join or rejoin a cluster, {{< param "PRODUCT_NAME" >}} tries to connect to a number of random peers limited by the `--cluster.max-join-peers` flag.
This flag can be useful for clusters of significant sizes because connecting to a high number of peers can be an expensive operation.
To disable this behavior, set the `--cluster.max-join-peers` flag to 0.
If the value of `--cluster.max-join-peers` is higher than the number of peers discovered, {{< param "PRODUCT_NAME" >}} connects to all of them.

The `--cluster.wait-for-size` flag specifies the minimum cluster size required before components that use clustering
begin processing traffic. When set to a value greater than zero, a node will join the cluster but the components that
use clustering will not take on any work until enough nodes are available. This ensures adequate cluster capacity - refer to
[estimate resource usage][] for guidelines. The default value is `0`, which disables this feature.

The `--cluster.wait-timeout` flag sets how long a node will wait for the cluster to reach the size specified by
`--cluster.wait-for-size`. If the timeout expires, the node will proceed with available nodes. Setting this to `0` (the
default) means wait indefinitely. For production environments, consider setting a timeout of several minutes as a
fallback.

The `--cluster.name` flag can be used to prevent clusters from accidentally merging.
When `--cluster.name` is provided, nodes only join peers who share the same cluster name value.
By default, the cluster name is empty, and any node that doesn't set the flag can join.
Attempting to join a cluster with a wrong `--cluster.name` results in a "failed to join memberlist" error.

### Join Address Format

The `--cluster.join-addresses` flag supports DNS names with discovery mode prefix.
You select a discovery mode by adding one of the following supported prefixes to the address:

* **`dns+`**\
The domain name after the prefix is looked up as an A/AAAA query.\
For example: `dns+alloy.local:11211`.
* **`dnssrv+`**\
The domain name after the prefix is looked up as a SRV query, and then each SRV record is resolved as an A/AAAA record.\
For example: `dnssrv+_alloy._tcp.alloy.namespace.svc.cluster.local`.
* **`dnssrvnoa+`**\
The domain name after the prefix is looked up as a SRV query, with no A/AAAA lookup made after that.\
For example: `dnssrvnoa+_alloy-memberlist._tcp.service.consul`

If no prefix is provided, Alloy will attempt to resolve the name using both A/AAAA and DNSSRV queries.

### Clustering states

Clustered {{< param "PRODUCT_NAME" >}}s are in one of three states:

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
[go-discover]: https://github.com/hashicorp/go-discover
[in-memory HTTP traffic]: ../../../get-started/component_controller/#in-memory-traffic
[data collection]: ../../../data-collection/
[support bundle]: ../../../troubleshoot/support_bundle/
[component controller]: ../../../get-started/component_controller/
[UI]: ../../../troubleshoot/debug/#clustering-page
[estimate resource usage]: ../../../introduction/estimate-resource-usage/
[`/debug/pprof`]: http://pkg.go.dev/net/http/pprof
