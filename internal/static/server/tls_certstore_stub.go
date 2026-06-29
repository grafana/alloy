//go:build !windows || !cgo

package server

type WinCertStoreHandler struct {
}

func (w WinCertStoreHandler) Run() {}

func (w WinCertStoreHandler) Stop() {}
