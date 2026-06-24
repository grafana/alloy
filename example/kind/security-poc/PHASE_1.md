# Phase 1: Component Gate

Gates which Alloy components may be instantiated at all.

**Policy field added this phase:** `components` section.

## What it blocks

Any component whose name is not on the allowlist (or is on the denylist). Examples an operator might deny: `remote.http`, `loki.write`, `otelcol.exporter.otlphttp`.

## Files to change

### New: `internal/securitypolicy/policy.go`

Shared foundation for all phases. Phase 1 only needs:

```go
package securitypolicy

type PolicySection struct {
    Mode string   // "allowlist" or "denylist"
    List []string
}

type SecurityPolicy struct {
    Components PolicySection `yaml:"components"`
}

func (p *SecurityPolicy) CheckComponent(name string) error { ... }
func LoadFromFile(path string) (*SecurityPolicy, error) { ... }
```

`CheckComponent` returns nil if allowed, error if denied. Logic:
- `mode: allowlist`: allow if name is in List, deny otherwise.
- `mode: denylist`: deny if name is in List, allow otherwise.
- Section absent (zero value): allow all.

### `internal/alloycli/cmd_run.go`

Add `--security-policy` flag and load the file once before runtime init:

```go
// on alloyRun struct
SecurityPolicyPath string

// in RunCommand():
cmd.Flags().StringVar(&fr.SecurityPolicyPath, "security-policy", "", "Path to security policy file")

// in Run(), before alloy_runtime.New():
var policy *securitypolicy.SecurityPolicy
if fr.SecurityPolicyPath != "" {
    policy, err = securitypolicy.LoadFromFile(fr.SecurityPolicyPath)
    if err != nil { return err }
}
```

### `internal/runtime/alloy.go`

Add `SecurityPolicy` to `Options` and thread into `ComponentGlobals`:

```go
type Options struct {
    // ... existing fields ...
    SecurityPolicy *securitypolicy.SecurityPolicy
}
```

### `internal/component/registry.go`

Extend `Registry.Get()` to enforce the policy. Pattern follows `featuregate.CheckAllowed`:

```go
func (r *Registry) Get(name string, opts GetOptions) (Registration, error) {
    // ... existing stability/community checks ...
    if opts.SecurityPolicy != nil {
        if err := opts.SecurityPolicy.CheckComponent(name); err != nil {
            return Registration{}, err
        }
    }
    // ... rest of existing logic ...
}
```

`GetOptions` already exists and carries stability/community flags — add `SecurityPolicy` there.

## Testing

- Unit test `PolicySection.Check` with allowlist and denylist modes.
- Integration test: load a policy denying `remote.http`; verify `registry.Get("remote.http", ...)` returns an error.
- `alloy validate` should also respect the gate (it uses the same registry).
