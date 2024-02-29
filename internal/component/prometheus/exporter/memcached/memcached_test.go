package memcached

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/static/integrations/memcached_exporter"
	"github.com/grafana/river"
	"github.com/stretchr/testify/assert"
)

func TestRiverUnmarshal(t *testing.T) {
	var exampleRiverConfig = `
address = "localhost:99"
timeout = "5s"`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	assert.NoError(t, err)

	expected := Arguments{
		Address: "localhost:99",
		Timeout: 5 * time.Second,
	}

	assert.Equal(t, expected, args)
}

func TestRiverUnmarshalTLS(t *testing.T) {
	var exampleRiverConfig = `
address = "localhost:99"
timeout = "5s"
tls_config {
  ca_file   = "/path/to/ca_file"
  cert_file = "/path/to/cert_file"
  key_file  = "/path/to/key_file"
}`
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	assert.NoError(t, err)

	expected := Arguments{
		Address: "localhost:99",
		Timeout: 5 * time.Second,
		TLSConfig: &config.TLSConfig{
			CAFile:   "/path/to/ca_file",
			CertFile: "/path/to/cert_file",
			KeyFile:  "/path/to/key_file",
		},
	}
	assert.Equal(t, expected, args)

	var invalidRiverConfig = `
address = "localhost:99"
timeout = "5s"
tls_config {
	ca_pem  = "ca"
	ca_file = "/path/to/ca_file"
}`
	err = river.Unmarshal([]byte(invalidRiverConfig), &args)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "at most one of")
}

func TestValidateNilTLSConfig(t *testing.T) {
	var args Arguments = Arguments{}
	err := args.Validate()
	assert.NoError(t, err)
}

func TestRiverUnmarshalDefaults(t *testing.T) {
	var exampleRiverConfig = ``

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	assert.NoError(t, err)

	expected := DefaultArguments

	assert.Equal(t, expected, args)
}

func TestRiverConvert(t *testing.T) {
	riverArguments := Arguments{
		Address: "localhost:99",
		Timeout: 5 * time.Second,
	}

	expected := &memcached_exporter.Config{
		MemcachedAddress: "localhost:99",
		Timeout:          5 * time.Second,
	}

	assert.Equal(t, expected, riverArguments.Convert())
}
