package aws

import (
	"net/url"
	"testing"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestConvert(t *testing.T) {
	// parse example proxy
	u, err := url.Parse("http://example:8080")
	require.NoError(t, err)
	httpClientConfig := config.DefaultHTTPClientConfig
	httpClientConfig.ProxyConfig = &config.ProxyConfig{
		ProxyURL: config.URL{
			URL: u,
		},
	}

	// example configuration
	alloyArgs := EC2Arguments{
		Region:           "us-east-1",
		HTTPClientConfig: httpClientConfig,
	}

	// ensure values are set
	promArgs := alloyArgs.Convert()
	assert.Equal(t, "us-east-1", promArgs.Region)
	assert.Equal(t, "http://example:8080", promArgs.HTTPClientConfig.ProxyURL.String())
}
