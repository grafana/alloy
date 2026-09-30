package remoteawssecretsmanager

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestRemoteAWSSecretsManager(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-remote-aws-secrets-manager",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	moto := deps.NewMoto(deps.MotoOptions{Namespace: ns.Name()})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-remote-aws-secrets-manager",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, moto, mimir, alloy},
	})

	// This value must match the test_name field that the moto seed Job stores.
	const testName = "remote-aws-secrets-manager"

	mimir.QueryMetrics(t, testName, []string{
		"remote_aws_secrets_manager_fetches_total",
		"remote_aws_secrets_manager_timestamp_last_success_unix_seconds",
	})
	// QueryPositive adds its own label selector, so pass the bare metric name.
	// Without errors, only the success series is positive.
	mimir.QueryPositive(t, testName, []string{
		"remote_aws_secrets_manager_fetches_total",
	})
}
