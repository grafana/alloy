//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"

	"go.uber.org/atomic"
	"golang.org/x/sys/unix"

	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
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

func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func (c *Component) runSubprocess(ctx context.Context) error {
	binary, err := embeddedBeylaBinary()
	if err != nil {
		err = fmt.Errorf("embedded Beyla binary unavailable (run `make download-beyla`): %w", err)
		c.opts.Logger.Error("failed to load Beyla binary", "err", err)
		c.health.SetUnhealthy(err)
		return err
	}

	c.opts.Logger.Info("starting Beyla subprocess", "port", c.subprocess.Port(), "config", c.subprocess.ConfigPath())

	proc, err := subprocess.Start(ctx, subprocess.Spec{
		Name:    "beyla",
		Binary:  binary,
		Args:    []string{"-config", c.subprocess.ConfigPath()},
		Env:     profileEnv(c.subprocess.ProfilePort()),
		PreExec: raiseSubprocessCaps,
		Stdout:  &logWriter{logger: c.opts.Logger, level: "info"},
		Stderr:  &logWriter{logger: c.opts.Logger, level: "error"},
	})
	if err != nil {
		c.opts.Logger.Error("failed to start Beyla subprocess", "err", err)
		c.health.SetUnhealthy(err)
		return err
	}

	pid, _ := proc.Pid()
	c.subprocess.SetPid(pid)
	c.opts.Logger.Info("Beyla subprocess started", "pid", pid)

	err = proc.Wait()
	if err != nil && ctx.Err() == nil {
		// Subprocess exited unexpectedly (not due to context cancellation).
		c.opts.Logger.Error("Beyla subprocess exited unexpectedly", "err", err)
		c.health.SetUnhealthy(err)
		return err
	}

	c.opts.Logger.Info("Beyla subprocess stopped")
	return nil
}

func profileEnv(profilePort int) []string {
	if profilePort == 0 {
		return nil
	}
	return []string{fmt.Sprintf("BEYLA_PROFILE_PORT=%d", profilePort)}
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
