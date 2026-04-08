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

Run all k8s-v2 tests:

```sh
make integration-test-k8s-v2
# equivalent:
go run ./integration-tests/k8s-v2/runner --all
```

Run selected tests manually (exact folder names):

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --test logs-loki
```

`--all` and `--test` are mutually exclusive. Use one or the other.

Keep the KinD cluster for debugging:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --keep-cluster
```

Pass extra `go test` flags:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir -- --count=1
```

Pass test folder paths (relative to your current directory):

```sh
go run ./integration-tests/k8s-v2/runner --test integration-tests/k8s-v2/tests/logs-loki
```

Each `--test` path is validated:

- the folder must exist,
- it must map to a discovered k8s-v2 test folder.

Tune setup/readiness timeouts:

```sh
go run ./integration-tests/k8s-v2/runner --all --setup-timeout 30m --readiness-timeout 5m
```

Enable debug logging (dependency apply/wait/readiness traces):

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --debug
```

For interactive cluster tooling (for example `k9s`), use `--keep-cluster` and copy the printed `KUBECONFIG` export command from runner output.

Use the Go wrapper directly:

```sh
go run ./integration-tests/k8s-v2/runner --help
go run ./integration-tests/k8s-v2/runner list
go run ./integration-tests/k8s-v2/runner --all
go run ./integration-tests/k8s-v2/runner --test logs-loki -- --count=1
```

## Notes

- The runner creates a KinD cluster through e2e-framework and installs only selected dependencies.
- On child test failure, output includes a deterministic repro command.
- The wrapper auto-discovers tests from `tests/*/requirements.yaml` so new tests do not require wrapper code changes.
- Run `go run ./integration-tests/k8s-v2/runner --help` for full Cobra help and flags.
- The runner always prints resolved test absolute paths and the exact `go test` command before execution.
- The harness prints high-level lifecycle steps (cluster setup, dependency readiness, test execution, and cleanup).
