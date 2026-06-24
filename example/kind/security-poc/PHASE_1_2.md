# Phase 1.2: Stdlib Function Gate

Gates which expression-language functions are available in config.

**Policy field added this phase:** `stdlib_funcs` section.

## What it blocks

Functions from `syntax/internal/stdlib/stdlib.go`. The security-relevant ones:

| Function | What it does |
|----------|-------------|
| `sys.env` | Reads any env var from the process environment |
| `env` (deprecated) | Same — legacy alias |
| `convert.nonsensitive` | Strips the `Secret` type, making a secret readable as plain string |

## Files to change

### `internal/securitypolicy/policy.go`

Add `StdlibFuncs` to `SecurityPolicy`:

```go
type SecurityPolicy struct {
    Components   PolicySection `yaml:"components"`
    ConfigBlocks PolicySection `yaml:"config_blocks"`
    StdlibFuncs  PolicySection `yaml:"stdlib_funcs"`
}

func (p *SecurityPolicy) FilterStdlib(identifiers map[string]any) map[string]any { ... }
```

`FilterStdlib` returns a copy of the map with denied identifiers removed (or only allowed identifiers kept). The `sys` namespace is a nested map — for `sys.env`, remove the `"env"` key from the nested map, and remove `"sys"` from the root if it becomes empty.

### Injection point in `syntax/vm/vm.go` (needs tracing)

The stdlib `Identifiers` map needs to be passed through policy filtering before it reaches the VM's root scope. The exact injection point requires tracing how `stdlib.Identifiers` flows into the expression evaluator — this is the main investigation task for this phase.

At a minimum: wherever the runtime constructs its root evaluation scope from `stdlib.Identifiers`, replace that with `policy.FilterStdlib(stdlib.Identifiers)` when a policy is present.

## Notes

- Do not mutate the global `stdlib.Identifiers` map — build a filtered copy.
- Both `sys.env` (namespaced) and `env` (deprecated top-level) need to be covered.
- The policy section name `stdlib_funcs` uses the dot-path notation for nested functions: `sys.env` means the `env` key inside the `sys` namespace.
