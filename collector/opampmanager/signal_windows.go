//go:build windows

package opampmanager

import "errors"

func SignalReload() error {
	return errors.New("opampmanager: SIGHUP reload not supported on Windows")
}
