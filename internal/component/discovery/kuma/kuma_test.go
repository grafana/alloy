package kuma

import (
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/xds"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	server = "http://kuma-control-plane.kuma-system.svc:5676"

	refresh_interval = "10s"
	fetch_timeout    = "50s"

	http_headers = {
		"foo" = ["foobar"],
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestBadAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	server = "http://kuma-control-plane.kuma-system.svc:5676"
	tls_config {
		ca_file = "/path/to/ca_file"
		ca_pem = "not a real pem"
	}
`

	// Make sure the TLSConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of ca_pem and ca_file must be configured")
}

func TestConvert(t *testing.T) {
	alloyArgs := Arguments{
		Server:          "srv",
		RefreshInterval: 30 * time.Second,
		FetchTimeout:    10 * time.Second,
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username: "username",
				Password: "pass",
			},
		},
	}

	promArgs := alloyArgs.Convert().(*prom_discovery.SDConfig)
	require.Equal(t, "srv", promArgs.Server)
	require.Equal(t, model.Duration(30*time.Second), promArgs.RefreshInterval)
	require.Equal(t, model.Duration(10*time.Second), promArgs.FetchTimeout)
	require.Equal(t, "username", promArgs.HTTPClientConfig.BasicAuth.Username)
	require.Equal(t, promConfig.Secret("pass"), promArgs.HTTPClientConfig.BasicAuth.Password)
}

func TestValidateNoServers(t *testing.T) {
	t.Run("validate fetch timeout", func(t *testing.T) {
		alloyArgs := Arguments{
			RefreshInterval: 10 * time.Second,
		}
		err := alloyArgs.Validate()
		require.ErrorContains(t, err, "fetch_timeout must be greater than 0")
	})
	t.Run("validate refresh interval", func(t *testing.T) {
		alloyArgs := Arguments{
			FetchTimeout: 10 * time.Second,
		}
		err := alloyArgs.Validate()
		require.ErrorContains(t, err, "refresh_interval must be greater than 0")
	})
}
