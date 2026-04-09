# k8s-v2 integration tests

Requirement-driven Kubernetes integration tests with a shared KinD lifecycle.

## What it demonstrates

- Test discovery from `tests/*/requirements.yaml`
- Per-run dependency union planning (`mimir`, `loki`)
- Single-test and multi-test execution from one runner
- Eventual assertions against Mimir and Loki APIs

## Test layout

Each test directory contains:

- `requirements.yaml` for dependency declarations
- `helm-alloy-values.yaml` as test-specific Alloy Helm values
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

The runner automatically sets required Go build tags:

- `alloyintegrationtests`
- `k8sv2integrationtests`

So manual runs stay ergonomic; you do not need to type tags yourself when using the runner.

Run selected tests manually (exact folder names):

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --test logs-loki
```

`--all` and `--test` are mutually exclusive. Use one or the other.

Keep the KinD cluster for debugging:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --keep-cluster
```

Keep cluster and dependencies untouched:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --keep-cluster --keep-deps
```

Reuse an existing Kind cluster:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --reuse-cluster alloy-k8s-v2-dev
```

Reuse an existing Kind cluster and skip dependency install/uninstall:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --reuse-cluster alloy-k8s-v2-dev --reuse-deps
```

Pass extra `go test` flags:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir -- --count=1
```

If you run `go test` directly (without the runner), include both tags or the k8s-v2 tests are intentionally excluded:

```sh
go test -tags "alloyintegrationtests k8sv2integrationtests" ./integration-tests/k8s-v2
```

Pass test folder paths (relative to your current directory):

```sh
go run ./integration-tests/k8s-v2/runner --test integration-tests/k8s-v2/tests/logs-loki
```

Each `--test` path is validated:

- the folder must exist,
- it must map to a discovered k8s-v2 test folder.

Each selected test must include `helm-alloy-values.yaml`; the harness installs Alloy from the local chart at `operations/helm/charts/alloy` in namespace `alloy`.

Tune setup/readiness timeouts:

```sh
go run ./integration-tests/k8s-v2/runner --all --setup-timeout 30m --readiness-timeout 5m
```

Enable debug logging (dependency apply/wait/readiness traces):

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --debug
```

For interactive cluster tooling, use `--keep-cluster` and copy the printed `KUBECONFIG` export command from runner output.

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
- High-level lifecycle step logs include durations to help identify slow areas.
- `--reuse-cluster` reuses an existing cluster by name. Reused clusters are left untouched by cleanup.
- `--reuse-deps` skips dependency install/uninstall when reusing a cluster; no dependency validation is performed.
- k8s-v2 tests are tag-gated so generic `go test ./...` jobs do not run them.
- Namespaces are explicit by role: Alloy=`alloy`, Loki=`loki`, Mimir=`mimir`, workloads=`k8s-v2-workloads`.
