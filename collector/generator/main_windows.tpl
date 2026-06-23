// GENERATED CODE: DO NOT EDIT

//go:build windows

package main

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/otelcol"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func run(params otelcol.CollectorSettings) error {
	if err := svc.Run("alloy", newSericeHandler()); err != nil {
		if errors.Is(err, windows.ERROR_FAILED_SERVICE_CONTROLLER_CONNECT) {
			// Per https://learn.microsoft.com/en-us/windows/win32/api/winsvc/nf-winsvc-startservicectrldispatchera#return-value
			// this means that the process is not running as a service, so run interactively.
			return runInteractive(params)
		}

		return fmt.Errorf("failed to run alloy service: %w", err)
	}

	return nil
}
