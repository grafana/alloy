# Phase 1.1: Config Block Gate

Gates which config block types may appear in any loaded config.

**Policy field added this phase:** `config_blocks` section.

## What it blocks

Config block names: `import.http`, `import.git`, `import.string`, `import.file`, `logging`, `tracing`, `foreach`, etc. Blocking `import.*` blocks prevents dynamic config loading entirely.

## Files to change

### `internal/securitypolicy/policy.go`

Add `ConfigBlocks` to `SecurityPolicy`:

```go
type SecurityPolicy struct {
    Components  PolicySection `yaml:"components"`
    ConfigBlocks PolicySection `yaml:"config_blocks"`
}

func (p *SecurityPolicy) CheckConfigBlock(name string) error { ... }
```

Same allow/deny logic as `CheckComponent`.

### `internal/runtime/internal/controller/node_config.go`

Extend `NewConfigNode()` or `checkFeatureStability()` to call `policy.CheckConfigBlock`:

```go
func NewConfigNode(block *ast.BlockStmt, globals ComponentGlobals) (BlockNode, diag.Diagnostics) {
    // ... existing stability check ...
    if globals.SecurityPolicy != nil {
        if err := globals.SecurityPolicy.CheckConfigBlock(block.GetBlockName()); err != nil {
            var diags diag.Diagnostics
            diags.Add(diag.SeverityLevelError, block.NamePos.Position(), err.Error())
            return nil, diags
        }
    }
    // ... rest of existing logic ...
}
```

`ComponentGlobals` is threaded from `runtime.Options` (wired in Phase 1) — no new wiring needed.

## Notes

- Config block names to test against: use the constants in `internal/nodeconf/importsource/import.go` (`BlockNameFile = "import.file"`, etc.) as reference for valid values.
- The gate runs on every config load and reload, including imported modules and remotecfg updates.
