package nomad

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestRiverUnmarshal(t *testing.T) {
	riverCfg := `
		allow_stale = false
		namespace = "foo"
		refresh_interval = "20s"
		region = "test"
		server = "http://foo:4949"
		tag_separator = ";"
		enable_http2 = true
		follow_redirects = false
		proxy_url = "http://example:8080"`

	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
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
}

func TestConvert(t *testing.T) {
	riverArgsOAuth := Arguments{
		AllowStale:      false,
		Namespace:       "test",
		RefreshInterval: time.Minute,
		Region:          "a",
		Server:          "http://foo:111",
		TagSeparator:    ";",
	}

	promArgs := riverArgsOAuth.Convert()
	assert.Equal(t, false, promArgs.AllowStale)
	assert.Equal(t, "test", promArgs.Namespace)
	assert.Equal(t, "a", promArgs.Region)
	assert.Equal(t, model.Duration(time.Minute), promArgs.RefreshInterval)
	assert.Equal(t, "http://foo:111", promArgs.Server)
	assert.Equal(t, ";", promArgs.TagSeparator)
}

func TestValidate(t *testing.T) {
	riverArgsNoServer := Arguments{
		Server: "",
	}
	err := riverArgsNoServer.Validate()
	assert.Error(t, err, "nomad SD configuration requires a server address")
}
