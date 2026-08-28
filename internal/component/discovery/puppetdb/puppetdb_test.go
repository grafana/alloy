package puppetdb

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/puppetdb"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/syntax"
)

var exampleAlloyConfig = `
url = "https://www.example.com"
query = "abc"
include_parameters = true
port = 29
refresh_interval = "1m"
basic_auth {
	username = "123"
	password = "456"
}
http_headers = {
	"foo" = ["foobar"],
}
`

func TestAlloyConfig(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
	assert.Equal(t, args.HTTPClientConfig.BasicAuth.Username, "123")
	assert.Equal(t, args.RefreshInterval, time.Minute)
	assert.Equal(t, args.URL, "https://www.example.com")
	assert.Equal(t, args.Query, "abc")
	assert.Equal(t, args.IncludeParameters, true)
	assert.Equal(t, args.Port, 29)

	header := args.HTTPClientConfig.HTTPHeaders.Headers["foo"][0]
	assert.Equal(t, "foobar", string(header))
}

func TestConvert(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	sd := args.Convert().(*prom_discovery.SDConfig)
	assert.Equal(t, "https://www.example.com", sd.URL)
	assert.Equal(t, model.Duration(60*time.Second), sd.RefreshInterval)
	assert.Equal(t, "abc", sd.Query)
	assert.Equal(t, true, sd.IncludeParameters)
	assert.Equal(t, 29, sd.Port)

	header := sd.HTTPClientConfig.HTTPHeaders.Headers["foo"].Secrets[0]
	assert.Equal(t, "foobar", string(header))
}

func TestValidate(t *testing.T) {
	alloyArgsBadUrl := Arguments{
		URL: string([]byte{0x7f}), // a control character to make url.Parse fail
	}
	err := alloyArgsBadUrl.Validate()
	assert.ErrorContains(t, err, "net/url: invalid")

	alloyArgsBadScheme := Arguments{
		URL: "smtp://foo.bar",
	}
	err = alloyArgsBadScheme.Validate()
	assert.ErrorContains(t, err, "URL scheme must be")

	alloyArgsNoHost := Arguments{
		URL: "http://#abc",
	}
	err = alloyArgsNoHost.Validate()
	assert.ErrorContains(t, err, "host is missing in URL")
}
