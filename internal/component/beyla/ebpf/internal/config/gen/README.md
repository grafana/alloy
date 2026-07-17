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
 schema.json           Beyla's published config schema (docs/config-schema.json),
                       pinned to BEYLA_VERSION. Downloaded by `make download-beyla-schema`.
                       NOT used for code generation — it's the reference the
                       schema-validation test checks the emitted YAML against.
 beyla-checksums.txt   committed sha256 of each release tarball, pinned to
                       BEYLA_VERSION. `make download-beyla` verifies against it, so a
                       compromised upstream can't swap in a binary we didn't review.
 download.go           downloads + embeds the Beyla binary (`make download-beyla`);
                       `--update-checksums` records beyla-checksums.txt.
```

## Safety net

`schema_validation_test.go` fully populates an `Arguments`, emits the YAML, and
asserts every emitted key exists in `schema.json` (a strict walk — an unknown key
is a failure, since the schema doesn't set `additionalProperties`). This catches a
typo'd or misplaced key, which Beyla would otherwise silently ignore.

A small `allowlist` in that test covers real Beyla keys the schema doesn't export.

## Upgrading Beyla

See [docs/developer/updating-beyla.md](../../../../../../../docs/developer/updating-beyla.md).
