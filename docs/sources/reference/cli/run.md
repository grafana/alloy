---
aliases:
- /docs/grafana-cloud/agent/flow/reference/cli/run/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/cli/run/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/cli/run/
- /docs/grafana-cloud/send-data/agent/flow/reference/cli/run/
canonical: https://grafana.com/docs/agent/latest/flow/reference/cli/run/
description: Learn about the run command
menuTitle: run
title: The run command
weight: 300
---

# The run command

The `run` command runs {{< param "PRODUCT_NAME" >}} in the foreground until an interrupt is received.

## Usage

Usage:

* `AGENT_MODE=flow grafana-agent run [FLAG ...] PATH_NAME`
* `grafana-agent-flow run [FLAG ...] PATH_NAME`

   Replace the following:

   * `FLAG`: One or more flags that define the input and output of the command.
   * `PATH_NAME`: Required. The {{< param "PRODUCT_NAME" >}} configuration file/directory path.

If the `PATH_NAME` argument is not provided, or if the configuration path can't be loaded or
contains errors during the initial load, the `run` command will immediately exit and show an error message.

If you give the `PATH_NAME` argument a directory path, {{< param "PRODUCT_NAME" >}} will find `*.river` files
(ignoring nested directories) and load them as a single configuration source. However, component names must
be **unique** across all River files, and configuration blocks must not be repeated.

{{< param "PRODUCT_NAME" >}} will continue to run if subsequent reloads of the configuration
file fail, potentially marking components as unhealthy depending on the nature
of the failure. When this happens, {{< param "PRODUCT_NAME" >}} will continue functioning
in the last valid state.

`run` launches an HTTP server that exposes metrics about itself and its
components. The HTTP server is also exposes a UI at `/` for debugging
running components.

The following flags are supported:

* `--server.http.enable-pprof`: Enable /debug/pprof profiling endpoints. (default `true`)
* `--server.http.memory-addr`: Address to listen for [in-memory HTTP traffic][] on
  (default `agent.internal:12345`).
* `--server.http.listen-addr`: Address to listen for HTTP traffic on (default `127.0.0.1:12345`).
* `--server.http.ui-path-prefix`: Base path where the UI is exposed (default `/`).
* `--storage.path`: Base directory where components can store data (default `data-agent/`).
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
* `--config.format`: The format of the source file. Supported formats: `flow`, `prometheus`, `promtail`, `static` (default `"flow"`).
* `--config.bypass-conversion-errors`: Enable bypassing errors when converting (default `false`).
* `--config.extra-args`: Extra arguments from the original format used by the converter.

[in-memory HTTP traffic]: {{< relref "../../concepts/component_controller.md#in-memory-traffic" >}}
[data collection]: {{< relref "../../../data-collection" >}}
[components]: {{< relref "../../concepts/components.md" >}}

## Update the configuration file

The configuration file can be reloaded from disk by either:

* Sending an HTTP POST request to the `/-/reload` endpoint.
* Sending a `SIGHUP` signal to the {{< param "PRODUCT_NAME" >}} process.

When this happens, the [component controller][] synchronizes the set of running
components with the latest set of components specified in the configuration file.
Components that are no longer defined in the configuration file after reloading are
shut down, and components that have been added to the configuration file since the
previous reload are created.

All components managed by the component controller are reevaluated after
reloading.

[component controller]: {{< relref "../../concepts/component_controller.md" >}}

## Clustering (beta)

The `--cluster.enabled` command-line argument starts {{< param "PRODUCT_ROOT_NAME" >}} in
[clustering][] mode. The rest of the `--cluster.*` command-line flags can be
used to configure how nodes discover and connect to one another.

Each cluster member’s name must be unique within the cluster. Nodes which try
to join with a conflicting name are rejected and will fall back to
bootstrapping a new cluster of their own.

Peers communicate over HTTP/2 on the built-in HTTP server. Each node
must be configured to accept connections on `--server.http.listen-addr` and the
address defined or inferred in `--cluster.advertise-address`.

If the `--cluster.advertise-address` flag isn't explicitly set, {{< param "PRODUCT_NAME" >}}
tries to infer a suitable one from `--cluster.advertise-interfaces`.
If `--cluster.advertise-interfaces` isn't explicitly set, {{< param "PRODUCT_NAME" >}} will
infer one from the `eth0` and `en0` local network interfaces.
{{< param "PRODUCT_NAME" >}} will fail to start if it can't determine the advertised address.
Since Windows doesn't use the interface names `eth0` or `en0`, Windows users must explicitly pass
at least one valid network interface for `--cluster.advertise-interfaces` or a value for `--cluster.advertise-address`.

