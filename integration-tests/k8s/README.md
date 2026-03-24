Use this command to run the tests:

```
make integration-test-k8s
```

To debug the test you can also set two environment variables:
* `ALLOY_STATEFUL_K8S_TEST=true` will retain the k8s clusters after the test terminates.
* `ALLOY_K8S_TEST_LOGGING=debug` will get the test to print log messages.


For example:

```
ALLOY_STATEFUL_K8S_TEST=true ALLOY_K8S_TEST_LOGGING=debug make integration-test-k8s
```

After you have finished debugging you can delete the clusters like this:

```
minikube delete -p alloy-int-test-prometheus-operator
minikube delete -p alloy-int-test-mimir-alerts-kubernetes
```
