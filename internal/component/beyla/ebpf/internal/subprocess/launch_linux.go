//go:build (linux && arm64) || (linux && amd64)

package subprocess

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

// Spec is the contract between Alloy and a subprocess: everything needed to launch it.
type Spec struct {
	Name   string   // memfd label, shown in /proc
	Binary []byte   // executable image, run from an anonymous in-memory file
	Args   []string // command-line arguments
	Env    []string // extra environment, appended to the parent's

	// PreExec runs on the locked OS thread immediately before exec, e.g. to raise
	// capabilities the fork must inherit. The returned func restores the parent's state.
	PreExec func() (cleanup func(), err error)

	Stdout io.Writer
	Stderr io.Writer
}

// Process is a launched subprocess.
type Process struct {
	cmd *exec.Cmd
}

// Start writes spec.Binary to an exec memfd, runs spec.PreExec on a locked OS thread,
// and execs it with Pdeathsig=SIGKILL so the child dies with Alloy. The parent's copy
// of the memfd is closed once the child holds its own reference.
func Start(ctx context.Context, spec Spec) (*Process, error) {
	fd, err := createExecMemfd(spec.Name)
	if err != nil {
		return nil, fmt.Errorf("create in-memory executable: %w", err)
	}

	if err := WriteAll(fd, spec.Binary); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("write executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, fmt.Sprintf("/proc/self/fd/%d", fd), spec.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr

	// Pin this goroutine to its OS thread so the PreExec changes are visible to the
	// fork() that cmd.Start() performs on this same thread.
	runtime.LockOSThread()

	var cleanup func()
	if spec.PreExec != nil {
		cleanup, err = spec.PreExec()
		if err != nil {
			runtime.UnlockOSThread()
			unix.Close(fd)
			return nil, fmt.Errorf("pre-exec: %w", err)
		}
	}

	startErr := cmd.Start()

	if cleanup != nil {
		cleanup()
	}
	runtime.UnlockOSThread()

	// The child has exec'd and holds its own reference to the memfd inode; drop ours.
	unix.Close(fd)

	if startErr != nil {
		return nil, fmt.Errorf("start subprocess: %w", startErr)
	}

	return &Process{cmd: cmd}, nil
}

// Wait blocks until the subprocess exits.
func (p *Process) Wait() error {
	return p.cmd.Wait()
}

// Pid returns the subprocess pid, or false if it has not started.
func (p *Process) Pid() (int, bool) {
	if p.cmd.Process == nil {
		return 0, false
	}

	return p.cmd.Process.Pid, true
}
