package deps

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
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

	localPort, stop, err := startPortForwardWithRetries(m.namespace, "mimir", 5, mimirHTTPPort)
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
		resp := curl(c, queryURL.String(), nil)

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
		resp := curl(c, endpoint, nil)

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

// QueryEquals runs an instant PromQL query and asserts (with retries) that the
// single scalar/vector result equals want. Handy for clustering assertions like
// count(up{...}) == number of expected targets.
func (m *Mimir) QueryEquals(t *testing.T, query string, want float64) {
	t.Helper()
	endpoint := m.endpoint("/prometheus/api/v1/query")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(endpoint)
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("query", query)
		queryURL.RawQuery = values.Encode()
		resp := curl(c, queryURL.String(), nil)

		var parsed queryResponse
		err = json.Unmarshal([]byte(resp), &parsed)
		require.NoError(c, err, "failed to parse mimir query response: %s", resp)
		require.Equal(c, "success", parsed.Status, "mimir query failed: %s", resp)
		require.NotEmptyf(c, parsed.Data.Result, "query %q returned no data", query)

		raw, ok := parsed.Data.Result[0].Value[1].(string)
		require.Truef(c, ok, "unexpected value shape in %s", resp)
		got, err := strconv.ParseFloat(raw, 64)
		require.NoError(c, err)
		require.Equalf(c, want, got, "query %q: want %v got %v", query, want, got)
	}, timeout, retryInterval)
}

// QueryAtLeast runs an instant PromQL query and asserts (with retries) that the
// single scalar/vector result is >= min. Useful for throughput checks like
// sum(scrape_samples_scraped{...}) >= N where the exact value isn't fixed.
func (m *Mimir) QueryAtLeast(t *testing.T, query string, min float64) {
	t.Helper()
	endpoint := m.endpoint("/prometheus/api/v1/query")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		queryURL, err := url.Parse(endpoint)
		require.NoError(c, err)
		values := queryURL.Query()
		values.Add("query", query)
		queryURL.RawQuery = values.Encode()
		resp := curl(c, queryURL.String(), nil)

		var parsed queryResponse
		err = json.Unmarshal([]byte(resp), &parsed)
		require.NoError(c, err, "failed to parse mimir query response: %s", resp)
		require.Equal(c, "success", parsed.Status, "mimir query failed: %s", resp)
		require.NotEmptyf(c, parsed.Data.Result, "query %q returned no data", query)

		raw, ok := parsed.Data.Result[0].Value[1].(string)
		require.Truef(c, ok, "unexpected value shape in %s", resp)
		got, err := strconv.ParseFloat(raw, 64)
		require.NoError(c, err)
		require.GreaterOrEqualf(c, got, min, "query %q: want >= %v got %v", query, min, got)
	}, timeout, retryInterval)
}

// QueryEqualsQuery polls until two instant PromQL queries return the same
// (non-zero) scalar, or fails after within. Used to assert an Alloy-reported
// total equals the ground truth, e.g.
// sum(prometheus_remote_write_wal_storage_active_series{...}) == count(real series).
func (m *Mimir) QueryEqualsQuery(t *testing.T, lhs, rhs string, within time.Duration, msg string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		l, okL := m.queryScalar(c, lhs)
		r, okR := m.queryScalar(c, rhs)
		require.True(c, okL, "no data for %q", lhs)
		require.True(c, okR, "no data for %q", rhs)
		require.NotZero(c, r, "ground-truth query %q returned 0", rhs)
		require.Equalf(c, r, l, "%s: %q=%v but %q=%v", msg, lhs, l, rhs, r)
	}, within, 5*time.Second)
}

// queryScalar runs an instant query and returns the first result's value.
func (m *Mimir) queryScalar(c *assert.CollectT, query string) (float64, bool) {
	queryURL, err := url.Parse(m.endpoint("/prometheus/api/v1/query"))
	require.NoError(c, err)
	values := queryURL.Query()
	values.Add("query", query)
	queryURL.RawQuery = values.Encode()
	resp := curl(c, queryURL.String(), nil)

	var parsed queryResponse
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil || parsed.Status != "success" || len(parsed.Data.Result) == 0 {
		return 0, false
	}
	raw, ok := parsed.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

type queryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"` // [ <ts>, "<value>" ]
		} `json:"result"`
	} `json:"data"`
}

func (m *Mimir) CheckAlertsConfig(t *testing.T, expectedFile string) {
	t.Helper()
	expectedMimirConfigBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	expectedMimirConfig := string(expectedMimirConfigBytes)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := curl(c, m.endpoint("/api/v1/alerts"), nil)
		require.Equal(c, expectedMimirConfig, actualMimirConfig)
	}, timeout, retryInterval)
}

func (m *Mimir) endpoint(path string) string {
	return "http://localhost:" + m.localPort + path
}
