# Clustering debug tooling

Tooling to help diagnose Alloy [clustering][] issues — in particular questions
that are hard to answer today, such as *which pod is scraping which target*,
*which endpoint is slow to respond*, and *are targets distributed evenly across
the cluster*.

[clustering]: https://grafana.com/docs/alloy/latest/get-started/clustering/

It relies only on data Alloy already exposes over its internal HTTP API — no
changes to Alloy or Prometheus, no extra metrics, no tracing:

| Endpoint | Used for |
| --- | --- |
| `GET /api/v0/web/peers` | Cluster peers and which one is "self" |
| `GET /api/v0/web/components` | Component list + health |
| `GET /api/v0/web/components/{id}` | Per-component `debugInfo`, including per-target health/URL/last-scrape/duration/error for scrape components |

> The built-in support bundle (`/-/support`) includes peers and the component
> list, but **not** the per-component target debug info, which is the data
> needed to attribute targets to pods. That is why this tool also hits the
> single-component endpoint.

## `clustering-debug` (Go)

Collects clustering + scrape-target state from **one** Alloy instance and prints
it as a single JSON document. Run it against each pod in a cluster and feed the
results to an analysis step that correlates ownership across the fleet.

The per-target debug data is large and nested (a busy `prometheus.scrape` can
hold thousands of targets, each with a full label set), so it is decoded with
real types rather than ad-hoc text munging. It's a self-contained Go module
using only the standard library.

### Usage

**StatefulSet mode (recommended for a cluster)** — collect from every pod
automatically. The tool finds the pods owned by the StatefulSet, port-forwards
each one in turn with `kubectl` (letting kubectl pick a free local port), and
writes one `<pod>.json` file per pod. Requires `kubectl` on your PATH and a
working kube-context.

```bash
cd operations/clustering-debug

# -remote-port is the Alloy HTTP port inside the pod.
go run . -statefulset grafana-agent-helm -remote-port 3090 -namespace grafana-agent -output-dir ./dump
```

This produces `./dump/alloy-0.json`, `./dump/alloy-1.json`, … Each file is
tagged with its `pod` name.

**Single-instance mode** — point at one already-reachable instance (e.g. a pod
you've port-forwarded yourself):

```bash
kubectl port-forward pod/alloy-0 12345:12345
go run . http://localhost:12345 -output alloy-0.json
```

Or build once and reuse:

```bash
go build -o clustering-debug .
./clustering-debug http://localhost:12345 > alloy-0.json
```

### Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-statefulset NAME` | — | StatefulSet mode: collect from every pod owned by this StatefulSet via `kubectl port-forward` |
| `-remote-port PORT` | — | StatefulSet mode: the pod port to forward to (the Alloy HTTP port). Required with `-statefulset` |
| `-namespace`, `-n` | current context | Kubernetes namespace for `-statefulset` |
| `-output-dir DIR` | `.` | StatefulSet mode: directory to write `<pod>.json` files into |
| `-output FILE` | stdout | Single-instance mode: write JSON to a file instead of stdout |
| `-filter REGEX` | `scrape` | Unanchored regex matched against component names; selects which components have their targets fetched into `scrape_components`. Does **not** affect the full `components` inventory. |
| `-all` | off | Fetch debug info for every component, not just matching ones |
| `-timeout DURATION` | `10s` | Per-request timeout (e.g. `30s`) |
| `-insecure` | off | Skip TLS certificate verification (for HTTPS endpoints) |
| `-header 'K: V'` | — | Extra HTTP header (repeatable), e.g. for authentication |
| `-api-prefix PATH` | `/api/v0/web` | API path prefix; falls back automatically if the default 404s |

### Output shape

```json
{
  "source": "http://localhost:12345",
  "api_prefix": "/api/v0/web",
  "fetched_at": "2026-06-10T12:00:00Z",
  "self": { "Name": "alloy-0", "Addr": "10.0.0.1:12345", "Self": true, "State": "participant" },
  "peers": [ { "Name": "...", "Addr": "...", "Self": false, "State": "..." } ],
  "components": [
    { "id": "prometheus.scrape.foo", "name": "prometheus.scrape", "module_id": "", "label": "foo", "health": { "state": "healthy" } }
  ],
  "scrape_components": [
    {
      "id": "prometheus.scrape.foo",
      "name": "prometheus.scrape",
      "label": "foo",
      "health": { "state": "healthy" },
      "target_count": 2,
      "targets": [
        {
          "job": "node",
          "url": "http://10.0.0.5:9100/metrics",
          "health": "up",
          "last_error": "",
          "last_scrape": "2026-06-10T12:00:00Z",
          "last_scrape_duration": "12.3ms",
          "labels": { "instance": "10.0.0.5:9100", "job": "node" }
        }
      ]
    }
  ],
  "errors": []
}
```

`components` is always the **full** component inventory with health (not affected
by `-filter`); it provides an at-a-glance overview and the `discovery.*` entries
needed for later gap analysis. `scrape_components` is the `-filter`-selected
subset, with per-target detail. If you only care about the scrape data, read
`.scrape_components`.

If clustering is disabled, the `/peers` call returns an error that is recorded
in `errors` while the rest of the output is still produced.

## Why per-instance gives the full picture

Each instance only reports the targets **it** actively scrapes — after
clustering distribution, Alloy feeds each `prometheus.scrape` only its
locally-owned targets. So running this against every pod and unioning the
results gives the whole fleet:

* **Overlap** — a target appearing under more than one pod.
* **Slow endpoints** — high `last_scrape_duration`, attributable to a pod.
* **Down/errored targets** — `health != "up"` or a non-empty `last_error`.
* **Imbalance** — skew in target counts across pods.
* **Gaps** — compare the union of scraped targets against the upstream
  `discovery.*` component exports to find discovered-but-unscraped targets.

Fanning out across all pods and correlating these is the planned next step
(an agent skill); this tool is the per-instance collector it builds on.

## Development

```bash
cd operations/clustering-debug
go test ./...
```
