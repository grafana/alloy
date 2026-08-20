//go:build !windows || !cgo

package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
)

type WinCertStoreHandler struct {
}

func (w *WinCertStoreHandler) Run() {}

func (w *WinCertStoreHandler) Stop() {}

func (w *WinCertStoreHandler) CertificateHandler(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, fmt.Errorf("the Windows Certificate filter is not available")
}

func (w *WinCertStoreHandler) VerifyPeer(_ [][]byte, verifiedChains [][]*x509.Certificate) error {
	return fmt.Errorf("the Windows Certificate filter is not available")
}

func NewWinCertStoreHandler(cfg WindowsCertificateFilter, clientAuth tls.ClientAuthType, l *slog.Logger) (*WinCertStoreHandler, error) {
	return nil, fmt.Errorf("the Windows Certificate filter is not available")
}
