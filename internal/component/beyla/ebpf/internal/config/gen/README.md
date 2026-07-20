# Beyla config

The Alloy config types and the Args→Beyla-YAML translation are **hand-written Go**
in the parent `config` package:

```
 args.go / args_types.go        the Arguments struct + all Alloy config types
 config.go / config_build.go    Build(args, rt) → the Beyla YAML config map
 validation.go / validation_enums.go   parse-time validation (rules + value sets)
```

`Build` decodes are one-way: Alloy `Arguments` in, a `map[string]any` (marshaled to
Beyla's YAML config file) out. The Alloy config surface is intentionally *not* a 1:1
mirror of Beyla's YAML — e.g. `metrics` splits into `prometheus_export`+`network`,
`filters` emits Beyla's `filter`, GenAI providers sit directly under `http`.

## `gen/`

```
 download.go               downloads + verifies the Beyla binary (`make download-beyla`);
                           `--update-checksums` regenerates beyla/beyla_version.yaml.
 beyla/beyla_version.yaml  pinned Beyla version + each release tarball's sha256.
                           `make download-beyla` verifies downloads against it, so a
                           compromised upstream can't swap in a binary we didn't review.
```

## Safety net

`golden_test.go` locks the full emitted YAML for a maximally-populated config
(byte-identical; regenerate with `UPDATE_GOLDEN=1`), guarding mechanical refactors
of the translation.

`tools/beyla-config-validator` strict-unmarshals a generated config into the
upstream `beyla.Config` struct (`make validate-beyla-config`), catching keys or
values Beyla wouldn't accept.

## Upgrading Beyla

See [docs/developer/updating-beyla.md](../../../../../../../docs/developer/updating-beyla.md).
