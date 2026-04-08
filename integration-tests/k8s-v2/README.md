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

## Commands

Run all k8s-v2 tests:

```sh
go test -v ./integration-tests/k8s-v2
```

Run one test:

```sh
go test -v ./integration-tests/k8s-v2 -args -k8s.v2.tests=metrics-mimir
```

Run two tests in one shared lifecycle:

```sh
go test -v ./integration-tests/k8s-v2 -args -k8s.v2.tests=metrics-mimir,logs-loki
```

Keep the KinD cluster for debugging:

```sh
go test -v ./integration-tests/k8s-v2 -args -k8s.v2.tests=logs-loki -k8s.v2.keep-cluster=true
```

## Notes

- The runner creates a KinD cluster through e2e-framework and installs only selected dependencies.
- On child test failure, output includes a deterministic repro command.
