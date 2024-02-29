//go:build windows

package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/static/config"
	"github.com/grafana/agent/internal/static/server"
	util_log "github.com/grafana/agent/internal/util/log"

	"golang.org/x/sys/windows/svc"
)

const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

// AgentService runs the Grafana Agent as a service.
type AgentService struct{}

// Execute starts the AgentService.
func (m *AgentService) Execute(args []string, serviceRequests <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	changes <- svc.Status{State: svc.StartPending}

	// Executable name and any command line parameters will be placed into os.args, this comes from
	// registry key `Computer\HKEY_LOCAL_MACHINE\SYSTEM\ControlSet001\Services\<servicename>\ImagePath`
	// oddly enough args is blank

	// Set up logging using default values before loading the config
	defaultServerCfg := server.DefaultConfig()
	logger := server.NewWindowsEventLogger(&defaultServerCfg)

	reloader := func(log *server.Logger) (*config.Config, error) {
		fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		return config.Load(fs, os.Args[1:], log)
	}
	cfg, err := reloader(logger)
	if err != nil {
		log.Fatalln(err)
	}

	// Pause is not accepted, we immediately set the service as running and trigger the entrypoint load in the background
	// this is because the WAL is reloaded and the timeout for a windows service starting is 30 seconds. In this case
	// the service is running but Agent may still be starting up reading the WAL and doing other operations.
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	// After this point we can start using go-kit logging.
	logger = server.NewWindowsEventLogger(cfg.Server)
	util_log.Logger = logger

	entrypointExit := make(chan error)

	// Kick off the server in the background so that we can respond to status queries
	var ep *Entrypoint
	go func() {
		ep, err = NewEntrypoint(logger, cfg, reloader)
		if err != nil {
			level.Error(logger).Log("msg", "error creating the agent server entrypoint", "err", err)
			os.Exit(1)
		}
		entrypointExit <- ep.Start()
	}()

loop:
	for {
		select {
		case c := <-serviceRequests:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			case svc.Pause:
			case svc.Continue:
			default:
				break loop
			}
		case err := <-entrypointExit:
			level.Error(logger).Log("msg", "error while running agent server entrypoint", "err", err)
			break loop
		}
	}
	// There is a chance the entrypoint may not be setup yet, in that case we don't want to stop.
	// Since it is in another go func it may start after this has returned, in either case the program
	// will exit.
	if ep != nil {
		ep.Stop()
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

// IsWindowsService returns whether the current process is running as a Windows
// Service. On non-Windows platforms, this always returns false.
func IsWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isService
}

// RunService runs the current process as a Windows servce. On non-Windows platforms,
// this is always a no-op.
func RunService() error {
	return svc.Run(server.ServiceName, &AgentService{})
}
