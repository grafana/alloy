package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// serviceManager manages an individual binary.
type serviceManager struct {
	log *slog.Logger
	cfg serviceManagerConfig
}

// serviceManagerConfig configures a service.
type serviceManagerConfig struct {
	// path to the binary to run.
	path string

	// args of the binary to run, not including the command itself.
	args []string

	// environment of the binary to run, including the command environment itself.
	environment []string

	// dir specifies the working directory to run the binary from. If dir is
	// empty, the working directory of the current process is used.
	dir string

	// stdout and stderr specify where the process' stdout and stderr will be
	// connected.
	//
	// If stdout or stderr are nil, they will default to os.DevNull.
	stdout, stderr io.Writer
}

// newServiceManager creates a new, unstarted serviceManager. Call
// [service.Run] to start the serviceManager.
//
// Logs from the serviceManager will be sent to w. Logs from the managed
// service will be written to cfg.stdout and cfg.stderr as appropriate.
func newServiceManager(l *slog.Logger, cfg serviceManagerConfig) *serviceManager {
	if l == nil {
		l = slog.New(slog.DiscardHandler)
	}

	return &serviceManager{
		log: l,
		cfg: cfg,
	}
}

// run starts the serviceManager. The binary associated with the serviceManager
// will be run until the provided context is canceled or the binary exits.
//
// Intermediate restarts will increase with an exponential backoff, which
// resets if the binary has been running for longer than the maximum
// exponential backoff period.
func (svc *serviceManager) run(ctx context.Context) {
	cmd := svc.buildCommand(ctx)

	svc.log.Info("starting program", "command", cmd.String())
	err := cmd.Run()

	// Handle the context being canceled before processing whether cmd.Run
	// exited with an error.
	if ctx.Err() != nil {
		return
	}

	exitCode := cmd.ProcessState.ExitCode()

	if err != nil {
		svc.log.Error("service exited with error", "err", err, "exit_code", exitCode)
	} else {
		svc.log.Info("service exited", "exit_code", exitCode)
	}
	os.Exit(exitCode)
}

func (svc *serviceManager) buildCommand(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, svc.cfg.path, svc.cfg.args...)
	cmd.Dir = svc.cfg.dir
	cmd.Stdout = svc.cfg.stdout
	cmd.Stderr = svc.cfg.stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, svc.cfg.environment...)

	// Put the child in its own process group so we can target it specifically
	// with a console control event.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	cmd.Cancel = func() error {
		return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))

	}

	return cmd
}
