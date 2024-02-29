package marathon

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/river"
	"github.com/grafana/river/rivertypes"
	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestRiverUnmarshalWithAuthToken(t *testing.T) {
	riverCfg := `
		servers = ["serv1", "serv2"]
		refresh_interval = "20s"
		auth_token = "auth_token"
		`

	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"serv1", "serv2"}, args.Servers)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, rivertypes.Secret("auth_token"), args.AuthToken)
}

func TestRiverUnmarshalWithAuthTokenFile(t *testing.T) {
	riverCfg := `
		servers = ["serv1", "serv2"]
		refresh_interval = "20s"
		auth_token_file = "auth_token_file"
		`

	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"serv1", "serv2"}, args.Servers)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, "auth_token_file", args.AuthTokenFile)
}

func TestRiverUnmarshalWithBasicAuth(t *testing.T) {
	riverCfg := `
		servers = ["serv1", "serv2"]
		refresh_interval = "20s"
		basic_auth {
			username = "username"
			password = "pass"
		}
		`

	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"serv1", "serv2"}, args.Servers)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, "username", args.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, rivertypes.Secret("pass"), args.HTTPClientConfig.BasicAuth.Password)
}

func TestConvert(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: time.Minute,
		AuthToken:       "auth_token",
		AuthTokenFile:   "auth_token_file",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username: "username",
				Password: "pass",
			},
		},
	}

	promArgs := riverArgs.Convert()
	require.ElementsMatch(t, []string{"serv1", "serv2"}, promArgs.Servers)
	assert.Equal(t, model.Duration(time.Minute), promArgs.RefreshInterval)
	assert.Equal(t, promConfig.Secret("auth_token"), promArgs.AuthToken)
	assert.Equal(t, "auth_token_file", promArgs.AuthTokenFile)
	assert.Equal(t, "username", promArgs.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("pass"), promArgs.HTTPClientConfig.BasicAuth.Password)
}

func TestValidateNoServers(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{},
		RefreshInterval: 10 * time.Second,
	}
	err := riverArgs.Validate()
	assert.Error(t, err, "at least one Marathon server must be specified")
}

func TestValidateAuthTokenAndAuthTokenFile(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		AuthTokenFile:   "auth_token_file",
	}
	err := riverArgs.Validate()
	assert.Error(t, err, "at most one of auth_token and auth_token_file must be configured")
}

func TestValidateAuthTokenAndBasicAuth(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username: "username",
				Password: "pass",
			},
		},
	}
	err := riverArgs.Validate()
	assert.Error(t, err, "at most one of basic_auth, auth_token & auth_token_file must be configured")
}

func TestValidateAuthTokenAndBearerToken(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		HTTPClientConfig: config.HTTPClientConfig{
			BearerToken: "bearerToken",
		},
	}
	err := riverArgs.Validate()
	assert.Error(t, err, "at most one of bearer_token, bearer_token_file, auth_token & auth_token_file must be configured")
}

func TestValidateAuthTokenAndBearerTokenFile(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		HTTPClientConfig: config.HTTPClientConfig{
			BearerTokenFile: "bearerToken",
		},
	}
	err := riverArgs.Validate()
	assert.Error(t, err, "at most one of bearer_token, bearer_token_file, auth_token & auth_token_file must be configured")
}

func TestValidateAuthTokenAndAuthorization(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		HTTPClientConfig: config.HTTPClientConfig{
			Authorization: &config.Authorization{
				CredentialsFile: "creds",
			},
		},
	}
	err := riverArgs.Validate()
	assert.Error(t, err, "at most one of auth_token, auth_token_file & authorization must be configured")
}

func TestValidateRefreshInterval(t *testing.T) {
	riverArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 0,
	}
	err := riverArgs.Validate()
	assert.Error(t, err, "refresh_interval must be greater than 0")
}
