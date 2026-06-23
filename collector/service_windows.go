//go:build windows

package main

import "golang.org/x/sys/windows/svc"

func newSericeHandler() *alloyServiceHandler {
	return &alloyServiceHandler{}
}

var _ svc.Handler = (*alloyServiceHandler)(nil)

type alloyServiceHandler struct {
}

// Execute implements svc.Handler.
func (a *alloyServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	return false, 0
}
