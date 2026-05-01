package deps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	intTestLabel  = "alloy_int_test"
	timeout       = 5 * time.Minute
	retryInterval = 500 * time.Millisecond
	diagTimeout   = 20 * time.Second

	kubeconfigEnv = "ALLOY_TESTS_KUBECONFIG"
)

type MetricsResponse struct {
	Status string `json:"status"`
	Data   []struct {
		Name string `json:"__name__"`
	} `json:"data"`
}

type MetadataResponse struct {
	Status string                     `json:"status"`
	Data   map[string][]MetadataEntry `json:"data"`
}

type MetadataEntry struct {
	Type string `json:"type"`
	Help string `json:"help"`
	Unit string `json:"unit"`
}

type ExpectedMetadata struct {
	Type string
	Help string
}

type Mimir struct {
	namespace       string
	localPort       string
	stopPortForward func()
}

type MimirOptions struct {
	Namespace string
}

func NewMimir(opts MimirOptions) *Mimir {
	return &Mimir{namespace: opts.Namespace}
}

func (m *Mimir) Name() string {
	return "mimir"
}

func (m *Mimir) Install(ctx *harness.TestContext) error {
	if m.namespace == "" {
		m.namespace = ctx.Namespace()
	}
	if m.namespace == "" {
		return fmt.Errorf("mimir namespace is required")
	}

	if err := installMimir(m.namespace); err != nil {
		return err
	}
	ctx.AddDiagnosticHook("mimir logs", m.diagnosticsHook())

	localPort, stop, err := startPortForwardWithRetries(m.namespace, 5)
	if err != nil {
		return err
	}
	m.localPort = localPort
	m.stopPortForward = stop
	return nil
}

func (m *Mimir) Cleanup() {
	if m.stopPortForward != nil {
		m.stopPortForward()
	}
}

func (m *Mimir) QueryMetrics(t *testing.T, alloyIntTest string, expectedMetrics []string) {
	t.Helper()
	mimirURL := m.endpoint("/prometheus/api/v1/")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(mimirURL + "series")
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("match[]", "{"+intTestLabel+"=\""+alloyIntTest+"\"}")
		queryURL.RawQuery = values.Encode()
		resp := curl(c, queryURL.String())

		var metricsResponse MetricsResponse
		err = json.Unmarshal([]byte(resp), &metricsResponse)
		require.NoError(c, err, "failed to parse mimir response: %s", resp)
		require.Equal(c, "success", metricsResponse.Status, "mimir query failed: %s", resp)

		actualMetrics := make(map[string]struct{}, len(metricsResponse.Data))
		for _, metric := range metricsResponse.Data {
			actualMetrics[metric.Name] = struct{}{}
		}

		var missingMetrics []string
		for _, expectedMetric := range expectedMetrics {
			if _, exists := actualMetrics[expectedMetric]; !exists {
				missingMetrics = append(missingMetrics, expectedMetric)
			}
		}

		require.Emptyf(c, missingMetrics, "missing expected metrics for %s=%s: %v found=%v", intTestLabel, alloyIntTest, missingMetrics, actualMetrics)
	}, timeout, retryInterval)
}

func (m *Mimir) QueryMetadata(t *testing.T, expectedMetadata map[string]ExpectedMetadata) {
	t.Helper()
	mimirURL := m.endpoint("/prometheus/api/v1/")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp := curl(c, mimirURL+"metadata")
		var metadataResponse MetadataResponse
		err := json.Unmarshal([]byte(resp), &metadataResponse)
		require.NoError(c, err, "failed to parse metadata response: %s", resp)
		require.Equal(c, "success", metadataResponse.Status, "mimir metadata query failed: %s", resp)

		var missingMetrics []string
		var mismatchedMetrics []string
		for metricName, expected := range expectedMetadata {
			entries, exists := metadataResponse.Data[metricName]
			if !exists || len(entries) == 0 {
				missingMetrics = append(missingMetrics, metricName)
				continue
			}
			entry := entries[0]
			if expected.Type != "" && entry.Type != expected.Type {
				mismatchedMetrics = append(mismatchedMetrics, metricName+": expected type="+expected.Type+", got="+entry.Type)
			}
			if expected.Help != "" && entry.Help != expected.Help {
				mismatchedMetrics = append(mismatchedMetrics, metricName+": expected help="+expected.Help+", got="+entry.Help)
			}
		}

		require.Emptyf(c, missingMetrics, "missing expected metadata for metrics: %v", missingMetrics)
		require.Emptyf(c, mismatchedMetrics, "mismatched metadata: %v", mismatchedMetrics)
	}, timeout, retryInterval)
}

