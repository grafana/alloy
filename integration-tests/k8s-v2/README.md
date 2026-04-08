# k8s-v2 integration tests (POC)

This proof-of-concept runs requirement-driven Kubernetes integration tests with a shared KinD lifecycle.

## What it demonstrates

- Test discovery from `tests/*/requirements.yaml`
- Per-run dependency union planning (`mimir`, `loki`)
- Single-test and multi-test execution from one runner
- Eventual assertions against Mimir and Loki APIs

## Test layout

Each test directory contains:

- `requirements.yaml` for dependency declarations
- `config.alloy` as test-specific Alloy config
- `workload.yaml` for test workload manifests
- `assert_test.go` for backend assertions

Shared dependency manifests are stored in:

- `internal/deps/manifests/mimir.yaml`
- `internal/deps/manifests/loki.yaml`

They are loaded by the dependency installers via `go:embed`.

## Commands

List available tests and aliases:

```sh
make integration-test-k8s-v2-list
```

Run all k8s-v2 tests:

```sh
make integration-test-k8s-v2-all
```

Run one test:

```sh
make integration-test-k8s-v2-metrics
make integration-test-k8s-v2-logs
```

Run selected tests (exact names or aliases like `metrics`, `logs`):

```sh
make integration-test-k8s-v2 TESTS=metrics-mimir,logs-loki
```

Keep the KinD cluster for debugging:

```sh
make integration-test-k8s-v2 TESTS=metrics KEEP_CLUSTER=1
```

Pass extra `go test` flags:

```sh
make integration-test-k8s-v2 TESTS=metrics EXTRA_GO_TEST_ARGS="-count=1"
```

Use the Go wrapper directly:

```sh
go run ./integration-tests/k8s-v2/cmd/k8s-v2-run --list
go run ./integration-tests/k8s-v2/cmd/k8s-v2-run --tests metrics,logs
go run ./integration-tests/k8s-v2/cmd/k8s-v2-run --tests metrics -- --count=1
```

## Notes

- The runner creates a KinD cluster through e2e-framework and installs only selected dependencies.
- On child test failure, output includes a deterministic repro command.
- The wrapper auto-discovers tests from `tests/*/requirements.yaml` so new tests do not require wrapper code changes.