The comma-separated list of addresses provided in `--cluster.join-addresses`
can either be IP addresses with an optional port, or DNS records to lookup.
The ports on the list of addresses default to the port used for the HTTP
listener if not explicitly provided. We recommend that you
align the port numbers on as many nodes as possible to simplify the deployment
process.

The `--cluster.discover-peers` command-line flag expects a list of tuples in
the form of `provider=XXX key=val key=val ...`. Clustering uses the
[go-discover] package to discover peers and fetch their IP addresses, based
on the chosen provider and the filtering key-values it supports. Clustering
supports the default set of providers available in go-discover and registers
the `k8s` provider on top.

If either the key or the value in a tuple pair contains a space, a backslash, or
double quotes, then it must be quoted with double quotes. Within this quoted
string, the backslash can be used to escape double quotes or the backslash
itself.

The `--cluster.rejoin-interval` flag defines how often each node should
rediscover peers based on the contents of the `--cluster.join-addresses` and
`--cluster.discover-peers` flags and try to rejoin them.  This operation
is useful for addressing split-brain issues if the initial bootstrap is
unsuccessful and for making clustering easier to manage in dynamic
environments. To disable this behavior, set the `--cluster.rejoin-interval`
flag to `"0s"`.

Discovering peers using the `--cluster.join-addresses` and
`--cluster.discover-peers` flags only happens on startup; after that, cluster
nodes depend on gossiping messages with each other to converge on the cluster's
state.

The first node that is used to bootstrap a new cluster (also known as
the "seed node") can either omit the flags that specify peers to join or can
try to connect to itself.

To join or rejoin a cluster, {{< param "PRODUCT_NAME" >}} will try to connect to a certain number of peers limited by the `--cluster.max-join-peers` flag.
This flag can be useful for clusters of significant sizes because connecting to a high number of peers can be an expensive operation.
To disable this behavior, set the `--cluster.max-join-peers` flag to 0.
If the value of `--cluster.max-join-peers` is higher than the number of peers discovered, {{< param "PRODUCT_NAME" >}} will connect to all of them.

The `--cluster.name` flag can be used to prevent clusters from accidentally merging.
When `--cluster.name` is provided, nodes will only join peers who share the same cluster name value.
By default, the cluster name is empty, and any node that doesn't set the flag can join.
Attempting to join a cluster with a wrong `--cluster.name` will result in a "failed to join memberlist" error.

### Clustering states

Clustered {{< param "PRODUCT_ROOT_NAME" >}}s are in one of three states:

* **Viewer**: {{< param "PRODUCT_NAME" >}} has a read-only view of the cluster and isn't participating in workload distribution.

* **Participant**: {{< param "PRODUCT_NAME" >}} is participating in workload distribution for components that have clustering enabled.

* **Terminating**: {{< param "PRODUCT_NAME" >}} is shutting down and will no longer assign new work to itself.

Each {{< param "PRODUCT_ROOT_NAME" >}} initially joins the cluster in the viewer state and then transitions to
the participant state after the process startup completes. Each {{< param "PRODUCT_ROOT_NAME" >}} then
transitions to the terminating state when shutting down.

The current state of a clustered {{< param "PRODUCT_ROOT_NAME" >}} is shown on the clustering page in the [UI][].

[UI]: {{< relref "../../tasks/debug.md#clustering-page" >}}

## Configuration conversion (beta)

When you use the `--config.format` command-line argument with a value
other than `flow`, {{< param "PRODUCT_ROOT_NAME" >}} converts the configuration file from
the source format to River and immediately starts running with the new
configuration. This conversion uses the converter API described in the
[grafana-agent-flow convert][] docs.

If you include the `--config.bypass-conversion-errors` command-line argument,
{{< param "PRODUCT_NAME" >}} will ignore any errors from the converter. Use this argument
with caution because the resulting conversion may not be equivalent to the
original configuration.

Include `--config.extra-args` to pass additional command line flags from the original format to the converter.
Refer to [grafana-agent-flow convert][] for more details on how `extra-args` work.

[grafana-agent-flow convert]: {{< relref "./convert.md" >}}
[clustering]:  {{< relref "../../concepts/clustering.md" >}}
[go-discover]: https://github.com/hashicorp/go-discover
