//go:build !windows

package http

import (
	"crypto/tls"
	"fmt"

	"github.com/grafana/alloy/internal/static/server"
)

// tlsConfig generates a tls.Config from args.
func (args *TLSArguments) winTlsConfig(_ *server.WinCertStoreHandler) (*tls.Config, error) {
	return nil, fmt.Errorf("the Windows Certificate filter is only available on Windows platforms")
}

func (s *Service) updateWindowsCertificateFilter(_ *TLSArguments) error {
	return fmt.Errorf("the Windows Certificate filter is only available on Windows platforms")
}
