//go:build !windows

package opampmanager

import (
	"os"
	"syscall"
)

func SignalReload() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGHUP)
}
