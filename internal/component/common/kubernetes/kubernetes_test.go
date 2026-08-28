package kubernetes

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalAlloy(t *testing.T) {
	var exampleAlloyConfig = `
		api_server = "localhost:9091"
		proxy_url = "http://0.0.0.0:11111"
	`
	var args ClientArguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	exampleAlloyConfig = `
		kubeconfig_file = "/etc/k8s/kubeconfig.yaml"
	`
	var args1 ClientArguments
	err = syntax.Unmarshal([]byte(exampleAlloyConfig), &args1)
	require.NoError(t, err)
}

func TestBadConfigs(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name: "api_server and kubeconfig_file",
			config: `
				api_server = "localhost:9091"
				kubeconfig_file = "/etc/k8s/kubeconfig.yaml"
			`,
		},
		{
			name: "kubeconfig_file and custom HTTP client",
			config: `
				kubeconfig_file = "/etc/k8s/kubeconfig.yaml"
				proxy_url = "http://0.0.0.0:11111"
			`,
		},
		{
			name: "api_server missing when using custom HTTP client",
			config: `
				proxy_url = "http://0.0.0.0:11111"
			`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var args ClientArguments
			err := syntax.Unmarshal([]byte(test.config), &args)
			require.Error(t, err)
		})
	}
}
