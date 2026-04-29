package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"time"
)

type PortForwardConfig struct {
	Kubeconfig    string
	Namespace     string
	Service       string
	TargetPort    int
	ReadinessPath string
	PollInterval  time.Duration
	ReadyTimeout  time.Duration
}

type PortForwardHandle struct {
	BaseURL string

	cancel context.CancelFunc
	done   chan error
	stderr *bytes.Buffer
	stdout *bytes.Buffer
}

func (h *PortForwardHandle) Close() error {
	h.cancel()
	select {
	case err := <-h.done:
		return err
	case <-time.After(5 * time.Second):
		return nil
	}
}

func StartPortForwardAndWait(ctx context.Context, cfg PortForwardConfig) (*PortForwardHandle, error) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.ReadyTimeout <= 0 {
		cfg.ReadyTimeout = 2 * time.Minute
	}
	localPort, err := allocatePort()
	if err != nil {
		return nil, fmt.Errorf("allocate local port: %w", err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", localPort)

	pfCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(
		pfCtx,
		"kubectl",
		"--kubeconfig", cfg.Kubeconfig,
		"-n", cfg.Namespace,
		"port-forward",
		"svc/"+cfg.Service,
		fmt.Sprintf("%d:%d", localPort, cfg.TargetPort),
	)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start port-forward for %s/%s: %w", cfg.Namespace, cfg.Service, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	handle := &PortForwardHandle{
		BaseURL: baseURL,
		cancel:  cancel,
		done:    done,
		stderr:  stderr,
		stdout:  stdout,
	}

	readyURL := baseURL + cfg.ReadinessPath
	readyCtx, readyCancel := context.WithTimeout(ctx, cfg.ReadyTimeout)
	defer readyCancel()

	var lastErr error
	for {
		select {
		case err := <-done:
			return nil, fmt.Errorf(
				"port-forward exited early for %s/%s: %w; stdout=%q stderr=%q",
				cfg.Namespace,
				cfg.Service,
				err,
				stdout.String(),
				stderr.String(),
			)
		default:
		}

		req, reqErr := http.NewRequestWithContext(readyCtx, http.MethodGet, readyURL, nil)
		if reqErr != nil {
			_ = handle.Close()
			return nil, fmt.Errorf("build readiness request %s: %w", readyURL, reqErr)
		}
		resp, doErr := http.DefaultClient.Do(req)
		if doErr == nil && resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return handle, nil
			}
			lastErr = fmt.Errorf("status=%d", resp.StatusCode)
		} else {
			lastErr = doErr
		}

		select {
		case <-readyCtx.Done():
			_ = handle.Close()
			return nil, fmt.Errorf(
				"port-forward readiness failed for %s/%s (%s): timeout=%s last_error=%v stdout=%q stderr=%q",
				cfg.Namespace,
				cfg.Service,
				readyURL,
				cfg.ReadyTimeout,
				lastErr,
				stdout.String(),
				stderr.String(),
			)
		case <-time.After(cfg.PollInterval):
		}
	}
}

func allocatePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("unexpected listener addr")
	}
	return tcpAddr.Port, nil
}
