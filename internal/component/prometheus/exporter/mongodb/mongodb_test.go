package mongodb

import (
	"testing"

	"github.com/grafana/alloy/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
	mongodb_uri = "mongodb://127.0.0.1:27017"
	direct_connect = true
	discovering_mode = true
	tls_basic_auth_config_path = "/etc/path-to-file"
	compatible_mode = true
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		URI:                    "mongodb://127.0.0.1:27017",
		DirectConnect:          true,
		DiscoveringMode:        true,
		TLSBasicAuthConfigPath: "/etc/path-to-file",
		CompatibleMode:         true,
	}

	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	alloyConfig := `
	mongodb_uri = "mongodb://127.0.0.1:27017"
	direct_connect = true
	discovering_mode = true
	tls_basic_auth_config_path = "/etc/path-to-file"
	compatible_mode = true
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := mongodb_exporter.Config{
		URI:                    "mongodb://127.0.0.1:27017",
		DirectConnect:          true,
		DiscoveringMode:        true,
		TLSBasicAuthConfigPath: "/etc/path-to-file",
		CompatibleMode:         true,
	}
	require.Equal(t, expected, *res)
}
