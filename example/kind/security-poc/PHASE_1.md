# Phase 1: Component, Config Block, and Stdlib Function Gates

Gates which components may instantiate, which config block types are allowed, and which expression functions are available.

**Policy fields added this phase:** `components`, `config_blocks`, `stdlib_funcs`.

---

## 1. Component gate

### What it blocks

Any component not on the allowlist (or on the denylist). Examples an operator might deny: `remote.http`, `loki.write`, `otelcol.exporter.otlphttp`.

### Files to change

**New: `internal/securitypolicy/policy.go`** — shared foundation for all phases. Phase 1 only needs:

```go
package securitypolicy

type PolicySection struct {
    Mode string   // "allowlist" or "denylist"
    List []string
}

type SecurityPolicy struct {
    Components   PolicySection `yaml:"components"`
    ConfigBlocks PolicySection `yaml:"config_blocks"`
    StdlibFuncs  PolicySection `yaml:"stdlib_funcs"`
}

func (p *SecurityPolicy) CheckComponent(name string) error   { ... }
func (p *SecurityPolicy) CheckConfigBlock(name string) error { ... }
func (p *SecurityPolicy) FilterStdlib(ids map[string]any) map[string]any { ... }
func LoadFromFile(path string) (*SecurityPolicy, error)      { ... }
```

`Check*` returns nil if allowed, error if denied. Logic: allowlist = allow if in list, deny otherwise; denylist = deny if in list, allow otherwise; section absent = allow all.

**`internal/alloycli/cmd_run.go`** — add `--security-policy` flag, load once before runtime init:

```go
cmd.Flags().StringVar(&fr.SecurityPolicyPath, "security-policy", "", "Path to security policy file")
// in Run():
policy, err = securitypolicy.LoadFromFile(fr.SecurityPolicyPath)
```

**`internal/runtime/alloy.go`** — add `SecurityPolicy *securitypolicy.SecurityPolicy` to `Options`; thread into `controller.ComponentGlobals`.

**`internal/component/registry.go`** — extend `Registry.Get()` alongside existing stability/community checks:

```go
if opts.SecurityPolicy != nil {
    if err := opts.SecurityPolicy.CheckComponent(name); err != nil {
        return Registration{}, err
    }
}
```

---

## 2. Config block gate

### What it blocks

Config block types: `import.http`, `import.git`, `import.string`, `import.file`, `logging`, `tracing`, `foreach`, etc. Blocking `import.*` prevents all dynamic config loading.

### Files to change

**`internal/runtime/internal/controller/node_config.go`** — extend `NewConfigNode()`:

```go
if globals.SecurityPolicy != nil {
    if err := globals.SecurityPolicy.CheckConfigBlock(block.GetBlockName()); err != nil {
        diags.Add(diag.SeverityLevelError, block.NamePos.Position(), err.Error())
        return nil, diags
    }
}
```

`ComponentGlobals` is already threaded in from `runtime.Options` — no new wiring needed.

Note: valid block names are the constants in `internal/nodeconf/importsource/import.go` (`BlockNameFile = "import.file"`, etc.).

---

## 3. Stdlib function gate

### What it blocks

| Function | What it does |
|----------|-------------|
| `sys.env` | Reads any env var from the process environment |
| `env` (deprecated) | Same — legacy alias |
| `convert.nonsensitive` | Strips the `Secret` type, making a secret readable as plain string |

### Files to change

**`internal/securitypolicy/policy.go`** — `FilterStdlib` returns a copy of the identifiers map with denied functions removed. For `sys.env`, remove the `"env"` key from the nested `sys` map and drop `"sys"` from root if it becomes empty. Do not mutate the global `stdlib.Identifiers` map.

**Injection point in `syntax/vm/vm.go`** (exact location needs tracing): wherever the runtime constructs its root evaluation scope from `stdlib.Identifiers`, replace with `policy.FilterStdlib(stdlib.Identifiers)` when a policy is present.

Note: `stdlib_funcs` list uses dot-path notation — `sys.env` means the `env` key inside the `sys` namespace.

---

## Testing

- Unit test `PolicySection` with allowlist and denylist modes for each of the three `Check*` / `FilterStdlib` methods.
- Integration test: deny `remote.http` → `registry.Get` returns error.
- Integration test: deny `import.http` → config with that block rejected on load.
- Integration test: deny `sys.env` → expression `sys.env("SECRET")` fails to evaluate.
