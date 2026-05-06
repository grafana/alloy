# Kubernetes Integration Tests

Run the Kubernetes integration tests with:

```sh
make integration-test-k8s
```

`integration-tests/k8s/runner` is the canonical entrypoint. It always uses a
runner-managed kind cluster and kubeconfig (never your default kube context),
then executes `go test` for `integration-tests/k8s/tests/...`.

Useful options (forwarded with `RUN_ARGS`):

```sh
make integration-test-k8s RUN_ARGS='--reuse-cluster'
make integration-test-k8s RUN_ARGS='--skip-alloy-image'
# Split test packages across 2 shards and run shard index 0.
make integration-test-k8s RUN_ARGS='--shard 0/2'
make integration-test-k8s RUN_ARGS='--package ./integration-tests/k8s/tests/prometheus-operator'
```

Per-test Alloy chart options (controller type, replicas, stability level, etc.)
are set via a helm values file in the test's `config/alloy-values.yaml` and
passed to `deps.NewAlloy(deps.AlloyOptions{ValuesPath: ...})`.

If reuse mode leaves a broken cluster behind:

```sh
kind delete cluster --name alloy-k8s-integration
```
