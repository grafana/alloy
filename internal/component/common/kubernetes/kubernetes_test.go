package kubernetes

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalRiver(t *testing.T) {
	var exampleRiverConfig = `
		api_server = "localhost:9091"
		proxy_url = "http://0.0.0.0:11111"
	`
	var args ClientArguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	exampleRiverConfig = `
		kubeconfig_file = "/etc/k8s/kubeconfig.yaml"
	`
	var args1 ClientArguments
	err = river.Unmarshal([]byte(exampleRiverConfig), &args1)
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
			err := river.Unmarshal([]byte(test.config), &args)
			require.Error(t, err)
		})
	}
}
