package linode

import (
	"testing"
	"time"

	promconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/linode"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	refresh_interval = "10s"
	port = 8080
	tag_separator = ";"
	basic_auth {
		username = "test"
		password = "pass"
	}
	http_headers = {
		"foo" = ["foobar"],
	}
`
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestConvert(t *testing.T) {
	alloyArgs := Arguments{
		Port:            8080,
		RefreshInterval: 15 * time.Second,
		TagSeparator:    ";",
		Region:          "us-west",
		HTTPClientConfig: config.HTTPClientConfig{
			BearerToken: "FOO",
			BasicAuth: &config.BasicAuth{
				Username: "test",
				Password: "pass",
			},
			HTTPHeaders: &config.Headers{
				Headers: map[string][]alloytypes.Secret{
					"foo": {"foobar"},
				},
			},
		},
	}

	promArgs := alloyArgs.Convert().(*prom_discovery.SDConfig)
	require.Equal(t, 8080, promArgs.Port)
	require.Equal(t, model.Duration(15*time.Second), promArgs.RefreshInterval)
	require.Equal(t, "us-west", promArgs.Region)
	require.Equal(t, ";", promArgs.TagSeparator)
	require.Equal(t, promconfig.Secret("FOO"), promArgs.HTTPClientConfig.BearerToken)
	require.Equal(t, "test", promArgs.HTTPClientConfig.BasicAuth.Username)
	require.Equal(t, "pass", string(promArgs.HTTPClientConfig.BasicAuth.Password))

	header := promArgs.HTTPClientConfig.HTTPHeaders.Headers["foo"].Secrets[0]
	require.Equal(t, "foobar", string(header))
}

func TestValidate(t *testing.T) {
	t.Run("validate RefreshInterval", func(t *testing.T) {
		alloyArgs := Arguments{
			RefreshInterval: 0,
		}
		err := alloyArgs.Validate()
		require.ErrorContains(t, err, "refresh_interval must be greater than 0")
	})
}
