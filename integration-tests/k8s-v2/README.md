# k8s-v2 integration tests

Requirement-driven Kubernetes integration tests with a shared KinD lifecycle.

## What it demonstrates

- Test discovery from `tests/*/requirements.yaml`
- Per-run dependency union planning (`mimir`, `loki`)
- Single-test and multi-test execution from a single Make target
- Eventual assertions against Mimir and Loki APIs

## Architecture

Three cooperating processes:

1. **Harness** (`go test` on the `k8s-v2` package). Plans selected tests and
   dependencies, creates (or reuses) a KinD cluster, installs only the
   dependencies the selection requires, then runs each selected test as a
   parallel `TestIntegrationV2/<name>` subtest.
2. **Per-test assertion binary** (`tests/<name>/assert_test.go`). Each test
   ships its assertions in its own Go package; the harness invokes them as
   another `go test` subprocess so flags stay scoped and a failing test
   prints an exact repro command.
3. **`make integration-test-k8s-v2`**. Thin Makefile target that invokes
   `go test` with the right build tags and translates env vars (`TEST`,
   `KEEP`, `REUSE`, ...) into harness flags.

Shared dependency metadata (namespace, service, readiness path, manifest
file) lives once in `internal/deps` as `deps.Spec`, and is consumed by both
the installer and the port-forward helper in `internal/assert`. Adding a
new backend is one `Spec` value plus one embedded manifest.

## Test layout

Each test directory contains:

- `requirements.yaml` — declares which backend(s) the test needs (`loki`,
  `mimir`, ...). Only declared backends are installed for runs that select
  the test.
- `helm-alloy-values.yaml` — Alloy Helm values; installed per-test into its
  own namespace.
- `workload.yaml` — test-specific resources (generators, services, ...).
- `assert_test.go` — Go assertions against the live backend.

Assets support `${TEST_ID}` and `${TEST_NAMESPACE}` placeholders for
per-test isolation. `${TEST_NAMESPACE}` is used as both kubernetes namespace
and Helm release name.

## Commands

Run everything:

```sh
make integration-test-k8s-v2
```

Run a subset (comma-separated):

```sh
make integration-test-k8s-v2 TEST=logs-loki
make integration-test-k8s-v2 TEST=logs-loki,metrics-mimir
```

Keep the cluster and installed dependencies for debugging (implies both):

```sh
make integration-test-k8s-v2 TEST=logs-loki KEEP=1
```

Iterate against the same cluster. After `KEEP=1`, the harness prints the
cluster name; pass it back:

```sh
make integration-test-k8s-v2 TEST=logs-loki REUSE=alloy-it-abc12345 REUSE_DEPS=1
```

Run against a locally built Alloy image (digest pins are rejected; use
`repo:tag`):

```sh
make ALLOY_IMAGE=alloy-ci:dev alloy-image
make integration-test-k8s-v2 \
  ALLOY_IMAGE=alloy-ci:dev \
  ALLOY_IMAGE_PULL_POLICY=IfNotPresent
```

Pass extra tokens straight to `go test -args`:

```sh
make integration-test-k8s-v2 K8S_V2_ARGS="-k8s.v2.parallel=1 -k8s.v2.debug=true"
```

### Running without the Makefile

```sh
go test -v -tags "alloyintegrationtests k8sv2integrationtests" \
  -timeout 30m ./integration-tests/k8s-v2 \
  -args -k8s.v2.tests=logs-loki
```

## Notes

- k8s-v2 tests are tag-gated; generic `go test ./...` does not run them.
- Dependencies share a fixed namespace each (Loki=`loki`, Mimir=`mimir`) and
  run once per invocation. Tests share them via the assert helpers.
- Runtime isolation uses a per-test `test_id` label and per-test
  namespace/release derived from the test name with a random suffix.
- Selected tests run in parallel by default (`-k8s.v2.parallel`, default 4).
- Child test failures print a deterministic `go test ...` repro command.
