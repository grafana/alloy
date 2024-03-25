package main

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// serviceManager manages an individual binary.
type serviceManager struct {
	log log.Logger
	cfg serviceManagerConfig
}

// serviceManagerConfig configures a service.
type serviceManagerConfig struct {
	// Path to the binary to run.
	Path string

	// Args of the binary to run, not including the command itself.
	Args []string

	// Environment of the binary to run, including the command environment itself.
	Environment []string

	// Dir specifies the working directory to run the binary from. If Dir is
	// empty, the working directory of the current process is used.
	Dir string

	// Stdout and Stderr specify where the process' stdout and stderr will be
	// connected.
	//
	// If Stdout or Stderr are nil, they will default to os.DevNull.
	Stdout, Stderr io.Writer
}

// newServiceManager creates a new, unstarted serviceManager. Call
// [service.Run] to start the serviceManager.
//
// Logs from the serviceManager will be sent to w. Logs from the managed
// service will be written to cfg.Stdout and cfg.Stderr as appropriate.
func newServiceManager(l log.Logger, cfg serviceManagerConfig) *serviceManager {
	if l == nil {
		l = log.NewNopLogger()
	}

	return &serviceManager{
		log: l,
		cfg: cfg,
	}
}

// Run starts the serviceManager. The binary associated with the serviceManager
// will be run until the provided context is canceled or the binary exits.
//
// Intermediate restarts will increase with an exponential backoff, which
// resets if the binary has been running for longer than the maximum
// exponential backoff period.
func (svc *serviceManager) Run(ctx context.Context) {
	cmd := svc.buildCommand(ctx)

	level.Info(svc.log).Log("msg", "starting program", "command", cmd.String())
	err := cmd.Run()

	// Handle the context being canceled before processing whether cmd.Run
	// exited with an error.
	if ctx.Err() != nil {
		return
	}

	exitCode := cmd.ProcessState.ExitCode()

	if err != nil {
		level.Error(svc.log).Log("msg", "service exited with error", "err", err, "exit_code", exitCode)
	} else {
		level.Info(svc.log).Log("msg", "service exited", "exit_code", exitCode)
	}
	os.Exit(exitCode)
}

func (svc *serviceManager) buildCommand(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, svc.cfg.Path, svc.cfg.Args...)
	cmd.Dir = svc.cfg.Dir
	cmd.Stdout = svc.cfg.Stdout
	cmd.Stderr = svc.cfg.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, svc.cfg.Environment...)
	return cmd
}
