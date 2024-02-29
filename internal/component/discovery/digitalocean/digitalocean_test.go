package digitalocean

import (
	"net/url"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/river"
	prom_common_config "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestRiverUnmarshal(t *testing.T) {
	var exampleRiverConfig = `
	refresh_interval = "5m"
	port = 8181
	bearer_token = "token"
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Minute, args.RefreshInterval)
	assert.Equal(t, 8181, args.Port)
	assert.Equal(t, "token", string(args.BearerToken))

	var fullerExampleRiverConfig = `
	refresh_interval = "3m"
	port = 9119
	proxy_url = "http://proxy:8080"
	follow_redirects = true
	enable_http2 = false
	bearer_token = "token"
	`
	err = river.Unmarshal([]byte(fullerExampleRiverConfig), &args)
	require.NoError(t, err)
	assert.Equal(t, 3*time.Minute, args.RefreshInterval)
	assert.Equal(t, 9119, args.Port)
	assert.Equal(t, "http://proxy:8080", args.ProxyConfig.ProxyURL.String())
	assert.Equal(t, true, args.FollowRedirects)
	assert.Equal(t, false, args.EnableHTTP2)
}

func TestBadRiverConfig(t *testing.T) {
	var badConfigTooManyBearerTokens = `
	refresh_interval = "5m"
	port = 8181
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"
	`

	var args Arguments
	err := river.Unmarshal([]byte(badConfigTooManyBearerTokens), &args)
	require.ErrorContains(t, err, "exactly one of bearer_token or bearer_token_file must be specified")

	var badConfigMissingAuth = `
	refresh_interval = "5m"
	port = 8181
	`
	var args2 Arguments
	err = river.Unmarshal([]byte(badConfigMissingAuth), &args2)
	require.ErrorContains(t, err, "exactly one of bearer_token or bearer_token_file must be specified")
}

func TestConvert(t *testing.T) {
	proxyUrl, _ := url.Parse("http://example:8080")
	args := Arguments{
		RefreshInterval: 5 * time.Minute,
		Port:            8181,
		BearerToken:     "token",
		ProxyConfig: &config.ProxyConfig{
			ProxyURL: config.URL{
				URL: proxyUrl,
			},
		},
		FollowRedirects: false,
		EnableHTTP2:     false,
	}

	converted := args.Convert()
	assert.Equal(t, model.Duration(5*time.Minute), converted.RefreshInterval)
	assert.Equal(t, 8181, converted.Port)
	assert.Equal(t, prom_common_config.Secret("token"), converted.HTTPClientConfig.BearerToken)
	assert.Equal(t, "http://example:8080", converted.HTTPClientConfig.ProxyURL.String())
	assert.Equal(t, false, converted.HTTPClientConfig.FollowRedirects)
	assert.Equal(t, false, converted.HTTPClientConfig.EnableHTTP2)
}
