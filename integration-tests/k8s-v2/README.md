# k8s-v2 integration tests

Requirement-driven Kubernetes integration tests with a shared KinD lifecycle.

## What it demonstrates

- Test discovery from `tests/*/requirements.yaml`
- Per-run dependency union planning (`mimir`, `loki`)
- Single-test and multi-test execution from one runner
- Eventual assertions against Mimir and Loki APIs

## Architecture

The harness runs as three cooperating processes. Understanding the split
makes it easier to reason about flags, context propagation, and where new
code belongs.

1. **Runner** (`runner/main.go`). A small Cobra CLI used locally and in CI.
   Its only job is to discover selected tests, resolve them to paths and
   names, and invoke `go test` on the harness package with the right build
   tags and internal `-k8s.v2.*` flags. It never talks to Kubernetes.

2. **Harness** (`./...`, built as `go test` with both build tags set). A
   single `TestMain` that:
   - plans selected tests and dependency union,
   - creates (or reuses) a KinD cluster,
   - installs shared dependencies (Loki/Mimir) in fixed namespaces,
   - runs each selected test as a parallel subtest of `TestIntegrationV2`,
     each subtest installing Alloy via Helm into a per-test namespace and
     applying test-specific workload manifests.

3. **Per-test assertion binary** (`tests/<name>/assert_test.go`). Each test
   ships its assertions in its own package, which the harness invokes as
   another `go test` subprocess (see `runGoTestPackage`). That gives every
   test its own flag namespace and repro command, and isolates imports so
   backends can evolve independently.

Shared dependency metadata (namespace, service, readiness path, manifest
file) lives once in `internal/backendspec` and is consumed by both the
installer in `internal/deps` and the port-forward helper in
`internal/assert`. Adding a new backend means one new `backendspec.Spec`
plus one embedded manifest under `internal/deps/manifests/`.

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
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --reuse-cluster alloy-it-dev
```

Reuse an existing Kind cluster and skip dependency install/uninstall:

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --reuse-cluster alloy-it-dev --reuse-deps
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

Control concurrent test execution (default is 4):

```sh
go run ./integration-tests/k8s-v2/runner --all --parallel 4
```

Enable debug logging (dependency apply/wait/readiness traces):

```sh
go run ./integration-tests/k8s-v2/runner --test metrics-mimir --debug
```

Run tests against a locally built Alloy image (loaded into Kind and forced in Helm):

```sh
make ALLOY_IMAGE=alloy-ci:dev alloy-image
go run ./integration-tests/k8s-v2/runner --all --alloy-image alloy-ci:dev --alloy-image-pull-policy IfNotPresent
```

`--alloy-image` must be a `repository:tag` reference. Digest pins
(`repo@sha256:...`) are rejected because Helm `--set-string image.tag` is
not compatible with digest-based references.

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
- Selected tests run in parallel by default (`--parallel`, default `4`) with isolated per-test namespace/release names.
- `--alloy-image` loads a local image into Kind and overrides Helm `image.repository`/`image.tag` so assertions run against the PR artifact instead of a released image.
- k8s-v2 tests are tag-gated so generic `go test ./...` jobs do not run them.
- Dependencies are shared in fixed namespaces: Loki=`loki`, Mimir=`mimir`.
- Runtime isolation uses a per-test `test_id` label and a per-test namespace/release derived from the test name.
- Test assets support `${TEST_ID}` and `${TEST_NAMESPACE}` placeholders. `${TEST_NAMESPACE}` is used for both namespace and Helm release.
