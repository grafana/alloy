# Security Policy: Design Overview

Defense against the attack in [SECURITY_POC.md](SECURITY_POC.md). Full design rationale in [MITIGATION.md](MITIGATION.md).

## What this builds

A `--security-policy=<path>` CLI flag that loads a policy file at startup (read-once, immutable). The policy gates what Alloy is allowed to run, without breaking backward compatibility: no flag = status quo.

## Policy file schema

Each section is independently allowlist or denylist mode. Omitted section = allow-all. Section present with empty list = deny-all for that section.

```yaml
components:
  mode: allowlist   # or: denylist
  list:
    - remote.http
    - loki.write

config_blocks:
  mode: denylist
  list:
    - import.http
    - import.git

stdlib_funcs:
  mode: denylist
  list:
    - sys.env
    - env
    - convert.nonsensitive

endpoints:
  mode: allowlist
  patterns:
    - "https://metrics.grafana.net/*"
    - "http://*.monitoring.svc:*"
```

## Architecture

```
--security-policy=policy.yaml
        │
        ▼
  SecurityPolicy (loaded once, immutable, threaded into runtime.Options)
        │
        ├── Phase 1:   Component + config block + stdlib gates
        ├── Phase 2:   Endpoint gate      → EgressComponent.EgressSpec() at evaluate()
        ├── Phase 3:   JWS signature gate → verify before any config is evaluated
        └── Phase 4:   check subcommand   → alloy security-policy check
```

## Phases

| Phase | File | What it gates | Shippable alone? |
|-------|------|---------------|-----------------|
| [1](PHASE_1.md) | components, config_blocks, stdlib_funcs | Component, config block, and stdlib function gates | Yes |
| [2](PHASE_2.md) | endpoints | Which outbound URLs components may connect to | Yes |
| [3](PHASE_3.md) | (new field) | Require JWS signature on all fetched config | Yes |
| [4](PHASE_4.md) | — | `alloy security-policy check` dry-run subcommand | Yes |

Each phase adds a self-contained gate. They can be implemented and shipped independently.

## Shared foundation (all phases depend on this)

- New package `internal/securitypolicy/`
- `--security-policy` flag in `internal/alloycli/cmd_run.go`
- `SecurityPolicy` field threaded through `alloy_runtime.Options` → `controller.ComponentGlobals`

Each phase document describes what to add to the shared `SecurityPolicy` struct for that phase only.
