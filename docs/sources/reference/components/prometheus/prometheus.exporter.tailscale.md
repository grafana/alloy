---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.tailscale/
aliases:
  - ../prometheus.exporter.tailscale/ # /docs/alloy/latest/reference/components/prometheus.exporter.tailscale/
description: Learn about prometheus.exporter.tailscale
labels:
  stage: experimental
  products:
    - oss
title: prometheus.exporter.tailscale
---

# `prometheus.exporter.tailscale`

`prometheus.exporter.tailscale` embeds an embedded Tailscale node in Grafana Alloy using [tsnet](https://pkg.go.dev/tailscale.com/tsnet), queries the Tailscale management API, and scrapes per-node Tailscale daemon metrics from each peer in the tailnet.

The component exposes three types of metrics:

- **Tailnet device status**—per-device authorization, online status, key expiry, and last seen timestamps, collected from the Tailscale management API.
- **Tailnet aggregates**—summary counts of total, online, and authorized devices.
- **Per-node daemon metrics**—raw Prometheus metrics scraped from port 5252 on each peer via the tsnet VPN, with a `node` label added to identify the source device.

{{< admonition type="note" >}}
`prometheus.exporter.tailscale` is experimental. Its behavior may change in future releases.
{{< /admonition >}}

## Prerequisites

Before per-node daemon metrics can be scraped, configure two things in your tailnet.

### Grant the embedded node access to the metrics ports

The embedded tsnet node joins the tailnet regardless of your tailnet policy, but data-plane connections to peers are governed by ACLs. Without a grant, every scrape is silently dropped and appears as a `context deadline exceeded` timeout, not `connection refused`. Add a grant allowing the node's tag to reach the metrics ports on your peers. For a node advertising `tag:ts-alloy` scraping Tailscale daemon metrics on `5252` and, for example, `node_exporter` on `9100`:

```json
{
  "src": ["tag:ts-alloy"],
  "dst": ["*"],
  "ip":  ["5252", "9100"]
}
```

### Enable client metrics on each device

Tailscale clients don't expose metrics on port `5252` by default. Run `tailscale set --webclient` on each device you want to scrape. Devices that haven't enabled metrics return `connection refused` and are skipped silently.

For details, refer to the Tailscale [client metrics](https://tailscale.com/kb/1482/client-metrics) documentation.

{{< admonition type="note" >}}
The Tailscale Kubernetes operator exposes its metrics on the cluster network, not over the tailnet. The embedded node can't scrape them, so operator-managed devices (proxies and ingresses) contribute only control-plane API status (online, authorized, key expiry), not per-node daemon metrics.
{{< /admonition >}}

## Usage

```alloy
prometheus.exporter.tailscale "<LABEL>" {
  tailnet  = "<TAILNET>"
  auth_key = "<TSNET_AUTH_KEY>"
  api_key  = "<API_KEY>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.tailscale`:

| Name                | Type       | Description                                                                               | Default                          | Required |
| ------------------- | ---------- | ----------------------------------------------------------------------------------------- | -------------------------------- | -------- |
| `tailnet`           | `string`   | Name of the tailnet to monitor (for example, `"example.com"`).                            |                                  | yes      |
| `api_key`           | `secret`   | Tailscale API key (`tskey-api-...`) used to query the management API.                     |                                  | no       |
| `api_key_file`      | `string`   | Path to a file containing the API key. Mutually exclusive with `api_key`.                 |                                  | no       |
| `auth_key`          | `secret`   | Tailscale pre-auth key (`tskey-auth-...`) used by the embedded node to join the tailnet.  |                                  | no       |
| `state_dir`         | `string`   | Directory for persistent tsnet state (WireGuard keys, certificates).                      | Component data path + `/tsnet`   | no       |
| `tsnet_hostname`    | `string`   | Hostname used by the embedded tsnet node when joining the tailnet.                        | `"alloy-tailscale-exporter"`     | no       |
| `api_base_url`      | `string`   | Base URL of the Tailscale management API.                                                 | `"https://api.tailscale.com"`    | no       |
| `refresh_interval`  | `duration` | How often to poll the API and scrape peer metrics.                                        | `"60s"`                          | no       |
| `peer_metrics_port` | `number`   | Port scraped on each peer when no `target` blocks are configured.                         | `5252`                           | no       |
| `peer_metrics_path` | `string`   | HTTP path scraped on each peer when no `target` blocks are configured.                    | `"/metrics"`                     | no       |

### Authentication

Choose one of two modes. They're mutually exclusive.

**API key + auth key** (simplest):

- **`api_key`** (or **`api_key_file`**): A Tailscale API key (`tskey-api-...`) generated under **Settings > API Keys**. Used for management API calls to list devices and their status.
- **`auth_key`**: A Tailscale pre-auth key (`tskey-auth-...`) generated under **Settings > Auth Keys**. Used once when the embedded node first joins the tailnet; after that, credentials are persisted in `state_dir` and the key isn't consumed again.

**OAuth** (recommended—scoped, short-lived tokens, no long-lived keys): configure an [`oauth`][oauth] block instead. A single OAuth client authenticates both the management API (via the client-credentials flow) and the tsnet node join (the node mints its own tagged auth keys). This replaces both `api_key` and `auth_key`.

### State directory

The embedded tsnet node stores WireGuard private keys, node certificates, and other persistent state in `state_dir`. This directory must be on persistent storage. If Alloy restarts and `state_dir` is empty or missing (for example, on a Kubernetes Pod with ephemeral storage), the node re-authenticates using `auth_key` and consumes a new auth key slot.

When running multiple instances of `prometheus.exporter.tailscale` in the same Alloy process, each instance must have a unique `tsnet_hostname` and a separate `state_dir`.

## Blocks

You can use the following block with `prometheus.exporter.tailscale`:

| Block               | Description                                                              | Required |
| ------------------- | ------------------------------------------------------------------------ | -------- |
| [`oauth`][oauth]    | Authenticate with a Tailscale OAuth client instead of `api_key`/`auth_key`. | no       |
| [`target`][target]  | Map a group of nodes (by tag) to the port/path where they expose metrics. | no       |

[oauth]: #oauth
[target]: #target

### `oauth`

Authenticate with a Tailscale OAuth client instead of `api_key` + `auth_key`. Mutually exclusive with `api_key`/`api_key_file`.

| Name                 | Type           | Description                                                                | Default | Required |
| -------------------- | -------------- | -------------------------------------------------------------------------- | ------- | -------- |
| `client_id`          | `string`       | OAuth client ID.                                                           |         | yes      |
| `client_secret`      | `secret`       | OAuth client secret. Mutually exclusive with `client_secret_file`.         |         | no       |
| `client_secret_file` | `string`       | Path to a file containing the OAuth client secret.                         |         | no       |
| `advertise_tags`     | `list(string)` | Tags applied to the embedded node. Required in practice, because OAuth-generated auth keys must be tagged, and the tags must be owned by the OAuth client in the tailnet policy. | `[]` | no |

Exactly one of `client_secret` or `client_secret_file` is required.

```alloy
prometheus.exporter.tailscale "default" {
  tailnet = "example.com"

  oauth {
    client_id      = sys.env("TS_OAUTH_CLIENT_ID")
    client_secret  = sys.env("TS_OAUTH_CLIENT_SECRET")
    advertise_tags = ["tag:ts-alloy"]
  }
}
```

### `target`

Different nodes can expose metrics on different ports. Tailscale clients serve daemon metrics on `5252`, while a node running an additional exporter—for example, `node_exporter`—can expose it on another port such as `9100`, as long as that exporter binds to the node's tailnet address. Use `target` blocks to scrape each group on the right port. A node matches the first `target` whose `match_tags` it satisfies, so it's scraped on a single port. When you specify no `target` blocks, every node is scraped on `peer_metrics_port`/`peer_metrics_path`.

| Name         | Type           | Description                                                                       | Default      | Required |
| ------------ | -------------- | --------------------------------------------------------------------------------- | ------------ | -------- |
| `port`       | `number`       | Port to scrape on matched nodes.                                                  |              | yes      |
| `match_tags` | `list(string)` | Tag glob patterns (for example `"tag:*-proxy"`). A node matches if any of its tags matches any pattern. An empty list matches every node, so it acts as a catch-all—list it last. | `[]`         | no       |
| `path`       | `string`       | HTTP path to scrape on matched nodes.                                             | `"/metrics"` | no       |

```alloy
prometheus.exporter.tailscale "example" {
  tailnet  = "example.com"
  api_key  = sys.env("TS_API_KEY")
  auth_key = sys.env("TS_AUTHKEY")

  // Nodes running node_exporter bound to their tailnet address.
  target {
    match_tags = ["tag:node-exporter"]
    port       = 9100
  }
  // Everything else: Tailscale daemon metrics on 5252.
  target {
    match_tags = [] // everything else
    port       = 5252
  }
}
```

Per-node daemon metrics are labeled with `node` (hostname), `tags`, and `os` so you can distinguish node types (clients, exit nodes, subnet routers, and so on) in queries.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.tailscale` is reported as unhealthy if given an invalid configuration, or if the embedded tsnet node fails to join the tailnet.

During normal operation, if a single API call or peer scrape fails, the component continues running and exports stale or partial metrics. It doesn't become unhealthy for transient errors.

## Debug information

`prometheus.exporter.tailscale` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.tailscale` doesn't expose any component-specific debug metrics.

## Metrics

In addition to per-node daemon metrics scraped from the peer metrics ports, the component exposes the following metrics at its own `/metrics` endpoint.

### Tailnet aggregates

| Metric name                       | Type  | Description                                              |
| --------------------------------- | ----- | -------------------------------------------------------- |
| `tailscale_devices_total`         | Gauge | Total number of devices in the tailnet.                  |
| `tailscale_devices_online_total`  | Gauge | Number of devices seen within the last 5 minutes.        |
| `tailscale_devices_authorized_total` | Gauge | Number of authorized devices in the tailnet.          |

### Per-device status

All per-device metrics include `name` and `id` labels identifying the device.

| Metric name                            | Type  | Labels                            | Description                                             |
| -------------------------------------- | ----- | --------------------------------- | ------------------------------------------------------- |
| `tailscale_device_info`                | Gauge | `name`, `id`, `os`, `tailscale_ip` | Static device info. Always 1.                          |
| `tailscale_device_authorized`          | Gauge | `name`, `id`                      | Whether the device is authorized (1) or not (0).       |
| `tailscale_device_online`              | Gauge | `name`, `id`                      | Whether the device was seen in the last 5 minutes.     |
| `tailscale_device_last_seen_seconds`   | Gauge | `name`, `id`                      | Unix timestamp when the device was last seen.          |
| `tailscale_device_key_expiry_seconds`  | Gauge | `name`, `id`                      | Unix timestamp when the device's key expires. `0` if key expiry is disabled. |
| `tailscale_device_update_available`    | Gauge | `name`, `id`                      | Whether a Tailscale client update is available (1) or not (0). |

### Exporter health

| Metric name                                                  | Type    | Labels | Description                                                    |
| ------------------------------------------------------------ | ------- | ------ | -------------------------------------------------------------- |
| `tailscale_exporter_last_refresh_success_timestamp_seconds`  | Gauge   |—| Unix timestamp of the last successful refresh cycle.           |
| `tailscale_exporter_last_refresh_duration_seconds`           | Gauge   |—| Duration in seconds of the last full refresh cycle.            |
| `tailscale_exporter_peer_scrape_errors_total`                | Counter | `node` | Total number of errors scraping per-node metrics.              |
| `tailscale_exporter_api_errors_total`                        | Counter |—| Total number of Tailscale management API errors.               |

### Per-node daemon metrics

The component scrapes `http://<tailscale_ip>:<peer_metrics_port><peer_metrics_path>` on each device using an HTTP client that routes traffic through the tsnet VPN. The raw Prometheus metrics from each peer are re-exposed with an additional `node=<hostname>` label.

Common metrics produced by the Tailscale daemon include counters for inbound and outbound packets and bytes, WireGuard peer counts, and DERP connection statistics. Devices that don't expose metrics on this port are skipped silently.

## Examples

### Basic configuration

The following example scrapes a tailnet named `example.com` and forwards metrics to Grafana Cloud:

```alloy
prometheus.exporter.tailscale "default" {
  tailnet  = "example.com"
  auth_key = env("TS_AUTHKEY")
  api_key  = env("TS_API_KEY")
}

prometheus.scrape "tailscale" {
  targets    = prometheus.exporter.tailscale.default.targets
  forward_to = [prometheus.remote_write.grafana_cloud.receiver]
}

prometheus.remote_write "grafana_cloud" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Custom refresh interval and state directory

The following example uses a faster refresh interval and an explicit state directory for the tsnet node:

```alloy
prometheus.exporter.tailscale "prod" {
  tailnet          = "example.com"
  auth_key         = env("TS_AUTHKEY")
  api_key          = env("TS_API_KEY")
  state_dir        = "/var/lib/alloy/tailscale-state"
  tsnet_hostname   = "alloy-prod-monitor"
  refresh_interval = "30s"
}

prometheus.scrape "tailscale_prod" {
  targets    = prometheus.exporter.tailscale.prod.targets
  forward_to = [prometheus.remote_write.grafana_cloud.receiver]
}
```

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.tailscale` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
