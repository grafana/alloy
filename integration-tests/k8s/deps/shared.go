package deps

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func startPortForwardWithRetries(namespace string, attempts int, port string) (string, func(), error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		localPort, err := pickFreeLocalPort()
		if err != nil {
			lastErr = err
			continue
		}
		stop, err := startPortForward(namespace, localPort, port)
		if err == nil {
			return localPort, stop, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unable to allocate local port for port-forward")
	}
	return "", nil, fmt.Errorf("failed to start mimir port-forward after %d attempts: %w", attempts, lastErr)
}

func startPortForward(namespace, localPort, port string) (func(), error) {
	cmd := exec.CommandContext(
		context.Background(),
		"kubectl",
		"port-forward",
		"--namespace", namespace,
		"service/mimir",
		localPort+":"+port,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = harness.CommandEnv()
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		return nil, fmt.Errorf("port-forward exited early: %w", err)
	case <-time.After(500 * time.Millisecond):
	}

	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
		}
	}, nil
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

func curl(c *assert.CollectT, targetURL string) string {
	client := http.Client{Timeout: curlTimeout}
	resp, err := client.Get(targetURL)
	require.NoError(c, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(c, err)
	return string(body)
}
