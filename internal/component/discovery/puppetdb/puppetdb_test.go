package puppetdb

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

var exampleRiverConfig = `
url = "https://www.example.com"
query = "abc"
include_parameters = true
port = 29
refresh_interval = "1m"
basic_auth {
	username = "123"
	password = "456"
}
`

func TestRiverConfig(t *testing.T) {
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
	assert.Equal(t, args.HTTPClientConfig.BasicAuth.Username, "123")
	assert.Equal(t, args.RefreshInterval, time.Minute)
	assert.Equal(t, args.URL, "https://www.example.com")
	assert.Equal(t, args.Query, "abc")
	assert.Equal(t, args.IncludeParameters, true)
	assert.Equal(t, args.Port, 29)
}

func TestConvert(t *testing.T) {
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	sd := args.Convert()
	assert.Equal(t, "https://www.example.com", sd.URL)
	assert.Equal(t, model.Duration(60*time.Second), sd.RefreshInterval)
	assert.Equal(t, "abc", sd.Query)
	assert.Equal(t, true, sd.IncludeParameters)
	assert.Equal(t, 29, sd.Port)
}

func TestValidate(t *testing.T) {
	riverArgsBadUrl := Arguments{
		URL: string([]byte{0x7f}), // a control character to make url.Parse fail
	}
	err := riverArgsBadUrl.Validate()
	assert.ErrorContains(t, err, "net/url: invalid")

	riverArgsBadScheme := Arguments{
		URL: "smtp://foo.bar",
	}
	err = riverArgsBadScheme.Validate()
	assert.ErrorContains(t, err, "URL scheme must be")

	riverArgsNoHost := Arguments{
		URL: "http://#abc",
	}
	err = riverArgsNoHost.Validate()
	assert.ErrorContains(t, err, "host is missing in URL")
}
