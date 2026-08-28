//go:build !windows

package server

type WinCertStoreHandler struct {
}

func (w WinCertStoreHandler) Run() {}

func (w WinCertStoreHandler) Stop() {}
