# Updating Beyla

## Overview

`beyla.ebpf` embeds a downloaded [Beyla](https://github.com/grafana/beyla) binary and
runs it as a subprocess. The version is pinned by `BEYLA_VERSION` in the `Makefile`.
Alloy translates the component config to Beyla's YAML with hand-written Go in
`internal/component/beyla/ebpf/internal/config`
([README](../../internal/component/beyla/ebpf/internal/config/gen/README.md)); the
config surface is intentionally not a 1:1 mirror of Beyla's YAML.

## Bumping the version

1. Edit `BEYLA_VERSION` in the `Makefile`.

2. Record the new release's tarball checksums, and review the diff:

   ```
   make update-beyla-checksums
   ```

   This writes `internal/component/beyla/ebpf/internal/config/gen/beyla-checksums.txt`
   (committed, sha256sum format). `make download-beyla` verifies downloads against it,
   so a compromised upstream can't swap in a binary we didn't review — the committed
   checksum is the trust anchor, like `go.sum`.

3. Download the binaries and schema, and sync the docs version:

   ```
   make beyla
   ```

   This runs `download-beyla`, `download-beyla-schema`, and
   `sync-beyla-docs-version` — the last updates `BEYLA_VERSION` in both
   `docs/sources/_index.md.t` and `docs/sources/_index.md` so the rendered docs
   match.

## Updating the config translation

Reconcile the hand-written types and translation with any new or changed Beyla
options. Two tests guard this:

- **`schema_validation_test.go`** emits a fully-populated config and asserts every
  emitted key exists in `schema.json`. A typo'd or misplaced key — which Beyla would
  silently ignore — fails the test.

- **`coverage_test.go`** (`TestSchemaCoverage`) snapshots the Beyla options Alloy does
  *not* expose into `testdata/unexposed_schema.txt`. A bump that adds new upstream
  options fails the test until you either expose them (add the `Arguments` field +
  `Convert()`) or accept them with `UPDATE_COVERAGE=1`.
