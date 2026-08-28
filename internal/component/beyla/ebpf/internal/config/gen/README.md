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
 beyla/schema.json         Beyla's published config schema (docs/config-schema.json),
                           pinned to that version. Downloaded by `make download-beyla-schema`;
                           the schema-validation test checks the emitted YAML against it.
```

## Safety net

`golden_test.go` locks the full emitted YAML for a maximally-populated config
(byte-identical; regenerate with `UPDATE_GOLDEN=1`), guarding mechanical refactors
of the translation.

`schema_validation_test.go` emits the same config and asserts every key exists in
Beyla's published `schema.json`, validated with `gojsonschema` (with
`additionalProperties` forced to false) — so a typo'd or misplaced key, which Beyla
silently ignores, fails. A small `allowlist` covers real Beyla keys the schema doesn't
export.

## Upgrading Beyla

See [docs/developer/updating-beyla.md](../../../../../../../docs/developer/updating-beyla.md).
