package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	intTestLabel  = "alloy_int_test"
	timeout       = 5 * time.Minute
	retryInterval = 500 * time.Millisecond
	diagTimeout   = 20 * time.Second
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

type MimirDependency struct {
	localPort       string
	stopPortForward func()
}

func newMimirDependency() *MimirDependency {
	return &MimirDependency{
		localPort: pickFreeLocalPort(),
	}
}

func (m *MimirDependency) Name() Backend {
	return BackendMimir
}

func (m *MimirDependency) Install(ctx *TestContext) error {
	if err := installMimir(ctx.namespace); err != nil {
		return err
	}
	ctx.registerDiagnosticHook("mimir logs", mimirDiagnosticsHook)

	stop, err := startPortForward(ctx.namespace, m.localPort)
	if err != nil {
		return err
	}
	m.stopPortForward = stop
	return nil
}

func (m *MimirDependency) Cleanup() {
	if m.stopPortForward != nil {
		m.stopPortForward()
	}
}

func (m *MimirDependency) endpoint(path string) string {
	return "http://localhost:" + m.localPort + path
}

func (ctx *TestContext) Mimir(t *testing.T) *MimirDependency {
	t.Helper()
	if ctx.mimir == nil {
		t.Fatal("mimir dependency is not installed for this test")
	}
	return ctx.mimir
}

func (ctx *TestContext) WaitForPodRunning(t *testing.T, namespace, labelSelector string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pods, err := ctx.client.CoreV1().Pods(namespace).List(t.Context(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		require.NoError(c, err)
		require.NotEmpty(c, pods.Items, "no pods for namespace=%s selector=%s", namespace, labelSelector)
		for _, pod := range pods.Items {
			require.Nil(c, pod.DeletionTimestamp, "pod %s is deleting", pod.Name)
			require.Equal(c, corev1.PodRunning, pod.Status.Phase, "pod %s is not running", pod.Name)
		}
	}, timeout, retryInterval)
}

func (m *MimirDependency) QueryMetrics(t *testing.T, alloyIntTest string, expectedMetrics []string) {
	t.Helper()
	mimirURL := m.endpoint("/prometheus/api/v1/")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(mimirURL + "series")
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("match[]", "{"+intTestLabel+"=\""+alloyIntTest+"\"}")
		queryURL.RawQuery = values.Encode()
		query := queryURL.String()
		resp := curl(c, query)

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

func (m *MimirDependency) QueryMetadata(t *testing.T, expectedMetadata map[string]ExpectedMetadata) {
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

func (m *MimirDependency) CheckConfig(t *testing.T, expectedFile string) {
	t.Helper()
	expectedMimirConfigBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	expectedMimirConfig := string(expectedMimirConfigBytes)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := curl(c, m.endpoint("/api/v1/alerts"))
		require.Equal(c, expectedMimirConfig, actualMimirConfig)
	}, timeout, retryInterval)
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
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}, nil
}

type diagnosticHook struct {
	name string
	fn   func(context.Context, *TestContext) error
}

func (ctx *TestContext) registerDiagnosticHook(name string, fn func(context.Context, *TestContext) error) {
	ctx.diagnosticHooks = append(ctx.diagnosticHooks, diagnosticHook{name: name, fn: fn})
}

func collectFailureDiagnostics(ctx *TestContext) {
	fmt.Printf("[k8s-itest] collecting failure diagnostics namespace=%s\n", ctx.namespace)
	for _, hook := range ctx.diagnosticHooks {
		hookCtx, cancel := context.WithTimeout(context.Background(), diagTimeout)
		start := time.Now()
		err := hook.fn(hookCtx, ctx)
		cancel()
		if err != nil {
			fmt.Printf("[k8s-itest] diagnostics hook failed name=%q time=%s err=%v\n", hook.name, time.Since(start).Round(time.Millisecond), err)
			continue
		}
		fmt.Printf("[k8s-itest] diagnostics hook done name=%q time=%s\n", hook.name, time.Since(start).Round(time.Millisecond))
	}
	fmt.Printf("[k8s-itest] repro: make integration-test-k8s RUN_ARGS='--package ./integration-tests/k8s/tests/%s'\n", ctx.name)
	fmt.Printf("[k8s-itest] kubeconfig: %s\n", os.Getenv(kubeconfigEnv))
}

func namespaceDiagnosticsHook(c context.Context, ctx *TestContext) error {
	return runDiagnosticCommands(c, [][]string{
		{"kubectl", "--namespace", ctx.namespace, "get", "pods", "-o", "wide"},
		{"kubectl", "--namespace", ctx.namespace, "describe", "pods"},
	})
}

func alloyDiagnosticsHook(c context.Context, ctx *TestContext) error {
	return runDiagnosticCommands(c, [][]string{
		{"kubectl", "--namespace", ctx.namespace, "logs", "-l", "app.kubernetes.io/name=alloy", "--all-containers=true", "--tail", "200"},
	})
}

func mimirDiagnosticsHook(c context.Context, ctx *TestContext) error {
	return runDiagnosticCommands(c, [][]string{
		{"kubectl", "--namespace", ctx.namespace, "logs", "-l", "app.kubernetes.io/component=distributor", "--all-containers=true", "--tail", "200"},
		{"kubectl", "--namespace", ctx.namespace, "logs", "-l", "app.kubernetes.io/component=alertmanager", "--all-containers=true", "--tail", "200"},
	})
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

func curl(c *assert.CollectT, targetURL string) string {
	resp, err := http.Get(targetURL)
	require.NoError(c, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(c, err)
	return string(body)
}
