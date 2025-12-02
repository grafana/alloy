//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"golang.org/x/sys/unix"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// beylaSubprocessCaps are the capabilities Beyla needs to perform eBPF instrumentation.
var beylaSubprocessCaps = []uintptr{
	unix.CAP_BPF,             // core eBPF; kernels < 5.8 need CAP_SYS_ADMIN instead
	unix.CAP_NET_ADMIN,       // context propagation and network TC
	unix.CAP_NET_RAW,         // app and network observability (fallback when NET_ADMIN absent)
	unix.CAP_PERFMON,         // app and network observability
	unix.CAP_DAC_READ_SEARCH, // app observability
	unix.CAP_SYS_PTRACE,      // app observability
	unix.CAP_CHECKPOINT_RESTORE,
	unix.CAP_SYS_RESOURCE, // kernels < 5.11
	unix.CAP_SYS_ADMIN,    // optional: NodeJS/Go library injection; required on kernels < 5.8
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
			unix.Capset(&hdr, &orig[0])
			return nil, fmt.Errorf("raise ambient cap %d: %w", cap, err)
		}
	}

	return func() {
		for _, cap := range beylaSubprocessCaps {
			unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_LOWER, cap, 0, 0)
		}
		unix.Capset(&hdr, &orig[0])
	}, nil
}

// createExecMemfd creates an anonymous executable file descriptor.
// MFD_EXEC is required on kernels >= 6.3 when vm.memfd_noexec > 0.
// Older kernels don't recognise the flag and return EINVAL, so we fall back.
func createExecMemfd(name string) (int, error) {
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC|unix.MFD_EXEC)
	if err != nil && errors.Is(err, unix.EINVAL) {
		fd, err = unix.MemfdCreate(name, unix.MFD_CLOEXEC)
	}
	return fd, err
}

func (c *Component) extractBeylaExecutable() (string, func(), error) {
	fd, err := createExecMemfd("beyla")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create in-memory file: %w", err)
	}

	data := beylaEmbeddedBinary
	for len(data) > 0 {
		n, err := unix.Write(fd, data)
		if err != nil {
			unix.Close(fd)
			return "", nil, fmt.Errorf("failed to write binary: %w", err)
		}
		data = data[n:]
	}

	binPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	level.Debug(c.opts.Logger).Log("msg", "loaded Beyla binary into memory", "size", len(beylaEmbeddedBinary))

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
	c.mut.Lock()
	exePath := c.beylaExePath
	configPath := c.configPath
	port := c.subprocessPort
	c.mut.Unlock()

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

	cmd.Stdout = &logWriter{logger: c.opts.Logger, level: "info"}
	cmd.Stderr = &logWriter{logger: c.opts.Logger, level: "error"}

	c.mut.Lock()
	c.subprocessCmd = cmd
	c.mut.Unlock()

	level.Info(c.opts.Logger).Log("msg", "starting Beyla subprocess", "binary", exePath, "port", port, "config", configPath)

	// Pin this goroutine to its OS thread so that the cap modifications below
	// are visible to the fork() that cmd.Start() performs on this same thread.
	runtime.LockOSThread()
	cleanupCaps, err := raiseSubprocessCaps()
	if err != nil {
		runtime.UnlockOSThread()
		level.Error(c.opts.Logger).Log("msg", "failed to prepare subprocess capabilities", "err", err)
		c.reportUnhealthy(err)
		return fmt.Errorf("failed to prepare subprocess capabilities: %w", err)
	}

	startErr := cmd.Start()

	// Restore parent's cap sets and release the thread lock before blocking on Wait.
	cleanupCaps()
	runtime.UnlockOSThread()

	if startErr != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to start Beyla subprocess", "err", startErr)
		c.reportUnhealthy(startErr)
		return fmt.Errorf("failed to start subprocess: %w", startErr)
	}

	// Child has exec'd and holds its own reference to the memfd inode; close ours now.
	c.mut.Lock()
	if c.beylaExeClose != nil {
		c.beylaExeClose()
		c.beylaExeClose = nil
	}
	c.mut.Unlock()

	level.Info(c.opts.Logger).Log("msg", "Beyla subprocess started", "pid", cmd.Process.Pid, "binary_size", len(beylaEmbeddedBinary))

	err = cmd.Wait()
	if err != nil && ctx.Err() == nil {
		// Subprocess exited unexpectedly (not due to context cancellation).
		level.Error(c.opts.Logger).Log("msg", "Beyla subprocess exited unexpectedly", "err", err)
		c.reportUnhealthy(err)
		return err
	}

	level.Info(c.opts.Logger).Log("msg", "Beyla subprocess stopped")
	return nil
}

func (c *Component) healthCheckLoop(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Beyla loads eBPF programs before opening its Prometheus port (~15-20s).
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(20 * time.Second):
	}

	consecutiveSuccesses := 0
	consecutiveFailures := 0
	const successesNeededToResetBackoff = 3
	const failuresBeforeKill = 5

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := c.checkSubprocessHealth(); err != nil {
				consecutiveFailures++
				level.Warn(c.opts.Logger).Log("msg", "subprocess health check failed", "err", err, "consecutive_failures", consecutiveFailures)
				c.reportUnhealthy(err)
				consecutiveSuccesses = 0
				if consecutiveFailures >= failuresBeforeKill {
					return fmt.Errorf("subprocess unresponsive after %d consecutive health check failures", consecutiveFailures)
				}
			} else {
				consecutiveFailures = 0
				c.mut.Lock()
				c.subprocessReady = true
				c.mut.Unlock()
				c.reportHealthy()
				consecutiveSuccesses++

				if consecutiveSuccesses >= successesNeededToResetBackoff {
					c.mut.Lock()
					if c.restartBackoff > 1*time.Second {
						level.Debug(c.opts.Logger).Log("msg", "resetting restart backoff after successful health checks")
						c.restartBackoff = 1 * time.Second
						c.restartCount = 0
					}
					c.mut.Unlock()
					consecutiveSuccesses = 0
				}
			}
		}
	}
}

func (c *Component) checkSubprocessHealth() error {
	c.mut.Lock()
	addr := c.subprocessAddr
	c.mut.Unlock()

	if addr == "" {
		return fmt.Errorf("subprocess not started")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", addr+"/metrics", nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("subprocess not responding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("subprocess returned status %d", resp.StatusCode)
	}
	return nil
}

type logWriter struct {
	logger log.Logger
	level  string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(string(p), "\n")
	if w.level == "error" {
		level.Error(w.logger).Log("msg", msg, "source", "beyla-subprocess")
	} else {
		level.Info(w.logger).Log("msg", msg, "source", "beyla-subprocess")
	}
	return len(p), nil
}
