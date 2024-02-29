package linode

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/river"
	promconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	refresh_interval = "10s"
	port = 8080
	tag_separator = ";"
	basic_auth {
		username = "test"
		password = "pass"
	}
`
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
}

func TestConvert(t *testing.T) {
	riverArgs := Arguments{
		Port:            8080,
		RefreshInterval: 15 * time.Second,
		TagSeparator:    ";",
		HTTPClientConfig: config.HTTPClientConfig{
			BearerToken: "FOO",
			BasicAuth: &config.BasicAuth{
				Username: "test",
				Password: "pass",
			},
		},
	}

	promArgs := riverArgs.Convert()
	require.Equal(t, 8080, promArgs.Port)
	require.Equal(t, model.Duration(15*time.Second), promArgs.RefreshInterval)
	require.Equal(t, ";", promArgs.TagSeparator)
	require.Equal(t, promconfig.Secret("FOO"), promArgs.HTTPClientConfig.BearerToken)
	require.Equal(t, "test", promArgs.HTTPClientConfig.BasicAuth.Username)
	require.Equal(t, "pass", string(promArgs.HTTPClientConfig.BasicAuth.Password))
}

func TestValidate(t *testing.T) {
	t.Run("validate RefreshInterval", func(t *testing.T) {
		riverArgs := Arguments{
			RefreshInterval: 0,
		}
		err := riverArgs.Validate()
		require.ErrorContains(t, err, "refresh_interval must be greater than 0")
	})
}
