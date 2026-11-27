package deps

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const (
	testNameLabel = "alloy_test_name"
	timeout       = 1 * time.Minute
	retryInterval = 500 * time.Millisecond
)

func startPortForwardWithRetries(namespace, name string, attempts int, port string) (string, func(), error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		localPort, err := pickFreeLocalPort()
		if err != nil {
			lastErr = err
			continue
		}
		stop, err := startPortForward(namespace, name, localPort, port)
		if err == nil {
			return localPort, stop, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unable to allocate local port for port-forward")
	}
	return "", nil, fmt.Errorf("failed to start port-forward after %d attempts: %w", attempts, lastErr)
}

func startPortForward(namespace, name, localPort, port string) (func(), error) {
	cmd := exec.CommandContext(
		context.Background(),
		"kubectl",
		"port-forward",
		"--namespace", namespace,
		"service/"+name,
		localPort+":"+port,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = harness.CommandEnv()
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()

	if err := waitForPortForwardReady(localPort, exited); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}

	return func() {
		_ = cmd.Process.Kill()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			fmt.Fprintf(os.Stderr, "warning: port-forward to service/%s didn't exit within 5s of being killed\n", name)
		}
	}, nil
}

// portForwardReadyTimeout is how long to wait for kubectl port-forward to
// accept connections on the local port.
const portForwardReadyTimeout = 30 * time.Second

// waitForPortForwardReady waits until the local port accepts connections.
// kubectl has no command to check this, and a fixed delay isn't enough on a
// busy runner.
func waitForPortForwardReady(localPort string, exited <-chan error) error {
	addr := net.JoinHostPort("localhost", localPort)
	timeout := time.After(portForwardReadyTimeout)
	for {
		if conn, err := net.Dial("tcp", addr); err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case err := <-exited:
			return fmt.Errorf("port-forward exited early: %w", err)
		case <-timeout:
			return fmt.Errorf("port-forward not ready on %s after %s", addr, portForwardReadyTimeout)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func pickFreeLocalPort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}
	return port, nil
}

// curlTimeout caps each HTTP attempt so a stalled port-forward doesn't
// block the outer EventuallyWithT past its deadline.
const curlTimeout = 5 * time.Second

func curl(c *assert.CollectT, targetURL string, header http.Header) string {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	require.NoError(c, err)
	req.Header = header

	client := http.Client{Timeout: curlTimeout}
	resp, err := client.Do(req)
	require.NoError(c, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(c, err)
	return string(body)
}

func pkgDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("could not determine package directory")
	}
	return filepath.Dir(file), nil
}
