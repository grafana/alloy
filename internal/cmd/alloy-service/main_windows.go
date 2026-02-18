package main

import (
	"context"
	"os"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"golang.org/x/sys/windows/svc"

	// Embed application manifest for Windows builds
	_ "github.com/grafana/alloy/internal/winmanifest"
)

const serviceName = "Alloy"

func main() {
	// Service wrapper logs (e.g. "starting program", "service exited") go to
	// stderr. Alloy's own logs are handled by the Alloy process; configure
	// windows_event_log in Alloy's logging block to write to Windows Event Log.
	logger := log.NewLogfmtLogger(os.Stderr)

	managerConfig, err := loadConfig()
	if err != nil {
		level.Error(logger).Log("msg", "failed to run service", "err", err)
		os.Exit(1)
	}

	cfg := serviceManagerConfig{
		Path:        managerConfig.ServicePath,
		Args:        managerConfig.Args,
		Environment: managerConfig.Environment,
		Dir:         managerConfig.WorkingDirectory,

		// Do not capture the child's stdout/stderr; Alloy logs via its own
		// logger (e.g. windows_event_log in the logging config).
		Stdout: nil,
		Stderr: nil,
	}

	as := &alloyService{logger: logger, cfg: cfg}
	if err := svc.Run(serviceName, as); err != nil {
		level.Error(logger).Log("msg", "failed to run service", "err", err)
		os.Exit(1)
	}
}

type alloyService struct {
	logger log.Logger
	cfg    serviceManagerConfig
}

const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

func (as *alloyService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	defer func() {
		s <- svc.Status{State: svc.Stopped}
	}()

	var workers sync.WaitGroup
	defer workers.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s <- svc.Status{State: svc.StartPending}

	// Run the serviceManager.
	{
		sm := newServiceManager(as.logger, as.cfg)

		workers.Add(1)
		go func() {
			// In case the service manager exits on its own, we cancel our context to
			// signal to the parent goroutine to exit.
			defer cancel()
			defer workers.Done()
			sm.Run(ctx)
		}()
	}

	s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	defer func() {
		s <- svc.Status{State: svc.StopPending}
	}()

	for {
		select {
		case <-ctx.Done():
			// Our managed service exited; shut down the service.
			return false, 0
		case req := <-r:
			switch req.Cmd {
			case svc.Interrogate:
				s <- req.CurrentStatus
			case svc.Pause, svc.Continue:
				// no-op
			default:
				// Every other command should terminate the service.
				return false, 0
			}
		}
	}
}
