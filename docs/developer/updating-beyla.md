# Updating Beyla

## Overview

`beyla.ebpf` embeds a downloaded [Beyla](https://github.com/grafana/beyla) binary and
runs it as a subprocess. The version and release-tarball checksums are pinned in
`internal/component/beyla/ebpf/internal/config/gen/beyla/beyla_version.yaml` (read by
the `Makefile`). Alloy translates the component config to Beyla's YAML with hand-written Go in
`internal/component/beyla/ebpf/internal/config`
([README](../../internal/component/beyla/ebpf/internal/config/gen/README.md)); the
config surface is intentionally not a 1:1 mirror of Beyla's YAML.

## Bumping the version

```
make update-beyla TAG=<beyla-version>    # e.g. TAG=v3.29.0
```

This one command:

- records the version and each release tarball's sha256 in
  `internal/component/beyla/ebpf/internal/config/gen/beyla/beyla_version.yaml` (the
  pinned manifest),
- downloads and verifies the binaries against those checksums,
- downloads the matching config schema,
- syncs the version into `docs/sources/_index.md.t` and `docs/sources/_index.md`.

Review the `beyla_version.yaml` diff before committing — the committed checksum is the
trust anchor, like `go.sum`.

## Updating the config translation

Reconcile the hand-written types and translation with any new or changed Beyla
options. Two checks guard this:

- **`golden_test.go`** locks the full emitted YAML byte-for-byte (regenerate with
  `UPDATE_GOLDEN=1` when a translation change is intended).

- **`schema_validation_test.go`** emits the same config and asserts every key exists
  in Beyla's published `schema.json` (validated with `gojsonschema`, with
  `additionalProperties` forced to false). A typo'd or misplaced key — which Beyla
  silently ignores — fails.
