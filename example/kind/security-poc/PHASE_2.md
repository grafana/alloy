# Phase 2: Endpoint Gate

Gates which outbound URLs components may connect to, using glob pattern matching at runtime.

**Policy field added this phase:** `endpoints` section.

---

## Why not a central dialer

~80–100 egress touch points across 7–8 distinct HTTP client patterns (Prometheus common config, OTel factories, client-go, AWS SDK, SQL drivers, etc.). No single hook exists. A central dialer is a future goal. This phase gates at the component argument level instead.

---

## Interface design

```go
// internal/component/registry.go

type EgressSpec struct {
    // Endpoints lists the literal outbound URLs resolved from the current arguments.
    // Empty when the component has no static outbound connections.
    Endpoints []string
    // HasDynamic is true when the component connects to endpoints not listed in
    // Endpoints — e.g. prometheus.scrape whose targets come from discovery, or
    // loki.source.kubernetes whose k8s API URL is implicit from the service account.
    HasDynamic bool
}

// EgressComponent is implemented by component Arguments structs that can
// declare their outbound endpoints. Called inside evaluate() after full
// expression resolution, so URLs are always real strings (never AST nodes).
type EgressComponent interface {
    EgressSpec() EgressSpec
}

// PolicyChecker is the interface the controller uses to validate endpoints.
// Defined here so the controller package can use it without importing securitypolicy.
type PolicyChecker interface {
    CheckEndpoint(url string) error
}
```

### Expression-built URLs (e.g. `env("LOKI_URL")`)

`EgressSpec()` is called at `BuiltinComponentNode.evaluate()` time — after the VM has fully resolved all expressions. So `env("LOKI_URL")`, `local.file.x.content`, or any other expression-built URL **is already a concrete string** when `EgressSpec()` is called. Runtime enforcement works correctly for all these cases.

### Static analysis (`alloy security-policy check`, Phase 4)

At parse time, expression arguments cannot be resolved (no environment, no running components). The CLI check does a best-effort parse: literal string URLs are extracted and checked; non-resolvable expressions yield a warning "endpoint cannot be validated statically — enforced at runtime." The runtime is the real enforcement point.

---

## `HasDynamic` semantics

| Scenario | `HasDynamic` | At runtime | In CLI check |
|----------|-------------|------------|-------------|
| `prometheus.scrape` targets from discovery | `true` | Can't check — targets unknown | Warn: "dynamic endpoints" |
| `loki.source.kubernetes` (k8s API implicit) | `true` | Can't check | Warn |
| `loki.write` URL from `env()` | `false` | ✅ string resolved, checked | Warn: "endpoint unresolvable statically" |
| `remote.http` literal URL | `false` | ✅ checked | ✅ checked |

`HasDynamic: true` **always emits a warning** (logged at evaluate time, reported in CLI check). It does not by itself cause a block — the component gate (Phase 1) is the backstop for components whose dynamic connectivity is unacceptable.

If the policy is in **allowlist mode** and a component has `HasDynamic: true`, the warning message makes this explicit: "component X has dynamic endpoints that cannot be validated against the endpoint allowlist."

---

## Policy field

```go
type EndpointsSection struct {
    Mode     string   `yaml:"mode"`     // "allowlist" or "denylist"
    Patterns []string `yaml:"patterns"` // URL globs, e.g. "https://grafana.com/**"
}

type SecurityPolicy struct {
    // ... previous phases ...
    Endpoints EndpointsSection `yaml:"endpoints"`
}
```

### URL normalization (applied before matching)

To prevent trivial bypasses, URLs are normalized before glob matching:
- Scheme and host lowercased
- Default ports stripped (`http` → 80, `https` → 443)
- Path normalized (clean double slashes, ensure leading `/`)

Example: `HTTPS://Grafana.com:443/loki/api/v1/push` normalizes to `https://grafana.com/loki/api/v1/push`.

### Glob semantics

Uses `github.com/bmatcuk/doublestar/v4` (already in go.mod):
- `*` matches within a single path segment (does not cross `/`)
- `**` matches across path separators

Examples:
- `https://grafana.com/**` — matches any path under that host
- `https://*.grafana.com/**` — matches any subdomain
- `https://grafana.com/loki/*` — matches one level under `/loki/`

---

## Files to change

### `internal/securitypolicy/policy.go`

Add `EndpointsSection` and `CheckEndpoint(url string) error` with URL normalization.

### `internal/component/registry.go`

Add `EgressSpec` struct, `EgressComponent` interface, and `PolicyChecker` interface.

### Implement `EgressSpec()` on high-value egress components

At minimum (covers attacker-relevant exfil paths from the POC):

| Component | Arguments file | What to return |
|-----------|----------------|----------------|
| `remote.http` | `internal/component/remote/http/http.go` | `{Endpoints: []string{a.URL}}` |
| `loki.write` | `internal/component/loki/write/write.go` | endpoint URLs from `a.Endpoints` |
| `prometheus.remote_write` | `internal/component/prometheus/remotewrite/` | `a.Endpoints[*].URL` |
| `discovery.*` | various | `{HasDynamic: true}` |
| `prometheus.scrape` | scrape package | `{HasDynamic: true}` |
| `loki.source.kubernetes` | loki/source/kubernetes | `{HasDynamic: true}` |

### `internal/runtime/internal/controller/node_builtin_component.go`

Add `PolicyChecker component.PolicyChecker` to `ComponentGlobals`.

In `evaluate()`, after `argsCopyValue` is built:

```go
if cn.globals.PolicyChecker != nil {
    if ec, ok := argsCopyValue.(component.EgressComponent); ok {
        spec := ec.EgressSpec()
        for _, u := range spec.Endpoints {
            if err := cn.globals.PolicyChecker.CheckEndpoint(u); err != nil {
                return fmt.Errorf("endpoint policy violation: %w", err)
            }
        }
        if spec.HasDynamic {
            cn.globals.Logger.Slog().Warn("component has dynamic endpoints that cannot be validated against endpoint policy",
                "component", cn.componentName)
        }
    }
}
```

### `internal/runtime/alloy.go`

Extend `SecurityPolicyChecker` to include `CheckEndpoint`, pass it as `ComponentGlobals.PolicyChecker`.

---

## Notes

- `EgressSpec()` is on the **Arguments struct**, not the component — so it works even before the component is built (first evaluate).
- Components with `HasDynamic: true` AND `Endpoints` (e.g. a component with one static + one discovery-driven target) have their static endpoints checked normally and emit a dynamic warning.
- URL normalization happens inside `CheckEndpoint`, not in `EgressSpec()` — component code stays clean.
