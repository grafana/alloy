package uyuni

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/river"
	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	cfg := `
	server = "https://uyuni.com"
	username = "exampleuser"
	password = "examplepassword"
	refresh_interval = "1m"
	tls_config {
		ca_file   = "/etc/ssl/certs/ca-certificates.crt"
		cert_file = "/etc/ssl/certs/client.crt"
		key_file  = "/etc/ssl/certs/client.key"
	}	
	`
	var args Arguments
	err := river.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)
}

func TestValidate(t *testing.T) {
	invalidServer := Arguments{
		Server:   "http://uyuni.com",
		Username: "exampleuser",
		Password: "examplepassword",
		TLSConfig: config.TLSConfig{
			CAFile:   "/etc/ssl/certs/ca-certificates.crt",
			CertFile: "/etc/ssl/certs/client.crt",

			// Check that the TLSConfig is being validated
			KeyFile: "/etc/ssl/certs/client.key",
			Key:     "key",
		},
	}

	err := invalidServer.Validate()
	require.Error(t, err)
}

func TestConvert(t *testing.T) {
	args := Arguments{
		Server:          "https://uyuni.com",
		Username:        "exampleuser",
		Password:        "examplepassword",
		RefreshInterval: 1 * time.Minute,
		EnableHTTP2:     true,
		FollowRedirects: true,
	}
	require.NoError(t, args.Validate())

	converted := args.Convert()
	require.Equal(t, "https://uyuni.com", converted.Server)
	require.Equal(t, "exampleuser", converted.Username)
	require.Equal(t, promcfg.Secret("examplepassword"), converted.Password)
	require.Equal(t, model.Duration(1*time.Minute), converted.RefreshInterval)
	require.Equal(t, promcfg.DefaultHTTPClientConfig, converted.HTTPClientConfig)
}
