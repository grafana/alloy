//go:build !windows

package http

import (
	"crypto/tls"
	"fmt"

	"github.com/grafana/agent/internal/static/server"
)

// tlsConfig generates a tls.Config from args.
func (args *TLSArguments) winTlsConfig(_ *server.WinCertStoreHandler) (*tls.Config, error) {
	return nil, fmt.Errorf("Windows Certificate filter is only available on Windows platforms.")
}

func (s *Service) updateWindowsCertificateFilter(_ *TLSArguments) error {
	return fmt.Errorf("Windows Certificate filter is only available on Windows platforms.")
}
