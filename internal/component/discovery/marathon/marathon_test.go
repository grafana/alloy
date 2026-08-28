package marathon

import (
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/marathon"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func TestAlloyUnmarshalWithAuthToken(t *testing.T) {
	alloyCfg := `
		servers = ["serv1", "serv2"]
		refresh_interval = "20s"
		auth_token = "auth_token"
		`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"serv1", "serv2"}, args.Servers)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, alloytypes.Secret("auth_token"), args.AuthToken)
}

func TestAlloyUnmarshalWithAuthTokenFile(t *testing.T) {
	alloyCfg := `
		servers = ["serv1", "serv2"]
		refresh_interval = "20s"
		auth_token_file = "auth_token_file"
		`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"serv1", "serv2"}, args.Servers)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, "auth_token_file", args.AuthTokenFile)
}

func TestAlloyUnmarshalWithBasicAuth(t *testing.T) {
	alloyCfg := `
		servers = ["serv1", "serv2"]
		refresh_interval = "20s"
		basic_auth {
			username = "username"
			password = "pass"
		}
		`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"serv1", "serv2"}, args.Servers)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, "username", args.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, alloytypes.Secret("pass"), args.HTTPClientConfig.BasicAuth.Password)
}

func TestConvert(t *testing.T) {
	alloyArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: time.Minute,
		AuthToken:       "auth_token",
		AuthTokenFile:   "auth_token_file",
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username: "username",
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
	require.ElementsMatch(t, []string{"serv1", "serv2"}, promArgs.Servers)
	assert.Equal(t, model.Duration(time.Minute), promArgs.RefreshInterval)
	assert.Equal(t, promConfig.Secret("auth_token"), promArgs.AuthToken)
	assert.Equal(t, "auth_token_file", promArgs.AuthTokenFile)
	assert.Equal(t, "username", promArgs.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("pass"), promArgs.HTTPClientConfig.BasicAuth.Password)

	header := promArgs.HTTPClientConfig.HTTPHeaders.Headers["foo"].Secrets[0]
	assert.Equal(t, "foobar", string(header))
}

func TestValidateNoServers(t *testing.T) {
	alloyArgs := Arguments{
		Servers:         []string{},
		RefreshInterval: 10 * time.Second,
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "at least one Marathon server must be specified")
}

func TestValidateAuthTokenAndAuthTokenFile(t *testing.T) {
	alloyArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		AuthTokenFile:   "auth_token_file",
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "at most one of auth_token and auth_token_file must be configured")
}

func TestValidateAuthTokenAndBasicAuth(t *testing.T) {
	alloyArgs := Arguments{
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
	err := alloyArgs.Validate()
	assert.Error(t, err, "at most one of basic_auth, auth_token & auth_token_file must be configured")
}

func TestValidateAuthTokenAndBearerToken(t *testing.T) {
	alloyArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		HTTPClientConfig: config.HTTPClientConfig{
			BearerToken: "bearerToken",
		},
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "at most one of bearer_token, bearer_token_file, auth_token & auth_token_file must be configured")
}

func TestValidateAuthTokenAndBearerTokenFile(t *testing.T) {
	alloyArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		HTTPClientConfig: config.HTTPClientConfig{
			BearerTokenFile: "bearerToken",
		},
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "at most one of bearer_token, bearer_token_file, auth_token & auth_token_file must be configured")
}

func TestValidateAuthTokenAndAuthorization(t *testing.T) {
	alloyArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 10 * time.Second,
		AuthToken:       "auth_token",
		HTTPClientConfig: config.HTTPClientConfig{
			Authorization: &config.Authorization{
				CredentialsFile: "creds",
			},
		},
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "at most one of auth_token, auth_token_file & authorization must be configured")
}

func TestValidateRefreshInterval(t *testing.T) {
	alloyArgs := Arguments{
		Servers:         []string{"serv1", "serv2"},
		RefreshInterval: 0,
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "refresh_interval must be greater than 0")
}
