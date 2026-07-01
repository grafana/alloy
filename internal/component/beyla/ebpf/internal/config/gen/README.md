# Beyla config generation

The bulk of the Alloy config types and the Args→Beyla-YAML translation are
**generated** from Beyla's own config schema. The schema is the source of truth.
Two inputs are hand-maintained: `mappings.json` (how the schema maps to Alloy) and
`args.go` (the `Arguments` struct + the types whose shape differs from Beyla's).

## Pipeline

```
  docs/config-schema.json (Beyla repo @ $BEYLA_VERSION)
            │  make download-beyla-schema
            ▼
     gen/schema.json
            │        hand-maintained inputs:
            │          gen/mappings.json
            │          args.go  (Arguments + Alloy-specific types) ──┐
            ▼                                                        │ parsed to learn
        gen/main.go  ◄───────────────────────────────────────────────┘ existing Go types
            │        (go generate)                                     + Arguments fields
            │
            ├─► args_gen.go            (Alloy structs)
            ├─► config_gen.go          (Args→YAML)
            ├─► validation_gen.go      (enum value-sets from schema)
            ├─► gen/mappings_examples.md (rendered before/after)
            └─► gen/sample_config.alloy  (full option skeleton + descriptions)

  GATE: every schema top-level must be exposed (a field in Arguments) or
        classified in mappings.json (manual / multi / excluded),
        else the generation fails.
```

Runtime: `args.go`+`args_gen.go` → `Arguments` (decoded from Alloy syntax);
`config.go`+`config_gen.go` → `config.Build(args, rt)` → Beyla YAML.

## Where a type lives

```
 a type ...                            lives in
 ──────────────────────────────────────────────────────────
 maps 1:1 to a schema $def         →   args_gen.go (generated)
 Alloy-specific shape / no $def    →   args.go     (hand-written)
```

`args.go` keeps the types whose shape differs from Beyla's (e.g. `metrics` is
split into `prometheus_export`+`network`; `Filters`, `traces`, `attributes`).

## Validation

Split across two files:

- `validation_gen.go` (generated) — the accepted-value sets (sampler names,
  metric features, instrumentations, `trace_printer`), each with a `valid<Name>()`
  helper. Read from the schema's `enum`s via `mappings.json` `enum_validators`, so
  a Beyla bump updates them automatically — no drift.
- `validation.go` (hand-written) — the rules the schema can't express: cross-field
  (`traces` needs `output`; app features need a discovery block), numeric ranges
  (sampler ratio ∈ [0,1]), "at least one of" (a service needs `open_ports` /
  `exe_path` / `kubernetes`). It calls the generated helpers for value checks.

## mappings.json reference

Rendered before/after for the starred (★) keys is in
[`mappings_examples.md`](./mappings_examples.md) (auto-generated).

| key | what it does | example |
|---|---|---|
| `name_overrides`   | schema `$def` name → Go type name                       | `BeylaDiscoveryConfig` → `Discovery` |
| `field_aliases`    | camelCase schema key → snake_case Alloy attr            | `podLabels` → `pod_labels` |
| `pointer_fields`   | make field `*T` so an explicit zero / default-true survives | `Javaagent.enabled` → `*bool` |
| `skip`             | leave a schema field out of generation (handled by hand)| `injector.enabled_sdks` |
| `enum_validators`  | schema `enum` → generated `valid<Name>()` + value list (used by `validation.go`) | `SamplerConfig.name` → valid sampler names |
| `exclude_toplevel` | top-level intentionally **not** exposed (Alloy-managed/injected/internal) | `prometheus_export`, `health_check`, `grafana`, … |
| `aliases` ★        | rename a schema key to an Alloy key (optional deprecation warn) | `filter` → `filters` |
| `multi_section` ★  | one Alloy block → several YAML sections (fill helpers)  | `metrics` → `prometheus_export` + `network` |
| `flatten_transforms` ★ | prefixed schema fields → one nested Alloy block     | `k8s_*` → `kubernetes { … }` |
| `map_keyed_by` ★   | YAML map keyed by a field ↔ repeated Alloy block        | `Selections` keyed by `attr` |
| `inject_wrappers` ★| insert an intermediate YAML key around fields           | `http.{openai,…}` → `http.genai.{openai,…}` |
| `manual_sections` ★| top-level absent from schema; emitted by hand-written code | `traces`, `attributes`, `internal_metrics` |

## The coverage gate

Every schema top-level property must be classified, or `go generate` fails:

```
exposed        Arguments has a matching block field (honoring a top-level alias)
multi_section  listed in multi_section
manual         listed in manual_sections
excluded       listed in exclude_toplevel
```

Otherwise:

```
top-level coverage: unclassified schema top-level(s) [foo]: expose each
(add an Arguments field) or add it to exclude_toplevel in mappings.json
```

This is what makes coverage **expose-by-default**: a new Beyla top-level can't be
silently dropped (the bug that once hid `injector`, `nodejs`, `javaagent`).

## Common tasks

```
Bump Beyla:        edit BEYLA_VERSION in Makefile → make beyla
                   (downloads schema, regenerates; gate fails if a new
                    top-level needs exposing or excluding)

Expose a section:  add a block field to Arguments in args.go → go generate
                   (auto-wired into Build via the addGeneratedConfig dispatcher)

Regenerate:        cd internal/component/beyla/ebpf/internal/config && go generate .
```
