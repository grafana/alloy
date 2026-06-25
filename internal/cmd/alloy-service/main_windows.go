package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"

	// Embed application manifest for Windows builds
	_ "github.com/grafana/alloy/internal/winmanifest"
)

const serviceName = "Alloy"

func main() {

	eventWriter, err := newWriter()
	if err != nil {
		// Ideally the logger never fails to be created, since if it does, there's
		// nowhere to send the failure to.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(eventWriter, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}))

	// Allocate a console that our child process will inherit. With it we can send a break event triggering
	// graceful shutdown of alloy.
	if r, _, err := windows.NewLazySystemDLL("kernel32.dll").NewProc("AllocConsole").Call(); r == 0 {
		logger.Error("failed to run service", "err", err)
		panic(err)
	}

	managerConfig, err := loadConfig()
	if err != nil {
		logger.Error("failed to run service", "err", err)
		os.Exit(1)
	}

	cfg := serviceManagerConfig{
		Path:        managerConfig.ServicePath,
		Args:        managerConfig.Args,
		Environment: managerConfig.Environment,
		Dir:         managerConfig.WorkingDirectory,

		// Send logs directly to the event logger.
		Stdout: eventWriter,
		Stderr: eventWriter,
	}

	as := &alloyService{logger: logger, cfg: cfg}
	if err := svc.Run(serviceName, as); err != nil {
		logger.Error("failed to run service", "err", err)
		os.Exit(1)
	}
}

type alloyService struct {
	logger *slog.Logger
	cfg    serviceManagerConfig
}

const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

func (as *alloyService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	defer func() {
		s <- svc.Status{State: svc.Stopped}
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s <- svc.Status{State: svc.StartPending}

	// Run the serviceManager.
	{
		sm := newServiceManager(as.logger, as.cfg)

		wg.Go(func() {
			// In case the service manager exits on its own, we cancel our context to
			// signal to the parent goroutine to exit.
			defer cancel()
			sm.Run(ctx)
		})
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
