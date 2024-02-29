package elasticsearch

import (
	"testing"
	"time"

	commonCfg "github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/static/integrations/elasticsearch_exporter"
	"github.com/grafana/river"
	"github.com/grafana/river/rivertypes"
	promCfg "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	riverConfig := `
	address              = "http://localhost:9300"
	timeout              = "10s"
	all                  = true
	node                 = "some_node"
	indices              = true
	indices_settings     = true
	cluster_settings     = true
	shards               = true
	aliases              = true
	snapshots            = true
	clusterinfo_interval = "10s"
	ca                   = "some_ca"
	client_private_key   = "some_client_cert"
	ssl_skip_verify      = true
	data_stream          = true
	slm                  = true
	basic_auth {
		username = "username"
		password = "pass"
	}
	`

	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		Address:                   "http://localhost:9300",
		Timeout:                   time.Duration(10) * time.Second,
		AllNodes:                  true,
		Node:                      "some_node",
		ExportIndices:             true,
		ExportIndicesSettings:     true,
		ExportClusterSettings:     true,
		ExportShards:              true,
		IncludeAliases:            true,
		ExportSnapshots:           true,
		ExportClusterInfoInterval: time.Duration(10) * time.Second,
		CA:                        "some_ca",
		ClientPrivateKey:          "some_client_cert",
		InsecureSkipVerify:        true,
		ExportDataStreams:         true,
		ExportSLM:                 true,
		BasicAuth: &commonCfg.BasicAuth{
			Username: "username",
			Password: rivertypes.Secret("pass"),
		},
	}

	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	riverConfig := `
	address              = "http://localhost:9300"
	timeout              = "10s"
	all                  = true
	node                 = "some_node"
	indices              = true
	indices_settings     = true
	cluster_settings     = true
	shards               = true
	aliases              = true
	snapshots            = true
	clusterinfo_interval = "10s"
	ca                   = "some_ca"
	client_private_key   = "some_client_cert"
	ssl_skip_verify      = true
	data_stream          = true
	slm                  = true
	basic_auth {
		username = "username"
		password = "pass"
	}
	`
	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := elasticsearch_exporter.Config{
		Address:                   "http://localhost:9300",
		Timeout:                   time.Duration(10) * time.Second,
		AllNodes:                  true,
		Node:                      "some_node",
		ExportIndices:             true,
		ExportIndicesSettings:     true,
		ExportClusterSettings:     true,
		ExportShards:              true,
		IncludeAliases:            true,
		ExportSnapshots:           true,
		ExportClusterInfoInterval: time.Duration(10) * time.Second,
		CA:                        "some_ca",
		ClientPrivateKey:          "some_client_cert",
		InsecureSkipVerify:        true,
		ExportDataStreams:         true,
		ExportSLM:                 true,
		BasicAuth: &promCfg.BasicAuth{
			Username: "username",
			Password: promCfg.Secret("pass"),
		},
	}
	require.Equal(t, expected, *res)
}
