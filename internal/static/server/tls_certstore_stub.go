//go:build !windows || !cgo

package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
)

type WinCertStoreHandler struct {
}

func NewWinCertStoreHandler(cfg WindowsCertificateFilter, clientAuth tls.ClientAuthType, l *slog.Logger) (*WinCertStoreHandler, error) {
	return nil, errors.New("windows certificate store is not supported on this platform or without CGO")
}

func (w *WinCertStoreHandler) Run() {}

func (w *WinCertStoreHandler) Stop() {}

func (w *WinCertStoreHandler) CertificateHandler(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, errors.New("not implemented")
}

func (w *WinCertStoreHandler) VerifyPeer(_ [][]byte, verifiedChains [][]*x509.Certificate) error {
	return errors.New("not implemented")
}
