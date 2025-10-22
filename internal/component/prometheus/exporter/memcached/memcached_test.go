package memcached

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/static/integrations/memcached_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
)

func TestAlloyUnmarshal(t *testing.T) {
	var exampleAlloyConfig = `
address = "localhost:99"
timeout = "5s"`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	assert.NoError(t, err)

	expected := Arguments{
		Address: "localhost:99",
		Timeout: 5 * time.Second,
	}

	assert.Equal(t, expected, args)
}

func TestAlloyUnmarshalTLS(t *testing.T) {
	var exampleAlloyConfig = `
address = "localhost:99"
timeout = "5s"
tls_config {
  ca_file   = "/path/to/ca_file"
  cert_file = "/path/to/cert_file"
  key_file  = "/path/to/key_file"
}`
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
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

	var invalidAlloyConfig = `
address = "localhost:99"
timeout = "5s"
tls_config {
	ca_pem  = "ca"
	ca_file = "/path/to/ca_file"
}`
	err = syntax.Unmarshal([]byte(invalidAlloyConfig), &args)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "at most one of")
}

func TestValidateNilTLSConfig(t *testing.T) {
	var args = Arguments{}
	err := args.Validate()
	assert.NoError(t, err)
}

func TestAlloyUnmarshalDefaults(t *testing.T) {
	var exampleAlloyConfig = ``

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	assert.NoError(t, err)

	expected := DefaultArguments

	assert.Equal(t, expected, args)
}

func TestAlloyConvert(t *testing.T) {
	alloyArguments := Arguments{
		Address: "localhost:99",
		Timeout: 5 * time.Second,
	}

	expected := &memcached_exporter.Config{
		MemcachedAddress: "localhost:99",
		Timeout:          5 * time.Second,
	}

	assert.Equal(t, expected, alloyArguments.Convert())
}
