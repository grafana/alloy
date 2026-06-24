# Phase 2: Endpoint Gate

Gates which outbound URLs components may connect to, using glob pattern matching at runtime.

**Policy field added this phase:** `endpoints` section.

## Why not a central dialer

~80–100 egress touch points across 7–8 distinct HTTP client patterns (Prometheus common config, OTel factories, client-go, AWS SDK, SQL drivers, etc.). No single hook exists. A central dialer is a future goal. This phase gates at the component argument level instead.

## Interface design

```go
// internal/component/registry.go

type EgressSpec struct {
    // Literal endpoint URLs from the current argument values.
    // Empty if the component has no outbound connections.
    Endpoints []string
    // HasDynamic is true when the component connects to endpoints not
    // listed in Endpoints (e.g. discovery-driven targets, not static config).
    HasDynamic bool
}

type EgressComponent interface {
    EgressSpec() EgressSpec
}
```

`EgressSpec()` is called at `BuiltinComponentNode.evaluate()` time — Arguments are fully resolved at that point, so even expression-built URLs are real strings. `HasDynamic: true` covers components like `prometheus.scrape` whose targets come from discovery, not from a static URL in arguments.

## Policy field

```go
type EndpointsSection struct {
    Mode     string   // "allowlist" or "denylist"
    Patterns []string // URL globs, e.g. "https://grafana.com/*"
}

type SecurityPolicy struct {
    // ... previous phases ...
    Endpoints EndpointsSection `yaml:"endpoints"`
}
```

Pattern matching: `path.Match`-style glob on the full URL string (`scheme://host:port/path`). An endpoint passes if it matches at least one allowed pattern (allowlist mode) or matches no denied pattern (denylist mode).

## Files to change

### `internal/securitypolicy/policy.go`

Add `EndpointsSection` and `CheckEndpoint(url string) error`.

### `internal/component/registry.go`

Add `EgressSpec` struct and `EgressComponent` interface.

### Implement `EgressSpec()` on high-value egress `Arguments`

At minimum (covers attacker-relevant exfil paths from the POC):

| Component | Arguments file | What to return |
|-----------|----------------|----------------|
| `remote.http` | `internal/component/remote/http/http.go` | `{Endpoints: []string{a.URL}}` |
| `loki.write` | `internal/component/loki/write/write.go` | endpoint URLs from `a.Endpoints` |
| `prometheus.remote_write` | `internal/component/prometheus/remotewrite/remotewrite.go` | `a.Endpoints[*].URL` |
| `pyroscope.write` | `internal/component/pyroscope/write/write.go` | `a.Endpoints[*].URL` |
| `otelcol.exporter.otlphttp` | otelcol exporter package | endpoint URL |
| `otelcol.exporter.otlp` | otelcol exporter package | endpoint URL |
| `remote.vault` | `internal/component/remote/vault/` | server URL |
| `remote.s3` | `internal/component/remote/s3/` | `{HasDynamic: true}` (endpoint is derived from S3 path + region) |
| `discovery.*` | various | `{HasDynamic: true}` (all discovery) |
| `prometheus.scrape` | scrape package | `{HasDynamic: true}` (targets from discovery) |

### `internal/runtime/internal/controller/node_builtin_component.go`

In `evaluate()`, after building `args`, add:

```go
if globals.SecurityPolicy != nil {
    if ec, ok := args.(component.EgressComponent); ok {
        spec := ec.EgressSpec()
        for _, url := range spec.Endpoints {
            if err := globals.SecurityPolicy.CheckEndpoint(url); err != nil {
                return err
            }
        }
    }
}
```

### `internal/nodeconf/importsource/import_git.go`

Check repo URL directly before opening connection (config block, not a component — not covered by `EgressComponent`):

```go
if globals.SecurityPolicy != nil {
    if err := globals.SecurityPolicy.CheckEndpoint(opts.Repository); err != nil {
        return err
    }
}
```

## Notes on dynamic URLs

- Components with `HasDynamic: true` cannot be fully validated at config parse time.
- The `alloy security-policy check` subcommand (Phase 4) emits a warning for these: "component X has dynamic endpoints — manual review required."
- The component allowlist (Phase 1) is the backstop: deny the component entirely if dynamic endpoint policy is unacceptable.
