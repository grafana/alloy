# Kubernetes Integration Tests

Run the Kubernetes integration tests with:

```sh
make integration-test-k8s
```

`integration-tests/k8s/run.sh` is the canonical entrypoint. It always uses a
runner-managed kind cluster and kubeconfig (never your default kube context),
then executes `go test` for `integration-tests/k8s/tests/...`.

Useful options (forwarded with `RUN_ARGS`):

```sh
make integration-test-k8s RUN_ARGS='--reuse-cluster'
make integration-test-k8s RUN_ARGS='--skip-alloy-image'
make integration-test-k8s RUN_ARGS='--shard 0/2'
make integration-test-k8s RUN_ARGS='--package ./integration-tests/k8s/tests/prometheus-operator'
```

Controller type is chosen per test package in its `TestMain` via
`harness.Options{Controller: ...}`.

If reuse mode leaves a broken cluster behind:

```sh
kind delete cluster --name alloy-k8s-integration
```
