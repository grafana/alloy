package vcs

import (
	"fmt"
	"os"

	"github.com/prometheus/common/config"
)

type GitTLSConfig struct {
	InsecureSkipTLS bool
	ClientCert      []byte
	ClientKey       []byte
	CABundle        []byte
}

func newGitTLSConfig(config *config.TLSConfig) (*GitTLSConfig, error) {
	var certBytes, keyBytes, caBytes []byte

	if len(config.CertFile) > 0 {
		bb, err := os.ReadFile(config.CertFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load server certificate: %w", err)
		}
		certBytes = bb
	} else if len(config.Cert) > 0 {
		certBytes = []byte(config.Cert)
	}

	if len(config.KeyFile) > 0 {
		bb, err := os.ReadFile(config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load server key: %w", err)
		}
		keyBytes = bb
	} else if len(config.Key) > 0 {
		keyBytes = []byte(config.Key)
	}

	if len(config.CAFile) > 0 {
		bb, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load client CA certificate: %w", err)
		}
		caBytes = bb
	} else if len(config.CA) > 0 {
		caBytes = []byte(config.CA)
	}

	return &GitTLSConfig{
		InsecureSkipTLS: config.InsecureSkipVerify,
		ClientCert:      certBytes,
		ClientKey:       keyBytes,
		CABundle:        caBytes,
	}, nil
}
