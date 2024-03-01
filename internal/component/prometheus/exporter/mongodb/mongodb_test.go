package mongodb

import (
	"testing"

	"github.com/grafana/agent/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	riverConfig := `
	mongodb_uri = "mongodb://127.0.0.1:27017"
	direct_connect = true
	discovering_mode = true
	tls_basic_auth_config_path = "/etc/path-to-file"
	`

	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		URI:                    "mongodb://127.0.0.1:27017",
		DirectConnect:          true,
		DiscoveringMode:        true,
		TLSBasicAuthConfigPath: "/etc/path-to-file",
	}

	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	riverConfig := `
	mongodb_uri = "mongodb://127.0.0.1:27017"
	direct_connect = true
	discovering_mode = true
	tls_basic_auth_config_path = "/etc/path-to-file"
	`
	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := mongodb_exporter.Config{
		URI:                    "mongodb://127.0.0.1:27017",
		DirectConnect:          true,
		DiscoveringMode:        true,
		TLSBasicAuthConfigPath: "/etc/path-to-file",
	}
	require.Equal(t, expected, *res)
}
