# beyla-config-validator

Strict-unmarshals a Beyla YAML config (as produced by Alloy's `beyla.ebpf`
Args→YAML translation) into the **upstream `beyla.Config` struct**, failing on any
key or value Beyla wouldn't accept. It's a stronger check than the in-repo
schema-validation test: it validates against Beyla's actual Go types (including the
custom enum unmarshalers), not just the published JSON schema.

Run it:

```
make validate-beyla-config
```

## Why a separate module

Importing `beyla.Config` transitively pulls in Beyla + OBI's eBPF dependency tree
(~100 modules). Keeping this in its own `go.mod` means that tree never reaches the
Alloy binary — the subprocess model's zero-Beyla-dependency property is preserved.

## The OBI replace (must stay in sync)

Beyla vendors OBI as a local submodule (`replace go.opentelemetry.io/obi => ./.obi-src`),
which ships the committed eBPF bindings. That local replace can't propagate to a
module that just `require`s Beyla, so this module replicates it with the fork's tag:

```
replace go.opentelemetry.io/obi => github.com/grafana/opentelemetry-ebpf-instrumentation v1.328.0
```

On a Beyla bump this must be updated to the fork tag matching Beyla's new `.obi-src`
commit (the same OBI-version resolution `agent_bump_beyla.yml` does for the collector).
If it drifts, the module stops building.

## The sample config

`testdata/beyla-config.yaml` is a **generated snapshot** of the translation output
for a representative valid `Arguments`. It is not auto-updated — regenerate it when
the translation changes (build a valid `Arguments`, `yaml.Marshal(config.Build(...))`,
write it here). Because it's a snapshot, it does not catch live translation
regressions the way an in-package test would; wiring it to live generation would
require the validator to reach `config.Build`, which is an internal package.

## Known costs

- A second module with the full Beyla/OBI dependency tree (`go.sum`, supply-chain
  surface, dependabot).
- The OBI-replace sync above, on every Beyla bump.
- A separate CI lane (not covered by the repo's `go test ./...`).
- The reflective maximal `Arguments` used by the schema test can't be reused here —
  its dummy values fail Beyla's enum validators, so a hand-maintained valid config
  is required.
