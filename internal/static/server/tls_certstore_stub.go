//go:build !windows

package server

import "fmt"

func (l *tlsListener) applyWindowsCertificateStore(_ TLSConfig) error {
	return fmt.Errorf("cannot use Windows certificate store on non-Windows platforms")
}

type WinCertStoreHandler struct {
}

func (w WinCertStoreHandler) Run() {}

func (w WinCertStoreHandler) Stop() {}
