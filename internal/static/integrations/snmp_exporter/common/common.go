package common

import (
	"bytes"
	"compress/gzip"
	_ "embed" // enables the go:embed directive
	"io"

	snmp_config "github.com/prometheus/snmp_exporter/config"
	"gopkg.in/yaml.v2"
)

//go:generate curl https://raw.githubusercontent.com/prometheus/snmp_exporter/main/snmp.yml --output snmp.yml
//go:generate gzip -9 snmp.yml
//go:embed snmp.yml.gz
var content []byte

// LoadEmbeddedConfig loads the SNMP config via a file using the go:embed directive.
func LoadEmbeddedConfig() (*snmp_config.Config, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}

	cfg := &snmp_config.Config{}
	err = yaml.UnmarshalStrict(b, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
