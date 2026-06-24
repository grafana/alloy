//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/collector/otelcol"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

func newServiceHandler(set otelcol.CollectorSettings) *alloyServiceHandler {
	return &alloyServiceHandler{
		set: set,
	}
}

var _ svc.Handler = (*alloyServiceHandler)(nil)

type alloyServiceHandler struct {
	set otelcol.CollectorSettings
}

func (a *alloyServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	// NOTE: We don't send a Stopped status ourselves. The svc package reports
	// SERVICE_STOPPED automatically once Execute returns.

	// The first argument supplied to service.Execute is the service name.
	if len(args) == 0 {
		return false, 1213 // 1213: ERROR_INVALID_SERVICENAME
	}

	s <- svc.Status{State: svc.StartPending}

	serviceName := args[0]
	elog, err := openEventLog(serviceName)
	if err != nil {
		return false, 1501 // ERROR_EVENTLOG_CANT_START
	}
	defer elog.Close()

	// NOTE(kalleep): Change the working directory to the directory of the Alloy
	// executable. SCM starts services in C:\Windows\System32, but the previous
	// out-of-process wrapper ran Alloy from its install directory, so a user's
	// config may reference paths relative to it. We chdir here to stay compatible.
	if exe, err := os.Executable(); err != nil {
		elog.Warning(2, fmt.Sprintf("could not determine executable path: %v", err))
	} else if err := os.Chdir(filepath.Dir(exe)); err != nil {
		elog.Warning(2, fmt.Sprintf("could not change working directory: %v", err))
	}

	cfg, err := configFromRegistry()
	if err != nil {
		elog.Error(3, fmt.Sprintf("could not load config from registry: %v", err))
		return false, 1064 // ERROR_EXCEPTION_IN_SERVICE
	}

	for _, env := range cfg.environment {
		kv := strings.SplitN(env, "=", 2)
		if len(kv) != 2 {
			elog.Warning(3, "malformed environment variable")
			continue
		}

		if err := os.Setenv(kv[0], kv[1]); err != nil {
			elog.Error(3, fmt.Sprintf("could not set environment variable: %v", err))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := newAlloyCommand(a.set)
	cmd.SetArgs(cfg.args)
	cmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Execute() }()

	const accepts = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.Running, Accepts: accepts}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				elog.Error(3, fmt.Sprintf("unexpected exit: %v", err))
				return false, 1064 // ERROR_EXCEPTION_IN_SERVICE
			}
			return false, 0
		case req := <-r:
			switch req.Cmd {
			case svc.Interrogate:
				s <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				cancel()

				var exitCode uint32
				if err := <-errCh; err != nil {
					elog.Error(3, fmt.Sprintf("error during shutdown: %v", err))
					exitCode = 1064 // ERROR_EXCEPTION_IN_SERVICE
				}
				return false, exitCode
			}
		}
	}
}

// config holds configuration options to run the service.
type config struct {
	// args holds arguments to pass to the alloy. os.Args[0] is not included.
	args []string

	// environment holds environment variables for the Alloy service.
	// Each item represents an environment variable in form "key=value".
	// All environments variables from the current process with be merged into Environment
	environment []string
}

func configFromRegistry() (*config, error) {
	// NOTE(rfratto): the key name below shouldn't be changed without being
	// able to either migrate from the old key to the new key or supporting
	// both the old and the new key at the same time.
	alloyKey, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\GrafanaLabs\Alloy`, registry.READ)
	if err != nil {
		return nil, fmt.Errorf("failed to open registry: %w", err)
	}

	args, _, err := alloyKey.GetStringsValue("Arguments")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key Arguments: %w", err)
	}

	env, _, err := alloyKey.GetStringsValue("Environment")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key Environment: %w", err)
	}

	return &config{args: args, environment: env}, nil
}

func openEventLog(serviceName string) (*eventlog.Log, error) {
	eventTypes := uint32(eventlog.Info | eventlog.Warning | eventlog.Error)
	err := eventlog.InstallAsEventCreate(serviceName, eventTypes)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}
	return eventlog.Open(serviceName)
}
