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

	// exited is closed once the process exits, after waitErr is set.
	var waitErr error
	exited := make(chan struct{})
	go func() {
		waitErr = cmd.Wait()
		close(exited)
	}()

	stop := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
	}

	if err := waitForLocalPort(localPort, exited, &waitErr); err != nil {
		stop()
		return nil, err
	}
	return stop, nil
}

// portForwardReadyTimeout is how long to wait for kubectl port-forward to
// accept connections on the local port.
const portForwardReadyTimeout = 30 * time.Second

// waitForLocalPort waits until something accepts TCP connections on
// localPort, or the port-forward process exits. kubectl starts listening some
// time after it starts, and on a busy runner a fixed delay isn't enough: tests
// then fail with "connection refused" on the local port.
func waitForLocalPort(localPort string, exited <-chan struct{}, waitErr *error) error {
	addr := net.JoinHostPort("localhost", localPort)
	deadline := time.Now().Add(portForwardReadyTimeout)
	for {
		select {
		case <-exited:
			return fmt.Errorf("port-forward exited early: %w", *waitErr)
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("port-forward not accepting connections on %s after %s: %w", addr, portForwardReadyTimeout, err)
		}
		time.Sleep(100 * time.Millisecond)
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
