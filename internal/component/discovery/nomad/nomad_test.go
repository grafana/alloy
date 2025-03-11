package nomad

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/nomad"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/syntax"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyCfg := `
		allow_stale = false
		namespace = "foo"
		refresh_interval = "20s"
		region = "test"
		server = "http://foo:4949"
		tag_separator = ";"
		enable_http2 = true
		follow_redirects = false
		proxy_url = "http://example:8080"
		http_headers = {
			"foo" = ["foobar"],
		}`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	assert.Equal(t, false, args.AllowStale)
	assert.Equal(t, "foo", args.Namespace)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, "test", args.Region)
	assert.Equal(t, "http://foo:4949", args.Server)
	assert.Equal(t, ";", args.TagSeparator)
	assert.Equal(t, true, args.HTTPClientConfig.EnableHTTP2)
	assert.Equal(t, false, args.HTTPClientConfig.FollowRedirects)
	assert.Equal(t, "http://example:8080", args.HTTPClientConfig.ProxyConfig.ProxyURL.String())

	header := args.HTTPClientConfig.HTTPHeaders.Headers["foo"][0]
	assert.Equal(t, "foobar", string(header))
}

func TestConvert(t *testing.T) {
	alloyArgsOAuth := Arguments{
		AllowStale:      false,
		Namespace:       "test",
		RefreshInterval: time.Minute,
		Region:          "a",
		Server:          "http://foo:111",
		TagSeparator:    ";",
		HTTPClientConfig: config.HTTPClientConfig{
			HTTPHeaders: &config.Headers{
				Headers: map[string][]alloytypes.Secret{
					"foo": {"foobar"},
				},
			},
		},
	}

	promArgs := alloyArgsOAuth.Convert().(*prom_discovery.SDConfig)
	assert.Equal(t, false, promArgs.AllowStale)
	assert.Equal(t, "test", promArgs.Namespace)
	assert.Equal(t, "a", promArgs.Region)
	assert.Equal(t, model.Duration(time.Minute), promArgs.RefreshInterval)
	assert.Equal(t, "http://foo:111", promArgs.Server)
	assert.Equal(t, ";", promArgs.TagSeparator)

	header := promArgs.HTTPClientConfig.HTTPHeaders.Headers["foo"].Secrets[0]
	assert.Equal(t, "foobar", string(header))
}

func TestValidate(t *testing.T) {
	alloyArgsNoServer := Arguments{
		Server: "",
	}
	err := alloyArgsNoServer.Validate()
	assert.Error(t, err, "nomad SD configuration requires a server address")
}
