# Phase 4: `alloy security-policy check` Subcommand

A dry-run subcommand to validate a config against a policy without running Alloy. Helps operators author and debug policies iteratively.

## Command

```sh
alloy security-policy check --security-policy=policy.yaml config.alloy
```

## Output

- **Violations** — components/blocks/functions/endpoints the config uses that the policy forbids. This config would be rejected at runtime.
- **Missing permissions** — what to add to the policy to make this config pass.
- **Tightening suggestions** — allowed entries the policy permits that the config never uses, so you can remove them and shrink the policy surface.
- **Dynamic endpoint warnings** — components with `HasDynamic: true` that cannot be statically verified.

Future: `--generate` flag to emit a maximally-tight policy from the config automatically.

## Files to change

### New: `internal/alloycli/cmd_security_policy.go`

Mirror structure of `internal/alloycli/cmd_validate.go`:
- Load config via `loadSourceFiles()`
- Parse with `validator.Validate()` (existing)
- Run each policy gate in report-only mode (collect violations instead of returning errors)
- Print structured output

Register in `internal/alloycli/alloycli.go`:

```go
cmd.AddCommand(securityPolicyCommand())
```

## Notes

- This subcommand is useful even before all runtime gates are implemented — it gives early feedback on policy authoring.
- The `check` command runs at parse time, so components with expression-derived URLs show as warnings, not violations.
- Can be integrated into CI to validate configs before deployment.
