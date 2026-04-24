package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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
	}, 5*time.Minute, 2*time.Second)
}

func (ctx *TestContext) Curl(c *assert.CollectT, url string) string {
	resp, err := http.Get(url)
	require.NoError(c, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(c, err)
	return string(body)
}

func (ctx *TestContext) QueryMimirMetrics(t *testing.T, alloyIntTest string, expectedMetrics []string) {
	t.Helper()
	mimirURL := "http://localhost:" + ctx.MimirLocalPort + "/prometheus/api/v1/"

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(mimirURL + "series")
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("match[]", "{"+intTestLabel+"=\""+alloyIntTest+"\"}")
		queryURL.RawQuery = values.Encode()
		query := queryURL.String()
		resp := ctx.Curl(c, query)

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

func (ctx *TestContext) QueryMimirMetadata(t *testing.T, expectedMetadata map[string]ExpectedMetadata) {
	t.Helper()
	mimirURL := "http://localhost:" + ctx.MimirLocalPort + "/prometheus/api/v1/"

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp := ctx.Curl(c, mimirURL+"metadata")
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

func (ctx *TestContext) CheckMimirConfig(t *testing.T, expectedFile string) {
	t.Helper()
	expectedMimirConfigBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	expectedMimirConfig := string(expectedMimirConfigBytes)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := ctx.Curl(c, "http://localhost:"+ctx.MimirLocalPort+"/api/v1/alerts")
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
	cmd.Env = os.Environ()
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

func collectFailureDiagnostics(ctx *TestContext) {
	fmt.Printf("[k8s-itest] collecting failure diagnostics namespace=%s\n", ctx.Namespace)
	_ = runCommand("kubectl", "--namespace", ctx.Namespace, "get", "pods", "-o", "wide")
	_ = runCommand("kubectl", "--namespace", ctx.Namespace, "describe", "pods")
	_ = runCommand("kubectl", "--namespace", ctx.Namespace, "logs", "deployment/alloy", "--all-containers=true", "--tail", "200")
	_ = runCommand("kubectl", "--namespace", ctx.Namespace, "logs", "deployment/prom-gen", "--all-containers=true", "--tail", "200")
	_ = runCommand("kubectl", "--namespace", ctx.Namespace, "logs", "deployment/blackbox-exporter", "--all-containers=true", "--tail", "200")
	fmt.Printf("[k8s-itest] repro: make integration-test-k8s RUN_ARGS='--package ./integration-tests/k8s/tests/%s'\n", ctx.Name)
	fmt.Printf("[k8s-itest] kubeconfig: %s\n", os.Getenv("KUBECONFIG"))
}