func (m *Mimir) CheckConfig(t *testing.T, expectedFile string) {
	t.Helper()
	expectedMimirConfigBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	expectedMimirConfig := string(expectedMimirConfigBytes)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := curl(c, m.endpoint("/api/v1/alerts"))
		require.Equal(c, expectedMimirConfig, actualMimirConfig)
	}, timeout, retryInterval)
}

func (m *Mimir) endpoint(path string) string {
	return "http://localhost:" + m.localPort + path
}

func (m *Mimir) diagnosticsHook() func(context.Context) error {
	namespace := m.namespace
	return func(c context.Context) error {
		hookCtx, cancel := context.WithTimeout(c, diagTimeout)
		defer cancel()
		return runDiagnosticCommands(hookCtx, [][]string{
			{"kubectl", "--namespace", namespace, "logs", "-l", "app.kubernetes.io/component=distributor", "--all-containers=true", "--tail", "200"},
			{"kubectl", "--namespace", namespace, "logs", "-l", "app.kubernetes.io/component=alertmanager", "--all-containers=true", "--tail", "200"},
		})
	}
}

func installMimir(namespace string) error {
	if err := step("helm repo add grafana", func() error {
		return runCommand("helm", "repo", "add", "grafana", "https://grafana.github.io/helm-charts")
	}); err != nil {
		return err
	}
	if err := step("helm repo update", func() error {
		return runCommand("helm", "repo", "update")
	}); err != nil {
		return err
	}
	return step("install mimir", func() error {
		return runCommand(
			"helm",
			"upgrade",
			"--install",
			"mimir",
			"grafana/mimir-distributed",
			"--version", "5.8.0",
			"--namespace", namespace,
			"--create-namespace",
			"--wait",
		)
	})
}

func startPortForwardWithRetries(namespace string, attempts int) (string, func(), error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		localPort, err := pickFreeLocalPort()
		if err != nil {
			lastErr = err
			continue
		}
		stop, err := startPortForward(namespace, localPort)
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

func startPortForward(namespace, localPort string) (func(), error) {
	cmd := exec.CommandContext(
		context.Background(),
		"kubectl",
		"port-forward",
		"--namespace", namespace,
		"service/mimir-nginx",
		localPort+":80",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = commandEnv()
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

func runDiagnosticCommands(c context.Context, commands [][]string) error {
	var errs []string
	for _, args := range commands {
		if len(args) == 0 {
			continue
		}
		if err := runDiagnosticCommand(c, args[0], args[1:]...); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func runDiagnosticCommand(c context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(c, name, args...)
	cmd.Env = commandEnv()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if out.Len() > 0 {
		fmt.Printf("%s", out.String())
	}
	if err == nil {
		return nil
	}
	if c.Err() != nil {
		return fmt.Errorf("%s %v timed out: %w", name, args, c.Err())
	}
	return fmt.Errorf("%s %v failed: %w", name, args, err)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = commandEnv()
	return cmd.Run()
}

func commandEnv() []string {
	env := os.Environ()
	if kubeconfig := os.Getenv(kubeconfigEnv); kubeconfig != "" {
		env = append(env, "KUBECONFIG="+kubeconfig)
	}
	return env
}

func step(name string, fn func() error) error {
	start := time.Now()
	fmt.Printf("[k8s-itest] %s...\n", name)
	err := fn()
	if err != nil {
		fmt.Printf("[k8s-itest] failed %s time=%s err=%v\n", name, time.Since(start).Round(time.Millisecond), err)
		return err
	}
	fmt.Printf("[k8s-itest] done %s time=%s\n", name, time.Since(start).Round(time.Millisecond))
	return nil
}

func curl(c *assert.CollectT, targetURL string) string {
	resp, err := http.Get(targetURL)
	require.NoError(c, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(c, err)
	return string(body)
}
