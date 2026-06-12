//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"go.uber.org/atomic"
	"golang.org/x/sys/unix"
)

var socketCounter atomic.Uint64

func abstractSocketAddr(role, componentID string) string {
	return fmt.Sprintf("@alloy-beyla-%s-%s-%d-%d", role, componentID, os.Getpid(), socketCounter.Add(1))
}

var beylaSubprocessCaps = []uintptr{
	unix.CAP_BPF,
	unix.CAP_NET_ADMIN,
	unix.CAP_NET_RAW,
	unix.CAP_PERFMON,
	unix.CAP_DAC_READ_SEARCH,
	unix.CAP_SYS_PTRACE,
	unix.CAP_CHECKPOINT_RESTORE,
	unix.CAP_SYS_RESOURCE,
	unix.CAP_SYS_ADMIN,
}

// raiseSubprocessCaps transfers beylaSubprocessCaps to the Beyla child via the ambient
// capability mechanism. The returned cleanup restores the parent's original sets.
//
// Required when running as non-root: setcap +ip (not +eip) seeds the permitted set
// without granting effective caps to Alloy itself, and this transfers them to the child:
//
//	setcap 'cap_bpf,cap_net_admin,cap_net_raw,cap_perfmon,cap_dac_read_search,cap_sys_ptrace,cap_checkpoint_restore,cap_sys_resource+ip' /path/to/alloy
func raiseSubprocessCaps() (func(), error) {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}

	var data [2]unix.CapUserData

	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return nil, fmt.Errorf("capget: %w", err)
	}

	orig := data

	for _, cap := range beylaSubprocessCaps {
		if cap < 32 {
			if data[0].Permitted&(1<<cap) == 0 {
				continue
			}
			data[0].Inheritable |= 1 << cap
		} else {
			if data[1].Permitted&(1<<(cap-32)) == 0 {
				continue
			}
			data[1].Inheritable |= 1 << (cap - 32)
		}
	}

	if err := unix.Capset(&hdr, &data[0]); err != nil {
		return nil, fmt.Errorf("capset: %w", err)
	}

	for _, cap := range beylaSubprocessCaps {
		var inInheritable bool
		if cap < 32 {
			inInheritable = data[0].Inheritable&(1<<cap) != 0
		} else {
			inInheritable = data[1].Inheritable&(1<<(cap-32)) != 0
		}
		if !inInheritable {
			continue
		}
		if err := unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_RAISE, cap, 0, 0); err != nil {
			_ = unix.Capset(&hdr, &orig[0])
			return nil, fmt.Errorf("raise ambient cap %d: %w", cap, err)
		}
	}

	return func() {
		for _, cap := range beylaSubprocessCaps {
			_ = unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_LOWER, cap, 0, 0)
		}
		_ = unix.Capset(&hdr, &orig[0])
	}, nil
}

// embeddedBeylaBinary returns the embedded Beyla binary for the current architecture.
// Only a placeholder is present until `make download-beyla` writes the real binary, so
// this returns an error when the binary hasn't been downloaded.
func embeddedBeylaBinary() ([]byte, error) {
	return beylaBinaryFS.ReadFile("binaries/" + runtime.GOARCH + "/beyla")
}

func (c *Component) extractBeylaExecutable() (string, func(), error) {
	binary, err := embeddedBeylaBinary()
	if err != nil {
		return "", nil, fmt.Errorf("embedded Beyla binary unavailable (run `make download-beyla`): %w", err)
	}

	fd, err := createExecMemfd("beyla")

	if err != nil {
		return "", nil, fmt.Errorf("failed to create in-memory file: %w", err)
	}

	if err := writeData(fd, binary); err != nil {
		unix.Close(fd)
		return "", nil, fmt.Errorf("failed to write binary: %w", err)
	}

	binPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	c.opts.Logger.Debug("loaded Beyla binary into memory", "size", len(binary))

	return binPath, func() { unix.Close(fd) }, nil
}

func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func (c *Component) runSubprocess(ctx context.Context) error {
	exePath, configPath, port, profilePort := c.subprocess.Launch()

	if exePath == "" {
		return fmt.Errorf("beyla executable path not set")
	}
	if configPath == "" {
		return fmt.Errorf("config path not set")
	}

	cmd := exec.CommandContext(ctx, exePath, "-config", configPath)

	// Ensure Beyla subprocess gets killed when Alloy dies (even with SIGKILL).
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	cmd.Env = os.Environ()
	if profilePort != 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BEYLA_PROFILE_PORT=%d", profilePort))
	}

	cmd.Stdout = &logWriter{logger: c.opts.Logger, level: "info"}
	cmd.Stderr = &logWriter{logger: c.opts.Logger, level: "error"}

	c.subprocess.SetCmd(cmd)

	c.opts.Logger.Info("starting Beyla subprocess", "binary", exePath, "port", port, "config", configPath)

	// Pin this goroutine to its OS thread so that the cap modifications below
	// are visible to the fork() that cmd.Start() performs on this same thread.
	runtime.LockOSThread()
	cleanupCaps, err := raiseSubprocessCaps()
	if err != nil {
		runtime.UnlockOSThread()
		c.opts.Logger.Error("failed to prepare subprocess capabilities", "err", err)
		c.health.SetUnhealthy(err)
		return fmt.Errorf("failed to prepare subprocess capabilities: %w", err)
	}

	startErr := cmd.Start()

	// Restore parent's cap sets and release the thread lock before blocking on Wait.
	cleanupCaps()
	runtime.UnlockOSThread()

	if startErr != nil {
		c.opts.Logger.Error("failed to start Beyla subprocess", "err", startErr)
		c.health.SetUnhealthy(startErr)
		return fmt.Errorf("failed to start subprocess: %w", startErr)
	}

	// Child has exec'd and holds its own reference to the memfd inode; close ours now.
	c.subprocess.CloseBinary()

	c.opts.Logger.Info("Beyla subprocess started", "pid", cmd.Process.Pid)

	err = cmd.Wait()
	if err != nil && ctx.Err() == nil {
		// Subprocess exited unexpectedly (not due to context cancellation).
		c.opts.Logger.Error("Beyla subprocess exited unexpectedly", "err", err)
		c.health.SetUnhealthy(err)
		return err
	}

	c.opts.Logger.Info("Beyla subprocess stopped")
	return nil
}

type logWriter struct {
	logger *slog.Logger
	level  string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(string(p), "\n")
	if w.level == "error" {
		w.logger.Error(msg, "source", "beyla-subprocess")
	} else {
		w.logger.Info(msg, "source", "beyla-subprocess")
	}
	return len(p), nil
}
