package consulagent

import (
	"testing"
	"time"

	"github.com/grafana/river"
	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvert(t *testing.T) {
	var exampleRiverConfig = `
	server = "localhost:8500"
	token = "token"
	datacenter = "dc"
	tag_separator = ","
	scheme = "scheme"
	username = "username"
	password = "pass"
	refresh_interval = "5m"
	services = ["service1", "service2"]
	tags = ["tag1", "tag2"]
	tls_config {
		ca_file = "/path/to/ca_file"
	}
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	converted := args.Convert()
	assert.Equal(t, "localhost:8500", converted.Server)
	assert.Equal(t, promcfg.Secret("token"), converted.Token)
	assert.Equal(t, "dc", converted.Datacenter)
	assert.Equal(t, ",", converted.TagSeparator)
	assert.Equal(t, "scheme", converted.Scheme)
	assert.Equal(t, "username", converted.Username)
	assert.Equal(t, promcfg.Secret("pass"), converted.Password)
	assert.Equal(t, model.Duration(5*time.Minute), converted.RefreshInterval)
	expectedServices := []string{"service1", "service2"}
	require.ElementsMatch(t, expectedServices, converted.Services)
	expectedTags := []string{"tag1", "tag2"}
	require.ElementsMatch(t, expectedTags, converted.ServiceTags)
	assert.Equal(t, "username", converted.Username)
	assert.Equal(t, "/path/to/ca_file", converted.TLSConfig.CAFile)
}

func TestBadTLSRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	server = "localhost:8500"
	token = "token"
	datacenter = "dc"
	tag_separator = ","
	scheme = "scheme"
	username = "username"
	password = "pass"
	refresh_interval = "10s"
	services = ["service1", "service2"]
	tags = ["tag1", "tag2"]
	tls_config {
		ca_file = "/path/to/ca_file"
		ca_pem = "not a real pem"
	}
`

	// Make sure the TLSConfig Validate function is being utilized correctly.
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.ErrorContains(t, err, "at most one of ca_pem and ca_file must be configured")
}

func TestBadRefreshIntervalRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	server = "localhost:8500"
	token = "token"
	datacenter = "dc"
	tag_separator = ","
	scheme = "scheme"
	username = "username"
	password = "pass"
	refresh_interval = "-1s"
	services = ["service1", "service2"]
	tags = ["tag1", "tag2"]
	tls_config {
		ca_file = "/path/to/ca_file"
	}
`

	// Make sure the Refresh Interval is tested correctly.
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.ErrorContains(t, err, "refresh_interval must be greater than 0")
}
