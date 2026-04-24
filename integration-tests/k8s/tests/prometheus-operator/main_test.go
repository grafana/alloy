//go:build alloyintegrationtests

package prometheusoperator

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestMain(m *testing.M) {
	harness.RunTestMain(m, harness.Options{
		Name:       "prometheus-operator",
		ConfigPath: "./testdata/config.alloy",
		Workloads:  "./testdata/workloads.yaml",
		Backends:   []harness.Backend{harness.BackendMimir},
	})
}
