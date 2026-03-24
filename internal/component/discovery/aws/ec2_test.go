package aws

import (
	"net/url"
	"testing"

	promaws "github.com/prometheus/prometheus/discovery/aws"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax/alloytypes"
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

	httpClientConfig.HTTPHeaders = &config.Headers{
		Headers: map[string][]alloytypes.Secret{
			"foo": {"foobar"},
		},
	}

	// example configuration
	alloyArgs := EC2Arguments{
		Region:           "us-east-1",
		HTTPClientConfig: httpClientConfig,
	}

	// ensure values are set
	converted := alloyArgs.Convert()
	promArgs, ok := converted.(*promaws.EC2SDConfig)
	require.True(t, ok)
	assert.Equal(t, "us-east-1", promArgs.Region)
	assert.Equal(t, "http://example:8080", promArgs.HTTPClientConfig.ProxyURL.String())

	header := promArgs.HTTPClientConfig.HTTPHeaders.Headers["foo"].Secrets[0]
	assert.Equal(t, "foobar", string(header))
}
