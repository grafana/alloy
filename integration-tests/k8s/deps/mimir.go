package deps

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testNameLabel = "alloy_test_name"
	timeout       = 1 * time.Minute
	retryInterval = 500 * time.Millisecond

	// Both must match manifests/mimir.yaml.
	mimirSelector = "app=mimir"
	mimirHTTPPort = "9009"
)

//go:embed manifests/mimir.yaml
var mimirManifest string

type metricsResponse struct {
	Status string `json:"status"`
	Data   []struct {
		Name string `json:"__name__"`
	} `json:"data"`
}

type metadataResponse struct {
	Status string                        `json:"status"`
	Data   map[string][]ExpectedMetadata `json:"data"`
}

// ExpectedMetadata is both the JSON payload shape from Mimir's metadata
// endpoint and the input to QueryMetadata. Empty fields are not asserted.
type ExpectedMetadata struct {
	Type string `json:"type"`
	Help string `json:"help"`
	Unit string `json:"unit"`
}

// Mimir runs a single-pod Mimir in monolithic mode (filesystem storage,
// in-memory rings). In-cluster URL: http://mimir:9009.
type Mimir struct {
	opts            MimirOptions
	namespace       string
	localPort       string
	stopPortForward func()
	installed       bool
}

type MimirOptions struct {
	Namespace string
}

func NewMimir(opts MimirOptions) *Mimir {
	return &Mimir{opts: opts, namespace: opts.Namespace}
}

func (m *Mimir) Name() string { return "mimir" }

func (m *Mimir) Install(ctx *harness.TestContext) error {
	if m.namespace == "" {
		return fmt.Errorf("mimir namespace is required")
	}

	if err := util.Step("apply mimir manifest", func() error {
		return harness.ApplyManifest(m.namespace, mimirManifest)
	}); err != nil {
		return err
	}
	m.installed = true

	if err := util.Step("wait for mimir pod ready", func() error {
		return harness.WaitForReady(m.namespace, mimirSelector)
	}); err != nil {
		return err
	}

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
	if !m.installed || m.namespace == "" {
		return
	}
	_ = harness.DeleteManifest(m.namespace, mimirManifest)
}

// QueryMetrics polls Mimir for series labelled with alloy_test_name=testName
// and asserts every expected metric name is present.
func (m *Mimir) QueryMetrics(t *testing.T, testName string, expectedMetrics []string) {
	t.Helper()
	mimirURL := m.endpoint("/prometheus/api/v1/")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(mimirURL + "series")
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("match[]", "{"+testNameLabel+"=\""+testName+"\"}")
		queryURL.RawQuery = values.Encode()
		resp := curl(c, queryURL.String())

		var parsed metricsResponse
		err = json.Unmarshal([]byte(resp), &parsed)
		require.NoError(c, err, "failed to parse mimir response: %s", resp)
		require.Equal(c, "success", parsed.Status, "mimir query failed: %s", resp)

		actualMetrics := make(map[string]struct{}, len(parsed.Data))
		for _, metric := range parsed.Data {
			actualMetrics[metric.Name] = struct{}{}
		}

		var missingMetrics []string
		for _, expectedMetric := range expectedMetrics {
			if _, exists := actualMetrics[expectedMetric]; !exists {
				missingMetrics = append(missingMetrics, expectedMetric)
			}
		}

		require.Emptyf(c, missingMetrics, "missing expected metrics for %s=%s: %v found=%v", testNameLabel, testName, missingMetrics, actualMetrics)
	}, timeout, retryInterval)
}

// QueryMetadata asserts each expected metric appears in Mimir's
// /api/v1/metadata with the requested Type/Help/Unit.
func (m *Mimir) QueryMetadata(t *testing.T, expected map[string]ExpectedMetadata) {
	t.Helper()
	endpoint := m.endpoint("/prometheus/api/v1/metadata")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp := curl(c, endpoint)

		var parsed metadataResponse
		err := json.Unmarshal([]byte(resp), &parsed)
		require.NoError(c, err, "failed to parse mimir metadata response: %s", resp)
		require.Equal(c, "success", parsed.Status, "mimir metadata query failed: %s", resp)

		var missing []string
		var mismatched []string
		for name, want := range expected {
			entries, ok := parsed.Data[name]
			if !ok || len(entries) == 0 {
				missing = append(missing, name)
				continue
			}
			got := entries[0]
			if want.Type != "" && got.Type != want.Type {
				mismatched = append(mismatched, fmt.Sprintf("%s: type want=%q got=%q", name, want.Type, got.Type))
			}
			if want.Help != "" && got.Help != want.Help {
				mismatched = append(mismatched, fmt.Sprintf("%s: help want=%q got=%q", name, want.Help, got.Help))
			}
			if want.Unit != "" && got.Unit != want.Unit {
				mismatched = append(mismatched, fmt.Sprintf("%s: unit want=%q got=%q", name, want.Unit, got.Unit))
			}
		}

		require.Emptyf(c, missing, "missing metadata for metrics: %v", missing)
		require.Emptyf(c, mismatched, "metadata mismatches: %v", mismatched)
	}, timeout, retryInterval)
}

func (m *Mimir) CheckAlertsConfig(t *testing.T, expectedFile string) {
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
		"service/mimir",
		localPort+":"+mimirHTTPPort,
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
