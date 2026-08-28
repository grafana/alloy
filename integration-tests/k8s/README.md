# Kubernetes Integration Tests

## Overview

These are integration tests that run Alloy against real workloads on a local
Kubernetes cluster. We use [kind](https://kind.sigs.k8s.io/) for the cluster
and Helm chart to deploy Alloy.

CI parallelises these tests by sharding test **packages** (one package == one heavy
test that brings up its own dependencies, e.g. Mimir, Loki, etc.). Each CI job runs with
`--shard i/n`, and a package runs only when it hashes to the current shard. This
way every package runs on exactly one shard, and as we add more tests we
just bump `n` to keep wall-clock time flat — the test suite shouldn't
become a bottleneck.

The other priority is local developer experience. `make
integration-test-k8s-local-dev` opens an interactive menu for the common
options (reuse cluster, skip image build, pick a single package, pick a
shard) so you can iterate fast without remembering flags. The kubeconfig
is dropped at a known path (see below) so `kubectl` and `k9s` work
straight away.

Adding a new test should stay easy as the suite grows. Each test is a
plain Go test that calls `harness.Setup` with a list of dependencies
(`deps.Namespace`, `deps.Mimir`, `deps.Alloy`, ...). The harness installs
them in order and tears them down in reverse. To add a test: drop a new directory
under `tests/`, and use the existing tests as an inspiration.

Currently the cluster itself is reused across tests in a single run, but every
dependency is installed and torn down per-test. That keeps tests isolated
and easy to reason about, at the cost of some setup time (~20s per test). If specific
heavy dependencies (e.g. Mimir, the prometheus-operator CRDs) become a
real bottleneck we can promote them to cluster-scoped install-once
fixtures — the harness is structured to allow adding this later if needed.

In CI, all the docker images the suite needs (Alloy under test, plus test
fixture images like prom-gen) are built once in a separate job and
restored into each shard via `--skip-image-builds`, so adding more shards
doesn't multiply the build time.

## Running tests locally

### One-shot run all tests

```sh
make integration-test-k8s
```

### Local dev interactive menu (recommended)

```sh
make integration-test-k8s-local-dev
```

Opens a small TUI to pick the common run options before tests start.

- Reusing kind cluster or skipping image builds (alloy, prom-gen) to speed up local development.
- Filtering tests by shard or by package.

### Inspecting the running cluster

The runner writes its kubeconfig to `integration-tests/k8s/.kube/kubeconfig`
(gitignored). Pass it explicitly to your tools to connect to the cluster.

```sh
kubectl --kubeconfig integration-tests/k8s/.kube/kubeconfig get pods -A
k9s    --kubeconfig integration-tests/k8s/.kube/kubeconfig
```

### Run specific tests from CLI (if you don't want to use the interactive menu)

```sh
# Split test packages across 2 shards and run shard index 0. Use this to reproduce a CI run.
make integration-test-k8s RUN_ARGS='--shard 0/2'

# Run a single test package.
make integration-test-k8s RUN_ARGS='--package ./integration-tests/k8s/tests/prometheus-operator'

# Skip rebuilding all docker images (alloy, prom-gen). They must already
# exist in the local docker daemon — useful after a previous run or when
# iterating on a single test.
make integration-test-k8s RUN_ARGS='--skip-image-builds'
```
